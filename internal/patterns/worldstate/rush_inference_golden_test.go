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
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// Integration golden test for rush detection (zergling_rush / cannon_rush).
// Runs the rush_*.rep fixtures through the parser + orchestrator + worldstate
// engine, collects every rush event, and diffs against testdata/rushes_golden.json.
//
// These are HUMAN-CURATED premises (issue #189): each fixture is a real rush
// confirmed by watching the replay. A change that drops one of these events, or
// adds a spurious one, is a regression — see ../GOLDEN_TIERS.md, not a blind
// refresh.
//
// Refresh with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/worldstate/ -run TestRushDetectionGolden
const (
	rushesGoldenPath = "testdata/rushes_golden.json"
	rushesReplaysDir = "testdata/replays"
	rushesFilePrefix = "rush_"
)

type rushGoldenRecord struct {
	Second    int    `json:"second"`
	Subtype   string `json:"subtype"`    // "zergling_rush" | "cannon_rush"
	SourcePID byte   `json:"source_pid"` // rusher's replay_player_id
}

type rushReplayGolden struct {
	Records []rushGoldenRecord `json:"records"`
}

type rushGoldenDoc map[string]rushReplayGolden

func TestRushDetectionGolden(t *testing.T) {
	entries, err := os.ReadDir(rushesReplaysDir)
	if err != nil {
		t.Fatalf("read replays dir: %v", err)
	}
	hasRushReplays := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".rep") && strings.HasPrefix(e.Name(), rushesFilePrefix) {
			hasRushReplays = true
			break
		}
	}
	if !hasRushReplays {
		t.Skip("no rush_*.rep replays in testdata; add them and re-run with UPDATE_GOLDEN=1")
	}

	actual, err := buildRushGolden(t)
	if err != nil {
		t.Fatalf("build rush golden: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeRushGolden(actual); err != nil {
			t.Fatalf("write rush golden: %v", err)
		}
	}
	expected, err := readRushGolden()
	if err != nil {
		t.Fatalf("read rush golden: %v", err)
	}
	if diff := diffRushGolden(expected, actual); diff != "" {
		t.Fatalf("rush golden mismatch:\n%s", diff)
	}
}

func buildRushGolden(t *testing.T) (rushGoldenDoc, error) {
	t.Helper()
	entries, err := os.ReadDir(rushesReplaysDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	doc := rushGoldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") || !strings.HasPrefix(e.Name(), rushesFilePrefix) {
			continue
		}
		path := filepath.Join(rushesReplaysDir, e.Name())
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		replay := parser.CreateReplayFromFileInfo(path, e.Name(), info.Size(), "")
		data, err := parser.ParseReplay(path, replay)
		if err != nil {
			return nil, err
		}
		orch, ok := data.PatternOrchestrator.(*patterns.Orchestrator)
		if !ok {
			doc[e.Name()] = rushReplayGolden{Records: []rushGoldenRecord{}}
			continue
		}
		doc[e.Name()] = rushReplayGolden{Records: collectRushRecords(orch.ReplayEvents())}
	}
	return doc, nil
}

func isRushSubtype(s string) bool {
	return s == "zergling_rush" || s == "cannon_rush"
}

func collectRushRecords(events []worldstate.ReplayEvent) []rushGoldenRecord {
	out := []rushGoldenRecord{}
	for _, ev := range events {
		if !isRushSubtype(ev.EventType) {
			continue
		}
		var pid byte
		if ev.SourceReplayPlayerID != nil {
			pid = *ev.SourceReplayPlayerID
		}
		out = append(out, rushGoldenRecord{Second: ev.Second, Subtype: ev.EventType, SourcePID: pid})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Second != out[j].Second {
			return out[i].Second < out[j].Second
		}
		return out[i].Subtype < out[j].Subtype
	})
	return out
}

func writeRushGolden(doc rushGoldenDoc) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(rushesGoldenPath, payload, 0o644)
}

func readRushGolden() (rushGoldenDoc, error) {
	raw, err := os.ReadFile(rushesGoldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return rushGoldenDoc{}, nil
		}
		return nil, err
	}
	var doc rushGoldenDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func diffRushGolden(expected, actual rushGoldenDoc) string {
	var b strings.Builder
	seen := map[string]struct{}{}
	for k := range expected {
		seen[k] = struct{}{}
	}
	for k := range actual {
		seen[k] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		e, okE := expected[k]
		a, okA := actual[k]
		if !okE {
			b.WriteString("unexpected replay in actual: " + k + "\n")
			continue
		}
		if !okA {
			b.WriteString("replay missing from actual: " + k + "\n")
			continue
		}
		if d := diffRushReplay(k, e.Records, a.Records); d != "" {
			b.WriteString(d)
		}
	}
	return b.String()
}

func diffRushReplay(name string, expected, actual []rushGoldenRecord) string {
	if len(expected) != len(actual) {
		return fmtRushMismatch(name, expected, actual)
	}
	for i := range expected {
		if expected[i] != actual[i] {
			return fmtRushMismatch(name, expected, actual)
		}
	}
	return ""
}

func fmtRushMismatch(name string, expected, actual []rushGoldenRecord) string {
	enc := func(rs []rushGoldenRecord) string {
		b, _ := json.Marshal(rs)
		return string(b)
	}
	return name + " mismatch:\n  expected=" + enc(expected) + "\n  actual=  " + enc(actual) + "\n"
}
