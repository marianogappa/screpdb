package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

func TestPruneGameAssetCache(t *testing.T) {
	root := t.TempDir()
	t.Setenv("SCREPDB_APPDATA_DIR", root)
	iofacade.Reset()
	t.Cleanup(iofacade.Reset)

	mustWrite := func(rel string, size int) string {
		t.Helper()
		path := filepath.Join(root, "game-assets", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	legacyMap := mustWrite("maps/fighting-spirit.png", 100)
	staleIcons := mustWrite("icons/1/marine.png", 100)
	currentMap := mustWrite("maps/v"+gameAssetMapRenderVersion+"/fighting-spirit.jpg", 100)
	currentIcon := mustWrite("icons/"+gameAssetIconRenderVersion+"/marine.png", 100)

	PruneGameAssetCache()

	for _, gone := range []string{legacyMap, filepath.Dir(staleIcons)} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("expected %s pruned, stat err=%v", gone, err)
		}
	}
	for _, kept := range []string{currentMap, currentIcon} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("expected %s kept: %v", kept, err)
		}
	}
}

func TestCapCacheDirSize(t *testing.T) {
	dir := t.TempDir()
	iofacade.Reset()
	t.Cleanup(iofacade.Reset)

	write := func(name string, size int, mtime time.Time) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
		return path
	}

	now := time.Now()
	oldest := write("a.jpg", 40, now.Add(-3*time.Hour))
	middle := write("b.jpg", 40, now.Add(-2*time.Hour))
	newest := write("c.jpg", 40, now.Add(-1*time.Hour))

	capCacheDirSize(dir, 100)

	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Fatalf("expected oldest deleted, stat err=%v", err)
	}
	for _, kept := range []string{middle, newest} {
		if _, err := os.Stat(kept); err != nil {
			t.Fatalf("expected %s kept: %v", kept, err)
		}
	}

	capCacheDirSize(filepath.Join(dir, "missing"), 100)
}
