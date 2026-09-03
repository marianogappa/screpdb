package library_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestLibraryHasNoDatabaseDependencies keeps the in-memory corpus free of the
// SQLite ingest stack: importing storage or ingest would drag database/sql and
// the driver into the dashboard binary path this package exists to replace.
func TestLibraryHasNoDatabaseDependencies(t *testing.T) {
	forbidden := []string{
		"database/sql",
		"modernc.org/sqlite",
		"github.com/marianogappa/screpdb/internal/storage",
		"github.com/marianogappa/screpdb/internal/ingest",
		"github.com/marianogappa/screpdb/internal/migrations",
		"github.com/marianogappa/screpdb/internal/dashboard",
	}
	fset := token.NewFileSet()
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			importPath, _ := strconv.Unquote(imp.Path.Value)
			for _, bad := range forbidden {
				if importPath == bad || strings.HasPrefix(importPath, bad+"/") {
					t.Errorf("%s imports %s", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
