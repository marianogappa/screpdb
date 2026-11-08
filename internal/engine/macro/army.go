package macro

import (
	"math"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// The army is read from 8:00, once every player has one worth commanding, and only in snapshots where it
// is worth at least 1,000 minerals and gas. It is expected to be worth at least a minute of the income of the
// 5 minutes before each snapshot, as ladder armies of every tier are.
const (
	armyFromSecond      = 480
	armyMinValue        = 1000
	incomeWindowSeconds = 300
	armyIncomeMinutes   = 1.0
	// A unit is idle once it has stood within one tile of the same spot with no order across two snapshots
	// (20 s): long enough that a unit pausing between commands does not count. The tile of slack is for air
	// units, which drift as they hover and never stand on the exact same pixel twice.
	idleSnapshots = 2
	stillPixels   = 32
	// Sieged Tanks, Lurkers and Carriers stand still or fight on their own by design, so they are idle only
	// once parked within a few cells and left without an order for a minute: a turtle's tanks sieged at home
	// all game are an idle army, a pro's sieged line that keeps being moved up is not.
	parkedCells   = 4
	parkedSeconds = 60
	// A Carrier is idle unless the player ordered it within the last 20 seconds: parked at home it is wasted,
	// and left to its interceptors at the front it is unmicroed, and pros micro the few Carriers they make.
	carrierCommandSeconds = 20
	// A minute of snapshots gives the army's worth. Idle army needs three minutes of a real army: over one
	// minute, a player out at 9:30 read as using an army they barely had time to stand around with, and on
	// ladder three minutes followed MMR better for every race.
	minArmySeen = 6
	minIdleSeen = 18
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
	// Sieged tanks and Lurkers hold a position by design, so their orders cannot say whether they are idle.
	holdsPosition = unitTypes(bwgame.UnitTypesTerranSiegeTankSiegeMode, bwgame.UnitTypesZergLurker)
	carrier       = uint16(bwgame.UnitTypesProtossCarrier)
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

// ladderArmyByMinute is the median army worth of ladder 1v1 players at each minute from 8:00, measured by
// scripts/skill-mine over 2,907 games of every tier. An army smaller than this counts what it lacks as idle,
// in every kind of game: ten Scouts flown around all game are an army in use, but a small one, and the team
// game and money map adjustments are fitted on top.
var ladderArmyByMinute = [...]float64{
	8: 1325, 1800, 2325, 2875, 3450, 3950, 4350, 4550, 5050, 5350, 5575, 5800, 6050, 6225, 6275, 6400, 6500,
	6625, 6650, 6700, 6600, 6450, 6375, 6325, 6775, 6450, 6800, 6900, 6600, 6850, 7400, 7250,
}

func ladderArmyAt(second int) float64 {
	return ladderArmyByMinute[min(second/secondsPerMin, len(ladderArmyByMinute)-1)]
}

// ArmyUse is how a player used their army from 8:00.
type ArmyUse struct {
	// IdleShare is the average share of the army standing idle on one spot, wherever a rally point left it,
	// with the army an income could have fielded but the player never built counted as idle: a handful of
	// Corsairs flying around all game is a small army kept busy, not a pro's.
	IdleShare float64
	// Value is the average worth of the army, Sieged Tanks, Lurkers and Carriers included.
	Value float64
}

// ArmyMoment is a player's army in one snapshot from 8:00: its whole worth, and the part standing idle.
type ArmyMoment struct {
	Second         int
	Full, Standing float64
}

// ArmyMoments are a player's army snapshots from 8:00 up to the trusted end of the game; nil when the digest
// does not know unit orders.
func ArmyMoments(d *engine.Digest, slot int8) []ArmyMoment {
	if !d.UnitOrdersKnown() {
		return nil
	}
	end := d.TrustedUntil()
	still, parked := map[uint32]stillUnit{}, map[uint32]stillUnit{}
	from := int32(armyFromSecond * framesPerSecond)
	var out []ArmyMoment
	for _, snap := range d.Army {
		if snap.Frame >= end {
			break
		}
		next, nextParked := map[uint32]stillUnit{}, map[uint32]stillUnit{}
		var full, standing float64
		for _, u := range snap.Units {
			if u.Owner != slot || notArmy[u.Type] || engine.CostOf(u.Type).Value() <= 0 {
				continue
			}
			v := float64(engine.CostOf(u.Type).Value())
			full += v
			if u.Type == carrier {
				if u.Commanded == engine.NoFrame || snap.Frame-u.Commanded >= carrierCommandSeconds*framesPerSecond {
					standing += v
				}
				continue
			}
			if holdsPosition[u.Type] {
				spot := stillUnit{x: u.X, y: u.Y}
				if prev, ok := parked[u.ID]; ok && abs32(prev.x-u.X) <= parkedCells && abs32(prev.y-u.Y) <= parkedCells {
					spot = stillUnit{x: prev.x, y: prev.y, snapshots: prev.snapshots + 1}
				}
				nextParked[u.ID] = spot
				uncommanded := u.Commanded == engine.NoFrame || snap.Frame-u.Commanded >= parkedSeconds*framesPerSecond
				if spot.snapshots >= idleSnapshots && uncommanded {
					standing += v
				}
				continue
			}
			if idleOrders[u.Order] {
				spot := stillUnit{x: u.X, y: u.Y}
				if prev, ok := still[u.ID]; ok && abs32(prev.x-u.X) <= stillPixels && abs32(prev.y-u.Y) <= stillPixels {
					spot = stillUnit{x: prev.x, y: prev.y, snapshots: prev.snapshots + 1}
				}
				next[u.ID] = spot
				if spot.snapshots >= idleSnapshots {
					standing += v
				}
			}
		}
		still, parked = next, nextParked
		if snap.Frame >= from {
			out = append(out, ArmyMoment{Second: int(snap.Frame) / framesPerSecond, Full: full, Standing: standing})
		}
	}
	return out
}

// ArmyUseOf measures a player's army; both values are NaN when the game gives them no basis.
func ArmyUseOf(d *engine.Digest, slot int8) ArmyUse {
	none := ArmyUse{IdleShare: math.NaN(), Value: math.NaN()}
	moments := ArmyMoments(d, slot)
	if moments == nil {
		return none
	}
	gathered := gatheredBySecond(d, slot)
	var idle, idleSeen, value float64
	for _, m := range moments {
		value += m.Full
		now, okNow := gathered[m.Second]
		then, okThen := gathered[m.Second-incomeWindowSeconds]
		if !okNow || !okThen {
			continue
		}
		missing := max(0, armyIncomeMinutes*(now-then)/(incomeWindowSeconds/60)-m.Full, ladderArmyAt(m.Second)-m.Full)
		if m.Full >= armyMinValue {
			idle += (m.Standing + missing) / (m.Full + missing)
			idleSeen++
		}
	}
	use := none
	if len(moments) >= minArmySeen {
		use.Value = value / float64(len(moments))
	}
	if idleSeen >= minIdleSeen {
		use.IdleShare = idle / idleSeen
	}
	return use
}

func gatheredBySecond(d *engine.Digest, slot int8) map[int]float64 {
	out := map[int]float64{}
	for _, e := range d.Economy {
		if e.Owner == slot {
			out[int(e.Frame)/framesPerSecond] = float64(e.GatheredMinerals + e.GatheredGas)
		}
	}
	return out
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

var notArmyProduction = unitTypes(bwgame.UnitTypesTerranSCV, bwgame.UnitTypesZergDrone, bwgame.UnitTypesProtossProbe,
	bwgame.UnitTypesZergOverlord, bwgame.UnitTypesZergLarva, bwgame.UnitTypesZergEgg, bwgame.UnitTypesProtossInterceptor,
	bwgame.UnitTypesProtossScarab, bwgame.UnitTypesTerranVultureSpiderMine, bwgame.UnitTypesTerranNuclearMissile,
	bwgame.UnitTypesZergCocoon, bwgame.UnitTypesZergLurkerEgg)

const cadenceWindowSeconds = 10

// ArmyStartCadence is the share of 10-second windows from 2:00 to endSecond in which an army unit started
// training or morphing; NaN when that is under two minutes.
func ArmyStartCadence(d *engine.Digest, slot int8, endSecond int) float64 {
	if endSecond-steadyFromSecond < 2*secondsPerMin {
		return math.NaN()
	}
	started := make([]bool, (endSecond-steadyFromSecond)/cadenceWindowSeconds+1)
	for _, pr := range d.Productions {
		if pr.Owner != slot || pr.Type >= uint16(bwgame.UnitTypesTerranCommandCenter) || notArmyProduction[pr.Type] {
			continue
		}
		if s := second(pr.Start); s >= steadyFromSecond && s < endSecond {
			started[(s-steadyFromSecond)/cadenceWindowSeconds] = true
		}
	}
	n := 0
	for _, s := range started {
		if s {
			n++
		}
	}
	return float64(n) / float64(len(started))
}
