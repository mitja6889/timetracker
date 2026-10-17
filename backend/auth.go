package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type ctxKey struct{}

// Auth je prijavljen uporabnik z njegovo rolo (naložena iz baze ob vsakem zahtevku,
// zato se spremembe pravic uveljavijo takoj).
type Auth struct {
	User User
	Role Role
}

func (a *Auth) Has(perm string) bool {
	for _, p := range a.Role.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

func authFrom(r *http.Request) *Auth {
	a, _ := r.Context().Value(ctxKey{}).(*Auth)
	return a
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if !readJSON(w, r, &req) {
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))

	var u User
	err := s.users.FindOne(r.Context(), bson.M{"username": username}).Decode(&u)
	// enako sporočilo za neobstoječega uporabnika in napačno geslo
	if err != nil || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "napačno uporabniško ime ali geslo")
		return
	}
	if !u.Active {
		writeErr(w, http.StatusForbidden, "uporabnik je onemogočen")
		return
	}

	claims := jwt.RegisteredClaims{
		Subject:   u.ID.Hex(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka pri izdaji žetona")
		return
	}

	a, err := s.loadAuth(r.Context(), u.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka pri nalaganju uporabnika")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "me": meDTO(a)})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, meDTO(authFrom(r)))
}

func meDTO(a *Auth) map[string]any {
	return map[string]any{
		"id":          a.User.ID.Hex(),
		"username":    a.User.Username,
		"name":        a.User.Name,
		"roleId":      a.User.RoleID.Hex(),
		"roleName":    a.Role.Name,
		"permissions": a.Role.Permissions,
	}
}

func (s *Server) loadAuth(ctx context.Context, id primitive.ObjectID) (*Auth, error) {
	var u User
	if err := s.users.FindOne(ctx, bson.M{"_id": id}).Decode(&u); err != nil {
		return nil, err
	}
	var role Role
	if err := s.roles.FindOne(ctx, bson.M{"_id": u.RoleID}).Decode(&role); err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		role = Role{Name: "(brez role)", Permissions: []string{}}
	}
	if role.Permissions == nil {
		role.Permissions = []string{}
	}
	return &Auth{User: u, Role: role}, nil
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeErr(w, http.StatusUnauthorized, "manjka prijava")
			return
		}
		claims := &jwt.RegisteredClaims{}
		tok, err := jwt.ParseWithClaims(strings.TrimPrefix(h, "Bearer "), claims,
			func(t *jwt.Token) (any, error) { return s.jwtSecret, nil },
			jwt.WithValidMethods([]string{"HS256"}))
		if err != nil || !tok.Valid {
			writeErr(w, http.StatusUnauthorized, "neveljavna ali pretekla seja")
			return
		}
		id, err := primitive.ObjectIDFromHex(claims.Subject)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "neveljavna seja")
			return
		}
		a, err := s.loadAuth(r.Context(), id)
		if err != nil || !a.User.Active {
			writeErr(w, http.StatusUnauthorized, "uporabnik ne obstaja ali je onemogočen")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, a)))
	}
}

func (s *Server) requirePerm(perm string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAnyPerm([]string{perm}, next)
}

func (s *Server) requireAnyPerm(perms []string, next http.HandlerFunc) http.HandlerFunc {
	return s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		a := authFrom(r)
		for _, p := range perms {
			if a.Has(p) {
				next(w, r)
				return
			}
		}
		writeErr(w, http.StatusForbidden, "nimate pravice za to dejanje")
	})
}

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}
