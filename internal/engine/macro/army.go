package macro

import (
	"math"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// The army is read from 8:00, once every player has one worth commanding, and only in snapshots where it
// is worth at least 1,000 minerals and gas.
const (
	armyFromSecond = 480
	armyMinValue   = 1000
	// A unit is idle once it has stood on the same cell with no order across two snapshots (20 s): long
	// enough that a unit pausing between commands does not count.
	idleSnapshots = 2
	minArmySeen   = 3
)

func unitTypes(ts ...bwgame.UnitTypes) map[uint16]bool {
	out := map[uint16]bool{}
	for _, t := range ts {
		out[uint16(t)] = true
	}
	return out
}

var (
	notArmy = unitTypes(bwgame.UnitTypesZergLarva, bwgame.UnitTypesZergEgg, bwgame.UnitTypesZergOverlord,
		bwgame.UnitTypesProtossInterceptor, bwgame.UnitTypesProtossScarab, bwgame.UnitTypesTerranVultureSpiderMine,
		bwgame.UnitTypesTerranNuclearMissile, bwgame.UnitTypesTerranSCV, bwgame.UnitTypesZergDrone, bwgame.UnitTypesProtossProbe)
	// Sieged tanks and Lurkers hold a position by design.
	holdsPosition = unitTypes(bwgame.UnitTypesTerranSiegeTankSiegeMode, bwgame.UnitTypesZergLurker)
	idleOrders    = map[uint8]bool{
		uint8(bwgame.OrdersStop): true, uint8(bwgame.OrdersGuard): true, uint8(bwgame.OrdersPlayerGuard): true,
		uint8(bwgame.OrdersNothing): true, uint8(bwgame.OrdersMedicIdle): true, uint8(bwgame.OrdersCarrierStop): true,
		uint8(bwgame.OrdersReaverStop): true, uint8(bwgame.OrdersHoldPosition): true,
		uint8(bwgame.OrdersCarrier): true, uint8(bwgame.OrdersReaver): true,
	}
)

type stillUnit struct {
	x, y      int32
	snapshots int
}

// IdleArmyShare is the average share of a player's army value standing idle on one spot from 8:00,
// wherever a rally point left it; NaN when the game gives it no basis.
func IdleArmyShare(d *engine.Digest, slot int8) float64 {
	if !d.UnitOrdersKnown() {
		return math.NaN()
	}
	end := d.TrustedUntil()
	still := map[uint32]stillUnit{}
	from := int32(armyFromSecond * framesPerSecond)
	var idle, snapshots float64
	for _, snap := range d.Army {
		if snap.Frame >= end {
			break
		}
		counted := snap.Frame >= from
		next := map[uint32]stillUnit{}
		var total, standing float64
		for _, u := range snap.Units {
			if u.Owner != slot || notArmy[u.Type] || engine.CostOf(u.Type).Value() <= 0 {
				continue
			}
			n := 0
			if idleOrders[u.Order] {
				if prev, ok := still[u.ID]; ok && prev.x == u.X && prev.y == u.Y {
					n = prev.snapshots + 1
				}
				next[u.ID] = stillUnit{u.X, u.Y, n}
			}
			if holdsPosition[u.Type] {
				continue
			}
			v := float64(engine.CostOf(u.Type).Value())
			total += v
			if n >= idleSnapshots {
				standing += v
			}
		}
		still = next
		if counted && total >= armyMinValue {
			idle += standing / total
			snapshots++
		}
	}
	if snapshots < minArmySeen {
		return math.NaN()
	}
	return idle / snapshots
}
