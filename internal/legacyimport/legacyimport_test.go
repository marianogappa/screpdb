package legacyimport

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
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
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mustExec(t, db, legacySchema)
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

	mustExec(t, db, `INSERT INTO bnet_game_results (aurora_id, game_id, create_time, toon, gateway, race, result, apm, duration_seconds, map_name, match_guid) VALUES (42, 'g1', 1756700000, 'Bisu', 30, 'Protoss', 'win', 300, 900, 'Polypoid', 'm1')`)

	second, err := Read(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.GameResults) != 1 || second.GameResults[0].GameID != "g1" || second.GameResults[0].CreateTimeUnix != 1756700000 || second.GameResults[0].APM != 300 {
		t.Fatalf("game results = %+v", second.GameResults)
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

// legacySchema is the shape a pre-library database has: the tables the
// dashboard used to own here, spelled out because the migrations no longer
// create them. The import reads exactly this and nothing else.
const legacySchema = `
CREATE TABLE settings (
	config_key TEXT PRIMARY KEY,
	game_type TEXT NOT NULL DEFAULT 'all',
	exclude_short_games BOOLEAN NOT NULL DEFAULT 1,
	exclude_computers BOOLEAN NOT NULL DEFAULT 1,
	ingest_input_dir TEXT NOT NULL DEFAULT '',
	game_types TEXT NOT NULL DEFAULT '[]',
	map_kinds TEXT NOT NULL DEFAULT '["regular","money"]'
);
INSERT INTO settings (config_key) VALUES ('global');
CREATE TABLE bnet_profiles (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	toon TEXT NOT NULL,
	gateway INTEGER NOT NULL,
	found BOOLEAN NOT NULL,
	aurora_id INTEGER NOT NULL DEFAULT 0,
	battle_tag TEXT NOT NULL DEFAULT '',
	country_code TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	UNIQUE (toon, gateway)
);
CREATE TABLE bnet_game_results (
	aurora_id INTEGER NOT NULL,
	game_id TEXT NOT NULL,
	create_time INTEGER NOT NULL,
	toon TEXT NOT NULL DEFAULT '',
	gateway INTEGER NOT NULL DEFAULT 0,
	race TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '',
	apm INTEGER NOT NULL DEFAULT 0,
	duration_seconds INTEGER NOT NULL DEFAULT 0,
	map_name TEXT NOT NULL DEFAULT '',
	match_guid TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (aurora_id, game_id)
);
`
