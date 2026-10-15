package gamedata

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/casc"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	DirName      = "bwdata"
	manifestName = "manifest.json"
	imagesTable  = "arr/images.tbl"
)

type Manifest struct {
	BuildKey  string `json:"build_key"`
	BuildName string `json:"build_name"`
	Files     int    `json:"files"`
}

// Prepare finds the install and returns the app-data folder that holds its game data, extracting it
// first when the folder is missing or from another SC:R build. It returns ErrNoInstall without one.
func Prepare() (string, Manifest, error) {
	install, err := FindInstall()
	if err != nil {
		return "", Manifest{}, err
	}
	dir, err := appdata.Path(DirName)
	if err != nil {
		return "", Manifest{}, err
	}
	m, err := Ensure(install, dir)
	return dir, m, err
}

// Ensure makes dataDir hold the classic files of installDir's active build. A complete extract is
// renamed into place only after its manifest is written, so a crash never leaves a half-filled dataDir.
func Ensure(installDir, dataDir string) (Manifest, error) {
	info, err := casc.ReadBuildInfo(installDir)
	if err != nil {
		return Manifest{}, err
	}
	if m, err := readManifest(dataDir); err == nil && m.BuildKey == info.Key {
		return m, nil
	}
	tmp := dataDir + ".tmp"
	if err := iofacade.RemoveAll(tmp); err != nil {
		return Manifest{}, err
	}
	m, err := extract(installDir, tmp)
	if err != nil {
		_ = iofacade.RemoveAll(tmp)
		return Manifest{}, err
	}
	if err := iofacade.RemoveAll(dataDir); err != nil {
		return Manifest{}, err
	}
	if err := iofacade.Rename(tmp, dataDir); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func readManifest(dataDir string) (Manifest, error) {
	var m Manifest
	b, err := iofacade.ReadFile(filepath.Join(dataDir, manifestName))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	if m.BuildKey == "" || m.Files == 0 {
		return m, errors.New("gamedata: incomplete manifest")
	}
	return m, nil
}

func extract(installDir, outDir string) (Manifest, error) {
	s, err := casc.Open(installDir)
	if err != nil {
		return Manifest{}, err
	}
	defer s.Close()
	names, err := fileNames(s)
	if err != nil {
		return Manifest{}, err
	}
	for _, name := range names {
		b, err := s.ReadFile(name)
		if err != nil {
			return Manifest{}, err
		}
		path := filepath.Join(outDir, filepath.FromSlash(name))
		if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return Manifest{}, err
		}
		if err := iofacade.WriteFile(path, b, 0o644); err != nil {
			return Manifest{}, err
		}
	}
	m := Manifest{BuildKey: s.BuildKey, BuildName: s.BuildName, Files: len(names)}
	b, err := json.Marshal(m)
	if err != nil {
		return Manifest{}, err
	}
	return m, iofacade.WriteFile(filepath.Join(outDir, manifestName), b, 0o644)
}

var (
	datFiles = []string{"units", "weapons", "upgrades", "techdata", "flingy", "sprites", "images", "orders"}
	tilesets = []string{"badlands", "platform", "install", "ashworld", "jungle", "desert", "ice", "twilight"}
)

// fileNames lists what openbw-go loads, lowercase: the fixed tables plus every unit/ file images.tbl names.
func fileNames(s *casc.Storage) ([]string, error) {
	var names []string
	for _, d := range datFiles {
		names = append(names, "arr/"+d+".dat")
	}
	names = append(names, imagesTable, "scripts/iscript.bin", "triggers/melee.trg")
	for _, t := range tilesets {
		names = append(names, "tileset/"+t+".cv5", "tileset/"+t+".vf4")
	}
	tbl, err := s.ReadFile(imagesTable)
	if err != nil {
		return nil, err
	}
	entries, err := parseTable(tbl)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := "unit/" + strings.ToLower(strings.ReplaceAll(e, "\\", "/"))
		if e == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names, nil
}

// parseTable reads a .tbl string table: a uint16 count, count uint16 offsets, NUL-terminated strings.
func parseTable(b []byte) ([]string, error) {
	if len(b) < 2 {
		return nil, fmt.Errorf("gamedata: %s is empty", imagesTable)
	}
	count := int(binary.LittleEndian.Uint16(b))
	if 2+2*count > len(b) {
		return nil, fmt.Errorf("gamedata: %s is truncated", imagesTable)
	}
	out := make([]string, 0, count)
	for i := range count {
		off := int(binary.LittleEndian.Uint16(b[2+2*i:]))
		if off >= len(b) {
			return nil, fmt.Errorf("gamedata: %s entry %d is out of range", imagesTable, i)
		}
		end := off
		for end < len(b) && b[end] != 0 {
			end++
		}
		out = append(out, string(b[off:end]))
	}
	return out, nil
}
