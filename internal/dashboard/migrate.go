package dashboard

import (
	"embed"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var fs embed.FS

// TODO fix schema problem.
func runMigrations(postgresConnectionString string) error {
	d, err := iofs.New(fs, "migrations")
	if err != nil {
		log.Fatal(err)
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
