package storage

import (
	"context"

	"github.com/marianogappa/screpdb/internal/models"
)

// Storage backend constants
const (
	StorageSQLite     = "sqlite"
	StoragePostgreSQL = "postgresql"
)

// ReplayDataChannel represents a channel for sending replay data to storage
type ReplayDataChannel chan *models.ReplayData

// Storage defines the interface for persisting replay data
type Storage interface {
	// Initialize sets up the storage (create tables, etc.)
	// If clean is true, drops all existing tables before creating new ones
	Initialize(ctx context.Context, clean bool) error

	// StartIngestion starts the ingestion process with batching
	// Returns a channel for sending replay data and a done channel
	StartIngestion(ctx context.Context) (ReplayDataChannel, <-chan error)

	// ReplayExists checks if a replay already exists by file path or checksum
	ReplayExists(ctx context.Context, filePath, checksum string) (bool, error)

	// BatchReplayExists checks if multiple replays already exist by file paths and checksums
	// Returns a map of file paths to boolean indicating existence
	BatchReplayExists(ctx context.Context, filePaths, checksums []string) (map[string]bool, error)

	// Query executes a SQL query and returns results
	Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)

	// StorageName returns the name of the storage backend
	StorageName() string

	// GetDatabaseSchema returns the database schema information
	GetDatabaseSchema(ctx context.Context) (string, error)

	// Close closes the storage connection
	Close() error
}
