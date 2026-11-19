package moments

import (
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func TestATurningPointTellsTheForceThatDidIt(t *testing.T) {
	d := tvzDigest()
	snap := army(500, 0, bwgame.UnitTypesZergMutalisk, 10, 3650, 3650)
	snap.Units = append(snap.Units, army(500, 1, bwgame.UnitTypesTerranMarine, 2, 1000, 1000).Units...)
	d.Army = []engine.ArmySnapshot{snap}
	d.Outcomes = []engine.Outcome{{Frame: f(560), Owner: 1, Kind: engine.OutcomeLeft}, {Frame: f(560), Owner: 0, Kind: engine.OutcomeWon}}
	events := []Event{
		{Type: "recall", Second: 300, Actor: 0, Target: 1},
		{Type: "attack", Second: 500, Actor: 0, Target: 1, X: 3700, Y: 3700, Region: Region{X: 3500, Y: 3500, W: 400, H: 400}, Outcome: Outcome{Killed: 1600, TownHall: true}},
	}
	p := TurningPoints(d, testBases, tvz, events, 1)
	if p.You == nil || p.You.Event != 1 {
		t.Fatalf("the attack is why the Terran left: %+v", p)
	}
	f := p.You.Force
	if f.Count != 10 || f.Mostly != ut(bwgame.UnitTypesZergMutalisk) || f.Defence == 0 || f.Value <= f.Defence || f.Recalls != 1 {
		t.Fatalf("ten Mutalisks against two Marines, after one Recall: %+v", f)
	}
	if p.LiveAllies != 0 || p.Final == nil {
		t.Fatalf("a 1v1 has no allies and one final turning point: %+v", p)
	}
}

func TestAForceIsToldByItsMainTypeElseItsClassElseItsMainPair(t *testing.T) {
	share := func(typ bwgame.UnitTypes, value int32) UnitShare {
		return UnitShare{Type: ut(typ), Count: 1, Value: value}
	}
	cases := []struct {
		name  string
		units []UnitShare
		check func(Force) bool
	}{
		{"one type", []UnitShare{share(bwgame.UnitTypesProtossCarrier, 700), share(bwgame.UnitTypesProtossZealot, 300)}, func(f Force) bool { return f.Mostly == ut(bwgame.UnitTypesProtossCarrier) }},
		{"air", []UnitShare{share(bwgame.UnitTypesTerranWraith, 400), share(bwgame.UnitTypesTerranValkyrie, 400), share(bwgame.UnitTypesTerranMarine, 200)}, func(f Force) bool { return f.Class == ClassAir }},
		{"a pair", []UnitShare{share(bwgame.UnitTypesProtossZealot, 450), share(bwgame.UnitTypesProtossDragoon, 400), share(bwgame.UnitTypesProtossHighTemplar, 150)}, func(f Force) bool {
			return f.MostlyPair == [2]uint16{ut(bwgame.UnitTypesProtossZealot), ut(bwgame.UnitTypesProtossDragoon)}
		}},
		{"too mixed", []UnitShare{share(bwgame.UnitTypesProtossZealot, 300), share(bwgame.UnitTypesProtossDragoon, 300), share(bwgame.UnitTypesProtossHighTemplar, 200), share(bwgame.UnitTypesProtossArchon, 200)}, func(f Force) bool {
			return f.Mostly == 0 && f.Class == "" && f.MostlyPair == [2]uint16{}
		}},
	}
	for _, c := range cases {
		f := Force{Units: c.units}
		for _, u := range c.units {
			f.Value += u.Value
		}
		f.describe()
		if !c.check(f) {
			t.Errorf("%s: %+v", c.name, f)
		}
	}
}
