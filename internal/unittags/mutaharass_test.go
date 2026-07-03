package unittags

import (
	"testing"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
	"github.com/marianogappa/screpdb/internal/models"
)

// frMs returns the frame whose Frame.Seconds() is closest to ms milliseconds.
// One frame is 42ms, so this lets tests place commands at fractional-second
// cadence (Analyze's harass detection measures commands-per-second).
func frMs(ms int) repcore.Frame { return repcore.Frame((ms + 21) / 42) }

func rightClickMs(pid byte, ms int, x, y uint16) *repcmd.RightClickCmd {
	return &repcmd.RightClickCmd{
		Base: &repcmd.Base{Frame: frMs(ms), PlayerID: pid, Type: &repcmd.Type{ID: repcmd.TypeIDRightClick}},
		Pos:  repcore.Point{X: x, Y: y},
	}
}

func targetedOrderMs(pid byte, ms int, order string, x, y uint16) *repcmd.TargetedOrderCmd {
	return &repcmd.TargetedOrderCmd{
		Base:  &repcmd.Base{Frame: frMs(ms), PlayerID: pid, Type: &repcmd.Type{ID: repcmd.TypeIDTargetedOrder}},
		Pos:   repcore.Point{X: x, Y: y},
		Order: &repcmd.Order{Enum: repcore.Enum{Name: order}},
	}
}

func hotkeyMs(pid byte, ms int, htype string, group byte) *repcmd.HotkeyCmd {
	c := hotkey(pid, 0, htype, group)
	c.Base.Frame = frMs(ms)
	return c
}

func selMs(pid byte, ms int, tags ...uint16) *repcmd.SelectCmd {
	c := sel(pid, 0, tags...)
	c.Base.Frame = frMs(ms)
	return c
}

func buildMs(pid byte, ms int, name string, x, y uint16) *repcmd.BuildCmd {
	c := build(pid, 0, name, x, y)
	c.Base.Frame = frMs(ms)
	return c
}

func morphMs(pid byte, ms int, name string) *repcmd.TrainCmd {
	c := morph(pid, 0, name)
	c.Base.Frame = frMs(ms)
	return c
}

var zergP1 = []*models.Player{{PlayerID: 1, Race: "Zerg"}}

// mutaSetup builds the game-level gate: a Spire and three Mutalisk morphs
// (6 lifetime mutas) at t=60s, so any spatial commands after t=60s can qualify.
func mutaSetup(pid byte) []repcmd.Cmd {
	return []repcmd.Cmd{
		buildMs(pid, 30_000, "Spire", 10, 10),
		morphMs(pid, 60_000, "Mutalisk"),
		morphMs(pid, 60_100, "Mutalisk"),
		morphMs(pid, 60_200, "Mutalisk"),
	}
}

// oscillatingFlock issues a hotkey-selected flock of `size` and then `n`
// right-clicks that alternate between two points `amp` px apart on X, every
// stepMs, starting at startMs. Consecutive move-vectors reverse direction so
// the hit-n-run reversal fraction is ~1.0.
func oscillatingFlock(pid byte, size int, group byte, startMs, stepMs, n int, amp uint16) []repcmd.Cmd {
	tags := make([]uint16, size)
	for i := range tags {
		tags[i] = uint16(0x100 + i)
	}
	cmds := []repcmd.Cmd{
		selMs(pid, startMs-2*stepMs, tags...),
		hotkeyMs(pid, startMs-stepMs, "Assign", group),
	}
	for i := 0; i < n; i++ {
		var x uint16
		if i%2 == 0 {
			x = 500
		} else {
			x = 500 + amp
		}
		cmds = append(cmds, rightClickMs(pid, startMs+i*stepMs, x, 500))
	}
	return cmds
}

func TestDetectMutaHarass_NilGuards(t *testing.T) {
	if got := DetectMutaHarass(nil, zergP1); got != nil {
		t.Errorf("nil replay should yield nil, got %+v", got)
	}
	if got := DetectMutaHarass(replayOf(), zergP1); got != nil {
		t.Errorf("empty replay should yield nil, got %+v", got)
	}
}

