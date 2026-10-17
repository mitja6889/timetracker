package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const maxDescription = 200

type entryDTO struct {
	Entry
	Username    string `json:"username"`
	Name        string `json:"name"`
	WorkSeconds int64  `json:"workSeconds"`
	CanEdit     bool   `json:"canEdit"`
	CanDelete   bool   `json:"canDelete"`
}

// canModify: ali ima prijavljeni uporabnik pravico spreminjati ta vnos (glede na rolo).
func canModify(a *Auth, e *Entry) bool {
	if a.Has(PermEditAll) {
		return true
	}
	return a.Has(PermEditOwn) && e.UserID == a.User.ID
}

func (s *Server) toDTO(a *Auth, e Entry, names map[primitive.ObjectID][2]string, now time.Time) entryDTO {
	if e.Pauses == nil {
		e.Pauses = []Pause{}
	}
	mod := canModify(a, &e)
	n := names[e.UserID]
	return entryDTO{
		Entry:       e,
		Username:    n[0],
		Name:        n[1],
		WorkSeconds: e.WorkSeconds(now),
		CanEdit:     mod && e.Status == StatusStopped,
		CanDelete:   mod,
	}
}

func (s *Server) userNames(r *http.Request, ids []primitive.ObjectID) map[primitive.ObjectID][2]string {
	out := map[primitive.ObjectID][2]string{}
	if len(ids) == 0 {
		return out
	}
	cur, err := s.users.Find(r.Context(), bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return out
	}
	var users []User
	if cur.All(r.Context(), &users) != nil {
		return out
	}
	for _, u := range users {
		out[u.ID] = [2]string{u.Username, u.Name}
	}
	return out
}

// ---------- štoparica (start / pavza / nadaljuj / stop) ----------

func (s *Server) handleTimerCurrent(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	now := time.Now().UTC()
	var e Entry
	err := s.entries.FindOne(r.Context(), bson.M{
		"userId": a.User.ID,
		"status": bson.M{"$in": []string{StatusRunning, StatusPaused}},
	}).Decode(&e)
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeJSON(w, http.StatusOK, map[string]any{"entry": nil, "now": now})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	names := map[primitive.ObjectID][2]string{a.User.ID: {a.User.Username, a.User.Name}}
	writeJSON(w, http.StatusOK, map[string]any{"entry": s.toDTO(a, e, names, now), "now": now})
}

func (s *Server) timerResponse(w http.ResponseWriter, a *Auth, e Entry) {
	now := time.Now().UTC()
	names := map[primitive.ObjectID][2]string{a.User.ID: {a.User.Username, a.User.Name}}
	writeJSON(w, http.StatusOK, map[string]any{"entry": s.toDTO(a, e, names, now), "now": now})
}

func (s *Server) handleTimerStart(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	var req struct {
		Description string `json:"description"`
	}
	// telo je neobvezno
	if r.ContentLength != 0 && !readJSON(w, r, &req) {
		return
	}
	desc := strings.TrimSpace(req.Description)
	if len(desc) > maxDescription {
		writeErr(w, http.StatusBadRequest, "opis je predolg")
		return
	}
	now := time.Now().UTC()
	e := Entry{
		UserID:      a.User.ID,
		Description: desc,
		StartedAt:   now,
		Pauses:      []Pause{},
		Status:      StatusRunning,
		Open:        true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	res, err := s.entries.InsertOne(r.Context(), e)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			writeErr(w, http.StatusConflict, "časovnik že teče - najprej ga ustavite")
			return
		}
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	e.ID = res.InsertedID.(primitive.ObjectID)
	s.timerResponse(w, a, e)
}

