package main

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var appLoc *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Ljubljana")
	if err != nil {
		loc = time.UTC
	}
	appLoc = loc
}

type TimeEventRecord struct {
	ID     primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID primitive.ObjectID `bson:"userId" json:"userId"`
	Type   string             `bson:"type" json:"type"`
	At     time.Time          `bson:"at" json:"at"`
}

type DayPause struct {
	ID      primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID  primitive.ObjectID `bson:"userId" json:"userId"`
	DayKey  string             `bson:"dayKey" json:"dayKey"`
	Minutes int                `bson:"minutes" json:"minutes"`
	Start   time.Time          `bson:"startAt,omitempty" json:"-"`
	End     time.Time          `bson:"endAt,omitempty" json:"-"`
	Note    string             `bson:"note,omitempty" json:"note,omitempty"`
}

type timeEventDTO struct {
	ID   string    `json:"id"`
	Type string    `json:"type"`
	At   time.Time `json:"at"`
}

type dayPauseDTO struct {
	ID      string `json:"id"`
	Minutes int    `json:"minutes"`
	Note    string `json:"note,omitempty"`
}

type timeDayDTO struct {
	DayKey      string         `json:"dayKey"`
	Events      []timeEventDTO `json:"events"`
	Pauses      []dayPauseDTO  `json:"pauses"`
	WorkSeconds int64          `json:"workSeconds"`
}

func dayKeyInLoc(t time.Time) string {
	lt := t.In(appLoc)
	return lt.Format("2006-01-02")
}

func dayBounds(dayKey string) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01-02", dayKey, appLoc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end := start.AddDate(0, 0, 1)
	return start.UTC(), end.UTC(), nil
}

