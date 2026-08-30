package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type BnetProfileRow struct {
	Toon        string
	Gateway     int64
	Found       bool
	AuroraID    int64
	BattleTag   string
	CountryCode string
	Payload     string
	FetchedAt   time.Time
}

// GetBnetProfile returns the cached profile for (toon, gateway), or nil if
// none is cached yet.
func (s *Store) GetBnetProfile(ctx context.Context, toon string, gateway int64) (*BnetProfileRow, error) {
	row, err := sqlcgen.New(Trace(s.defaultDB)).GetBnetProfile(ctx, sqlcgen.GetBnetProfileParams{
		Toon:    toon,
		Gateway: gateway,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fetchedAt, err := time.Parse(time.RFC3339, row.FetchedAt)
	if err != nil {
		return nil, err
	}
	return &BnetProfileRow{
		Toon:        toon,
		Gateway:     gateway,
		Found:       row.Found,
		AuroraID:    row.AuroraID,
		BattleTag:   row.BattleTag,
		CountryCode: row.CountryCode,
		Payload:     row.Payload,
		FetchedAt:   fetchedAt,
	}, nil
}

func (s *Store) UpsertBnetProfile(ctx context.Context, row BnetProfileRow) error {
	return sqlcgen.New(Trace(s.defaultDB)).UpsertBnetProfile(ctx, sqlcgen.UpsertBnetProfileParams{
		Toon:        row.Toon,
		Gateway:     row.Gateway,
		Found:       row.Found,
		AuroraID:    row.AuroraID,
		BattleTag:   row.BattleTag,
		CountryCode: row.CountryCode,
		Payload:     row.Payload,
		FetchedAt:   row.FetchedAt.UTC().Format(time.RFC3339),
	})
}
