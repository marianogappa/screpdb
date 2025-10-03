package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// ResourcesInserter implements BatchInserter for resources
type ResourcesInserter struct{}

// NewResourcesInserter creates a new resources inserter
func NewResourcesInserter() *ResourcesInserter {
	return &ResourcesInserter{}
}

// TableName returns the table name for resources
func (r *ResourcesInserter) TableName() string {
	return "resources"
}

var resourcesColumnNames = []string{
	"replay_id", "type", "x", "y", "amount",
}

// ColumnNames returns the column names for resources
func (r *ResourcesInserter) ColumnNames() []string {
	return resourcesColumnNames
}

// EntityCount returns the number of columns for resources
func (r *ResourcesInserter) EntityCount() int {
	return len(resourcesColumnNames)
}

// BuildArgs builds the arguments for a resource entity
func (r *ResourcesInserter) BuildArgs(entity any, args []any, offset int) error {
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
