package db

type ViewportAggregateRow struct {
	PlayerKey  string
	PlayerName string
	RawValue   string
}

type ViewportGameRow struct {
	PlayerID int64
	RawValue string
}