func TestDetectMutaHarass_CompositionGateFails(t *testing.T) {
	// A textbook oscillating flock, but the game-level gates are not met, so no
	// episode is surfaced regardless of the micro.
	flock := oscillatingFlock(1, 6, 1, 65_000, 400, 20, 300)

	tests := []struct {
		name string
		pre  []repcmd.Cmd
	}{
		{name: "no spire, no muta", pre: nil},
		{
			name: "spire but no muta production",
			pre:  []repcmd.Cmd{buildMs(1, 30_000, "Spire", 10, 10)},
		},
		{
			name: "spire + only one muta morph (below 6-muta floor)",
			pre: []repcmd.Cmd{
				buildMs(1, 30_000, "Spire", 10, 10),
				morphMs(1, 60_000, "Mutalisk"), // 2 mutas < 6
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds := append(append([]repcmd.Cmd(nil), tt.pre...), flock...)
			if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
				t.Errorf("gate should suppress detection, got %+v", got)
			}
		})
	}
}

func TestDetectMutaHarass_NonZergIgnored(t *testing.T) {
	// Same qualifying stream, but the player's race is Protoss: not tracked.
	cmds := append(mutaSetup(1), oscillatingFlock(1, 6, 1, 65_000, 400, 20, 300)...)
	protoss := []*models.Player{{PlayerID: 1, Race: "Protoss"}}
	if got := DetectMutaHarass(replayOf(cmds...), protoss); got != nil {
		t.Errorf("non-Zerg player must not produce harass episodes, got %+v", got)
	}
}

func TestDetectMutaHarass_DetectedEpisode(t *testing.T) {
	// 20 oscillating right-clicks every 400ms => ~2.5 cmds/sec over ~7.6s, all
	// reversing direction. Clears every hit-n-run threshold and the window gate.
	cmds := append(mutaSetup(1), oscillatingFlock(1, 8, 1, 65_000, 400, 20, 300)...)
	got := DetectMutaHarass(replayOf(cmds...), zergP1)
	if len(got) != 1 {
		t.Fatalf("expected exactly one harass window, got %d: %+v", len(got), got)
	}
	ep := got[0]
	if ep.PlayerID != 1 {
		t.Errorf("PlayerID: got %d, want 1", ep.PlayerID)
	}
	// 20 alternating clicks => 19 significant moves => 18 direction reversals.
	if ep.Cycles != 18 {
		t.Errorf("Cycles (reversals): got %d, want 18", ep.Cycles)
	}
	if ep.GroupSize != 8 {
		t.Errorf("GroupSize (median selection): got %d, want 8", ep.GroupSize)
	}
	if ep.StartSec < 65 {
		t.Errorf("StartSec %d should be at/after first click (~65s)", ep.StartSec)
	}
	if ep.EndSec-ep.StartSec < minWindowDurSec {
		t.Errorf("window duration %d below floor %d", ep.EndSec-ep.StartSec, minWindowDurSec)
	}
	if len(ep.Path) < hnrMinSigMoves {
		t.Errorf("path too short: %d points", len(ep.Path))
	}
}

func TestDetectMutaHarass_MonotonicAttackRejected(t *testing.T) {
	// A-move march: the flock walks steadily one direction (no reversals), which
	// is an attack, not hit-n-run. Fails the reversal-fraction discriminator.
	tags := []uint16{0x100, 0x101, 0x102, 0x103, 0x104, 0x105}
	cmds := mutaSetup(1)
	cmds = append(cmds, selMs(1, 64_000, tags...), hotkeyMs(1, 64_400, "Assign", 1))
	for i := 0; i < 20; i++ {
		x := uint16(200 + i*80) // strictly increasing => zero reversals
		cmds = append(cmds, rightClickMs(1, 65_000+i*400, x, 500))
	}
	if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
		t.Errorf("monotonic march must not be flagged as hit-n-run, got %+v", got)
	}
}

func TestDetectMutaHarass_BelowWindowFloorRejected(t *testing.T) {
	// A single short oscillating burst clears scoreEpisode's per-burst bar but
	// falls short of the merged-window campaign floor (minWindowReversals=10):
	// 8 clicks => 7 significant moves => 6 reversals < 10, so mergeEpisodes drops it.
	cmds := append(mutaSetup(1), oscillatingFlock(1, 6, 1, 65_000, 400, 8, 300)...)
	if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
		t.Errorf("burst below window floor must be dropped, got %+v", got)
	}
}

