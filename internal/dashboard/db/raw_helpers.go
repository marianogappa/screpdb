package db

import "database/sql"

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
	if _, err := db.Exec(`CREATE TEMP VIEW replay_events AS SELECT * FROM main.replay_events WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	return nil
}
