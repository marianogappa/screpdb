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

// Integration golden test for recall target-location inference. Runs the
// six hand-curated golden replays through the parser + orchestrator + worldstate
// engine, collects every event_type=="recall" event, and diffs the per-cluster
// records against testdata/recalls_golden.json. The golden file is the source
// of truth for the user's hand-annotated targets.
//
// These are HUMAN-CURATED premises (the annotated recall targets) — a change
// that moves a target is a regression, not a blind refresh. See ../GOLDEN_TIERS.md.
//
// Refresh with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/worldstate/ -run TestRecallTargetInferenceGolden
//
// Each replay's record list contains:
//
//	second           - first cast in the cluster (mm:ss in human form)
//	count            - cluster size (>= 1)
//	source_label     - human-readable source-base label (e.g. "9", "9's natural")
//	target_label     - human-readable target-base label, "" if unknown
//	target_via       - "a" (attack-coincidence) | "t" (unit-tag) | "" if unknown
//	target_owner_pid - replay_player_id of target owner if any, 0 otherwise
//
// The label format mirrors the user's annotation style ("9", "9's natural",
// "expansion at 5", "center base") — see formatRecallBaseLabel below.
const (
	recallsGoldenPath  = "testdata/recalls_golden.json"
	recallsReplaysDir  = "testdata/replays"
	recallsFilePrefix  = "recalls_"
	recallEventType    = "recall"
)

type recallGoldenRecord struct {
	Second         int    `json:"second"`
	Count          int    `json:"count"`
	SourceLabel    string `json:"source_label,omitempty"`
	TargetLabel    string `json:"target_label,omitempty"`
	TargetVia      string `json:"target_via,omitempty"`
	TargetOwnerPID byte   `json:"target_owner_pid,omitempty"`
}

type recallReplayGolden struct {
	Records []recallGoldenRecord `json:"records"`
}

type recallGoldenDoc map[string]recallReplayGolden

func TestRecallTargetInferenceGolden(t *testing.T) {
	actual, err := buildRecallGolden(t)
	if err != nil {
		t.Fatalf("build recall golden: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeRecallGolden(actual); err != nil {
			t.Fatalf("write recall golden: %v", err)
		}
	}
	expected, err := readRecallGolden()
	if err != nil {
		t.Fatalf("read recall golden: %v", err)
	}
	if diff := diffRecallGolden(expected, actual); diff != "" {
		t.Fatalf("recall golden mismatch:\n%s", diff)
	}
}

func buildRecallGolden(t *testing.T) (recallGoldenDoc, error) {
	t.Helper()
	entries, err := os.ReadDir(recallsReplaysDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	doc := recallGoldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") || !strings.HasPrefix(e.Name(), recallsFilePrefix) {
			continue
		}
		path := filepath.Join(recallsReplaysDir, e.Name())
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
			doc[e.Name()] = recallReplayGolden{Records: []recallGoldenRecord{}}
			continue
		}
		records := collectRecallRecords(orch.ReplayEvents())
		doc[e.Name()] = recallReplayGolden{Records: records}
	}
	return doc, nil
}

func collectRecallRecords(events []worldstate.ReplayEvent) []recallGoldenRecord {
	out := []recallGoldenRecord{}
	for _, ev := range events {
		if ev.EventType != recallEventType {
			continue
		}
		rec := recallGoldenRecord{
			Second: ev.Second,
			Count:  1,
		}
		if ev.LocationBaseType != nil || ev.LocationBaseOclock != nil {
			rec.SourceLabel = formatRecallBaseLabel(ev.LocationBaseType, ev.LocationBaseOclock, ev.LocationNaturalOfClock, ev.LocationMineralOnly)
		}
		if ev.Payload != nil && *ev.Payload != "" {
			pl := decodeRecallPayload(*ev.Payload)
			if pl.N > 1 {
				rec.Count = pl.N
			}
			if pl.TB != nil {
				kind := pl.TB.K
				oc := pl.TB.O
				naturalOf := pl.TB.NO
				mineralOnly := pl.TB.MO
				rec.TargetLabel = formatRecallBaseLabel(&kind, &oc, naturalOf, mineralOnly)
			}
			rec.TargetVia = pl.TV
			rec.TargetOwnerPID = pl.TP
		}
		out = append(out, rec)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Second < out[j].Second })
	return out
}

