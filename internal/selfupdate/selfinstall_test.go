//go:build !js

package selfupdate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marianogappa/screpdb/internal/buildinfo"
)

func TestShouldSelfInstall(t *testing.T) {
	prev := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = prev })

	downloads := filepath.FromSlash("/home/u/Downloads")
	install := filepath.FromSlash("/home/u/AppData/Local/Programs/screpdb")
	p := filepath.FromSlash

	cases := []struct {
		name      string
		version   string
		self      string
		downloads string
		want      bool
	}{
		{"browser download", "v1.31.0", p("/home/u/Downloads/screpdb-gui-windows-amd64.exe"), downloads, true},
		{"renamed duplicate download", "v1.31.0", p("/home/u/Downloads/screpdb-gui-windows-amd64 (1).exe"), downloads, true},
		{"download folder case differs", "v1.31.0", p("/home/u/downloads/screpdb.exe"), downloads, true},
		{"already installed", "v1.31.0", p("/home/u/AppData/Local/Programs/screpdb/screpdb.exe"), downloads, false},
		{"placed elsewhere on purpose", "v1.31.0", p("/home/u/Tools/screpdb.exe"), downloads, false},
		{"sibling folder sharing a prefix", "v1.31.0", p("/home/u/Downloads-old/screpdb.exe"), downloads, false},
		{"scoop install", "v1.31.0", p("/home/u/Downloads/scoop/apps/screpdb/screpdb.exe"), downloads, false},
		{"dev build", "dev", p("/home/u/Downloads/screpdb.exe"), downloads, false},
		{"unknown downloads folder", "v1.31.0", p("/home/u/Downloads/screpdb.exe"), "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buildinfo.Version = c.version
			if got := shouldSelfInstall(c.self, c.downloads, install); got != c.want {
				t.Fatalf("shouldSelfInstall(%q) = %v, want %v", c.self, got, c.want)
			}
		})
	}
}

func TestTakeMovedDownloadClearsMarker(t *testing.T) {
	t.Setenv(movedFromEnv, "C:/Users/u/Downloads/screpdb.exe")
	if got := TakeMovedDownload(); got != "C:/Users/u/Downloads/screpdb.exe" {
		t.Fatalf("got %q", got)
	}
	if v, ok := os.LookupEnv(movedFromEnv); ok {
		t.Fatalf("marker still set to %q", v)
	}
}

func TestRemoveMovedDownload(t *testing.T) {
	f := filepath.Join(t.TempDir(), "screpdb.exe")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	RemoveMovedDownload(f)
	if _, err := os.Stat(f); !os.IsNotExist(err) {
		t.Fatalf("download still present: %v", err)
	}
	RemoveMovedDownload(f)
}
