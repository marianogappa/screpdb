package legacyimport

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/marianogappa/screpdb/internal/migrations"
)

func TestReadMissingDatabase(t *testing.T) {
	_, err := Read(context.Background(), filepath.Join(t.TempDir(), "absent.db"))
	if !errors.Is(err, ErrNoDatabase) {
		t.Fatalf("err = %v, want ErrNoDatabase", err)
	}
}

func TestReadLegacyStateWithAndWithoutOptionalColumns(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "screp.db")
	if err := migrations.RunMigrations(path); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mustExec(t, db, `UPDATE settings SET ingest_input_dir = ' /replays ', game_type = 'melee', game_types = '[]', exclude_short_games = 0, map_kinds = '["Money"]' WHERE config_key = 'global'`)

	first, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Settings == nil {
		t.Fatal("expected settings row")
	}
	if first.Settings.ReplayDir != "/replays" || first.Settings.ExcludeShortGames || !first.Settings.ExcludeComputers {
		t.Fatalf("settings = %+v", *first.Settings)
	}
	if len(first.Settings.GameTypes) != 1 || first.Settings.GameTypes[0] != "melee" {
		t.Fatalf("legacy game_type fallback: %v", first.Settings.GameTypes)
	}
	if len(first.Settings.MapKinds) != 1 || first.Settings.MapKinds[0] != "money" {
		t.Fatalf("map kinds = %v", first.Settings.MapKinds)
	}
	if len(first.Settings.FeatureFlags) != 0 || len(first.Profiles) != 0 {
		t.Fatalf("expected no flags and no profiles, got %+v %+v", first.Settings.FeatureFlags, first.Profiles)
	}

	mustExec(t, db, `ALTER TABLE settings ADD COLUMN feature_flags TEXT NOT NULL DEFAULT '{}'`)
	mustExec(t, db, `UPDATE settings SET feature_flags = '{"gaming_session":true}', game_types = '["one_on_one","Melee"]', game_type = 'ums' WHERE config_key = 'global'`)
	mustExec(t, db, `INSERT INTO bnet_profiles (toon, gateway, found, aurora_id, battle_tag, country_code, payload, fetched_at) VALUES ('Bisu', 30, 1, 42, 'Bisu#1234', 'KR', '{"x":1}', '2026-09-01T00:00:00Z'), ('Nobody', 10, 0, 0, '', '', '{}', '2026-09-02T00:00:00Z')`)

	second, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Settings.FeatureFlags["gaming_session"] {
		t.Fatalf("feature flags = %v", second.Settings.FeatureFlags)
	}
	if len(second.Settings.GameTypes) != 2 || second.Settings.GameTypes[1] != "melee" {
		t.Fatalf("game types = %v", second.Settings.GameTypes)
	}
	if len(second.Profiles) != 2 {
		t.Fatalf("profiles = %+v", second.Profiles)
	}
	p := second.Profiles[0]
	if p.Toon != "Bisu" || p.Gateway != 30 || !p.Found || p.AuroraID != 42 || p.BattleTag != "Bisu#1234" || p.CountryCode != "KR" || p.Payload != `{"x":1}` {
		t.Fatalf("profile = %+v", p)
	}
	if second.Profiles[1].Found {
		t.Fatal("negative cache entry should read as not found")
	}
}

func TestReadDatabaseWithoutDashboardTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `CREATE TABLE replays (id INTEGER PRIMARY KEY)`)
	db.Close()
	res, err := Read(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Settings != nil || len(res.Profiles) != 0 {
		t.Fatalf("expected empty result, got %+v", res)
	}
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}
