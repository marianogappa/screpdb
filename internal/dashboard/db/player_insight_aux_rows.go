package db

type RaceOrderRow struct {
	PlayerID    int64
	Race        string
	ActionType  string
	TechName    *string
	UpgradeName *string
	Second      int64
}

type MatchupOrderRow struct {
	PlayerID    int64
	OwnRace     string
	OppRace     string
	ReplayID    int64
	ActionType  string
	TechName    *string
	UpgradeName *string
	Second      int64
}

type PlayerChatRow struct {
	ReplayID int64
	Message  string
}

type TimingRow struct {
	PlayerID int64
	Second   int64
	Label    string
}