func overlapSeconds(aStart, aEnd, bStart, bEnd time.Time) int64 {
	if !aEnd.After(aStart) {
		return 0
	}
	s := maxTime(aStart, bStart)
	e := minTime(aEnd, bEnd)
	if !e.After(s) {
		return 0
	}
	return int64(e.Sub(s).Seconds())
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func workIntervalsFromEvents(events []TimeEventRecord, now time.Time) [][2]time.Time {
	sort.Slice(events, func(i, j int) bool { return events[i].At.Before(events[j].At) })
	var out [][2]time.Time
	var lastStart *time.Time
	for _, ev := range events {
		switch ev.Type {
		case EventStart:
			t := ev.At
			lastStart = &t
		case EventStop:
			if lastStart != nil && ev.At.After(*lastStart) {
				out = append(out, [2]time.Time{*lastStart, ev.At})
				lastStart = nil
			}
		}
	}
	if lastStart != nil && now.After(*lastStart) {
		out = append(out, [2]time.Time{*lastStart, now})
	}
	return out
}

func dayWorkSeconds(dayKey string, intervals [][2]time.Time, pauses []DayPause, now time.Time) (int64, error) {
	dayStart, dayEnd, err := dayBounds(dayKey)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, iv := range intervals {
		total += overlapSeconds(iv[0], iv[1], dayStart, dayEnd)
	}
	for _, p := range pauses {
		if p.DayKey != dayKey {
			continue
		}
		if p.Minutes > 0 {
			total -= int64(p.Minutes) * 60
		} else if p.End.After(p.Start) {
			total -= overlapSeconds(p.Start, p.End, dayStart, dayEnd)
		}
	}
	if total < 0 {
		total = 0
	}
	return total, nil
}

func (s *Server) fetchUserTimeEvents(ctx context.Context, userID primitive.ObjectID) ([]TimeEventRecord, error) {
	cur, err := s.timeEvents.Find(ctx, bson.M{"userId": userID}, options.Find().SetSort(bson.D{
		{Key: "at", Value: 1},
		{Key: "_id", Value: 1},
	}))
	if err != nil {
		return nil, err
	}
	var events []TimeEventRecord
	if err := cur.All(ctx, &events); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Server) validateNewTimeEvent(ctx context.Context, userID primitive.ObjectID, rec TimeEventRecord) error {
	existing, err := s.fetchUserTimeEvents(ctx, userID)
	if err != nil {
		return err
	}
	trial := append(existing, rec)
	return validateUserTimeEvents(trial)
}

func (s *Server) timerRunning(ctx context.Context, userID primitive.ObjectID) (bool, *TimeEventRecord, error) {
	var last TimeEventRecord
	err := s.timeEvents.FindOne(ctx, bson.M{"userId": userID}, options.FindOne().SetSort(bson.D{{Key: "at", Value: -1}, {Key: "_id", Value: -1}})).Decode(&last)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	if last.Type == EventStart {
		return true, &last, nil
	}
	return false, &last, nil
}

func (s *Server) handleTimerCurrent(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	now := time.Now().UTC()
	running, last, err := s.timerRunning(r.Context(), a.User.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var elapsed int64
	if running && last != nil {
		elapsed = int64(now.Sub(last.At).Seconds())
		if elapsed < 0 {
			elapsed = 0
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"running":       running,
		"runningSince":  nullableTime(running, last),
		"sessionSeconds": elapsed,
		"now":           now,
	})
}

func nullableTime(running bool, last *TimeEventRecord) any {
	if running && last != nil {
		return last.At
	}
	return nil
}

func (s *Server) handleTimerStart(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	running, _, err := s.timerRunning(r.Context(), a.User.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	if running {
		writeErr(w, http.StatusConflict, "časovnik že teče — najprej ustavite")
		return
	}
	now := time.Now().UTC()
	rec := TimeEventRecord{UserID: a.User.ID, Type: EventStart, At: now}
	if err := s.validateNewTimeEvent(r.Context(), a.User.ID, rec); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.timeEvents.InsertOne(r.Context(), rec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	rec.ID = res.InsertedID.(primitive.ObjectID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"event": map[string]any{
			"id":   rec.ID.Hex(),
			"type": rec.Type,
			"at":   rec.At,
		},
		"running":      true,
		"runningSince": rec.At,
		"now":          now,
	})
}

func (s *Server) handleTimerStop(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	running, _, err := s.timerRunning(r.Context(), a.User.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	if !running {
		writeErr(w, http.StatusConflict, "časovnik ne teče")
		return
	}
	now := time.Now().UTC()
	rec := TimeEventRecord{UserID: a.User.ID, Type: EventStop, At: now}
	if err := s.validateNewTimeEvent(r.Context(), a.User.ID, rec); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.timeEvents.InsertOne(r.Context(), rec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	rec.ID = res.InsertedID.(primitive.ObjectID)
	writeJSON(w, http.StatusOK, map[string]any{
		"event": map[string]any{
			"id":   rec.ID.Hex(),
			"type": rec.Type,
			"at":   rec.At,
		},
		"running": false,
		"now":     now,
	})
}

func (s *Server) resolveTimeUserID(a *Auth, qUser string) (primitive.ObjectID, error) {
	if a.Has(PermViewAll) && qUser != "" {
		return primitive.ObjectIDFromHex(qUser)
	}
	return a.User.ID, nil
}

// GET /api/time/days?from=YYYY-MM-DD&to=YYYY-MM-DD&userId=
func (s *Server) handleTimeDays(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	q := r.URL.Query()
	userID, err := s.resolveTimeUserID(a, q.Get("userId"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljaven userId")
		return
	}
	fromKey := strings.TrimSpace(q.Get("from"))
	toKey := strings.TrimSpace(q.Get("to"))
	if fromKey == "" || toKey == "" {
		writeErr(w, http.StatusBadRequest, "from in to sta obvezna (YYYY-MM-DD)")
		return
	}
	rangeStart, _, err := dayBounds(fromKey)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljaven from")
		return
	}
	_, rangeEnd, err := dayBounds(toKey)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljaven to")
		return
	}
	if !rangeEnd.After(rangeStart) {
		writeErr(w, http.StatusBadRequest, "to mora biti enak ali po from")
		return
	}

	ctx := r.Context()
	cur, err := s.timeEvents.Find(ctx, bson.M{
		"userId": userID,
		"at":     bson.M{"$gte": rangeStart, "$lt": rangeEnd},
	}, options.Find().SetSort(bson.D{{Key: "at", Value: 1}}))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var allEvents []TimeEventRecord
	if err := cur.All(ctx, &allEvents); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}

	// Za pare start/stop upoštevamo celoten potek (tudi čez polnoc).
	extStart := rangeStart.AddDate(0, 0, -1)
	extEnd := rangeEnd.AddDate(0, 0, 1)
	cur2, err := s.timeEvents.Find(ctx, bson.M{
		"userId": userID,
		"at":     bson.M{"$gte": extStart, "$lt": extEnd},
	}, options.Find().SetSort(bson.D{{Key: "at", Value: 1}}))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var stream []TimeEventRecord
	if err := cur2.All(ctx, &stream); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}

	pCur, err := s.dayPauses.Find(ctx, bson.M{
		"userId": userID,
		"dayKey": bson.M{"$gte": fromKey, "$lte": toKey},
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	var pauses []DayPause
	if err := pCur.All(ctx, &pauses); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}

	now := time.Now().UTC()
	intervals := workIntervalsFromEvents(stream, now)

	dayKeys := listDayKeys(fromKey, toKey)
	days := make([]timeDayDTO, 0, len(dayKeys))
	var rangeTotal int64
	pauseByDay := map[string][]DayPause{}
	for _, p := range pauses {
		pauseByDay[p.DayKey] = append(pauseByDay[p.DayKey], p)
	}
	eventsByDay := map[string][]timeEventDTO{}
	for _, ev := range allEvents {
		k := dayKeyInLoc(ev.At)
		eventsByDay[k] = append(eventsByDay[k], timeEventDTO{ID: ev.ID.Hex(), Type: ev.Type, At: ev.At})
	}

	for _, dk := range dayKeys {
		ws, err := dayWorkSeconds(dk, intervals, pauses, now)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "napaka dneva")
			return
		}
		rangeTotal += ws
		pDTO := make([]dayPauseDTO, 0, len(pauseByDay[dk]))
		for _, p := range pauseByDay[dk] {
			mins := p.Minutes
			if mins <= 0 && p.End.After(p.Start) {
				mins = int(p.End.Sub(p.Start).Minutes())
				if mins < 1 {
					mins = 1
				}
			}
			pDTO = append(pDTO, dayPauseDTO{ID: p.ID.Hex(), Minutes: mins, Note: p.Note})
		}
		evs := eventsByDay[dk]
		if evs == nil {
			evs = []timeEventDTO{}
		}
		if len(evs) == 0 && len(pDTO) == 0 && ws == 0 {
			continue
		}
		days = append(days, timeDayDTO{
			DayKey:      dk,
			Events:      evs,
			Pauses:      pDTO,
			WorkSeconds: ws,
		})
	}

	sort.Slice(days, func(i, j int) bool { return days[i].DayKey > days[j].DayKey })

	writeJSON(w, http.StatusOK, map[string]any{
		"days":             days,
		"rangeWorkSeconds": rangeTotal,
	})
}

func listDayKeys(from, to string) []string {
	start, err := time.ParseInLocation("2006-01-02", from, appLoc)
	if err != nil {
		return nil
	}
	end, err := time.ParseInLocation("2006-01-02", to, appLoc)
	if err != nil {
		return nil
	}
	var keys []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		keys = append(keys, d.Format("2006-01-02"))
	}
	return keys
}

