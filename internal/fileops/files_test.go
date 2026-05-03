package fileops

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestGetReplayFiles_IgnoresLastReplay(t *testing.T) {
	t.Helper()
	rootDir := t.TempDir()
	lastReplayPath := filepath.Join(rootDir, "LastReplay.rep")
	normalReplayPath := filepath.Join(rootDir, "Game1.rep")
	if err := os.WriteFile(lastReplayPath, []byte("last"), 0o644); err != nil {
		t.Fatalf("write LastReplay.rep: %v", err)
	}
	if err := os.WriteFile(normalReplayPath, []byte("game"), 0o644); err != nil {
		t.Fatalf("write Game1.rep: %v", err)
	}

	files, err := GetReplayFiles(rootDir)
	if err != nil {
		t.Fatalf("GetReplayFiles returned error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 replay file, got %d", len(files))
	}
	if files[0].Name != "Game1.rep" {
		t.Fatalf("expected Game1.rep, got %s", files[0].Name)
	}
}

func TestWalkReplayFiles_NoChecksum(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "a.rep"), []byte("a"), 0o644); err != nil {
		t.Fatalf("write a.rep: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "b.rep"), []byte("b"), 0o644); err != nil {
		t.Fatalf("write b.rep: %v", err)
	}

	files, err := WalkReplayFiles(rootDir)
	if err != nil {
		t.Fatalf("WalkReplayFiles: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	for _, f := range files {
		if f.Checksum != "" {
			t.Fatalf("expected empty Checksum from WalkReplayFiles, got %q for %s", f.Checksum, f.Name)
		}
		if f.Path == "" || f.Name == "" {
			t.Fatalf("expected Path and Name populated, got %+v", f)
		}
	}
}

func TestHashFiles_PopulatesChecksumAndSkipsAlreadyHashed(t *testing.T) {
	rootDir := t.TempDir()
	contents := map[string]string{
		"a.rep": "alpha-content",
		"b.rep": "beta-content",
	}
	expected := make(map[string]string, len(contents))
	for name, body := range contents {
		if err := os.WriteFile(filepath.Join(rootDir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		sum := sha256.Sum256([]byte(body))
		expected[name] = fmt.Sprintf("%x", sum)
	}

	walked, err := WalkReplayFiles(rootDir)
	if err != nil {
		t.Fatalf("WalkReplayFiles: %v", err)
	}

	// Pre-hash one entry; HashFiles must leave it alone.
	const sentinel = "PRESET-SENTINEL-NOT-A-REAL-SHA"
	for i := range walked {
		if walked[i].Name == "a.rep" {
			walked[i].Checksum = sentinel
		}
	}

	hashed, err := HashFiles(context.Background(), walked)
	if err != nil {
		t.Fatalf("HashFiles: %v", err)
	}
	if len(hashed) != len(walked) {
		t.Fatalf("HashFiles changed length: %d → %d", len(walked), len(hashed))
	}

	for _, f := range hashed {
		switch f.Name {
		case "a.rep":
			if f.Checksum != sentinel {
				t.Fatalf("a.rep should have kept preset checksum, got %q", f.Checksum)
			}
		case "b.rep":
			if f.Checksum != expected["b.rep"] {
				t.Fatalf("b.rep checksum mismatch: got %q want %q", f.Checksum, expected["b.rep"])
			}
		default:
			t.Fatalf("unexpected file %s", f.Name)
		}
	}
}

func TestHasReplayFiles_IgnoresLastReplay(t *testing.T) {
	t.Helper()
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "LastReplay.rep"), []byte("last"), 0o644); err != nil {
		t.Fatalf("write LastReplay.rep: %v", err)
	}

	hasReplayFiles, err := HasReplayFiles(rootDir)
	if err != nil {
		t.Fatalf("HasReplayFiles returned error: %v", err)
	}
	if hasReplayFiles {
		t.Fatalf("expected LastReplay.rep to be ignored")
	}
}
