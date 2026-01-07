package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed *.sql
var fs embed.FS

// RunMigrations runs all pending migrations
func RunMigrations(postgresConnectionString string) error {
	d, err := iofs.New(fs, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, hackToFixPostgresConnectionStringFormat(postgresConnectionString))
	if err != nil {
		return fmt.Errorf("failed to create migrator instance: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// DropAllMigrations drops all migrations (runs down migrations)
func DropAllMigrations(postgresConnectionString string) error {
	d, err := iofs.New(fs, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, hackToFixPostgresConnectionStringFormat(postgresConnectionString))
	if err != nil {
		return fmt.Errorf("failed to create migrator instance: %w", err)
	}

	// Get the current version
	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// If no migrations have been run, nothing to drop
	if err == migrate.ErrNilVersion {
		return nil
	}

	// If database is in dirty state, force version to allow dropping
	if dirty {
		if err := m.Force(int(version)); err != nil {
			return fmt.Errorf("failed to force migration version: %w", err)
		}
	}

	// Drop all migrations by calling Down() until we reach version 0
	for {
		if err := m.Down(); err != nil {
			if err == migrate.ErrNoChange {
				// No more migrations to drop
				break
			}
			return fmt.Errorf("failed to drop migrations: %w", err)
		}
	}

	return nil
}

// CleanAndRunMigrations drops all migrations and runs them again
func CleanAndRunMigrations(postgresConnectionString string) error {
	if err := DropAllMigrations(postgresConnectionString); err != nil {
		return fmt.Errorf("failed to drop migrations: %w", err)
	}
	if err := RunMigrations(postgresConnectionString); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// DropDashboardTables drops only dashboard-related tables
func DropDashboardTables(postgresConnectionString string) error {
	db, err := sql.Open("postgres", postgresConnectionString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Start a transaction to match the pattern in down.sql
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop indexes first
	indexes := []string{
		"idx_dashboard_widgets_dashboard_id",
		"idx_dashboard_widgets_dashboard_id_widget_order",
	}

	for _, index := range indexes {
		if _, err := tx.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", index)); err != nil {
			return fmt.Errorf("failed to drop index %s: %w", index, err)
		}
	}

	// Drop tables in reverse order of dependencies
	// dashboard_widget_prompt_history depends on dashboard_widgets (via widget_id)
	// dashboard_widgets depends on dashboards (via dashboard_id)
	tables := []string{
		"dashboard_widget_prompt_history",
		"dashboard_widgets",
		"dashboards",
	}

	for _, table := range tables {
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DropNonDashboardTables drops all tables except dashboard-related tables
// Tables are dropped in the same order as in 000001_initial.down.sql
func DropNonDashboardTables(postgresConnectionString string) error {
	db, err := sql.Open("postgres", postgresConnectionString)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Start a transaction to match the pattern in down.sql
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop indexes first (non-dashboard indexes only)
	indexes := []string{
		"idx_detected_patterns_replay_player_player_id",
		"idx_detected_patterns_replay_player_replay_id",
		"idx_detected_patterns_replay_team_replay_id",
		"idx_detected_patterns_replay_replay_id",
		"idx_commands_frame",
		"idx_commands_player_id",
		"idx_commands_replay_id",
		"idx_players_replay_id",
		"idx_replays_replay_date",
		"idx_replays_file_checksum",
		"idx_replays_file_path",
	}

	for _, index := range indexes {
		if _, err := tx.Exec(fmt.Sprintf("DROP INDEX IF EXISTS %s", index)); err != nil {
			return fmt.Errorf("failed to drop index %s: %w", index, err)
		}
	}

	// Drop tables in reverse order of dependencies (matching down.sql order)
	tables := []string{
		"detected_patterns_replay_player",
		"detected_patterns_replay_team",
		"detected_patterns_replay",
		"commands",
		"players",
		"replays",
	}

	for _, table := range tables {
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CleanNonDashboardAndRunMigrations drops all non-dashboard tables and runs migrations again
func CleanNonDashboardAndRunMigrations(postgresConnectionString string) error {
	if err := DropNonDashboardTables(postgresConnectionString); err != nil {
		return fmt.Errorf("failed to drop non-dashboard tables: %w", err)
	}
	if err := RunMigrations(postgresConnectionString); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

func hackToFixPostgresConnectionStringFormat(postgresConnectionString string) string {
	if !strings.Contains(postgresConnectionString, "dbname=") {
		return postgresConnectionString
	}

	parts := strings.Fields(postgresConnectionString)
	kv := map[string]string{
		"dbname":   "postgres",
		"host":     "localhost",
		"port":     "5432",
		"user":     "postgres",
		"password": "",
	}
	for _, part := range parts {
		keyValue := strings.Split(part, "=")
		if len(keyValue) != 2 {
			continue
		}
		kv[keyValue[0]] = keyValue[1]
	}

	queryString := make([]string, 0, len(kv))
	for key, value := range kv {
		if key != "dbname" && key != "host" && key != "port" && key != "user" && key != "password" {
			queryString = append(queryString, fmt.Sprintf("%s=%s", key, value))
		}
	}

	question := "?"
	serializedQueryString := strings.Join(queryString, "&")
	if len(queryString) == 0 {
		question = ""
		serializedQueryString = ""
	}

	return fmt.Sprintf(
		"postgresql://%s:%s@%s/%s%s%s",
		kv["user"],
		kv["password"],
		kv["host"],
		kv["dbname"],
		question,
		serializedQueryString,
	)
}
