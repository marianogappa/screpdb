package moments

import (
	"slices"
	"testing"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

var tvp = []Player{{Slot: 0, Race: openbw.RaceTerran, Human: true}, {Slot: 1, Race: openbw.RaceProtoss, Human: true}}

func addDeath(d *engine.Digest, sec int, owner int8, t bwgame.UnitTypes, x, y int32, killer int8, killerType bwgame.UnitTypes) {
	d.Deaths = append(d.Deaths, engine.Death{
		Frame: f(sec), Unit: uint32(1000 + len(d.Deaths)), Type: ut(t), Owner: owner, X: x, Y: y,
		Reason: uint8(openbw.ReasonDamage), KillerOwner: killer, KillerType: ut(killerType),
	})
}

func ofType(events []Event, kind string) []Event {
	var out []Event
	for _, e := range events {
		if e.Type == kind {
			out = append(out, e)
		}
	}
	return out
}

func building(owner int8, t bwgame.UnitTypes, unit uint32, x, y int32, start, complete, cancel int32) engine.Production {
	return engine.Production{Owner: owner, Type: ut(t), From: ut(bwgame.UnitTypesZergDrone), Unit: unit, X: x, Y: y, Start: start, Complete: complete, Cancel: cancel}
}

func regionHolds(r Region, x, y int32) bool {
	return x >= r.X && x <= r.X+r.W && y >= r.Y && y <= r.Y+r.H
}

func TestFieldBattleIsADecisiveFightAwayFromBases(t *testing.T) {
	d := digest()
	addDeath(d, 700, 1, bwgame.UnitTypesProtossDragoon, 2000, 2000, 0, bwgame.UnitTypesZergHydralisk)
	for range 14 {
		addDeath(d, 720, 1, bwgame.UnitTypesProtossDragoon, 2010, 1990, 0, bwgame.UnitTypesZergHydralisk)
	}
	for range 5 {
		addDeath(d, 721, 0, bwgame.UnitTypesZergHydralisk, 1990, 2010, 1, bwgame.UnitTypesProtossDragoon)
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 {
		t.Fatalf("want one field battle: %+v", got)
	}
	e := got[0]
	if e.Type != "attack" || e.Actor != 0 || e.Target != 1 || e.Base != NoBase || e.Second != 700 || e.SnapshotSecond != 719 {
		t.Fatalf("the side that killed more is the attacker, shown at the peak: %+v", e)
	}
	if e.Outcome.Killed != 15*175 || e.Outcome.Lost != 5*100 || e.Outcome.Workers != 0 || e.Outcome.TownHall {
		t.Fatalf("outcome: %+v", e.Outcome)
	}
	if !slices.Equal(e.Units, []uint16{ut(bwgame.UnitTypesZergHydralisk)}) {
		t.Fatalf("units are the winner's killers: %v", e.Units)
	}
	if !regionHolds(e.Region, 2000, 2000) || e.Region.W < minRegionSide || e.Region.H < minRegionSide {
		t.Fatalf("region frames the fight: %+v", e.Region)
	}
}

func TestFieldBattleBelowTheTradeIsDropped(t *testing.T) {
	d := digest()
	for range 10 {
		addDeath(d, 720, 1, bwgame.UnitTypesProtossDragoon, 2000, 2000, 0, bwgame.UnitTypesZergHydralisk)
	}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("1750 traded is a skirmish: %v", types(got))
	}
}

func TestFieldBattleIgnoresATownHallDyingInAHeldBase(t *testing.T) {
	d := digest()
	d.Deaths = append(d.Deaths, engine.Death{Frame: f(800), Unit: 2, Type: ut(bwgame.UnitTypesProtossNexus), Owner: 1, X: 3700, Y: 3700, Reason: uint8(openbw.ReasonDamage), KillerOwner: 0, KillerType: ut(bwgame.UnitTypesZergUltralisk)})
	for range 16 {
		addDeath(d, 800, 1, bwgame.UnitTypesProtossDragoon, 3700, 3700, 0, bwgame.UnitTypesZergUltralisk)
	}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("a fight in a base held until that second is not open field: %+v", got)
	}
}

