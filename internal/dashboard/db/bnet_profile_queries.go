package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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

func (s *Store) GetBnetCountryCodesByPlayerKeys(ctx context.Context, playerKeys []string) (map[string]string, error) {
	if len(playerKeys) == 0 {
		return map[string]string{}, nil
	}
	placeholders := make([]string, len(playerKeys))
	args := make([]any, len(playerKeys))
	for i, key := range playerKeys {
		placeholders[i] = "?"
		args[i] = key
	}
	query := `SELECT LOWER(TRIM(toon)) AS player_key, country_code
		FROM bnet_profiles
		WHERE found = 1 AND country_code != '' AND LOWER(TRIM(toon)) IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY LOWER(TRIM(toon))`
	rows, err := Trace(s.defaultDB).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]string, len(playerKeys))
	for rows.Next() {
		var playerKey, countryCode string
		if err := rows.Scan(&playerKey, &countryCode); err != nil {
			return nil, err
		}
		result[playerKey] = countryCode
	}
	return result, rows.Err()
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

// BnetProfilePayloadRow is one cached profile payload.
type BnetProfilePayloadRow struct {
	Toon    string
	Gateway int64
	Payload string
}

// ListBnetProfilePayloadsByPlayerKeys returns every cached, found profile for
// the given normalized player keys. Read-only: it never triggers a fetch.
func (s *Store) ListBnetProfilePayloadsByPlayerKeys(ctx context.Context, playerKeys []string) ([]BnetProfilePayloadRow, error) {
	if len(playerKeys) == 0 {
		return []BnetProfilePayloadRow{}, nil
	}
	placeholders := make([]string, len(playerKeys))
	args := make([]any, len(playerKeys))
	for i, key := range playerKeys {
		placeholders[i] = "?"
		args[i] = key
	}
	query := `SELECT toon, gateway, payload
		FROM bnet_profiles
		WHERE found = 1 AND LOWER(TRIM(toon)) IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := Trace(s.defaultDB).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BnetProfilePayloadRow{}
	for rows.Next() {
		var row BnetProfilePayloadRow
		if err := rows.Scan(&row.Toon, &row.Gateway, &row.Payload); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
