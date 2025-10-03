package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// BuildingsInserter implements BatchInserter for buildings
type BuildingsInserter struct{}

// NewBuildingsInserter creates a new buildings inserter
func NewBuildingsInserter() *BuildingsInserter {
	return &BuildingsInserter{}
}

// TableName returns the table name for buildings
func (b *BuildingsInserter) TableName() string {
	return "buildings"
}

var buildingsColumnNames = []string{
	"replay_id", "player_id", "building_id", "type", "name", "created", "created_frame", "x", "y",
}

// ColumnNames returns the column names for buildings
func (b *BuildingsInserter) ColumnNames() []string {
	return buildingsColumnNames
}

// EntityCount returns the number of columns for buildings
func (b *BuildingsInserter) EntityCount() int {
	return len(buildingsColumnNames)
}

// BuildArgs builds the arguments for a building entity
func (b *BuildingsInserter) BuildArgs(entity any, args []any, offset int) error {
	building, ok := entity.(*models.Building)
	if !ok {
		return fmt.Errorf("expected *models.Building, got %T", entity)
	}

	args[offset] = building.ReplayID
	args[offset+1] = building.PlayerID
	args[offset+2] = building.BuildingID
	args[offset+3] = building.Type
	args[offset+4] = building.Name
	args[offset+5] = building.Created
	args[offset+6] = building.CreatedFrame
	args[offset+7] = building.X
	args[offset+8] = building.Y

	return nil
}
