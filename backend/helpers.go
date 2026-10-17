package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "neveljavno telo zahtevka")
		return false
	}
	return true
}

func parseID(w http.ResponseWriter, r *http.Request) (primitive.ObjectID, bool) {
	id, err := primitive.ObjectIDFromHex(r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljaven id")
		return id, false
	}
	return id, true
}

func intParam(r *http.Request, key string, def, min, max int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return def
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