func TestFieldBattleNearAKeptAttackIsDeduped(t *testing.T) {
	d := digest()
	for sec := 500; sec <= 520; sec += 10 {
		d.Army = append(d.Army, army(sec, 1, bwgame.UnitTypesProtossZealot, 6, 350, 350))
	}
	for i := range 4 {
		addDeath(d, 505+i, 0, bwgame.UnitTypesZergDrone, 340, 340, 1, bwgame.UnitTypesProtossZealot)
	}
	for range 20 {
		addDeath(d, 540, 0, bwgame.UnitTypesZergHydralisk, 2000, 2000, 1, bwgame.UnitTypesProtossDragoon)
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Base != 0 || got[0].Second != 500 {
		t.Fatalf("a field battle 40 s after the same player's attack is the same moment: %+v", got)
	}
}

func TestWorkerKills(t *testing.T) {
	probes := func(d *engine.Digest, n int, killerType bwgame.UnitTypes) {
		for i := range n {
			addDeath(d, 600+i, 1, bwgame.UnitTypesProtossProbe, 3650, 3650, 0, killerType)
		}
	}
	d := digest()
	probes(d, 5, bwgame.UnitTypesZergMutalisk)
	got := Detect(d, testBases, zvp)
	if len(got) != 1 {
		t.Fatalf("five Probes killed: %+v", got)
	}
	e := got[0]
	if e.Type != "worker_kills" || e.Actor != 0 || e.Target != 1 || e.Base != 2 || e.Second != 600 || e.SnapshotSecond != 600 {
		t.Fatalf("worker kills: %+v", e)
	}
	if e.Outcome.Workers != 5 || e.Outcome.Killed != 250 || !slices.Equal(e.Units, []uint16{ut(bwgame.UnitTypesZergMutalisk)}) {
		t.Fatalf("worker kills outcome: %+v %v", e.Outcome, e.Units)
	}

	d = digest()
	probes(d, 4, bwgame.UnitTypesZergMutalisk)
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("four workers are not worth a moment: %v", types(got))
	}

	d = digest()
	for i := range 6 {
		addDeath(d, 600+i, 1, bwgame.UnitTypesProtossProbe, 2000, 2000, 0, bwgame.UnitTypesZergMutalisk)
	}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("workers killed away from any base: %v", types(got))
	}

	d = digest()
	probes(d, 5, bwgame.UnitTypesZergZergling)
	for sec := 595; sec <= 615; sec += 10 {
		d.Army = append(d.Army, army(sec, 0, bwgame.UnitTypesZergZergling, 6, 3650, 3650))
	}
	got = Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "attack" || got[0].Outcome.Workers != 5 {
		t.Fatalf("an attack already tells the worker kills: %+v", got)
	}

	d = digest()
	for i := range 5 {
		addDeath(d, 600+i, 0, bwgame.UnitTypesZergDrone, 350, 350, 1, bwgame.UnitTypesProtossInterceptor)
	}
	addDeath(d, 606, 0, bwgame.UnitTypesZergDrone, 350, 350, 1, bwgame.UnitTypesProtossScarab)
	got = Detect(d, testBases, zvp)
	want := []uint16{ut(bwgame.UnitTypesProtossCarrier), ut(bwgame.UnitTypesProtossReaver)}
	if len(got) != 1 || got[0].Type != "worker_kills" || !slices.Equal(got[0].Units, want) {
		t.Fatalf("Interceptor and Scarab kills belong to their Carrier and Reaver: %+v", got)
	}
}

func TestFighter(t *testing.T) {
	for in, want := range map[bwgame.UnitTypes]bwgame.UnitTypes{
		bwgame.UnitTypesProtossInterceptor: bwgame.UnitTypesProtossCarrier,
		bwgame.UnitTypesProtossScarab:      bwgame.UnitTypesProtossReaver,
		bwgame.UnitTypesZergMutalisk:       bwgame.UnitTypesZergMutalisk,
	} {
		if got := fighter(ut(in)); got != ut(want) {
			t.Errorf("fighter(%v) = %v, want %v", in, got, want)
		}
	}
}

func economy(sec int, owner int8, used, maxSupply, minerals int32) engine.EconomySample {
	return engine.EconomySample{Frame: f(sec), Owner: owner, SupplyUsed: used, SupplyMax: maxSupply, Minerals: minerals}
}

