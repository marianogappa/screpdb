package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLitePlacedUnitsInserter implements SQLiteBatchInserter for placed units
type SQLitePlacedUnitsInserter struct{}

// NewSQLitePlacedUnitsInserter creates a new SQLite placed units inserter
func NewSQLitePlacedUnitsInserter() *SQLitePlacedUnitsInserter {
	return &SQLitePlacedUnitsInserter{}
}

// TableName returns the table name for placed units
func (p *SQLitePlacedUnitsInserter) TableName() string {
	return "placed_units"
}

var sqlitePlacedUnitsColumnNames = []string{
	"replay_id", "player_id", "type", "name", "x", "y",
}

// ColumnNames returns the column names for placed units
func (p *SQLitePlacedUnitsInserter) ColumnNames() []string {
	return sqlitePlacedUnitsColumnNames
}

// EntityCount returns the number of columns for placed units
func (p *SQLitePlacedUnitsInserter) EntityCount() int {
	return len(sqlitePlacedUnitsColumnNames)
}

// BuildArgs builds the arguments for a placed unit entity
func (p *SQLitePlacedUnitsInserter) BuildArgs(entity any, args []any, offset int) error {
	placedUnit, ok := entity.(*models.PlacedUnit)
	if !ok {
		return fmt.Errorf("expected *models.PlacedUnit, got %T", entity)
	}

	args[offset] = placedUnit.ReplayID
	args[offset+1] = placedUnit.PlayerID
	args[offset+2] = placedUnit.Type
	args[offset+3] = placedUnit.Name
	args[offset+4] = placedUnit.X
	args[offset+5] = placedUnit.Y

	return nil
}
