package storage

import (
	"context"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/models"
)

// Storage backend constants
const (
	StorageSQLite = "sqlite"
)

// ReplayDataChannel represents a channel for sending replay data to storage
type ReplayDataChannel chan *models.ReplayData

type IngestionHooks struct {
	OnReplayStored    func()
	OnDuplicateReplay func(error)
	OnStoreError      func(error)
}

// Storage defines the interface for persisting replay data
type Storage interface {
	// Initialize sets up the storage (create tables, etc.)
	// If clean is true, drops all non-dashboard tables before creating new ones
	// If cleanDashboard is true, drops all dashboard tables
	Initialize(ctx context.Context, clean bool) error

	// StartIngestion starts the ingestion process with batching
	// Returns a channel for sending replay data and a done channel
	StartIngestion(ctx context.Context, hooks IngestionHooks) (ReplayDataChannel, <-chan error)

	// FilterOutExistingReplays filters out replays that already exist in the database
	// Returns only the FileInfo objects for replays that don't exist yet
	FilterOutExistingReplays(ctx context.Context, files []fileops.FileInfo) ([]fileops.FileInfo, error)

	// FilterOutExistingReplaysByPath filters out replays whose file_path is already
	// known to the database. Cheaper than FilterOutExistingReplays because it does
	// not require Checksum to be populated, so callers can run it before hashing.
	// Survivors still need a checksum-aware second pass to catch the file-moved /
	// file-renamed case where the same content lives at a new path.
	FilterOutExistingReplaysByPath(ctx context.Context, files []fileops.FileInfo) ([]fileops.FileInfo, error)

	// Query executes a SQL query and returns results
	Query(ctx context.Context, query string, args ...any) ([]map[string]any, error)

	// GetDatabaseSchema returns the database schema information
	GetDatabaseSchema(ctx context.Context) (string, error)

	// Close closes the storage connection
	Close() error
}
