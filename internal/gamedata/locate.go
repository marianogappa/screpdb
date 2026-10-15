// Package gamedata finds a StarCraft: Remastered install and extracts the classic (SD) data files the
// engine pass needs into the app-data dir, once per SC:R build. Without an install the engine pass is
// off and screpdb runs on the fast pass only.
package gamedata

import (
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/marianogappa/screpdb/internal/casc"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	InstallDirEnv    = "SCREPDB_STARCRAFT_DIR"
	starCraftProduct = "StarCraft1"
)

var ErrNoInstall = errors.New("gamedata: no StarCraft: Remastered install found")

// FindInstall returns the first candidate that holds a StarCraft: Remastered CASC storage, and leaves
// only that folder readable through iofacade.
func FindInstall() (string, error) {
	home, _ := os.UserHomeDir()
	for _, dir := range candidates(runtime.GOOS, os.Getenv, home) {
		if err := iofacade.AllowReadOnlyDir(dir); err != nil {
			continue
		}
		if info, err := casc.ReadBuildInfo(dir); err == nil && info.Product == starCraftProduct {
			return dir, nil
		}
		iofacade.RevokeReadOnlyDir(dir)
	}
	return "", ErrNoInstall
}

func candidates(goos string, getenv func(string) string, home string) []string {
	var dirs []string
	add := func(d string) {
		if d == "" {
			return
		}
		for _, seen := range dirs {
			if seen == d {
				return
			}
		}
		dirs = append(dirs, d)
	}
	add(getenv(InstallDirEnv))
	for _, d := range battleNetInstallPaths(productDBPath(goos, getenv)) {
		add(d)
	}
	switch goos {
	case "linux":
		for _, prefix := range winePrefixes(getenv, home) {
			if prefix == "" {
				continue
			}
			for _, d := range battleNetInstallPaths(filepath.Join(prefix, "drive_c", "ProgramData", "Battle.net", "Agent", "product.db")) {
				add(inPrefix(prefix, d))
			}
			add(filepath.Join(prefix, "drive_c", "Program Files (x86)", "StarCraft"))
			add(filepath.Join(prefix, "drive_c", "Program Files", "StarCraft"))
		}
	case "darwin":
		add("/Applications/StarCraft")
	case "windows":
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if base := getenv(env); base != "" {
				add(filepath.Join(base, "StarCraft"))
			}
		}
	}
	return dirs
}

// winePrefixes lists where Linux players usually run Battle.net: plain Wine, Lutris's Battle.net
// installer and Bottles (native and Flatpak). SCREPDB_STARCRAFT_DIR covers any other place.
func winePrefixes(getenv func(string) string, home string) []string {
	prefixes := []string{getenv("WINEPREFIX")}
	if home == "" {
		return prefixes
	}
	for _, rel := range []string{
		".wine",
		"Games/battlenet",
		"Games/battle-net",
		"Games/starcraft",
		"Games/starcraft-remastered",
		".local/share/bottles/bottles/Battle.net",
		".var/app/com.usebottles.bottles/data/bottles/bottles/Battle.net",
	} {
		prefixes = append(prefixes, filepath.Join(home, filepath.FromSlash(rel)))
	}
	return prefixes
}

// inPrefix maps a Windows path that Battle.net wrote inside a Wine prefix to the Linux path:
// C: is the prefix's drive_c, other letters live under dosdevices.
func inPrefix(prefix, winPath string) string {
	if prefix == "" || len(winPath) < 3 || winPath[1] != ':' {
		return ""
	}
	rest := filepath.FromSlash(strings.ReplaceAll(winPath[3:], "\\", "/"))
	drive := strings.ToLower(winPath[:1])
	if drive == "c" {
		return filepath.Join(prefix, "drive_c", rest)
	}
	return filepath.Join(prefix, "dosdevices", drive+":", rest)
}

func productDBPath(goos string, getenv func(string) string) string {
	switch goos {
	case "darwin":
		return "/Users/Shared/Battle.net/Agent/product.db"
	case "windows":
		if base := getenv("ProgramData"); base != "" {
			return filepath.Join(base, "Battle.net", "Agent", "product.db")
		}
	}
	return ""
}

// battleNetInstallPaths reads Battle.net's product.db, a protobuf that holds each installed game's
// folder as a length-prefixed string; every absolute path in it is a candidate that FindInstall checks.
func battleNetInstallPaths(productDB string) []string {
	if productDB == "" || iofacade.AllowReadOnlyDir(productDB) != nil {
		return nil
	}
	defer iofacade.RevokeReadOnlyDir(productDB)
	b, err := iofacade.ReadFile(productDB)
	if err != nil {
		return nil
	}
	return installPathsIn(b)
}

func installPathsIn(b []byte) []string {
	var paths []string
	for i := range b {
		n, size := binary.Uvarint(b[i:])
		if size <= 0 || n < 3 || n > 1024 || i+size+int(n) > len(b) {
			continue
		}
		if s := string(b[i+size : i+size+int(n)]); isAbsolutePath(s) {
			paths = append(paths, s)
		}
	}
	return paths
}

func isAbsolutePath(s string) bool {
	for _, c := range s {
		if c < 0x20 || c > 0x7e {
			return false
		}
	}
	return s[0] == '/' || len(s) > 3 && (s[1] == ':' && (s[2] == '\\' || s[2] == '/'))
}
