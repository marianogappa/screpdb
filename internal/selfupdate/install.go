package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/marianogappa/screpdb/internal/buildinfo"
)

// Reasons surfaced when self-update is unavailable. They drive the dashboard's
// fallback messaging (e.g. "update via your package manager").
const (
	ReasonNone        = ""
	ReasonDevBuild    = "dev-build"
	ReasonManaged     = "managed"
	ReasonNotWritable = "not-writable"
	ReasonUnsupported = "unsupported-platform"
)

type installInfo struct {
	supported bool
	reason    string
	manager   string // populated when reason == ReasonManaged, e.g. "scoop"
}

// detectInstall decides whether the running binary may swap itself in place.
// It refuses dev builds (no embedded version), package-manager-managed installs
// (so it never clobbers `scoop update`/`brew upgrade`), and read-only install
// directories (which would need elevation).
func detectInstall(self string) installInfo {
	if !parseSemver(buildinfo.Version).ok {
		return installInfo{reason: ReasonDevBuild}
	}
	if mgr := packageManager(self); mgr != "" {
		return installInfo{reason: ReasonManaged, manager: mgr}
	}
	if !dirWritable(filepath.Dir(self)) {
		return installInfo{reason: ReasonNotWritable}
	}
	return installInfo{supported: true}
}

// packageManager returns the name of the package manager that owns this install,
// or "" if the binary looks self-managed. Detection is path-based: each manager
// lays its binaries down under a recognizable directory.
func packageManager(self string) string {
	// Normalize backslashes explicitly: filepath.ToSlash is a no-op on non-Windows
	// hosts, but a self path captured on Windows still uses backslashes.
	lower := strings.ReplaceAll(strings.ToLower(self), "\\", "/")
	switch {
	case strings.Contains(lower, "/scoop/"):
		return "scoop"
	case strings.Contains(lower, "/cellar/"), strings.Contains(lower, "/homebrew/"), strings.Contains(lower, "/linuxbrew/"):
		return "homebrew"
	default:
		return ""
	}
}

// dirWritable reports whether a file can be created in dir, the precondition for
// an atomic in-place swap (the new binary is written alongside the old one).
func dirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".screpdb-write-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// assetName returns the GitHub release asset filename for the given platform and
// build variant, and whether the platform is supported. Only the Windows GUI
// dashboard ships a distinct asset; every other platform uses the root binary.
func assetName(goos, goarch, variant string) (string, bool) {
	switch goos {
	case "windows":
		if goarch != "amd64" {
			return "", false
		}
		if variant == "dashboard" {
			return "screpdb-dashboard-windows-amd64.exe", true
		}
		return "screpdb-windows-amd64.exe", true
	case "linux":
		switch goarch {
		case "amd64":
			return "screpdb-linux-amd64", true
		case "arm64":
			return "screpdb-linux-arm64", true
		}
	case "darwin":
		switch goarch {
		case "amd64":
			return "screpdb-darwin-amd64", true
		case "arm64":
			return "screpdb-darwin-arm64", true
		}
	}
	return "", false
}

func currentAssetName() (string, bool) {
	return assetName(runtime.GOOS, runtime.GOARCH, buildinfo.Variant)
}
