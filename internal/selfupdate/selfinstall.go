//go:build !js

package selfupdate

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/buildinfo"
)

// movedFromEnv carries the path of the browser-downloaded copy to the installed
// copy it launched, so the installed copy can delete the download once the
// original process has exited and released its file lock.
const movedFromEnv = "SCREPDB_MOVED_FROM"

// installedExeName is the file name the PowerShell installer also uses, so both
// install routes land on the same binary and Start Menu shortcut.
const installedExeName = "screpdb.exe"

// shouldSelfInstall reports whether the running binary is a browser download
// that should move itself into the install dir. Anything outside the Downloads
// folder was placed deliberately and is left alone.
func shouldSelfInstall(self, downloadsDir, installDir string) bool {
	if downloadsDir == "" || installDir == "" {
		return false
	}
	if !parseSemver(buildinfo.Version).ok || packageManager(self) != "" {
		return false
	}
	if samePath(filepath.Dir(self), installDir) {
		return false
	}
	return isWithin(self, downloadsDir)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func isWithin(path, dir string) bool {
	rel, err := filepath.Rel(strings.ToLower(filepath.Clean(dir)), strings.ToLower(filepath.Clean(path)))
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

// TakeMovedDownload returns the browser-downloaded copy this process was moved
// from, if any, and clears the marker so child processes don't inherit it.
func TakeMovedDownload() string {
	src := os.Getenv(movedFromEnv)
	_ = os.Unsetenv(movedFromEnv)
	return src
}

// RemoveMovedDownload deletes src, retrying while the original process is still
// exiting and holding its lock.
func RemoveMovedDownload(src string) {
	for range 20 {
		if err := os.Remove(src); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
}
