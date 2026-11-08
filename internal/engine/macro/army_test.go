package macro

import (
	"math"
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func armyDigest(snapshots int, incomePerMinute int32, units func(i int) []engine.ArmyUnit) *engine.Digest {
	d := &engine.Digest{EngineVersion: engine.EngineVersion, LastFrame: 2000 * framesPerSecond, Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame}}
	for s := range 2000 {
		for _, slot := range []int8{0, 1} {
			d.Economy = append(d.Economy, engine.EconomySample{Frame: int32(s * framesPerSecond), Owner: slot, GatheredMinerals: incomePerMinute * int32(s) / 60})
		}
	}
	for i := range snapshots {
		d.Army = append(d.Army, engine.ArmySnapshot{Frame: int32(armyFromSecond*framesPerSecond + i*engine.ArmyEveryFrames), Units: units(i)})
	}
	return d
}

func TestArmyUseCountsUnitsStandingIdle(t *testing.T) {
	marine, tank := uint16(bwgame.UnitTypesTerranMarine), uint16(bwgame.UnitTypesTerranSiegeTankSiegeMode)
	guard, move := uint8(bwgame.OrdersPlayerGuard), uint8(bwgame.OrdersMove)
	d := armyDigest(18, 1000, func(i int) []engine.ArmyUnit {
		var units []engine.ArmyUnit
		for id := range uint32(20) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: marine, ID: id, X: 10, Y: 10, Order: guard})
		}
		for id := uint32(20); id < 40; id++ {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: marine, ID: id, X: int32(i), Y: 10, Order: move})
		}
		frame := int32(armyFromSecond*framesPerSecond + i*engine.ArmyEveryFrames)
		return append(units, engine.ArmyUnit{Owner: 0, Type: tank, ID: 40, X: 50, Y: 50, Order: guard, Commanded: frame - framesPerSecond})
	})
	// The first two snapshots cannot tell idle yet; the other sixteen see half the marines standing. The sieged
	// tank was just ordered, so it is army in use. The army is worth more than a minute of income, and more
	// than ladder armies until 10:00, when they reach 2,325 and the 75 it lacks counts as idle.
	use := ArmyUseOf(d, 0)
	if want := (10*1000.0/2250 + 6*1075.0/2325) / 18; !(math.Abs(use.IdleShare-want) < 1e-9) {
		t.Fatalf("idle share %v, want %v", use.IdleShare, want)
	}
	if want := 40*50.0 + 250; use.Value != want {
		t.Fatalf("army value %v, want %v", use.Value, want)
	}
	if !math.IsNaN(ArmyUseOf(d, 1).IdleShare) {
		t.Fatalf("a player without an army or income has no idle share")
	}
	d.EngineVersion = engine.OldestReadableEngineVersion
	if !math.IsNaN(ArmyUseOf(d, 0).IdleShare) {
		t.Fatalf("a digest without unit orders has no idle share")
	}
}

func TestArmyUseCountsTheArmyNeverBuiltAsIdle(t *testing.T) {
	corsair, move := uint16(bwgame.UnitTypesProtossCorsair), uint8(bwgame.OrdersMove)
	d := armyDigest(18, 6000, func(i int) []engine.ArmyUnit {
		var units []engine.ArmyUnit
		for id := range uint32(6) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: corsair, ID: id, X: int32(i) * 100, Y: 10, Order: move})
		}
		return units
	})
	// Six Corsairs (1,500) kept moving against 6,000 income a minute: three quarters of the army it could
	// have fielded was never built.
	if got, want := ArmyUseOf(d, 0).IdleShare, 4500.0/6000; !(math.Abs(got-want) < 1e-9) {
		t.Fatalf("idle share %v, want %v", got, want)
	}
}

func TestArmyUseSeesHoveringAirAndCountsCarriersAsArmy(t *testing.T) {
	muta, carrier, zealot := uint16(bwgame.UnitTypesZergMutalisk), uint16(bwgame.UnitTypesProtossCarrier), uint16(bwgame.UnitTypesProtossZealot)
	guard, move := uint8(bwgame.OrdersPlayerGuard), uint8(bwgame.OrdersMove)
	d := armyDigest(18, 1000, func(i int) []engine.ArmyUnit {
		var units []engine.ArmyUnit
		for id := range uint32(10) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: muta, ID: id, X: 100 + int32(i%2)*12, Y: 100 - int32(i%2)*9, Order: guard})
		}
		frame := int32(armyFromSecond*framesPerSecond + i*engine.ArmyEveryFrames)
		for id := uint32(60); id < 66; id++ {
			units = append(units, engine.ArmyUnit{Owner: 1, Type: carrier, ID: id, X: 10, Y: 10, Order: guard, Commanded: frame - framesPerSecond})
		}
		return append(units, engine.ArmyUnit{Owner: 1, Type: zealot, ID: 70, X: int32(i) * 50, Y: 10, Order: move})
	})
	if got, want := ArmyUseOf(d, 0).IdleShare, 16.0/18; !(math.Abs(got-want) < 1e-9) {
		t.Fatalf("mutalisks hovering on one spot: idle share %v, want %v", got, want)
	}
	// Carriers just given an order and a moving Zealot are an army in use.
	if got := ArmyUseOf(d, 1).IdleShare; got != 0 {
		t.Fatalf("commanded carriers and a moving zealot: idle share %v, want 0", got)
	}
}

