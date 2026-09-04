package db

import ()

type UnitCadenceReplayMetricRow struct {
	ReplayID        int64
	PlayerKey       string
	PlayerName      string
	FileName        string
	DurationSeconds int64
	WindowSeconds   int64
	UnitsProduced   int64
	GapCount        int64
	RatePerMinute   float64
	CVGap           float64
	Burstiness      float64
	Idle20Ratio     float64
	CadenceScore    float64
}

type UnitSliceCommandRow struct {
	PlayerID int64
	Second   int64
	UnitType string
}

type FirstUnitCommandRow struct {
	PlayerID   int64
	Second     int64
	ActionType string
	UnitType   *string
	UnitTypes  *string
}

type GameUnitCadenceRow struct {
	PlayerID      int64
	WindowSeconds int64
	UnitsProduced int64
	GapCount      int64
	RatePerMinute *float64
	CVGap         *float64
	Burstiness    *float64
	Idle20Ratio   *float64
	CadenceScore  *float64
}
