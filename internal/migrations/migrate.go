package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/pgx"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
)

//go:embed replay/*.sql
var replayFS embed.FS

//go:embed dashboard/*.sql
var dashboardFS embed.FS

// MigrationSet represents which set of migrations to run
type MigrationSet string

const (
	MigrationSetReplay    MigrationSet = "replay"
	MigrationSetDashboard MigrationSet = "dashboard"
)

// RunMigrations runs all pending migrations for both replay and dashboard sets
func RunMigrations(postgresConnectionString string) error {
	if err := RunMigrationSet(postgresConnectionString, MigrationSetReplay); err != nil {
		return err
	}
	if err := RunMigrationSet(postgresConnectionString, MigrationSetDashboard); err != nil {
		return err
	}
	return nil
}

// RunMigrationSet runs migrations for a specific set (replay or dashboard)
func RunMigrationSet(postgresConnectionString string, set MigrationSet) error {
	var fs embed.FS
	var subdir string
	var stateTableName string

	switch set {
	case MigrationSetReplay:
		fs = replayFS
		subdir = "replay"
		stateTableName = "schema_migrations_replay"
	case MigrationSetDashboard:
		fs = dashboardFS
		subdir = "dashboard"
		stateTableName = "schema_migrations_dashboard"
	default:
		return fmt.Errorf("unknown migration set: %s", set)
	}

	d, err := iofs.New(fs, subdir)
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Open database connection
	dbURL := hackToFixPostgresConnectionStringFormat(postgresConnectionString)
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create database instance with custom state table name
	instance, err := pgx.WithInstance(db, &pgx.Config{
		MigrationsTable: stateTableName,
	})
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "pgx", instance)
	if err != nil {
		return fmt.Errorf("failed to create migrator instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// DropAllMigrations drops all migrations (runs down migrations for both sets)
func DropAllMigrations(postgresConnectionString string) error {
	if err := DropMigrationSet(postgresConnectionString, MigrationSetReplay); err != nil {
		return err
	}
	if err := DropMigrationSet(postgresConnectionString, MigrationSetDashboard); err != nil {
		return err
	}
	return nil
}

// DropMigrationSet drops migrations for a specific set
func DropMigrationSet(postgresConnectionString string, set MigrationSet) error {
	var fs embed.FS
	var subdir string
	var stateTableName string

	switch set {
	case MigrationSetReplay:
		fs = replayFS
		subdir = "replay"
		stateTableName = "schema_migrations_replay"
	case MigrationSetDashboard:
		fs = dashboardFS
		subdir = "dashboard"
		stateTableName = "schema_migrations_dashboard"
	default:
		return fmt.Errorf("unknown migration set: %s", set)
	}

	d, err := iofs.New(fs, subdir)
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Open database connection
	dbURL := hackToFixPostgresConnectionStringFormat(postgresConnectionString)
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Create database instance with custom state table name
	instance, err := pgx.WithInstance(db, &pgx.Config{
		MigrationsTable: stateTableName,
	})
	if err != nil {
		return fmt.Errorf("failed to create database instance: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", d, "pgx", instance)
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

// CleanAndRunMigrationSet drops and runs migrations for a specific set
func CleanAndRunMigrationSet(postgresConnectionString string, set MigrationSet) error {
	if err := DropMigrationSet(postgresConnectionString, set); err != nil {
		return fmt.Errorf("failed to drop migrations: %w", err)
	}
	if err := RunMigrationSet(postgresConnectionString, set); err != nil {
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
