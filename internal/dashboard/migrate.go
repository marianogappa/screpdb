package dashboard

import (
	"database/sql"
	"fmt"

	"github.com/marianogappa/screpdb/internal/migrations"
)

// NOTE: Migration routines intentionally keep direct SQL calls in this file.
// Runtime dashboard query/scan paths should go through internal/dashboard/db.

func runMigrations(sqlitePath string) error {
	if err := migrations.RunMigrations(sqlitePath); err != nil {
		return err
	}
	if err := ensureDashboardReplayFilterColumn(sqlitePath); err != nil {
		return err
	}
	if err := ensureDashboardVariablesColumn(sqlitePath); err != nil {
		return err
	}
	return ensureGlobalReplayFilterConfigColumns(sqlitePath)
}

func ensureDashboardReplayFilterColumn(sqlitePath string) error {
	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`PRAGMA table_info(dashboards);`)
	if err != nil {
		return fmt.Errorf("failed to query dashboards table info: %w", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("failed to scan table info: %w", err)
		}
		if name == "replays_filter_sql" {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read table info: %w", err)
	}
	if found {
		return nil
	}

	if _, err := db.Exec(`ALTER TABLE dashboards ADD COLUMN replays_filter_sql TEXT;`); err != nil {
		return fmt.Errorf("failed to add dashboards.replays_filter_sql: %w", err)
	}
	return nil
}

func ensureDashboardVariablesColumn(sqlitePath string) error {
	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(`PRAGMA table_info(dashboards);`)
	if err != nil {
		return fmt.Errorf("failed to query dashboards table info: %w", err)
	}
	defer rows.Close()

	var found bool
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("failed to scan table info: %w", err)
		}
		if name == "variables" {
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read table info: %w", err)
	}
	if found {
		return nil
	}

	if _, err := db.Exec(`ALTER TABLE dashboards ADD COLUMN variables TEXT;`); err != nil {
		return fmt.Errorf("failed to add dashboards.variables: %w", err)
	}
	return nil
}

func ensureGlobalReplayFilterConfigColumns(sqlitePath string) error {
	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	settingsExists, err := tableExists(db, "settings")
	if err != nil {
		return err
	}
	legacyExists, err := tableExists(db, "global_replay_filter_config")
	if err != nil {
		return err
	}
	if !settingsExists && legacyExists {
		if _, err := db.Exec(`ALTER TABLE global_replay_filter_config RENAME TO settings;`); err != nil {
			return fmt.Errorf("failed to rename global_replay_filter_config to settings: %w", err)
		}
		settingsExists = true
	}
	if !settingsExists {
		if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS settings (
			config_key TEXT PRIMARY KEY,
			game_type TEXT NOT NULL DEFAULT 'all',
			exclude_short_games BOOLEAN NOT NULL DEFAULT 1,
			exclude_computers BOOLEAN NOT NULL DEFAULT 1,
			included_maps TEXT NOT NULL DEFAULT '[]',
			excluded_maps TEXT NOT NULL DEFAULT '[]',
			included_players TEXT NOT NULL DEFAULT '[]',
			excluded_players TEXT NOT NULL DEFAULT '[]',
			ingest_input_dir TEXT NOT NULL DEFAULT '',
			compiled_replays_filter_sql TEXT,
			updated_at TEXT DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT settings_config_key_check CHECK (config_key = 'global')
		);`); err != nil {
			return fmt.Errorf("failed to create settings table: %w", err)
		}
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO settings (
		config_key,
		game_type,
		exclude_short_games,
		exclude_computers,
		included_maps,
		excluded_maps,
		included_players,
		excluded_players
	) VALUES ('global', 'all', 1, 1, '[]', '[]', '[]', '[]');`); err != nil {
		return fmt.Errorf("failed to ensure settings row: %w", err)
	}

	rows, err := db.Query(`PRAGMA table_info(settings);`)
	if err != nil {
		return fmt.Errorf("failed to query settings table info: %w", err)
	}
	defer rows.Close()

	existingColumns := map[string]struct{}{}
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("failed to scan settings table info: %w", err)
		}
		existingColumns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to read settings table info: %w", err)
	}

	requiredColumns := []struct {
		name string
		sql  string
	}{
		{name: "game_types_mode", sql: `ALTER TABLE settings ADD COLUMN game_types_mode TEXT NOT NULL DEFAULT 'only_these';`},
		{name: "game_types", sql: `ALTER TABLE settings ADD COLUMN game_types TEXT NOT NULL DEFAULT '[]';`},
		{name: "map_filter_mode", sql: `ALTER TABLE settings ADD COLUMN map_filter_mode TEXT NOT NULL DEFAULT 'only_these';`},
		{name: "maps", sql: `ALTER TABLE settings ADD COLUMN maps TEXT NOT NULL DEFAULT '[]';`},
		{name: "player_filter_mode", sql: `ALTER TABLE settings ADD COLUMN player_filter_mode TEXT NOT NULL DEFAULT 'only_these';`},
		{name: "players", sql: `ALTER TABLE settings ADD COLUMN players TEXT NOT NULL DEFAULT '[]';`},
		{name: "ingest_input_dir", sql: `ALTER TABLE settings ADD COLUMN ingest_input_dir TEXT NOT NULL DEFAULT '';`},
	}

	for _, column := range requiredColumns {
		if _, ok := existingColumns[column.name]; ok {
			continue
		}
		if _, err := db.Exec(column.sql); err != nil {
			return fmt.Errorf("failed to add settings.%s: %w", column.name, err)
		}
	}
	return nil
}

func tableExists(db *sql.DB, tableName string) (bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&count); err != nil {
		return false, fmt.Errorf("failed checking for table %s: %w", tableName, err)
	}
	return count > 0, nil
}