// POST /api/time/days/{dayKey}/pauses
func (s *Server) handleCreateDayPause(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	if !a.Has(PermTimerUse) && !a.Has(PermEditOwn) {
		writeErr(w, http.StatusForbidden, "nimate pravice za pavze")
		return
	}
	dayKey := strings.TrimSpace(r.PathValue("dayKey"))
	if _, _, err := dayBounds(dayKey); err != nil {
		writeErr(w, http.StatusBadRequest, "neveljaven dan")
		return
	}
	var req struct {
		Minutes int    `json:"minutes"`
		Note    string `json:"note"`
		UserID  string `json:"userId"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	target := a.User.ID
	if req.UserID != "" {
		if !a.Has(PermEditAll) && !a.Has(PermViewAll) {
			writeErr(w, http.StatusForbidden, "nimate pravice")
			return
		}
		id, err := primitive.ObjectIDFromHex(req.UserID)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "neveljaven userId")
			return
		}
		target = id
	}
	if req.Minutes < 1 || req.Minutes > 24*60 {
		writeErr(w, http.StatusBadRequest, "vnesite trajanje pavze med 1 in 1440 minut")
		return
	}
	if len(strings.TrimSpace(req.Note)) > 200 {
		writeErr(w, http.StatusBadRequest, "opomba je predolga")
		return
	}
	p := DayPause{
		UserID:  target,
		DayKey:  dayKey,
		Minutes: req.Minutes,
		Note:    strings.TrimSpace(req.Note),
	}
	res, err := s.dayPauses.InsertOne(r.Context(), p)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": res.InsertedID.(primitive.ObjectID).Hex()})
}

func (s *Server) resolveManualTimeUser(a *Auth, userIDHex string) (primitive.ObjectID, int, string) {
	target := a.User.ID
	if userIDHex != "" {
		id, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			return target, http.StatusBadRequest, "neveljaven userId"
		}
		if id != a.User.ID {
			if !a.Has(PermEditAll) {
				return target, http.StatusForbidden, "nimate pravice za dogodke drugih uporabnikov"
			}
			if n, err := s.users.CountDocuments(context.Background(), bson.M{"_id": id}); err != nil || n == 0 {
				return target, http.StatusBadRequest, "uporabnik ne obstaja"
			}
		}
		target = id
	}
	if target != a.User.ID && !a.Has(PermEditAll) {
		return target, http.StatusForbidden, "nimate pravice"
	}
	if target == a.User.ID && !a.Has(PermTimerUse) && !a.Has(PermEditOwn) && !a.Has(PermEditAll) {
		return target, http.StatusForbidden, "nimate pravice za ročne dogodke"
	}
	return target, 0, ""
}

// POST /api/time/events — ročni dogodek start/stop
func (s *Server) handleCreateTimeEvent(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	var req struct {
		UserID string `json:"userId"`
		Type   string `json:"type"`
		Date   string `json:"date"`
		Time   string `json:"time"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	target, status, msg := s.resolveManualTimeUser(a, strings.TrimSpace(req.UserID))
	if status != 0 {
		writeErr(w, status, msg)
		return
	}
	switch req.Type {
	case EventStart, EventStop:
	default:
		writeErr(w, http.StatusBadRequest, "tip mora biti start ali stop")
		return
	}
	date := strings.TrimSpace(req.Date)
	timePart := strings.TrimSpace(req.Time)
	if date == "" || timePart == "" {
		writeErr(w, http.StatusBadRequest, "datum in čas sta obvezna")
		return
	}
	at, err := time.ParseInLocation("2006-01-02T15:04", date+"T"+timePart, appLoc)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "neveljaven datum ali čas")
		return
	}
	at = at.UTC()
	if at.After(time.Now().UTC().Add(time.Minute)) {
		writeErr(w, http.StatusBadRequest, "dogodek ne sme biti v prihodnosti")
		return
	}
	rec := TimeEventRecord{UserID: target, Type: req.Type, At: at}
	if err := s.validateNewTimeEvent(r.Context(), target, rec); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := s.timeEvents.InsertOne(r.Context(), rec)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":   res.InsertedID.(primitive.ObjectID).Hex(),
		"type": rec.Type,
		"at":   rec.At,
	})
}

