package db

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/marianogappa/screpdb/internal/library"
)

func newFileSettings(t *testing.T) (*FileSettings, *library.Library) {
	t.Helper()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	settings, err := NewFileSettings(t.TempDir(), lib)
	if err != nil {
		t.Fatalf("NewFileSettings: %v", err)
	}
	return settings, lib
}

func TestFileSettingsStartsFromTheDefaults(t *testing.T) {
	settings, lib := newFileSettings(t)
	ctx := context.Background()

	if dir, err := settings.GetIngestInputDir(ctx, "global"); err != nil || dir != "" {
		t.Fatalf("replay folder = %q, %v; want empty", dir, err)
	}
	raw, err := settings.GetGlobalReplayFilterConfigRaw(ctx, "global")
	if err != nil {
		t.Fatal(err)
	}
	if !raw.ExcludeShortGames || !raw.ExcludeComputers {
		t.Fatalf("defaults should exclude short games and computers: %+v", raw)
	}
	if raw.CompiledReplaysFilterSQL != nil {
		t.Fatal("no SQL should be reported any more")
	}
	if !lib.Filter().Equal(library.DefaultFilterConfig()) {
		t.Fatalf("library filter = %+v", lib.Filter())
	}
}

func TestFileSettingsPersistsAndAppliesTheFilter(t *testing.T) {
	settings, lib := newFileSettings(t)
	ctx := context.Background()

	if err := settings.UpdateGlobalReplayFilterConfigRaw(ctx, "global", "", filterModeOnlyThese,
		`["one_on_one"]`, false, false, filterModeOnlyThese, `["money"]`, filterModeOnlyThese, "[]", ""); err != nil {
		t.Fatalf("update: %v", err)
	}

	raw, err := settings.GetGlobalReplayFilterConfigRaw(ctx, "global")
	if err != nil {
		t.Fatal(err)
	}
	var gameTypes, mapKinds []string
	if err := json.Unmarshal([]byte(raw.GameTypesJSON), &gameTypes); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw.MapKindsJSON), &mapKinds); err != nil {
		t.Fatal(err)
	}
	if len(gameTypes) != 1 || gameTypes[0] != "one_on_one" || len(mapKinds) != 1 || mapKinds[0] != "money" {
		t.Fatalf("round trip lost the filter: %v %v", gameTypes, mapKinds)
	}
	if raw.ExcludeShortGames || raw.ExcludeComputers {
		t.Fatalf("exclusions were not persisted: %+v", raw)
	}

	applied := lib.Filter()
	if len(applied.GameTypes) != 1 || applied.GameTypes[0] != "one_on_one" || applied.ExcludeShortGames {
		t.Fatalf("library filter = %+v", applied)
	}

	reopened, err := NewFileSettings(settings.root, lib)
	if err != nil {
		t.Fatal(err)
	}
	if !reopened.Settings().GlobalFilter.Equal(applied) {
		t.Fatalf("reopened filter = %+v, want %+v", reopened.Settings().GlobalFilter, applied)
	}
}

func TestFileSettingsRoundTripsFolderAndFlags(t *testing.T) {
	settings, _ := newFileSettings(t)
	ctx := context.Background()

	if err := settings.SetIngestInputDir(ctx, "global", "/replays"); err != nil {
		t.Fatal(err)
	}
	if dir, err := settings.GetIngestInputDir(ctx, "global"); err != nil || dir != "/replays" {
		t.Fatalf("replay folder = %q, %v", dir, err)
	}

	if err := settings.SetFeatureFlagsJSON(ctx, "global", `{"gaming_session":true}`); err != nil {
		t.Fatal(err)
	}
	raw, err := settings.GetFeatureFlagsJSON(ctx, "global")
	if err != nil {
		t.Fatal(err)
	}
	flags := map[string]bool{}
	if err := json.Unmarshal([]byte(raw), &flags); err != nil {
		t.Fatal(err)
	}
	if !flags["gaming_session"] {
		t.Fatalf("flags = %v", flags)
	}
	if err := settings.SetFeatureFlagsJSON(ctx, "global", "not json"); err == nil {
		t.Fatal("malformed flags should be rejected")
	}
}

func TestFileSettingsRejectsMalformedFilterJSON(t *testing.T) {
	settings, _ := newFileSettings(t)
	err := settings.UpdateGlobalReplayFilterConfigRaw(context.Background(), "global", "", filterModeOnlyThese,
		"not json", true, true, filterModeOnlyThese, `["regular"]`, filterModeOnlyThese, "[]", "")
	if err == nil {
		t.Fatal("malformed game types should be rejected")
	}
}

var _ SettingsStore = (*FileSettings)(nil)
