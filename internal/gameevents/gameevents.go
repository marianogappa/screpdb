// Package gameevents chooses a game's key events: the few the Game Events tab shows by default.
// It knows nothing of where events come from (the engine or the command stream); each source detects
// its candidates, describes how much each one mattered, and Select keeps the mandatory ones plus the
// weightiest others until the game's budget is spent. Longer games and games with more players get a
// bigger budget, up to a cap: past it, one more event no longer makes the list better.
package gameevents

import (
	"math"
	"sort"
)

// Magnitude is what a source knows about how much an event mattered. Zero values mean unknown.
type Magnitude struct {
	// Traded is the resources both sides lost in it.
	Traded int
	// Workers is how many workers the actor killed.
	Workers int
	// TownHall is set when the actor killed a town hall.
	TownHall bool
	// Nth counts this actor's events of this type so far, from 1.
	Nth int
	// First is set for the actor's first event of this type against this target.
	First bool
}

type Candidate struct {
	Type   string
	Second int
	// Actor is any stable id of the player the event is about; 0 for none.
	Actor     int64
	Magnitude Magnitude
}

type Game struct {
	DurationSeconds int
	Players         int
}

// mandatory events are always key: how the game opened, who left, who allied.
var mandatory = map[string]bool{
	"bo_openers": true, "player_start": true, "leave_game": true, "player_dropped": true, "mass_disconnect": true,
	"player_stopped_playing": true, "late_alliance": true, "team_stacking_detected": true,
}

func Mandatory(eventType string) bool { return mandatory[eventType] }

// baseWeight is how interesting a type is when nothing else is known about the event.
var baseWeight = map[string]float64{
	"zergling_rush": 0.95, "cannon_rush": 0.95, "bunker_rush": 0.95,
	"proxy_gate": 0.9, "proxy_rax": 0.9, "proxy_factory": 0.85, "proxy_starport": 0.85,
	"nuke": 0.85, "recall": 0.85, "nydus_attack": 0.85,
	"became_terran": 0.8, "became_zerg": 0.8,
	"reaver_drop": 0.75, "dt_drop": 0.75, "drop": 0.55,
	"manner_pylon": 0.6,
	"attack":       0.45,
	"worker_kills": 0.45,
	"expansion":    0.3,
	"first_reaver": 0.45, "first_corsair": 0.45, "speedlot": 0.4, "mech_transition": 0.4,
	"takeover":          0.35,
	"long_supply_block": 0.2,
	"scout":             0.1,
	"location_inactive": 0.05,
}

const defaultWeight = 0.3

// Weight is how much one candidate deserves a place: its type, then what happened in it. A trade of
// 3000 resources or a killed town hall makes any fight a headline; a player's first expansions say
// more than their sixth.
func Weight(c Candidate) float64 {
	w, ok := baseWeight[c.Type]
	if !ok {
		w = defaultWeight
	}
	m := c.Magnitude
	switch c.Type {
	case "attack", "worker_kills", "drop", "reaver_drop", "dt_drop", "recall", "nydus_attack":
		w += min(0.4, float64(m.Traded)/7500)
		w += min(0.3, float64(m.Workers)/30)
		if m.TownHall {
			w += 0.3
		}
		if m.First {
			w += 0.1
		}
	case "expansion":
		switch {
		case m.Nth == 1:
			w += 0.3
		case m.Nth == 2:
			w += 0.15
		case m.Nth > 4:
			w -= 0.15
		}
	}
	return w
}

// Budget is how many non-mandatory events a game shows by default.
func Budget(g Game) int {
	minutes := float64(g.DurationSeconds) / 60
	b := 3 + 0.5*minutes + float64(max(0, g.Players-2))
	return int(math.Max(4, math.Min(22, math.Round(b))))
}

const (
	// Each key event already chosen for the same actor and type makes the next one less welcome, so
	// one player's string of attacks does not crowd out everything else.
	repeatFactor = 0.6
	// Two key events seconds apart mostly tell one moment.
	crowdSeconds = 20
	crowdFactor  = 0.7
	minWeight    = 0.12
)

// Select marks the key events: every mandatory one, then greedily the weightiest of the rest.
func Select(cs []Candidate, g Game) []bool {
	key := make([]bool, len(cs))
	weights := make([]float64, len(cs))
	for i, c := range cs {
		key[i] = mandatory[c.Type]
		weights[i] = Weight(c)
	}
	type typeActor struct {
		t string
		a int64
	}
	repeats := map[typeActor]int{}
	var chosen []int
	for range Budget(g) {
		best, bestScore := -1, minWeight
		for i, c := range cs {
			if key[i] {
				continue
			}
			score := weights[i] * math.Pow(repeatFactor, float64(repeats[typeActor{c.Type, c.Actor}]))
			for _, j := range chosen {
				if abs(cs[j].Second-c.Second) <= crowdSeconds {
					score *= crowdFactor
				}
			}
			if score > bestScore || score == bestScore && best >= 0 && c.Second < cs[best].Second {
				best, bestScore = i, score
			}
		}
		if best < 0 {
			break
		}
		key[best] = true
		chosen = append(chosen, best)
		repeats[typeActor{cs[best].Type, cs[best].Actor}]++
	}
	return key
}

// Number fills Magnitude.Nth: the order of each actor's events of each type, by time.
func Number(cs []Candidate) {
	order := make([]int, len(cs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return cs[order[a]].Second < cs[order[b]].Second })
	seen := map[[2]any]int{}
	for _, i := range order {
		k := [2]any{cs[i].Type, cs[i].Actor}
		seen[k]++
		cs[i].Magnitude.Nth = seen[k]
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
