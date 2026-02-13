package dashboard

import (
	"database/sql"
	"fmt"

	"github.com/marianogappa/screpdb/internal/migrations"
)

func runMigrations(sqlitePath string) error {
	if err := migrations.RunMigrations(sqlitePath); err != nil {
		return err
	}
	return ensureDashboardReplayFilterColumn(sqlitePath)
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
