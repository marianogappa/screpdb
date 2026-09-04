package db

type ReplaySummaryRow struct {
	ReplayID           int64
	ReplayDate         string
	FileName           string
	FilePath           string
	FileChecksum       string
	MapName            string
	MapKind            string
	GameSource         string
	LobbyKind          string
	DurationSeconds    int64
	GameType           string
	TeamStacking       bool
	TeamInfoIncomplete bool
}

type ReplayPlayerDetailRow struct {
	PlayerID            int64
	Name                string
	Color               string
	Race                string
	Team                int64
	IsWinner            bool
	StartLocationOclock *int64
	APM                 int64
	EAPM                int64
}

type PatternValueRow struct {
	PatternName    string
	Value          string
	DetectedSecond int64
	Payload        string
}

type PlayerPatternValueRow struct {
	PlayerID       int64
	PatternName    string
	Value          string
	DetectedSecond int64
	Payload        string
}

type ReplayEventRow struct {
	EventType              string
	Second                 int64
	SourcePlayerID         *int64
	SourcePlayerName       string
	SourcePlayerColor      string
	TargetPlayerID         *int64
	TargetPlayerName       string
	TargetPlayerColor      string
	LocationBaseType       *string
	LocationBaseOclock     *int64
	LocationNaturalOfClock *int64
	LocationMineralOnly    *bool
	AttackUnitTypes        *string
	AttackCastCounts       *string
	Payload                *string
}

type PlayerOverviewSummaryRow struct {
	PlayerName  string
	GamesPlayed int64
	Wins        int64
	AverageAPM  float64
	AverageEAPM float64
}

type PlayerRecentGameRow struct {
	ReplayID           int64
	ReplayDate         string
	FileName           string
	MapName            string
	MapKind            string
	GameSource         string
	LobbyKind          string
	DurationSeconds    int64
	GameType           string
	TeamFormat         string
	Matchup            string
	TeamStacking       bool
	TeamInfoIncomplete bool
	PlayersLabel       string
	WinnersLabel       string
}

type PlayerApmAggregateRow struct {
	PlayerKey   string
	PlayerName  string
	AverageAPM  float64
	GamesPlayed int64
}

type ReplayPlayerForAllianceRow struct {
	PlayerID   int64
	Name       string
	Race       string
	Type       string
	Team       int64
	IsObserver bool
	SlotID     int64
}

type ReplayAllianceCommandRow struct {
	PlayerID             int64
	SecondsFromGameStart int64
	AlliancePlayerIDs    string // JSON array of slot IDs
}