func TestSupplyBlocks(t *testing.T) {
	block := func(until int, minerals int32) *engine.Digest {
		d := digest()
		for sec := 100; sec < until; sec += 5 {
			d.Economy = append(d.Economy, economy(sec, 0, 36, 36, minerals))
		}
		d.Economy = append(d.Economy, economy(until, 0, 36, 52, minerals))
		return d
	}
	got := Detect(block(160, 150), testBases, zvp)
	if len(got) != 1 {
		t.Fatalf("a minute blocked with money: %+v", got)
	}
	e := got[0]
	if e.Type != "long_supply_block" || e.Actor != 0 || e.Base != 0 || e.Second != 100 || e.Seconds != 60 || e.SnapshotSecond != 130 {
		t.Fatalf("supply block: %+v", e)
	}
	if !regionHolds(e.Region, 300, 300) {
		t.Fatalf("region shows the main: %+v", e.Region)
	}
	if got := Detect(block(140, 150), testBases, zvp); len(got) != 0 {
		t.Fatalf("40 s blocked is the ordinary wait for a depot: %+v", got)
	}
	if got := Detect(block(160, 50), testBases, zvp); len(got) != 0 {
		t.Fatalf("blocked with no money to spend: %+v", got)
	}

	d := digest()
	d.Economy = []engine.EconomySample{economy(1100, 0, 36, 36, 500), economy(1110, 0, 36, 36, 500)}
	got = Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Seconds != 100 {
		t.Fatalf("a block still open at the end runs to the last frame: %+v", got)
	}

	d = digest()
	d.Economy = []engine.EconomySample{economy(100, 0, 400, 400, 500), economy(200, 0, 400, 400, 500)}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("maxed out is not supply blocked: %+v", got)
	}

	d = block(160, 150)
	if got := Detect(d, testBases, []Player{{Slot: 0, Race: openbw.RaceZerg}, {Slot: 1, Race: openbw.RaceProtoss, Human: true}}); len(got) != 0 {
		t.Fatalf("a computer's supply block: %+v", got)
	}
}

func TestCannonRushNextToAnEnemyNatural(t *testing.T) {
	d := digest()
	d.Productions = []engine.Production{
		building(0, bwgame.UnitTypesZergHatchery, 10, 900, 400, f(100), f(170), engine.NoFrame),
		building(1, bwgame.UnitTypesProtossPhotonCannon, 11, 900, 950, f(200), f(240), engine.NoFrame),
		building(1, bwgame.UnitTypesProtossPhotonCannon, 12, 920, 950, f(210), f(250), engine.NoFrame),
	}
	got := ofType(Detect(d, testBases, zvp), "cannon_rush")
	if len(got) != 1 {
		t.Fatalf("one cannon rush per player: %+v", got)
	}
	e := got[0]
	if e.Actor != 1 || e.Target != 0 || e.Base != 1 || e.Second != 200 || e.SnapshotSecond != 202 || !regionHolds(e.Region, 900, 950) {
		t.Fatalf("cannon rush: %+v", e)
	}

	d.Productions = d.Productions[:2]
	d.Productions[1].Start = f(cannonRushBy + 1)
	if got := ofType(Detect(d, testBases, zvp), "cannon_rush"); len(got) != 0 {
		t.Fatalf("a cannon after 4:00 is not a rush: %+v", got)
	}

	d.Productions[1].Start = f(200)
	d.Productions[1].X, d.Productions[1].Y = 1500, 2000
	if got := ofType(Detect(d, testBases, zvp), "cannon_rush"); len(got) != 0 {
		t.Fatalf("a cannon in the middle of the map is not a rush: %+v", got)
	}
}

