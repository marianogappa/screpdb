package persist

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestBnetCacheRoundTrip(t *testing.T) {
	root := t.TempDir()
	cache := NewBnetCache(root)
	if err := cache.Load(); err != nil || cache.Len() != 0 {
		t.Fatalf("empty cache: len=%d err=%v", cache.Len(), err)
	}

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	flash := BnetProfile{Toon: "Flash", Gateway: 30, Found: true, AuroraID: 11, BattleTag: "Flash#1", CountryCode: "KR", FetchedAt: now, Payload: `{"big":"payload"}`}
	flashOld := BnetProfile{Toon: "FLASH", Gateway: 10, Found: true, AuroraID: 12, CountryCode: "US", FetchedAt: now.Add(-48 * time.Hour), Payload: `{"old":true}`}
	missing := BnetProfile{Toon: "Nobody", Gateway: 30, Found: false, FetchedAt: now}
	noCountry := BnetProfile{Toon: "Bisu", Gateway: 30, Found: true, AuroraID: 13, FetchedAt: now, Payload: `{}`}
	for _, p := range []BnetProfile{flash, flashOld, missing, noCountry} {
		if err := cache.Upsert(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.Upsert(BnetProfile{}); err == nil {
		t.Fatal("empty toon must be rejected")
	}

	path := BnetProfilePath(root, "Flash", 30)
	if !strings.HasSuffix(path, filepath.Join("bnet_profiles", "30", filepath.Base(path))) || len(filepath.Base(path)) != len("0123456789abcdef.json") {
		t.Fatalf("unexpected path %s", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"toon", "gateway", "found", "aurora_id", "battle_tag", "country_code", "fetched_at", "payload"} {
		if _, ok := onDisk[key]; !ok {
			t.Fatalf("entry lacks %q", key)
		}
	}

	reloaded := NewBnetCache(root)
	if err := reloaded.Load(); err != nil {
		t.Fatal(err)
	}
	if reloaded.Len() != 4 {
		t.Fatalf("reloaded %d entries", reloaded.Len())
	}
	got, err := reloaded.Get("Flash", 30)
	if err != nil || got == nil || got.Payload != flash.Payload || got.AuroraID != 11 || !got.FetchedAt.Equal(now) {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if got, err := reloaded.Get("Flash", 99); err != nil || got != nil {
		t.Fatal("unknown gateway must return nil, nil")
	}

	codes := reloaded.CountryCodesByToons([]string{" flash ", "nobody", "bisu", "ghost"})
	if len(codes) != 1 || codes["flash"] != "KR" {
		t.Fatalf("country codes %+v", codes)
	}
	payloads, err := reloaded.PayloadsByToons([]string{"Flash", "Nobody", "Bisu"})
	if err != nil || len(payloads) != 3 {
		t.Fatalf("payloads %d %v", len(payloads), err)
	}
	for _, p := range payloads {
		if !p.Found || (p.Toon != "Bisu" && p.Payload == "") {
			t.Fatalf("payload entry %+v", p)
		}
	}

	updated := flash
	updated.CountryCode = "JP"
	updated.Payload = `{"v":2}`
	if err := reloaded.Upsert(updated); err != nil {
		t.Fatal(err)
	}
	if reloaded.Len() != 4 {
		t.Fatal("upsert must replace, not duplicate")
	}
	if got, _ := reloaded.Get("Flash", 30); got.CountryCode != "JP" || got.Payload != `{"v":2}` {
		t.Fatalf("after upsert %+v", got)
	}

	pruned, err := reloaded.PruneOlderThan(time.Since(now) + 24*time.Hour)
	if err != nil || pruned != 1 || reloaded.Len() != 3 {
		t.Fatalf("pruned %d err %v len %d", pruned, err, reloaded.Len())
	}
	if _, err := os.Stat(BnetProfilePath(root, "FLASH", 10)); !os.IsNotExist(err) {
		t.Fatal("pruned file must be deleted")
	}
}

func TestBnetCacheLoadSkipsCorruptFiles(t *testing.T) {
	root := t.TempDir()
	cache := NewBnetCache(root)
	if err := cache.Upsert(BnetProfile{Toon: "Flash", Gateway: 30, Found: true, FetchedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache.Dir(), "30", "garbage.json"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache.Dir(), "30", "notes.txt"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	reloaded := NewBnetCache(root)
	if err := reloaded.Load(); err != nil || reloaded.Len() != 1 {
		t.Fatalf("len=%d err=%v", reloaded.Len(), err)
	}
}
