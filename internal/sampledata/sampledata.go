// Package sampledata embeds a small curated set of StarCraft replays so a
// first-run user can explore every screpdb feature without owning a .rep file.
//
// Five 1v1 progamer ladder games from CWAL.gg:
// Soma (ZvP), Jaedong (ZvT), Light (TvZ), RoyaL (ZvT), Bisu (PvT).
// All are long games chosen for feature coverage (build orders, timings, etc.).
package sampledata

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

//go:embed replays/*.rep
var replaysFS embed.FS

const replaysDir = "replays"

// Extract writes the embedded sample replays into destDir, creating it if
// needed. It is idempotent: files already present (by name) are left untouched.
// Writes go through iofacade so destDir is added to the I/O allow-list.
func Extract(destDir string) error {
	if err := iofacade.AllowDir(destDir); err != nil {
		return fmt.Errorf("allow sample dir: %w", err)
	}
	if err := iofacade.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create sample dir: %w", err)
	}

	entries, err := replaysFS.ReadDir(replaysDir)
	if err != nil {
		return fmt.Errorf("read embedded replays: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		destPath := filepath.Join(destDir, entry.Name())
		if _, statErr := iofacade.Stat(destPath); statErr == nil {
			continue
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", destPath, statErr)
		}

		data, readErr := fs.ReadFile(replaysFS, filepath.ToSlash(filepath.Join(replaysDir, entry.Name())))
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", entry.Name(), readErr)
		}
		if writeErr := iofacade.WriteFile(destPath, data, 0o644); writeErr != nil {
			return fmt.Errorf("write %s: %w", destPath, writeErr)
		}
	}
	return nil
}