func TestBunkerRushNeedsMarines(t *testing.T) {
	zvt := []Player{{Slot: 0, Race: openbw.RaceZerg, Human: true}, {Slot: 1, Race: openbw.RaceTerran, Human: true}}
	d := digest()
	d.Productions = []engine.Production{building(1, bwgame.UnitTypesTerranBunker, 20, 400, 450, f(250), f(280), engine.NoFrame)}
	if got := ofType(Detect(d, testBases, zvt), "bunker_rush"); len(got) != 0 {
		t.Fatalf("a bunker with no Marines: %+v", got)
	}
	d.Army = []engine.ArmySnapshot{army(240, 1, bwgame.UnitTypesTerranMarine, 2, 420, 460), army(320, 1, bwgame.UnitTypesTerranMarine, 2, 420, 460)}
	if got := ofType(Detect(d, testBases, zvt), "bunker_rush"); len(got) != 0 {
		t.Fatalf("Marines before the bunker and after a minute do not count: %+v", got)
	}
	d.Army = []engine.ArmySnapshot{army(255, 1, bwgame.UnitTypesTerranMarine, 2, 3000, 3000), army(260, 1, bwgame.UnitTypesTerranMarine, 2, 420, 460)}
	got := ofType(Detect(d, testBases, zvt), "bunker_rush")
	want := []uint16{ut(bwgame.UnitTypesTerranBunker), ut(bwgame.UnitTypesTerranMarine)}
	if len(got) != 1 || got[0].Actor != 1 || got[0].Target != 0 || got[0].Base != 0 || !slices.Equal(got[0].Units, want) {
		t.Fatalf("bunker rush: %+v", got)
	}
}

func TestProxies(t *testing.T) {
	gate := func(x, y int32, sec int) *engine.Digest {
		d := digest()
		d.Productions = []engine.Production{building(1, bwgame.UnitTypesProtossGateway, 30, x, y, f(sec), f(sec+60), engine.NoFrame)}
		return d
	}
	got := Detect(gate(1500, 1500, 100), testBases, zvp)
	if len(got) != 1 || got[0].Type != "proxy_gate" || got[0].Actor != 1 || got[0].Base != NoBase || got[0].SnapshotSecond != 106 {
		t.Fatalf("a Gateway far forward: %+v", got)
	}
	for name, d := range map[string]*engine.Digest{
		"in its own main":      gate(3500, 3500, 100),
		"at its own natural":   gate(3100, 3600, 100),
		"near home":            gate(3000, 3000, 100),
		"too late":             gate(1500, 1500, proxyEarlyBy+1),
		"behind the enemy far": gate(100, 3900, 100),
	} {
		if got := Detect(d, testBases, zvp); len(got) != 0 {
			t.Errorf("%s is no proxy: %+v", name, got)
		}
	}
	d := gate(1500, 1500, 100)
	d.Productions = append(d.Productions, building(1, bwgame.UnitTypesProtossGateway, 31, 1520, 1500, f(110), f(170), engine.NoFrame))
	if got := Detect(d, testBases, zvp); len(got) != 1 {
		t.Fatalf("one proxy_gate per player: %+v", got)
	}
	ffa := append(slices.Clone(zvp), Player{Slot: 2, Race: openbw.RaceTerran, Human: true})
	if got := Detect(gate(1500, 1500, 100), testBases, ffa); len(got) != 0 {
		t.Fatalf("proxies are a two-player rule: %+v", got)
	}
}

func TestMannerPylon(t *testing.T) {
	d := digest()
	d.Productions = []engine.Production{building(1, bwgame.UnitTypesProtossPylon, 40, 350, 350, f(300), f(318), engine.NoFrame)}
	got := Detect(d, testBases, tvp)
	if len(got) != 1 || got[0].Type != "manner_pylon" || got[0].Actor != 1 || got[0].Target != 0 || got[0].Base != 0 || got[0].SnapshotSecond != 306 {
		t.Fatalf("a Pylon in a Terran main: %+v", got)
	}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("Zerg is not mannered by a Pylon: %+v", got)
	}
	d.Productions[0].Start = f(mannerPylonBy + 1)
	if got := Detect(d, testBases, tvp); len(got) != 0 {
		t.Fatalf("a late Pylon in the main: %+v", got)
	}
}

func TestNukes(t *testing.T) {
	d := digest()
	d.Casts = []engine.Cast{
		{Frame: f(900), Owner: 0, Order: orderNukePaint, X: 3650, Y: 3650},
		{Frame: f(950), Owner: 0, Order: orderNukePaint, X: 2000, Y: 400},
		{Frame: f(960), Owner: 0, Order: orderRecall, X: 3650, Y: 3650},
	}
	got := Detect(d, testBases, tvp)
	if len(got) != 1 || got[0].Type != "nuke" || got[0].Target != 1 || got[0].Base != 2 || got[0].Second != 900 || got[0].Units[0] != ut(bwgame.UnitTypesTerranGhost) {
		t.Fatalf("one nuke on an enemy base: %+v", got)
	}
}

