//go:build windows

package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyBinaryReplacesExistingInstall(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "download.exe")
	dst := filepath.Join(dir, "Programs", "screpdb", installedExeName)
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyBinary(src, dst); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := os.WriteFile(src, []byte("newer"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyBinary(src, dst); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "newer" {
		t.Fatalf("installed copy = %q, want %q", got, "newer")
	}
	if got, _ := os.ReadFile(filepath.Join(filepath.Dir(dst), "."+installedExeName+".old")); string(got) != "new" {
		t.Fatalf("previous copy = %q, want it set aside as .old", got)
	}
}

func TestCreateShortcut(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, installedExeName)
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	lnk := filepath.Join(dir, "screpdb.lnk")
	if err := createShortcut(lnk, target, dir); err != nil {
		t.Fatalf("createShortcut: %v", err)
	}
	info, err := os.Stat(lnk)
	if err != nil || info.Size() == 0 {
		t.Fatalf("shortcut not written: %v", err)
	}
}
