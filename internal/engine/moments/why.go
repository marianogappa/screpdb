package moments

import (
	"slices"

	"github.com/marianogappa/screpdb/internal/engine"
)

// Why points at the event that explains why a losing team left. In a 1v1 the losing team is one player.
// End is the second the team ended.
type Why struct {
	Loser int8
	Event int
	End   int
	Force Force
	// TargetBases is how many town halls the event's target had standing when it began.
	TargetBases int
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
			w.End = end
			out = append(out, w)
		}
	}
	if !known && !anyLeft && len(order) == 2 {
		end := min(second(g.d.LastFrame), g.trusted)
		a, sa, oka := bestWhy(events, teams[order[0]], end)
		b, sb, okb := bestWhy(events, teams[order[1]], end)
		a.End, b.End = end, end
		switch {
		case oka && (!okb || sa >= sb):
			return []Why{a}
		case okb:
			return []Why{b}
		}
	}
	slices.SortFunc(out, func(a, b Why) int { return a.End - b.End })
	return out
}

// Perspective is what one viewer is told about why a game ended. You is the viewer's own turning point: why
// they lost, or when they won, why the last side against them did. When the viewer lost and nothing in their
// last four minutes was aimed at them, the costliest blow to an ally in that time stands in, and Ally names
// that ally: a side often collapses together, the viewer leaving after a teammate is beaten. Final is why the
// game ended: the turning point of the last side to lose.
type Perspective struct {
	You   *Why
	Ally  int8
	Final *Why
	// LiveAllies is how many of the viewer's allies were still playing when their turning point happened.
	LiveAllies int
}

// TurningPoints is the Perspective of the player in slot you, or NoPlayer for a game with no viewer in it.
func TurningPoints(d *engine.Digest, bases []Base, players []Player, events []Event, you int8) Perspective {
	g := newGame(d, bases, players)
	p := g.turningPoints(events, you)
	for _, w := range []*Why{p.You, p.Final} {
		if w != nil {
			w.Force = g.force(events[w.Event], events)
			w.TargetBases = g.townHallsOf(events[w.Event].Target, events[w.Event].Second)
		}
	}
	if p.You != nil {
		p.LiveAllies = g.liveAllies(you, events[p.You.Event].Second)
	}
	return p
}

func (g *game) turningPoints(events []Event, you int8) Perspective {
	p := Perspective{Ally: NoPlayer}
	whys := g.whys(events)
	if len(whys) > 0 {
		p.Final = &whys[len(whys)-1]
	}
	if you == NoPlayer || !g.players[you].Human {
		return p
	}
	if !g.youLost(you, whys) {
		if p.Final != nil {
			w := *p.Final
			p.You = &w
		}
		return p
	}
	end := g.endOf(you)
	if end < 0 {
		end = second(g.d.LastFrame)
	}
	end = min(end, g.trusted)
	if w, _, ok := bestWhy(events, []int8{you}, end); ok {
		w.End = end
		p.You = &w
		return p
	}
	var allies []int8
	for _, slot := range sortedSlots(g.players) {
		if slot != you && g.players[slot].Human && g.teamOf(slot) == g.teamOf(you) {
			allies = append(allies, slot)
		}
	}
	if w, _, ok := bestWhy(events, allies, end); ok {
		w.End = end
		p.You, p.Ally = &w, w.Loser
	}
	return p
}

// townHallsOf counts a player's standing town halls by place, since a Lair or Hive is also recorded as the
// Hatchery it morphed from.
func (g *game) townHallsOf(slot int8, sec int) int {
	at := map[[2]int32]bool{}
	for _, b := range g.buildingsAt(sec) {
		if b.owner == slot && IsTownHall(b.typ) {
			at[[2]int32{b.x, b.y}] = true
		}
	}
	return len(at)
}

func (g *game) liveAllies(you int8, sec int) int {
	n := 0
	for _, slot := range sortedSlots(g.players) {
		if slot != you && g.players[slot].Human && g.teamOf(slot) == g.teamOf(you) {
			if end := g.endOf(slot); end < 0 || end >= sec {
				n++
			}
		}
	}
	return n
}

// youLost says the viewer's side lost: by the library's result, else unless the engine saw them win, since a
// viewer's own replay with no result ends when they quit.
func (g *game) youLost(you int8, whys []Why) bool {
	if r := g.players[you].Result; r != 0 {
		return r == ResultLost
	}
	return !g.won(you)
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
		case slices.Contains(team, e.Target) && (decisive(e) || bigWin(e) || crushingRush(e)):
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

var rushes = map[string]bool{"zergling_rush": true, "cannon_rush": true, "bunker_rush": true}

// crushingRush is a rush that crippled its target: a town hall, 6 workers or 1500 resources, for less than it
// killed.
func crushingRush(e Event) bool {
	o := e.Outcome
	return rushes[e.Type] && (o.TownHall || o.Workers >= 6 || o.Killed >= 1500) && o.Killed >= o.Lost
}

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