// DELETE /api/time/events/{id}
func (s *Server) handleDeleteTimeEvent(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var ev TimeEventRecord
	if err := s.timeEvents.FindOne(r.Context(), bson.M{"_id": id}).Decode(&ev); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeErr(w, http.StatusNotFound, "dogodek ni najden")
		} else {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
		}
		return
	}
	if ev.UserID != a.User.ID && !a.Has(PermEditAll) {
		writeErr(w, http.StatusForbidden, "nimate pravice")
		return
	}
	if ev.UserID == a.User.ID && !a.Has(PermTimerUse) && !a.Has(PermEditOwn) && !a.Has(PermEditAll) {
		writeErr(w, http.StatusForbidden, "nimate pravice")
		return
	}
	if _, err := s.timeEvents.DeleteOne(r.Context(), bson.M{"_id": id}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DELETE /api/time/pauses/{id}
func (s *Server) handleDeleteDayPause(w http.ResponseWriter, r *http.Request) {
	a := authFrom(r)
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var p DayPause
	if err := s.dayPauses.FindOne(r.Context(), bson.M{"_id": id}).Decode(&p); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			writeErr(w, http.StatusNotFound, "pavza ni najdena")
		} else {
			writeErr(w, http.StatusInternalServerError, "napaka baze")
		}
		return
	}
	if p.UserID != a.User.ID && !a.Has(PermEditAll) {
		writeErr(w, http.StatusForbidden, "nimate pravice")
		return
	}
	if _, err := s.dayPauses.DeleteOne(r.Context(), bson.M{"_id": id}); err != nil {
		writeErr(w, http.StatusInternalServerError, "napaka baze")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
