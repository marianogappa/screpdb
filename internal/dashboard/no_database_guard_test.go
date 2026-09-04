package dashboard_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestDashboardDoesNotOpenADatabase keeps the dashboard answering its reads
// from the in-memory replay library, offline pack building included. Reaching
// for the SQL stack again would reintroduce the ingest step, the migrations
// and the re-ingest that this package exists to be rid of.
func TestDashboardDoesNotOpenADatabase(t *testing.T) {
	forbidden := []string{
		"database/sql",
		"modernc.org/sqlite",
		"github.com/marianogappa/screpdb/internal/storage",
		"github.com/marianogappa/screpdb/internal/migrations",
		"github.com/marianogappa/screpdb/internal/ingest",
	}
	fset := token.NewFileSet()
	for _, dir := range []string{".", "db", "service", "apigen"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			path := filepath.Join(dir, name)
			file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			for _, imported := range file.Imports {
				importPath, _ := strconv.Unquote(imported.Path.Value)
				for _, bad := range forbidden {
					if importPath == bad || strings.HasPrefix(importPath, bad+"/") {
						t.Errorf("%s imports %s", path, importPath)
					}
				}
			}
		}
	}
}

// TestEmbeddedSpecHasNoIngestOperations pins the API rename: ingestion is not
// a step the user takes any more, so the operations that described it must not
// come back.
func TestEmbeddedSpecHasNoIngestOperations(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi", "dashboard.v1.yaml"))
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
