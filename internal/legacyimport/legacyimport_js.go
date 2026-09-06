//go:build js

package legacyimport

import (
	"context"
	"errors"
)

var ErrNoDatabase = errors.New("legacyimport: no legacy database")

type Settings struct {
	ReplayDir         string
	GameTypes         []string
	ExcludeShortGames bool
	ExcludeComputers  bool
	MapKinds          []string
	FeatureFlags      map[string]bool
}

type BnetProfile struct {
	Toon        string
	Gateway     int
	Found       bool
	AuroraID    int64
	BattleTag   string
	CountryCode string
	Payload     string
	FetchedAt   string
}

type BnetGameResult struct {
	AuroraID        int64
	GameID          string
	CreateTimeUnix  int64
	Toon            string
	Gateway         int
	Race            string
	Result          string
	APM             int
	DurationSeconds int
	MapName         string
	MatchGUID       string
}

type Result struct {
	Settings    *Settings
	Profiles    []BnetProfile
	GameResults []BnetGameResult
}

func Read(context.Context, string) (Result, error) {
	return Result{}, ErrNoDatabase
}
