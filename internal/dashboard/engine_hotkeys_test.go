//go:build !js

package dashboard

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

func TestEngineAssignsPairStreamAssignsInOrderWithinASecond(t *testing.T) {
	frame := func(sec int64) int32 { return int32(sec * 1000 / 42) }
	dg := &engine.Digest{Hotkeys: []engine.HotkeyAssign{
		{Frame: frame(100), Owner: 1, Group: 1, MainType: 37, Count: 12},
		{Frame: frame(100), Owner: 2, Group: 1, MainType: 0, Count: 3},
		{Frame: frame(300), Owner: 1, Group: 1, MainType: 43, Count: 9},
	}}
	a := newEngineAssigns(dg, 1)
	if m, ok := a.match(1, 100); !ok || m.MainType != 37 || m.Count != 12 {
		t.Fatalf("first assign: %+v %v", m, ok)
	}
	if m, ok := a.match(1, 100); !ok || m.MainType != 37 {
		t.Fatalf("a repeated ctrl+number in the same second is the same assign: %+v %v", m, ok)
	}
	if _, ok := a.match(1, 200); ok {
		t.Fatal("no engine assign near 200s")
	}
	if m, ok := a.match(1, 300); !ok || m.MainType != 43 {
		t.Fatalf("later assign: %+v %v", m, ok)
	}
	if _, ok := a.match(2, 100); ok {
		t.Fatal("another group never matches")
	}
}

func TestUnitCompositionNamesEveryUnit(t *testing.T) {
	names := map[uint16]string{}
	got := unitComposition([]engine.UnitCount{{Type: 43, Count: 9}, {Type: 37, Count: 3}}, names)
	if len(got) != 2 || got[0] != [2]int{43, 9} || got[1] != [2]int{37, 3} || names[43] != "Mutalisk" || names[37] != "Zergling" {
		t.Fatalf("composition %v names %v", got, names)
	}
}

func TestApplyEngineAssignShowsAnUnusedKeyedBuildingAsABuilding(t *testing.T) {
	const hatchery, zergling = 131, 37
	payload := &hotkeyTimelinePayload{Buildings: map[byte]string{}, Units: map[uint16]string{}}
	payload.Players = []hotkeyTimelinePlayer{{Events: [][9]int{
		{466, int(hotkeystream.TypeAssignUnits), 9, 1},
		{500, int(hotkeystream.TypeAssignUnits), 1, 6},
	}}}
	p := &payload.Players[0]
	applyEngineAssign(p, 0, engine.HotkeyAssign{MainType: hatchery, Count: 1, Units: []engine.UnitCount{{Type: hatchery, Count: 1}}}, payload)
	applyEngineAssign(p, 1, engine.HotkeyAssign{MainType: zergling, Count: 6, Units: []engine.UnitCount{{Type: zergling, Count: 6}}}, payload)

	id := hotkeystream.BuildingID("Hatchery")
	want := [9]int{466, int(hotkeystream.TypeAssignBuilding), 9, 0, int(id), int(hotkeystream.TileUnknown), int(hotkeystream.TileUnknown)}
	if p.Events[0] != want || payload.Buildings[id] != "Hatchery" || p.Compositions[0] != nil {
		t.Fatalf("keyed Hatchery: event %v buildings %v composition %v", p.Events[0], payload.Buildings, p.Compositions[0])
	}
	if p.Events[1][evType] != int(hotkeystream.TypeAssignUnits) || p.Events[1][evUnitType] != zergling || p.Events[1][evUnitCount] != 6 || len(p.Compositions[1]) != 1 {
		t.Fatalf("Zergling group: event %v composition %v", p.Events[1], p.Compositions[1])
	}
}
