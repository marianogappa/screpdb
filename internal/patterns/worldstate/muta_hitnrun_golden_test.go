package worldstate_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
)

// Golden test for the conservative Mutalisk hit-and-run confidence flag (#194).
// Runs the muta_*.rep fixtures through the full parser pipeline and records, per
// replay, which human players the worldstate engine flags via HasMutaHitnRun.
//
// This is the load-bearing guarantee behind the presence-only "Muta hit-n-run"
// pill: per-window timing is intentionally not surfaced (a microed muta attack
// is geometrically indistinguishable from hit-n-run), so the regression guard
// is the per-game-player flag, not timings. Human-curated premises:
//   - muta_tvz_somajyj (ZvT) and muta_tvz_attitude (TvZ): confirmed real
//     sustained hit-n-run → must flag.
//   - muta_pvz_soma (PvZ): confirmed NO hit-n-run (only a one-off attack) →
//     must stay silent.
//   - muta_neg_pvt_cannon (PvP): no Zerg → must stay silent.
//   - muta_tvz_skins (TvZ): real but marginal (goliath defense) → below the
//     conservative bar, stays silent by design.
//
// Refresh with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/worldstate/ -run TestMutaHitnRunGolden
const (
	mutaHitnRunGoldenPath = "testdata/muta_hitnrun_golden.json"
	mutaHitnRunReplaysDir = "testdata/replays"
	mutaHitnRunFilePrefix = "muta_"
)

type mutaHitnRunGoldenDoc map[string][]int // replay filename → sorted flagged replay_player_ids

func TestMutaHitnRunGolden(t *testing.T) {
	entries, err := os.ReadDir(mutaHitnRunReplaysDir)
	if err != nil {
		t.Fatalf("read replays dir: %v", err)
	}
	actual := mutaHitnRunGoldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") || !strings.HasPrefix(e.Name(), mutaHitnRunFilePrefix) {
			continue
		}
		path := filepath.Join(mutaHitnRunReplaysDir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat %s: %v", e.Name(), err)
		}
		replay := parser.CreateReplayFromFileInfo(path, e.Name(), info.Size(), "")
		data, err := parser.ParseReplay(path, replay)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator)
		if !ok {
			actual[e.Name()] = []int{}
			continue
		}
		eng := orch.WorldStateEngine()
		flagged := []int{}
		for _, p := range replay.Players {
			if p.IsObserver || p.Type != "Human" {
				continue
			}
			if eng.HasMutaHitnRun(p.PlayerID) {
				flagged = append(flagged, int(p.PlayerID))
			}
		}
		sort.Ints(flagged)
		actual[e.Name()] = flagged
	}

	if len(actual) == 0 {
		t.Skip("no muta_*.rep fixtures present")
	}

	if os.Getenv("UPDATE_GOLDEN") != "" {
		payload, err := json.MarshalIndent(actual, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile(mutaHitnRunGoldenPath, payload, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}

	raw, err := os.ReadFile(mutaHitnRunGoldenPath)
	if err != nil {
		t.Fatalf("read golden (run UPDATE_GOLDEN=1 to seed): %v", err)
	}
	var expected mutaHitnRunGoldenDoc
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	enc := func(d mutaHitnRunGoldenDoc) string { b, _ := json.Marshal(d); return string(b) }
	if enc(expected) != enc(actual) {
		t.Fatalf("muta hit-n-run golden mismatch:\n  expected=%s\n  actual=  %s", enc(expected), enc(actual))
	}
}
