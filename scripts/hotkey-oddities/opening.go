package main

import (
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/propack"
)

const (
	// openingHabitShare is how often a player must open with the same binds for it to be their opening.
	openingHabitShare = 0.7
	// openingSharedShare is how often another player opening the same way counts as sharing it.
	openingSharedShare = 0.2
)

func openingShares(gs []sideTraits) map[string]float64 {
	shares := map[string]float64{}
	for _, g := range gs {
		if g.Opening != "" {
			shares[g.Opening] += 1 / float64(len(gs))
		}
	}
	return shares
}

// openingOddity is a player's habitual opening binds when no other built-in player of the race opens that
// way in even a fifth of their games. A one-bind opening says nothing, so it needs at least two binds.
func openingOddity(k proRace, peers []proRace, shares map[proRace]map[string]float64) (propack.HotkeyOddity, bool) {
	best, bestShare := "", 0.0
	for o, s := range shares[k] {
		if s > bestShare {
			best, bestShare = o, s
		}
	}
	binds := strings.Split(best, ",")
	if bestShare < openingHabitShare || len(binds) < 2 {
		return propack.HotkeyOddity{}, false
	}
	for _, j := range peers {
		if shares[j][best] >= openingSharedShare {
			return propack.HotkeyOddity{}, false
		}
	}
	o := propack.HotkeyOddity{Kind: propack.OddityOpening, Value: int(bestShare*100 + 0.5), Population: len(peers) + 1}
	for _, b := range binds {
		key, what, _ := strings.Cut(b, ":")
		n, _ := strconv.Atoi(key)
		o.Binds = append(o.Binds, propack.HotkeyBind{Key: n, Building: strings.TrimPrefix(what, "units")})
	}
	return o, true
}
