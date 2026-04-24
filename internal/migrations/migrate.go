package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed replay/*.sql
var replayFS embed.FS

//go:embed dashboard/*.sql
var dashboardFS embed.FS

// MigrationSet represents which set of migrations to run
type MigrationSet string

const (
	MigrationSetReplay    MigrationSet = "replay"
	MigrationSetDashboard MigrationSet = "dashboard"
)

// RunMigrations runs all pending migrations for both replay and dashboard sets
func RunMigrations(sqlitePath string) error {
	if err := RunMigrationSet(sqlitePath, MigrationSetReplay); err != nil {
		return err
	}
	if err := RunMigrationSet(sqlitePath, MigrationSetDashboard); err != nil {
		return err
	}
	return nil
}

// RunMigrationSet runs migrations for a specific set (replay or dashboard).
// Applied migrations are recorded in the `schema_migrations_<set>` table so
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
	case MigrationSetDashboard:
		fs = dashboardFS
		subdir = "dashboard"
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

// DropAllMigrations drops all migrations (runs down migrations for both sets)
func DropAllMigrations(sqlitePath string) error {
	if err := DropMigrationSet(sqlitePath, MigrationSetReplay); err != nil {
		return err
	}
	if err := DropMigrationSet(sqlitePath, MigrationSetDashboard); err != nil {
		return err
	}
	return nil
}

// DropMigrationSet drops migrations for a specific set
func DropMigrationSet(sqlitePath string, set MigrationSet) error {
	var fs embed.FS
	var subdir string

	switch set {
	case MigrationSetReplay:
		fs = replayFS
		subdir = "replay"
	case MigrationSetDashboard:
		fs = dashboardFS
		subdir = "dashboard"
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

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".down.sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	for _, name := range files {
		migrationPath := path.Join(subdir, name)
		body, err := fs.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", name, err)
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
