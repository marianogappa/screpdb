package spec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// committedSpecPath returns the absolute path of the committed SPECIFICATION.md
// at the repo root (this test file lives in internal/spec, two levels down).
func committedSpecPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current file")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "SPECIFICATION.md")
}

// TestSpecificationUpToDate is the generation-parity guard: the committed
// SPECIFICATION.md must byte-for-byte equal freshly generated output. This is
// what makes the document unable to drift from the code. Enforced by the
// existing `go test ./...` step in CI.
func TestSpecificationUpToDate(t *testing.T) {
	want, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	got, err := os.ReadFile(committedSpecPath(t))
	if err != nil {
		t.Fatalf("read committed SPECIFICATION.md: %v (run `go generate ./...`)", err)
	}

	if string(got) != string(want) {
		t.Fatalf("SPECIFICATION.md is stale or hand-edited: regenerated output differs from the committed file.\n" +
			"Run `go generate ./...` and commit the result.")
	}
}
