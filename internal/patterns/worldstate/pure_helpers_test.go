package worldstate

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestOrderPredicates(t *testing.T) {
	if !isRushBuilding("Photon Cannon") || !isRushBuilding("Bunker") {
		t.Fatal("photon cannon / bunker should be rush buildings")
	}
	if isRushBuilding("Pylon") {
		t.Fatal("pylon is not a rush building")
	}
	if !isDropOrder("Unload") || !isDropOrder("Unload All") {
		t.Fatal("unload should be a drop order")
	}
	if isDropOrder("Move") {
		t.Fatal("move is not a drop order")
	}
	if !isRecallOrder("CastRecall") || !isRecallOrder("Recall") {
		t.Fatal("recall variants should match")
	}
	if isRecallOrder("Move") {
		t.Fatal("move is not recall")
	}
	if !isNukeOrder("NukeLaunch") || !isNukeOrder("Nuke") {
		t.Fatal("nuke variants should match")
	}
	if isNukeOrder("Attack") {
		t.Fatal("attack is not nuke")
	}
}

func TestIsTownHallUnit(t *testing.T) {
	for _, u := range []string{"Command Center", "Hatchery", "Nexus", "Lair"} {
		if u == "Lair" {
			if isTownHallUnit(u) {
				t.Fatal("lair should not match the town-hall substrings")
			}
			continue
		}
		if !isTownHallUnit(u) {
			t.Fatalf("%s should be a town hall", u)
		}
	}
	if isTownHallUnit("Barracks") {
		t.Fatal("barracks is not a town hall")
	}
}

func TestBuildAndLeavePredicates(t *testing.T) {
	if !isBuildLike("Build") || !isBuildLike("Land") {
		t.Fatal("build/land should be build-like")
	}
	if isBuildLike("Move") {
		t.Fatal("move is not build-like")
	}
	if !isLeaveAction("Leave Game") || !isLeaveAction("LeaveGame") {
		t.Fatal("leave game variants should match")
	}
	if isLeaveAction("Build") {
		t.Fatal("build is not a leave action")
	}
}

func TestNormalize(t *testing.T) {
	if normalize("Attack Move") != "attackmove" {
		t.Fatalf("normalize failed: %q", normalize("Attack Move"))
	}
	if normalize("Command_Center") != "commandcenter" {
		t.Fatalf("normalize underscore failed: %q", normalize("Command_Center"))
	}
}

func TestTileAndPixelConversions(t *testing.T) {
	if tileToPixel(0) != 16 {
		t.Fatalf("tileToPixel(0)=%v want 16", tileToPixel(0))
	}
	if tileToPixel(8) != 8*32+16 {
		t.Fatalf("tileToPixel(8)=%v want %d", tileToPixel(8), 8*32+16)
	}
}

func TestCommandCoords(t *testing.T) {
	x, y, ok := commandCoords(&models.Command{X: intPtr(10), Y: intPtr(20)})
	if !ok || x != 10 || y != 20 {
		t.Fatalf("commandCoords got (%v,%v,%v) want (10,20,true)", x, y, ok)
	}
	if _, _, ok := commandCoords(&models.Command{X: intPtr(10)}); ok {
		t.Fatal("missing Y should yield ok=false")
	}
	if _, _, ok := commandCoords(&models.Command{}); ok {
		t.Fatal("missing coords should yield ok=false")
	}
}

func TestCasterUnitForCast(t *testing.T) {
	if u, ok := casterUnitForCast(models.UnitOrderCastPsionicStorm); !ok || u != models.GeneralUnitHighTemplar {
		t.Fatalf("psionic storm -> (%q,%v) want High Templar", u, ok)
	}
	if u, ok := casterUnitForCast(models.UnitOrderNukeLaunch); !ok || u != models.GeneralUnitGhost {
		t.Fatalf("nuke -> (%q,%v) want Ghost", u, ok)
	}
	if u, ok := casterUnitForCast(models.UnitOrderCastRecall); !ok || u != models.GeneralUnitArbiter {
		t.Fatalf("recall -> (%q,%v) want Arbiter", u, ok)
	}
	if _, ok := casterUnitForCast(""); ok {
		t.Fatal("empty order should not resolve")
	}
	if _, ok := casterUnitForCast("Not A Real Order"); ok {
		t.Fatal("unknown order should not resolve")
	}
}

func TestMapKeys(t *testing.T) {
	if mapKeys(nil) != nil {
		t.Fatal("nil map should give nil")
	}
	if mapKeys(map[string]bool{}) != nil {
		t.Fatal("empty map should give nil")
	}
	got := mapKeys(map[string]bool{"zergling": true, "marine": true, "drone": true})
	want := []string{"drone", "marine", "zergling"}
	if len(got) != len(want) {
		t.Fatalf("mapKeys len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mapKeys=%v want sorted %v", got, want)
		}
	}
}

func TestJoinReasons(t *testing.T) {
	if joinReasons(nil) != "" {
		t.Fatal("nil reasons should be empty")
	}
	if joinReasons([]string{"a"}) != "a" {
		t.Fatal("single reason wrong")
	}
	if joinReasons([]string{"first-attack", "rush-window"}) != "first-attack, rush-window" {
		t.Fatalf("joinReasons wrong: %q", joinReasons([]string{"first-attack", "rush-window"}))
	}
}
