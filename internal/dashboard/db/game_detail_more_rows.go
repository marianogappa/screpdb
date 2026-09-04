package db

type RaceSectionRow struct {
	Race      string
	GameCount int64
	Wins      int64
}

type PlayerFirstExpansionTimingRow struct {
	Race                 string
	MapKind              string
	ReplayID             int64
	FirstExpansionSecond int64
}

// PhaseBoundaries carries the early/mid game-end seconds persisted as
// replay-level markers at ingest. Either or both may be 0 when the
// replay never reached the corresponding boundary (the game ended
// inside Early, or inside Mid). Same "0 = not detected" convention used
// elsewhere in the codebase.
type PhaseBoundaries struct {
	EarlyEndsAtSecond int64 // = mid_game_starts marker's second
	MidEndsAtSecond   int64 // = late_game_starts marker's second
}

// UnitProductionOrCastRow is one Train / Unit Morph / spell-cast
// command for the per-game composition computation. Caster rows have
// ActionType empty and OrderName populated; production rows have
// ActionType in (Train, Unit Morph) and a non-nil UnitType.
type UnitProductionOrCastRow struct {
	PlayerID             int64
	ActionType           string
	UnitType             *string
	UnitTypes            *string
	OrderName            *string
	SecondsFromGameStart int64
}

type PlayerMatchupRow struct {
	OwnRace string
	OppRace string
	Games   int64
	Wins    int64
}
