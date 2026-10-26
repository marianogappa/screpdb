//go:build !js

package dashboard

import (
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
)

func TestSnapshotBasesNamesWhoHoldsWhat(t *testing.T) {
	square := func(x, y int32) []library.MapPoint {
		return []library.MapPoint{{X: x - 200, Y: y - 200}, {X: x + 200, Y: y - 200}, {X: x + 200, Y: y + 200}, {X: x - 200, Y: y + 200}}
	}
	rec := &library.Replay{
		ID: 7,
		Players: []library.Player{
			{Slot: 0, StartX: 300, StartY: 300},
			{Slot: 1, StartX: 3700, StartY: 3700},
		},
		Layout: &library.MapLayout{Bases: []library.MapBase{
			{Name: "11", Kind: "start", NaturalExpansion: "natural of 11", Clock: 11, CenterX: 300, CenterY: 300, Polygon: square(300, 300)},
			{Name: "natural of 11", Kind: "expansion", Clock: 11, CenterX: 900, CenterY: 400, Polygon: square(900, 400)},
			{Name: "5", Kind: "start", Clock: 5, CenterX: 3700, CenterY: 3700, Polygon: square(3700, 3700)},
			{Name: "center", Kind: "expansion", CenterX: 2000, CenterY: 2000, Polygon: square(2000, 2000)},
		}},
	}
	hatch := uint16(bwgame.UnitTypesZergHatchery)
	nexus := uint16(bwgame.UnitTypesProtossNexus)
	dg := &engine.Digest{
		Initial:     []engine.InitialUnit{{Unit: 1, Type: hatch, Owner: 0, X: 300, Y: 300}, {Unit: 2, Type: nexus, Owner: 1, X: 3700, Y: 3700}},
		Productions: []engine.Production{{Owner: 1, Type: nexus, Unit: 3, X: 900, Y: 400, Start: 200 * engine.EconomyEveryFrames, Complete: engine.NoFrame, Cancel: engine.NoFrame}},
	}
	halls := []moments.Hall{
		{Type: hatch, Owner: 0, X: 300, Y: 300, Complete: true},
		{Type: nexus, Owner: 1, X: 900, Y: 400},
		{Type: nexus, Owner: 1, X: 3700, Y: 3700, Complete: true},
	}
	bySlot := map[int8]int64{0: library.PlayerID(7, 0), 1: library.PlayerID(7, 1)}
	got := snapshotBases(dg, rec, 250, halls, bySlot)
	if len(got) != 3 {
		t.Fatalf("got %+v", got)
	}
	natural := got[1]
	if natural.Kind != "natural" || natural.NaturalOf != bySlot[0] || natural.PlayerID != bySlot[1] || natural.Complete || natural.Since == nil || *natural.Since != 200 || natural.TownHall != "Nexus" {
		t.Fatalf("natural = %+v", natural)
	}
	if got[0].Kind != "main" || got[2].Kind != "main" || len(got[0].Polygon) != 4 {
		t.Fatalf("mains = %+v %+v", got[0], got[2])
	}
	if out := snapshotBases(dg, &library.Replay{}, 250, halls, bySlot); len(out) != 0 {
		t.Fatalf("no layout: %+v", out)
	}
}
