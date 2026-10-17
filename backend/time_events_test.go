package main

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func tev(typ string, at time.Time) TimeEventRecord {
	return TimeEventRecord{ID: primitive.NewObjectID(), Type: typ, At: at}
}

func TestValidateUserTimeEvents(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)

	if err := validateUserTimeEvents(nil); err != nil {
		t.Fatalf("empty: %v", err)
	}
	if err := validateUserTimeEvents([]TimeEventRecord{tev(EventStart, t0), tev(EventStop, t1)}); err != nil {
		t.Fatalf("valid pair: %v", err)
	}
	if err := validateUserTimeEvents([]TimeEventRecord{tev(EventStart, t0), tev(EventStart, t1)}); err == nil {
		t.Fatal("expected consecutive start error")
	}
	if err := validateUserTimeEvents([]TimeEventRecord{tev(EventStop, t0)}); err == nil {
		t.Fatal("expected first must be start")
	}
	if err := validateUserTimeEvents([]TimeEventRecord{tev(EventStart, t1), tev(EventStop, t0)}); err == nil {
		t.Fatal("expected non-monotonic error")
	}
	same := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	if err := validateUserTimeEvents([]TimeEventRecord{tev(EventStart, same), tev(EventStop, same)}); err == nil {
		t.Fatal("expected equal timestamp error")
	}
}

func TestParseManualEventAt(t *testing.T) {
	start, err := parseManualEventAt("2026-10-17", "22:19:00", EventStart)
	if err != nil {
		t.Fatalf("start with seconds: %v", err)
	}
	stop, err := parseManualEventAt("2026-10-17", "22:19", EventStop)
	if err != nil {
		t.Fatalf("stop minute only: %v", err)
	}
	if !stop.After(start) {
		t.Fatalf("stop should be after start same minute: start=%v stop=%v", start, stop)
	}
}
