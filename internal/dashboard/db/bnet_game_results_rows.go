package db

import (
	"time"
)

// BnetGameResultRow is one cached game_results entry of a Battle.net account.
type BnetGameResultRow struct {
	AuroraID        int64
	GameID          string
	CreateTime      time.Time
	Toon            string
	Gateway         int
	Race            string
	Result          string
	APM             int
	DurationSeconds int
	MapName         string
	MatchGUID       string
}
