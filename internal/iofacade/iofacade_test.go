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

func TestRemoveAllAndReadDirEnforceRoots(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	allowedDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := Configure(allowedDir); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	sub := filepath.Join(allowedDir, "cache", "v1")
	if err := MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := WriteFile(filepath.Join(sub, "f.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	entries, err := ReadDir(filepath.Join(allowedDir, "cache"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "v1" {
		t.Fatalf("ReadDir inside: entries=%v err=%v", entries, err)
	}
	if err := RemoveAll(filepath.Join(allowedDir, "cache")); err != nil {
		t.Fatalf("RemoveAll inside: %v", err)
	}
	if _, err := Stat(filepath.Join(allowedDir, "cache")); err == nil {
		t.Fatal("RemoveAll left the subtree behind")
	}

	if _, err := ReadDir(outsideDir); !errors.Is(err, ErrForbidden) {
		t.Fatalf("ReadDir outside: want ErrForbidden, got %v", err)
	}
	if err := RemoveAll(outsideDir); !errors.Is(err, ErrForbidden) {
		t.Fatalf("RemoveAll outside: want ErrForbidden, got %v", err)
	}
}

func TestCreateRenameRemoveWalkEnforceRoots(t *testing.T) {
	Reset()
	t.Cleanup(Reset)

	allowedDir := t.TempDir()
	outsideDir := t.TempDir()
	if err := Configure(allowedDir); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	src := filepath.Join(allowedDir, "src.txt")
	f, err := Create(src)
	if err != nil {
		t.Fatalf("Create inside: %v", err)
	}
	if _, err := f.WriteString("ok"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	dst := filepath.Join(allowedDir, "dst.txt")
	if err := Rename(src, dst); err != nil {
		t.Fatalf("Rename inside: %v", err)
	}

	var walked []string
	if err := Walk(allowedDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			walked = append(walked, filepath.Base(path))
		}
		return nil
	}); err != nil {
		t.Fatalf("Walk inside: %v", err)
	}
	if len(walked) != 1 || walked[0] != "dst.txt" {
		t.Fatalf("Walk inside: got %v", walked)
	}

	if err := Remove(dst); err != nil {
		t.Fatalf("Remove inside: %v", err)
	}
	if _, err := Stat(dst); err == nil {
		t.Fatal("Remove left the file behind")
	}

	outside := filepath.Join(outsideDir, "f.txt")
	if _, err := Create(outside); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create outside: want ErrForbidden, got %v", err)
	}
	if err := Rename(outside, filepath.Join(outsideDir, "g.txt")); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Rename outside src: want ErrForbidden, got %v", err)
	}
	if err := Rename(filepath.Join(allowedDir, "x.txt"), outside); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Rename outside dst: want ErrForbidden, got %v", err)
	}
	if err := Remove(outside); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Remove outside: want ErrForbidden, got %v", err)
	}
	if err := Walk(outsideDir, func(string, os.FileInfo, error) error { return nil }); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Walk outside: want ErrForbidden, got %v", err)
	}
}
