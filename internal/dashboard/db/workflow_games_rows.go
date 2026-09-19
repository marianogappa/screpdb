package db

import (
	"github.com/marianogappa/screpdb/internal/models"
)

type WorkflowGameListRow struct {
	ReplayID           int64
	ReplayDate         string
	FileName           string
	MapName            string
	MapKind            string
	GameSource         string
	LobbyKind          string
	DurationSeconds    int64
	GameType           string
	Matchup            string
	TeamStacking       bool
	TeamInfoIncomplete bool
}

type WorkflowGamePlayerRow struct {
	ReplayID int64
	PlayerID int64
	Name     string
	Race     string
	Team     int64
	// TeamWon is the match fact: did this player's side take the game. The
	// roster is styled from this, never from the personal result.
	TeamWon bool
	// PlayerOutcome is how this player personally fared, which TeamWon cannot
	// express: a replay often cannot tell a loss from a game it never resolved.
	PlayerOutcome models.Outcome
	// Dropped is its own outcome, neither a loss nor an unknown.
	Dropped bool
	// Why the results are what they are, for the debug overlay.
	PlayerOutcomeReason string
	TeamOutcomeReason   string
	BnetOutcomeSource   string
	// CompleteCopy marks a co-player recording downloaded through Battle.net
	// because the user's own copy ended before the game did (issue #341).
	CompleteCopy bool
}

type WorkflowPlayerPatternRow struct {
	ReplayID       int64
	PatternName    string
	ValueBool      *bool
	ValueInt       *int64
	ValueString    *string
	ValueTimestamp *int64
	DetectedSecond int64
}

type WorkflowReplayEventRow struct {
	ReplayID  int64
	EventType string
}

type WorkflowCurrentPlayerRow struct {
	ReplayID int64
	PlayerID int64
	Name     string
	Race     string
	TeamWon bool
	APM      int64
	EAPM     int64
}

type WorkflowCurrentPlayerPatternRow struct {
	PlayerID       int64
	PatternName    string
	PatternValue   string
	DetectedSecond int64
	Payload        string
}

type WorkflowFilterOptionRow struct {
	Key   string
	Label string
	Games int64
}

type WorkflowPlayersListRow struct {
	PlayerKey         string
	PlayerName        string
	Race              string
	GamesPlayed       int64
	AverageAPM        float64
	LastPlayed        string
	LastPlayedDaysAgo int64
}
