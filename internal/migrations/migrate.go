package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed replay/*.sql
var replayFS embed.FS

// MigrationSet represents which set of migrations to run
type MigrationSet string

const MigrationSetReplay MigrationSet = "replay"

// RunMigrations runs every pending migration. Only the replay set is left:
// the dashboard reads the replay folder into memory and keeps its own state in
// JSON files, so the tables it used to own here are gone.
func RunMigrations(sqlitePath string) error {
	return RunMigrationSet(sqlitePath, MigrationSetReplay)
}

// RunMigrationSet runs migrations for a specific set. Applied migrations are
// recorded in the `schema_migrations_<set>` table so
// re-invocations become no-ops — important because rebuild-style migrations
// (e.g. 000003_replay_events_refinement) destructively copy rows and would
// re-trip CHECK constraints against newer schemas (markers) added in later
// migrations like 000008.
func RunMigrationSet(sqlitePath string, set MigrationSet) error {
	var fs embed.FS
	var subdir string

	switch set {
	case MigrationSetReplay:
		fs = replayFS
		subdir = "replay"
	default:
		return fmt.Errorf("unknown migration set: %s", set)
	}

	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := ensureMigrationsTable(db, set); err != nil {
		return err
	}

	entries, err := fs.ReadDir(subdir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	applied, err := loadAppliedMigrations(db, set)
	if err != nil {
		return err
	}

	for _, name := range files {
		if _, ok := applied[name]; ok {
			continue
		}
		migrationPath := path.Join(subdir, name)
		body, err := fs.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", name, err)
		}
		if err := recordMigrationApplied(db, set, name); err != nil {
			return err
		}
	}
	return nil
}

func migrationsTableName(set MigrationSet) string {
	return "schema_migrations_" + string(set)
}

func ensureMigrationsTable(db *sql.DB, set MigrationSet) error {
	// Guarded name: schema_migrations_replay / schema_migrations_dashboard. Keeps
	// the two migration sets' histories independent.
	table := migrationsTableName(set)
	query := `CREATE TABLE IF NOT EXISTS ` + table + ` (
		name TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to ensure migrations table %s: %w", table, err)
	}
	return nil
}

func loadAppliedMigrations(db *sql.DB, set MigrationSet) (map[string]struct{}, error) {
	table := migrationsTableName(set)
	rows, err := db.Query(`SELECT name FROM ` + table)
	if err != nil {
		return nil, fmt.Errorf("failed to load applied migrations from %s: %w", table, err)
	}
	defer rows.Close()

	applied := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan applied migration name: %w", err)
		}
		applied[name] = struct{}{}
	}
	return applied, rows.Err()
}

func recordMigrationApplied(db *sql.DB, set MigrationSet, name string) error {
	table := migrationsTableName(set)
	if _, err := db.Exec(`INSERT OR IGNORE INTO `+table+` (name) VALUES (?)`, name); err != nil {
		return fmt.Errorf("failed to record applied migration %s: %w", name, err)
	}
	return nil
}

// DropAllMigrations drops every migration set. Used for fresh-DB nukes only
// (test setup, full reset).
func DropAllMigrations(sqlitePath string) error {
	return DropMigrationSet(sqlitePath, MigrationSetReplay)
}

// createTableRegexp matches `CREATE TABLE [IF NOT EXISTS] "?name"?` in a SQL file.
// Used by DropMigrationSet to rebuild the drop list when an up-only migration
// directory has no .down.sql counterparts.
var createTableRegexp = regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + "`?\"?" + `(\w+)` + "`?\"?")

// DropMigrationSet drops every table created by a specific migration set, then
// clears that set's migrations-applied ledger. RunMigrationSet can re-apply the
// migrations from scratch afterwards.
//
// Two paths:
//
//  1. If the migration directory contains .down.sql files, run them in reverse
//     filename order (the canonical down-migration flow).
//
//  2. If the directory is up-only (the .down.sql siblings were dropped during
//     a schema-consolidation), fall back to parsing every CREATE TABLE statement
//     out of the .up.sql files and DROP TABLE-ing them in reverse order. This is
//     the only correct behaviour for the "erase data" UI checkbox + the CLI
//     --clean flag now that the project has consolidated to a single up-only
//     migration per set; the previous .down.sql-only path silently dropped only
//     the migrations ledger and left the data tables intact.
func DropMigrationSet(sqlitePath string, set MigrationSet) error {
	var fs embed.FS
	var subdir string

	switch set {
	case MigrationSetReplay:
		fs = replayFS
		subdir = "replay"
	default:
		return fmt.Errorf("unknown migration set: %s", set)
	}

	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	entries, err := fs.ReadDir(subdir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var downFiles, upFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.HasSuffix(entry.Name(), ".down.sql"):
			downFiles = append(downFiles, entry.Name())
		case strings.HasSuffix(entry.Name(), ".up.sql"):
			upFiles = append(upFiles, entry.Name())
		}
	}

	if len(downFiles) > 0 {
		sort.Sort(sort.Reverse(sort.StringSlice(downFiles)))
		for _, name := range downFiles {
			migrationPath := path.Join(subdir, name)
			body, err := fs.ReadFile(migrationPath)
			if err != nil {
				return fmt.Errorf("failed to read migration %s: %w", name, err)
			}
			if _, err := db.Exec(string(body)); err != nil {
				return fmt.Errorf("failed to execute migration %s: %w", name, err)
			}
		}
	} else {
		// Up-only fallback: extract table names from every .up.sql, DROP TABLE
		// them in reverse declaration order so dependents go first. Indexes
		// die with their tables.
		var tables []string
		seen := map[string]struct{}{}
		sort.Strings(upFiles)
		for _, name := range upFiles {
			migrationPath := path.Join(subdir, name)
			body, err := fs.ReadFile(migrationPath)
			if err != nil {
				return fmt.Errorf("failed to read migration %s: %w", name, err)
			}
			for _, m := range createTableRegexp.FindAllStringSubmatch(string(body), -1) {
				name := m[1]
				if _, dup := seen[name]; dup {
					continue
				}
				seen[name] = struct{}{}
				tables = append(tables, name)
			}
		}
		for i := len(tables) - 1; i >= 0; i-- {
			if _, err := db.Exec(`DROP TABLE IF EXISTS ` + tables[i]); err != nil {
				return fmt.Errorf("failed to drop table %s: %w", tables[i], err)
			}
		}
	}

	// Clear the migrations-applied ledger so a subsequent RunMigrationSet re-applies
	// everything from scratch. DROP TABLE IF EXISTS keeps this safe on a fresh DB
	// where the ledger table was never created.
	if _, err := db.Exec(`DROP TABLE IF EXISTS ` + migrationsTableName(set)); err != nil {
		return fmt.Errorf("failed to drop migrations ledger %s: %w", migrationsTableName(set), err)
	}

	return nil
}

// CleanAndRunMigrations drops all migrations and runs them again
func CleanAndRunMigrations(sqlitePath string) error {
	if err := DropAllMigrations(sqlitePath); err != nil {
		return fmt.Errorf("failed to drop migrations: %w", err)
	}
	if err := RunMigrations(sqlitePath); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// CleanAndRunMigrationSet drops and runs migrations for a specific set
func CleanAndRunMigrationSet(sqlitePath string, set MigrationSet) error {
	if err := DropMigrationSet(sqlitePath, set); err != nil {
		return fmt.Errorf("failed to drop migrations: %w", err)
	}
	if err := RunMigrationSet(sqlitePath, set); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

func sqliteDSN(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "file:screp.db?_pragma=foreign_keys(1)"
	}
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		if strings.Contains(path, "_pragma=foreign_keys(1)") {
			return path
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return path + sep + "_pragma=foreign_keys(1)"
	}
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
}
