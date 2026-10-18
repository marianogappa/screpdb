package casc

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

// activeBuildKey reads .build.info: a "|"-separated table whose header names each column "Name!TYPE:size".
func activeBuildKey(path string) (string, error) {
	b, err := iofacade.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("casc: %w", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("casc: %s has no build rows", path)
	}
	columns := map[string]int{}
	for i, h := range strings.Split(lines[0], "|") {
		name, _, _ := strings.Cut(h, "!")
		columns[name] = i
	}
	keyCol, ok := columns["Build Key"]
	if !ok {
		return "", fmt.Errorf("casc: %s has no Build Key column", path)
	}
	activeCol, hasActive := columns["Active"]
	for _, line := range lines[1:] {
		fields := strings.Split(line, "|")
		if keyCol >= len(fields) || fields[keyCol] == "" {
			continue
		}
		if hasActive && activeCol < len(fields) && fields[activeCol] != "1" {
			continue
		}
		return fields[keyCol], nil
	}
	return "", fmt.Errorf("casc: %s has no active build", path)
}

func configPath(dataDir, hexKey string) string {
	return filepath.Join(dataDir, "config", hexKey[0:2], hexKey[2:4], hexKey)
}

// readBuildConfig returns the first value of each "name = value ..." line.
func readBuildConfig(dataDir, buildKey string) (map[string]string, error) {
	if len(buildKey) != 32 {
		return nil, fmt.Errorf("casc: bad build key %q", buildKey)
	}
	b, err := iofacade.ReadFile(configPath(dataDir, buildKey))
	if err != nil {
		return nil, fmt.Errorf("casc: build config: %w", err)
	}
	config := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		name, value, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok || strings.HasPrefix(name, "#") {
			continue
		}
		config[name] = value
	}
	for _, required := range []string{"root", "encoding"} {
		if config[required] == "" {
			return nil, fmt.Errorf("casc: build config has no %s", required)
		}
	}
	return config, nil
}

type BuildInfo struct {
	Key     string
	Name    string
	Product string
}

// ReadBuildInfo reads only the install's active build, without opening the storage.
func ReadBuildInfo(installDir string) (BuildInfo, error) {
	buildKey, err := activeBuildKey(filepath.Join(installDir, ".build.info"))
	if err != nil {
		return BuildInfo{}, err
	}
	config, err := readBuildConfig(filepath.Join(installDir, "Data"), buildKey)
	if err != nil {
		return BuildInfo{}, err
	}
	return BuildInfo{Key: buildKey, Name: config["build-name"], Product: config["build-product"]}, nil
}
