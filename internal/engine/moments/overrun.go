package moments

import (
	"slices"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

const InsightOverrun = "overrun"

// An overrun ends a game: an army reaches its victim's buildings and the victim has nothing to answer it with,
// so leaves soon after. The force is the attacker's mobile army within reach of the victim side's buildings;
// the answer is the victim side's whole ground-hitting army, wherever it is, and its static defence in reach of
// the force. A victim with an army elsewhere is not unprepared, so a lone harasser is no overrun. The force must
// be worth 100 resources and twice the answer, within 90 s of the victim leaving. A victim's building in a base
// the attacker holds a town hall in does not count: that army is at home, beside a ruin or a building the
// victim lifted away long ago, and neither does a force standing in a base the attacker holds.
const (
	overrunReach      = 10 * tile
	overrunFightReach = 25 * tile
	overrunMinForce   = 100
	overrunRatio      = 2
	overrunEndWithin  = 90
)

// cannotHitGround are units that do nothing against an army on the ground, so they defend nothing from it.
var cannotHitGround = map[uint16]bool{
	ut(bwgame.UnitTypesProtossCorsair): true, ut(bwgame.UnitTypesProtossObserver): true, ut(bwgame.UnitTypesProtossShuttle): true,
	ut(bwgame.UnitTypesZergScourge): true, ut(bwgame.UnitTypesZergDevourer): true,
	ut(bwgame.UnitTypesTerranValkyrie): true, ut(bwgame.UnitTypesTerranDropship): true, ut(bwgame.UnitTypesTerranScienceVessel): true,
}

var staticDefence = map[uint16]bool{ut(bwgame.UnitTypesProtossPhotonCannon): true, ut(bwgame.UnitTypesTerranBunker): true, ut(bwgame.UnitTypesZergSunkenColony): true}

type standingBuilding struct {
	owner int8
	typ   uint16
	x, y  int32
}

// buildingsAt are the buildings standing at a second: the starting town halls and every finished building,
// less those that died.
func (g *game) buildingsAt(sec int) []standingBuilding {
	dead := map[uint32]bool{}
	for _, d := range g.d.Deaths {
		if second(d.Frame) <= sec {
			dead[d.Unit] = true
		}
	}
	var out []standingBuilding
	for _, u := range g.d.Initial {
		if _, building := FootprintOf(u.Type); building && !dead[u.Unit] {
			out = append(out, standingBuilding{u.Owner, u.Type, u.X, u.Y})
		}
	}
	for _, p := range g.d.Productions {
		if _, building := FootprintOf(p.Type); !building || p.Complete == engine.NoFrame || second(p.Complete) > sec || dead[p.Unit] {
			continue
		}
		out = append(out, standingBuilding{p.Owner, p.Type, p.X, p.Y})
	}
	return out
}

// overruns finds at most one overrun per side, on a side that lost when the library knows who did. A loser
// nobody saw leave is the recorder, whose replay was cut when they quit, so the game's end stands in. The
// overrun starts where the victim stopped answering for good: the last unbroken run of unanswered snapshots
// before it left, not a first raider the defence then held off.
func (g *game) overruns() []Event {
	var out []Event
	done := map[int]bool{}
	nobodyLeft := !slices.ContainsFunc(g.d.Outcomes, func(o engine.Outcome) bool { return o.Kind != engine.OutcomeWon })
	known := false
	for _, p := range g.players {
		known = known || p.Result != 0
	}
	standing := map[int][]standingBuilding{}
	buildingsAt := func(sec int) []standingBuilding {
		if _, ok := standing[sec]; !ok {
			standing[sec] = g.buildingsAt(sec)
		}
		return standing[sec]
	}
	for _, victim := range sortedSlots(g.players) {
		if !g.players[victim].Human || done[g.teamOf(victim)] || g.won(victim) || known && g.players[victim].Result != ResultLost {
			continue
		}
		end := g.endOf(victim)
		if end < 0 && (nobodyLeft || known) {
			end = second(g.d.LastFrame)
		}
		if end < 0 {
			continue
		}
		var run, last *Event
		for _, snap := range g.d.Army {
			sec := second(snap.Frame)
			if sec >= g.trusted || sec > end {
				break
			}
			if end-sec > overrunEndWithin {
				continue
			}
			e, ok := g.overrunAt(snap, buildingsAt(sec), victim, end)
			switch {
			case !ok:
				if run != nil {
					last = run
				}
				run = nil
			case run == nil:
				run = &e
			}
		}
		if run == nil {
			run = last
		}
		if run != nil {
			done[g.teamOf(victim)] = true
			out = append(out, *run)
		}
	}
	slices.SortFunc(out, func(a, b Event) int { return a.Second - b.Second })
	return out
}

type overrunForce struct {
	value  int32
	units  []uint16
	sx, sy int64
}

func (g *game) overrunAt(snap engine.ArmySnapshot, standing []standingBuilding, victim int8, end int) (Event, bool) {
	sec := second(snap.Frame)
	var theirs []standingBuilding
	for _, b := range standing {
		if b.owner == victim || g.players[b.owner].Human && !g.enemies(b.owner, victim) {
			theirs = append(theirs, b)
		}
	}
	if len(theirs) == 0 {
		return Event{}, false
	}
	within := func(x, y, reach int32) func(standingBuilding) bool {
		return func(b standingBuilding) bool { return dist2(x, y, b.x, b.y) <= int64(reach)*int64(reach) }
	}
	halls := map[int8]map[int]bool{}
	for _, b := range standing {
		if IsTownHall(b.typ) {
			if halls[b.owner] == nil {
				halls[b.owner] = map[int]bool{}
			}
			halls[b.owner][g.baseOf(b.x, b.y)] = true
		}
	}
	forces := map[int8]*overrunForce{}
	for _, u := range snap.Units {
		if !g.enemies(u.Owner, victim) || !mobileArmy(u.Type) {
			continue
		}
		ux, uy := u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels
		atHome := halls[u.Owner]
		if !slices.ContainsFunc(theirs, func(b standingBuilding) bool {
			return within(ux, uy, overrunReach)(b) && !atHome[g.baseOf(b.x, b.y)]
		}) {
			continue
		}
		f := forces[u.Owner]
		if f == nil {
			f = &overrunForce{}
			forces[u.Owner] = f
		}
		f.value += CostOf(u.Type).Value()
		f.units = append(f.units, u.Type)
		f.sx, f.sy = f.sx+int64(ux), f.sy+int64(uy)
	}
	for _, attacker := range sortedSlots(forces) {
		f := forces[attacker]
		n := int64(len(f.units))
		x, y := int32(f.sx/n), int32(f.sy/n)
		var defence int32
		defenders := 0
		for _, u := range snap.Units {
			if g.enemies(u.Owner, victim) || u.Owner == attacker || !g.players[u.Owner].Human || !mobileArmy(u.Type) || cannotHitGround[u.Type] {
				continue
			}
			defence += CostOf(u.Type).Value()
			defenders++
		}
		for _, b := range theirs {
			if staticDefence[b.typ] && within(x, y, overrunReach+4*tile)(b) {
				defence += CostOf(b.typ).Value()
				defenders++
			}
		}
		if f.value < overrunMinForce || f.value < overrunRatio*defence {
			continue
		}
		e := g.event(InsightOverrun, sec, attacker, victim, g.baseOf(x, y), x, y, byCount(f.units))
		e.Seconds = end - sec
		e.Count = len(f.units)
		e.Defenders = defenders
		if halls[attacker][g.baseOf(x, y)] {
			continue
		}
		e.Outcome = g.outcome(attacker, g.fightNear(attacker, victim, sec-10, end, x, y, overrunFightReach))
		e.Region = around(x, y, overrunFightReach)
		return e, true
	}
	return Event{}, false
}

func sortedSlots[V any](m map[int8]V) []int8 {
	var out []int8
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// mobileArmy is a unit that fights and moves: not a building, an Overlord, a worker or a spawned unit.
func mobileArmy(t uint16) bool {
	if _, building := FootprintOf(t); building {
		return false
	}
	return !notAVictim[t] && !isWorker(t) && t != ut(bwgame.UnitTypesZergOverlord) && CostOf(t).Value() > 0
}
