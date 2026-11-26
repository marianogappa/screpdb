package moments

import (
	"testing"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

var terranVsZerg = []Player{{Slot: 0, Race: openbw.RaceTerran, Human: true}, {Slot: 1, Race: openbw.RaceZerg, Human: true}}

type mechGame struct {
	lastSecond              int
	bioFrom, bioTo, bioStep int
	mechFrom, mechStep      int
	tanksBefore             []int
	factories               []int
}

// switching is a Terran making a Marine every 8 s from 3:00 to 11:00, two Tanks off the first Factory, four
// more Factories around 10:00 to 11:20, then a mech unit every 15 s.
func switching() mechGame {
	return mechGame{
		lastSecond: 1200, bioFrom: 180, bioTo: 660, bioStep: 8, mechFrom: 700, mechStep: 15,
		tanksBefore: []int{400, 500}, factories: []int{300, 600, 620, 640, 680},
	}
}

func (m mechGame) digest() *engine.Digest {
	d := &engine.Digest{
		LastFrame: f(m.lastSecond),
		Players:   []engine.Player{{Slot: 0}, {Slot: 1}},
		Initial: []engine.InitialUnit{
			{Unit: 1, Type: ut(bwgame.UnitTypesTerranCommandCenter), Owner: 0, X: 300, Y: 300},
			{Unit: 2, Type: ut(bwgame.UnitTypesZergHatchery), Owner: 1, X: 3700, Y: 3700},
		},
		Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame},
	}
	unit := uint32(100)
	add := func(t bwgame.UnitTypes, from bwgame.UnitTypes, sec int, x int32) {
		unit++
		d.Productions = append(d.Productions, engine.Production{Owner: 0, Type: ut(t), From: ut(from), Unit: unit, X: x, Y: 400, Start: f(sec), Complete: f(sec + 30), Cancel: engine.NoFrame})
	}
	for sec := m.bioFrom; sec <= m.bioTo; sec += m.bioStep {
		add(bwgame.UnitTypesTerranMarine, bwgame.UnitTypesTerranBarracks, sec, 300)
	}
	for _, sec := range m.tanksBefore {
		add(bwgame.UnitTypesTerranSiegeTankTankMode, bwgame.UnitTypesTerranFactory, sec, 300)
	}
	for i, sec := range m.factories {
		add(bwgame.UnitTypesTerranFactory, bwgame.UnitTypesTerranSCV, sec, 400+int32(i)*130)
	}
	if m.mechStep > 0 {
		kinds := []bwgame.UnitTypes{bwgame.UnitTypesTerranVulture, bwgame.UnitTypesTerranVulture, bwgame.UnitTypesTerranSiegeTankTankMode}
		for i, sec := 0, m.mechFrom; sec < m.lastSecond; i, sec = i+1, sec+m.mechStep {
			add(kinds[i%len(kinds)], bwgame.UnitTypesTerranFactory, sec, 300)
		}
	}
	return d
}

func TestMechTransitionIsTheLastBioUnitBeforeFactoriesTakeOver(t *testing.T) {
	got := ofType(Detect(switching().digest(), testBases, terranVsZerg), "mech_transition")
	if len(got) != 1 {
		t.Fatalf("want one mech transition: %+v", got)
	}
	e := got[0]
	if e.Actor != 0 || e.Target != 1 || e.Second != 690 || e.SnapshotSecond != 750 || e.Base != 0 {
		t.Fatalf("the switch is the last Marine out, shown a minute on in the Terran main: %+v", e)
	}
	if e.Count != 5 {
		t.Fatalf("five Factories stand by then: %d", e.Count)
	}
	want := []uint16{ut(bwgame.UnitTypesTerranFactory), ut(bwgame.UnitTypesTerranVulture), ut(bwgame.UnitTypesTerranSiegeTankTankMode)}
	if len(e.Units) != len(want) || e.Units[0] != want[0] || e.Units[1] != want[1] || e.Units[2] != want[2] {
		t.Fatalf("units are the Factory, then the mech made after, most first: %v", e.Units)
	}
	for _, x := range []int32{530, 660, 790, 920} {
		if !regionHolds(e.Region, x, 400) {
			t.Fatalf("region frames the new Factories: %+v", e.Region)
		}
	}
}

func TestMechTransitionNeedsBioToStopForGood(t *testing.T) {
	g := switching()
	g.bioTo = 1100
	if got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition"); len(got) != 0 {
		t.Fatalf("Barracks kept making bio beside the Factories: %+v", got)
	}
}

func TestMechTransitionNeedsABioArmyFirst(t *testing.T) {
	g := switching()
	for sec := 360; sec < 660; sec += 9 {
		g.tanksBefore = append(g.tanksBefore, sec)
	}
	if got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition"); len(got) != 0 {
		t.Fatalf("a Terran already making as much mech as bio does not transition: %+v", got)
	}
}

func TestMechTransitionNeedsNewFactories(t *testing.T) {
	g := switching()
	g.factories = []int{300, 340, 640}
	if got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition"); len(got) != 0 {
		t.Fatalf("one Factory added at the switch is not a transition: %+v", got)
	}
}

func TestMechTransitionIsNotAFourthFactory(t *testing.T) {
	g := switching()
	g.factories = []int{200, 250, 300, 600, 640}
	if got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition"); len(got) != 0 {
		t.Fatalf("two Factories added to three already standing: %+v", got)
	}
}

func TestMechTransitionShowsTheMapOnceTheFactoriesAreUp(t *testing.T) {
	g := switching()
	g.factories = []int{300, 600, 620, 700, 730}
	got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition")
	if len(got) != 1 || got[0].SnapshotSecond != 760 {
		t.Fatalf("the last new Factory finishes at 12:40: %+v", got)
	}
}

func TestMechTransitionNeedsMechAfter(t *testing.T) {
	g := switching()
	g.mechStep = 0
	if got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition"); len(got) != 0 {
		t.Fatalf("Factories that make nothing: %+v", got)
	}
}

func TestMechTransitionNeedsTheGameToGoOn(t *testing.T) {
	g := switching()
	g.lastSecond = 760
	if got := ofType(Detect(g.digest(), testBases, terranVsZerg), "mech_transition"); len(got) != 0 {
		t.Fatalf("a game over within two minutes of the last Marine cannot tell a switch from a loss: %+v", got)
	}
}

func TestMechTransitionIsTvZOnly(t *testing.T) {
	if got := ofType(Detect(switching().digest(), testBases, tvp), "mech_transition"); len(got) != 0 {
		t.Fatalf("TvP: %+v", got)
	}
}

func TestFactoryClusterKeepsTheFactoriesStandingTogether(t *testing.T) {
	burst := []engine.Production{{X: 300, Y: 300}, {X: 3700, Y: 3700}, {X: 400, Y: 350}, {X: 500, Y: 300}}
	if got := factoryCluster(burst); len(got) != 3 {
		t.Fatalf("the Factory at the far base stays out: %+v", got)
	}
}
