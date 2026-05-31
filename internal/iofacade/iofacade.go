// Package iofacade is the single sanctioned entry point for real filesystem
// access in screpdb. Every os/filepath/ioutil file operation in the shipped
// codebase must go through this package; the enforcement test in
// internal/iofacade/enforcement_test.go fails the build if any package bypasses
// it.
//
// The facade permits I/O only inside an allowlist of roots registered at
// startup (see issue #135):
//
//   - the current working directory — the SQLite database (and its -wal/-shm
//     siblings) plus opt-in debug artifacts live here;
//   - the replays folder — read replays and write "watch me" replays;
//   - the OS user-cache dir — cached game-asset PNGs.
//
// Until at least one root is registered the facade is permissive, so unit tests
// and pre-config bootstrap code keep working. Once any root is registered, every
// path is checked against the allowlist and out-of-bounds access returns
// ErrForbidden.
//
// This is a best-effort, in-process guard, not a sandbox: paths handed to
// trusted third-party dependencies (the SQLite driver, the screp parser,
// scmapanalyzer) are opened inside those libraries and are not — and cannot be —
// routed through here. The attack surface is reduced by keeping our own I/O
// behind this chokepoint and by minimizing dependencies, per #135.
package iofacade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrForbidden is returned (wrapped) when a path falls outside every permitted root.
var ErrForbidden = errors.New("iofacade: path outside permitted roots")

var (
	mu      sync.RWMutex
	allowed []string // absolute, cleaned permitted roots
)

// AllowDir registers dir and its subtree as a permitted root. Calling it with a
// path already covered is a no-op, as is an empty dir. Registering the first
// root flips the facade from permissive to enforcing.
func AllowDir(dir string) error {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	abs = filepath.Clean(abs)
	mu.Lock()
	defer mu.Unlock()
	for _, r := range allowed {
		if r == abs {
			return nil
		}
	}
	allowed = append(allowed, abs)
	return nil
}

// Configure replaces the allowlist with the given roots. Empty entries are
// skipped. Primarily used by command entrypoints and tests.
func Configure(dirs ...string) error {
	mu.Lock()
	allowed = nil
	mu.Unlock()
	for _, d := range dirs {
		if err := AllowDir(d); err != nil {
			return err
		}
	}
	return nil
}

// Reset clears the allowlist, returning the facade to permissive mode. For tests.
func Reset() {
	mu.Lock()
	allowed = nil
	mu.Unlock()
}

// resolve cleans path to an absolute form and verifies it sits within a
// permitted root (or that the facade is still permissive).
func resolve(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)

	mu.RLock()
	defer mu.RUnlock()
	if len(allowed) == 0 {
		return abs, nil // permissive until the first root is registered
	}
	for _, r := range allowed {
		if abs == r || strings.HasPrefix(abs, r+string(os.PathSeparator)) {
			return abs, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrForbidden, abs)
}

// Open opens a permitted path for reading.
func Open(path string) (*os.File, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

// Create creates/truncates a permitted path for writing.
func Create(path string) (*os.File, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, err
	}
	return os.Create(p)
}

// ReadFile reads the contents of a permitted file.
func ReadFile(path string) ([]byte, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

// WriteFile writes data to a permitted path.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	p, err := resolve(path)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, perm)
}

// MkdirAll creates a permitted directory tree.
func MkdirAll(path string, perm os.FileMode) error {
	p, err := resolve(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(p, perm)
}

// Stat stats a permitted path.
func Stat(path string) (os.FileInfo, error) {
	p, err := resolve(path)
	if err != nil {
		return nil, err
	}
	return os.Stat(p)
}

// Rename moves oldPath to newPath; both must be permitted.
func Rename(oldPath, newPath string) error {
	op, err := resolve(oldPath)
	if err != nil {
		return err
	}
	np, err := resolve(newPath)
	if err != nil {
		return err
	}
	return os.Rename(op, np)
}

// Remove deletes a permitted path.
func Remove(path string) error {
	p, err := resolve(path)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Walk walks the file tree rooted at a permitted directory. The whole subtree
// is implicitly permitted because it lives under root.
func Walk(root string, fn filepath.WalkFunc) error {
	r, err := resolve(root)
	if err != nil {
		return err
	}
	return filepath.Walk(r, fn)
}

// FindAndReadAncestorFile walks up from startDir (inclusive) through up to
// maxLevels parent directories, returning the path and contents of the first
// file named name that it finds. It is an explicit, read-only exception to the
// root allowlist: StarCraft stores CSettings.json in an ancestor of the replays
// folder, so this lookup is intentionally allowed to read above the permitted
// roots. Returns ("", nil, nil) when no such file exists.
func FindAndReadAncestorFile(startDir, name string, maxLevels int) (string, []byte, error) {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" || strings.TrimSpace(name) == "" {
		return "", nil, nil
	}
	cur := filepath.Clean(startDir)
	seen := map[string]struct{}{}
	for i := 0; i < maxLevels; i++ {
		if _, dup := seen[cur]; !dup {
			seen[cur] = struct{}{}
			candidate := filepath.Join(cur, name)
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				data, readErr := os.ReadFile(candidate)
				return candidate, data, readErr
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return "", nil, nil
}
