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
	for _, dir := range candidates(runtime.GOOS, os.Getenv) {
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

func candidates(goos string, getenv func(string) string) []string {
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
