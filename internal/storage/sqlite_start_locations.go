package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLiteStartLocationsInserter implements SQLiteBatchInserter for start locations
type SQLiteStartLocationsInserter struct{}

// NewSQLiteStartLocationsInserter creates a new SQLite start locations inserter
func NewSQLiteStartLocationsInserter() *SQLiteStartLocationsInserter {
	return &SQLiteStartLocationsInserter{}
}

// TableName returns the table name for start locations
func (s *SQLiteStartLocationsInserter) TableName() string {
	return "start_locations"
}

var sqliteStartLocationsColumnNames = []string{
	"replay_id", "x", "y",
}

// ColumnNames returns the column names for start locations
func (s *SQLiteStartLocationsInserter) ColumnNames() []string {
	return sqliteStartLocationsColumnNames
}

// EntityCount returns the number of columns for start locations
func (s *SQLiteStartLocationsInserter) EntityCount() int {
	return len(sqliteStartLocationsColumnNames)
}

// BuildArgs builds the arguments for a start location entity
func (s *SQLiteStartLocationsInserter) BuildArgs(entity any, args []any, offset int) error {
	startLoc, ok := entity.(*models.StartLocation)
	if !ok {
		return fmt.Errorf("expected *models.StartLocation, got %T", entity)
	}

	args[offset] = startLoc.ReplayID
	args[offset+1] = startLoc.X
	args[offset+2] = startLoc.Y

	return nil
}
