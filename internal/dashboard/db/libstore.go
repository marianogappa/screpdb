package db

import (
	"context"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

// SettingsStore is the JSON-file settings surface the settings-backed Reader
// methods need. It is an interface so the persistence wiring can land
// independently of the reads that use it.
type SettingsStore interface {
	GetIngestInputDir(ctx context.Context, configKey string) (string, error)
	SetIngestInputDir(ctx context.Context, configKey, inputDir string) error
	GetFeatureFlagsJSON(ctx context.Context, configKey string) (string, error)
	SetFeatureFlagsJSON(ctx context.Context, configKey, raw string) error
	GetGlobalReplayFilterConfigRaw(ctx context.Context, configKey string) (GlobalReplayFilterConfigRaw, error)
	UpdateGlobalReplayFilterConfigRaw(ctx context.Context, configKey string, legacyGameType string, gameTypesMode string, gameTypesJSON string, excludeShortGames bool, excludeComputers bool, mapKindFilterMode string, mapKindsJSON string, playerFilterMode string, playersJSON string, compiledReplaysFilterSQL string) error
}

// LibStore answers the dashboard's reads from the in-memory replay library
// instead of SQL. Methods are added per use case in the libstore_*.go files.
//
// The integrator adds `var _ Reader = (*LibStore)(nil)` once every use case
// has landed; asserting it earlier would not compile.
type LibStore struct {
	lib      *library.Library
	bnet     *persist.BnetCache
	settings SettingsStore
}

func NewLibStore(lib *library.Library, bnet *persist.BnetCache, settings SettingsStore) *LibStore {
	return &LibStore{lib: lib, bnet: bnet, settings: settings}
}
