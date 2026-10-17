package main

import (
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	EventStart = "start"
	EventStop  = "stop"
)

type TimeEvent struct {
	Type string    `bson:"type" json:"type"`
	At   time.Time `bson:"at" json:"at"`
}

// validateUserTimeEvents preveri, da se start/stop izmenjujeta (po času).
func validateUserTimeEvents(events []TimeEventRecord) error {
	if len(events) == 0 {
		return nil
	}
	sorted := append([]TimeEventRecord(nil), events...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].At.Equal(sorted[j].At) {
			return sorted[i].ID.Hex() < sorted[j].ID.Hex()
		}
		return sorted[i].At.Before(sorted[j].At)
	})
	if sorted[0].Type != EventStart {
		return fmt.Errorf("prvi dogodek mora biti start")
	}
	for i := 1; i < len(sorted); i++ {
		prev, cur := sorted[i-1], sorted[i]
		if !cur.At.After(prev.At) {
			if cur.At.Equal(prev.At) {
				return fmt.Errorf("dva dogodka ne smeta imeti enakega časa")
			}
			return fmt.Errorf("dogodki morajo biti po času naraščajoče")
		}
		if cur.Type == prev.Type {
			if cur.Type == EventStart {
				return fmt.Errorf("dva zaporedna start dogodka nista dovoljena")
			}
			return fmt.Errorf("dva zaporedna stop dogodka nista dovoljena")
		}
	}
	return nil
}

// nextEventTypeAfter returns the type that must follow the last event in time order.
func nextEventTypeAfter(lastType string) string {
	if lastType == EventStart {
		return EventStop
	}
	return EventStart
}

func trialTimeEventRecord(rec TimeEventRecord) TimeEventRecord {
	if rec.ID.IsZero() {
		rec.ID = primitive.NewObjectID()
	}
	return rec
}

func WorkSecondsFromEvents(events []TimeEvent, now time.Time) int64 {
	if len(events) == 0 {
		return 0
	}
	var total time.Duration
	var lastStart *time.Time
	for _, ev := range events {
		switch ev.Type {
		case EventStart:
			t := ev.At
			lastStart = &t
		case EventStop:
			if lastStart != nil {
				if ev.At.After(*lastStart) {
					total += ev.At.Sub(*lastStart)
				}
				lastStart = nil
			}
		}
	}
	if lastStart != nil && now.After(*lastStart) {
		total += now.Sub(*lastStart)
	}
	if total < 0 {
		return 0
	}
	return int64(total.Seconds())
}

func (e *Entry) EnsureEvents() {
	if len(e.Events) > 0 {
		return
	}
	if e.StartedAt.IsZero() {
		e.Events = []TimeEvent{}
		return
	}
	events := []TimeEvent{{Type: EventStart, At: e.StartedAt.UTC()}}
	for _, p := range e.Pauses {
		events = append(events, TimeEvent{Type: EventStop, At: p.Start.UTC()})
		if p.End != nil {
			events = append(events, TimeEvent{Type: EventStart, At: p.End.UTC()})
		}
	}
	if e.EndedAt != nil {
		events = append(events, TimeEvent{Type: EventStop, At: e.EndedAt.UTC()})
	} else if e.Status == StatusPaused && len(e.Pauses) > 0 {
		// odprta pavza v starem modelu — zadnji dogodek je že stop ob začetku pavze
	} else if e.Status == StatusRunning || e.Open {
		// tekoči vnos brez konca — samo start (ali start po zadnji pavzi)
	}
	e.Events = events
}

func (e *Entry) SyncStatusFromEvents() {
	e.EnsureEvents()
	if len(e.Events) == 0 {
		return
	}
	if !e.Open {
		e.Status = StatusStopped
		return
	}
	last := e.Events[len(e.Events)-1]
	switch last.Type {
	case EventStart:
		e.Status = StatusRunning
	case EventStop:
		e.Status = StatusPaused
	}
}

func eventsFromManualEntry(start, end time.Time, pauses []Pause) ([]TimeEvent, error) {
	start, end = start.UTC(), end.UTC()
	events := []TimeEvent{{Type: EventStart, At: start}}
	for _, p := range pauses {
		ps := p.Start.UTC()
		if !ps.After(start) || ps.After(end) {
			return nil, fmt.Errorf("pavza mora biti znotraj vnosa")
		}
		events = append(events, TimeEvent{Type: EventStop, At: ps})
		if p.End == nil {
			return nil, fmt.Errorf("pavza mora imeti konec")
		}
		pe := p.End.UTC()
		if !pe.After(ps) || pe.After(end) {
			return nil, fmt.Errorf("konec pavze mora biti znotraj vnosa")
		}
		events = append(events, TimeEvent{Type: EventStart, At: pe})
	}
	events = append(events, TimeEvent{Type: EventStop, At: end})
	if err := validateEventSequence(events, false); err != nil {
		return nil, err
	}
	return events, nil
}

func validateEventSequence(events []TimeEvent, openSession bool) error {
	if len(events) == 0 {
		return fmt.Errorf("vsaj en dogodek je obvezen")
	}
	if events[0].Type != EventStart {
		return fmt.Errorf("vnos se mora začeti z dogodkom start")
	}
	for i, ev := range events {
		if ev.At.IsZero() {
			return fmt.Errorf("manjkajoč timestamp dogodka")
		}
		if i > 0 {
			if ev.Type == events[i-1].Type {
				return fmt.Errorf("dogodki start/stop se morajo izmenjevati")
			}
			if !ev.At.After(events[i-1].At) {
				return fmt.Errorf("dogodki morajo biti po času naraščajoče")
			}
		}
	}
	last := events[len(events)-1]
	if !openSession && last.Type != EventStop {
		return fmt.Errorf("zaključen vnos mora imeti zadnji dogodek stop")
	}
	return nil
}

func firstStartAt(events []TimeEvent) time.Time {
	for _, ev := range events {
		if ev.Type == EventStart {
			return ev.At
		}
	}
	return time.Time{}
}

func lastStopAt(events []TimeEvent) *time.Time {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Type == EventStop {
			t := events[i].At
			return &t
		}
	}
	return nil
}
