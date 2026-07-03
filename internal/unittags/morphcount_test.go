package unittags

import (
	"reflect"
	"testing"

	"github.com/icza/screp/rep"
	"github.com/icza/screp/rep/repcmd"
)

func TestMorphSelectionSizes_NilGuards(t *testing.T) {
	if got := MorphSelectionSizes(nil); len(got) != 0 {
		t.Errorf("nil replay should yield empty map, got %+v", got)
	}
	if got := MorphSelectionSizes(&rep.Replay{}); len(got) != 0 {
		t.Errorf("replay without commands should yield empty map, got %+v", got)
	}
}

func TestMorphSelectionSizes_MultiLarvaMorph(t *testing.T) {
	// Select three larvae, morph Drone => one command carrying selection size 3.
	// Only the morph command (index 1) gets an entry.
	cmds := []repcmd.Cmd{
		sel(2, 10, 0x100, 0x101, 0x102),
		morph(2, 11, "Drone"),
	}
	got := MorphSelectionSizes(replayOf(cmds...))
	if want := map[int]int{1: 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("MorphSelectionSizes = %+v, want %+v", got, want)
	}
}

func TestMorphSelectionSizes_NonLarvaAndNonMorphAbsent(t *testing.T) {
	cmds := []repcmd.Cmd{
		sel(2, 10, 0x100, 0x101),
		morph(2, 11, "Lurker"), // not a larva-morph unit -> absent
		sel(2, 20, 0x200),
		train(2, 21, "Mutalisk"), // Train, not UnitMorph -> untracked type here
	}
	got := MorphSelectionSizes(replayOf(cmds...))
	if len(got) != 0 {
		t.Errorf("no larva-morph commands => empty map, got %+v", got)
	}
}

func TestMorphSelectionSizes_EmptySelectionSkipped(t *testing.T) {
	// A morph with no prior selection has len(cur)==0 and is not recorded.
	cmds := []repcmd.Cmd{morph(2, 11, "Zergling")}
	got := MorphSelectionSizes(replayOf(cmds...))
	if len(got) != 0 {
		t.Errorf("morph with empty selection must be absent, got %+v", got)
	}
}

func TestMorphSelectionSizes_HotkeyAndAddRemoveTracked(t *testing.T) {
	// Selection is tracked through hotkey assign/select/add and select add/remove
	// exactly as Analyze does. Final selection before the morph is size 3.
	cmds := []repcmd.Cmd{
		sel(2, 5, 0x10, 0x11),      // {0x10,0x11}
		hotkey(2, 6, "Assign", 1),  // group 1 = {0x10,0x11}
		sel(2, 10, 0x20),           // {0x20}
		hotkey(2, 11, "Add", 1),    // {0x20,0x10,0x11}
		morph(2, 12, "Overlord"),   // idx 4: size 3
		hotkey(2, 20, "Select", 1), // {0x10,0x11}
		selAdd(2, 21, 0x30),        // {0x10,0x11,0x30}
		selRemove(2, 22, 0x11),     // {0x10,0x30}
		morph(2, 23, "Hydralisk"),  // idx 8: size 2
	}
	got := MorphSelectionSizes(replayOf(cmds...))
	if want := map[int]int{4: 3, 8: 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("MorphSelectionSizes = %+v, want %+v", got, want)
	}
}

func TestMorphSelectionSizes_PerPlayerIsolation(t *testing.T) {
	// Two players' selections do not bleed into each other.
	cmds := []repcmd.Cmd{
		sel(1, 5, 0xA1, 0xA2, 0xA3, 0xA4),
		sel(2, 6, 0xB1),
		morph(2, 7, "Zergling"), // idx 2: player 2 selection size 1
		morph(1, 8, "Drone"),    // idx 3: player 1 selection size 4
	}
	got := MorphSelectionSizes(replayOf(cmds...))
	if want := map[int]int{2: 1, 3: 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("MorphSelectionSizes = %+v, want %+v", got, want)
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, 0},
		{5, 5},
		{-5, 5},
		{-1, 1},
	}
	for _, tt := range tests {
		if got := abs(tt.in); got != tt.want {
			t.Errorf("abs(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestMatchProducerTagsToBuilds_NoBuildsNoMatch(t *testing.T) {
	// Producing tags but zero Build commands: nothing matches, empty map.
	tags := map[uint16]*Production{
		0xA: {FirstSec: 100},
		0xB: {FirstSec: 200},
	}
	got := matchProducerTagsToBuilds(tags, nil)
	if len(got) != 0 {
		t.Errorf("no builds => no matches, got %+v", got)
	}
}

func TestMatchProducerTagsToBuilds_BuildAfterFirstProductionUnclaimed(t *testing.T) {
	// A single tag first produces at sec 50, but the only Build is placed at
	// sec 100 (a building cannot produce before it is commanded), so the tag
	// stays unmatched.
	tags := map[uint16]*Production{0xA: {FirstSec: 50}}
	builds := []Build{{Sec: 100, X: 9, Y: 9}}
	got := matchProducerTagsToBuilds(tags, builds)
	if len(got) != 0 {
		t.Errorf("build placed after first production must not match, got %+v", got)
	}
}

func TestMatchProducerTagsToBuilds_EarliestProducerClaimsEarliestBuild(t *testing.T) {
	// Two tags, two eligible builds: earliest producer claims earliest build.
	tags := map[uint16]*Production{
		0xA: {FirstSec: 300},
		0xB: {FirstSec: 150},
	}
	builds := []Build{
		{Sec: 100, X: 1, Y: 1},
		{Sec: 200, X: 2, Y: 2},
	}
	got := matchProducerTagsToBuilds(tags, builds)
	if got[0xB] != (Build{Sec: 100, X: 1, Y: 1}) {
		t.Errorf("earliest producer B should claim earliest build, got %+v", got[0xB])
	}
	if got[0xA] != (Build{Sec: 200, X: 2, Y: 2}) {
		t.Errorf("later producer A should claim the remaining build, got %+v", got[0xA])
	}
}
