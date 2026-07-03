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

// replayDataTables are the tables owned by the replay migration set that a
// --clean wipe must drop (player_aliases is preserved and tested separately).
var replayDataTables = []string{"replays", "players", "commands", "commands_low_value", "replay_events"}

func TestDropAllMigrations_DropsEveryTableIncludingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	db := openDB(t, path)

	for _, tbl := range append(append([]string{}, replayDataTables...), "player_aliases", "settings") {
		if !tableExists(t, db, tbl) {
			t.Fatalf("precondition: table %q must exist before DropAllMigrations", tbl)
		}
	}

	if err := DropAllMigrations(path); err != nil {
		t.Fatalf("DropAllMigrations: %v", err)
	}

	// Unlike --clean, DropAllMigrations wipes the settings set too, so
	// player_aliases and settings must be gone along with the replay tables.
	for _, tbl := range append(append([]string{}, replayDataTables...), "player_aliases", "settings") {
		if tableExists(t, db, tbl) {
			t.Errorf("table %q should be dropped by DropAllMigrations", tbl)
		}
	}
	for _, set := range []MigrationSet{MigrationSetReplay, MigrationSetDashboard, MigrationSetSettings} {
		if tableExists(t, db, migrationsTableName(set)) {
			t.Errorf("ledger for set %q should be dropped by DropAllMigrations", set)
		}
	}
}

func TestDropAllMigrations_SafeOnFreshDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := DropAllMigrations(path); err != nil {
		t.Fatalf("DropAllMigrations on fresh DB should be a no-op, got: %v", err)
	}
}

func TestCleanAndRunMigrations_DropAndReapplyCycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("initial RunMigrations: %v", err)
	}

	db := openDB(t, path)
	// player_aliases survives --clean but NOT CleanAndRunMigrations, which
	// drops the settings set too; seeding it proves the full wipe.
	if _, err := db.Exec(
		`INSERT INTO player_aliases (canonical_alias, battle_tag_normalized, battle_tag_raw, source) VALUES ('me','me','Me','you')`,
	); err != nil {
		t.Fatalf("seed alias row: %v", err)
	}

	if err := CleanAndRunMigrations(path); err != nil {
		t.Fatalf("CleanAndRunMigrations: %v", err)
	}

	// Every table is recreated by the reapply.
	for _, tbl := range append(append([]string{}, replayDataTables...), "player_aliases", "settings") {
		if !tableExists(t, db, tbl) {
			t.Errorf("table %q should be recreated after CleanAndRunMigrations", tbl)
		}
	}
	for _, set := range []MigrationSet{MigrationSetReplay, MigrationSetDashboard, MigrationSetSettings} {
		if !tableExists(t, db, migrationsTableName(set)) {
			t.Errorf("ledger for set %q should be recreated after CleanAndRunMigrations", set)
		}
	}

	// The seeded row must be gone: clean drops the table, reapply makes it empty.
	var aliasCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM player_aliases`).Scan(&aliasCount); err != nil {
		t.Fatalf("count aliases after clean+run: %v", err)
	}
	if aliasCount != 0 {
		t.Errorf("player_aliases should be empty after clean+reapply, got %d rows", aliasCount)
	}

	// Ledgers are fully repopulated so subsequent RunMigrations no-ops.
	if got := appliedNames(t, db, MigrationSetReplay); len(got) != 2 {
		t.Errorf("replay ledger should have 2 applied migrations after reapply, got %v", got)
	}
}

func TestCleanAndRunMigrations_OnFreshDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := CleanAndRunMigrations(path); err != nil {
		t.Fatalf("CleanAndRunMigrations on fresh DB: %v", err)
	}
	db := openDB(t, path)
	for _, tbl := range append(append([]string{}, replayDataTables...), "player_aliases", "settings") {
		if !tableExists(t, db, tbl) {
			t.Errorf("table %q should exist after CleanAndRunMigrations on fresh DB", tbl)
		}
	}
}

func TestCleanAndRunMigrationSet_ReappliesSingleSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrations(path); err != nil {
		t.Fatalf("initial RunMigrations: %v", err)
	}

	db := openDB(t, path)
	// Seed a preserved-set row to prove CleanAndRunMigrationSet(replay) leaves it intact.
	if _, err := db.Exec(
		`INSERT INTO player_aliases (canonical_alias, battle_tag_normalized, battle_tag_raw, source) VALUES ('a','a','A','you')`,
	); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	if err := CleanAndRunMigrationSet(path, MigrationSetReplay); err != nil {
		t.Fatalf("CleanAndRunMigrationSet(replay): %v", err)
	}

	for _, tbl := range replayDataTables {
		if !tableExists(t, db, tbl) {
			t.Errorf("table %q should be recreated by CleanAndRunMigrationSet(replay)", tbl)
		}
	}

	// Dropping+reapplying only the replay set must not disturb settings-owned data.
	var aliasCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM player_aliases`).Scan(&aliasCount); err != nil {
		t.Fatalf("count aliases: %v", err)
	}
	if aliasCount != 1 {
		t.Errorf("player_aliases should survive CleanAndRunMigrationSet(replay), got %d rows", aliasCount)
	}

	if got := appliedNames(t, db, MigrationSetReplay); len(got) != 2 {
		t.Errorf("replay ledger should be repopulated, got %v", got)
	}
}

