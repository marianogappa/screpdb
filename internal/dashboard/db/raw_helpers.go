package db

import (
	"context"
	"database/sql"
	"fmt"
)

func EnableForeignKeys(db *sql.DB) error {
	_, err := db.Exec(`PRAGMA foreign_keys = ON;`)
	return err
}

func ApplyReplayTempViews(db *sql.DB, qualifiedFilterSQL string) error {
	if _, err := db.Exec(`CREATE TEMP VIEW replays AS ` + qualifiedFilterSQL); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW players AS SELECT * FROM main.players WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW commands AS SELECT * FROM main.commands WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW commands_low_value AS SELECT * FROM main.commands_low_value WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW detected_patterns_replay AS SELECT * FROM main.detected_patterns_replay WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW replay_events AS SELECT * FROM main.replay_events WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW detected_patterns_replay_player AS
		SELECT *
		FROM main.detected_patterns_replay_player
		WHERE replay_id IN (SELECT id FROM replays)
			AND player_id IN (SELECT id FROM players)`); err != nil {
		return err
	}
	return nil
}

func ValidateSelectOnDB(ctx context.Context, db *sql.DB, qualifiedSQL string) error {
	row := QueryRowContextOnDB(ctx, db, fmt.Sprintf("SELECT 1 FROM (%s) LIMIT 1", qualifiedSQL))
	var tmp int
	if err := row.Scan(&tmp); err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func ReplayIDFilterSQL(replayID int64) string {
	return fmt.Sprintf("SELECT * FROM replays WHERE id = %d", replayID)
}
