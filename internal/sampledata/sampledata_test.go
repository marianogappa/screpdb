package sampledata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractWritesAllReplays(t *testing.T) {
	dir := t.TempDir()
	if err := Extract(dir); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.rep"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 5 {
		t.Fatalf("expected 5 sample replays, got %d: %v", len(matches), matches)
	}
	for _, m := range matches {
		info, statErr := os.Stat(m)
		if statErr != nil || info.Size() == 0 {
			t.Fatalf("sample replay %s missing or empty (err=%v)", m, statErr)
		}
	}
}

func TestExtractIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := Extract(dir); err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	sample := filepath.Join(dir, "01_zvt_zergling_rush.rep")
	before, err := os.Stat(sample)
	if err != nil {
		t.Fatal(err)
	}

	if err := Extract(dir); err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	after, err := os.Stat(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("Extract rewrote existing file: modtime changed %v -> %v", before.ModTime(), after.ModTime())
	}
}
