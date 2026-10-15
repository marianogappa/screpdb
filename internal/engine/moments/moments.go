// Package moments finds the moments of a game worth looking at from its engine digest: fights with
// their resource trade, expansions, bases lost, long supply blocks and floated money. Each moment has a
// time and a region of the map to snapshot.
package moments

import (
	"slices"
	"sort"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/macro"
)

type Kind string

const (
	KindFight       Kind = "fight"
	KindExpansion   Kind = "expansion"
	KindBaseLost    Kind = "base_lost"
	KindSupplyBlock Kind = "supply_block"
	KindFloat       Kind = "float"
)

const (
	tile            = 32
	framesPerSecond = engine.EconomyEveryFrames
	// Deaths within 20 s and 14 tiles of a fight belong to it: a fight pauses while units regroup, and
	// two fights on one map are rarely that close.
	fightGapSeconds  = 20
	fightRadius      = 14 * tile
	minFightValue    = 400
	minFightDeaths   = 6
	regionMargin     = 8 * tile
	minRegionSide    = 24 * tile
	snapshotLeadSecs = 2
	NoPlayer         = int8(-1)
)

// Region is a rectangle of the map in pixels.
type Region struct{ X, Y, W, H int32 }

type Side struct {
	Owner     int8
	Lost      Cost
	Killed    Cost
	UnitsLost map[uint16]int
}

type Moment struct {
	Kind           Kind
	Second         int
	EndSecond      int
	SnapshotSecond int
	Actor, Target  int8
	Region         *Region
	Sides          []Side
	UnitType       uint16
	X, Y           int32
	Cancelled      bool
}

func second(frame int32) int { return int(frame) / framesPerSecond }

// Find lists the moments up to the digest's trusted frame, in time order.
func Find(d *engine.Digest, players []macro.Player) []Moment {
	trusted := second(d.TrustedUntil())
	var out []Moment
	out = append(out, fights(d, trusted)...)
	out = append(out, expansions(d, trusted)...)
	out = append(out, basesLost(d, trusted)...)
	for _, p := range players {
		for _, e := range p.LongSupplyBlocks {
			out = append(out, Moment{Kind: KindSupplyBlock, Second: e.From, EndSecond: e.To, SnapshotSecond: e.From, Actor: p.Slot, Target: NoPlayer})
		}
		for _, e := range p.Floats {
			out = append(out, Moment{Kind: KindFloat, Second: e.From, EndSecond: e.To, SnapshotSecond: e.From, Actor: p.Slot, Target: NoPlayer})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Second < out[j].Second })
	return out
}

func isPlayer(d *engine.Digest, owner int8) bool {
	return slices.ContainsFunc(d.Players, func(p engine.Player) bool { return p.Slot == owner })
}

type fight struct {
	deaths     []engine.Death
	sumX, sumY int64
	last       int
}

func (f *fight) center() (int32, int32) {
	n := int64(len(f.deaths))
	return int32(f.sumX / n), int32(f.sumY / n)
}

func enemyKill(d *engine.Digest, x engine.Death) bool {
	switch openbw.DeathReason(x.Reason) {
	case openbw.ReasonDamage, openbw.ReasonSpell, openbw.ReasonContainer, openbw.ReasonDetonated:
	default:
		return false
	}
	return x.KillerOwner != x.Owner && isPlayer(d, x.Owner) && isPlayer(d, x.KillerOwner)
}

func fights(d *engine.Digest, trusted int) []Moment {
	var open, closed []*fight
	for _, x := range d.Deaths {
		s := second(x.Frame)
		if s >= trusted {
			break
		}
		if !enemyKill(d, x) {
			continue
		}
		var still []*fight
		for _, f := range open {
			if s-f.last > fightGapSeconds {
				closed = append(closed, f)
			} else {
				still = append(still, f)
			}
		}
		open = still
		var joined *fight
		for _, f := range open {
			cx, cy := f.center()
			if dx, dy := int64(x.X-cx), int64(x.Y-cy); dx*dx+dy*dy <= fightRadius*fightRadius {
				joined = f
				break
			}
		}
		if joined == nil {
			joined = &fight{}
			open = append(open, joined)
		}
		joined.deaths = append(joined.deaths, x)
		joined.sumX += int64(x.X)
		joined.sumY += int64(x.Y)
		joined.last = s
	}
	closed = append(closed, open...)
	var out []Moment
	for _, f := range closed {
		if m, ok := fightMoment(f); ok {
			out = append(out, m)
		}
	}
	return out
}

