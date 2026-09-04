package db

import ()

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
	IsWinner bool
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
	IsWinner bool
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
