package migrations

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openDB(t *testing.T, sqlitePath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var got string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&got)
	switch err {
	case nil:
		return true
	case sql.ErrNoRows:
		return false
	default:
		t.Fatalf("query sqlite_master for %s: %v", name, err)
		return false
	}
}

func appliedNames(t *testing.T, db *sql.DB, set MigrationSet) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM ` + migrationsTableName(set) + ` ORDER BY name`)
	if err != nil {
		t.Fatalf("query applied for %s: %v", set, err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan applied name: %v", err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	return names
}

func TestMigrationsTableName(t *testing.T) {
	cases := map[MigrationSet]string{
		MigrationSetReplay:    "schema_migrations_replay",
		MigrationSetDashboard: "schema_migrations_dashboard",
		MigrationSetSettings:  "schema_migrations_settings",
	}
	for set, want := range cases {
		if got := migrationsTableName(set); got != want {
			t.Errorf("migrationsTableName(%q) = %q, want %q", set, got, want)
		}
	}
}

func TestRunMigrations_CreatesSchemaAndLedgers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	db := openDB(t, path)

	for _, set := range []MigrationSet{MigrationSetReplay, MigrationSetDashboard, MigrationSetSettings} {
		if !tableExists(t, db, migrationsTableName(set)) {
			t.Errorf("missing ledger table for set %q", set)
		}
	}

	dataTables := []string{
		"replays", "players", "commands", "commands_low_value", "replay_events",
		"player_aliases", "settings",
	}
	for _, tbl := range dataTables {
		if !tableExists(t, db, tbl) {
			t.Errorf("expected data table %q to exist after RunMigrations", tbl)
		}
	}
}

func TestRunMigrations_RecordsEveryEmbeddedUpFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	db := openDB(t, path)

	want := map[MigrationSet][]string{
		MigrationSetReplay:    {"000001_initial.up.sql", "000002_add_load_action_types.up.sql"},
		MigrationSetDashboard: {"000001_initial.up.sql"},
		MigrationSetSettings:  {"000001_initial.up.sql"},
	}
	for set, wantNames := range want {
		got := appliedNames(t, db, set)
		if len(got) != len(wantNames) {
			t.Fatalf("set %q applied %v, want %v", set, got, wantNames)
		}
		for i := range wantNames {
			if got[i] != wantNames[i] {
				t.Errorf("set %q applied[%d]=%q, want %q", set, i, got[i], wantNames[i])
			}
		}
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	db := openDB(t, path)
	before := map[MigrationSet][]string{}
	for _, set := range []MigrationSet{MigrationSetReplay, MigrationSetDashboard, MigrationSetSettings} {
		before[set] = appliedNames(t, db, set)
	}

	// The default settings row is a good idempotency canary: the settings
	// migration does INSERT OR IGNORE, so re-running must not duplicate it and
	// must not error re-executing the CREATE TABLE / INSERT statements.
	var settingsRows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&settingsRows); err != nil {
		t.Fatalf("count settings: %v", err)
	}

	if err := RunMigrations(path); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	for _, set := range []MigrationSet{MigrationSetReplay, MigrationSetDashboard, MigrationSetSettings} {
		after := appliedNames(t, db, set)
		if len(after) != len(before[set]) {
			t.Errorf("set %q applied count changed: before %v, after %v", set, before[set], after)
		}
	}

	var settingsRowsAfter int
	if err := db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&settingsRowsAfter); err != nil {
		t.Fatalf("count settings after: %v", err)
	}
	if settingsRowsAfter != settingsRows {
		t.Errorf("settings row count changed on re-run: %d -> %d", settingsRows, settingsRowsAfter)
	}
}

func TestRunMigrationSet_AppliesOnlyOneSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrationSet(path, MigrationSetReplay); err != nil {
		t.Fatalf("RunMigrationSet(replay): %v", err)
	}
	db := openDB(t, path)

	if !tableExists(t, db, migrationsTableName(MigrationSetReplay)) {
		t.Error("replay ledger should exist")
	}
	if !tableExists(t, db, "replays") {
		t.Error("replays table should exist after replay set")
	}
	if tableExists(t, db, migrationsTableName(MigrationSetDashboard)) {
		t.Error("dashboard ledger should NOT exist when only replay set ran")
	}
	if tableExists(t, db, migrationsTableName(MigrationSetSettings)) {
		t.Error("settings ledger should NOT exist when only replay set ran")
	}
}

func TestRunMigrationSet_UnknownSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrationSet(path, MigrationSet("bogus")); err == nil {
		t.Fatal("expected error for unknown migration set")
	}
}

func TestEnsureMigrationsTable_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	db := openDB(t, path)

	if err := ensureMigrationsTable(db, MigrationSetReplay); err != nil {
		t.Fatalf("first ensureMigrationsTable: %v", err)
	}
	if err := ensureMigrationsTable(db, MigrationSetReplay); err != nil {
		t.Fatalf("second ensureMigrationsTable: %v", err)
	}
	if !tableExists(t, db, migrationsTableName(MigrationSetReplay)) {
		t.Error("ledger table should exist after ensureMigrationsTable")
	}

	// Empty ledger before any migration is recorded.
	if names := appliedNames(t, db, MigrationSetReplay); len(names) != 0 {
		t.Errorf("fresh ledger should be empty, got %v", names)
	}
}

func TestRecordAndLoadAppliedMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	db := openDB(t, path)
	if err := ensureMigrationsTable(db, MigrationSetReplay); err != nil {
		t.Fatalf("ensureMigrationsTable: %v", err)
	}

	if err := recordMigrationApplied(db, MigrationSetReplay, "000001_initial.up.sql"); err != nil {
		t.Fatalf("record: %v", err)
	}
	// Recording the same name twice must be a no-op (INSERT OR IGNORE).
	if err := recordMigrationApplied(db, MigrationSetReplay, "000001_initial.up.sql"); err != nil {
		t.Fatalf("record duplicate: %v", err)
	}

	applied, err := loadAppliedMigrations(db, MigrationSetReplay)
	if err != nil {
		t.Fatalf("loadAppliedMigrations: %v", err)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 applied migration, got %d: %v", len(applied), applied)
	}
	if _, ok := applied["000001_initial.up.sql"]; !ok {
		t.Errorf("expected recorded migration name to load back, got %v", applied)
	}
}

func TestCleanAndRunMigrations_PreservesSettingsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	db := openDB(t, path)
	if _, err := db.Exec(
		`INSERT INTO player_aliases (canonical_alias, battle_tag_normalized, battle_tag_raw, source) VALUES ('me','me','Me','you')`,
	); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	// --clean equivalent: drops replay+dashboard, preserves settings tables.
	if err := DropMigrationSet(path, MigrationSetReplay); err != nil {
		t.Fatalf("DropMigrationSet(replay): %v", err)
	}
	if err := DropMigrationSet(path, MigrationSetDashboard); err != nil {
		t.Fatalf("DropMigrationSet(dashboard): %v", err)
	}

	var aliasCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM player_aliases`).Scan(&aliasCount); err != nil {
		t.Fatalf("count aliases after clean: %v", err)
	}
	if aliasCount != 1 {
		t.Errorf("player_aliases should survive replay/dashboard drop, got %d rows", aliasCount)
	}
	if tableExists(t, db, "replays") {
		t.Error("replays table should be dropped by DropMigrationSet(replay)")
	}
}
