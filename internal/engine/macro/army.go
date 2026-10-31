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
	// A unit is idle once it has stood within one tile of the same spot with no order across two snapshots
	// (20 s): long enough that a unit pausing between commands does not count. The tile of slack is for air
	// units, which drift as they hover and never stand on the exact same pixel twice.
	idleSnapshots = 2
	stillPixels   = 32
	// A minute of snapshots with a real army, so a handful of units seen a few times cannot read as an army
	// that never stands around.
	minArmySeen = 6
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
	// Sieged tanks and Lurkers hold a position by design, and Carriers are in a fight order whenever their
	// interceptors find something in range, so a fleet parked at home still reads busy.
	holdsPosition = unitTypes(bwgame.UnitTypesTerranSiegeTankSiegeMode, bwgame.UnitTypesZergLurker, bwgame.UnitTypesProtossCarrier)
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
				spot := stillUnit{x: u.X, y: u.Y}
				if prev, ok := still[u.ID]; ok && abs32(prev.x-u.X) <= stillPixels && abs32(prev.y-u.Y) <= stillPixels {
					spot = stillUnit{x: prev.x, y: prev.y, snapshots: prev.snapshots + 1}
				}
				next[u.ID] = spot
				n = spot.snapshots
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

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
