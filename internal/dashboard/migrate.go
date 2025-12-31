package dashboard

import (
	"github.com/marianogappa/screpdb/internal/migrations"
)

func runMigrations(postgresConnectionString string) error {
	return migrations.RunMigrations(postgresConnectionString)
}
