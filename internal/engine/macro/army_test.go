package macro

import (
	"math"
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

func TestIdleArmyShareCountsUnitsStandingIdle(t *testing.T) {
	marine, tank := uint16(bwgame.UnitTypesTerranMarine), uint16(bwgame.UnitTypesTerranSiegeTankSiegeMode)
	guard, move := uint8(bwgame.OrdersPlayerGuard), uint8(bwgame.OrdersMove)
	d := &engine.Digest{EngineVersion: engine.EngineVersion, LastFrame: 2000 * framesPerSecond, Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame}}
	for i := range 6 {
		var units []engine.ArmyUnit
		for id := range uint32(20) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: marine, ID: id, X: 10, Y: 10, Order: guard})
		}
		for id := uint32(20); id < 40; id++ {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: marine, ID: id, X: int32(i), Y: 10, Order: move})
		}
		units = append(units, engine.ArmyUnit{Owner: 0, Type: tank, ID: 40, X: 50, Y: 50, Order: guard})
		d.Army = append(d.Army, engine.ArmySnapshot{Frame: int32(armyFromSecond*framesPerSecond + i*engine.ArmyEveryFrames), Units: units})
	}
	// The first two snapshots cannot tell idle yet; the last four see half the marines standing. The sieged
	// tank holds its position by design.
	if got, want := IdleArmyShare(d, 0), 4.0/6*0.5; !(math.Abs(got-want) < 1e-9) {
		t.Fatalf("idle share %v, want %v", got, want)
	}
	if !math.IsNaN(IdleArmyShare(d, 1)) {
		t.Fatalf("a player without an army has no idle share")
	}
	d.EngineVersion = engine.OldestReadableEngineVersion
	if !math.IsNaN(IdleArmyShare(d, 0)) {
		t.Fatalf("a digest without unit orders has no idle share")
	}
}

func TestIdleArmyShareSeesHoveringAirAndSkipsCarriers(t *testing.T) {
	muta, carrier := uint16(bwgame.UnitTypesZergMutalisk), uint16(bwgame.UnitTypesProtossCarrier)
	guard := uint8(bwgame.OrdersPlayerGuard)
	d := &engine.Digest{EngineVersion: engine.EngineVersion, LastFrame: 2000 * framesPerSecond, Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame}}
	for i := range 8 {
		var units []engine.ArmyUnit
		for id := range uint32(10) {
			units = append(units, engine.ArmyUnit{Owner: 0, Type: muta, ID: id, X: 100 + int32(i%2)*12, Y: 100 - int32(i%2)*9, Order: guard})
		}
		units = append(units, engine.ArmyUnit{Owner: 1, Type: carrier, ID: 50, X: int32(i) * 200, Y: 10, Order: guard})
		for id := uint32(60); id < 66; id++ {
			units = append(units, engine.ArmyUnit{Owner: 1, Type: carrier, ID: id, X: 10, Y: 10, Order: guard})
		}
		d.Army = append(d.Army, engine.ArmySnapshot{Frame: int32(armyFromSecond*framesPerSecond + i*engine.ArmyEveryFrames), Units: units})
	}
	if got, want := IdleArmyShare(d, 0), 6.0/8; !(math.Abs(got-want) < 1e-9) {
		t.Fatalf("mutalisks hovering on one spot: idle share %v, want %v", got, want)
	}
	if !math.IsNaN(IdleArmyShare(d, 1)) {
		t.Fatalf("an army of Carriers has no idle share")
	}
}
