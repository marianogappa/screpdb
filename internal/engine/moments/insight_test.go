package moments

import (
	"slices"
	"testing"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

var tvz = []Player{{Slot: 0, Race: openbw.RaceZerg, Human: true}, {Slot: 1, Race: openbw.RaceTerran, Human: true}}

func tvzDigest() *engine.Digest {
	d := digest()
	d.EngineVersion = engine.EngineVersion
	d.Initial[1].Type = ut(bwgame.UnitTypesTerranCommandCenter)
	return d
}

func killed(sec int, unit uint32, victim bwgame.UnitTypes, owner int8, x, y int32, killer bwgame.UnitTypes, by int8) engine.Death {
	return engine.Death{Frame: f(sec), Unit: unit, Type: ut(victim), Owner: owner, X: x, Y: y, Reason: 1, KillerType: ut(killer), KillerOwner: by}
}

func drones(from, n int, killer bwgame.UnitTypes) []engine.Death {
	var out []engine.Death
	for i := range n {
		out = append(out, killed(from+i, uint32(100+i), bwgame.UnitTypesZergDrone, 0, 300+int32(i)*8, 320, killer, 1))
	}
	return out
}

func TestEraserNeedsAVesselIrradiatedByItsOwnSide(t *testing.T) {
	d := tvzDigest()
	d.Deaths = drones(605, 6, bwgame.UnitTypesTerranScienceVessel)
	d.Casts = []engine.Cast{{Frame: f(600), Owner: 1, Order: orderIrradiate, X: 300, Y: 300, CasterType: vessel, Target: 7, TargetType: vessel, TargetOwner: 1}}
	in := DetectInsights(d, testBases, tvz)
	i := slices.IndexFunc(in.Events, func(e Event) bool { return e.Type == InsightEraser })
	if i < 0 || in.Events[i].Outcome.Workers != 6 || len(in.Events[i].Path) != 6 {
		t.Fatalf("eraser %+v", in.Events)
	}

	d.Casts[0].TargetType, d.Casts[0].TargetOwner = ut(bwgame.UnitTypesZergDrone), 0
	if slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightEraser }) {
		t.Fatal("Irradiate on an enemy Drone is not an Eraser")
	}
}

func TestEraserNeedsCastTargets(t *testing.T) {
	d := tvzDigest()
	d.EngineVersion = 11
	d.Deaths = drones(605, 6, bwgame.UnitTypesTerranScienceVessel)
	d.Casts = []engine.Cast{{Frame: f(600), Owner: 1, Order: orderIrradiate, TargetType: vessel, TargetOwner: 1}}
	if slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightEraser }) {
		t.Fatal("a digest without cast targets cannot prove an Eraser")
	}
}

func TestUndefendedAirHarassIsTheWhyWhenTheVictimLeaves(t *testing.T) {
	d := tvzDigest()
	for i := range 8 {
		d.Deaths = append(d.Deaths, killed(400+i, uint32(200+i), bwgame.UnitTypesTerranSCV, 1, 3700+int32(i)*8, 3720, bwgame.UnitTypesZergMutalisk, 0))
	}
	d.Outcomes = []engine.Outcome{{Frame: f(500), Owner: 1, Kind: engine.OutcomeLeft}, {Frame: f(500), Owner: 0, Kind: engine.OutcomeWon}}
	in := DetectInsights(d, testBases, tvz)
	if in.Why == nil || in.Why.Loser != 1 || in.Events[in.Why.Event].Type != InsightUndefendedAir {
		t.Fatalf("why %+v events %+v", in.Why, in.Events)
	}

	d.Productions = []engine.Production{{Owner: 1, Type: ut(bwgame.UnitTypesTerranMissileTurret), Unit: 9, X: 3650, Y: 3650, Start: f(300), Complete: f(350), Cancel: engine.NoFrame}}
	if in := DetectInsights(d, testBases, tvz); len(in.Events) != 0 || in.Why != nil {
		t.Fatalf("a Missile Turret in the base is defence: %+v", in.Events)
	}
}

func TestOverlordsShotDownInADropAreNotAHunt(t *testing.T) {
	d := tvzDigest()
	for i := range 9 {
		d.Deaths = append(d.Deaths, killed(500+i, uint32(300+i), bwgame.UnitTypesZergOverlord, 0, 2000, 2000, bwgame.UnitTypesTerranMarine, 1))
	}
	if !slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightOverlordHunt }) {
		t.Fatal("nine Overlords killed in a minute is a hunt")
	}
	d.Unloads = []engine.Unload{{Frame: f(498), Owner: 0, TransportType: ut(bwgame.UnitTypesZergOverlord), X: 2010, Y: 2010, Cargo: []uint16{ut(bwgame.UnitTypesZergHydralisk)}}}
	if slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightOverlordHunt }) {
		t.Fatal("Overlords dying beside their own unload are a drop shot down")
	}
}

func walledBases() []Base {
	bases := slices.Clone(testBases)
	bases[0].Polygon = [][2]int32{{0, 0}, {600, 0}, {600, 600}, {0, 600}}
	return bases
}

func hopUnload(sec int, transport uint32, loadX, loadY, x, y int32) engine.Unload {
	marine := ut(bwgame.UnitTypesTerranMarine)
	return engine.Unload{Frame: f(sec), Transport: transport, TransportType: ut(bwgame.UnitTypesTerranDropship), Owner: 1, X: x, Y: y, Cargo: []uint16{marine, marine}, LoadFrame: f(sec - 5), LoadX: loadX, LoadY: loadY}
}

func elevated(d *engine.Digest) bool {
	return slices.ContainsFunc(DetectInsights(d, walledBases(), tvz).Events, func(e Event) bool { return e.Type == InsightElevator })
}

