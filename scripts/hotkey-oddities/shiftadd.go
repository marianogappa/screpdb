package main

import (
	"math"

	"github.com/marianogappa/screpdb/internal/propack"
)

const (
	// shiftAddMinShare is the share of group edits a player must make with Shift+number: most players edit
	// groups with Ctrl+number almost every time, so a tenth already stands out.
	shiftAddMinShare = 0.1
	// shiftAddMaxShared keeps the habit to players few others of the race share.
	shiftAddMaxShared = 2
)

func shiftAddOddity(k proRace, peers []proRace, medians map[proRace]map[string]float64) (propack.HotkeyOddity, bool) {
	own, ok := medians[k]["shift_add_share"]
	if !ok || own < shiftAddMinShare {
		return propack.HotkeyOddity{}, false
	}
	all := []float64{own}
	shared := 0
	for _, j := range peers {
		v, ok := medians[j]["shift_add_share"]
		if !ok {
			continue
		}
		all = append(all, v)
		if v >= shiftAddMinShare {
			shared++
		}
	}
	if shared > shiftAddMaxShared {
		return propack.HotkeyOddity{}, false
	}
	return propack.HotkeyOddity{Kind: propack.OddityShiftAdds, Value: int(math.Round(own * 100)), Baseline: int(math.Round(median(all) * 100)), Population: len(all)}, true
}
