package db

import (
	"context"
	"time"
)

// BnetGameResultRow is one cached game_results entry of a Battle.net account.
type BnetGameResultRow struct {
	AuroraID        int64
	GameID          string
	CreateTime      time.Time
	Toon            string
	Gateway         int
	Race            string
	Result          string
	APM             int
	DurationSeconds int
	MapName         string
	MatchGUID       string
}

// UpsertBnetGameResults records the games a profile fetch reported. Existing
// rows are left untouched: a game does not change after it was played.
func (s *Store) UpsertBnetGameResults(ctx context.Context, rows []BnetGameResultRow) error {
	if len(rows) == 0 {
		return nil
	}
	tx, err := s.defaultDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO bnet_game_results
		(aurora_id, game_id, create_time, toon, gateway, race, result, apm, duration_seconds, map_name, match_guid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		if row.AuroraID == 0 || row.GameID == "" {
			continue
		}
		if _, err := stmt.ExecContext(ctx, row.AuroraID, row.GameID, row.CreateTime.Unix(), row.Toon, row.Gateway,
			row.Race, row.Result, row.APM, row.DurationSeconds, row.MapName, row.MatchGUID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ListBnetGameTimes returns the play times of an account's cached games since
// a point in time, newest first.
func (s *Store) ListBnetGameTimes(ctx context.Context, auroraID int64, since time.Time) ([]time.Time, error) {
	rows, err := Trace(s.defaultDB).QueryContext(ctx, `SELECT create_time FROM bnet_game_results
		WHERE aurora_id = ? AND create_time >= ? ORDER BY create_time DESC`, auroraID, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []time.Time{}
	for rows.Next() {
		var unix int64
		if err := rows.Scan(&unix); err != nil {
			return nil, err
		}
		out = append(out, time.Unix(unix, 0).UTC())
	}
	return out, rows.Err()
}
