package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLiteUnitsInserter implements SQLiteBatchInserter for units
type SQLiteUnitsInserter struct{}

// NewSQLiteUnitsInserter creates a new SQLite units inserter
func NewSQLiteUnitsInserter() *SQLiteUnitsInserter {
	return &SQLiteUnitsInserter{}
}

// TableName returns the table name for units
func (u *SQLiteUnitsInserter) TableName() string {
	return "units"
}

var sqliteUnitsColumnNames = []string{
	"replay_id", "player_id", "type", "created", "created_frame",
}

// ColumnNames returns the column names for units
func (u *SQLiteUnitsInserter) ColumnNames() []string {
	return sqliteUnitsColumnNames
}

// EntityCount returns the number of columns for units
func (u *SQLiteUnitsInserter) EntityCount() int {
	return len(sqliteUnitsColumnNames)
}

// BuildArgs builds the arguments for a unit entity
func (u *SQLiteUnitsInserter) BuildArgs(entity any, args []any, offset int) error {
	unit, ok := entity.(*models.Unit)
	if !ok {
		return fmt.Errorf("expected *models.Unit, got %T", entity)
	}

	args[offset] = unit.ReplayID
	args[offset+1] = unit.PlayerID
	args[offset+2] = unit.Type
	args[offset+3] = unit.Created
	args[offset+4] = unit.CreatedFrame

	return nil
}
