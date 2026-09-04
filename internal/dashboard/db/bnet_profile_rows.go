package db

import (
	"time"
)

type BnetProfileRow struct {
	Toon        string
	Gateway     int64
	Found       bool
	AuroraID    int64
	BattleTag   string
	CountryCode string
	Payload     string
	FetchedAt   time.Time
}

// BnetProfilePayloadRow is one cached profile payload.
type BnetProfilePayloadRow struct {
	Toon    string
	Gateway int64
	Payload string
}
