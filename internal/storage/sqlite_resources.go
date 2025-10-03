package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLiteResourcesInserter implements SQLiteBatchInserter for resources
type SQLiteResourcesInserter struct{}

// NewSQLiteResourcesInserter creates a new SQLite resources inserter
func NewSQLiteResourcesInserter() *SQLiteResourcesInserter {
	return &SQLiteResourcesInserter{}
}

// TableName returns the table name for resources
func (r *SQLiteResourcesInserter) TableName() string {
	return "resources"
}

var sqliteResourcesColumnNames = []string{
	"replay_id", "type", "x", "y", "amount",
}

// ColumnNames returns the column names for resources
func (r *SQLiteResourcesInserter) ColumnNames() []string {
	return sqliteResourcesColumnNames
}

// EntityCount returns the number of columns for resources
func (r *SQLiteResourcesInserter) EntityCount() int {
	return len(sqliteResourcesColumnNames)
}

// BuildArgs builds the arguments for a resource entity
func (r *SQLiteResourcesInserter) BuildArgs(entity any, args []any, offset int) error {
	resource, ok := entity.(*models.Resource)
	if !ok {
		return fmt.Errorf("expected *models.Resource, got %T", entity)
	}

	args[offset] = resource.ReplayID
	args[offset+1] = resource.Type
	args[offset+2] = resource.X
	args[offset+3] = resource.Y
	args[offset+4] = resource.Amount

	return nil
}