func fightMoment(f *fight) (Moment, bool) {
	sides := map[int8]*Side{}
	side := func(owner int8) *Side {
		if s, ok := sides[owner]; ok {
			return s
		}
		s := &Side{Owner: owner, UnitsLost: map[uint16]int{}}
		sides[owner] = s
		return s
	}
	var total int32
	minX, minY, maxX, maxY := f.deaths[0].X, f.deaths[0].Y, f.deaths[0].X, f.deaths[0].Y
	for _, x := range f.deaths {
		c := CostOf(x.Type)
		total += c.Value()
		lost := side(x.Owner)
		lost.Lost.Minerals += c.Minerals
		lost.Lost.Gas += c.Gas
		lost.UnitsLost[x.Type]++
		killer := side(x.KillerOwner)
		killer.Killed.Minerals += c.Minerals
		killer.Killed.Gas += c.Gas
		minX, minY, maxX, maxY = min(minX, x.X), min(minY, x.Y), max(maxX, x.X), max(maxY, x.Y)
	}
	if len(f.deaths) < 2 || total < minFightValue && len(f.deaths) < minFightDeaths {
		return Moment{}, false
	}
	m := Moment{
		Kind:           KindFight,
		Second:         second(f.deaths[0].Frame),
		EndSecond:      f.last,
		SnapshotSecond: max(0, second(f.deaths[0].Frame)-snapshotLeadSecs),
		Actor:          NoPlayer,
		Target:         NoPlayer,
		Region:         regionAround(minX, minY, maxX, maxY),
	}
	for _, s := range sides {
		m.Sides = append(m.Sides, *s)
	}
	sort.Slice(m.Sides, func(i, j int) bool { return m.Sides[i].Killed.Value() > m.Sides[j].Killed.Value() })
	m.Actor = m.Sides[0].Owner
	if len(m.Sides) > 1 {
		m.Target = m.Sides[1].Owner
	}
	return m, true
}

func regionAround(minX, minY, maxX, maxY int32) *Region {
	r := Region{X: minX - regionMargin, Y: minY - regionMargin, W: maxX - minX + 2*regionMargin, H: maxY - minY + 2*regionMargin}
	if r.W < minRegionSide {
		r.X -= (minRegionSide - r.W) / 2
		r.W = minRegionSide
	}
	if r.H < minRegionSide {
		r.Y -= (minRegionSide - r.H) / 2
		r.H = minRegionSide
	}
	return &r
}

func expansions(d *engine.Digest, trusted int) []Moment {
	var out []Moment
	for _, pr := range d.Productions {
		s := second(pr.Start)
		if s >= trusted || !IsTownHall(pr.Type) || pr.From != uint16(openbw.UnitTypeNone) && IsTownHall(pr.From) {
			continue
		}
		out = append(out, Moment{
			Kind: KindExpansion, Second: s, EndSecond: s, SnapshotSecond: s, Actor: pr.Owner, Target: NoPlayer,
			UnitType: pr.Type, X: pr.X, Y: pr.Y, Cancelled: pr.Cancel != engine.NoFrame,
			Region: regionAround(pr.X, pr.Y, pr.X, pr.Y),
		})
	}
	return out
}

func basesLost(d *engine.Digest, trusted int) []Moment {
	var out []Moment
	for _, x := range d.Deaths {
		s := second(x.Frame)
		if s >= trusted || !IsTownHall(x.Type) || !enemyKill(d, x) {
			continue
		}
		out = append(out, Moment{
			Kind: KindBaseLost, Second: s, EndSecond: s, SnapshotSecond: max(0, s-snapshotLeadSecs), Actor: x.KillerOwner, Target: x.Owner,
			UnitType: x.Type, X: x.X, Y: x.Y, Region: regionAround(x.X, x.Y, x.X, x.Y),
		})
	}
	return out
}
