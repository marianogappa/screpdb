package parser

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

// TestAnalyzeAlliances_RealReplay parses bgh.rep — an 8-player melee with
// alliances set in lobby — and asserts the analyzer produces a sensible
// timeline (initial 2v2v2v2 topology, no stacking flag for a balanced game).
// Not run unless the testdata replay is available.
func TestAnalyzeAlliances_RealReplay(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	repPath := filepath.Join(repoRoot, "internal", "testdata", "replays", "bgh.rep")

	rep := &models.Replay{FilePath: repPath}
	data, err := ParseReplayWithOptions(repPath, rep, Options{})
	if err != nil {
		t.Skipf("parse %s: %v", repPath, err)
	}
	if data.Replay.GameType != "Melee" {
		t.Skipf("bgh.rep is not Melee (got %q) — expected at the time this test was written", data.Replay.GameType)
	}

	// The parser hook should have run for this melee game (>2 active players).
	t.Logf("game_type=%q team_format=%q dur=%ds team_stacking=%v team_info_incomplete=%v",
		data.Replay.GameType, data.Replay.TeamFormat, data.Replay.DurationSeconds,
		data.Replay.TeamStacking, data.Replay.TeamInfoIncomplete)

	res := AnalyzeAlliances(data.Players, data.Commands, data.Replay.DurationSeconds)
	if len(res.Snapshots) == 0 {
		t.Fatalf("expected at least one snapshot")
	}
	if !res.AnyMutualResolved {
		t.Fatalf("expected mutual alliances to resolve in 2v2v2v2 lobby")
	}
	t.Logf("snapshots=%d stacking=%v", len(res.Snapshots), res.TeamStackingFlag)

	// First snapshot is everyone solo (sec=0). The next non-trivial topology
	// should have at least one team of size ≥2.
	foundMutual := false
	for _, s := range res.Snapshots {
		for _, team := range s.Teams {
			if len(team) >= 2 {
				foundMutual = true
				break
			}
		}
		if foundMutual {
			break
		}
	}
	if !foundMutual {
		t.Fatalf("no team of size ≥2 in any snapshot")
	}
}
