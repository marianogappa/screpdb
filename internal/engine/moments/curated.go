// Package moments finds the curated game events of a game from its engine digest: the same event types
// the command-inferred detectors guess at (attacks, rushes, proxies, drops, recalls, nydus, expansions,
// nukes, first tech, mind control), decided from where units really were and what really happened.
// Few events on purpose: an event must be worth looking at, not merely have occurred.
package moments

import (
	"math"
	"slices"
	"sort"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

const (
	tile            = 32
	framesPerSecond = engine.EconomyEveryFrames
	NoPlayer        = int8(-1)
	NoBase          = -1

	// A point belongs to the base whose polygon holds it, else to the nearest base centre within 15 tiles.
	baseRadius = 15 * tile
	// Rushes: Zerglings from a 9 Pool or earlier (Pool started by 1:10; an overpool starts it near 1:16,
	// a 12 Pool near 1:30) standing in an enemy main or natural by 3:30; a Cannon by 4:00 or a Bunker
	// by 5:00 in an enemy base.
	rushPoolBy = 70
	rushLingBy = 210
	// A rush commits: overpool Zerglings poking a natural are standard play. The lings must kill 2 units
	// or a building within 90 s, or 6 of them must stay in the base for two position samples (10 s apart).
	rushDamageWindow  = 90
	rushMinKills      = 2
	rushStayLings     = 6
	rushStaySeconds   = 10
	rushLingsMin      = 3
	cannonRushBy      = 240
	bunkerRushBy      = 300
	rushBuildingReach = 10 * tile
	bunkerMarineReach = 10 * tile
	proxyEarlyBy      = 240
	proxyLateBy       = 300
	mannerPylonBy     = 480
	firstTechBefore   = 600
	// Drops: unloads by one player at one base within 20 s are one drop.
	dropGapSeconds   = 20
	dropFollowUp     = 30
	dropMinCargo     = 4
	teleportMinUnits = 2
	recallCastBefore = 10
	recallCastReach  = 12 * tile
	nydusRepeatAfter = 180
	// Attacks: an army of 4+ combat units inside a base the enemy holds, with blood drawn on either side.
	attackMinUnits    = 4
	attackGapSeconds  = 30
	attackMinTrade    = 300
	attackMinDeaths   = 3
	decisiveKilled    = 1000
	fieldBattleTraded = 3000
	fieldBattleRadius = 14 * tile
	leaveAfterAttack  = 60
	// One attack per attacker per minute: the same push seen at two bases is one moment.
	attackDedupSeconds = 60
	regionMargin       = 6 * tile
	minRegionSide      = 20 * tile
	// A special event at a base inside an attack window speaks for that moment; the attack is dropped.
	attackOverlapSeconds = 10
)

type BaseKind string

const (
	BaseStart     BaseKind = "start"
	BaseNatural   BaseKind = "natural"
	BaseExpansion BaseKind = "expansion"
)

// Base is a base location of the map in map pixels.
type Base struct {
	Kind    BaseKind
	X, Y    int32
	Polygon [][2]int32
}

type Player struct {
	Slot  int8
	Race  openbw.Race
	Human bool
}

// Event is one curated moment. Base is an index into the bases given to Detect, or NoBase.
// Region is what the map should show: the fight, the new base, the drop, not the whole base area.
type Event struct {
	Type           string
	Second         int
	SnapshotSecond int
	Actor, Target  int8
	Base           int
	SourceBase     int
	X, Y           int32
	Units          []uint16
	Region         Region
	Outcome        Outcome
	// Seconds is how long the event lasted, for those that are a span (a supply block).
	Seconds int
}

// Outcome is what an attack, drop or harass did, from the actor's side.
type Outcome struct {
	// Killed and Lost are resources: the target's units the actor killed there, and the actor's own.
	Killed, Lost int32
	Workers      int
	Buildings    int
	TownHall     bool
	// First is the actor's first attack on that target.
	First bool
}

// Region is a rectangle of the map in pixels.
type Region struct{ X, Y, W, H int32 }

func second(frame int32) int { return int(frame) / framesPerSecond }

type game struct {
	d         *engine.Digest
	bases     []Base
	players   map[int8]Player
	trusted   int
	ownership [][]ownedSpan
	deaths    []engine.Death
	mains     map[int8]int
}

type ownedSpan struct {
	owner    int8
	from, to int
}

func Detect(d *engine.Digest, bases []Base, players []Player) []Event {
	g := &game{d: d, bases: bases, players: map[int8]Player{}, trusted: second(d.TrustedUntil()), mains: map[int8]int{}}
	for _, p := range players {
		g.players[p.Slot] = p
	}
	g.ownership = g.buildOwnership()
	for _, x := range d.Deaths {
		if second(x.Frame) < g.trusted {
			g.deaths = append(g.deaths, x)
		}
	}
	var out []Event
	out = append(out, g.expansions()...)
	out = append(out, g.zerglingRushes()...)
	out = append(out, g.buildingRushes()...)
	out = append(out, g.proxies()...)
	out = append(out, g.mannerPylons()...)
	out = append(out, g.firstTech()...)
	out = append(out, g.becameRace()...)
	out = append(out, g.drops()...)
	out = append(out, g.teleports()...)
	out = append(out, g.nukes()...)
	out = append(out, g.attacks(out)...)
	out = append(out, g.workerKills(out)...)
	out = append(out, g.supplyBlocks()...)
	lastShown := min(g.trusted-1, second(d.LastFrame))
	kept := out[:0]
	for _, e := range out {
		if e.Second < g.trusted {
			e.SnapshotSecond = max(e.Second, min(e.SnapshotSecond, lastShown))
			kept = append(kept, e)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Second < kept[j].Second })
	return kept
}

func (g *game) baseOf(x, y int32) int {
	for i, b := range g.bases {
		if len(b.Polygon) >= 3 && inPolygon(b.Polygon, x, y) {
			return i
		}
	}
	best, bestDist := NoBase, int64(baseRadius*baseRadius)
	for i, b := range g.bases {
		if d := dist2(b.X, b.Y, x, y); d <= bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func inPolygon(poly [][2]int32, x, y int32) bool {
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		xi, yi, xj, yj := poly[i][0], poly[i][1], poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) && float64(x) < float64(xj-xi)*float64(y-yi)/float64(yj-yi)+float64(xi) {
			inside = !inside
		}
	}
	return inside
}

func dist2(ax, ay, bx, by int32) int64 {
	dx, dy := int64(ax-bx), int64(ay-by)
	return dx*dx + dy*dy
}

// buildOwnership gives each base its holders over time: a player holds a base from the moment a town hall
// of theirs starts there until it dies or the construction is cancelled.
func (g *game) buildOwnership() [][]ownedSpan {
	out := make([][]ownedSpan, len(g.bases))
	deathOf := map[uint32]int{}
	for _, x := range g.d.Deaths {
		if _, seen := deathOf[x.Unit]; !seen {
			deathOf[x.Unit] = second(x.Frame)
		}
	}
	end := func(unit uint32) int {
		if s, ok := deathOf[unit]; ok {
			return s
		}
		return math.MaxInt32
	}
	for _, u := range g.d.Initial {
		if !IsTownHall(u.Type) {
			continue
		}
		if b := g.baseOf(u.X, u.Y); b != NoBase {
			out[b] = append(out[b], ownedSpan{owner: u.Owner, from: 0, to: end(u.Unit)})
			g.mains[u.Owner] = b
		}
	}
	for _, pr := range g.d.Productions {
		if !IsTownHall(pr.Type) || IsTownHall(pr.From) {
			continue
		}
		b := g.baseOf(pr.X, pr.Y)
		if b == NoBase {
			continue
		}
		to := end(pr.Unit)
		if pr.Cancel != engine.NoFrame {
			to = min(to, second(pr.Cancel))
		}
		out[b] = append(out[b], ownedSpan{owner: pr.Owner, from: second(pr.Start), to: to})
	}
	return out
}

func (g *game) holder(base, sec int) int8 {
	if base == NoBase {
		return NoPlayer
	}
	for _, s := range g.ownership[base] {
		if sec >= s.from && sec < s.to {
			return s.owner
		}
	}
	return NoPlayer
}

func (g *game) enemies(a, b int8) bool {
	return a != b && a != NoPlayer && b != NoPlayer && g.players[a].Human && g.players[b].Human
}

// enemyHomeBase is the main or natural an enemy of actor holds at the point, for rushes that only
// make sense at home bases.
func (g *game) enemyHomeBase(actor int8, x, y int32, sec int) (int, int8) {
	b := g.baseOf(x, y)
	if b == NoBase || g.bases[b].Kind == BaseExpansion {
		return NoBase, NoPlayer
	}
	owner := g.holder(b, sec)
	if g.bases[b].Kind == BaseStart && owner == NoPlayer {
		for slot, main := range g.mains {
			if main == b {
				owner = slot
			}
		}
	}
	if !g.enemies(actor, owner) {
		return NoBase, NoPlayer
	}
	return b, owner
}

func (g *game) regionOfBase(b int, x, y int32) Region {
	if b != NoBase && len(g.bases[b].Polygon) >= 3 {
		minX, minY, maxX, maxY := int32(math.MaxInt32), int32(math.MaxInt32), int32(math.MinInt32), int32(math.MinInt32)
		for _, p := range g.bases[b].Polygon {
			minX, minY, maxX, maxY = min(minX, p[0]), min(minY, p[1]), max(maxX, p[0]), max(maxY, p[1])
		}
		return regionAround(minX, minY, maxX, maxY)
	}
	return regionAround(x, y, x, y)
}

func regionAround(minX, minY, maxX, maxY int32) Region {
	r := Region{X: minX - regionMargin, Y: minY - regionMargin, W: maxX - minX + 2*regionMargin, H: maxY - minY + 2*regionMargin}
	if r.W < minRegionSide {
		r.X -= (minRegionSide - r.W) / 2
		r.W = minRegionSide
	}
	if r.H < minRegionSide {
		r.Y -= (minRegionSide - r.H) / 2
		r.H = minRegionSide
	}
	return r
}

// The map shows a moment a little after an event starts: a building placed this second is still a worker
// on the map, an unload is one unit at a time, and a unit that just finished is still in its building.
const (
	placedLead  = 2
	arrivalLead = 1
	// A new town hall is shown well under way, so the map shows the base going up, not a worker.
	expansionLead = 12
	// buildingFocus frames a single building with what stands around it, in pixels from its centre.
	buildingFocus = 5 * tile
	// A transport empties in about three seconds; a drop that goes on longer is shown at the first one.
	dropUnloadSeconds = 3
)

func (e Event) shownAt(sec int) Event {
	e.SnapshotSecond = sec
	return e
}

func around(x, y, r int32) Region { return regionAround(x-r, y-r, x+r, y+r) }

func (g *game) event(kind string, sec int, actor, target int8, base int, x, y int32, units []uint16) Event {
	return Event{Type: kind, Second: sec, SnapshotSecond: sec, Actor: actor, Target: target, Base: base, SourceBase: NoBase, X: x, Y: y, Units: units, Region: g.regionOfBase(base, x, y)}
}

func ut(t bwgame.UnitTypes) uint16 { return uint16(t) }

func (g *game) expansions() []Event {
	var out []Event
	held := map[int8]map[int]bool{}
	for b, spans := range g.ownership {
		for _, s := range spans {
			if s.from == 0 {
				if held[s.owner] == nil {
					held[s.owner] = map[int]bool{}
				}
				held[s.owner][b] = true
			}
		}
	}
	for _, pr := range g.d.Productions {
		if !IsTownHall(pr.Type) || IsTownHall(pr.From) || pr.Cancel != engine.NoFrame {
			continue
		}
		b := g.baseOf(pr.X, pr.Y)
		if b == NoBase || held[pr.Owner][b] {
			continue
		}
		if held[pr.Owner] == nil {
			held[pr.Owner] = map[int]bool{}
		}
		held[pr.Owner][b] = true
		e := g.event("expansion", second(pr.Start), pr.Owner, NoPlayer, b, pr.X, pr.Y, []uint16{pr.Type}).shownAt(second(pr.Start) + expansionLead)
		e.Region = around(pr.X, pr.Y, buildingFocus)
		out = append(out, e)
	}
	return out
}

// zerglingRushes fires when Zerglings of an early pool really stood in an enemy main or natural.
func (g *game) zerglingRushes() []Event {
	var out []Event
	done := map[int8]bool{}
	earlyPool := map[int8]bool{}
	for _, pr := range g.d.Productions {
		if pr.Type == ut(bwgame.UnitTypesZergSpawningPool) && second(pr.Start) <= rushPoolBy {
			earlyPool[pr.Owner] = true
		}
	}
	for _, snap := range g.d.Army {
		sec := second(snap.Frame)
		if sec > rushLingBy {
			break
		}
		count := map[[2]int64]int{}
		where := map[[2]int64][2]int32{}
		for _, u := range snap.Units {
			if u.Type != ut(bwgame.UnitTypesZergZergling) || done[u.Owner] || !earlyPool[u.Owner] {
				continue
			}
			x, y := u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels
			b, owner := g.enemyHomeBase(u.Owner, x, y, sec)
			if b == NoBase {
				continue
			}
			k := [2]int64{int64(u.Owner), int64(b)<<8 | int64(owner)}
			count[k]++
			where[k] = [2]int32{x, y}
		}
		for k, n := range count {
			actor := int8(k[0])
			if n < rushLingsMin || done[actor] {
				continue
			}
			b, target := int(k[1]>>8), int8(k[1]&0xff)
			if !g.lingsCommitted(actor, target, b, sec) {
				continue
			}
			done[actor] = true
			e := g.event("zergling_rush", sec, actor, target, b, where[k][0], where[k][1], []uint16{ut(bwgame.UnitTypesZergZergling)})
			deaths := g.fight(actor, target, b, sec, sec+rushDamageWindow)
			e.Outcome = g.outcome(actor, deaths)
			e.SnapshotSecond = min(peakSecond(deaths, sec), sec+peakLate)
			e.Region = fightRegion(deaths, e.Region, e.X, e.Y)
			out = append(out, e)
		}
	}
	return out
}

// buildingRushes fires for a Photon Cannon by 4:00 or a Bunker by 5:00 really started in or next to an
// enemy main or natural; a Bunker only counts with Marines of the rusher beside it.
func (g *game) buildingRushes() []Event {
	var out []Event
	done := map[[2]int64]bool{}
	for _, pr := range g.d.Productions {
		var kind string
		var by int
		switch bwgame.UnitTypes(pr.Type) {
		case bwgame.UnitTypesProtossPhotonCannon:
			kind, by = "cannon_rush", cannonRushBy
		case bwgame.UnitTypesTerranBunker:
			kind, by = "bunker_rush", bunkerRushBy
		default:
			continue
		}
		sec := second(pr.Start)
		if sec > by || done[[2]int64{int64(pr.Owner), int64(pr.Type)}] {
			continue
		}
		b, target := g.enemyHomeBase(pr.Owner, pr.X, pr.Y, sec)
		if b == NoBase {
			b, target = g.nearEnemyHome(pr.Owner, pr.X, pr.Y, sec)
		}
		if b == NoBase {
			continue
		}
		units := []uint16{pr.Type}
		if kind == "bunker_rush" {
			if !g.unitNear(pr.Owner, ut(bwgame.UnitTypesTerranMarine), pr.X, pr.Y, sec, sec+60, bunkerMarineReach) {
				continue
			}
			units = append(units, ut(bwgame.UnitTypesTerranMarine))
		}
		done[[2]int64{int64(pr.Owner), int64(pr.Type)}] = true
		e := g.event(kind, sec, pr.Owner, target, b, pr.X, pr.Y, units).shownAt(sec + placedLead)
		e.Region = around(pr.X, pr.Y, buildingFocus+4*tile)
		out = append(out, e)
	}
	return out
}

func (g *game) lingsCommitted(actor, target int8, base, arrival int) bool {
	kills := 0
	for _, x := range g.deaths {
		sec := second(x.Frame)
		if sec < arrival || sec > arrival+rushDamageWindow || x.Owner != target || x.KillerOwner != actor || g.baseOf(x.X, x.Y) != base {
			continue
		}
		if _, building := FootprintOf(x.Type); building {
			return true
		}
		kills++
	}
	if kills >= rushMinKills {
		return true
	}
	stayFrom := -1
	for _, snap := range g.d.Army {
		sec := second(snap.Frame)
		if sec < arrival {
			continue
		}
		if sec > arrival+rushDamageWindow {
			break
		}
		n := 0
		for _, u := range snap.Units {
			if u.Owner == actor && u.Type == ut(bwgame.UnitTypesZergZergling) && g.baseOf(u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels) == base {
				n++
			}
		}
		switch {
		case n < rushStayLings:
			stayFrom = -1
		case stayFrom < 0:
			stayFrom = sec
		case sec-stayFrom >= rushStaySeconds:
			return true
		}
	}
	return false
}

func (g *game) nearEnemyHome(actor int8, x, y int32, sec int) (int, int8) {
	for i, b := range g.bases {
		if b.Kind == BaseExpansion || dist2(b.X, b.Y, x, y) > int64(rushBuildingReach+baseRadius)*int64(rushBuildingReach+baseRadius) {
			continue
		}
		if owner := g.holder(i, sec); g.enemies(actor, owner) {
			return i, owner
		}
	}
	return NoBase, NoPlayer
}

func (g *game) unitNear(owner int8, t uint16, x, y int32, from, to int, reach int32) bool {
	for _, snap := range g.d.Army {
		sec := second(snap.Frame)
		if sec < from {
			continue
		}
		if sec > to {
			return false
		}
		for _, u := range snap.Units {
			if u.Owner == owner && u.Type == t && dist2(u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels, x, y) <= int64(reach)*int64(reach) {
				return true
			}
		}
	}
	return false
}

func (g *game) humans() []Player {
	var out []Player
	for _, p := range g.players {
		if p.Human {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slot < out[j].Slot })
	return out
}

// proxies keeps the command-inferred rule, two-player games only: a production building placed outside
// its owner's main and natural, at least 0.7 of the half way from its owner's main and at most 1.3 of
// it from the enemy's; but only buildings the engine really started count.
func (g *game) proxies() []Event {
	hs := g.humans()
	if len(hs) != 2 {
		return nil
	}
	kinds := map[bwgame.UnitTypes]struct {
		kind string
		by   int
	}{
		bwgame.UnitTypesProtossGateway: {"proxy_gate", proxyEarlyBy}, bwgame.UnitTypesTerranBarracks: {"proxy_rax", proxyEarlyBy},
		bwgame.UnitTypesTerranFactory: {"proxy_factory", proxyLateBy}, bwgame.UnitTypesTerranStarport: {"proxy_starport", proxyLateBy},
	}
	type proxyKey struct {
		owner int8
		kind  string
	}
	var out []Event
	done := map[proxyKey]bool{}
	for _, pr := range g.d.Productions {
		k, ok := kinds[bwgame.UnitTypes(pr.Type)]
		sec := second(pr.Start)
		if !ok || sec > k.by || done[proxyKey{pr.Owner, k.kind}] {
			continue
		}
		enemy := hs[0].Slot
		if enemy == pr.Owner {
			enemy = hs[1].Slot
		}
		own, theirs := g.mains[pr.Owner], g.mains[enemy]
		if own == theirs {
			continue
		}
		b := g.baseOf(pr.X, pr.Y)
		if b == own || b != NoBase && g.bases[b].Kind == BaseNatural && g.naturalOf(b) == own {
			continue
		}
		half := math.Sqrt(float64(dist2(g.bases[own].X, g.bases[own].Y, g.bases[theirs].X, g.bases[theirs].Y))) / 2
		fromOwn := math.Sqrt(float64(dist2(g.bases[own].X, g.bases[own].Y, pr.X, pr.Y)))
		toEnemy := math.Sqrt(float64(dist2(g.bases[theirs].X, g.bases[theirs].Y, pr.X, pr.Y)))
		if fromOwn < 0.7*half || toEnemy > 1.3*half {
			continue
		}
		done[proxyKey{pr.Owner, k.kind}] = true
		e := g.event(k.kind, sec, pr.Owner, NoPlayer, b, pr.X, pr.Y, []uint16{pr.Type}).shownAt(sec + placedLead + 4)
		e.Region = around(pr.X, pr.Y, buildingFocus)
		out = append(out, e)
	}
	return out
}

// naturalOf is the start base whose centre is nearest to a natural.
func (g *game) naturalOf(natural int) int {
	best, bestDist := NoBase, int64(math.MaxInt64)
	for i, b := range g.bases {
		if b.Kind != BaseStart {
			continue
		}
		if d := dist2(b.X, b.Y, g.bases[natural].X, g.bases[natural].Y); d < bestDist {
			best, bestDist = i, d
		}
	}
	return best
}

func (g *game) mannerPylons() []Event {
	hs := g.humans()
	if len(hs) != 2 {
		return nil
	}
	var out []Event
	done := map[int8]bool{}
	for _, pr := range g.d.Productions {
		sec := second(pr.Start)
		if pr.Type != ut(bwgame.UnitTypesProtossPylon) || sec > mannerPylonBy || done[pr.Owner] {
			continue
		}
		b := g.baseOf(pr.X, pr.Y)
		for _, enemy := range hs {
			if enemy.Slot == pr.Owner || g.mains[enemy.Slot] != b || enemy.Race == openbw.RaceZerg {
				continue
			}
			done[pr.Owner] = true
			e := g.event("manner_pylon", sec, pr.Owner, enemy.Slot, b, pr.X, pr.Y, []uint16{pr.Type}).shownAt(sec + placedLead + 4)
			e.Region = around(pr.X, pr.Y, buildingFocus)
			out = append(out, e)
		}
	}
	return out
}

// firstTech marks a Protoss player's first Reaver (against Protoss or Terran) or first Corsair (against
// Zerg) the moment it is out, and the start of Zealot speed research (against Zerg), all before 10:00.
func (g *game) firstTech() []Event {
	hs := g.humans()
	if len(hs) != 2 {
		return nil
	}
	var out []Event
	for _, p := range hs {
		if p.Race != openbw.RaceProtoss {
			continue
		}
		enemy := hs[0]
		if enemy.Slot == p.Slot {
			enemy = hs[1]
		}
		first := func(kind string, t bwgame.UnitTypes) {
			for _, pr := range g.d.Productions {
				if pr.Owner == p.Slot && pr.Type == ut(t) && pr.Complete != engine.NoFrame && second(pr.Complete) < firstTechBefore {
					out = append(out, g.event(kind, second(pr.Complete), p.Slot, NoPlayer, NoBase, pr.X, pr.Y, []uint16{pr.Type}).shownAt(second(pr.Complete)+arrivalLead))
					return
				}
			}
		}
		if enemy.Race == openbw.RaceZerg {
			first("first_corsair", bwgame.UnitTypesProtossCorsair)
			for _, r := range g.d.Research {
				if r.Owner == p.Slot && r.Upgrade && r.ID == uint8(bwgame.UpgradeTypesLegEnhancements) && second(r.Start) < firstTechBefore {
					out = append(out, g.event("speedlot", second(r.Start), p.Slot, NoPlayer, NoBase, 0, 0, []uint16{ut(bwgame.UnitTypesProtossZealot)}))
					break
				}
			}
		} else {
			first("first_reaver", bwgame.UnitTypesProtossReaver)
		}
	}
	return out
}

// becameRace marks a Protoss player's first building of another race, raised with a mind-controlled worker.
func (g *game) becameRace() []Event {
	var out []Event
	done := map[[2]int8]bool{}
	for _, pr := range g.d.Productions {
		p, ok := g.players[pr.Owner]
		if !ok || p.Race != openbw.RaceProtoss {
			continue
		}
		if _, building := FootprintOf(pr.Type); !building {
			continue
		}
		var kind string
		var race int8
		switch {
		case pr.Type >= ut(bwgame.UnitTypesTerranCommandCenter) && pr.Type <= ut(bwgame.UnitTypesTerranBunker):
			kind, race = "became_terran", 1
		case pr.Type >= ut(bwgame.UnitTypesZergInfestedCommandCenter) && pr.Type <= ut(bwgame.UnitTypesZergExtractor):
			kind, race = "became_zerg", 0
		default:
			continue
		}
		if done[[2]int8{pr.Owner, race}] {
			continue
		}
		done[[2]int8{pr.Owner, race}] = true
		out = append(out, g.event(kind, second(pr.Start), pr.Owner, NoPlayer, g.baseOf(pr.X, pr.Y), pr.X, pr.Y, []uint16{pr.Type}).shownAt(second(pr.Start)+placedLead))
	}
	return out
}

var dangerousCargo = map[bwgame.UnitTypes]bool{
	bwgame.UnitTypesProtossReaver: true, bwgame.UnitTypesProtossDarkTemplar: true, bwgame.UnitTypesProtossHighTemplar: true,
	bwgame.UnitTypesProtossArchon: true, bwgame.UnitTypesTerranSiegeTankTankMode: true, bwgame.UnitTypesZergLurker: true,
	bwgame.UnitTypesZergDefiler: true, bwgame.UnitTypesTerranGhost: true,
}

// drops are unloads into a base an enemy holds. A drop is kept when it carried something dangerous (a
// Reaver, Dark Templar, High Templar, tank, Lurker), 4 or more units, or killed something within 30 s.
// dropKind names a drop after the cargo that decides it: Reavers, else Dark Templar, else a plain drop.
func dropKind(cargo []uint16) (string, uint16) {
	for _, k := range []struct {
		kind string
		unit bwgame.UnitTypes
	}{{"reaver_drop", bwgame.UnitTypesProtossReaver}, {"dt_drop", bwgame.UnitTypesProtossDarkTemplar}} {
		if slices.Contains(cargo, ut(k.unit)) {
			return k.kind, ut(k.unit)
		}
	}
	return "drop", 0
}

// dropUnits lists the transport, then the unit the drop is named after, then the rest by count.
func dropUnits(transport, named uint16, cargo []uint16) []uint16 {
	units := []uint16{transport}
	if named != 0 {
		units = append(units, named)
	}
	for _, t := range byCount(cargo) {
		if t != named || named == 0 {
			units = append(units, t)
		}
	}
	return units
}

func (g *game) drops() []Event {
	type cluster struct {
		actor, target int8
		base          int
		first, last   int
		x, y          int32
		cargo         []uint16
		transport     uint16
	}
	var clusters []*cluster
	for _, u := range g.d.Unloads {
		sec := second(u.Frame)
		b := g.baseOf(u.X, u.Y)
		target := g.holder(b, sec)
		if !g.enemies(u.Owner, target) {
			continue
		}
		var combat []uint16
		for _, t := range u.Cargo {
			if !isWorker(t) {
				combat = append(combat, t)
			}
		}
		if len(combat) == 0 {
			continue
		}
		var c *cluster
		for _, open := range clusters {
			if open.actor == u.Owner && open.base == b && sec-open.last <= dropGapSeconds {
				c = open
			}
		}
		if c == nil {
			c = &cluster{actor: u.Owner, target: target, base: b, first: sec, x: u.X, y: u.Y, transport: u.TransportType}
			clusters = append(clusters, c)
		}
		c.last = sec
		c.cargo = append(c.cargo, combat...)
	}
	var out []Event
	for _, c := range clusters {
		dangerous := slices.ContainsFunc(c.cargo, func(t uint16) bool { return dangerousCargo[bwgame.UnitTypes(t)] })
		if !dangerous && len(c.cargo) < dropMinCargo && g.killsAt(c.actor, c.target, c.base, c.first, c.last+dropFollowUp) == 0 {
			continue
		}
		kind, named := dropKind(c.cargo)
		e := g.event(kind, c.first, c.actor, c.target, c.base, c.x, c.y, dropUnits(c.transport, named, c.cargo)).shownAt(min(c.last, c.first+dropUnloadSeconds) + arrivalLead)
		deaths := g.fight(c.actor, c.target, c.base, c.first, c.last+dropFollowUp)
		e.Outcome = g.outcome(c.actor, deaths)
		e.Region = fightRegion(deaths, around(c.x, c.y, buildingFocus+2*tile), c.x, c.y)
		out = append(out, e)
	}
	return out
}

func isWorker(t uint16) bool {
	switch bwgame.UnitTypes(t) {
	case bwgame.UnitTypesTerranSCV, bwgame.UnitTypesProtossProbe, bwgame.UnitTypesZergDrone:
		return true
	}
	return false
}

// byCount lists unit types most numerous first, each once.
func byCount(types []uint16) []uint16 {
	counts := map[uint16]int{}
	for _, t := range types {
		counts[t]++
	}
	var out []uint16
	for t := range counts {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if counts[out[i]] != counts[out[j]] {
			return counts[out[i]] > counts[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// killsAt is the value the actor killed of the target's at a base (or within its radius) in a window.
func (g *game) killsAt(actor, target int8, base int, from, to int) int32 {
	var value int32
	for _, x := range g.deaths {
		sec := second(x.Frame)
		if sec < from || sec > to || x.KillerOwner != actor || x.Owner != target {
			continue
		}
		if g.baseOf(x.X, x.Y) == base {
			value += max(CostOf(x.Type).Value(), 1)
		}
	}
	return value
}

// teleports turns units crossing the map in a second into recalls (Protoss, with an accepted Recall cast
// near the arrival) and nydus attacks (Zerg arriving in a base an enemy holds).
func (g *game) teleports() []Event {
	var out []Event
	lastNydus := map[[2]int64]int{}
	for _, t := range g.d.Teleports {
		if len(t.Units) < teleportMinUnits {
			continue
		}
		sec := second(t.Frame)
		p := g.players[t.Owner]
		to := g.baseOf(t.ToX, t.ToY)
		target := g.holder(to, sec)
		switch p.Race {
		case openbw.RaceProtoss:
			if !g.recallCast(t.Owner, sec, t.ToX, t.ToY) {
				continue
			}
			if !g.enemies(t.Owner, target) {
				target = NoPlayer
			}
			e := g.event("recall", sec, t.Owner, target, to, t.ToX, t.ToY, append([]uint16{ut(bwgame.UnitTypesProtossArbiter)}, byCount(t.Units)...)).shownAt(sec + arrivalLead)
			e.SourceBase = g.baseOf(t.FromX, t.FromY)
			out = append(out, e)
		case openbw.RaceZerg:
			if !g.enemies(t.Owner, target) {
				continue
			}
			k := [2]int64{int64(t.Owner), int64(to)}
			if last, ok := lastNydus[k]; ok && sec-last < nydusRepeatAfter {
				continue
			}
			lastNydus[k] = sec
			out = append(out, g.event("nydus_attack", sec, t.Owner, target, to, t.ToX, t.ToY, byCount(t.Units)).shownAt(sec+arrivalLead))
		}
	}
	return out
}

func (g *game) recallCast(owner int8, sec int, x, y int32) bool {
	for _, c := range g.d.Casts {
		cs := second(c.Frame)
		if c.Owner == owner && c.Order == orderRecall && cs <= sec && sec-cs <= recallCastBefore && dist2(c.X, c.Y, x, y) <= int64(recallCastReach)*int64(recallCastReach) {
			return true
		}
	}
	return false
}

const (
	orderRecall    = 0x89
	orderNukePaint = 0x7e
)

func (g *game) nukes() []Event {
	var out []Event
	for _, c := range g.d.Casts {
		if c.Order != orderNukePaint {
			continue
		}
		sec := second(c.Frame)
		b := g.baseOf(c.X, c.Y)
		target := g.holder(b, sec)
		if !g.enemies(c.Owner, target) {
			continue
		}
		out = append(out, g.event("nuke", sec, c.Owner, target, b, c.X, c.Y, []uint16{ut(bwgame.UnitTypesTerranGhost)}))
	}
	return out
}

func combatUnit(t uint16) bool {
	switch bwgame.UnitTypes(t) {
	case bwgame.UnitTypesTerranSCV, bwgame.UnitTypesProtossProbe, bwgame.UnitTypesZergDrone, bwgame.UnitTypesZergOverlord,
		bwgame.UnitTypesProtossObserver, bwgame.UnitTypesTerranDropship, bwgame.UnitTypesProtossShuttle, bwgame.UnitTypesTerranScienceVessel,
		bwgame.UnitTypesProtossInterceptor, bwgame.UnitTypesProtossScarab, bwgame.UnitTypesTerranVultureSpiderMine:
		return false
	}
	return true
}

type attackWindow struct {
	actor, target int8
	base          int
	first, last   int
	x, y          int32
	units         []uint16
}

// attacks finds armies of 4+ combat units standing in a base the enemy holds, merged while they stay
// (gaps of 30 s or less), and keeps one only if blood was drawn there (300 resources or 3 deaths on either
// side) and the moment matters: the attacker's first attack on that player, a unit type they had not
// attacked with yet, a decisive trade (1000 resources killed), a town hall killed, or the defender
// leaving within a minute. Decisive battles in the open field (3000 resources traded) count too.
// A rush, drop, recall or nydus at that base during the window already tells that moment.
func (g *game) attacks(special []Event) []Event {
	var windows []*attackWindow
	for _, snap := range g.d.Army {
		sec := second(snap.Frame)
		groups := map[[2]int64]*attackWindow{}
		var order [][2]int64
		for _, u := range snap.Units {
			if !combatUnit(u.Type) {
				continue
			}
			x, y := u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels
			b := g.baseOf(x, y)
			target := g.holder(b, sec)
			if !g.enemies(u.Owner, target) {
				continue
			}
			k := [2]int64{int64(u.Owner), int64(b)}
			w, ok := groups[k]
			if !ok {
				w = &attackWindow{actor: u.Owner, target: target, base: b, first: sec, last: sec, x: x, y: y}
				groups[k] = w
				order = append(order, k)
			}
			w.units = append(w.units, u.Type)
		}
		for _, k := range order {
			w := groups[k]
			if len(w.units) < attackMinUnits {
				continue
			}
			var open *attackWindow
			for _, prev := range windows {
				if prev.actor == w.actor && prev.base == w.base && sec-prev.last <= attackGapSeconds {
					open = prev
				}
			}
			if open == nil {
				windows = append(windows, w)
				continue
			}
			open.last = sec
			open.units = append(open.units, w.units...)
		}
	}

	var out []Event
	attacked := map[[2]int8]bool{}
	seenTypes := map[int8]map[uint16]bool{}
	lastKept := map[int8]int{}
	for _, w := range windows {
		if g.toldBySpecial(special, w) {
			continue
		}
		if last, ok := lastKept[w.actor]; ok && w.first-last < attackDedupSeconds {
			continue
		}
		killed := g.killsAt(w.actor, w.target, w.base, w.first, w.last+attackGapSeconds)
		lost := g.killsAt(w.target, w.actor, w.base, w.first, w.last+attackGapSeconds)
		deaths := g.deathsAt(w.actor, w.target, w.base, w.first, w.last+attackGapSeconds)
		if killed+lost < attackMinTrade && deaths < attackMinDeaths {
			continue
		}
		types := byCount(w.units)
		newType := false
		for _, t := range types {
			if !seenTypes[w.actor][t] {
				newType = true
			}
		}
		deathList := g.fight(w.actor, w.target, w.base, w.first, w.last+attackGapSeconds)
		outcome := g.outcome(w.actor, deathList)
		outcome.First = !attacked[[2]int8{w.actor, w.target}]
		relevant := outcome.First || newType || killed >= decisiveKilled || outcome.TownHall || outcome.Workers >= workerKillsMin ||
			g.leftWithin(w.target, w.last, leaveAfterAttack)
		if !relevant {
			continue
		}
		attacked[[2]int8{w.actor, w.target}] = true
		lastKept[w.actor] = w.first
		if seenTypes[w.actor] == nil {
			seenTypes[w.actor] = map[uint16]bool{}
		}
		for _, t := range types {
			seenTypes[w.actor][t] = true
		}
		e := g.event("attack", w.first, w.actor, w.target, w.base, w.x, w.y, types)
		e.Outcome = outcome
		e.SnapshotSecond = peakSecond(deathList, w.first)
		e.Region = fightRegion(deathList, e.Region, w.x, w.y)
		// A long stay with the fighting late is told when the fighting happens, so the time read matches the map shown.
		if e.SnapshotSecond-e.Second > peakLate {
			e.Second = e.SnapshotSecond - peakLead
		}
		out = append(out, e)
	}
	for _, e := range g.fieldBattles() {
		if last, ok := lastKept[e.Actor]; ok && abs(e.Second-last) < attackDedupSeconds {
			continue
		}
		lastKept[e.Actor] = e.Second
		out = append(out, e)
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (g *game) toldBySpecial(special []Event, w *attackWindow) bool {
	for _, e := range special {
		if e.Actor == w.actor && e.Base == w.base && e.Second >= w.first-attackOverlapSeconds && e.Second <= w.last {
			return true
		}
	}
	return false
}

func (g *game) deathsAt(a, b int8, base int, from, to int) int {
	n := 0
	for _, x := range g.deaths {
		sec := second(x.Frame)
		if sec < from || sec > to || g.baseOf(x.X, x.Y) != base {
			continue
		}
		if x.Owner == a && x.KillerOwner == b || x.Owner == b && x.KillerOwner == a {
			n++
		}
	}
	return n
}

func (g *game) leftWithin(player int8, after, within int) bool {
	for _, o := range g.d.Outcomes {
		if o.Owner == player && o.Kind != engine.OutcomeWon && second(o.Frame) >= after && second(o.Frame) <= after+within {
			return true
		}
	}
	return false
}

// fieldBattles are decisive fights away from any base: 3000 resources traded within 14 tiles, deaths no
// more than 30 s apart. The side that killed more is the attacker.
func (g *game) fieldBattles() []Event {
	type battle struct {
		deaths     []engine.Death
		sumX, sumY int64
		last       int
	}
	var battles []*battle
	for _, x := range g.deaths {
		// A town hall's death ends its owner's hold that very second, so the hold is read a second before.
		if b := g.baseOf(x.X, x.Y); !g.enemies(x.Owner, x.KillerOwner) || g.holder(b, second(x.Frame)) != NoPlayer || g.holder(b, second(x.Frame)-1) != NoPlayer {
			continue
		}
		sec := second(x.Frame)
		var joined *battle
		for _, b := range battles {
			n := int64(len(b.deaths))
			if sec-b.last <= attackGapSeconds && dist2(int32(b.sumX/n), int32(b.sumY/n), x.X, x.Y) <= fieldBattleRadius*fieldBattleRadius {
				joined = b
			}
		}
		if joined == nil {
			joined = &battle{}
			battles = append(battles, joined)
		}
		joined.deaths = append(joined.deaths, x)
		joined.sumX += int64(x.X)
		joined.sumY += int64(x.Y)
		joined.last = sec
	}
	var out []Event
	for _, b := range battles {
		killedBy := map[int8]int32{}
		killers := map[int8][]uint16{}
		var traded int32
		for _, x := range b.deaths {
			if x.KillerType != uint16(openbw.UnitTypeNone) {
				killers[x.KillerOwner] = append(killers[x.KillerOwner], fighter(x.KillerType))
			}
			v := CostOf(x.Type).Value()
			killedBy[x.KillerOwner] += v
			traded += v
		}
		if traded < fieldBattleTraded {
			continue
		}
		winner, loser := NoPlayer, NoPlayer
		for p, v := range killedBy {
			if winner == NoPlayer || v > killedBy[winner] || v == killedBy[winner] && p < winner {
				winner = p
			}
		}
		for _, x := range b.deaths {
			if x.Owner != winner {
				loser = x.Owner
				break
			}
		}
		n := int64(len(b.deaths))
		x, y := int32(b.sumX/n), int32(b.sumY/n)
		sec := second(b.deaths[0].Frame)
		e := g.event("attack", sec, winner, loser, NoBase, x, y, byCount(killers[winner]))
		e.Outcome = g.outcome(winner, b.deaths)
		e.SnapshotSecond = peakSecond(b.deaths, sec)
		e.Region = fightRegion(b.deaths, around(x, y, fieldBattleRadius/2), x, y)
		out = append(out, e)
	}
	return out
}

// fight is every death between two players at a base (or, for NoBase, anywhere) in a window.
func (g *game) fight(a, b int8, base int, from, to int) []engine.Death {
	var out []engine.Death
	for _, x := range g.deaths {
		sec := second(x.Frame)
		if sec < from || sec > to || base != NoBase && g.baseOf(x.X, x.Y) != base {
			continue
		}
		if x.Owner == a && x.KillerOwner == b || x.Owner == b && x.KillerOwner == a {
			out = append(out, x)
		}
	}
	return out
}

func (g *game) outcome(actor int8, deaths []engine.Death) Outcome {
	var o Outcome
	for _, x := range deaths {
		v := max(CostOf(x.Type).Value(), 1)
		if x.Owner == actor {
			o.Lost += v
			continue
		}
		o.Killed += v
		_, building := FootprintOf(x.Type)
		switch {
		case isWorker(x.Type):
			o.Workers++
		case IsTownHall(x.Type):
			o.TownHall = true
			o.Buildings++
		case building:
			o.Buildings++
		}
	}
	return o
}

const (
	// peakWindow is the stretch of a fight peakSecond looks for the most deaths in.
	peakWindow = 8
	peakLate   = 30
	peakLead   = 5
)

// peakSecond is the moment to show a fight: early in its fiercest 8 s, while both armies still stand.
func peakSecond(deaths []engine.Death, fallback int) int {
	if len(deaths) == 0 {
		return fallback
	}
	secs := make([]int, len(deaths))
	for i, x := range deaths {
		secs[i] = second(x.Frame)
	}
	sort.Ints(secs)
	best, bestN := secs[0], 0
	for i, from := range secs {
		n := sort.SearchInts(secs[i:], from+peakWindow+1)
		if n > bestN {
			best, bestN = from, n
		}
	}
	return max(fallback, best-1)
}

// fightRegion frames where units died, falling back to the given region when too few did to tell.
func fightRegion(deaths []engine.Death, fallback Region, x, y int32) Region {
	if len(deaths) < 2 {
		return fallback
	}
	minX, minY, maxX, maxY := x, y, x, y
	for _, d := range deaths {
		minX, minY, maxX, maxY = min(minX, d.X), min(minY, d.Y), max(maxX, d.X), max(maxY, d.Y)
	}
	return regionAround(minX, minY, maxX, maxY)
}

const (
	workerKillsMin = 5
	workerKillsGap = 20
)

// workerKills finds a player killing 5 or more of an enemy's workers at one base with no attack or
// drop already telling it: the Mutalisks, Vultures, Reavers or Storms that cost a player their economy.
func (g *game) workerKills(told []Event) []Event {
	type cluster struct {
		actor, target int8
		base          int
		first, last   int
		deaths        []engine.Death
	}
	var clusters []*cluster
	for _, x := range g.deaths {
		if !isWorker(x.Type) || !g.enemies(x.KillerOwner, x.Owner) {
			continue
		}
		sec := second(x.Frame)
		b := g.baseOf(x.X, x.Y)
		var c *cluster
		for _, open := range clusters {
			if open.actor == x.KillerOwner && open.target == x.Owner && open.base == b && sec-open.last <= workerKillsGap {
				c = open
			}
		}
		if c == nil {
			c = &cluster{actor: x.KillerOwner, target: x.Owner, base: b, first: sec}
			clusters = append(clusters, c)
		}
		c.last = sec
		c.deaths = append(c.deaths, x)
	}
	var out []Event
	for _, c := range clusters {
		if len(c.deaths) < workerKillsMin || c.base == NoBase {
			continue
		}
		if slices.ContainsFunc(told, func(e Event) bool {
			return e.Actor == c.actor && e.Base == c.base && e.Second <= c.last && e.Second >= c.first-attackGapSeconds
		}) {
			continue
		}
		var killers []uint16
		for _, x := range c.deaths {
			if x.KillerType != uint16(openbw.UnitTypeNone) {
				killers = append(killers, fighter(x.KillerType))
			}
		}
		x, y := c.deaths[0].X, c.deaths[0].Y
		e := g.event("worker_kills", c.first, c.actor, c.target, c.base, x, y, byCount(killers))
		e.Outcome = g.outcome(c.actor, g.fight(c.actor, c.target, c.base, c.first, c.last))
		e.SnapshotSecond = peakSecond(c.deaths, c.first)
		e.Region = fightRegion(c.deaths, e.Region, x, y)
		out = append(out, e)
	}
	return out
}

const (
	longSupplyBlock = 45
	supplyBlockCash = 100
	supplyCap       = 400
)

// supplyBlocks finds a player at their supply cap with money to spend for 45 s or more: long enough
// to cost the game, not the ordinary seconds before a depot finishes.
func (g *game) supplyBlocks() []Event {
	var out []Event
	from := map[int8]int{}
	flush := func(owner int8, until int) {
		start, open := from[owner]
		delete(from, owner)
		if !open || until-start < longSupplyBlock {
			return
		}
		main, ok := g.mains[owner]
		if !ok {
			return
		}
		b := g.bases[main]
		e := g.event("long_supply_block", start, owner, NoPlayer, main, b.X, b.Y, nil).shownAt((start + until) / 2)
		e.Seconds = until - start
		e.Region = around(b.X, b.Y, buildingFocus+3*tile)
		out = append(out, e)
	}
	for _, s := range g.d.Economy {
		if !g.players[s.Owner].Human {
			continue
		}
		sec := second(s.Frame)
		blocked := s.SupplyMax > 0 && s.SupplyMax < supplyCap && s.SupplyUsed >= s.SupplyMax && s.Minerals >= supplyBlockCash
		_, open := from[s.Owner]
		switch {
		case blocked && !open:
			from[s.Owner] = sec
		case !blocked && open:
			flush(s.Owner, sec)
		}
	}
	for owner := range from {
		flush(owner, second(g.d.LastFrame))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Second < out[j].Second })
	return out
}

// fighter is the unit a kill is credited to: an Interceptor's kill is its Carrier's, a Scarab's its Reaver's.
func fighter(t uint16) uint16 {
	switch bwgame.UnitTypes(t) {
	case bwgame.UnitTypesProtossInterceptor:
		return ut(bwgame.UnitTypesProtossCarrier)
	case bwgame.UnitTypesProtossScarab:
		return ut(bwgame.UnitTypesProtossReaver)
	}
	return t
}
