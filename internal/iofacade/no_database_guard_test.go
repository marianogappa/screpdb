package iofacade_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// legacyImportPackage is the one place in the binary allowed to open a SQLite
// database. It reads a pre-library screp.db once, read-only, so an upgrade
// keeps the user's replay folder, filters and Battle.net cache (issue #381).
const legacyImportPackage = "internal/legacyimport"

// TestBinaryHasNoDatabaseDependencies is the module-wide successor to the
// per-package guards the dashboard and the replay library carried while the
// SQL stack was being removed. Now that ingest, storage and migrations are
// gone the rule describes the whole binary: nothing shipped may import them,
// database/sql, or the driver — except the one-time legacy import.
//
// The database went because the dashboard stopped exercising the write path,
// and a schema, migrations and queries nobody runs are how they quietly get
// buggy. Reaching for them again would bring the ingest step back with them.
func TestBinaryHasNoDatabaseDependencies(t *testing.T) {
	forbidden := []string{
		"database/sql",
		"modernc.org/sqlite",
		"github.com/marianogappa/screpdb/internal/storage",
		"github.com/marianogappa/screpdb/internal/migrations",
		"github.com/marianogappa/screpdb/internal/ingest",
	}
	root := moduleRoot(t)
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if skipDir(root, path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || skipFile(root, path) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, legacyImportPackage+"/") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imported := range file.Imports {
			importPath, _ := strconv.Unquote(imported.Path.Value)
			for _, bad := range forbidden {
				if importPath == bad || strings.HasPrefix(importPath, bad+"/") {
					t.Errorf("%s imports %s", rel, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
}

// TestLegacyImportIsTheOnlyDatabaseReader pins the exemption above to a real
// package, so deleting the legacy import also retires the exemption — and with
// it the last reason to depend on the SQLite driver.
func TestLegacyImportIsTheOnlyDatabaseReader(t *testing.T) {
	dir := filepath.Join(moduleRoot(t), legacyImportPackage)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("%s is gone; drop the exemption here and modernc.org/sqlite from go.mod", legacyImportPackage)
	}
}

// TestSpecHasNoIngestOperations pins the API rename: ingestion is not a step
// the user takes any more, so the operations that described it must not return.
func TestSpecHasNoIngestOperations(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(moduleRoot(t), "api", "openapi", "dashboard.v1.yaml"))
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	for _, gone := range []string{"/api/custom/ingest", "/api/custom/replays/stale-count", "compiled_replays_filter_sql"} {
		if strings.Contains(string(raw), gone) {
			t.Errorf("the spec still describes %s", gone)
		}
	}
	for _, wanted := range []string{"/api/custom/library/settings", "/api/custom/library/events", "/api/custom/library/rescan"} {
		if !strings.Contains(string(raw), wanted) {
			t.Errorf("the spec is missing %s", wanted)
		}
	}
}