func (s *Server) handleTimerPause(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	now := time.Now().UTC()
	var e Entry
	err := s.entries.FindOneAndUpdate(r.Context(),
		bson.M{"userId": a.User.ID, "status": StatusRunning},
		bson.M{
			"$set":  bson.M{"status": StatusPaused, "updatedAt": now},
			"$push": bson.M{"pauses": Pause{Start: now}},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&e)
	s.finishTimerOp(w, a, e, err, "ni tekočega vnosa, ki bi ga lahko pavzirali")
}

func (s *Server) handleTimerResume(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	now := time.Now().UTC()
	var e Entry
	err := s.entries.FindOneAndUpdate(r.Context(),
		bson.M{"userId": a.User.ID, "status": StatusPaused},
		bson.M{"$set": bson.M{
			"status":          StatusRunning,
			"updatedAt":       now,
			"pauses.$[p].end": now,
		}},
		options.FindOneAndUpdate().
			SetReturnDocument(options.After).
			SetArrayFilters(options.ArrayFilters{Filters: []any{bson.M{"p.end": bson.M{"$exists": false}}}}),
	).Decode(&e)
	s.finishTimerOp(w, a, e, err, "ni pavziranega vnosa, ki bi ga lahko nadaljevali")
}

func (s *Server) handleTimerStop(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	now := time.Now().UTC()
	var e Entry
	err := s.entries.FindOneAndUpdate(r.Context(),
		bson.M{"userId": a.User.ID, "status": bson.M{"$in": []string{StatusRunning, StatusPaused}}},
		bson.M{"$set": bson.M{
			"status":          StatusStopped,
			"open":            false,
			"endedAt":         now,
			"updatedAt":       now,
			"pauses.$[p].end": now, // zapre morebitno odprto pavzo
		}},
		options.FindOneAndUpdate().
			SetReturnDocument(options.After).
			SetArrayFilters(options.ArrayFilters{Filters: []any{bson.M{"p.end": bson.M{"$exists": false}}}}),
	).Decode(&e)
	s.finishTimerOp(w, a, e, err, "ni tekočega vnosa")
}

func (s *Server) finishTimerOp(w http.ResponseWriter, a *Auth, e Entry, err error, conflictMsg string) {
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeErr(w, http.StatusConflict, conflictMsg)
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	s.timerResponse(w, a, e)
}

// ---------- seznam z filtriranjem na strežniku ----------

var sortFields = map[string]string{
	"startedAt":   "startedAt",
	"endedAt":     "endedAt",
	"description": "description",
	"status":      "status",
	"createdAt":   "createdAt",
}

func parseTimeParam(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

func (s *Server) entriesFilter(a *Auth, q url.Values) (bson.M, error) {
	filter := bson.M{}

	if a.Has(PermViewAll) {
		if uid := q.Get("userId"); uid != "" {
			id, err := primitive.ObjectIDFromHex(uid)
			if err != nil {
				return nil, errBadRequest("neveljaven userId")
			}
			filter["userId"] = id
		}
	} else {
		filter["userId"] = a.User.ID
	}

	switch st := q.Get("status"); st {
	case "":
	case StatusRunning, StatusPaused, StatusStopped:
		filter["status"] = st
	default:
		return nil, errBadRequest("neveljaven status")
	}

	rng := bson.M{}
	if t, ok := parseTimeParam(q.Get("from")); ok {
		rng["$gte"] = t
	}
	if t, ok := parseTimeParam(q.Get("to")); ok {
		rng["$lt"] = t
	}
	if len(rng) > 0 {
		filter["startedAt"] = rng
	}

	if text := strings.TrimSpace(q.Get("q")); text != "" {
		if len(text) > 100 {
			text = text[:100]
		}
		filter["description"] = bson.M{"$regex": regexp.QuoteMeta(text), "$options": "i"}
	}
	return filter, nil
}

func (s *Server) sumFilteredWorkSeconds(ctx context.Context, filter bson.M, now time.Time) (int64, error) {
	cur, err := s.entries.Find(ctx, filter)
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	var sum int64
	for cur.Next(ctx) {
		var e Entry
		if err := cur.Decode(&e); err != nil {
			return 0, err
		}
		sum += e.WorkSeconds(now)
	}
	return sum, cur.Err()
}

type httpError struct {
	status  int
	message string
}

func (e httpError) Error() string { return e.message }

func errBadRequest(msg string) error { return httpError{status: http.StatusBadRequest, message: msg} }

// GET /api/entries?page=&pageSize=&sort=&order=&from=&to=&userId=&status=&q=
// "from" je vključen, "to" izključen (glede na startedAt).
// Uporabnik brez pravice entries.view_all vidi VEDNO samo svoje vnose,
// ne glede na parameter userId.
func (s *Server) handleListEntries(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	q := r.URL.Query()
	filter, err := s.entriesFilter(a, q)
	if err != nil {
		if he, ok := err.(httpError); ok {
			writeErr(w, he.status, he.message)
			return
		}
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}

	field, ok := sortFields[q.Get("sort")]
	if !ok {
		field = "startedAt"
	}
	dir := -1
	if q.Get("order") == "asc" {
		dir = 1
	}

	page := intParam(r, "page", 1, 1, 1_000_000)
	pageSize := intParam(r, "pageSize", 10, 1, 100)

	total, err := s.entries.CountDocuments(r.Context(), filter)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	cur, err := s.entries.Find(r.Context(), filter, options.Find().
		SetSort(bson.D{{Key: field, Value: dir}, {Key: "_id", Value: dir}}).
		SetSkip(int64((page-1)*pageSize)).
		SetLimit(int64(pageSize)))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var entries []Entry
	if err := cur.All(r.Context(), &entries); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}

	idSet := map[primitive.ObjectID]bool{}
	for _, e := range entries {
		idSet[e.UserID] = true
	}
	ids := make([]primitive.ObjectID, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	names := s.userNames(r, ids)

	now := time.Now().UTC()
	items := make([]entryDTO, 0, len(entries))
	for _, e := range entries {
		items = append(items, s.toDTO(a, e, names, now))
	}

	var rangeWorkSeconds int64
	if q.Get("from") != "" || q.Get("to") != "" {
		rangeWorkSeconds, err = s.sumFilteredWorkSeconds(r.Context(), filter, now)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":            items,
		"total":            total,
		"page":             page,
		"pageSize":         pageSize,
		"rangeWorkSeconds": rangeWorkSeconds,
	})
}

// ---------- ročno dodajanje / urejanje / brisanje ----------

type entryReq struct {
	UserID      string    `json:"userId"`
	Description string    `json:"description"`
	StartedAt   time.Time `json:"startedAt"`
	EndedAt     time.Time `json:"endedAt"`
	Pauses      []struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"pauses"`
}

// buildEntryFields preveri časovnice in vrne očiščene podatke.
func buildEntryFields(req *entryReq) (string, []Pause, string) {
	desc := strings.TrimSpace(req.Description)
	if len(desc) > maxDescription {
		return "", nil, "opis je predolg"
	}
	if req.StartedAt.IsZero() || req.EndedAt.IsZero() {
		return "", nil, "začetek in konec sta obvezna"
	}
	start, end := req.StartedAt.UTC(), req.EndedAt.UTC()
	if !end.After(start) {
		return "", nil, "konec mora biti po začetku"
	}
	if end.After(time.Now().UTC().Add(time.Minute)) {
		return "", nil, "konec ne sme biti v prihodnosti"
	}
	pauses := make([]Pause, 0, len(req.Pauses))
	for _, p := range req.Pauses {
		ps, pe := p.Start.UTC(), p.End.UTC()
		if !pe.After(ps) {
			return "", nil, "konec pavze mora biti po njenem začetku"
		}
		if ps.Before(start) || pe.After(end) {
			return "", nil, "pavza mora biti znotraj vnosa"
		}
		pe2 := pe
		pauses = append(pauses, Pause{Start: ps, End: &pe2})
	}
	sort.Slice(pauses, func(i, j int) bool { return pauses[i].Start.Before(pauses[j].Start) })
	for i := 1; i < len(pauses); i++ {
		if pauses[i].Start.Before(*pauses[i-1].End) {
			return "", nil, "pavze se prekrivajo"
		}
	}
	return desc, pauses, ""
}

func (s *Server) handleCreateEntry(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	var req entryReq
	if !readJSON(w, r, &req) {
		return
	}
	target := a.User.ID
	if req.UserID != "" {
		id, err := primitive.ObjectIDFromHex(req.UserID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "neveljaven userId")
			return
		}
		target = id
	}
	probe := Entry{UserID: target}
	if !canModify(a, &probe) {
		writeErr(w, http.StatusForbidden, "nimate pravice za dodajanje vnosov")
		return
	}
	if target != a.User.ID {
		if n, err := s.users.CountDocuments(r.Context(), bson.M{"_id": target}); err != nil || n == 0 {
			writeErr(w, http.StatusBadRequest, "uporabnik ne obstaja")
			return
		}
	}
	desc, pauses, msg := buildEntryFields(&req)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	now := time.Now().UTC()
	end := req.EndedAt.UTC()
	e := Entry{
		UserID:      target,
		Description: desc,
		StartedAt:   req.StartedAt.UTC(),
		EndedAt:     &end,
		Pauses:      pauses,
		Status:      StatusStopped,
		Open:        false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	res, err := s.entries.InsertOne(r.Context(), e)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": res.InsertedID.(primitive.ObjectID).Hex()})
}

func (s *Server) loadEntry(w http.ResponseWriter, r *http.Request) (*Entry, bool) {
	id, ok := parseID(w, r)
	if !ok {
		return nil, false
	}
	var e Entry
	if err := s.entries.FindOne(r.Context(), bson.M{"_id": id}).Decode(&e); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeErr(w, http.StatusNotFound, "vnos ni najden")
		} else {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
		}
		return nil, false
	}
	return &e, true
}

func (s *Server) handleUpdateEntry(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	// uporabnik brez pravice vsaj za ogled ne sme izvedeti, da vnos obstaja
	if !canModify(a, e) {
		if e.UserID != a.User.ID && !a.Has(PermViewAll) {
			writeErr(w, http.StatusNotFound, "vnos ni najden")
		} else {
			writeErr(w, http.StatusForbidden, "tega vnosa ne smete urejati")
		}
		return
	}
	if e.Status != StatusStopped {
		writeErr(w, http.StatusConflict, "tekoč vnos najprej ustavite")
		return
	}
	var req entryReq
	if !readJSON(w, r, &req) {
		return
	}
	desc, pauses, msg := buildEntryFields(&req)
	if msg != "" {
		writeErr(w, http.StatusBadRequest, msg)
		return
	}
	end := req.EndedAt.UTC()
	_, err := s.entries.UpdateOne(r.Context(), bson.M{"_id": e.ID, "status": StatusStopped}, bson.M{"$set": bson.M{
		"description": desc,
		"startedAt":   req.StartedAt.UTC(),
		"endedAt":     end,
		"pauses":      pauses,
		"updatedAt":   time.Now().UTC(),
	}})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteEntry(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	e, ok := s.loadEntry(w, r)
	if !ok {
		return
	}
	if !canModify(a, e) {
		if e.UserID != a.User.ID && !a.Has(PermViewAll) {
			writeErr(w, http.StatusNotFound, "vnos ni najden")
		} else {
			writeErr(w, http.StatusForbidden, "tega vnosa ne smete izbrisati")
		}
		return
	}
	if _, err := s.entries.DeleteOne(r.Context(), bson.M{"_id": e.ID}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
