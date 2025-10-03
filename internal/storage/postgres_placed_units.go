package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// PlacedUnitsInserter implements BatchInserter for placed units
type PlacedUnitsInserter struct{}

// NewPlacedUnitsInserter creates a new placed units inserter
func NewPlacedUnitsInserter() *PlacedUnitsInserter {
	return &PlacedUnitsInserter{}
}

// TableName returns the table name for placed units
func (p *PlacedUnitsInserter) TableName() string {
	return "placed_units"
}

// ColumnNames returns the column names for placed units
func (p *PlacedUnitsInserter) ColumnNames() []string {
	return []string{
		"replay_id", "player_id", "type", "name", "x", "y",
	}
}

// EntityCount returns the number of columns for placed units
func (p *PlacedUnitsInserter) EntityCount() int {
	return 6
}

// BuildArgs builds the arguments for a placed unit entity
func (p *PlacedUnitsInserter) BuildArgs(entity any, args []any, offset int) error {
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
