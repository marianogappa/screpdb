package moments

import (
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func TestBaseOwnersReadsTheHallsOnScreen(t *testing.T) {
	d := digest()
	d.Productions = append(d.Productions,
		building(0, bwgame.UnitTypesZergHatchery, 50, 900, 400, f(170), f(240), engine.NoFrame),
		building(1, bwgame.UnitTypesProtossNexus, 50, 3100, 3300, f(400), engine.NoFrame, engine.NoFrame),
	)
	d.Deaths = append(d.Deaths, engine.Death{Frame: f(100), Unit: 50, Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0})
	halls := []Hall{
		{Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0, X: 300, Y: 300, Complete: true},
		{Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0, X: 900, Y: 400, Complete: true},
		{Type: ut(bwgame.UnitTypesProtossNexus), Owner: 1, X: 3700, Y: 3700, Complete: true},
		{Type: ut(bwgame.UnitTypesTerranCommandCenter), Owner: 1, X: 2000, Y: 2000, Lifted: true},
	}
	got := BaseOwners(d, testBases, 300, halls)
	want := []BaseOwner{
		{Base: 0, Owner: 0, TownHall: ut(bwgame.UnitTypesZergHatchery), Complete: true, Since: 0},
		{Base: 1, Owner: 0, TownHall: ut(bwgame.UnitTypesZergHatchery), Complete: true, Since: 170},
		{Base: 2, Owner: 1, TownHall: ut(bwgame.UnitTypesProtossNexus), Complete: true, Since: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("base %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestBaseOwnersPrefersAFinishedHallThenTheOlderOne(t *testing.T) {
	d := digest()
	halls := []Hall{
		{Type: ut(bwgame.UnitTypesProtossNexus), Owner: 1, X: 310, Y: 300},
		{Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0, X: 300, Y: 300, Complete: true},
	}
	got := BaseOwners(d, testBases, 300, halls)
	if len(got) != 1 || got[0].Owner != 0 {
		t.Fatalf("got %+v", got)
	}
	if got := BaseOwners(d, testBases, 300, halls[:1]); len(got) != 1 || got[0].Owner != 1 || got[0].Since != -1 || got[0].Complete {
		t.Fatalf("a hall the digest never saw built: %+v", got)
	}
}
