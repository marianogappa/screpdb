package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// UnitsInserter implements BatchInserter for units
type UnitsInserter struct{}

// NewUnitsInserter creates a new units inserter
func NewUnitsInserter() *UnitsInserter {
	return &UnitsInserter{}
}

// TableName returns the table name for units
func (u *UnitsInserter) TableName() string {
	return "units"
}

var unitsColumnNames = []string{
	"replay_id", "player_id", "unit_id", "type", "name", "created", "created_frame", "x", "y",
}

// ColumnNames returns the column names for units
func (u *UnitsInserter) ColumnNames() []string {
	return unitsColumnNames
}

// EntityCount returns the number of columns for units
func (u *UnitsInserter) EntityCount() int {
	return len(unitsColumnNames)
}

// BuildArgs builds the arguments for a unit entity
func (u *UnitsInserter) BuildArgs(entity any, args []any, offset int) error {
	unit, ok := entity.(*models.Unit)
	if !ok {
		return fmt.Errorf("expected *models.Unit, got %T", entity)
	}

	args[offset] = unit.ReplayID
	args[offset+1] = unit.PlayerID
	args[offset+2] = unit.UnitID
	args[offset+3] = unit.Type
	args[offset+4] = unit.Name
	args[offset+5] = unit.Created
	args[offset+6] = unit.CreatedFrame
	args[offset+7] = unit.X
	args[offset+8] = unit.Y

	return nil
}
