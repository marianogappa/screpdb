package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

// filterModeOnlyThese is the only mode the settings row ever held; the filter
// is a presence-based whitelist, so the column is vestigial.
const filterModeOnlyThese = "only_these"

// FileSettings is the settings surface backed by settings.json instead of a
// SQLite row. It applies the stored global filter to the library on every
// change, so what is persisted and what the views filter on cannot diverge.
type FileSettings struct {
	root string
	lib  *library.Library

	mu      sync.Mutex
	current persist.Settings
}

// NewFileSettings loads settings.json under root and applies its filter to
// lib. A missing file yields the defaults without writing anything.
func NewFileSettings(root string, lib *library.Library) (*FileSettings, error) {
	loaded, _, err := persist.LoadSettings(root)
	if err != nil {
		return nil, err
	}
	s := &FileSettings{root: root, lib: lib, current: loaded}
	if lib != nil {
		if err := lib.SetFilter(loaded.GlobalFilter); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Settings is the current persisted state.
func (s *FileSettings) Settings() persist.Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Save replaces the persisted state and reapplies the filter.
func (s *FileSettings) Save(next persist.Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(next)
}

func (s *FileSettings) saveLocked(next persist.Settings) error {
	if err := persist.SaveSettings(s.root, next); err != nil {
		return err
	}
	if s.lib != nil {
		if err := s.lib.SetFilter(next.GlobalFilter); err != nil {
			return err
		}
	}
	reloaded, _, err := persist.LoadSettings(s.root)
	if err != nil {
		return err
	}
	s.current = reloaded
	return nil
}

func (s *FileSettings) GetIngestInputDir(_ context.Context, _ string) (string, error) {
	return s.Settings().ReplayFolder, nil
}

func (s *FileSettings) SetIngestInputDir(_ context.Context, _ string, inputDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.current
	next.ReplayFolder = inputDir
	return s.saveLocked(next)
}

func (s *FileSettings) GetFeatureFlagsJSON(_ context.Context, _ string) (string, error) {
	raw, err := json.Marshal(s.Settings().FeatureFlags)
	if err != nil {
		return "{}", err
	}
	return string(raw), nil
}

func (s *FileSettings) SetFeatureFlagsJSON(_ context.Context, _ string, raw string) error {
	flags := map[string]bool{}
	if err := json.Unmarshal([]byte(raw), &flags); err != nil {
		return fmt.Errorf("feature flags are not a JSON object of booleans: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.current
	next.FeatureFlags = flags
	return s.saveLocked(next)
}

func (s *FileSettings) GetGlobalReplayFilterConfigRaw(_ context.Context, _ string) (GlobalReplayFilterConfigRaw, error) {
	filter := s.Settings().GlobalFilter
	gameTypes, err := json.Marshal(filter.GameTypes)
	if err != nil {
		return GlobalReplayFilterConfigRaw{}, err
	}
	mapKinds, err := json.Marshal(filter.MapKinds)
	if err != nil {
		return GlobalReplayFilterConfigRaw{}, err
	}
	return GlobalReplayFilterConfigRaw{
		GameTypesMode:             filterModeOnlyThese,
		GameTypesJSON:             string(gameTypes),
		ExcludeShortGames:         filter.ExcludeShortGames,
		ExcludeComputers:          filter.ExcludeComputers,
		MapKindFilterMode:         filterModeOnlyThese,
		MapKindsJSON:              string(mapKinds),
		PlayerFilterMode:          filterModeOnlyThese,
		PlayersJSON:               "[]",
		LegacyIncludedPlayersJSON: "[]",
		LegacyExcludedPlayersJSON: "[]",
	}, nil
}

func (s *FileSettings) UpdateGlobalReplayFilterConfigRaw(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	gameTypesJSON string,
	excludeShortGames bool,
	excludeComputers bool,
	_ string,
	mapKindsJSON string,
	_ string,
	_ string,
	_ string,
) error {
	gameTypes, err := decodeStringSlice(gameTypesJSON)
	if err != nil {
		return fmt.Errorf("game types are not a JSON array of strings: %w", err)
	}
	mapKinds, err := decodeStringSlice(mapKindsJSON)
	if err != nil {
		return fmt.Errorf("map kinds are not a JSON array of strings: %w", err)
	}
	filter, err := library.FilterConfig{
		GameTypes:         gameTypes,
		ExcludeShortGames: excludeShortGames,
		ExcludeComputers:  excludeComputers,
		MapKinds:          mapKinds,
	}.Normalize()
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.current
	next.GlobalFilter = filter
	return s.saveLocked(next)
}

func decodeStringSlice(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
