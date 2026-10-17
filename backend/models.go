package main

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Katalog pravic. Ključ se shranjuje v roli.
const (
	PermTimerUse    = "timer.use"        // lahko uporablja start/pavza/stop
	PermViewAll     = "entries.view_all" // vidi vnose vseh uporabnikov
	PermEditOwn     = "entries.edit_own" // ureja/dodaja/briše svoje vnose
	PermEditAll     = "entries.edit_all" // ureja/dodaja/briše vnose vseh
	PermUsersManage = "users.manage"     // CRUD uporabnikov
	PermRolesManage = "roles.manage"     // CRUD rolov
)

type PermissionInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

var PermissionCatalog = []PermissionInfo{
	{PermTimerUse, "Beleženje časa (start / pavza / stop)"},
	{PermViewAll, "Ogled vnosov vseh uporabnikov"},
	{PermEditOwn, "Urejanje, dodajanje in brisanje lastnih vnosov"},
	{PermEditAll, "Urejanje, dodajanje in brisanje vnosov vseh uporabnikov"},
	{PermUsersManage, "Upravljanje uporabnikov"},
	{PermRolesManage, "Upravljanje rol in pravic"},
}

func validPermission(p string) bool {
	for _, x := range PermissionCatalog {
		if x.Key == p {
			return true
		}
	}
	return false
}

func allPermissionKeys() []string {
	keys := make([]string, 0, len(PermissionCatalog))
	for _, p := range PermissionCatalog {
		keys = append(keys, p.Key)
	}
	return keys
}

type Role struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	Permissions []string           `bson:"permissions" json:"permissions"`
	System      bool               `bson:"system" json:"system"`
}

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Username     string             `bson:"username" json:"username"`
	Name         string             `bson:"name" json:"name"`
	PasswordHash string             `bson:"passwordHash" json:"-"`
	RoleID       primitive.ObjectID `bson:"roleId" json:"roleId"`
	Active       bool               `bson:"active" json:"active"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
}

type Pause struct {
	Start time.Time  `bson:"start" json:"start"`
	End   *time.Time `bson:"end,omitempty" json:"end"`
}

const (
	StatusRunning = "running"
	StatusPaused  = "paused"
	StatusStopped = "stopped"
)

type Entry struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      primitive.ObjectID `bson:"userId" json:"userId"`
	Description string             `bson:"description" json:"description"`
	StartedAt   time.Time          `bson:"startedAt" json:"startedAt"`
	EndedAt     *time.Time         `bson:"endedAt,omitempty" json:"endedAt"`
	Pauses      []Pause            `bson:"pauses" json:"pauses"`
	Status      string             `bson:"status" json:"status"`
	// Open je true, dokler vnos teče ali je pavziran; unikatni delni indeks
	// zagotavlja največ en odprt vnos na uporabnika (tudi ob dvojnem kliku).
	Open      bool      `bson:"open" json:"-"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

// WorkSeconds vrne čisti delovni čas (brez pavz). Za tekoče vnose šteje do "now".
func (e *Entry) WorkSeconds(now time.Time) int64 {
	end := now
	if e.EndedAt != nil {
		end = *e.EndedAt
	}
	total := end.Sub(e.StartedAt)
	for _, p := range e.Pauses {
		pe := end
		if p.End != nil {
			pe = *p.End
		}
		if pe.After(end) {
			pe = end
		}
		if pe.After(p.Start) {
			total -= pe.Sub(p.Start)
		}
	}
	if total < 0 {
		return 0
	}
	return int64(total.Seconds())
}
