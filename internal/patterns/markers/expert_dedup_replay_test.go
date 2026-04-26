package markers_test

import (
	"os"
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// Regression test for the "two Gateways at the same time" UI bug. The replay
// at the local path below has IlIlIllIlllIlll (Zerg) vs 321gggg (Protoss);
// 321gggg places Gateways at frames 1978/1994/2003/2060/2132 — i.e. five
// raw Build commands within ~7s but only three distinct positions
// (anti-spam double-clicks at the same tile). Pre-fix, ResolveExpert ran
// against raw facts and surfaced "1st Gateway @ 83s" + "2nd Gateway @ 83s"
// — visually identical timestamps in the UI. With ApplyBuildDedup wired
// into the call site, the duplicates collapse and "2nd Gateway" lands on
// the genuine second placement (~86s).
//
// The replay lives outside the repo (sibling project's replay archive),
// so the test skips when the file is absent — keeps CI green for anyone
// who hasn't pulled that data.
func TestApplyBuildDedup_ResolvesSecondGatewayAfterDoubleClickSpam(t *testing.T) {
	replayPath := "/Users/mariano.gappa/Code/go/src/github.com/marianogappa/cwalggdl/replays-cwal-dl/11-MORE-IlIlIllIlllIlll/MM-9098BD62-3EBC-11F1-A3DD-FA167B5461B6.rep"
	if _, err := os.Stat(replayPath); err != nil {
		t.Skipf("replay not present at %s; skipping (set up cwalggdl checkout to run)", replayPath)
	}

	info, err := os.Stat(replayPath)
	if err != nil {
		t.Fatalf("stat replay: %v", err)
	}
	replay := parser.CreateReplayFromFileInfo(replayPath, "MM-9098BD62.rep", info.Size(), "")
	data, err := parser.ParseReplay(replayPath, replay)
	if err != nil {
		t.Fatalf("parse replay: %v", err)
	}

	// 321gggg is the Protoss player; locate them so we can filter their commands.
	var protossPlayerID int64
	var protossReplayPlayerID byte
	for _, p := range data.Players {
		if p == nil {
			continue
		}
		if p.Name == "321gggg" {
			protossPlayerID = p.ID
			protossReplayPlayerID = p.PlayerID
			break
		}
	}
	if protossPlayerID == 0 && protossReplayPlayerID == 0 {
		// IDs may not be assigned pre-storage; fall back to per-command Player.Race match.
	}

	// Build the EnrichedCommand list for the Protoss player. The detector
	// processes commands in stream order, classifies each, and feeds
	// KindMakeBuilding facts through dedup. We replicate that here.
	var facts []cmdenrich.EnrichedCommand
	for _, cmd := range data.Commands {
		if cmd == nil || cmd.Player == nil {
			continue
		}
		if cmd.Player.Name != "321gggg" {
			continue
		}
		ec, ok := cmdenrich.Classify(cmd)
		if !ok {
			continue
		}
		facts = append(facts, ec)
	}
	if len(facts) == 0 {
		t.Fatalf("no enriched commands for Protoss player")
	}

	// Sanity: raw facts contain ≥4 Gateway Build commands inside the
	// dedup window. If this ever changes (the replay archive is mutable),
	// re-derive the expected timings from the new commands.
	rawGateways := 0
	for _, f := range facts {
		if f.Kind == cmdenrich.KindMakeBuilding && f.Subject == "Gateway" {
			rawGateways++
		}
	}
	if rawGateways < 4 {
		t.Fatalf("expected ≥4 raw Gateway Build commands, got %d (replay drift?)", rawGateways)
	}

	bo := findBOForExpertTest(t, "2 Gate")

	// Belt: without dedup, the bug is reproducible — 1st and 2nd Gateway
	// land within BuildDedupGapSeconds of each other (the original bug).
	rawResolutions := bo.ResolveExpert(facts)
	var rawFirst, rawSecond *markers.ExpertResolution
	for i := range rawResolutions {
		switch rawResolutions[i].Key {
		case "1st Gateway":
			rawFirst = &rawResolutions[i]
		case "2nd Gateway":
			rawSecond = &rawResolutions[i]
		}
	}
	if rawFirst == nil || rawSecond == nil || !rawFirst.Found || !rawSecond.Found {
		t.Fatalf("raw resolution should still find both Gateways; got 1st=%v 2nd=%v", rawFirst, rawSecond)
	}
	if rawSecond.ActualSecond-rawFirst.ActualSecond >= markers.BuildDedupGapSeconds {
		t.Fatalf("raw replay no longer reproduces the spam-build bug (gap=%ds); replace replay or rewrite test",
			rawSecond.ActualSecond-rawFirst.ActualSecond)
	}

	// Suspenders: dedup collapses duplicates, post-dedup gap ≥ window.
	deduped := markers.ApplyBuildDedup(facts)
	resolutions := bo.ResolveExpert(deduped)

	var first, second *markers.ExpertResolution
	for i := range resolutions {
		switch resolutions[i].Key {
		case "1st Gateway":
			first = &resolutions[i]
		case "2nd Gateway":
			second = &resolutions[i]
		}
	}
	if first == nil || !first.Found {
		t.Fatalf("1st Gateway not resolved")
	}
	if second == nil || !second.Found {
		t.Fatalf("2nd Gateway not resolved")
	}
	gap := second.ActualSecond - first.ActualSecond
	if gap < markers.BuildDedupGapSeconds {
		t.Fatalf("post-dedup 1st/2nd Gateway should be at least %ds apart, got %ds (1st=%d, 2nd=%d)",
			markers.BuildDedupGapSeconds, gap, first.ActualSecond, second.ActualSecond)
	}
}

func findBOForExpertTest(t *testing.T, name string) markers.Marker {
	t.Helper()
	for _, bo := range markers.Markers() {
		if bo.Name == name {
			return bo
		}
	}
	t.Fatalf("BO %q not registered", name)
	return markers.Marker{}
}
