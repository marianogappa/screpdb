package dashboard

import (
	"embed"
	"fmt"
	"log"

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
	// db, err := sql.Open("postgres", postgresConnectionString)
	// if err != nil {
	// 	return fmt.Errorf("failed to connect to Postgres: %w", err)
	// }
	// if err := db.Ping(); err != nil {
	// 	return fmt.Errorf("failed to ping database: %w", err)
	// }
	// driver, err := postgres.WithInstance(db, &postgres.Config{})
	// if err != nil {
	// 	return fmt.Errorf("failed to create database driver: %w", err)
	// }
	temp := "postgres://marianol@localhost/screpdb?sslmode=disable"
	m, err := migrate.NewWithSourceInstance("iofs", d, temp)
	if err != nil {
		return fmt.Errorf("failed to create migrator instance: %w", err)
	}
	if err := m.Up(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}
