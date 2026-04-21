package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

func TestParseAliasImportJSONSkipsIdentityRows(t *testing.T) {
	raw := []byte(`{
		"Same": [{"battle_tag": "same"}],
		"Bisu": [{"battle_tag": "  lIlIlIlIIIll  "}]
	}`)
	records, err := parseAliasImportJSON(raw, aliasSourceImported)
	if err != nil {
		t.Fatalf("parseAliasImportJSON failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (identity skipped), got %d", len(records))
	}
	if records[0].CanonicalAlias != "Bisu" {
		t.Fatalf("expected Bisu record, got %#v", records[0])
	}
}

func TestParseAliasImportJSON(t *testing.T) {
	raw := []byte(`{
		"Bisu": [{"aurora_id": 123, "battle_tag": "  lIlIlIlIIIll  "}],
		"Flash": [{"battle_tag": "flashwolf"}]
	}`)
	records, err := parseAliasImportJSON(raw, aliasSourceImported)
	if err != nil {
		t.Fatalf("parseAliasImportJSON failed: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	found := map[string]aliasUpsertRecord{}
	for _, record := range records {
		found[record.CanonicalAlias] = record
	}
	if found["Bisu"].BattleTagNormalized != "lilililiiill" {
		t.Fatalf("unexpected normalized tag for Bisu: %q", found["Bisu"].BattleTagNormalized)
	}
	if found["Flash"].BattleTagNormalized != "flashwolf" {
		t.Fatalf("unexpected normalized tag for Flash: %q", found["Flash"].BattleTagNormalized)
	}
}

func TestCsettingsSearchDirsRemasteredReplays(t *testing.T) {
	base := filepath.FromSlash("/Users/me/Library/Application Support/Blizzard/StarCraft/Maps/Replays")
	dirs := csettingsSearchDirs(base)
	if len(dirs) < 3 {
		t.Fatalf("expected at least 3 dirs, got %d", len(dirs))
	}
	wantRoot := filepath.FromSlash("/Users/me/Library/Application Support/Blizzard/StarCraft")
	if dirs[2] != wantRoot {
		t.Fatalf("expected dirs[2]=%q, got %q", wantRoot, dirs[2])
	}
}

func TestFindCSettingsPathFromReplayDirWalksAncestors(t *testing.T) {
	root := t.TempDir()
	replays := filepath.Join(root, "StarCraft", "Maps", "Replays", "Ladder")
	if err := os.MkdirAll(replays, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(root, "StarCraft", "CSettings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := findCSettingsPathFromReplayDir(replays)
	if got != settingsPath {
		t.Fatalf("expected %q, got %q", settingsPath, got)
	}
}

func TestCsettingsSearchDirsIncludesStarCraftRootFromNestedReplays(t *testing.T) {
	base := filepath.FromSlash("/Users/x/Library/Application Support/Blizzard/StarCraft/Maps/Replays/Ladder")
	want := filepath.FromSlash("/Users/x/Library/Application Support/Blizzard/StarCraft")
	var found bool
	for _, d := range csettingsSearchDirs(base) {
		if d == want {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected StarCraft install root in search dirs for %q", base)
	}
}

func TestAliasSourcePrecedence(t *testing.T) {
	imported := dashboarddb.PlayerAliasRow{CanonicalAlias: "ImportedAlias", Source: aliasSourceImported, UpdatedAt: "2026-01-01 00:00:00"}
	manual := dashboarddb.PlayerAliasRow{CanonicalAlias: "ManualAlias", Source: aliasSourceManual, UpdatedAt: "2025-01-01 00:00:00"}
	you := dashboarddb.PlayerAliasRow{CanonicalAlias: "you", Source: aliasSourceYou, UpdatedAt: "2020-01-01 00:00:00"}

	if !chooseBetterAlias(&imported, manual) {
		t.Fatalf("manual alias should outrank imported alias")
	}
	if !chooseBetterAlias(&manual, you) {
		t.Fatalf("you alias should outrank manual alias")
	}
}

func TestParseCSettingsBattleTagsIgnoresNonAccountKeys(t *testing.T) {
	payload := map[string]any{
		"Accounts": []any{
			map[string]any{"battle_tag": "Ignored"},
			map[string]any{"DisplayName": "Ignored2"},
			map[string]any{"gateway": "U.S. West", "account": "chobo86", "timestamp": 1776624272},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	tags, err := parseCSettingsBattleTags(raw)
	if err != nil {
		t.Fatalf("parseCSettingsBattleTags failed: %v", err)
	}
	if len(tags) != 1 || tags[0] != "chobo86" {
		t.Fatalf("expected only chobo86, got %#v", tags)
	}
}

func TestParseCSettingsBattleTagsRecentAccountObjects(t *testing.T) {
	payload := map[string]any{
		"RecentLogins": []any{
			map[string]any{"gateway": "U.S. West", "account": "chobo86", "timestamp": 1776624272},
			map[string]any{"gateway": "U.S. East", "account": "chobo85s", "timestamp": 1701780823},
			map[string]any{"gateway": "Asia", "account": "chobo85s", "timestamp": 1731227821},
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	tags, err := parseCSettingsBattleTags(raw)
	if err != nil {
		t.Fatalf("parseCSettingsBattleTags failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 unique accounts, got %d: %#v", len(tags), tags)
	}
	found := map[string]struct{}{}
	for _, tag := range tags {
		found[tag] = struct{}{}
	}
	if _, ok := found["chobo86"]; !ok {
		t.Fatalf("missing chobo86: %#v", tags)
	}
	if _, ok := found["chobo85s"]; !ok {
		t.Fatalf("missing chobo85s: %#v", tags)
	}
}
