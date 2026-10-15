package moments

import (
	"sort"
	"testing"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func f(second int) int32 { return int32(second * framesPerSecond) }

func u(t bwgame.UnitTypes) uint16 { return uint16(t) }

func baseDigest() *engine.Digest {
	return &engine.Digest{
		LastFrame: f(900),
		Players:   []engine.Player{{Slot: 0, Race: 1}, {Slot: 1, Race: 2}},
		Drift:     engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame},
	}
}

func kill(sec int, victim int8, t bwgame.UnitTypes, x, y int32) engine.Death {
	return engine.Death{Frame: f(sec), Unit: uint32(sec*100 + int(x)), Type: u(t), Owner: victim, X: x, Y: y, Reason: uint8(openbw.ReasonDamage), KillerOwner: 1 - victim}
}

func TestFightsClusterInSpaceAndTime(t *testing.T) {
	d := baseDigest()
	for i := range 6 {
		d.Deaths = append(d.Deaths, kill(300+i*3, 0, bwgame.UnitTypesTerranMarine, 1000+int32(i)*10, 1000))
	}
	d.Deaths = append(d.Deaths, kill(305, 1, bwgame.UnitTypesProtossDragoon, 1020, 1010))
	d.Deaths = append(d.Deaths, kill(306, 1, bwgame.UnitTypesProtossZealot, 4000, 4000))
	d.Deaths = append(d.Deaths, kill(500, 0, bwgame.UnitTypesTerranSCV, 1000, 1000))
	sort.Slice(d.Deaths, func(i, j int) bool { return d.Deaths[i].Frame < d.Deaths[j].Frame })
	got := fights(d, 900)
	if len(got) != 1 {
		t.Fatalf("fights %+v", got)
	}
	m := got[0]
	if m.Second != 300 || m.SnapshotSecond != 298 || m.EndSecond != 315 {
		t.Fatalf("times %d %d %d", m.Second, m.SnapshotSecond, m.EndSecond)
	}
	if m.Actor != 1 || m.Target != 0 {
		t.Fatalf("actor %d target %d, want the Protoss who killed more value first", m.Actor, m.Target)
	}
	if m.Sides[0].Killed.Minerals != 300 || m.Sides[1].Lost.Value() != 300 || m.Sides[0].Lost.Value() != 175 || m.Sides[1].UnitsLost[u(bwgame.UnitTypesTerranMarine)] != 6 {
		t.Fatalf("sides %+v", m.Sides)
	}
	if m.Region.W < minRegionSide || m.Region.X > 1000 || m.Region.X+m.Region.W < 1050 {
		t.Fatalf("region %+v does not cover the deaths", m.Region)
	}
}

func TestExpansionsAndBasesLost(t *testing.T) {
	d := baseDigest()
	d.Productions = []engine.Production{
		{Owner: 0, Type: u(bwgame.UnitTypesTerranCommandCenter), From: uint16(openbw.UnitTypeNone), Unit: 10, X: 3000, Y: 500, Start: f(200), Complete: f(275), Cancel: engine.NoFrame},
		{Owner: 1, Type: u(bwgame.UnitTypesZergLair), From: u(bwgame.UnitTypesZergHatchery), Unit: 11, Start: f(250), Complete: engine.NoFrame, Cancel: engine.NoFrame},
		{Owner: 1, Type: u(bwgame.UnitTypesProtossNexus), From: uint16(openbw.UnitTypeNone), Unit: 12, Start: f(400), Complete: engine.NoFrame, Cancel: f(410)},
	}
	d.Deaths = []engine.Death{kill(600, 0, bwgame.UnitTypesTerranCommandCenter, 3000, 500)}
	ms := Find(d, nil)
	var kinds []Kind
	for _, m := range ms {
		kinds = append(kinds, m.Kind)
	}
	if len(ms) != 3 || ms[0].Kind != KindExpansion || ms[0].Cancelled || !ms[1].Cancelled || ms[2].Kind != KindBaseLost || ms[2].Actor != 1 || ms[2].Target != 0 {
		t.Fatalf("moments %v %+v", kinds, ms)
	}
}

