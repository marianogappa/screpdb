package engine

import (
	"reflect"
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
)

func TestHidden(t *testing.T) {
	u := &bwgame.Unit{}
	if !hidden(u) {
		t.Fatal("a unit without a sprite is drawn")
	}
	u.Sprite = &bwgame.Sprite{}
	if hidden(u) {
		t.Fatal("a visible sprite is hidden")
	}
	u.Sprite.Flags = bwgame.SpriteFlagHidden
	if !hidden(u) {
		t.Fatal("a hidden sprite is drawn")
	}
}

func buildingUnit(buildTime, remaining int32, completed bool, order bwgame.Orders, queued ...int32) *bwgame.Unit {
	u := &bwgame.Unit{}
	u.UnitType = &bwgame.UnitTypeT{BuildTime: buildTime}
	u.RemainingBuildTime = remaining
	if completed {
		u.StatusFlags = bwgame.UnitStatusFlagCompleted
	}
	if order != 0 {
		u.OrderType = &bwgame.OrderTypeT{ID: order}
	}
	for _, bt := range queued {
		u.BuildQueue.PushBack(&bwgame.UnitTypeT{BuildTime: bt})
	}
	return u
}

func TestProgress(t *testing.T) {
	cases := []struct {
		name string
		u    *bwgame.Unit
		want uint8
	}{
		{"completed", buildingUnit(100, 0, true, 0), ProgressDone},
		{"half built", buildingUnit(100, 50, false, 0), 127},
		{"just started", buildingUnit(100, 100, false, 0), 0},
		{"no build time", buildingUnit(0, 0, false, 0), ProgressDone},
		{"remaining past total clamps to zero", buildingUnit(100, 150, false, 0), 0},
		{"morph measured against the queued type", buildingUnit(1000, 50, true, bwgame.OrdersZergUnitMorph, 200), 190},
		{"incomplete morph", buildingUnit(1000, 100, false, bwgame.OrdersIncompleteMorphing, 400), 190},
		{"morph without a queue uses its own type", buildingUnit(100, 25, true, bwgame.OrdersZergUnitMorph), 190},
	}
	for _, c := range cases {
		if got := progress(c.u); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func creepState(w, h int, creep ...int) *bwgame.State {
	st := &bwgame.State{}
	st.Game = &bwgame.GameState{MapTileWidth: uint(w), MapTileHeight: uint(h)}
	st.Tiles = make([]bwgame.Tile, w*h)
	for _, i := range creep {
		st.Tiles[i].Flags |= bwgame.TileFlagHasCreep
	}
	return st
}

func TestCreepBits(t *testing.T) {
	w, bits := creepBits(creepState(5, 3, 0, 6, 14))
	if w != 5 || !reflect.DeepEqual(bits, []byte{0b0100_0001, 0b0100_0000}) {
		t.Fatalf("width %d bits %08b", w, bits)
	}
	s := Snapshot{CreepW: w, Creep: bits}
	if !s.HasCreep(0, 0) || !s.HasCreep(1, 1) || !s.HasCreep(4, 2) || s.HasCreep(2, 1) {
		t.Fatal("HasCreep disagrees with the tiles creepBits read")
	}

	if w, bits := creepBits(creepState(4, 4)); w != 4 || bits != nil {
		t.Fatalf("no creep: width %d bits %v", w, bits)
	}
	if w, bits := creepBits(creepState(0, 4)); w != 0 || bits != nil {
		t.Fatalf("zero width: width %d bits %v", w, bits)
	}
	short := creepState(4, 4, 1)
	short.Tiles = short.Tiles[:8]
	if w, bits := creepBits(short); w != 0 || bits != nil {
		t.Fatalf("too few tiles: width %d bits %v", w, bits)
	}
}
