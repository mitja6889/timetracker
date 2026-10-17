package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Server struct {
	db        *mongo.Database
	users     *mongo.Collection
	roles     *mongo.Collection
	entries   *mongo.Collection
	jwtSecret []byte
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(env("MONGO_URI", "mongodb://localhost:27017")))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("mongo ping: %v", err)
	}

	secret := env("JWT_SECRET", "")
	if secret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		secret = hex.EncodeToString(b)
		log.Println("OPOZORILO: JWT_SECRET ni nastavljen, uporabljam naključnega (seje potečejo ob ponovnem zagonu)")
	}

	db := client.Database(env("MONGO_DB", "timetracker"))
	s := &Server{
		db:        db,
		users:     db.Collection("users"),
		roles:     db.Collection("roles"),
		entries:   db.Collection("entries"),
		jwtSecret: []byte(secret),
	}

	if err := s.ensureIndexes(ctx); err != nil {
		log.Fatalf("indexes: %v", err)
	}
	if err := s.seed(ctx); err != nil {
		log.Fatalf("seed: %v", err)
	}

	addr := ":" + env("PORT", "8080")
	log.Printf("API posluša na %s", addr)
	srv := &http.Server{
		Addr:              addr,
		Handler:           s.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func (s *Server) ensureIndexes(ctx context.Context) error {
	if _, err := s.users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	if _, err := s.roles.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "name", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}
	_, err := s.entries.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "startedAt", Value: -1}}},
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "status", Value: 1}}},
		{
			Keys: bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("one_open_entry_per_user").
				SetPartialFilterExpression(bson.M{"open": true}),
		},
		{Keys: bson.D{{Key: "startedAt", Value: -1}}},
	})
	return err
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// auth
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))

	// pravice + role
	mux.HandleFunc("GET /api/permissions", s.requireAuth(s.handlePermissions))
	mux.HandleFunc("GET /api/roles", s.requireAnyPerm([]string{PermRolesManage, PermUsersManage}, s.handleListRoles))
	mux.HandleFunc("POST /api/roles", s.requirePerm(PermRolesManage, s.handleCreateRole))
	mux.HandleFunc("PUT /api/roles/{id}", s.requirePerm(PermRolesManage, s.handleUpdateRole))
	mux.HandleFunc("DELETE /api/roles/{id}", s.requirePerm(PermRolesManage, s.handleDeleteRole))

	// uporabniki
	mux.HandleFunc("GET /api/users", s.requireAnyPerm([]string{PermUsersManage, PermViewAll, PermEditAll}, s.handleListUsers))
	mux.HandleFunc("POST /api/users", s.requirePerm(PermUsersManage, s.handleCreateUser))
	mux.HandleFunc("PUT /api/users/{id}", s.requirePerm(PermUsersManage, s.handleUpdateUser))
	mux.HandleFunc("DELETE /api/users/{id}", s.requirePerm(PermUsersManage, s.handleDeleteUser))

	// štoparica
	mux.HandleFunc("GET /api/timer/current", s.requirePerm(PermTimerUse, s.handleTimerCurrent))
	mux.HandleFunc("POST /api/timer/start", s.requirePerm(PermTimerUse, s.handleTimerStart))
	mux.HandleFunc("POST /api/timer/pause", s.requirePerm(PermTimerUse, s.handleTimerPause))
	mux.HandleFunc("POST /api/timer/resume", s.requirePerm(PermTimerUse, s.handleTimerResume))
	mux.HandleFunc("POST /api/timer/stop", s.requirePerm(PermTimerUse, s.handleTimerStop))

	// vnosi
	mux.HandleFunc("GET /api/entries", s.requireAuth(s.handleListEntries))
	mux.HandleFunc("POST /api/entries", s.requireAuth(s.handleCreateEntry))
	mux.HandleFunc("PUT /api/entries/{id}", s.requireAuth(s.handleUpdateEntry))
	mux.HandleFunc("DELETE /api/entries/{id}", s.requireAuth(s.handleDeleteEntry))

	return mux
}