func TestArmyUseCountsTanksParkedWithoutOrdersAsIdle(t *testing.T) {
	tank, marine := uint16(bwgame.UnitTypesTerranSiegeTankSiegeMode), uint16(bwgame.UnitTypesTerranMarine)
	move := uint8(bwgame.OrdersMove)
	d := armyDigest(18, 1000, func(i int) []engine.ArmyUnit {
		units := []engine.ArmyUnit{{Owner: 0, Type: marine, ID: 99, X: int32(i) * 50, Y: 10, Order: move}}
		for id := range uint32(10) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: tank, ID: id, X: 20 + int32(i%2), Y: 20, Order: uint8(bwgame.OrdersSieging), Commanded: engine.NoFrame})
		}
		return units
	})
	// Ten Sieged Tanks parked at home and never ordered again stand idle from the third snapshot on, while
	// one Marine keeps moving.
	if got, want := ArmyUseOf(d, 0).IdleShare, 16.0/18*2500/2550; !(math.Abs(got-want) < 1e-9) {
		t.Fatalf("a turtle's parked tanks: idle share %v, want %v", got, want)
	}
}

func TestArmyStartCadenceCountsWindowsWithArmyStarted(t *testing.T) {
	marine, scv := uint16(bwgame.UnitTypesTerranMarine), uint16(bwgame.UnitTypesTerranSCV)
	d := &engine.Digest{Productions: []engine.Production{
		{Owner: 0, Type: marine, Start: f(100)},
		{Owner: 0, Type: marine, Start: f(125)},
		{Owner: 0, Type: marine, Start: f(128)},
		{Owner: 0, Type: scv, Start: f(145)},
		{Owner: 1, Type: marine, Start: f(155)},
	}}
	// Windows from 2:00 to 4:00: 13 of them, a marine starting in one; earlier marines, SCVs and the other
	// player's units do not count.
	if got, want := ArmyStartCadence(d, 0, 240), 1.0/13; math.Abs(got-want) > 1e-9 {
		t.Fatalf("start cadence %v, want %v", got, want)
	}
	if !math.IsNaN(ArmyStartCadence(d, 0, 200)) {
		t.Fatal("under two minutes past 2:00 has no cadence")
	}
}

func TestArmyUseCountsCarriersLeftToTheirInterceptorsAsIdle(t *testing.T) {
	carrier := uint16(bwgame.UnitTypesProtossCarrier)
	fight := uint8(bwgame.OrdersCarrierFight)
	d := armyDigest(18, 1000, func(i int) []engine.ArmyUnit {
		frame := int32(armyFromSecond*framesPerSecond + i*engine.ArmyEveryFrames)
		var units []engine.ArmyUnit
		for id := range uint32(4) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: carrier, ID: id, X: int32(i) * 3, Y: 10, Order: fight, Commanded: 400 * framesPerSecond})
		}
		for id := uint32(4); id < 8; id++ {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: carrier, ID: id, X: int32(i) * 3, Y: 10, Order: fight, Commanded: frame - 5*framesPerSecond})
		}
		return units
	})
	// Half the fleet fights with no order since 6:40, the other half was ordered seconds ago.
	if got := ArmyUseOf(d, 0).IdleShare; !(math.Abs(got-0.5) < 1e-9) {
		t.Fatalf("carriers left to their interceptors: idle share %v, want 0.5", got)
	}
}

func TestArmyUseCountsAnArmySmallerThanLadderArmiesAsPartlyIdle(t *testing.T) {
	scout, move := uint16(bwgame.UnitTypesProtossScout), uint8(bwgame.OrdersMove)
	d := armyDigest(0, 100, nil)
	for i := range 18 {
		var units []engine.ArmyUnit
		for id := range uint32(10) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: scout, ID: id, X: int32(i) * 100, Y: 10, Order: move})
		}
		d.Army = append(d.Army, engine.ArmySnapshot{Frame: int32(20*60*framesPerSecond + i*engine.ArmyEveryFrames), Units: units})
	}
	// Ten Scouts (4,000) kept moving from 20:00 to 22:50, when ladder armies are worth 6,050, 6,225 and 6,275:
	// what they lack is idle.
	want := (2050.0/6050 + 2225.0/6225 + 2275.0/6275) / 3
	if got := ArmyUseOf(d, 0).IdleShare; !(math.Abs(got-want) < 1e-9) {
		t.Fatalf("ten scouts at 20:00: idle share %v, want %v", got, want)
	}
}
