package moments

import (
	"slices"
	"sort"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// Insights is the Labs layer over Detect: techniques worth a filter pill, mistakes that cost something,
// and, in a 1v1, the one of them that best says why the loser left. Why is nil when no mechanism explains
// the loss: a lost fight alone is an event, not a reason.
type Insights struct {
	Events []Event
	Why    *Why
}

const (
	InsightUndetectedCloak   = "undetected_cloak"
	InsightUndefendedAir     = "undefended_air_harass"
	InsightOverlordHunt      = "overlord_hunt"
	InsightElevator          = "elevator"
	InsightEraser            = "eraser"
	InsightDefilersIrradiate = "defilers_irradiated"
	InsightNydusDenied       = "nydus_denied"
	InsightLurkerContain     = "lurker_contain"

	CloakNoDetection = "none"
	CloakNotThere    = "elsewhere"
	CloakScansOnly   = "scan"
)

const (
	orderIrradiate = 0x8f

	cloakEpisodeGap  = 60
	cloakMinKills    = 4
	cloakMinValue    = 600
	cloakStaticReach = 8 * tile
	cloakMobileReach = 12 * tile
	airEpisodeGap    = 45
	airMinWorkers    = 5
	airMobileEnough  = 3
	// Static anti-air that harassers kill on the way in still defended the base.
	airDefenceLookBack = 60
	huntEpisodeGap     = 60
	// A hunt is one area cleared of Overlords, or one party chasing them down; Overlords picked off across the
	// map are routine scouting losses. A chase links kills up to 20 s apart, its hunters flying 3 tiles a second.
	huntReach        = 12 * tile
	huntChaseGap     = 20
	huntChaseSpeed   = 3 * tile
	huntMinOverlords = 5
	huntBlockAfter   = 60
	// Five Overlords are a hunt only when they cost a supply block; eight are one regardless.
	huntManyOverlords = 8
	huntLongBlock     = 30
	// Overlords lost a few at a time are routine; the hunt is timed at its biggest burst.
	huntBurstGap = 20
	// A Zerg the enemy outnumbers three times in supply, or twice in supply and three times in workers, has
	// already lost: Overlords sent to die then are not a hunt.
	huntLostSupplyRatio   = 3
	huntBehindSupplyRatio = 2
	huntLostWorkerRatio   = 3
	// An Overlord dying next to its owner's unload is a transport shot down in a drop, not a hunted one.
	dropOverlordReach   = 10 * tile
	dropOverlordSeconds = 20
	// A fight's aftermath: deaths up to 45 s after a drop or a Defiler loss count toward its outcome.
	fightFollowUp    = 45
	elevatorWindow   = 60
	elevatorMinTrips = 2
	elevatorMinCargo = 4
	// An elevator hops a wall: loaded below the base's edge, unloaded just inside it, a short flight apart.
	elevatorHop        = 16 * tile
	elevatorLoadEdge   = 10 * tile
	elevatorUnloadEdge = 5 * tile
	elevatorNear       = 6 * tile
	elevatorTogether   = 2 * 24
	// Irradiate lasts about 37 seconds; the Eraser's kills are the caster's Science Vessel kills meanwhile.
	eraserWindow     = 40
	eraserMinWorkers = 3
	eraserOtherReach = 5 * tile
	defilerBurstGap  = 90
	defilerMinBurst  = 3
	nydusUseReach    = 8 * tile
	nydusBaseReach   = 20 * tile
	nydusBaseAfter   = 120
	// A contain is Lurkers burrowed outside an enemy base's choke with an army beside them, held for a minute.
	containReach        = 14 * tile
	containSupportReach = 18 * tile
	containMinLurkers   = 3
	containMinSupport   = 6
	containMinSeconds   = 60
	containHeldReach    = tile
)

var (
	vessel  = ut(bwgame.UnitTypesTerranScienceVessel)
	defiler = ut(bwgame.UnitTypesZergDefiler)
	nydus   = ut(bwgame.UnitTypesZergNydusCanal)

	cloakedKillers = []uint16{ut(bwgame.UnitTypesProtossDarkTemplar), ut(bwgame.UnitTypesZergLurker)}
	airHarassers   = []uint16{ut(bwgame.UnitTypesZergMutalisk), ut(bwgame.UnitTypesTerranWraith)}

	staticDetectors = map[uint16]bool{ut(bwgame.UnitTypesProtossPhotonCannon): true, ut(bwgame.UnitTypesTerranMissileTurret): true, ut(bwgame.UnitTypesZergSporeColony): true}
	mobileDetector  = map[openbw.Race]uint16{openbw.RaceProtoss: ut(bwgame.UnitTypesProtossObserver), openbw.RaceTerran: vessel, openbw.RaceZerg: ut(bwgame.UnitTypesZergOverlord)}
	otherDetectors  = map[uint16]bool{ut(bwgame.UnitTypesProtossObserver): true, vessel: true, ut(bwgame.UnitTypesTerranComsatStation): true}
	staticAntiAir   = staticDetectors
	mobileAntiAir   = map[openbw.Race]map[uint16]bool{
		openbw.RaceProtoss: {ut(bwgame.UnitTypesProtossCorsair): true, ut(bwgame.UnitTypesProtossDragoon): true, ut(bwgame.UnitTypesProtossArchon): true, ut(bwgame.UnitTypesProtossScout): true, ut(bwgame.UnitTypesProtossCarrier): true, ut(bwgame.UnitTypesProtossHighTemplar): true},
		openbw.RaceTerran:  {ut(bwgame.UnitTypesTerranMarine): true, ut(bwgame.UnitTypesTerranGoliath): true, ut(bwgame.UnitTypesTerranValkyrie): true, ut(bwgame.UnitTypesTerranWraith): true, ut(bwgame.UnitTypesTerranBattlecruiser): true, ut(bwgame.UnitTypesTerranGhost): true},
		openbw.RaceZerg:    {ut(bwgame.UnitTypesZergHydralisk): true, ut(bwgame.UnitTypesZergMutalisk): true, ut(bwgame.UnitTypesZergScourge): true, ut(bwgame.UnitTypesZergDevourer): true},
	}
	notAVictim = map[uint16]bool{ut(bwgame.UnitTypesProtossScarab): true, ut(bwgame.UnitTypesProtossInterceptor): true, ut(bwgame.UnitTypesTerranVultureSpiderMine): true, ut(bwgame.UnitTypesZergLarva): true, ut(bwgame.UnitTypesZergEgg): true}
	// Reaver and High Templar shuttles unload and reload the same unit over and over; that is micro, not an elevator.
	microCargo  = map[uint16]bool{ut(bwgame.UnitTypesProtossReaver): true, ut(bwgame.UnitTypesProtossHighTemplar): true}
	biological  = map[uint16]bool{}
	lurker      = ut(bwgame.UnitTypesZergLurker)
	containArmy = set(ut(bwgame.UnitTypesZergHydralisk), ut(bwgame.UnitTypesZergZergling), ut(bwgame.UnitTypesZergUltralisk), ut(bwgame.UnitTypesZergDefiler))
)

func init() {
	for _, t := range []bwgame.UnitTypes{
		bwgame.UnitTypesTerranMarine, bwgame.UnitTypesTerranGhost, bwgame.UnitTypesTerranSCV, bwgame.UnitTypesTerranFirebat, bwgame.UnitTypesTerranMedic,
		bwgame.UnitTypesZergDrone, bwgame.UnitTypesZergZergling, bwgame.UnitTypesZergHydralisk, bwgame.UnitTypesZergUltralisk, bwgame.UnitTypesZergOverlord,
		bwgame.UnitTypesZergMutalisk, bwgame.UnitTypesZergGuardian, bwgame.UnitTypesZergQueen, bwgame.UnitTypesZergDefiler, bwgame.UnitTypesZergScourge,
		bwgame.UnitTypesZergInfestedTerran, bwgame.UnitTypesZergDevourer, bwgame.UnitTypesZergLurker, bwgame.UnitTypesZergLarva, bwgame.UnitTypesZergEgg,
		bwgame.UnitTypesProtossZealot, bwgame.UnitTypesProtossDarkTemplar, bwgame.UnitTypesProtossHighTemplar, bwgame.UnitTypesProtossArchon, bwgame.UnitTypesProtossDarkArchon,
	} {
		biological[ut(t)] = true
	}
}

func DetectInsights(d *engine.Digest, bases []Base, players []Player) Insights {
	g := newGame(d, bases, players)
	var out []Event
	out = append(out, g.undetectedCloak()...)
	out = append(out, g.undefendedAirHarass()...)
	out = append(out, g.overlordHunts()...)
	out = append(out, g.elevators()...)
	if d.CastTargetsKnown() {
		out = append(out, g.erasers()...)
	}
	out = append(out, g.defilersIrradiated()...)
	out = append(out, g.nydusDenied()...)
	out = append(out, g.lurkerContains()...)
	out = append(out, g.overruns()...)
	kept := out[:0]
	for _, e := range out {
		if e.Second < g.trusted {
			e.Insight = true
			kept = append(kept, e)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Second < kept[j].Second })
	in := Insights{Events: kept}
	if whys := g.whys(kept); len(whys) > 0 {
		in.Why = &whys[len(whys)-1]
	}
	return in
}

// leftAt is the second a player left, dropped or was defeated, or -1.
func (g *game) leftAt(slot int8) int {
	for _, o := range g.d.Outcomes {
		if o.Owner == slot && o.Kind != engine.OutcomeWon {
			return second(o.Frame)
		}
	}
	return -1
}

func (g *game) raceOf(slot int8) openbw.Race { return g.players[slot].Race }

// episodes splits deaths, oldest first, wherever two are more than gap seconds apart.
func episodes(deaths []engine.Death, gap int) [][]engine.Death {
	var out [][]engine.Death
	var cur []engine.Death
	for _, d := range deaths {
		if len(cur) > 0 && second(d.Frame)-second(cur[len(cur)-1].Frame) > gap {
			out = append(out, cur)
			cur = nil
		}
		cur = append(cur, d)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func (g *game) perBaseEpisodes(deaths []engine.Death, gap int) [][]engine.Death {
	byBase := map[int][]engine.Death{}
	var order []int
	for _, d := range deaths {
		b := g.baseOf(d.X, d.Y)
		if _, seen := byBase[b]; !seen {
			order = append(order, b)
		}
		byBase[b] = append(byBase[b], d)
	}
	var out [][]engine.Death
	for _, b := range order {
		out = append(out, episodes(byBase[b], gap)...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i][0].Frame < out[j][0].Frame })
	return out
}

// killsOf is every unit of the victim's that an enemy killed with the killer type.
func (g *game) killsOf(victim int8, killer uint16, keep func(engine.Death) bool) []engine.Death {
	var out []engine.Death
	for _, d := range g.deaths {
		if d.Owner == victim && fighter(d.KillerType) == killer && g.enemies(victim, d.KillerOwner) && !notAVictim[d.Type] && (keep == nil || keep(d)) {
			out = append(out, d)
		}
	}
	return out
}

func (g *game) diedAt(unit uint32) int {
	if g.diedSec == nil {
		g.diedSec = map[uint32]int{}
		for _, d := range g.d.Deaths {
			if _, seen := g.diedSec[d.Unit]; !seen {
				g.diedSec[d.Unit] = second(d.Frame)
			}
		}
	}
	if s, ok := g.diedSec[unit]; ok {
		return s
	}
	return 1 << 30
}

// standing is the owner's completed, alive buildings or units of the types at a second, within reach of a point
// (reach 0 is anywhere).
func (g *game) standing(owner int8, types map[uint16]bool, sec int, x, y, reach int32) int {
	n := 0
	for _, pr := range g.d.Productions {
		if pr.Owner != owner || !types[pr.Type] || pr.Complete == engine.NoFrame || second(pr.Complete) > sec || g.diedAt(pr.Unit) <= sec {
			continue
		}
		if reach == 0 || dist2(pr.X, pr.Y, x, y) <= int64(reach)*int64(reach) {
			n++
		}
	}
	return n
}

// standingDuring is the owner's completed buildings or units of the types alive at any point between two
// seconds, within reach of a point.
func (g *game) standingDuring(owner int8, types map[uint16]bool, from, to int, x, y, reach int32) int {
	n := 0
	for _, pr := range g.d.Productions {
		if pr.Owner != owner || !types[pr.Type] || pr.Complete == engine.NoFrame || second(pr.Complete) > to || g.diedAt(pr.Unit) <= from {
			continue
		}
		if dist2(pr.X, pr.Y, x, y) <= int64(reach)*int64(reach) {
			n++
		}
	}
	return n
}

// armyNear counts the owner's units of the types within reach of a point in the position sample at or before
// a second (reach 0 is anywhere), and in the sample after when within is set.
func (g *game) armyNear(owner int8, types map[uint16]bool, sec int, x, y, reach int32, base int) int {
	best := -1
	for i, snap := range g.d.Army {
		if second(snap.Frame) <= sec {
			best = i
		}
	}
	if best < 0 {
		return 0
	}
	n := 0
	for _, u := range g.d.Army[best].Units {
		if u.Owner != owner || !types[u.Type] {
			continue
		}
		ux, uy := u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels
		switch {
		case base != NoBase:
			if g.baseOf(ux, uy) == base {
				n++
			}
		case reach == 0 || dist2(ux, uy, x, y) <= int64(reach)*int64(reach):
			n++
		}
	}
	return n
}

func set(types ...uint16) map[uint16]bool {
	m := map[uint16]bool{}
	for _, t := range types {
		m[t] = true
	}
	return m
}

func (g *game) insight(kind string, actor, target int8, ds []engine.Death, units []uint16) Event {
	first, last := ds[0], ds[len(ds)-1]
	b := g.baseOf(first.X, first.Y)
	e := g.event(kind, second(first.Frame), actor, target, b, first.X, first.Y, units)
	e.SnapshotSecond = peakOf(ds)
	e.Seconds = second(last.Frame) - second(first.Frame)
	e.Outcome = g.outcome(actor, ds)
	e.Count = len(ds)
	e.Region = fightRegion(ds, around(first.X, first.Y, buildingFocus+2*tile), first.X, first.Y)
	for _, d := range ds {
		if d.Killer != 0 && !slices.Contains(e.Members, d.Killer) {
			e.Members = append(e.Members, d.Killer)
		}
	}
	return e
}

// peakOf is the second with the most deaths within the next peakWindow seconds.
func peakOf(ds []engine.Death) int {
	best, bestN := second(ds[0].Frame), 0
	for i, d := range ds {
		n := 0
		for _, o := range ds[i:] {
			if second(o.Frame)-second(d.Frame) <= peakWindow {
				n++
			}
		}
		if n > bestN {
			best, bestN = second(d.Frame), n
		}
	}
	return best
}

// detection says whether the victim could see a cloaked unit at a point: a static detector or mobile one there,
// only Scanner Sweeps, a detector elsewhere, or none at all.
func (g *game) detection(victim int8, sec int, x, y int32) string {
	if g.standing(victim, staticDetectors, sec, x, y, cloakStaticReach) > 0 {
		return ""
	}
	mobile := set(mobileDetector[g.raceOf(victim)])
	if g.armyNear(victim, mobile, sec, x, y, cloakMobileReach, NoBase) > 0 || g.armyNear(victim, mobile, sec+10, x, y, cloakMobileReach, NoBase) > 0 {
		return ""
	}
	if g.standing(victim, set(ut(bwgame.UnitTypesTerranComsatStation)), sec, 0, 0, 0) > 0 {
		return CloakScansOnly
	}
	if g.standing(victim, otherDetectors, sec, 0, 0, 0) > 0 || g.standing(victim, staticDetectors, sec, 0, 0, 0) > 0 {
		return CloakNotThere
	}
	return CloakNoDetection
}

func (g *game) homeBase(slot int8, b int) bool {
	if b == NoBase {
		return false
	}
	if main, ok := g.mains[slot]; ok && (b == main || g.naturalOf(b) == main) {
		return true
	}
	return g.holder(b, 0) == slot
}

// undetectedCloak is Dark Templar or Lurkers killing 4+ units, or 600 worth, where the victim had no detection.
// A Terran with a Comsat or a player with a detector elsewhere only counts when it cost them workers at home.
func (g *game) undetectedCloak() []Event {
	var out []Event
	for _, killer := range cloakedKillers {
		for victim := range g.players {
			for _, ep := range g.perBaseEpisodes(g.killsOf(victim, killer, nil), cloakEpisodeGap) {
				if len(ep) < cloakMinKills {
					continue
				}
				first := ep[0]
				status := g.detection(victim, second(first.Frame), first.X, first.Y)
				o := g.outcome(first.KillerOwner, ep)
				home := g.homeBase(victim, g.baseOf(first.X, first.Y))
				switch {
				case status == "":
					continue
				case status == CloakNotThere && !(home && o.Workers >= 4):
					continue
				case status == CloakScansOnly && !(home && o.Workers >= 6):
					continue
				}
				if o.Workers < cloakMinKills && o.Killed < cloakMinValue {
					continue
				}
				e := g.insight(InsightUndetectedCloak, first.KillerOwner, victim, ep, []uint16{killer})
				e.Result = status
				out = append(out, e)
			}
		}
	}
	return out
}

// undefendedAirHarass is Mutalisks or Wraiths killing 5+ workers at a base that had no static anti-air in
// the minute before and fewer than 3 mobile anti-air units there.
func (g *game) undefendedAirHarass() []Event {
	var out []Event
	for _, killer := range airHarassers {
		for victim := range g.players {
			workers := g.killsOf(victim, killer, func(d engine.Death) bool { return isWorker(d.Type) })
			for _, ep := range g.perBaseEpisodes(workers, airEpisodeGap) {
				if len(ep) < airMinWorkers {
					continue
				}
				first := ep[0]
				sec, b := second(first.Frame), g.baseOf(first.X, first.Y)
				if b == NoBase || g.standingDuring(victim, staticAntiAir, sec-airDefenceLookBack, sec, g.bases[b].X, g.bases[b].Y, baseRadius) > 0 {
					continue
				}
				mobile := g.armyNear(victim, mobileAntiAir[g.raceOf(victim)], sec, 0, 0, 0, b)
				if mobile >= airMobileEnough {
					continue
				}
				e := g.insight(InsightUndefendedAir, first.KillerOwner, victim, ep, []uint16{killer})
				e.Defenders = mobile
				out = append(out, e)
			}
		}
	}
	return out
}

// overlordHunts is 5+ Overlords killed in one area with under a minute between them, and how long Zerg was supply
// blocked from the first until a minute after the last, timed at the biggest burst. A Zerg who has already
// lost is left out.
func (g *game) overlordHunts() []Event {
	var out []Event
	overlord := ut(bwgame.UnitTypesZergOverlord)
	for victim, p := range g.players {
		if p.Race != openbw.RaceZerg {
			continue
		}
		var ds []engine.Death
		for _, d := range g.deaths {
			if d.Owner == victim && d.Type == overlord && g.enemies(victim, d.KillerOwner) && !g.droppingNear(victim, d) {
				ds = append(ds, d)
			}
		}
		for _, ep := range huntEpisodes(ds) {
			if len(ep) < huntMinOverlords {
				continue
			}
			blocked := g.blockedSeconds(victim, second(ep[0].Frame), second(ep[len(ep)-1].Frame)+huntBlockAfter)
			if len(ep) < huntManyOverlords && blocked < huntLongBlock || g.alreadyLost(victim, ep[0].KillerOwner, second(ep[0].Frame)) {
				continue
			}
			var killers []uint16
			for _, d := range ep {
				killers = append(killers, fighter(d.KillerType))
			}
			burst := slices.MaxFunc(episodes(ep, huntBurstGap), func(a, b []engine.Death) int { return len(a) - len(b) })
			e := g.insight(InsightOverlordHunt, ep[0].KillerOwner, victim, burst, byCount(killers))
			e.Count = len(ep)
			e.Seconds = blocked
			out = append(out, e)
		}
	}
	return out
}

// huntEpisodes splits deaths, oldest first, into groups chained by deaths in one area under a minute apart,
// or close enough in time for one party to have flown from one to the other.
func huntEpisodes(deaths []engine.Death) [][]engine.Death {
	group := make([]int, len(deaths))
	for i := range group {
		group[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for group[i] != i {
			i = group[i]
		}
		return i
	}
	for i, a := range deaths {
		for j := i + 1; j < len(deaths) && second(deaths[j].Frame)-second(a.Frame) <= huntEpisodeGap; j++ {
			b := deaths[j]
			reach := int64(huntReach)
			if dt := second(b.Frame) - second(a.Frame); dt <= huntChaseGap {
				reach += int64(dt) * huntChaseSpeed
			}
			if dist2(a.X, a.Y, b.X, b.Y) <= reach*reach {
				group[root(j)] = root(i)
			}
		}
	}
	byRoot := map[int][]engine.Death{}
	var order []int
	for i, d := range deaths {
		r := root(i)
		if _, seen := byRoot[r]; !seen {
			order = append(order, r)
		}
		byRoot[r] = append(byRoot[r], d)
	}
	out := make([][]engine.Death, 0, len(order))
	for _, r := range order {
		out = append(out, byRoot[r])
	}
	return out
}

func (g *game) alreadyLost(victim, enemy int8, sec int) bool {
	var mine, theirs engine.EconomySample
	for _, s := range g.d.Economy {
		if second(s.Frame) > sec {
			break
		}
		switch s.Owner {
		case victim:
			mine = s
		case enemy:
			theirs = s
		}
	}
	if theirs.SupplyUsed == 0 {
		return false
	}
	return mine.SupplyUsed*huntLostSupplyRatio <= theirs.SupplyUsed ||
		mine.SupplyUsed*huntBehindSupplyRatio <= theirs.SupplyUsed && mine.Workers*huntLostWorkerRatio <= theirs.Workers
}

func (g *game) droppingNear(owner int8, d engine.Death) bool {
	sec := second(d.Frame)
	return slices.ContainsFunc(g.d.Unloads, func(u engine.Unload) bool {
		us := second(u.Frame)
		return u.Owner == owner && us-sec <= dropOverlordSeconds && sec-us <= dropOverlordSeconds && dist2(u.X, u.Y, d.X, d.Y) <= dropOverlordReach*dropOverlordReach
	})
}

func (g *game) blockedSeconds(owner int8, from, to int) int {
	n := 0
	for _, s := range g.d.Economy {
		sec := second(s.Frame)
		if s.Owner == owner && sec >= from && sec <= to && s.SupplyMax > 0 && s.SupplyMax < supplyCap && s.SupplyUsed >= s.SupplyMax {
			n++
		}
	}
	return n
}

type unloadRun struct {
	actor, target int8
	base          int
	unloads       []engine.Unload
}

// enemyUnloads groups combat unloads by player and enemy base: unloads gap seconds or less apart are one run.
func (g *game) enemyUnloads(gap int) []*unloadRun {
	var runs []*unloadRun
	for _, u := range g.d.Unloads {
		if second(u.Frame) >= g.trusted {
			continue
		}
		sec := second(u.Frame)
		b := g.baseOf(u.X, u.Y)
		target := g.holder(b, sec)
		if !g.enemies(u.Owner, target) || !slices.ContainsFunc(u.Cargo, func(t uint16) bool { return !isWorker(t) }) {
			continue
		}
		var r *unloadRun
		for _, open := range runs {
			if open.actor == u.Owner && open.base == b && sec-second(open.unloads[len(open.unloads)-1].Frame) <= gap {
				r = open
			}
		}
		if r == nil {
			r = &unloadRun{actor: u.Owner, target: target, base: b}
			runs = append(runs, r)
		}
		r.unloads = append(r.unloads, u)
	}
	return runs
}

func combatCargo(us []engine.Unload) []uint16 {
	var out []uint16
	for _, u := range us {
		for _, t := range u.Cargo {
			if !isWorker(t) {
				out = append(out, t)
			}
		}
	}
	return out
}

// elevators is two or more hops over a base's wall to the same spot, one after another: cargo loaded just
// outside the base's edge and unloaded just inside, a short flight away. Transports flying together are one hop.
func (g *game) elevators() []Event {
	var out []Event
	for _, r := range g.enemyUnloads(elevatorWindow) {
		for _, trips := range hopClusters(g.hops(r)) {
			if waves(trips) < elevatorMinTrips {
				continue
			}
			var cargo []uint16
			for _, t := range trips {
				cargo = append(cargo, t.cargo...)
			}
			if len(slices.DeleteFunc(slices.Clone(cargo), func(t uint16) bool { return microCargo[t] })) < elevatorMinCargo {
				continue
			}
			first, last := trips[0].first, trips[len(trips)-1]
			e := g.event(InsightElevator, second(first.Frame), r.actor, r.target, r.base, first.X, first.Y, dropUnits(first.TransportType, 0, cargo)).shownAt(second(last.end) + arrivalLead)
			e.Count = waves(trips)
			deaths := g.fight(r.actor, r.target, r.base, second(first.Frame), second(last.end)+fightFollowUp)
			e.Outcome = g.outcome(r.actor, deaths)
			e.Region = fightRegion(deaths, g.regionOfBase(r.base, first.X, first.Y), first.X, first.Y)
			for _, t := range trips {
				e.Path = append(e.Path, [2]int32{t.first.X, t.first.Y})
				e.Members = append(e.Members, t.units...)
			}
			out = append(out, e)
		}
	}
	return out
}

// hop is one transport's load carried over a wall: unloads of cargo that boarded before its first unload.
type hop struct {
	first engine.Unload
	end   int32
	cargo []uint16
	units []uint32
}

// hops is a run's Shuttle and Dropship trips over the base's wall, oldest first.
func (g *game) hops(r *unloadRun) []hop {
	if r.base == NoBase || len(g.bases[r.base].Polygon) < 3 {
		return nil
	}
	poly := g.bases[r.base].Polygon
	open := map[uint32]int{}
	var out []hop
	for _, u := range r.unloads {
		if u.LoadFrame == engine.NoFrame || u.TransportType != ut(bwgame.UnitTypesProtossShuttle) && u.TransportType != ut(bwgame.UnitTypesTerranDropship) {
			continue
		}
		if inPolygon(poly, u.LoadX, u.LoadY) || !inPolygon(poly, u.X, u.Y) || dist2(u.LoadX, u.LoadY, u.X, u.Y) > elevatorHop*elevatorHop {
			continue
		}
		if edgeDist2(poly, u.LoadX, u.LoadY) > elevatorLoadEdge*elevatorLoadEdge || edgeDist2(poly, u.X, u.Y) > elevatorUnloadEdge*elevatorUnloadEdge {
			continue
		}
		if i, ok := open[u.Transport]; ok && u.LoadFrame <= out[i].first.Frame {
			out[i].end = u.Frame
			out[i].cargo = append(out[i].cargo, u.Cargo...)
			out[i].units = append(out[i].units, u.Units...)
			continue
		}
		open[u.Transport] = len(out)
		out = append(out, hop{first: u, end: u.Frame, cargo: slices.Clone(u.Cargo), units: slices.Clone(u.Units)})
	}
	return out
}

// hopClusters groups hops whose load and unload points are both near the group's first hop.
func hopClusters(trips []hop) [][]hop {
	var out [][]hop
	for _, t := range trips {
		i := slices.IndexFunc(out, func(c []hop) bool {
			f := c[0].first
			return dist2(f.LoadX, f.LoadY, t.first.LoadX, t.first.LoadY) <= elevatorNear*elevatorNear && dist2(f.X, f.Y, t.first.X, t.first.Y) <= elevatorNear*elevatorNear
		})
		if i < 0 {
			out = append(out, []hop{t})
		} else {
			out[i] = append(out[i], t)
		}
	}
	return out
}

// waves counts hops made one after another: a hop that starts unloading while the previous wave still is
// flew alongside it.
func waves(trips []hop) int {
	n, end := 0, int32(-1)
	for _, t := range trips {
		if t.first.Frame > end+elevatorTogether {
			n++
			end = t.end
			continue
		}
		end = max(end, t.end)
	}
	return n
}

// edgeDist2 is the squared distance from a point to the nearest edge of a polygon.
func edgeDist2(poly [][2]int32, x, y int32) int64 {
	best := int64(1) << 62
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		best = min(best, segmentDist2(poly[j], poly[i], x, y))
	}
	return best
}

func segmentDist2(a, b [2]int32, x, y int32) int64 {
	dx, dy := float64(b[0]-a[0]), float64(b[1]-a[1])
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = min(1, max(0, (float64(x-a[0])*dx+float64(y-a[1])*dy)/l))
	}
	px, py := float64(a[0])+t*dx, float64(a[1])+t*dy
	ex, ey := float64(x)-px, float64(y)-py
	return int64(ex*ex + ey*ey)
}

// erasers is a Science Vessel irradiated by its own side and flown over the enemy: the caster's kills of
// biological units while it lasts, leaving out kills next to another Irradiate cast meanwhile.
func (g *game) erasers() []Event {
	var out []Event
	last := map[int8]int{}
	for _, c := range g.d.Casts {
		sec := second(c.Frame)
		if c.Order != orderIrradiate || c.TargetType != vessel || c.TargetOwner != c.Owner {
			continue
		}
		if prev, ok := last[c.Owner]; ok && sec-prev < eraserWindow {
			continue
		}
		last[c.Owner] = sec
		var others [][2]int32
		for _, o := range g.d.Casts {
			if os := second(o.Frame); o.Owner == c.Owner && o.Order == orderIrradiate && o.TargetType != vessel && os >= sec && os <= sec+eraserWindow {
				others = append(others, [2]int32{o.X, o.Y})
			}
		}
		var ds []engine.Death
		for _, d := range g.deaths {
			ds2 := second(d.Frame)
			if ds2 < sec || ds2 > sec+eraserWindow || d.KillerType != vessel || d.KillerOwner != c.Owner || !g.enemies(c.Owner, d.Owner) || !biological[d.Type] {
				continue
			}
			if slices.ContainsFunc(others, func(p [2]int32) bool { return dist2(p[0], p[1], d.X, d.Y) <= eraserOtherReach*eraserOtherReach }) {
				continue
			}
			ds = append(ds, d)
		}
		o := g.outcome(c.Owner, ds)
		if o.Workers < eraserMinWorkers {
			continue
		}
		e := g.insight(InsightEraser, c.Owner, ds[0].Owner, ds, []uint16{vessel})
		for _, d := range ds {
			e.Path = append(e.Path, [2]int32{d.X, d.Y})
		}
		out = append(out, e)
	}
	return out
}

// defilersIrradiated is three or more Defilers killed by Science Vessels with under 90 s between them; a
// Science Vessel only kills with Irradiate.
func (g *game) defilersIrradiated() []Event {
	var out []Event
	for victim := range g.players {
		ds := g.killsOf(victim, vessel, func(d engine.Death) bool { return d.Type == defiler })
		for _, ep := range episodes(ds, defilerBurstGap) {
			if len(ep) < defilerMinBurst {
				continue
			}
			e := g.insight(InsightDefilersIrradiate, ep[0].KillerOwner, victim, ep, []uint16{vessel})
			fight := g.fight(e.Actor, victim, NoBase, e.Second, e.Second+e.Seconds+fightFollowUp)
			e.Outcome = g.outcome(e.Actor, fight)
			out = append(out, e)
		}
	}
	return out
}

// nydusDenied is a Nydus Canal the enemy killed before anything came through it, or right before taking the
// base it was there to defend.
func (g *game) nydusDenied() []Event {
	var out []Event
	for _, pr := range g.d.Productions {
		if pr.Type != nydus || pr.Complete == engine.NoFrame {
			continue
		}
		var death *engine.Death
		for i := range g.deaths {
			if d := &g.deaths[i]; d.Unit == pr.Unit && g.enemies(pr.Owner, d.KillerOwner) {
				death = d
			}
		}
		if death == nil {
			continue
		}
		dsec := second(death.Frame)
		used := false
		for _, t := range g.d.Teleports {
			ts := second(t.Frame)
			if t.Owner == pr.Owner && ts >= second(pr.Complete) && ts <= dsec && (dist2(t.ToX, t.ToY, pr.X, pr.Y) <= nydusUseReach*nydusUseReach || dist2(t.FromX, t.FromY, pr.X, pr.Y) <= nydusUseReach*nydusUseReach) {
				used = true
			}
		}
		var hall []engine.Death
		for _, d := range g.deaths {
			if d.Owner == pr.Owner && IsTownHall(d.Type) && g.enemies(pr.Owner, d.KillerOwner) && second(d.Frame) >= dsec && second(d.Frame) <= dsec+nydusBaseAfter && dist2(d.X, d.Y, pr.X, pr.Y) <= nydusBaseReach*nydusBaseReach {
				hall = append(hall, d)
			}
		}
		if used && len(hall) == 0 {
			continue
		}
		e := g.event(InsightNydusDenied, dsec, death.KillerOwner, pr.Owner, g.baseOf(pr.X, pr.Y), pr.X, pr.Y, []uint16{nydus}).shownAt(max(0, dsec-1))
		e.Region = around(pr.X, pr.Y, buildingFocus+3*tile)
		if len(hall) > 0 {
			e.Outcome = g.outcome(death.KillerOwner, hall)
		}
		out = append(out, e)
	}
	return out
}

// lurkerContains is Lurkers held still outside a choke of a base the enemy holds, with an army beside them,
// for a minute or more: the enemy cannot walk out without running into them. One army held between two
// chokes is one contain, at the choke it held longest.
func (g *game) lurkerContains() []Event {
	type key struct {
		actor, target int8
		base, choke   int
	}
	type span struct {
		from, to, peak int
		units          []uint16
		lurkers        []uint32
	}
	open := map[key]*span{}
	var out []Event
	closeSpan := func(k key, sp *span) {
		if sp.to-sp.from < containMinSeconds {
			return
		}
		c := g.bases[k.base].Chokes[k.choke]
		e := g.event(InsightLurkerContain, sp.from, k.actor, k.target, k.base, c.X, c.Y, byCount(sp.units)).shownAt(sp.from)
		e.Seconds = sp.to - sp.from
		e.Count = sp.peak
		e.Region = around(c.X, c.Y, containSupportReach)
		e.Outcome = g.outcome(k.actor, g.fightNear(k.actor, k.target, sp.from, sp.to, c.X, c.Y, containSupportReach))
		e.Members = sp.lurkers
		out = append(out, e)
	}
	var prev map[uint32][2]int32
	for _, snap := range g.d.Army {
		sec := second(snap.Frame)
		if sec >= g.trusted {
			break
		}
		at := map[uint32][2]int32{}
		for _, u := range snap.Units {
			if u.Type == lurker {
				at[u.ID] = [2]int32{u.X * engine.ArmyCellPixels, u.Y * engine.ArmyCellPixels}
			}
		}
		seen := map[key]bool{}
		for b, base := range g.bases {
			target := g.heldBy(b, sec)
			if target == NoPlayer {
				continue
			}
			for ci, c := range base.Chokes {
				if c.To != NoBase && g.heldBy(c.To, sec) == target {
					continue
				}
				for actor := range g.players {
					if !g.enemies(actor, target) {
						continue
					}
					lurkers, support := 0, 0
					var units []uint16
					var held []uint32
					for _, u := range snap.Units {
						if u.Owner != actor {
							continue
						}
						x, y := u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels
						switch {
						case u.Type == lurker && dist2(x, y, c.X, c.Y) <= containReach*containReach && !inPolygon(base.Polygon, x, y):
							if p, ok := prev[u.ID]; ok && dist2(p[0], p[1], x, y) <= containHeldReach*containHeldReach {
								lurkers++
								units = append(units, u.Type)
								held = append(held, u.ID)
							}
						case containArmy[u.Type] && dist2(x, y, c.X, c.Y) <= containSupportReach*containSupportReach:
							support++
							units = append(units, u.Type)
						}
					}
					if lurkers < containMinLurkers || support < containMinSupport {
						continue
					}
					k := key{actor, target, b, ci}
					seen[k] = true
					sp := open[k]
					if sp == nil {
						sp = &span{from: sec}
						open[k] = sp
					}
					sp.to = sec
					for _, id := range held {
						if !slices.Contains(sp.lurkers, id) {
							sp.lurkers = append(sp.lurkers, id)
						}
					}
					if lurkers > sp.peak {
						sp.peak, sp.units = lurkers, units
					}
				}
			}
		}
		for k, sp := range open {
			if !seen[k] && sec-sp.to > engine.ArmyEveryFrames/24 {
				closeSpan(k, sp)
				delete(open, k)
			}
		}
		prev = at
	}
	for k, sp := range open {
		closeSpan(k, sp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seconds != out[j].Seconds {
			return out[i].Seconds > out[j].Seconds
		}
		return out[i].Second < out[j].Second || out[i].Second == out[j].Second && out[i].Base < out[j].Base
	})
	var kept []Event
	for _, e := range out {
		if !slices.ContainsFunc(kept, func(k Event) bool {
			return k.Actor == e.Actor && k.Target == e.Target && e.Second <= k.Second+k.Seconds && k.Second <= e.Second+e.Seconds
		}) {
			kept = append(kept, e)
		}
	}
	return kept
}

// heldBy is who holds a base at a second; a main nobody has a town hall in belongs to the player who started there.
func (g *game) heldBy(b, sec int) int8 {
	if owner := g.holder(b, sec); owner != NoPlayer || g.bases[b].Kind != BaseStart {
		return owner
	}
	for slot, main := range g.mains {
		if main == b {
			return slot
		}
	}
	return NoPlayer
}

// fightNear is the deaths between two players within reach of a point between two seconds.
func (g *game) fightNear(a, b int8, from, to int, x, y, reach int32) []engine.Death {
	var out []engine.Death
	for _, d := range g.deaths {
		sec := second(d.Frame)
		if sec >= from && sec <= to && (d.Owner == a && d.KillerOwner == b || d.Owner == b && d.KillerOwner == a) && dist2(d.X, d.Y, x, y) <= int64(reach)*int64(reach) {
			out = append(out, d)
		}
	}
	return out
}

func decisive(e Event) bool {
	o := e.Outcome
	switch e.Type {
	case InsightUndetectedCloak, InsightUndefendedAir, InsightOverrun:
		return true
	case InsightElevator:
		return o.TownHall || o.Workers >= 8 || o.Killed >= 1500
	case InsightEraser:
		return o.Workers >= 6
	case InsightDefilersIrradiate:
		return o.Killed >= 1500 && o.Killed >= 2*o.Lost
	}
	return false
}
