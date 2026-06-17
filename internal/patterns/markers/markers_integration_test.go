package markers_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// Integration golden test: runs a handful of real SC:BW replays stashed in
// testdata/replays through the full parser + orchestrator pipeline and
// asserts the marker results match testdata/markers_golden.json. The
// replays are hand-picked to collectively exercise every marker we know
// how to detect (see marker coverage below).
//
// Some assertions are human-curated premises that must NOT be blindly
// refreshed — see ../GOLDEN_TIERS.md (e.g. bo_team_mech_111.rep build orders).
//
// Refresh the golden with:
//
//	UPDATE_GOLDEN=1 go test ./internal/patterns/markers/ -run TestMarkersGolden
//
// The golden format is a map of replay filename → list of per-player
// detections (player index + pattern name + string-serialized value) so a
// diff pinpoints which specific marker regressed.
//
// The 10 hand-picked replays collectively exercise every marker we know
// how to detect. Trust the golden JSON for the exact set per replay; the
// list is large and changes whenever marker definitions are tweaked.
// Refresh with UPDATE_GOLDEN=1 (see above).

const (
	goldenRelativePath = "testdata/markers_golden.json"
	replaysDir         = "testdata/replays"
)

type playerDetection struct {
	ReplayPlayerID byte   `json:"replay_player_id"`
	PatternName    string `json:"pattern_name"`
	// Value is the serialized marker value (string / int / bool / time).
	// Bool values become "true"/"false"; ints become their decimal string;
	// nil leaves "". This keeps the golden stable across SQLite's value_*
	// column format without committing to a richer schema here.
	Value string `json:"value,omitempty"`
}

type replayGolden struct {
	Detections []playerDetection `json:"detections"`
}

type goldenDoc map[string]replayGolden

func TestMarkersGolden(t *testing.T) {
	actual, err := buildMarkersGolden(t)
	if err != nil {
		t.Fatalf("build golden: %v", err)
	}
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := writeMarkersGolden(actual); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	expected, err := readMarkersGolden()
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if diff := diffGolden(expected, actual); diff != "" {
		t.Fatalf("markers golden mismatch:\n%s", diff)
	}
}

func buildMarkersGolden(t *testing.T) (goldenDoc, error) {
	t.Helper()
	entries, err := os.ReadDir(replaysDir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	doc := goldenDoc{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rep") {
			continue
		}
		path := filepath.Join(replaysDir, e.Name())
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
			doc[e.Name()] = replayGolden{Detections: []playerDetection{}}
			continue
		}
		doc[e.Name()] = replayGolden{Detections: collectPlayerDetections(orch.GetResults())}
	}
	return doc, nil
}

func collectPlayerDetections(results []*core.PatternResult) []playerDetection {
	out := make([]playerDetection, 0, len(results))
	for _, r := range results {
		if r == nil || r.Level != core.LevelPlayer {
			continue
		}
		var rpID byte
		if r.ReplayPlayerID != nil {
			rpID = *r.ReplayPlayerID
		}
		out = append(out, playerDetection{
			ReplayPlayerID: rpID,
			PatternName:    r.PatternName,
			Value:          formatPatternValue(r),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ReplayPlayerID != out[j].ReplayPlayerID {
			return out[i].ReplayPlayerID < out[j].ReplayPlayerID
		}
		if out[i].PatternName != out[j].PatternName {
			return out[i].PatternName < out[j].PatternName
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func formatPatternValue(r *core.PatternResult) string {
	// Post-cleanup: PatternResult carries only DetectedAtSecond + Payload. Golden
	// files still compare a single "value" string per detection — synthesize it
	// to match the old shape: JSON payload when present, else stringified second.
	if len(r.Payload) > 0 {
		return string(r.Payload)
	}
	return intToString(r.DetectedAtSecond)
}

func intToString(v int) string {
	// Avoid strconv pull-in (tiny helper; nothing here is hot).
	if v == 0 {
		return "0"
	}
	neg := false
	if v < 0 {
		neg = true
		v = -v
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func writeMarkersGolden(doc goldenDoc) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(goldenRelativePath, payload, 0o644)
}

func readMarkersGolden() (goldenDoc, error) {
	raw, err := os.ReadFile(goldenRelativePath)
	if err != nil {
		return nil, err
	}
	var doc goldenDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func diffGolden(expected, actual goldenDoc) string {
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
		if diff := diffDetections(k, e.Detections, a.Detections); diff != "" {
			b.WriteString(diff)
		}
	}
	return b.String()
}

func diffDetections(replay string, expected, actual []playerDetection) string {
	if len(expected) != len(actual) {
		return "  " + replay + ": count mismatch expected=" + intToString(len(expected)) + " actual=" + intToString(len(actual)) + "\n    expected=" + formatDetections(expected) + "\n    actual=  " + formatDetections(actual) + "\n"
	}
	var b strings.Builder
	for i := range expected {
		e := expected[i]
		a := actual[i]
		if e.ReplayPlayerID != a.ReplayPlayerID || e.PatternName != a.PatternName || e.Value != a.Value {
			b.WriteString("  " + replay + "[" + intToString(i) + "] mismatch:\n    expected=" + formatOne(e) + "\n    actual=  " + formatOne(a) + "\n")
		}
	}
	return b.String()
}

func formatDetections(ds []playerDetection) string {
	parts := make([]string, 0, len(ds))
	for _, d := range ds {
		parts = append(parts, formatOne(d))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func formatOne(d playerDetection) string {
	s := "P" + intToString(int(d.ReplayPlayerID)) + "/" + d.PatternName
	if d.Value != "" {
		s += "=" + d.Value
	}
	return s
}