func TestNydusAttacks(t *testing.T) {
	d := digest()
	tp := func(sec int, toX, toY int32, n int) engine.Teleport {
		units := make([]uint16, n)
		for i := range units {
			units[i] = ut(bwgame.UnitTypesZergHydralisk)
		}
		return engine.Teleport{Frame: f(sec), Owner: 0, FromX: 300, FromY: 300, ToX: toX, ToY: toY, Units: units}
	}
	d.Teleports = []engine.Teleport{
		tp(600, 3650, 3650, 1),
		tp(610, 3650, 3650, 3),
		tp(700, 3650, 3650, 3),
		tp(800, 3650, 3650, 3),
		tp(820, 2000, 400, 3),
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 2 || got[0].Type != "nydus_attack" || got[0].Second != 610 || got[1].Second != 800 {
		t.Fatalf("nydus at 10:10 and again three minutes later: %+v", got)
	}
	if got[0].Target != 1 || got[0].Base != 2 || got[0].SnapshotSecond != 611 || !slices.Equal(got[0].Units, []uint16{ut(bwgame.UnitTypesZergHydralisk)}) {
		t.Fatalf("nydus: %+v", got[0])
	}
}

func TestRecallIntoAnUnheldBaseHasNoTarget(t *testing.T) {
	d := digest()
	d.Teleports = []engine.Teleport{{Frame: f(700), Owner: 1, FromX: 3600, FromY: 3600, ToX: 2000, ToY: 400, Units: []uint16{65, 65}}}
	d.Casts = []engine.Cast{{Frame: f(680), Owner: 1, Order: orderRecall, X: 2000, Y: 400}}
	if got := Detect(d, testBases, zvp); len(got) != 0 {
		t.Fatalf("a Recall cast 20 s earlier is not this one: %+v", got)
	}
	d.Casts[0].Frame = f(695)
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "recall" || got[0].Target != NoPlayer || got[0].Base != 4 || got[0].Units[0] != ut(bwgame.UnitTypesProtossArbiter) {
		t.Fatalf("recall into the open expansion: %+v", got)
	}
}

func TestDropOutcome(t *testing.T) {
	d := digest()
	d.Unloads = []engine.Unload{
		{Frame: f(600), TransportType: ut(bwgame.UnitTypesProtossShuttle), Owner: 1, X: 350, Y: 350, Cargo: []uint16{ut(bwgame.UnitTypesProtossZealot)}},
		{Frame: f(605), TransportType: ut(bwgame.UnitTypesProtossShuttle), Owner: 1, X: 360, Y: 360, Cargo: []uint16{ut(bwgame.UnitTypesProtossZealot), ut(bwgame.UnitTypesProtossProbe)}},
	}
	for i := range 3 {
		addDeath(d, 610+i, 0, bwgame.UnitTypesZergDrone, 300+int32(i)*40, 300, 1, bwgame.UnitTypesProtossZealot)
	}
	addDeath(d, 615, 1, bwgame.UnitTypesProtossZealot, 320, 320, 0, bwgame.UnitTypesZergZergling)
	got := Detect(d, testBases, zvp)
	if len(got) != 1 {
		t.Fatalf("one drop from two unloads: %+v", got)
	}
	e := got[0]
	if e.Type != "drop" || e.Actor != 1 || e.Target != 0 || e.Second != 600 || e.SnapshotSecond != 604 {
		t.Fatalf("drop: %+v", e)
	}
	if !slices.Equal(e.Units, []uint16{ut(bwgame.UnitTypesProtossShuttle), ut(bwgame.UnitTypesProtossZealot)}) {
		t.Fatalf("a Probe in the cargo is not part of the drop: %v", e.Units)
	}
	if e.Outcome.Killed != 150 || e.Outcome.Lost != 100 || e.Outcome.Workers != 3 {
		t.Fatalf("drop outcome: %+v", e.Outcome)
	}
	if !regionHolds(e.Region, 380, 300) {
		t.Fatalf("region frames the deaths: %+v", e.Region)
	}
}

func TestAttackOutcomeAndLateFightingMovesTheSecond(t *testing.T) {
	d := digest()
	for sec := 500; sec <= 550; sec += 10 {
		d.Army = append(d.Army, army(sec, 1, bwgame.UnitTypesProtossZealot, 6, 350, 350))
	}
	for i := range 4 {
		addDeath(d, 550+i, 0, bwgame.UnitTypesZergDrone, 340, 340, 1, bwgame.UnitTypesProtossZealot)
	}
	addDeath(d, 552, 1, bwgame.UnitTypesProtossZealot, 360, 360, 0, bwgame.UnitTypesZergDrone)
	d.Deaths = append(d.Deaths, engine.Death{Frame: f(555), Unit: 1, Type: ut(bwgame.UnitTypesZergHatchery), Owner: 0, X: 300, Y: 300, Reason: uint8(openbw.ReasonDamage), KillerOwner: 1, KillerType: ut(bwgame.UnitTypesProtossZealot)})
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Type != "attack" {
		t.Fatalf("one attack: %+v", got)
	}
	e := got[0]
	want := Outcome{Killed: 4*50 + 300, Lost: 100, Workers: 4, Buildings: 1, TownHall: true, First: true}
	if e.Outcome != want {
		t.Fatalf("outcome %+v, want %+v", e.Outcome, want)
	}
	if e.SnapshotSecond != 549 || e.Second != 544 {
		t.Fatalf("a fight 50 s into the stay is told at it: second %d snapshot %d", e.Second, e.SnapshotSecond)
	}
	if e.Actor != 1 || e.Target != 0 || e.Base != 0 {
		t.Fatalf("attack: %+v", e)
	}
}

func TestAttackFightingEarlyKeepsTheArrivalSecond(t *testing.T) {
	d := digest()
	for sec := 500; sec <= 530; sec += 10 {
		d.Army = append(d.Army, army(sec, 1, bwgame.UnitTypesProtossZealot, 6, 350, 350))
	}
	for i := range 3 {
		addDeath(d, 510+i, 0, bwgame.UnitTypesZergDrone, 340, 340, 1, bwgame.UnitTypesProtossZealot)
	}
	got := Detect(d, testBases, zvp)
	if len(got) != 1 || got[0].Second != 500 || got[0].SnapshotSecond != 509 || got[0].Outcome.TownHall || !got[0].Outcome.First {
		t.Fatalf("attack: %+v", got)
	}
}

func TestSecondAttackSameTypesIsNotRelevant(t *testing.T) {
	d := digest()
	push := func(at int) {
		for sec := at; sec <= at+20; sec += 10 {
			d.Army = append(d.Army, army(sec, 1, bwgame.UnitTypesProtossZealot, 6, 350, 350))
		}
		for i := range 3 {
			addDeath(d, at+5+i, 0, bwgame.UnitTypesZergDrone, 340, 340, 1, bwgame.UnitTypesProtossZealot)
		}
	}
	push(500)
	push(700)
	if got := Detect(d, testBases, zvp); len(got) != 1 || got[0].Second != 500 {
		t.Fatalf("the same Zealots back again without a decisive trade: %+v", got)
	}
	d.Outcomes = []engine.Outcome{{Frame: f(750), Owner: 0, Kind: engine.OutcomeLeft}}
	if got := Detect(d, testBases, zvp); len(got) != 2 || got[1].Second != 700 {
		t.Fatalf("an attack the defender left after matters: %+v", got)
	}
}

func TestSnapshotAt(t *testing.T) {
	d := digest()
	d.Productions = []engine.Production{
		{Owner: 1, Type: ut(bwgame.UnitTypesProtossPylon), Unit: 20, X: 3500, Y: 3500, Start: f(100), Complete: f(118), Cancel: engine.NoFrame},
		{Owner: 1, Type: ut(bwgame.UnitTypesProtossGateway), Unit: 21, X: 3400, Y: 3500, Start: f(100), Complete: engine.NoFrame, Cancel: f(105)},
		{Owner: 0, Type: ut(bwgame.UnitTypesZergLair), From: ut(bwgame.UnitTypesZergHatchery), Unit: 1, X: 310, Y: 310, Start: f(300), Complete: f(400), Cancel: engine.NoFrame},
		{Owner: 0, Type: ut(bwgame.UnitTypesZergZergling), Unit: 22, X: 300, Y: 300, Start: f(100), Complete: f(124), Cancel: engine.NoFrame},
		{Owner: 1, Type: ut(bwgame.UnitTypesProtossPylon), Unit: 2, X: 3000, Y: 3000, Start: f(600), Complete: f(618), Cancel: engine.NoFrame},
	}
	d.Deaths = []engine.Death{{Frame: f(500), Unit: 2, Type: ut(bwgame.UnitTypesProtossNexus), Owner: 1, X: 3700, Y: 3700}}
	d.Army = []engine.ArmySnapshot{army(10, 0, bwgame.UnitTypesZergDrone, 3, 300, 300), army(20, 1, bwgame.UnitTypesProtossProbe, 2, 3700, 3700)}

	find := func(s Snapshot, t uint16) *Building {
		for i := range s.Buildings {
			if s.Buildings[i].Type == t {
				return &s.Buildings[i]
			}
		}
		return nil
	}
	pylon, gate, hatch, lair, nexus := ut(bwgame.UnitTypesProtossPylon), ut(bwgame.UnitTypesProtossGateway), ut(bwgame.UnitTypesZergHatchery), ut(bwgame.UnitTypesZergLair), ut(bwgame.UnitTypesProtossNexus)

	s := SnapshotAt(d, 5)
	if len(s.Buildings) != 2 || s.ArmySecond != -1 || len(s.Units) != 0 || !find(s, hatch).Complete || !find(s, nexus).Complete {
		t.Fatalf("at 0:05, the two start town halls and no army: %+v", s)
	}

	s = SnapshotAt(d, 104)
	if b := find(s, pylon); b == nil || b.Complete || b.Progress < 0.2 || b.Progress > 0.25 {
		t.Fatalf("a Pylon 4 s into 18 s: %+v", b)
	}
	if b := find(s, gate); b == nil || b.Complete {
		t.Fatalf("a Gateway not yet cancelled: %+v", b)
	}
	if find(s, ut(bwgame.UnitTypesZergZergling)) != nil {
		t.Fatal("units are not buildings")
	}

	s = SnapshotAt(d, 109)
	if b := find(s, pylon); b == nil || b.Progress != 0.5 {
		t.Fatalf("a Pylon half way: %+v", b)
	}
	if find(s, gate) != nil {
		t.Fatal("a cancelled Gateway is gone")
	}

	s = SnapshotAt(d, 120)
	if b := find(s, pylon); b == nil || !b.Complete {
		t.Fatalf("a finished Pylon: %+v", b)
	}

	s = SnapshotAt(d, 350)
	if b := find(s, hatch); b == nil || !b.Complete || find(s, lair) != nil {
		t.Fatalf("a Hatchery morphing to Lair shows as the Hatchery: %+v", s.Buildings)
	}

	s = SnapshotAt(d, 400)
	if b := find(s, lair); b == nil || !b.Complete || b.X != 300 || b.Y != 300 || find(s, hatch) != nil {
		t.Fatalf("the finished Lair stands where the Hatchery was: %+v", s.Buildings)
	}

	if find(SnapshotAt(d, 499), nexus) == nil || find(SnapshotAt(d, 500), nexus) != nil {
		t.Fatal("the Nexus stands until it dies")
	}

	s = SnapshotAt(d, 610)
	if b := find(s, pylon); b == nil || b.X != 3000 || b.Complete {
		var pylons []Building
		for _, b := range s.Buildings {
			if b.Type == pylon {
				pylons = append(pylons, b)
			}
		}
		t.Fatalf("a new building under a dead one's id is new, not a morph: %+v", pylons)
	}
	if s.ArmySecond != 20 || len(s.Units) != 2 || s.Units[0].Owner != 1 || s.Units[0].X != cell(3700)*engine.ArmyCellPixels+engine.ArmyCellPixels/2 {
		t.Fatalf("units from the latest army sample, at cell centres: %d %+v", s.ArmySecond, s.Units)
	}
}

func TestSnapshotAtCancelledMorphLeavesTheOldBuilding(t *testing.T) {
	d := digest()
	d.Productions = []engine.Production{{Owner: 0, Type: ut(bwgame.UnitTypesZergLair), From: ut(bwgame.UnitTypesZergHatchery), Unit: 1, X: 300, Y: 300, Start: f(300), Complete: engine.NoFrame, Cancel: f(320)}}
	s := SnapshotAt(d, 330)
	if len(s.Buildings) != 2 || s.Buildings[0].Type != ut(bwgame.UnitTypesZergHatchery) || !s.Buildings[0].Complete {
		t.Fatalf("a cancelled Lair leaves the Hatchery: %+v", s.Buildings)
	}
}

func TestInPolygon(t *testing.T) {
	square := [][2]int32{{0, 0}, {100, 0}, {100, 100}, {0, 100}}
	for _, c := range []struct {
		x, y int32
		in   bool
	}{{50, 50, true}, {1, 99, true}, {150, 50, false}, {50, -1, false}, {-10, 50, false}} {
		if got := inPolygon(square, c.x, c.y); got != c.in {
			t.Errorf("inPolygon(%d,%d) = %v", c.x, c.y, got)
		}
	}
	triangle := [][2]int32{{0, 0}, {100, 0}, {0, 100}}
	if !inPolygon(triangle, 10, 10) || inPolygon(triangle, 90, 90) {
		t.Error("triangle")
	}
}

func TestBaseOfPrefersThePolygon(t *testing.T) {
	g := &game{bases: []Base{
		{Kind: BaseStart, X: 300, Y: 300, Polygon: [][2]int32{{0, 0}, {1000, 0}, {1000, 1000}, {0, 1000}}},
		{Kind: BaseNatural, X: 900, Y: 900},
	}}
	if b := g.baseOf(900, 900); b != 0 {
		t.Fatalf("a point inside a base's polygon belongs to it even at another base's centre: %d", b)
	}
	if b := g.baseOf(1100, 1100); b != 1 {
		t.Fatalf("outside every polygon, the nearest centre: %d", b)
	}
	if b := g.baseOf(3000, 3000); b != NoBase {
		t.Fatalf("far from everything: %d", b)
	}
	r := g.regionOfBase(0, 500, 500)
	if r != (Region{X: -regionMargin, Y: -regionMargin, W: 1000 + 2*regionMargin, H: 1000 + 2*regionMargin}) {
		t.Fatalf("a base with a polygon is framed by it: %+v", r)
	}
}

func TestPeakSecond(t *testing.T) {
	if got := peakSecond(nil, 42); got != 42 {
		t.Fatalf("no deaths: %d", got)
	}
	ds := func(secs ...int) []engine.Death {
		var out []engine.Death
		for _, s := range secs {
			out = append(out, engine.Death{Frame: f(s)})
		}
		return out
	}
	if got := peakSecond(ds(130, 100, 131, 132, 133, 200), 100); got != 129 {
		t.Fatalf("the fiercest stretch, a second early: %d", got)
	}
	if got := peakSecond(ds(100, 101), 150); got != 150 {
		t.Fatalf("never before the fallback: %d", got)
	}
}

func TestFightRegion(t *testing.T) {
	fallback := Region{X: 1, Y: 2, W: 3, H: 4}
	if got := fightRegion([]engine.Death{{X: 500, Y: 500}}, fallback, 0, 0); got != fallback {
		t.Fatalf("one death cannot frame a fight: %+v", got)
	}
	got := fightRegion([]engine.Death{{X: 100, Y: 200}, {X: 1500, Y: 300}}, fallback, 800, 250)
	want := Region{X: 100 - regionMargin, Y: 200 - regionMargin - (minRegionSide-(100+2*regionMargin))/2, W: 1400 + 2*regionMargin, H: minRegionSide}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRegionAroundAndAround(t *testing.T) {
	if got := regionAround(1000, 1000, 1000, 1000); got != (Region{X: 1000 - minRegionSide/2, Y: 1000 - minRegionSide/2, W: minRegionSide, H: minRegionSide}) {
		t.Fatalf("a point is widened to the minimum side, centred: %+v", got)
	}
	if got := regionAround(0, 0, 2000, 3000); got != (Region{X: -regionMargin, Y: -regionMargin, W: 2000 + 2*regionMargin, H: 3000 + 2*regionMargin}) {
		t.Fatalf("a large area gets only the margin: %+v", got)
	}
	r := int32(20 * tile)
	if got := around(5000, 5000, r); got != (Region{X: 5000 - r - regionMargin, Y: 5000 - r - regionMargin, W: 2*r + 2*regionMargin, H: 2*r + 2*regionMargin}) {
		t.Fatalf("around: %+v", got)
	}
}

func TestAbs(t *testing.T) {
	for in, want := range map[int]int{-5: 5, 0: 0, 7: 7} {
		if got := abs(in); got != want {
			t.Errorf("abs(%d) = %d", in, got)
		}
	}
}
