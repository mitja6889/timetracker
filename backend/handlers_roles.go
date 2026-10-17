package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type roleReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

type roleDTO struct {
	Role
	UserCount int64 `json:"userCount"`
}

func (s *Server) handlePermissions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, PermissionCatalog)
}

func (s *Server) handleListRoles(w http.ResponseWriter, r *http.Request) {
	cur, err := s.roles.Find(r.Context(), bson.M{})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var roles []Role
	if err := cur.All(r.Context(), &roles); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	out := make([]roleDTO, 0, len(roles))
	for _, role := range roles {
		if role.Permissions == nil {
			role.Permissions = []string{}
		}
		n, _ := s.users.CountDocuments(r.Context(), bson.M{"roleId": role.ID})
		out = append(out, roleDTO{Role: role, UserCount: n})
	}
	writeJSON(w, http.StatusOK, out)
}

// normalizeRole preveri vnos in vrne očiščen seznam pravic.
func normalizeRole(req *roleReq) (string, bool) {
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) < 2 || len(req.Name) > 50 {
		return "ime role mora imeti 2-50 znakov", false
	}
	seen := map[string]bool{}
	perms := []string{}
	for _, p := range req.Permissions {
		if !validPermission(p) {
			return "neznana pravica: " + p, false
		}
		if !seen[p] {
			seen[p] = true
			perms = append(perms, p)
		}
	}
	req.Permissions = perms
	return "", true
}

func (s *Server) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req roleReq
	if !readJSON(w, r, &req) {
		return
	}
	if msg, ok := normalizeRole(&req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	role := Role{Name: req.Name, Description: strings.TrimSpace(req.Description), Permissions: req.Permissions}
	res, err := s.roles.InsertOne(r.Context(), role)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeErr(w, http.StatusConflict, "role s tem imenom že obstaja")
			return
		}
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	role.ID = res.InsertedID.(primitive.ObjectID)
	writeJSON(w, http.StatusCreated, roleDTO{Role: role})
}

func (s *Server) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var req roleReq
	if !readJSON(w, r, &req) {
		return
	}
	if msg, ok := normalizeRole(&req); !ok {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	var existing Role
	if err := s.roles.FindOne(r.Context(), bson.M{"_id": id}).Decode(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeErr(w, http.StatusNotFound, "role ni najdena")
			return
		}
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}

	if existing.System {
		// sistemska role (Admin): ime in pravice so zaklenjene, opis se lahko ureja
		req.Name = existing.Name
		req.Permissions = existing.Permissions
	} else if hasPerm(existing.Permissions, PermUsersManage) && !hasPerm(req.Permissions, PermUsersManage) {
		// ne dovoli, da bi ostali brez upravljalca uporabnikov
		n, err := s.activeManagersExcluding(r.Context(), primitive.NilObjectID, id)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
			return
		}
		if n == 0 {
			writeErr(w, http.StatusConflict, "ostal ne bi noben aktiven uporabnik s pravico upravljanja uporabnikov")
			return
		}
	}

	_, err := s.roles.UpdateOne(r.Context(), bson.M{"_id": id}, bson.M{"$set": bson.M{
		"name":        req.Name,
		"description": strings.TrimSpace(req.Description),
		"permissions": req.Permissions,
	}})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeErr(w, http.StatusConflict, "role s tem imenom že obstaja")
			return
		}
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var existing Role
	if err := s.roles.FindOne(r.Context(), bson.M{"_id": id}).Decode(&existing); err != nil {
		writeErr(w, http.StatusNotFound, "role ni najdena")
		return
	}
	if existing.System {
		writeErr(w, http.StatusConflict, "sistemske role ni mogoče izbrisati")
		return
	}
	n, err := s.users.CountDocuments(r.Context(), bson.M{"roleId": id})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	if n > 0 {
		writeErr(w, http.StatusConflict, "role so dodeljeni uporabniki; najprej jih prestavite v drugo rolo")
		return
	}
	if _, err := s.roles.DeleteOne(r.Context(), bson.M{"_id": id}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func hasPerm(perms []string, p string) bool {
	for _, x := range perms {
		if x == p {
			return true
		}
	}
	return false
}

// activeManagersExcluding prešteje aktivne uporabnike (razen excludeUser) z rolo, ki ima
// pravico users.manage (razen excludeRole).
func (s *Server) activeManagersExcluding(ctx context.Context, excludeUser, excludeRole primitive.ObjectID) (int64, error) {
	cur, err := s.roles.Find(ctx, bson.M{
		"permissions": PermUsersManage,
		"_id":         bson.M{"$ne": excludeRole},
	})
	if err != nil {
		return 0, err
	}
	var roles []Role
	if err := cur.All(ctx, &roles); err != nil {
		return 0, err
	}
	if len(roles) == 0 {
		return 0, nil
	}
	ids := make([]primitive.ObjectID, 0, len(roles))
	for _, ro := range roles {
		ids = append(ids, ro.ID)
	}
	return s.users.CountDocuments(ctx, bson.M{
		"roleId": bson.M{"$in": ids},
		"active": true,
		"_id":    bson.M{"$ne": excludeUser},
	})
}
