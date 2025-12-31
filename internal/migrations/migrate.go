package migrations

import (
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
