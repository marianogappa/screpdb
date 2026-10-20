//go:build !js

package dashboard

import (
	"testing"

	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
)

func TestMomentPlayersMapsRaceAndHumanity(t *testing.T) {
	rec := &library.Replay{Players: []library.Player{
		{Slot: 0, Race: library.RaceZerg, Type: library.PlayerTypeHuman},
		{Slot: 1, Race: library.RaceTerran},
		{Slot: 2, Race: library.RaceProtoss, Type: library.PlayerTypeHuman, Flags: library.PlayerObserver},
		{Slot: 3, Race: library.RaceUnknown, Type: library.PlayerTypeHuman},
	}}
	got := momentPlayers(rec)
	want := []moments.Player{
		{Slot: 0, Race: openbw.RaceZerg, Human: true},
		{Slot: 1, Race: openbw.RaceTerran, Human: false},
		{Slot: 2, Race: openbw.RaceProtoss, Human: false},
		{Slot: 3, Race: openbw.RaceRandom, Human: true},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d players", len(got))
	}
	for i := range want {
		if got[i].Slot != want[i].Slot || got[i].Race != want[i].Race || got[i].Human != want[i].Human {
			t.Errorf("player %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestMomentBasesKinds(t *testing.T) {
	clock := int64(12)
	metas := []overlayBaseMeta{
		{IsStarting: true, Base: workflowGameEventBase{Center: workflowGameEventPoint{X: 100, Y: 200}, Polygon: []workflowGameEventPoint{{X: 1, Y: 2}, {X: 3, Y: 4}}}},
		{Base: workflowGameEventBase{NaturalOfClock: &clock, Center: workflowGameEventPoint{X: 300, Y: 400}}},
		{Base: workflowGameEventBase{Center: workflowGameEventPoint{X: 500, Y: 600}}},
	}
	got := momentBases(metas)
	if len(got) != 3 {
		t.Fatalf("got %d bases", len(got))
	}
	if got[0].Kind != moments.BaseStart || got[0].X != 100 || got[0].Y != 200 || len(got[0].Polygon) != 2 || got[0].Polygon[1] != [2]int32{3, 4} {
		t.Errorf("start = %+v", got[0])
	}
	if got[1].Kind != moments.BaseNatural || got[1].X != 300 {
		t.Errorf("natural = %+v", got[1])
	}
	if got[2].Kind != moments.BaseExpansion || len(got[2].Polygon) != 0 {
		t.Errorf("expansion = %+v", got[2])
	}
}

func TestLibraryBasesMarksNaturalsOfStarts(t *testing.T) {
	layout := &library.MapLayout{Bases: []library.MapBase{
		{Name: "Main", Kind: "Starting", NaturalExpansion: "Nat", CenterX: 10, CenterY: 20, Polygon: []library.MapPoint{{X: 1, Y: 2}}},
		{Name: "Nat", Kind: "expansion", CenterX: 30, CenterY: 40},
		{Name: "Third", Kind: "expansion", NaturalExpansion: "Main", CenterX: 50, CenterY: 60},
		{Name: "Other main", Kind: "start"},
	}}
	got := libraryBases(layout)
	wantKinds := []moments.BaseKind{moments.BaseStart, moments.BaseNatural, moments.BaseExpansion, moments.BaseStart}
	for i, k := range wantKinds {
		if got[i].Kind != k {
			t.Errorf("base %d kind = %q, want %q", i, got[i].Kind, k)
		}
	}
	if got[0].X != 10 || got[0].Y != 20 || got[0].Polygon[0] != [2]int32{1, 2} || got[2].X != 50 {
		t.Errorf("coordinates lost: %+v", got)
	}
}

func TestIsStartKind(t *testing.T) {
	for kind, want := range map[string]bool{"start": true, "Starting": true, "START": true, "natural": false, "": false} {
		if got := isStartKind(kind); got != want {
			t.Errorf("isStartKind(%q) = %v", kind, got)
		}
	}
}

func TestRegionAroundBase(t *testing.T) {
	small := &workflowGameEventBase{Center: workflowGameEventPoint{X: 1000, Y: 1000}, Polygon: []workflowGameEventPoint{{X: 900, Y: 950}, {X: 1100, Y: 1050}}}
	if got := *regionAroundBase(small); got != (workflowEngineRegion{X: 616, Y: 616, W: 768, H: 768}) {
		t.Errorf("small base region = %+v", got)
	}
	big := &workflowGameEventBase{Center: workflowGameEventPoint{X: 1000, Y: 1000}, Polygon: []workflowGameEventPoint{{X: 0, Y: 500}, {X: 2000, Y: 1500}}}
	if got := *regionAroundBase(big); got != (workflowEngineRegion{X: -256, Y: 244, W: 2512, H: 1512}) {
		t.Errorf("big base region = %+v", got)
	}
	bare := &workflowGameEventBase{Center: workflowGameEventPoint{X: 500, Y: 500}}
	if got := *regionAroundBase(bare); got != (workflowEngineRegion{X: 116, Y: 116, W: 768, H: 768}) {
		t.Errorf("polygonless base region = %+v", got)
	}
}

func oneVsOneReplay() (*library.Replay, map[int8]workflowGameEventPlayer) {
	rec := &library.Replay{Players: []library.Player{
		{Slot: 0, StartX: 100, StartY: 100},
		{Slot: 1, StartX: 1100, StartY: 100},
		{Slot: 2, StartX: 600, StartY: 600, Flags: library.PlayerObserver},
	}}
	players := map[int8]workflowGameEventPlayer{0: {PlayerID: 1, Name: "Alice"}, 1: {PlayerID: 2, Name: "Bob"}}
	return rec, players
}

func TestOpenFieldBaseOneVsOneByPositionBetweenMains(t *testing.T) {
	rec, players := oneVsOneReplay()
	cases := []struct {
		x        int32
		relation string
		ref      string
		name     string
	}{
		{200, "near_player_base", "Alice", "near Alice's base"},
		{500, "player_side", "Alice", "on Alice's side of the map"},
		{600, "middle", "", "in the middle"},
		{750, "player_side", "Bob", "on Bob's side of the map"},
		{1000, "near_player_base", "Bob", "near Bob's base"},
	}
	for _, c := range cases {
		m := moments.Event{Actor: 1, Target: 0, X: c.x, Y: 100}
		got := openFieldBase(m, nil, rec, players, nil, nil)
		if got.Kind != "open_field" || got.Center != (workflowGameEventPoint{X: float64(c.x), Y: 100}) {
			t.Errorf("x=%d: base = %+v", c.x, got)
		}
		if got.OpenField == nil || got.OpenField.Relation != c.relation || got.OpenField.Ref != c.ref || got.Name != c.name {
			t.Errorf("x=%d: got %+v %q, want %s/%s %q", c.x, got.OpenField, got.Name, c.relation, c.ref, c.name)
		}
	}
}

func TestOpenFieldBaseMultiplayerNamesNearestBase(t *testing.T) {
	rec := &library.Replay{Players: []library.Player{
		{Slot: 0, StartX: 100, StartY: 100},
		{Slot: 1, StartX: 3000, StartY: 100},
		{Slot: 2, StartX: 100, StartY: 3000},
	}}
	clock := int64(12)
	metas := []overlayBaseMeta{
		{Base: workflowGameEventBase{Name: "12 natural", Kind: "natural", NaturalOfClock: &clock, Center: workflowGameEventPoint{X: 2000, Y: 2000}}},
		{Base: workflowGameEventBase{Name: "3 expa", Kind: "expansion", Center: workflowGameEventPoint{X: 2400, Y: 2000}}},
	}
	roster := []workflowGamePlayer{{PlayerID: 7, Name: "Carol"}}
	startClocks := map[int64]int{7: 12}

	near := openFieldBase(moments.Event{Actor: 0, Target: 1, X: 2100, Y: 2000}, metas, rec, nil, startClocks, roster)
	if near.OpenField == nil || near.OpenField.Relation != "near_base" || near.OpenField.Ref != "Carol's natural" || near.Name != "near Carol's natural" {
		t.Errorf("near natural = %+v %q", near.OpenField, near.Name)
	}
	expa := openFieldBase(moments.Event{Actor: 0, Target: 1, X: 2350, Y: 2000}, metas, rec, nil, startClocks, roster)
	if expa.OpenField == nil || expa.OpenField.Ref != "3 expa" || expa.Name != "near 3 expa" {
		t.Errorf("near expa = %+v %q", expa.OpenField, expa.Name)
	}
	far := openFieldBase(moments.Event{Actor: 0, Target: 1, X: 100, Y: 100}, metas, rec, nil, startClocks, roster)
	if far.OpenField == nil || far.OpenField.Relation != "middle" || far.OpenField.Ref != "" || far.Name != "in the middle" {
		t.Errorf("far from bases = %+v %q", far.OpenField, far.Name)
	}
}

func TestOpenFieldBaseOneVsOneWithoutKnownMainsFallsBackToBases(t *testing.T) {
	rec := &library.Replay{Players: []library.Player{{Slot: 0}, {Slot: 1, StartX: 500, StartY: 500}}}
	got := openFieldBase(moments.Event{Actor: 1, Target: 0, X: 10, Y: 10}, nil, rec, nil, nil, nil)
	if got.OpenField == nil || got.OpenField.Relation != "middle" {
		t.Fatalf("got %+v", got.OpenField)
	}
}
