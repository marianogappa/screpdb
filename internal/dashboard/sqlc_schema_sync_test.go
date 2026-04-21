package dashboard

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type sqliteColumnDef struct {
	Type    string
	NotNull bool
}

func TestSQLCSchemaMatchesRuntimeSchemaForDeclaredColumns(t *testing.T) {
	runtimeDBPath := filepath.Join(t.TempDir(), "runtime.db")
	if err := runMigrations(runtimeDBPath); err != nil {
		t.Fatalf("failed to run runtime migrations: %v", err)
	}

	runtimeDB, err := sql.Open("sqlite", sqliteDSN(runtimeDBPath))
	if err != nil {
		t.Fatalf("failed to open runtime db: %v", err)
	}
	defer runtimeDB.Close()

	sqlcDBPath := filepath.Join(t.TempDir(), "sqlc.db")
	sqlcDB, err := sql.Open("sqlite", sqliteDSN(sqlcDBPath))
	if err != nil {
		t.Fatalf("failed to open sqlc db: %v", err)
	}
	defer sqlcDB.Close()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve current file")
	}
	schemaPath := filepath.Join(filepath.Dir(currentFile), "db", "sqlc", "schema.sql")
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("failed to read sqlc schema file: %v", err)
	}
	if _, err := sqlcDB.Exec(string(schemaSQL)); err != nil {
		t.Fatalf("failed to apply sqlc schema: %v", err)
	}

	tables := []string{
		"settings",
		"replays",
		"players",
		"player_aliases",
		"dashboards",
		"dashboard_widgets",
		"detected_patterns_replay_player",
		"detected_patterns_replay",
		"replay_events",
		"commands",
		"commands_low_value",
	}
	for _, table := range tables {
		sqlcColumns, err := loadSQLiteTableColumns(sqlcDB, table)
		if err != nil {
			t.Fatalf("failed to read sqlc columns for %s: %v", table, err)
		}
		if len(sqlcColumns) == 0 {
			t.Fatalf("sqlc schema table %s has no columns", table)
		}

		runtimeColumns, err := loadSQLiteTableColumns(runtimeDB, table)
		if err != nil {
			t.Fatalf("failed to read runtime columns for %s: %v", table, err)
		}

		for columnName, sqlcColumn := range sqlcColumns {
			runtimeColumn, found := runtimeColumns[columnName]
			if !found {
				t.Errorf("runtime schema missing column %s.%s declared in sqlc schema", table, columnName)
				continue
			}
			if normalizeSQLiteType(sqlcColumn.Type) != normalizeSQLiteType(runtimeColumn.Type) {
				t.Errorf(
					"type mismatch for %s.%s: sqlc=%q runtime=%q",
					table,
					columnName,
					sqlcColumn.Type,
					runtimeColumn.Type,
				)
			}
			if sqlcColumn.NotNull != runtimeColumn.NotNull {
				t.Errorf(
					"nullability mismatch for %s.%s: sqlc notNull=%t runtime notNull=%t",
					table,
					columnName,
					sqlcColumn.NotNull,
					runtimeColumn.NotNull,
				)
			}
		}
	}
}

func loadSQLiteTableColumns(db *sql.DB, tableName string) (map[string]sqliteColumnDef, error) {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `);`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := map[string]sqliteColumnDef{}
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[name] = sqliteColumnDef{
			Type:    colType,
			NotNull: notNull == 1,
		}
	}
	return columns, rows.Err()
}

func normalizeSQLiteType(colType string) string {
	return strings.ToUpper(strings.TrimSpace(colType))
}