func TestDetectMutaHarass_EpisodesMergeIntoWindow(t *testing.T) {
	// Two oscillating bursts, each 8 clicks (7 significant moves => 6 reversals),
	// separated by a short regroup pause (<12s mergeGap but >3s episodeGap so
	// they are distinct episodes). Neither alone clears the 10-reversal window
	// floor, but merged they total 12 reversals over a long-enough span => one
	// surfaced window.
	cmds := mutaSetup(1)
	cmds = append(cmds, oscillatingFlock(1, 6, 1, 65_000, 400, 8, 300)...)
	// Second burst starts ~8s after the first ends (gap > episodeGapSec=3s so a
	// new episode; gap < mergeGapSec=12s so it merges).
	cmds = append(cmds, oscillatingFlock(1, 6, 1, 76_000, 400, 8, 300)...)
	got := DetectMutaHarass(replayOf(cmds...), zergP1)
	if len(got) != 1 {
		t.Fatalf("two nearby bursts should merge into one window, got %d: %+v", len(got), got)
	}
	if got[0].Cycles != 12 {
		t.Errorf("merged Cycles: got %d, want 12 (6+6)", got[0].Cycles)
	}
	// Path is concatenated from both episodes.
	if len(got[0].Path) < 16 {
		t.Errorf("merged path should concatenate both bursts, got %d points", len(got[0].Path))
	}
}

func TestDetectMutaHarass_JitterMovesIgnored(t *testing.T) {
	// A dense, sustained burst but every click lands within moveEpsPx (jitter,
	// re-clicking the same spot). Zero significant moves => not hit-n-run.
	tags := []uint16{0x100, 0x101, 0x102, 0x103, 0x104, 0x105}
	cmds := mutaSetup(1)
	cmds = append(cmds, selMs(1, 64_000, tags...), hotkeyMs(1, 64_400, "Assign", 1))
	for i := 0; i < 20; i++ {
		x := uint16(500 + i%2*10) // 10px jitter, below 50px eps
		cmds = append(cmds, rightClickMs(1, 65_000+i*400, x, 500))
	}
	if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
		t.Errorf("jitter-only burst must not be flagged, got %+v", got)
	}
}

func TestDetectMutaHarass_TargetedOrderNonMicroExcluded(t *testing.T) {
	// Targeted orders that are not flock micro (a rally-point "Rally" order) are
	// excluded by recordSpatial, so no spatial commands accumulate => no episode.
	tags := []uint16{0x100, 0x101, 0x102, 0x103, 0x104, 0x105}
	cmds := mutaSetup(1)
	cmds = append(cmds, selMs(1, 64_000, tags...), hotkeyMs(1, 64_400, "Assign", 1))
	for i := 0; i < 20; i++ {
		var x uint16 = 500
		if i%2 == 1 {
			x = 800
		}
		cmds = append(cmds, targetedOrderMs(1, 65_000+i*400, "Rally Point Unit", x, 500))
	}
	if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
		t.Errorf("non-micro targeted orders must not accumulate, got %+v", got)
	}
}

func TestDetectMutaHarass_TargetedAttackMoveDetected(t *testing.T) {
	// Attack-Move targeted orders ARE flock micro. An oscillating Attack Move
	// burst is detected just like right-clicks.
	tags := make([]uint16, 6)
	for i := range tags {
		tags[i] = uint16(0x100 + i)
	}
	cmds := mutaSetup(1)
	cmds = append(cmds, selMs(1, 64_000, tags...), hotkeyMs(1, 64_400, "Assign", 1))
	for i := 0; i < 20; i++ {
		var x uint16 = 500
		if i%2 == 1 {
			x = 800
		}
		cmds = append(cmds, targetedOrderMs(1, 65_000+i*400, "Attack Move", x, 500))
	}
	got := DetectMutaHarass(replayOf(cmds...), zergP1)
	if len(got) != 1 {
		t.Fatalf("oscillating Attack Move flock should be detected, got %d: %+v", len(got), got)
	}
	if got[0].Cycles != 18 {
		t.Errorf("Cycles: got %d, want 18", got[0].Cycles)
	}
}

