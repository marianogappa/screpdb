package iofacade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPermissiveUntilConfigured(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	// No roots registered yet: the facade is permissive.
	if err := WriteFile(path, []byte("hi"), 0o644); err != nil {
		t.Fatalf("permissive WriteFile: %v", err)
	}
	if _, err := ReadFile(path); err != nil {
		t.Fatalf("permissive ReadFile: %v", err)
	}
}

func TestEnforcesAllowedRoots(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	allowedDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := Configure(allowedDir); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// Inside the permitted root: allowed.
	inside := filepath.Join(allowedDir, "sub", "f.txt")
	if err := MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatalf("MkdirAll inside: %v", err)
	}
	if err := WriteFile(inside, []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile inside: %v", err)
	}

	// Outside every permitted root: rejected with ErrForbidden.
	outside := filepath.Join(outsideDir, "f.txt")
	err := WriteFile(outside, []byte("nope"), 0o644)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("WriteFile outside: want ErrForbidden, got %v", err)
	}
	if _, err := ReadFile(outside); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ReadFile outside: want ErrForbidden, got %v", err)
	}
	if _, err := Open(outside); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Open outside: want ErrForbidden, got %v", err)
	}
}

func TestRejectsSiblingPrefixEscape(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	base := t.TempDir()
	allowedDir := filepath.Join(base, "replays")
	sibling := filepath.Join(base, "replays-evil")
	if err := os.MkdirAll(allowedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Configure(allowedDir); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// "replays-evil" shares a string prefix with "replays" but is NOT inside it.
	if err := WriteFile(filepath.Join(sibling, "f.txt"), []byte("x"), 0o644); !errors.Is(err, ErrForbidden) {
		t.Fatalf("sibling prefix escape: want ErrForbidden, got %v", err)
	}
}

func TestFindAndReadAncestorFile(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	base := t.TempDir()
	deep := filepath.Join(base, "StarCraft", "Maps", "Replays")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "StarCraft", "CSettings.json")
	if err := os.WriteFile(want, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Enforce a narrow root that does NOT include the ancestor; the ancestor
	// read is a sanctioned exception and must still succeed.
	if err := Configure(deep); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	got, data, err := FindAndReadAncestorFile(deep, "CSettings.json", 20)
	if err != nil {
		t.Fatalf("FindAndReadAncestorFile: %v", err)
	}
	if got != want {
		t.Fatalf("path: want %s, got %s", want, got)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("data: got %q", string(data))
	}

	// Missing file returns empty, no error.
	if p, _, err := FindAndReadAncestorFile(deep, "DoesNotExist.json", 20); p != "" || err != nil {
		t.Fatalf("missing file: want empty/nil, got %q/%v", p, err)
	}
}
