package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type ViewportAggregateRow struct {
	PlayerKey string
	PlayerName string
	RawValue string
}

func (s *Store) ListViewportAggregateRows(ctx context.Context, patternName string) ([]ViewportAggregateRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListViewportAggregateRows(ctx, patternName)
	if err != nil {
		return nil, err
	}
	out := make([]ViewportAggregateRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ViewportAggregateRow{
			PlayerKey:  row.PlayerKey,
			PlayerName: row.PlayerName,
			RawValue:   row.RawValue,
		})
	}
	return out, nil
}

type ViewportGameRow struct {
	PlayerID int64
	RawValue string
}

func (s *Store) ListViewportGameRows(ctx context.Context, replayID int64, patternName string) ([]ViewportGameRow, error) {
	sqlcRows, err := sqlcgen.New(s.replayScoped()).ListViewportGameRows(ctx, sqlcgen.ListViewportGameRowsParams{
		ReplayID:    replayID,
		PatternName: patternName,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ViewportGameRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		out = append(out, ViewportGameRow{
			PlayerID: row.PlayerID,
			RawValue: row.RawValue,
		})
	}
	return out, nil
}
