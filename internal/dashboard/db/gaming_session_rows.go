package db

// SessionCandidateRow is one of the user's games, with just enough to decide
// whether it belongs to the current session.
type SessionCandidateRow struct {
	ReplayID   int64
	ReplayDate string
	FilePath   string
	PlayerKey  string
	PlayerName string
}

// SessionReplayRow is the game-level half of a session game.
type SessionReplayRow struct {
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

// SessionPlayerAPMRow carries the per-game APM the session summary averages.
type SessionPlayerAPMRow struct {
	ReplayID  int64
	PlayerKey string
	APM       int64
	EAPM      int64
}
