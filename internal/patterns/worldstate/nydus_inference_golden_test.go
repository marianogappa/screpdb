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

// Integration golden test for offensive-nydus detection. Runs the
// nydus_*.rep hand-curated golden replays through the parser + orchestrator +
// worldstate engine, collects every nydus_attack event, and diffs the
// per-event records against testdata/nydus_golden.json.
//
// The nydus_*.rep fixtures are human-curated premises (verified by watching the
// replays) and must NOT be blindly refreshed — see ../GOLDEN_TIERS.md.
//
// Refresh with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/worldstate/ -run TestNydusInferenceGolden
const (
	nydusGoldenPath = "testdata/nydus_golden.json"
	nydusReplaysDir = "testdata/replays"
	nydusFilePrefix = "nydus_"
)

type nydusGoldenRecord struct {
	Second         int    `json:"second"`
	TargetLabel    string `json:"target_label,omitempty"`
	TargetVia      string `json:"target_via,omitempty"`
	TargetOwnerPID byte   `json:"target_owner_pid,omitempty"`
}

type nydusReplayGolden struct {
	Records []nydusGoldenRecord `json:"records"`
}

type nydusGoldenDoc map[string]nydusReplayGolden

func TestNydusInferenceGolden(t *testing.T) {
	entries, err := os.ReadDir(nydusReplaysDir)
	if err != nil {
		t.Fatalf("read replays dir: %v", err)
	}
	hasNydusReplays := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".rep") && strings.HasPrefix(e.Name(), nydusFilePrefix) {
			hasNydusReplays = true
			break
		}
	}
	if !hasNydusReplays {
		t.Skip("no nydus_*.rep replays in testdata; add them and re-run with UPDATE_GOLDEN=1")
	}

	actual, err := buildNydusGolden(t)
	if err != nil {
		t.Fatalf("build nydus golden: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeNydusGolden(actual); err != nil {
			t.Fatalf("write nydus golden: %v", err)
		}
	}
	expected, err := readNydusGolden()
	if err != nil {
		t.Fatalf("read nydus golden: %v", err)
	}
	if diff := diffNydusGolden(expected, actual); diff != "" {
		t.Fatalf("nydus golden mismatch:\n%s", diff)
	}
}

func buildNydusGolden(t *testing.T) (nydusGoldenDoc, error) {
	t.Helper()
	entries, err := os.ReadDir(nydusReplaysDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	doc := nydusGoldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") || !strings.HasPrefix(e.Name(), nydusFilePrefix) {
			continue
		}
		path := filepath.Join(nydusReplaysDir, e.Name())
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
			doc[e.Name()] = nydusReplayGolden{Records: []nydusGoldenRecord{}}
			continue
		}
		doc[e.Name()] = nydusReplayGolden{Records: collectNydusRecords(orch.ReplayEvents())}
	}
	return doc, nil
}

func collectNydusRecords(events []worldstate.ReplayEvent) []nydusGoldenRecord {
	out := []nydusGoldenRecord{}
	for _, ev := range events {
		if ev.EventType != "nydus_attack" {
			continue
		}
		rec := nydusGoldenRecord{Second: ev.Second}
		if ev.LocationBaseType != nil || ev.LocationBaseOclock != nil {
			rec.TargetLabel = formatRecallBaseLabel(ev.LocationBaseType, ev.LocationBaseOclock, ev.LocationNaturalOfClock, ev.LocationMineralOnly)
		}
		if ev.Payload != nil && *ev.Payload != "" {
			var pl struct {
				TP byte   `json:"tp"`
				TV string `json:"tv"`
			}
			_ = json.Unmarshal([]byte(*ev.Payload), &pl)
			rec.TargetVia = pl.TV
			rec.TargetOwnerPID = pl.TP
		}
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Second < out[j].Second })
	return out
}

func writeNydusGolden(doc nydusGoldenDoc) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(nydusGoldenPath, payload, 0o644)
}

func readNydusGolden() (nydusGoldenDoc, error) {
	raw, err := os.ReadFile(nydusGoldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nydusGoldenDoc{}, nil
		}
		return nil, err
	}
	var doc nydusGoldenDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func diffNydusGolden(expected, actual nydusGoldenDoc) string {
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
		if d := diffNydusReplay(k, e.Records, a.Records); d != "" {
			b.WriteString(d)
		}
	}
	return b.String()
}

func diffNydusReplay(replay string, expected, actual []nydusGoldenRecord) string {
	if len(expected) != len(actual) {
		return fmt.Sprintf("  %s: event count mismatch expected=%d actual=%d\n    expected=%s\n    actual=  %s\n",
			replay, len(expected), len(actual), formatNydusList(expected), formatNydusList(actual))
	}
	var b strings.Builder
	for i := range expected {
		if expected[i] != actual[i] {
			b.WriteString(fmt.Sprintf("  %s[%d] mismatch:\n    expected=%s\n    actual=  %s\n",
				replay, i, formatNydusOne(expected[i]), formatNydusOne(actual[i])))
		}
	}
	return b.String()
}

func formatNydusList(rs []nydusGoldenRecord) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, formatNydusOne(r))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatNydusOne(r nydusGoldenRecord) string {
	tgt := r.TargetLabel
	if tgt == "" {
		tgt = "(unknown)"
	}
	owner := ""
	if r.TargetOwnerPID > 0 {
		owner = fmt.Sprintf(" [owner=p%d]", r.TargetOwnerPID)
	}
	via := "?"
	if r.TargetVia != "" {
		via = r.TargetVia
	}
	return fmt.Sprintf("@%d:%02d →%s%s [via=%s]", r.Second/60, r.Second%60, tgt, owner, via)
}
