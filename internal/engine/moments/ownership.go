package moments

import (
	"math"
	"slices"

	"github.com/marianogappa/screpdb/internal/engine"
)

// Hall is a town hall standing on the map at one moment, as a snapshot has it.
type Hall struct {
	Type     uint16
	Owner    int8
	X, Y     int32
	Complete bool
	Lifted   bool
}

// BaseOwner is who holds a base at one moment. Since is the second their town hall there was started,
// or -1 when the digest cannot tell (a Command Center built elsewhere and flown in).
type BaseOwner struct {
	Base     int
	Owner    int8
	TownHall uint16
	Complete bool
	Since    int
}

// BaseOwners reads who holds each base off the town halls standing at sec: a base belongs to the owner
// of a landed town hall inside it. Reading the halls of the very snapshot the map draws keeps the two in
// agreement, flown Command Centers included. When two players have a hall in one base, a finished hall
// beats one under construction, then the earlier one wins.
func BaseOwners(d *engine.Digest, bases []Base, sec int, halls []Hall) []BaseOwner {
	g := &game{d: d, bases: bases}
	deaths := map[uint32][]int{}
	for _, x := range d.Deaths {
		if IsTownHall(x.Type) {
			deaths[x.Unit] = append(deaths[x.Unit], second(x.Frame))
		}
	}
	best := map[int]BaseOwner{}
	for _, h := range halls {
		if h.Lifted || !IsTownHall(h.Type) {
			continue
		}
		b := g.baseOf(h.X, h.Y)
		if b == NoBase {
			continue
		}
		cand := BaseOwner{Base: b, Owner: h.Owner, TownHall: h.Type, Complete: h.Complete, Since: g.holdingSince(b, h.Owner, sec, deaths)}
		if cur, ok := best[b]; !ok || beats(cand, cur) {
			best[b] = cand
		}
	}
	out := make([]BaseOwner, 0, len(best))
	for _, o := range best {
		out = append(out, o)
	}
	slices.SortFunc(out, func(a, b BaseOwner) int { return a.Base - b.Base })
	return out
}

func beats(a, b BaseOwner) bool {
	if a.Complete != b.Complete {
		return a.Complete
	}
	return sinceOrLast(a.Since) < sinceOrLast(b.Since)
}

func sinceOrLast(s int) int {
	if s < 0 {
		return math.MaxInt32
	}
	return s
}

// holdingSince is when the owner's oldest town hall still standing at base b at sec was started. Unit ids
// are recycled, so only a death after a hall's own start ends it.
func (g *game) holdingSince(b int, owner int8, sec int, deaths map[uint32][]int) int {
	standing := func(unit uint32, from int) bool {
		return !slices.ContainsFunc(deaths[unit], func(dead int) bool { return dead >= from && dead <= sec })
	}
	since := -1
	keep := func(from int) {
		if since < 0 || from < since {
			since = from
		}
	}
	for _, u := range g.d.Initial {
		if u.Owner == owner && IsTownHall(u.Type) && g.baseOf(u.X, u.Y) == b && standing(u.Unit, 0) {
			keep(0)
		}
	}
	for _, pr := range g.d.Productions {
		from := second(pr.Start)
		if pr.Owner != owner || !IsTownHall(pr.Type) || IsTownHall(pr.From) || from > sec {
			continue
		}
		if pr.Cancel != engine.NoFrame && second(pr.Cancel) <= sec {
			continue
		}
		if g.baseOf(pr.X, pr.Y) == b && standing(pr.Unit, from) {
			keep(from)
		}
	}
	return since
}
