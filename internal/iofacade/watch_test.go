//go:build !darwin

package iofacade

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestDirWatcherDeliversEventsInsideAllowlist(t *testing.T) {
	Reset()
	t.Cleanup(Reset)
	dir := t.TempDir()
	if err := AllowDir(dir); err != nil {
		t.Fatal(err)
	}
	w, err := NewDirWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := w.Add(dir); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := w.Add(filepath.Join(t.TempDir(), "outside")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Add outside allowlist: got %v, want ErrForbidden", err)
	}
	target := filepath.Join(dir, "game.rep")
	if err := WriteFile(target, []byte("rep"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(3 * time.Second)
	for {
		select {
		case ev := <-w.Events():
			if ev.Path == target && (ev.Op == FSCreate || ev.Op == FSWrite) {
				return
			}
		case err := <-w.Errors():
			t.Fatalf("watcher error: %v", err)
		case <-deadline:
			t.Fatal("no event for created file")
		}
	}
}

func TestFSOpString(t *testing.T) {
	for op, want := range map[FSOp]string{FSCreate: "create", FSWrite: "write", FSRemove: "remove", FSRename: "rename", FSOp(9): "unknown"} {
		if got := op.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", op, got, want)
		}
	}
}
