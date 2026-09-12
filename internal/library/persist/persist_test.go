package persist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
)

func TestSettingsRoundTripAndDefaults(t *testing.T) {
	root := filepath.Join(t.TempDir(), "appdata")

	s, found, err := LoadSettings(root)
	if err != nil || found {
		t.Fatalf("missing file: found=%v err=%v", found, err)
	}
	if !s.GlobalFilter.Equal(library.DefaultFilterConfig()) || s.FeatureFlags == nil || s.Version != SettingsVersion {
		t.Fatalf("defaults %+v", s)
	}

	s.ReplayFolder = "/replays"
	s.GlobalFilter = library.FilterConfig{GameTypes: []string{"Melee", "melee"}, ExcludeComputers: true}
	s.FeatureFlags = map[string]bool{"gaming_session": true}
	s.SampleSetAutoLoaded = true
	if err := SaveSettings(root, s); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SettingsPath(root) + ".tmp"); !os.IsNotExist(err) {
		t.Fatal("temp file must not survive a save")
	}

	loaded, found, err := LoadSettings(root)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if loaded.ReplayFolder != "/replays" || !loaded.SampleSetAutoLoaded || !loaded.FeatureFlags["gaming_session"] {
		t.Fatalf("loaded %+v", loaded)
	}
	if !loaded.GlobalFilter.Equal(library.FilterConfig{GameTypes: []string{"melee"}, MapKinds: []string{}, ExcludeComputers: true}) {
		t.Fatalf("filter not normalised: %+v", loaded.GlobalFilter)
	}
	if loaded.UpdatedAt.IsZero() || loaded.Version != SettingsVersion {
		t.Fatal("version and updated_at must be stamped")
	}

	raw, _ := os.ReadFile(SettingsPath(root))
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"version", "replay_folder", "global_filter", "feature_flags", "sample_set_auto_loaded", "updated_at"} {
		if _, ok := onDisk[key]; !ok {
			t.Fatalf("settings.json lacks %q", key)
		}
	}

	if err := SaveSettings(root, Settings{GlobalFilter: library.FilterConfig{GameTypes: []string{"bogus"}}}); err == nil {
		t.Fatal("an invalid filter must not be saved")
	}
}

func TestCorruptSettingsAreMovedAside(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(SettingsPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, found, err := LoadSettings(root)
	if err != nil || found {
		t.Fatalf("corrupt file: found=%v err=%v", found, err)
	}
	if !s.GlobalFilter.Equal(library.DefaultFilterConfig()) {
		t.Fatal("corrupt file must fall back to defaults")
	}
	if _, err := os.Stat(SettingsPath(root)); !os.IsNotExist(err) {
		t.Fatal("corrupt file must be renamed away")
	}
	entries, _ := os.ReadDir(root)
	aside := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), SettingsFileName+".corrupt-") {
			aside++
		}
	}
	if aside != 1 {
		t.Fatalf("expected one .corrupt-<unix> file, got %d", aside)
	}
}

func TestSettingsLoadFillsMissingFields(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(SettingsPath(root), []byte(`{"replay_folder":"/x","global_filter":{"game_types":["ums"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, found, err := LoadSettings(root)
	if err != nil || !found {
		t.Fatal(err)
	}
	if s.FeatureFlags == nil || s.Version != SettingsVersion || s.ReplayFolder != "/x" {
		t.Fatalf("%+v", s)
	}
	if !s.GlobalFilter.Equal(library.DefaultFilterConfig()) {
		t.Fatal("an invalid stored filter must fall back to the default filter")
	}
}
