package worldstate_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// Integration golden test for nuke detection. Runs the nuke_*.rep hand-curated
// golden replays through the parser + orchestrator + worldstate engine, collects
// every nuke event, and diffs the per-event records against
// testdata/nuke_golden.json.
//
// The nuke_*.rep fixtures (issue #187) are PENDING human review — see
// ../GOLDEN_TIERS.md. Until the nukes are confirmed by watching the replays,
// this golden is a tier-2 drift guard (refreshing on a deliberate, explainable
// change is fine). Once verified, it is promoted to a tier-1 premise where
// dropping a detection or adding a spurious one is a regression.
//
// Refresh with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/worldstate/ -run TestNukeInferenceGolden
const (
	nukeGoldenPath = "testdata/nuke_golden.json"
	nukeReplaysDir = "testdata/replays"
	nukeFilePrefix = "nuke_"
)

type nukeGoldenRecord struct {
	Second         int    `json:"second"`
	SourcePID      byte   `json:"source_pid"` // nuker's replay_player_id
	TargetLabel    string `json:"target_label,omitempty"`
	TargetOwnerPID byte   `json:"target_owner_pid,omitempty"`
}

type nukeReplayGolden struct {
	Records []nukeGoldenRecord `json:"records"`
}

type nukeGoldenDoc map[string]nukeReplayGolden

func TestNukeInferenceGolden(t *testing.T) {
	entries, err := os.ReadDir(nukeReplaysDir)
	if err != nil {
		t.Fatalf("read replays dir: %v", err)
	}
	hasNukeReplays := false
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".rep") && strings.HasPrefix(e.Name(), nukeFilePrefix) {
			hasNukeReplays = true
			break
		}
	}
	if !hasNukeReplays {
		t.Skip("no nuke_*.rep replays in testdata; add them and re-run with UPDATE_GOLDEN=1")
	}

	actual, err := buildNukeGolden(t)
	if err != nil {
		t.Fatalf("build nuke golden: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeNukeGolden(actual); err != nil {
			t.Fatalf("write nuke golden: %v", err)
		}
	}
	expected, err := readNukeGolden()
	if err != nil {
		t.Fatalf("read nuke golden: %v", err)
	}
	if diff := diffNukeGolden(expected, actual); diff != "" {
		t.Fatalf("nuke golden mismatch:\n%s", diff)
	}
}

func buildNukeGolden(t *testing.T) (nukeGoldenDoc, error) {
	t.Helper()
	entries, err := os.ReadDir(nukeReplaysDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	doc := nukeGoldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") || !strings.HasPrefix(e.Name(), nukeFilePrefix) {
			continue
		}
		path := filepath.Join(nukeReplaysDir, e.Name())
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
			doc[e.Name()] = nukeReplayGolden{Records: []nukeGoldenRecord{}}
			continue
		}
		doc[e.Name()] = nukeReplayGolden{Records: collectNukeRecords(orch.ReplayEvents())}
	}
	return doc, nil
}

func collectNukeRecords(events []worldstate.ReplayEvent) []nukeGoldenRecord {
	out := []nukeGoldenRecord{}
	for _, ev := range events {
		if ev.EventType != "nuke" {
			continue
		}
		rec := nukeGoldenRecord{Second: ev.Second}
		if ev.SourceReplayPlayerID != nil {
			rec.SourcePID = *ev.SourceReplayPlayerID
		}
		if ev.TargetReplayPlayerID != nil {
			rec.TargetOwnerPID = *ev.TargetReplayPlayerID
		}
		if ev.LocationBaseType != nil || ev.LocationBaseOclock != nil {
			rec.TargetLabel = formatRecallBaseLabel(ev.LocationBaseType, ev.LocationBaseOclock, ev.LocationNaturalOfClock, ev.LocationMineralOnly)
		}
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Second < out[j].Second })
	return out
}

func writeNukeGolden(doc nukeGoldenDoc) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(nukeGoldenPath, payload, 0o644)
}

func readNukeGolden() (nukeGoldenDoc, error) {
	raw, err := os.ReadFile(nukeGoldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nukeGoldenDoc{}, nil
		}
		return nil, err
	}
	var doc nukeGoldenDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func diffNukeGolden(expected, actual nukeGoldenDoc) string {
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
		if d := diffNukeReplay(k, e.Records, a.Records); d != "" {
			b.WriteString(d)
		}
	}
	return b.String()
}

func diffNukeReplay(replay string, expected, actual []nukeGoldenRecord) string {
	if len(expected) != len(actual) {
		return fmt.Sprintf("  %s: event count mismatch expected=%d actual=%d\n    expected=%s\n    actual=  %s\n",
			replay, len(expected), len(actual), formatNukeList(expected), formatNukeList(actual))
	}
	var b strings.Builder
	for i := range expected {
		if expected[i] != actual[i] {
			b.WriteString(fmt.Sprintf("  %s[%d] mismatch:\n    expected=%s\n    actual=  %s\n",
				replay, i, formatNukeOne(expected[i]), formatNukeOne(actual[i])))
		}
	}
	return b.String()
}

func formatNukeList(rs []nukeGoldenRecord) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, formatNukeOne(r))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatNukeOne(r nukeGoldenRecord) string {
	tgt := r.TargetLabel
	if tgt == "" {
		tgt = "(unknown)"
	}
	owner := ""
	if r.TargetOwnerPID > 0 {
		owner = fmt.Sprintf(" [owner=p%d]", r.TargetOwnerPID)
	}
	return fmt.Sprintf("@%d:%02d p%d →%s%s", r.Second/60, r.Second%60, r.SourcePID, tgt, owner)
}
