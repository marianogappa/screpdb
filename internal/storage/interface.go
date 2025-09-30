package storage

import (
	"context"

	"github.com/marianogappa/screpdb/internal/models"
)

// Storage defines the interface for persisting replay data
type Storage interface {
	// Initialize sets up the storage (create tables, etc.)
	Initialize(ctx context.Context) error

	// StoreReplay stores a complete replay data structure
	StoreReplay(ctx context.Context, data *models.ReplayData) error

	// ReplayExists checks if a replay already exists by file path or checksum
	ReplayExists(ctx context.Context, filePath, checksum string) (bool, error)

	// Query executes a SQL query and returns results
	Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)

	// Close closes the storage connection
	Close() error
}