func TestDetectMutaHarass_NoActiveGroupExcluded(t *testing.T) {
	// Direct selection (never hotkeyed) leaves activeGrp = -1, so recordSpatial
	// drops the commands: harass must be driven by a stable hotkey group.
	tags := make([]uint16, 6)
	for i := range tags {
		tags[i] = uint16(0x100 + i)
	}
	cmds := mutaSetup(1)
	cmds = append(cmds, selMs(1, 64_000, tags...)) // selected but not assigned to a hotkey
	for i := 0; i < 20; i++ {
		var x uint16 = 500
		if i%2 == 1 {
			x = 800
		}
		cmds = append(cmds, rightClickMs(1, 65_000+i*400, x, 500))
	}
	if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
		t.Errorf("commands without an active hotkey group must be excluded, got %+v", got)
	}
}

func TestDetectMutaHarass_OversizedSelectionExcluded(t *testing.T) {
	// A 20-unit selection exceeds mutaMaxGroupSize (16): a main-army a-move, not
	// a harass flock, so recordSpatial drops every command.
	tags := make([]uint16, 20)
	for i := range tags {
		tags[i] = uint16(0x100 + i)
	}
	cmds := mutaSetup(1)
	cmds = append(cmds, selMs(1, 64_000, tags...), hotkeyMs(1, 64_400, "Assign", 1))
	for i := 0; i < 20; i++ {
		var x uint16 = 500
		if i%2 == 1 {
			x = 800
		}
		cmds = append(cmds, rightClickMs(1, 65_000+i*400, x, 500))
	}
	if got := DetectMutaHarass(replayOf(cmds...), zergP1); got != nil {
		t.Errorf("oversized selection must be excluded, got %+v", got)
	}
}

func selAddMs(pid byte, ms int, tags ...uint16) *repcmd.SelectCmd {
	c := selAdd(pid, 0, tags...)
	c.Base.Frame = frMs(ms)
	return c
}

func selRemoveMs(pid byte, ms int, tags ...uint16) *repcmd.SelectCmd {
	c := selRemove(pid, 0, tags...)
	c.Base.Frame = frMs(ms)
	return c
}

func TestDetectMutaHarass_FlockBuiltViaAddRemoveAndHotkeyAdd(t *testing.T) {
	// The flock is assembled through SelectAdd, SelectRemove and a hotkey "Add"
	// union (mirroring how a player grows a control group), then hotkey-selected
	// to make it the active group. It still detects as one harass window.
	cmds := mutaSetup(1)
	cmds = append(cmds,
		selMs(1, 60_500, 0x100, 0x101, 0x102), // base flock
		selAddMs(1, 60_600, 0x103, 0x104),     // grow to 5
		hotkeyMs(1, 60_700, "Assign", 2),      // group 2 = {0x100..0x104}
		selMs(1, 60_800, 0x200, 0x201),        // stray selection
		selRemoveMs(1, 60_900, 0x201),         // trim it
		hotkeyMs(1, 61_000, "Add", 2),         // union group 2 in => 6 units
		hotkeyMs(1, 61_100, "Select", 2),      // make group 2 active
	)
	for i := 0; i < 20; i++ {
		var x uint16 = 500
		if i%2 == 1 {
			x = 800
		}
		cmds = append(cmds, rightClickMs(1, 65_000+i*400, x, 500))
	}
	got := DetectMutaHarass(replayOf(cmds...), zergP1)
	if len(got) != 1 {
		t.Fatalf("flock assembled via add/remove/hotkey-add should detect, got %d: %+v", len(got), got)
	}
	if got[0].Cycles != 18 {
		t.Errorf("Cycles: got %d, want 18", got[0].Cycles)
	}
}

func TestMergeEpisodes_Empty(t *testing.T) {
	if got := mergeEpisodes(nil); got != nil {
		t.Errorf("mergeEpisodes(nil) should be nil, got %+v", got)
	}
}

func TestMedianSize(t *testing.T) {
	tests := []struct {
		name string
		sel  []int
		want int
	}{
		{name: "single", sel: []int{7}, want: 7},
		{name: "odd count picks middle", sel: []int{3, 9, 5}, want: 5},
		{name: "even count picks upper-middle", sel: []int{4, 6, 8, 10}, want: 8},
		{name: "duplicates", sel: []int{5, 5, 5}, want: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seg := make([]spatialCmd, len(tt.sel))
			for i, s := range tt.sel {
				seg[i] = spatialCmd{selSize: s}
			}
			if got := medianSize(seg); got != tt.want {
				t.Errorf("medianSize(%v) = %d, want %d", tt.sel, got, tt.want)
			}
		})
	}
}
