// Package appdata resolves screpdb's single per-OS application-data root — the
// one directory all of screpdb's own writes are consolidated under (issue #237).
//
// Consolidating writes here is the prerequisite for the Windows Low-integrity
// sandbox: the launcher marks this one directory Low-writable so the Low worker
// can write the database, cache, logs, crash reports, and sample replays while
// every other write (e.g. from a compromised parser) is blocked by the OS.
//
// The root is:
//   - Windows: %LOCALAPPDATA%\screpdb          (Local, not Roaming)
//   - macOS:   ~/Library/Application Support/screpdb
//   - Linux:   $XDG_CONFIG_HOME/screpdb or ~/.config/screpdb
//
// The SCREPDB_APPDATA_DIR environment variable overrides the whole resolution
// (test seam and portable-install hook).
package appdata

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	// appDirName is the leaf directory created under the per-OS base.
	appDirName = "screpdb"

	// OverrideEnv, when set, replaces the resolved root entirely.
	OverrideEnv = "SCREPDB_APPDATA_DIR"

	// dbFileName is the default SQLite database filename inside the root.
	dbFileName = "screp.db"
)

// resolveBase returns the per-OS base directory that screpdb's app-data root
// lives under, given the current GOOS and an environment lookup. It is pure
// (no filesystem access) so it can be unit-tested across platforms.
//
// os.UserConfigDir would be wrong on Windows: it returns Roaming %AppData%, but
// containment wants Local. We branch explicitly and use %LOCALAPPDATA% there.
func resolveBase(goos string, getenv func(string) string, userConfigDir func() (string, error)) (string, error) {
	if goos == "windows" {
		if local := strings.TrimSpace(getenv("LOCALAPPDATA")); local != "" {
			return local, nil
		}
		// Fall back to the OS cache dir, which is also under Local on Windows.
		return os.UserCacheDir()
	}
	return userConfigDir()
}

// root computes the absolute app-data root without creating it.
func root() (string, error) {
	if override := strings.TrimSpace(os.Getenv(OverrideEnv)); override != "" {
		return filepath.Abs(override)
	}
	base, err := resolveBase(runtime.GOOS, os.Getenv, os.UserConfigDir)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// Dir returns the app-data root, creating it if needed and registering it as a
// permitted iofacade root. This is the single grantable write root for screpdb.
func Dir() (string, error) {
	dir, err := root()
	if err != nil {
		return "", err
	}
	if err := iofacade.AllowDir(dir); err != nil {
		return "", err
	}
	if err := iofacade.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Path joins sub onto the app-data root (creating and registering the root),
// returning the absolute path. It does not create sub itself.
func Path(sub ...string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(append([]string{dir}, sub...)...), nil
}

// DefaultDBPath returns the default SQLite database path inside the app-data
// root.
func DefaultDBPath() (string, error) {
	return Path(dbFileName)
}

// ResolveDBPath maps the sqlite-path flag to a concrete path: the sentinel
// default (the bare filename) resolves to the app-data root, while any explicit
// user-provided path is honored as-is.
func ResolveDBPath(flag string) (string, error) {
	trimmed := strings.TrimSpace(flag)
	if trimmed == "" || trimmed == dbFileName {
		return DefaultDBPath()
	}
	return flag, nil
}
