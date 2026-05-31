// Command genspec regenerates SPECIFICATION.md at the repo root from the Go
// source of truth. Invoked by `go generate ./...` (see ../../generate.go) and
// the Makefile `spec-generate` target.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/marianogappa/screpdb/internal/spec"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fail(err)
	}

	doc, err := spec.Generate()
	if err != nil {
		fail(err)
	}

	out := filepath.Join(root, "SPECIFICATION.md")
	if err := os.WriteFile(out, doc, 0o644); err != nil {
		fail(err)
	}
}

// findRepoRoot walks up from the current working directory to the directory
// containing go.mod. This makes the tool robust under `go generate ./...`, where
// the working directory is the package dir, not the repo root.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "genspec: %v\n", err)
	os.Exit(1)
}
