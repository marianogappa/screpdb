package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// StartLocationsInserter implements BatchInserter for start locations
type StartLocationsInserter struct{}

// NewStartLocationsInserter creates a new start locations inserter
func NewStartLocationsInserter() *StartLocationsInserter {
	return &StartLocationsInserter{}
}

// TableName returns the table name for start locations
func (s *StartLocationsInserter) TableName() string {
	return "start_locations"
}

// ColumnNames returns the column names for start locations
func (s *StartLocationsInserter) ColumnNames() []string {
	return []string{
		"replay_id", "x", "y",
	}
}

// EntityCount returns the number of columns for start locations
func (s *StartLocationsInserter) EntityCount() int {
	return 3
}

// BuildArgs builds the arguments for a start location entity
func (s *StartLocationsInserter) BuildArgs(entity any, args []any, offset int) error {
	startLoc, ok := entity.(*models.StartLocation)
	if !ok {
		return fmt.Errorf("expected *models.StartLocation, got %T", entity)
	}

	args[offset] = startLoc.ReplayID
	args[offset+1] = startLoc.X
	args[offset+2] = startLoc.Y

	return nil
}
