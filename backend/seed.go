package main

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// seed ob prvem zagonu (prazna zbirka uporabnikov) ustvari role "Admin" in "Uporabnik"
// ter administratorja. Poverilnice so iz ADMIN_USERNAME / ADMIN_PASSWORD.
func (s *Server) seed(ctx context.Context) error {
	n, err := s.users.CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	adminRoleID, err := s.upsertSystemRole(ctx, "Admin", "Polni dostop do vsega", allPermissionKeys(), true)
	if err != nil {
		return err
	}
	if _, err := s.upsertSystemRole(ctx, "Uporabnik", "Beleži lasten čas; vidi samo svoje vnose",
		[]string{PermTimerUse}, false); err != nil {
		return err
	}

	username := env("ADMIN_USERNAME", "admin")
	password := env("ADMIN_PASSWORD", "admin123")
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.users.InsertOne(ctx, User{
		Username:     username,
		Name:         "Administrator",
		PasswordHash: hash,
		RoleID:       adminRoleID,
		Active:       true,
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	log.Printf("Prvi zagon: ustvarjen admin uporabnik %q (geslo iz ADMIN_PASSWORD, privzeto %q) - GESLO TAKOJ SPREMENITE", username, "admin123")
	return nil
}

func (s *Server) upsertSystemRole(ctx context.Context, name, desc string, perms []string, system bool) (primitive.ObjectID, error) {
	var existing Role
	if err := s.roles.FindOne(ctx, bson.M{"name": name}).Decode(&existing); err == nil {
		return existing.ID, nil
	}
	res, err := s.roles.InsertOne(ctx, Role{Name: name, Description: desc, Permissions: perms, System: system})
	if err != nil {
		return primitive.NilObjectID, err
	}
	return res.InsertedID.(primitive.ObjectID), nil
}
