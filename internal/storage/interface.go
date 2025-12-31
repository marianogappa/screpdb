package storage

import (
	"context"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// Storage backend constants
const (
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

	// FilterOutExistingReplays filters out replays that already exist in the database
	// Returns only the FileInfo objects for replays that don't exist yet
	FilterOutExistingReplays(ctx context.Context, files []fileops.FileInfo) ([]fileops.FileInfo, error)

	// Query executes a SQL query and returns results
	Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)

	// StorageName returns the name of the storage backend
	StorageName() string

	// GetDatabaseSchema returns the database schema information
	GetDatabaseSchema(ctx context.Context) (string, error)

	// Pattern detection methods
	// FilterOutExistingPatternDetections filters out replays that already have pattern detection run
	// with the current algorithm version. Returns only FileInfo objects for replays that need detection.
	FilterOutExistingPatternDetections(ctx context.Context, files []fileops.FileInfo, algorithmVersion int) ([]fileops.FileInfo, error)

	// DeletePatternDetectionsForReplay deletes all pattern detection results for a replay
	// Used when algorithm version is lower than current
	DeletePatternDetectionsForReplay(ctx context.Context, replayID int64) error

	// BatchInsertPatternResults inserts pattern detection results in batch
	BatchInsertPatternResults(ctx context.Context, results []*core.PatternResult) error

	// Close closes the storage connection
	Close() error
}
