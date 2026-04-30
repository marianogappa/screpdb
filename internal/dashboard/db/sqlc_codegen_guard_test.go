package db

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSqlcQueriesAreASCII guards against multi-byte UTF-8 characters in the
// hand-edited query files under sqlc/queries/. sqlc v1.30.0 has a byte-offset
// arithmetic bug that strips the last two characters of the final keyword in
// a query when its preceding comment contains a multi-byte rune. The most
// visible symptom is `ORDER BY ... ASC;` becoming `ORDER BY ... A` in
// generated Go, which then fails at runtime with `near "A": syntax error`.
//
// Until we upgrade sqlc, keep .sql files ASCII-only.
func TestSqlcQueriesAreASCII(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("failed to resolve current file")
	}
	queriesDir := filepath.Join(filepath.Dir(currentFile), "sqlc", "queries")

	err := filepath.WalkDir(queriesDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for lineNum, line := range strings.Split(string(body), "\n") {
			for col, r := range line {
				if r > 0x7F {
					t.Errorf("%s:%d:%d non-ASCII rune %q (U+%04X): sqlc 1.30.0 will mistruncate the next ORDER BY/LIMIT clause; replace with ASCII",
						filepath.Base(path), lineNum+1, col+1, r, r)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}
