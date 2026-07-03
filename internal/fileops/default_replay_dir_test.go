package fileops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

func TestResolveDefaultReplayDir_ContractPerPlatform(t *testing.T) {
	// ResolveDefaultReplayDir calls iofacade.AllowDir on any candidate it finds,
	// which flips the process-global facade into enforcing mode. Reset afterward
	// so the rest of the suite keeps running in permissive mode.
	t.Cleanup(iofacade.Reset)
	dir, err := ResolveDefaultReplayDir()
	if err != nil {
		if !errors.Is(err, errDefaultReplayDirNotFound) {
			t.Fatalf("expected errDefaultReplayDirNotFound, got %v", err)
		}
		if dir != "" {
			t.Fatalf("expected empty dir on error, got %q", dir)
		}
		return
	}
	// If a real default replay dir was found, it must satisfy the same contract
	// ResolveDefaultReplayDir enforces internally.
	if verr := ValidateReplayDir(dir); verr != nil {
		t.Fatalf("resolved dir %q failed ValidateReplayDir: %v", dir, verr)
	}
}

func TestGetDefaultReplayDir_MatchesResolve(t *testing.T) {
	t.Cleanup(iofacade.Reset)
	got := GetDefaultReplayDir()
	dir, err := ResolveDefaultReplayDir()
	if err != nil {
		if got != "" {
			t.Fatalf("GetDefaultReplayDir = %q, want empty when Resolve errors", got)
		}
		return
	}
	if got != dir {
		t.Fatalf("GetDefaultReplayDir = %q, ResolveDefaultReplayDir = %q; want equal", got, dir)
	}
}

func TestStrategyMacUser_Contract(t *testing.T) {
	dir, ok, err := strategyMacUser()()
	if runtime.GOOS == "windows" {
		if ok || err != nil || dir != "" {
			t.Fatalf("on windows strategyMacUser must be a no-op, got (%q, %v, %v)", dir, ok, err)
		}
		return
	}
	if err != nil {
		t.Fatalf("strategyMacUser returned error: %v", err)
	}
	if !ok {
		t.Fatalf("strategyMacUser should report ok on non-windows")
	}
	const suffix = "Library/Application Support/Blizzard/StarCraft/Maps/Replays"
	if !strings.HasSuffix(dir, suffix) {
		t.Fatalf("dir %q does not end with %q", dir, suffix)
	}
	if !filepath.IsAbs(dir) {
		t.Fatalf("expected absolute path, got %q", dir)
	}
}

func TestWindowsOnlyStrategies_NoOpOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this test asserts the non-windows no-op branch")
	}
	strategies := map[string]findReplayDirStrategy{
		"strategyWindowsUser":    strategyWindowsUser(),
		"strategyOneDriveUser":   strategyOneDriveUser(),
		"strategyWindowsUserOld": strategyWindowsUserOld(),
	}
	for name, strategy := range strategies {
		t.Run(name, func(t *testing.T) {
			dir, ok, err := strategy()
			if err != nil {
				t.Fatalf("%s returned error on non-windows: %v", name, err)
			}
			if ok {
				t.Fatalf("%s should report not-ok on non-windows", name)
			}
			if dir != "" {
				t.Fatalf("%s should return empty dir on non-windows, got %q", name, dir)
			}
		})
	}
}

func TestHashFiles_ErrorOnUnreadableEntry(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good.rep")
	if err := os.WriteFile(good, []byte("good"), 0o644); err != nil {
		t.Fatalf("write good.rep: %v", err)
	}

	files := []FileInfo{
		{Path: good, Name: "good.rep"},
		{Path: filepath.Join(root, "missing.rep"), Name: "missing.rep"},
	}

	_, err := HashFiles(context.Background(), files)
	if err == nil {
		t.Fatalf("expected error hashing a missing file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to calculate checksum") {
		t.Fatalf("error %q does not mention checksum failure", err.Error())
	}
}

func TestHashFiles_EmptyInputNoError(t *testing.T) {
	out, err := HashFiles(context.Background(), nil)
	if err != nil {
		t.Fatalf("HashFiles(nil) error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty output, got %d entries", len(out))
	}
}

func TestHashFiles_CancelledContext(t *testing.T) {
	root := t.TempDir()
	var files []FileInfo
	for _, name := range []string{"a.rep", "b.rep", "c.rep"} {
		p := filepath.Join(root, name)
		if err := os.WriteFile(p, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		files = append(files, FileInfo{Path: p, Name: name})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// A pre-cancelled context must not yield partially-hashed successful output;
	// either every checksum is computed before cancellation is observed, or the
	// call reports the cancellation error.
	out, err := HashFiles(ctx, files)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
		return
	}
	for _, f := range out {
		if f.Checksum == "" {
			t.Fatalf("no error returned but %s has empty checksum", f.Name)
		}
	}
}
