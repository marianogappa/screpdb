// Package persist stores the dashboard's small durable state as JSON files
// under the app-data root: settings.json and one file per cached Battle.net
// profile. Every file operation goes through iofacade.
package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
)

const (
	SettingsFileName = "settings.json"
	SettingsVersion  = 1
)

// Settings is everything the dashboard remembers between runs.
type Settings struct {
	Version             int                  `json:"version"`
	ReplayFolder        string               `json:"replay_folder"`
	GlobalFilter        library.FilterConfig `json:"global_filter"`
	FeatureFlags        map[string]bool      `json:"feature_flags"`
	SampleSetAutoLoaded bool                 `json:"sample_set_auto_loaded"`
	// MaxReplays caps how many of the newest replays in the folder are read.
	// Zero means the built-in default; a negative value reads all of them.
	// There is no screen for this: a folder large enough to want it changed is
	// rare enough to edit the file.
	MaxReplays int       `json:"max_replays"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func DefaultSettings() Settings {
	return Settings{
		Version:      SettingsVersion,
		GlobalFilter: library.DefaultFilterConfig(),
		FeatureFlags: map[string]bool{},
	}
}

func SettingsPath(root string) string { return filepath.Join(root, SettingsFileName) }

// LoadSettings reads settings.json under root. A missing file yields the
// defaults with found=false. A malformed file is renamed aside to
// settings.json.corrupt-<unix> and the defaults are returned, so a bad write
// never blocks startup.
func LoadSettings(root string) (Settings, bool, error) {
	path := SettingsPath(root)
	raw, err := iofacade.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSettings(), false, nil
	}
	if err != nil {
		return DefaultSettings(), false, fmt.Errorf("persist: read %s: %w", path, err)
	}
	s := DefaultSettings()
	if err := json.Unmarshal(raw, &s); err != nil {
		aside := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		if renameErr := iofacade.Rename(path, aside); renameErr != nil {
			return DefaultSettings(), false, fmt.Errorf("persist: settings.json is malformed (%v) and could not be moved aside: %w", err, renameErr)
		}
		return DefaultSettings(), false, nil
	}
	normalized, err := s.GlobalFilter.Normalize()
	if err != nil {
		normalized = library.DefaultFilterConfig()
	}
	s.GlobalFilter = normalized
	if s.FeatureFlags == nil {
		s.FeatureFlags = map[string]bool{}
	}
	if s.Version == 0 {
		s.Version = SettingsVersion
	}
	return s, true, nil
}

// SaveSettings writes settings.json atomically (temp file + rename), stamping
// Version and UpdatedAt.
func SaveSettings(root string, s Settings) error {
	s.Version = SettingsVersion
	s.UpdatedAt = time.Now().UTC()
	if s.FeatureFlags == nil {
		s.FeatureFlags = map[string]bool{}
	}
	normalized, err := s.GlobalFilter.Normalize()
	if err != nil {
		return fmt.Errorf("persist: %w", err)
	}
	s.GlobalFilter = normalized
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("persist: encode settings: %w", err)
	}
	return writeFileAtomic(SettingsPath(root), raw)
}

func writeFileAtomic(path string, data []byte) error {
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("persist: create %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := iofacade.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("persist: write %s: %w", tmp, err)
	}
	if err := iofacade.Rename(tmp, path); err != nil {
		_ = iofacade.Remove(tmp)
		return fmt.Errorf("persist: rename %s: %w", tmp, err)
	}
	return nil
}
