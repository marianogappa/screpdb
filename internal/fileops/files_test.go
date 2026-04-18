package fileops

import (
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