func TestCleanAndRunMigrationSet_UnknownSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := CleanAndRunMigrationSet(path, MigrationSet("bogus")); err == nil {
		t.Fatal("expected error for unknown migration set")
	}
}

func TestDropMigrationSet_UnknownSet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := DropMigrationSet(path, MigrationSet("bogus")); err == nil {
		t.Fatal("expected error for unknown migration set")
	}
}

func TestDropMigrationSet_ClearsLedgerAllowingReapply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.db")
	if err := RunMigrationSet(path, MigrationSetReplay); err != nil {
		t.Fatalf("RunMigrationSet(replay): %v", err)
	}
	db := openDB(t, path)
	if got := appliedNames(t, db, MigrationSetReplay); len(got) != 2 {
		t.Fatalf("precondition: replay ledger should have 2 entries, got %v", got)
	}

	if err := DropMigrationSet(path, MigrationSetReplay); err != nil {
		t.Fatalf("DropMigrationSet(replay): %v", err)
	}
	if tableExists(t, db, migrationsTableName(MigrationSetReplay)) {
		t.Error("replay ledger table should be dropped")
	}

	// Reapply must recreate the tables and repopulate the ledger from scratch.
	if err := RunMigrationSet(path, MigrationSetReplay); err != nil {
		t.Fatalf("reapply RunMigrationSet(replay): %v", err)
	}
	if !tableExists(t, db, "replays") {
		t.Error("replays should exist after reapply")
	}
	if got := appliedNames(t, db, MigrationSetReplay); len(got) != 2 {
		t.Errorf("replay ledger should be repopulated on reapply, got %v", got)
	}
}

// unwritableDBPath returns a path that sqlite cannot open as a database file:
// the path itself is an existing directory, so the first Exec fails. Used to
// exercise the error-return branches without touching production code.
func unwritableDBPath(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func TestRunMigrations_PropagatesReplaySetError(t *testing.T) {
	if err := RunMigrations(unwritableDBPath(t)); err == nil {
		t.Fatal("expected RunMigrations to fail when the DB path is a directory")
	}
}

func TestRunMigrationSet_PropagatesExecError(t *testing.T) {
	if err := RunMigrationSet(unwritableDBPath(t), MigrationSetReplay); err == nil {
		t.Fatal("expected RunMigrationSet to fail when the DB path is a directory")
	}
}

func TestDropMigrationSet_PropagatesExecError(t *testing.T) {
	if err := DropMigrationSet(unwritableDBPath(t), MigrationSetReplay); err == nil {
		t.Fatal("expected DropMigrationSet to fail when the DB path is a directory")
	}
}

func TestDropAllMigrations_PropagatesExecError(t *testing.T) {
	if err := DropAllMigrations(unwritableDBPath(t)); err == nil {
		t.Fatal("expected DropAllMigrations to fail when the DB path is a directory")
	}
}

func TestCleanAndRunMigrations_PropagatesDropError(t *testing.T) {
	if err := CleanAndRunMigrations(unwritableDBPath(t)); err == nil {
		t.Fatal("expected CleanAndRunMigrations to fail when the DB path is a directory")
	}
}

func TestCleanAndRunMigrationSet_PropagatesDropError(t *testing.T) {
	if err := CleanAndRunMigrationSet(unwritableDBPath(t), MigrationSetReplay); err == nil {
		t.Fatal("expected CleanAndRunMigrationSet to fail when the DB path is a directory")
	}
}

func TestSqliteDSN(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "file:screp.db?_pragma=foreign_keys(1)"},
		{"whitespace trimmed to empty", "   ", "file:screp.db?_pragma=foreign_keys(1)"},
		{"plain path", "/tmp/x.db", "file:/tmp/x.db?_pragma=foreign_keys(1)"},
		{"plain path trimmed", "  /tmp/x.db  ", "file:/tmp/x.db?_pragma=foreign_keys(1)"},
		{"memory", ":memory:", ":memory:?_pragma=foreign_keys(1)"},
		{"file scheme no pragma", "file:/tmp/x.db", "file:/tmp/x.db?_pragma=foreign_keys(1)"},
		{"file scheme with existing query", "file:/tmp/x.db?cache=shared", "file:/tmp/x.db?cache=shared&_pragma=foreign_keys(1)"},
		{"file scheme already has pragma", "file:/tmp/x.db?_pragma=foreign_keys(1)", "file:/tmp/x.db?_pragma=foreign_keys(1)"},
		// ":memory:" only matches when it is the exact string; with a query
		// suffix it is neither ":memory:" nor a "file:" URI, so it falls through
		// to the plain-path branch and gets wrapped as a file: path.
		{"memory with query falls through to plain path", ":memory:?_pragma=foreign_keys(1)", "file::memory:?_pragma=foreign_keys(1)?_pragma=foreign_keys(1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sqliteDSN(tc.in); got != tc.want {
				t.Errorf("sqliteDSN(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
