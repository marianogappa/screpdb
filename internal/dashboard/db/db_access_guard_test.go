package db

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestStoreQueriesPreferSQLCForStaticSQL(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve current file")
	}
	rootDir := filepath.Dir(currentFile)

	allowedManualQueryFiles := map[string]struct{}{
		"store.go":                 {}, // query wrapper implementation
		"player_insight_queries.go": {}, // dynamic outlier SQL composition
		"unit_cadence_queries.go":   {}, // dynamic per-race/per-unit SQL composition
		"workflow_games_queries.go": {}, // runtime-composed workflow filters/sorts
	}

	manualQueryPattern := regexp.MustCompile(`\bs\.(Replay|Default)Query(Row)?Context\(`)

	err := filepath.WalkDir(rootDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "sqlc" || d.Name() == "sqlcgen" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		base := filepath.Base(path)
		if _, ok := allowedManualQueryFiles[base]; ok {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if manualQueryPattern.Match(content) {
			t.Errorf("found manual store query helper usage in %s; use sqlc-generated query methods for static SQL", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk db package files: %v", err)
	}
}
