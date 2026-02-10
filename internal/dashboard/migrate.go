package dashboard

import (
	"github.com/marianogappa/screpdb/internal/migrations"
)

func runMigrations(sqlitePath string) error {
	return migrations.RunMigrations(sqlitePath)
}