func TestMomentsStopAtTheTrustedFrame(t *testing.T) {
	d := baseDigest()
	d.Drift.DesyncFrame = f(250)
	d.Productions = []engine.Production{{Owner: 0, Type: u(bwgame.UnitTypesTerranCommandCenter), From: uint16(openbw.UnitTypeNone), Start: f(300), Complete: engine.NoFrame, Cancel: engine.NoFrame}}
	if ms := Find(d, nil); len(ms) != 0 {
		t.Fatalf("moments after the desync: %+v", ms)
	}
}

func TestSnapshotBuildings(t *testing.T) {
	d := baseDigest()
	d.Initial = []engine.InitialUnit{{Unit: 1, Type: u(bwgame.UnitTypesZergHatchery), Owner: 1, X: 100, Y: 100}, {Unit: 2, Type: u(bwgame.UnitTypesZergDrone), Owner: 1}}
	d.Productions = []engine.Production{
		{Owner: 1, Type: u(bwgame.UnitTypesZergLair), From: u(bwgame.UnitTypesZergHatchery), Unit: 1, Start: f(200), Complete: f(300), Cancel: engine.NoFrame},
		{Owner: 1, Type: u(bwgame.UnitTypesZergSpawningPool), From: u(bwgame.UnitTypesZergDrone), Unit: 3, X: 300, Y: 300, Start: f(60), Complete: f(120), Cancel: engine.NoFrame},
		{Owner: 1, Type: u(bwgame.UnitTypesZergExtractor), Unit: 4, X: 500, Y: 500, Start: f(80), Complete: f(110), Cancel: engine.NoFrame},
		{Owner: 1, Type: u(bwgame.UnitTypesZergExtractor), Unit: 4, X: 500, Y: 500, Start: f(400), Complete: engine.NoFrame, Cancel: engine.NoFrame},
		{Owner: 1, Type: u(bwgame.UnitTypesZergEvolutionChamber), Unit: 5, X: 700, Y: 700, Start: f(100), Complete: engine.NoFrame, Cancel: f(130)},
	}
	d.Deaths = []engine.Death{{Frame: f(350), Unit: 4, Type: u(bwgame.UnitTypesZergExtractor), Owner: 1}}
	d.Army = []engine.ArmySnapshot{{Frame: f(240), Units: []engine.ArmyUnit{{Owner: 1, Type: u(bwgame.UnitTypesZergZergling), X: 10, Y: 20}}}, {Frame: f(480)}}

	types := func(s Snapshot) map[uint16]bool {
		out := map[uint16]bool{}
		for _, b := range s.Buildings {
			out[b.Type] = b.Complete
		}
		return out
	}
	at := SnapshotAt(d, 250)
	if got := types(at); !got[u(bwgame.UnitTypesZergHatchery)] || !got[u(bwgame.UnitTypesZergSpawningPool)] || len(got) != 3 {
		t.Fatalf("at 250: %+v", at.Buildings)
	}
	if at.ArmySecond != 240 || len(at.Units) != 1 || at.Units[0].X != 10*engine.ArmyCellPixels+engine.ArmyCellPixels/2 {
		t.Fatalf("army %+v", at)
	}
	if got := types(SnapshotAt(d, 120)); len(got) != 4 {
		t.Fatalf("at 120 want hatch, pool, extractor and the evolution chamber under construction: %+v", got)
	}
	later := types(SnapshotAt(d, 420))
	if !later[u(bwgame.UnitTypesZergLair)] || later[u(bwgame.UnitTypesZergExtractor)] {
		t.Fatalf("at 420 want a finished Lair and a rebuilding Extractor: %+v", later)
	}
	if _, ok := types(SnapshotAt(d, 360))[u(bwgame.UnitTypesZergExtractor)]; ok {
		t.Fatal("the destroyed Extractor still stands")
	}
}
