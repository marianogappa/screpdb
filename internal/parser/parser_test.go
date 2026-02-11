package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

type replayGolden struct {
	File     string        `json:"file"`
	Players  int           `json:"players"`
	Commands int           `json:"commands"`
	Patterns patternCounts `json:"patterns"`
}

type patternCounts struct {
	Replay int `json:"replay"`
	Team   int `json:"team"`
	Player int `json:"player"`
}

type goldenFile struct {
	Replays []replayGolden `json:"replays"`
}

func TestParserGolden(t *testing.T) {
	replayDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}

	actual, err := buildGoldenFromDir(replayDir)
	if err != nil {
		t.Fatalf("buildGoldenFromDir: %v", err)
	}

	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeGolden(actual); err != nil {
			t.Fatalf("writeGolden: %v", err)
		}
	}

	expected, err := readGolden()
	if err != nil {
		t.Fatalf("readGolden: %v", err)
	}

	normalizeGolden(expected)
	normalizeGolden(actual)

	if diff := compareGolden(expected, actual); diff != "" {
		t.Fatalf("golden mismatch: %s", diff)
	}
}

func buildGoldenFromDir(replayDir string) (*goldenFile, error) {
	files, err := fileops.GetReplayFiles(replayDir)
	if err != nil {
		return nil, err
	}

	result := &goldenFile{Replays: make([]replayGolden, 0, len(files))}
	for _, file := range files {
		replay := CreateReplayFromFileInfo(file.Path, file.Name, file.Size, file.Checksum)
		data, err := ParseReplay(file.Path, replay)
		if err != nil {
			return nil, err
		}

		counts := patternCounts{}
		if orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator); ok {
			for _, res := range orch.GetResults() {
				switch res.Level {
				case core.LevelReplay:
					counts.Replay++
				case core.LevelTeam:
					counts.Team++
				case core.LevelPlayer:
					counts.Player++
				}
			}
		}

		result.Replays = append(result.Replays, replayGolden{
			File:     file.Name,
			Players:  len(data.Players),
			Commands: len(data.Commands),
			Patterns: counts,
		})
	}

	return result, nil
}

func writeGolden(golden *goldenFile) error {
	normalizeGolden(golden)
	payload, err := json.MarshalIndent(golden, "", "  ")
	if err != nil {
		return err
	}

	path, err := goldenFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func readGolden() (*goldenFile, error) {
	path, err := goldenFilePath()
	if err != nil {
		return nil, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var golden goldenFile
	if err := json.Unmarshal(payload, &golden); err != nil {
		return nil, err
	}
	return &golden, nil
}

func normalizeGolden(golden *goldenFile) {
	sort.Slice(golden.Replays, func(i, j int) bool {
		return golden.Replays[i].File < golden.Replays[j].File
	})
}

func goldenFilePath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	baseDir := filepath.Dir(thisFile)
	return filepath.Join(baseDir, "..", "testdata", "replays", "golden.json"), nil
}

func compareGolden(expected, actual *goldenFile) string {
	if len(expected.Replays) != len(actual.Replays) {
		return "replay count mismatch"
	}
	for i := range expected.Replays {
		exp := expected.Replays[i]
		act := actual.Replays[i]
		if exp != act {
			return "replay mismatch for " + exp.File
		}
	}
	return ""
}

func resolveReplayDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	baseDir := filepath.Dir(thisFile)
	candidates := []string{
		filepath.Join(baseDir, "..", "testdata", "replays"),
		filepath.Join(baseDir, "..", "..", "testutils", "replays"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}