func TestElevatorHopsTheSameWallTwice(t *testing.T) {
	d := tvzDigest()
	d.Unloads = []engine.Unload{hopUnload(400, 7, 660, 300, 560, 300), hopUnload(430, 7, 650, 320, 550, 310)}
	if !elevated(d) {
		t.Fatal("two hops over the same wall are an elevator")
	}
	d.Unloads = d.Unloads[:1]
	if elevated(d) {
		t.Fatal("one hop is a drop")
	}
	d.Unloads = []engine.Unload{hopUnload(400, 7, 660, 300, 560, 300), hopUnload(430, 7, 660, 300, 300, 300)}
	if elevated(d) {
		t.Fatal("an unload in the middle of the base did not hop the wall")
	}
	d.Unloads = []engine.Unload{hopUnload(400, 7, 1400, 300, 560, 300), hopUnload(430, 7, 1400, 300, 550, 310)}
	if elevated(d) {
		t.Fatal("a long flight in is a drop, not a hop")
	}
	d.Unloads = []engine.Unload{hopUnload(400, 7, 660, 300, 560, 300), hopUnload(401, 8, 650, 320, 550, 310), hopUnload(402, 7, 660, 300, 560, 300)}
	if elevated(d) {
		t.Fatal("two Dropships hopping together, unloading over a few seconds, are one trip")
	}
}

func TestOverlordHuntIsOneArea(t *testing.T) {
	d := tvzDigest()
	for i := range 9 {
		d.Deaths = append(d.Deaths, killed(500+30*i, uint32(300+i), bwgame.UnitTypesZergOverlord, 0, 300+int32(i)*450, 2000, bwgame.UnitTypesProtossDragoon, 1))
	}
	if slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightOverlordHunt }) {
		t.Fatal("Overlords picked off across the map are not a hunt")
	}
}

func TestOverlordHuntSkipsAZergWhoHasLost(t *testing.T) {
	d := tvzDigest()
	for i := range 9 {
		d.Deaths = append(d.Deaths, killed(500+i, uint32(300+i), bwgame.UnitTypesZergOverlord, 0, 2000, 2000, bwgame.UnitTypesTerranMarine, 1))
	}
	d.Economy = []engine.EconomySample{{Frame: f(490), Owner: 0, SupplyUsed: 130, SupplyMax: 200, Workers: 11}, {Frame: f(490), Owner: 1, SupplyUsed: 370, SupplyMax: 400, Workers: 63}}
	if slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightOverlordHunt }) {
		t.Fatal("Overlords of a Zerg with a third of the enemy's supply and a sixth of its workers are not a hunt")
	}
}

func TestTurretsKilledOnTheWayInStillDefended(t *testing.T) {
	d := tvzDigest()
	for i := range 8 {
		d.Deaths = append(d.Deaths, killed(400+i, uint32(200+i), bwgame.UnitTypesTerranSCV, 1, 3700+int32(i)*8, 3720, bwgame.UnitTypesZergMutalisk, 0))
	}
	d.Productions = []engine.Production{{Owner: 1, Type: ut(bwgame.UnitTypesTerranMissileTurret), Unit: 9, X: 3650, Y: 3650, Start: f(300), Complete: f(350), Cancel: engine.NoFrame}}
	d.Deaths = append([]engine.Death{killed(380, 9, bwgame.UnitTypesTerranMissileTurret, 1, 3650, 3650, bwgame.UnitTypesZergMutalisk, 0)}, d.Deaths...)
	if slices.ContainsFunc(DetectInsights(d, testBases, tvz).Events, func(e Event) bool { return e.Type == InsightUndefendedAir }) {
		t.Fatal("a turret the Mutalisks killed first was anti-air")
	}
}

func containBases() []Base {
	bases := slices.Clone(testBases)
	bases[2].Polygon = [][2]int32{{3400, 3400}, {4000, 3400}, {4000, 4000}, {3400, 4000}}
	bases[2].Chokes = []Choke{{X: 3350, Y: 3700, To: NoBase}}
	return bases
}

func containSnapshots(from, to int, drift int32) []engine.ArmySnapshot {
	var out []engine.ArmySnapshot
	for sec := from; sec <= to; sec += 10 {
		var units []engine.ArmyUnit
		for i := range 4 {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: ut(bwgame.UnitTypesZergLurker), ID: uint32(50 + i), X: (3150 + int32(sec-from)*drift) / engine.ArmyCellPixels, Y: (3640 + int32(i)*40) / engine.ArmyCellPixels})
		}
		for i := range 8 {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: ut(bwgame.UnitTypesZergHydralisk), ID: uint32(60 + i), X: 3000 / engine.ArmyCellPixels, Y: (3600 + int32(i)*30) / engine.ArmyCellPixels})
		}
		out = append(out, engine.ArmySnapshot{Frame: f(sec), Units: units})
	}
	return out
}

func contained(d *engine.Digest) bool {
	return slices.ContainsFunc(DetectInsights(d, containBases(), tvz).Events, func(e Event) bool {
		return e.Type == InsightLurkerContain && e.Actor == 0 && e.Target == 1
	})
}

func TestLurkersHeldOutsideTheChokeAreAContain(t *testing.T) {
	d := tvzDigest()
	d.Army = containSnapshots(400, 490, 0)
	if !contained(d) {
		t.Fatal("four Lurkers and eight Hydralisks held outside the main's choke for 90 s are a contain")
	}
	d.Army = containSnapshots(400, 430, 0)
	if contained(d) {
		t.Fatal("thirty seconds is not a contain")
	}
	d.Army = containSnapshots(400, 490, 4)
	if contained(d) {
		t.Fatal("Lurkers walking past the choke are not holding it")
	}
}
