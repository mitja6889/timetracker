package main

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var usernameRe = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)

type userDTO struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	RoleID    string     `json:"roleId,omitempty"`
	RoleName  string     `json:"roleName,omitempty"`
	Active    *bool      `json:"active,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
}

type userReq struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Password string `json:"password"`
	RoleID   string `json:"roleId"`
	Active   *bool  `json:"active"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	full := a.Has(PermUsersManage)

	cur, err := s.users.Find(r.Context(), bson.M{})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var users []User
	if err := cur.All(r.Context(), &users); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}

	roleNames := map[primitive.ObjectID]string{}
	if full {
		rc, err := s.roles.Find(r.Context(), bson.M{})
		if err == nil {
			var roles []Role
			if rc.All(r.Context(), &roles) == nil {
				for _, ro := range roles {
					roleNames[ro.ID] = ro.Name
				}
			}
		}
	}

	out := make([]userDTO, 0, len(users))
	for _, u := range users {
		d := userDTO{ID: u.ID.Hex(), Username: u.Username, Name: u.Name}
		if full {
			active := u.Active
			d.RoleID = u.RoleID.Hex()
			d.RoleName = roleNames[u.RoleID]
			d.Active = &active
			created := u.CreatedAt
			d.CreatedAt = &created
		}
		out = append(out, d)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) roleExists(r *http.Request, id primitive.ObjectID) (*Role, bool) {
	var role Role
	if err := s.roles.FindOne(r.Context(), bson.M{"_id": id}).Decode(&role); err != nil {
		return nil, false
	}
	return &role, true
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req userReq
	if !readJSON(w, r, &req) {
		return
	}
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	req.Name = strings.TrimSpace(req.Name)
	if !usernameRe.MatchString(req.Username) {
		writeErr(w, http.StatusBadRequest, "uporabniško ime: 3-32 znakov (a-z, 0-9, . _ -)")
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "ime je obvezno")
		return
	}
	if len(req.Password) < 8 {
		writeErr(w, http.StatusBadRequest, "geslo mora imeti vsaj 8 znakov")
		return
	}
	roleID, err := primitive.ObjectIDFromHex(req.RoleID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljavna role")
		return
	}
	if _, ok := s.roleExists(r, roleID); !ok {
		writeErr(w, http.StatusBadRequest, "role ne obstaja")
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka pri šifriranju gesla")
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	u := User{
		Username:     req.Username,
		Name:         req.Name,
		PasswordHash: hash,
		RoleID:       roleID,
		Active:       active,
		CreatedAt:    time.Now().UTC(),
	}
	res, err := s.users.InsertOne(r.Context(), u)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeErr(w, http.StatusConflict, "uporabniško ime je že zasedeno")
			return
		}
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": res.InsertedID.(primitive.ObjectID).Hex()})
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req userReq
	if !readJSON(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "ime je obvezno")
		return
	}
	roleID, err := primitive.ObjectIDFromHex(req.RoleID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljavna role")
		return
	}
	newRole, ok := s.roleExists(r, roleID)
	if !ok {
		writeErr(w, http.StatusBadRequest, "role ne obstaja")
		return
	}

	var existing User
	if err := s.users.FindOne(r.Context(), bson.M{"_id": id}).Decode(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeErr(w, http.StatusNotFound, "uporabnik ni najden")
			return
		}
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	active := existing.Active
	if req.Active != nil {
		active = *req.Active
	}

	// zaščita: vsaj en aktiven upravljalec uporabnikov mora ostati
	oldRole, _ := s.roleExists(r, existing.RoleID)
	wasManager := existing.Active && oldRole != nil && hasPerm(oldRole.Permissions, PermUsersManage)
	willBeManager := active && hasPerm(newRole.Permissions, PermUsersManage)
	if wasManager && !willBeManager {
		n, err := s.activeManagersExcluding(r.Context(), id, primitive.NilObjectID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
			return
		}
		if n == 0 {
			writeErr(w, http.StatusConflict, "to je zadnji aktivni uporabnik s pravico upravljanja uporabnikov")
			return
		}
	}

	set := bson.M{"name": req.Name, "roleId": roleID, "active": active}
	if req.Password != "" {
		if len(req.Password) < 8 {
			writeErr(w, http.StatusBadRequest, "geslo mora imeti vsaj 8 znakov")
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "napaka pri šifriranju gesla")
			return
		}
		set["passwordHash"] = hash
	}
	if _, err := s.users.UpdateOne(r.Context(), bson.M{"_id": id}, bson.M{"$set": set}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	a := authFrom(r)
	if id == a.User.ID {
		writeErr(w, http.StatusConflict, "ne morete izbrisati samega sebe")
		return
	}
	var existing User
	if err := s.users.FindOne(r.Context(), bson.M{"_id": id}).Decode(&existing); err != nil {
		writeErr(w, http.StatusNotFound, "uporabnik ni najden")
		return
	}
	if role, ok := s.roleExists(r, existing.RoleID); ok && existing.Active && hasPerm(role.Permissions, PermUsersManage) {
		n, err := s.activeManagersExcluding(r.Context(), id, primitive.NilObjectID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
			return
		}
		if n == 0 {
			writeErr(w, http.StatusConflict, "to je zadnji aktivni uporabnik s pravico upravljanja uporabnikov")
			return
		}
	}
	if _, err := s.entries.DeleteMany(r.Context(), bson.M{"userId": id}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	if _, err := s.users.DeleteOne(r.Context(), bson.M{"_id": id}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