// recallPayloadGolden mirrors the on-disk payload struct used by emitRecallEvents.
// Decoded with `json:"-"`-style short keys so the test stays decoupled from
// production types; if the production payload schema changes, this also
// changes — that's intentional, the golden is the contract.
type recallPayloadGolden struct {
	N  int                  `json:"n,omitempty"`
	LE int                  `json:"le,omitempty"`
	S  []int                `json:"s,omitempty"`
	T  []int                `json:"t,omitempty"`
	TB *recallTargetBaseGld `json:"tb,omitempty"`
	TP byte                 `json:"tp,omitempty"`
	TV string               `json:"tv,omitempty"`
}

type recallTargetBaseGld struct {
	K  string `json:"k,omitempty"`
	O  int    `json:"o,omitempty"`
	NO *int   `json:"no,omitempty"`
	MO *bool  `json:"mo,omitempty"`
}

func decodeRecallPayload(payload string) recallPayloadGolden {
	var pl recallPayloadGolden
	_ = json.Unmarshal([]byte(payload), &pl)
	return pl
}

// formatRecallBaseLabel renders a base label in the user's annotation style:
//   - starting → "<oclock>"          ("9")
//   - natural  → "<naturalOf>'s natural" ("9's natural") when NaturalOfClock is set
//   - expansion → "expansion at <oclock>"
//   - clock 0  → "center base"
func formatRecallBaseLabel(baseType *string, oclock *int, naturalOf *int, mineralOnly *bool) string {
	clock := -1
	if oclock != nil {
		clock = *oclock
	}
	if clock == 0 {
		return "center base"
	}
	bt := ""
	if baseType != nil {
		bt = *baseType
	}
	switch bt {
	case "starting":
		if clock >= 0 {
			return fmt.Sprintf("%d", clock)
		}
	case "natural":
		if naturalOf != nil {
			return fmt.Sprintf("%d's natural", *naturalOf)
		}
		if clock >= 0 {
			return fmt.Sprintf("natural at %d", clock)
		}
	case "expansion":
		if clock >= 0 {
			return fmt.Sprintf("expansion at %d", clock)
		}
	}
	if clock >= 0 {
		return fmt.Sprintf("base at %d", clock)
	}
	return ""
}

func writeRecallGolden(doc recallGoldenDoc) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(recallsGoldenPath, payload, 0o644)
}

func readRecallGolden() (recallGoldenDoc, error) {
	raw, err := os.ReadFile(recallsGoldenPath)
	if err != nil {
		return nil, err
	}
	var doc recallGoldenDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func diffRecallGolden(expected, actual recallGoldenDoc) string {
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
		if d := diffRecallReplay(k, e.Records, a.Records); d != "" {
			b.WriteString(d)
		}
	}
	return b.String()
}

func diffRecallReplay(replay string, expected, actual []recallGoldenRecord) string {
	if len(expected) != len(actual) {
		return fmt.Sprintf("  %s: cluster count mismatch expected=%d actual=%d\n    expected=%s\n    actual=  %s\n",
			replay, len(expected), len(actual), formatRecallList(expected), formatRecallList(actual))
	}
	var b strings.Builder
	for i := range expected {
		if expected[i] != actual[i] {
			b.WriteString(fmt.Sprintf("  %s[%d] mismatch:\n    expected=%s\n    actual=  %s\n",
				replay, i, formatRecallOne(expected[i]), formatRecallOne(actual[i])))
		}
	}
	return b.String()
}

func formatRecallList(rs []recallGoldenRecord) string {
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		parts = append(parts, formatRecallOne(r))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatRecallOne(r recallGoldenRecord) string {
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
	return fmt.Sprintf("@%d:%02d%s %s→%s%s [via=%s]", mm, ss, count, r.SourceLabel, tgt, owner, via)
}
