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

// Integration golden test for drop target-location inference. Runs the
// drops_*.rep hand-curated golden replays through the parser + orchestrator +
// worldstate engine, collects every drop-class event (drop / reaver_drop /
// cliff_drop), and diffs the per-cluster records against
// testdata/drops_golden.json.
//
// The drops_cliff_bgh_*.rep fixtures are human-curated premises (verified by
// watching the replays) and must NOT be blindly refreshed — see
// ../GOLDEN_TIERS.md.
//
// Refresh with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/worldstate/ -run TestDropTargetInferenceGolden
const (
	dropsGoldenPath = "testdata/drops_golden.json"
	dropsReplaysDir = "testdata/replays"
	dropsFilePrefix = "drops_"
)

type dropGoldenRecord struct {
	Second         int    `json:"second"`
	Count          int    `json:"count"`
	Subtype        string `json:"subtype"` // "drop" | "reaver_drop" | "cliff_drop"
	SourceLabel    string `json:"source_label,omitempty"`
	TargetLabel    string `json:"target_label,omitempty"`
	TargetVia      string `json:"target_via,omitempty"`
	TargetOwnerPID byte   `json:"target_owner_pid,omitempty"`
}

type dropReplayGolden struct {
	Records []dropGoldenRecord `json:"records"`
}

type dropGoldenDoc map[string]dropReplayGolden

func TestDropTargetInferenceGolden(t *testing.T) {
	entries, err := os.ReadDir(dropsReplaysDir)
	if err != nil {
		t.Fatalf("read replays dir: %v", err)
	}
	hasDropReplays := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".rep") && strings.HasPrefix(e.Name(), dropsFilePrefix) {
			hasDropReplays = true
			break
		}
	}
	if !hasDropReplays {
		t.Skip("no drops_*.rep replays in testdata; add them and re-run with UPDATE_GOLDEN=1")
	}

	actual, err := buildDropGolden(t)
	if err != nil {
		t.Fatalf("build drop golden: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeDropGolden(actual); err != nil {
			t.Fatalf("write drop golden: %v", err)
		}
	}
	expected, err := readDropGolden()
	if err != nil {
		t.Fatalf("read drop golden: %v", err)
	}
	if diff := diffDropGolden(expected, actual); diff != "" {
		t.Fatalf("drop golden mismatch:\n%s", diff)
	}
}

func buildDropGolden(t *testing.T) (dropGoldenDoc, error) {
	t.Helper()
	entries, err := os.ReadDir(dropsReplaysDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	doc := dropGoldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") || !strings.HasPrefix(e.Name(), dropsFilePrefix) {
			continue
		}
		path := filepath.Join(dropsReplaysDir, e.Name())
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
			doc[e.Name()] = dropReplayGolden{Records: []dropGoldenRecord{}}
			continue
		}
		records := collectDropRecords(orch.ReplayEvents())
		doc[e.Name()] = dropReplayGolden{Records: records}
	}
	return doc, nil
}

func isDropSubtype(t string) bool {
	switch t {
	case "drop", "reaver_drop", "cliff_drop":
		return true
	}
	return false
}

func collectDropRecords(events []worldstate.ReplayEvent) []dropGoldenRecord {
	out := []dropGoldenRecord{}
	for _, ev := range events {
		if !isDropSubtype(ev.EventType) {
			continue
		}
		rec := dropGoldenRecord{
			Second:  ev.Second,
			Count:   1,
			Subtype: ev.EventType,
		}
		// The drop's `LocationBaseType/Oclock` columns refer to the
		// destination (where the unload landed) — same convention used by
		// attack events. The golden records the destination as TargetLabel.
		if ev.LocationBaseType != nil || ev.LocationBaseOclock != nil {
			rec.TargetLabel = formatRecallBaseLabel(ev.LocationBaseType, ev.LocationBaseOclock, ev.LocationNaturalOfClock, ev.LocationMineralOnly)
		}
		if ev.Payload != nil && *ev.Payload != "" {
			pl := decodeRecallPayload(*ev.Payload) // drop payload mirrors recall shape
			if pl.N > 1 {
				rec.Count = pl.N
			}
			// For drops we surface SourceLabel from the payload's TB only if
			// present, but the source isn't keyed by base — it's a raw point.
			// We leave SourceLabel empty and rely on TargetVia/Owner for the
			// golden diff signal.
			rec.TargetVia = pl.TV
			rec.TargetOwnerPID = pl.TP
		}
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Second < out[j].Second })
	return out
}

func writeDropGolden(doc dropGoldenDoc) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dropsGoldenPath, payload, 0o644)
}

func readDropGolden() (dropGoldenDoc, error) {
	raw, err := os.ReadFile(dropsGoldenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return dropGoldenDoc{}, nil
		}
		return nil, err
	}
	var doc dropGoldenDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func diffDropGolden(expected, actual dropGoldenDoc) string {
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
		if d := diffDropReplay(k, e.Records, a.Records); d != "" {
			b.WriteString(d)
		}
	}
	return b.String()
}

func diffDropReplay(replay string, expected, actual []dropGoldenRecord) string {
	if len(expected) != len(actual) {
		return fmt.Sprintf("  %s: cluster count mismatch expected=%d actual=%d\n    expected=%s\n    actual=  %s\n",
			replay, len(expected), len(actual), formatDropList(expected), formatDropList(actual))
	}
	var b strings.Builder
	for i := range expected {
		if expected[i] != actual[i] {
			b.WriteString(fmt.Sprintf("  %s[%d] mismatch:\n    expected=%s\n    actual=  %s\n",
				replay, i, formatDropOne(expected[i]), formatDropOne(actual[i])))
		}
	}
	return b.String()
}

func formatDropList(rs []dropGoldenRecord) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, formatDropOne(r))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatDropOne(r dropGoldenRecord) string {
	mm := r.Second / 60
	ss := r.Second % 60
	via := "?"
	if r.TargetVia != "" {
		via = r.TargetVia
	}
	tgt := r.TargetLabel
	if tgt == "" {
		tgt = "(unknown)"
	}
	owner := ""
	if r.TargetOwnerPID > 0 {
		owner = fmt.Sprintf(" [owner=p%d]", r.TargetOwnerPID)
	}
	count := ""
	if r.Count > 1 {
		count = fmt.Sprintf(" ×%d", r.Count)
	}
	return fmt.Sprintf("@%d:%02d%s %s →%s%s [via=%s]", mm, ss, count, r.Subtype, tgt, owner, via)
}
