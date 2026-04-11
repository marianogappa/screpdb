package dashboard

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestDashboardDBAccessUsesStore(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve current file")
	}
	rootDir := filepath.Dir(currentFile)

	allowedDirectSQLFiles := map[string]struct{}{
		"migrate.go": {}, // schema migration code path
	}

	dbCallPattern := regexp.MustCompile(`\bdb\.(QueryContext|QueryRowContext|ExecContext|QueryRow|Query|Exec)\(`)

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "db" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		base := filepath.Base(path)
		if _, ok := allowedDirectSQLFiles[base]; ok {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if dbCallPattern.Match(content) {
			t.Errorf("found direct db Query/Exec call in %s; use internal/dashboard/db store wrappers", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk dashboard files: %v", err)
	}
}
