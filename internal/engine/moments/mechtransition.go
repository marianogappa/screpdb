package moments

import (
	"math"
	"sort"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// A mech transition is a TvZ Terran who played bio, then stopped making bio for good and built Factories
// to make mech. Thresholds come from 1,772 ladder TvZ Terrans: 106 qualify and 92% of them lift two or more
// Barracks around their last bio unit, which these rules never look at.
const (
	// The last bio unit before Barracks fall silent comes after 6:00, with at least 20 bio units in the five
	// minutes before it, outnumbering mech units two to one (one to one in supply), and 30 bio units by then.
	mechSwitchFrom      = 360
	mechBioWindow       = 300
	mechBioBefore       = 20
	mechBioTotal        = 30
	mechBioToMechRatio  = 2
	mechQuietTolerance  = 2
	mechQuietShare      = 0.15
	mechAfterAtLeast    = 120
	mechFactoriesBefore = 240
	mechFactoriesAfter  = 120
	// A transition is a burst of new Factories that outnumbers the ones already standing, typically one:
	// a Terran with three Factories who adds a fourth is not switching.
	mechNewFactories     = 2
	mechFactoriesTotal   = 3
	mechUnitsPer5Minutes = 10
	mechUnitsAtLeast     = 4
	// The event is when the last bio unit came out. The map shows the switch once the new Factories are up,
	// at least a minute on, when the Barracks have lifted, and at most two and a half.
	mechShownAtLeast = 60
	mechShownAtMost  = 150
	mechRunGap       = 60
)

var (
	bioTypes  = map[uint16]bool{ut(bwgame.UnitTypesTerranMarine): true, ut(bwgame.UnitTypesTerranFirebat): true, ut(bwgame.UnitTypesTerranMedic): true}
	mechTypes = map[uint16]bool{ut(bwgame.UnitTypesTerranVulture): true, ut(bwgame.UnitTypesTerranGoliath): true, ut(bwgame.UnitTypesTerranSiegeTankTankMode): true}
)

type mechStart struct {
	sec  int
	unit uint16
}

// mechTransition finds the Terran's switch in a 1v1 TvZ. The rules read when bio units were started; the event
// is when the last one came out. Count is the Factories standing when the map shows it; Units are the Factory,
// then the mech made in the five minutes after the last bio unit, most first, so the map highlights both.
func (g *game) mechTransition() []Event {
	hs := g.humans()
	if len(hs) != 2 || hs[0].Race == hs[1].Race {
		return nil
	}
	terran, zerg := hs[0], hs[1]
	if terran.Race != openbw.RaceTerran {
		terran, zerg = zerg, terran
	}
	if terran.Race != openbw.RaceTerran || zerg.Race != openbw.RaceZerg {
		return nil
	}
	var bio []int
	out := map[int]int{}
	var mech []mechStart
	var factories []engine.Production
	for _, pr := range g.d.Productions {
		if pr.Owner != terran.Slot || pr.Cancel != engine.NoFrame || second(pr.Start) >= g.trusted {
			continue
		}
		switch {
		case bioTypes[pr.Type]:
			start := second(pr.Start)
			bio = append(bio, start)
			if pr.Complete != engine.NoFrame {
				out[start] = max(out[start], second(pr.Complete))
			}
		case mechTypes[pr.Type]:
			mech = append(mech, mechStart{second(pr.Start), pr.Type})
		case pr.Type == ut(bwgame.UnitTypesTerranFactory):
			factories = append(factories, pr)
		}
	}
	sort.Ints(bio)
	sort.Slice(mech, func(i, j int) bool { return mech[i].sec < mech[j].sec })
	sort.Slice(factories, func(i, j int) bool { return factories[i].Start < factories[j].Start })
	mechBetween := func(from, to int) (n int) {
		for _, m := range mech {
			if m.sec > from && m.sec <= to {
				n++
			}
		}
		return n
	}
	for i, last := range bio {
		if last < mechSwitchFrom {
			continue
		}
		after := min(mechBioWindow, g.trusted-1-last)
		if after < mechAfterAtLeast {
			return nil
		}
		before := countBetween(bio, last-mechBioWindow, last)
		if before < mechBioBefore || countBetween(bio, -1, last) < mechBioTotal || before < mechBioToMechRatio*mechBetween(last-mechBioWindow, last) {
			continue
		}
		resumed := countBetween(bio, last, last+after)
		if float64(resumed) > math.Max(mechQuietTolerance, mechQuietShare*float64(before)*float64(after)/mechBioWindow) {
			continue
		}
		var burst []engine.Production
		already := 0
		for _, f := range factories {
			sec := second(f.Start)
			switch {
			case sec > last+mechFactoriesAfter:
			case sec > last-mechFactoriesBefore:
				burst = append(burst, f)
			case g.aliveAt(f.Unit, last-mechFactoriesBefore):
				already++
			}
		}
		if len(burst) <= already || len(burst) < mechNewFactories || already+len(burst) < mechFactoriesTotal {
			continue
		}
		made := mechBetween(last, last+after)
		if float64(made) < math.Max(mechUnitsAtLeast, mechUnitsPer5Minutes*float64(after)/mechBioWindow) {
			continue
		}
		end := runEnd(bio, i)
		lastOut := end
		for _, start := range bio[:sort.SearchInts(bio, end+1)] {
			lastOut = max(lastOut, out[start])
		}
		return []Event{g.mechEvent(terran.Slot, zerg.Slot, end, min(lastOut, g.trusted-1), burst, factories, mech, after)}
	}
	return nil
}

func (g *game) mechEvent(terran, zerg int8, last, lastOut int, burst, factories []engine.Production, mech []mechStart, after int) Event {
	shown := lastOut + mechShownAtLeast
	for _, f := range burst {
		if f.Complete != engine.NoFrame {
			shown = max(shown, second(f.Complete))
		}
	}
	shown = min(shown, lastOut+mechShownAtMost)
	standing := 0
	for _, f := range factories {
		if second(f.Start) <= shown && g.aliveAt(f.Unit, shown) {
			standing++
		}
	}
	var made []uint16
	for _, m := range mech {
		if m.sec > last && m.sec <= last+after {
			made = append(made, m.unit)
		}
	}
	cluster := factoryCluster(burst)
	minX, minY, maxX, maxY := int32(math.MaxInt32), int32(math.MaxInt32), int32(math.MinInt32), int32(math.MinInt32)
	for _, f := range cluster {
		minX, minY, maxX, maxY = min(minX, f.X), min(minY, f.Y), max(maxX, f.X), max(maxY, f.Y)
	}
	at := cluster[0]
	units := append([]uint16{ut(bwgame.UnitTypesTerranFactory)}, byCount(made)...)
	e := g.event("mech_transition", lastOut, terran, zerg, g.baseOf(at.X, at.Y), at.X, at.Y, units).shownAt(shown)
	e.Region = regionAround(minX, minY, maxX, maxY)
	e.Count = standing
	return e
}

// factoryCluster is the new Factories standing together: those within reach of the one with the most
// neighbours, so a Factory built at another base does not stretch the map to its full height.
func factoryCluster(burst []engine.Production) []engine.Production {
	near := func(a, b engine.Production) bool {
		return dist2(a.X, a.Y, b.X, b.Y) <= int64(baseRadius)*int64(baseRadius)
	}
	var best []engine.Production
	for _, a := range burst {
		var group []engine.Production
		for _, b := range burst {
			if near(a, b) {
				group = append(group, b)
			}
		}
		if len(group) > len(best) {
			best = group
		}
	}
	return best
}

func (g *game) aliveAt(unit uint32, sec int) bool {
	for _, x := range g.d.Deaths {
		if x.Unit == unit {
			return second(x.Frame) > sec
		}
	}
	return true
}

// runEnd is the last bio unit of the production run bio[i] belongs to: the few the quiet tolerance lets
// through right after it are still the Barracks winding down, not a return to bio.
func runEnd(bio []int, i int) int {
	for i+1 < len(bio) && bio[i+1]-bio[i] <= mechRunGap {
		i++
	}
	return bio[i]
}

// countBetween counts the sorted seconds in (from, to].
func countBetween(secs []int, from, to int) int {
	return sort.SearchInts(secs, to+1) - sort.SearchInts(secs, from+1)
}
