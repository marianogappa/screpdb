package moments

import (
	"slices"

	"github.com/marianogappa/screpdb/internal/engine"
)

// Why points at the event that explains why a losing team left. In a 1v1 the losing team is one player.
type Why struct {
	Loser int8
	Event int
}

// whyLookBack is how long before a team left an event may have happened and still explain it.
const whyLookBack = 240

// Whys picks, for each team that lost, the event in its last four minutes that cost it the most, if any cost
// it enough to explain the loss. It reads the curated events and the insights together, after Combine.
func Whys(d *engine.Digest, bases []Base, players []Player, events []Event) []Why {
	return newGame(d, bases, players).whys(events)
}

// A team lost when the library says so, or, when it does not know, when every member left, dropped or was
// defeated and none won. It ended when its last member went, so a teammate quitting in the first minutes does
// not end it; a losing team nobody saw leave is the recorder's own, its replay cut when they quit. With no
// result at all, between two sides, the stronger explanation names who lost.
func (g *game) whys(events []Event) []Why {
	teams := map[int][]int8{}
	var order []int
	known := false
	for slot, p := range g.players {
		if !p.Human {
			continue
		}
		known = known || p.Result != 0
		t := g.teamOf(slot)
		if _, seen := teams[t]; !seen {
			order = append(order, t)
		}
		teams[t] = append(teams[t], slot)
	}
	if len(teams) < 2 {
		return nil
	}
	slices.Sort(order)
	var out []Why
	anyLeft := false
	for _, t := range order {
		end, lost, left := g.teamEnd(teams[t], known)
		anyLeft = anyLeft || left
		if !lost {
			continue
		}
		if w, _, ok := bestWhy(events, teams[t], min(end, g.trusted)); ok {
			out = append(out, w)
		}
	}
	if !known && !anyLeft && len(order) == 2 {
		end := min(second(g.d.LastFrame), g.trusted)
		a, sa, oka := bestWhy(events, teams[order[0]], end)
		b, sb, okb := bestWhy(events, teams[order[1]], end)
		switch {
		case oka && (!okb || sa >= sb):
			return []Why{a}
		case okb:
			return []Why{b}
		}
	}
	slices.SortFunc(out, func(a, b Why) int { return events[a.Event].Second - events[b.Event].Second })
	return out
}

// endOf is when a player left: the engine's record, else the commands', else -1.
func (g *game) endOf(slot int8) int {
	if at := g.leftAt(slot); at >= 0 {
		return at
	}
	if at := g.players[slot].LeftAt; at > 0 {
		return at
	}
	return -1
}

// teamEnd is when a team ended, whether it lost, and whether any member was seen leaving.
func (g *game) teamEnd(members []int8, known bool) (end int, lost, anyLeft bool) {
	if known {
		lost = slices.ContainsFunc(members, func(m int8) bool { return g.players[m].Result == ResultLost })
		end = -1
		for _, m := range members {
			if at := g.endOf(m); at >= 0 {
				anyLeft = true
				end = max(end, at)
			}
		}
		if end < 0 {
			end = second(g.d.LastFrame)
		}
		return end, lost, anyLeft
	}
	lost = true
	for _, m := range members {
		if g.won(m) {
			return 0, false, true
		}
		at := g.leftAt(m)
		if at < 0 {
			lost = false
			continue
		}
		anyLeft = true
		end = max(end, at)
	}
	return end, lost, anyLeft
}

func (g *game) won(slot int8) bool {
	if r := g.players[slot].Result; r != 0 {
		return r == ResultWon
	}
	return slices.ContainsFunc(g.d.Outcomes, func(o engine.Outcome) bool { return o.Owner == slot && o.Kind == engine.OutcomeWon })
}

// bestWhy is the event in a team's last four minutes that cost it the most: an attack on it it could not
// hold, or one of its own attacks that threw its army away.
func bestWhy(events []Event, team []int8, end int) (Why, int32, bool) {
	best, bestScore, loser := -1, int32(0), NoPlayer
	for i, e := range events {
		if e.Second+e.Seconds < end-whyLookBack || e.Second > end {
			continue
		}
		var score int32
		var who int8
		switch {
		case slices.Contains(team, e.Target) && (decisive(e) || bigWin(e)):
			score, who = e.Outcome.Killed+100*int32(e.Outcome.Workers), e.Target
			if e.Outcome.TownHall {
				score += 2000
			}
		case slices.Contains(team, e.Actor) && bigLoss(e):
			score, who = e.Outcome.Lost, e.Actor
		default:
			continue
		}
		if best < 0 || score > bestScore {
			best, bestScore, loser = i, score, who
		}
	}
	return Why{Loser: loser, Event: best}, bestScore, best >= 0
}

var winningAttacks = map[string]bool{"attack": true, "drop": true, "reaver_drop": true, "dt_drop": true, "cliff_drop": true, "nydus_attack": true, "recall": true}

// bigLoss is an attack, drop or recall that threw its army away: 1500 resources lost for less than half of
// it killed, or a recall the engine saw trade poorly or get wiped out for 1000.
func bigLoss(e Event) bool {
	o := e.Outcome
	if e.Type == "recall" && (e.Result == RecallTradedPoorly || e.Result == RecallWiped) && o.Lost >= 1000 {
		return true
	}
	return winningAttacks[e.Type] && o.Lost >= 1500 && o.Lost >= 2*o.Killed
}

// bigWin is a plain attack that explains a loss on its own: it took a town hall or 1500 resources and won the
// exchange.
func bigWin(e Event) bool {
	o := e.Outcome
	return winningAttacks[e.Type] && (o.TownHall || o.Killed >= 1500) && o.Killed >= o.Lost
}
