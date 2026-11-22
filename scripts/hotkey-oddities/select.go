package main

import (
	"math"
	"sort"

	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/propack"
)

const (
	minGamesPerPro = 20
	// maxSharedWith keeps a key habit only when at most this many other pros of the race share it.
	maxSharedWith = 2
	maxOddities   = 3
	// minRateGames is the fewest games a pro's shown tempo is measured over.
	minRateGames = 5
)

var raceNames = map[string]string{"P": "Protoss", "T": "Terran", "Z": "Zerg"}

var kindPriority = []string{
	propack.OddityHallKey, propack.OddityHotkeyShare, propack.OddityFirstSupply, propack.OddityComsatKeys, propack.OddityRareUnit, propack.OddityDeselects, propack.OddityUnitKey, propack.OddityProdKey, propack.OddityOpening, propack.OddityShiftAdds, propack.OddityRebinds, propack.OddityFastPresses, propack.OddityFewKeys,
	propack.OddityWorkerKey, propack.OddityTechBinds, propack.OddityFewHallBinds, propack.OddityStandardLayout,
}

type proRace struct{ pro, race string }

// proRates is each pro's median hotkey tempo per race, keyed by pro label and race name.
func proRates(games []sideTraits) map[string]map[string]hotkeystream.Rates {
	byPro := map[proRace][]hotkeystream.Rates{}
	for _, g := range games {
		k := proRace{g.Pro, g.Race}
		byPro[k] = append(byPro[k], hotkeystream.Rates{PressesPerMinute: g.Measures["presses_min"], RebindsPerMinute: g.Measures["rebinds_min"]})
	}
	out := map[string]map[string]hotkeystream.Rates{}
	for k, rates := range byPro {
		if len(rates) < minRateGames {
			continue
		}
		if out[k.pro] == nil {
			out[k.pro] = map[string]hotkeystream.Rates{}
		}
		m := hotkeystream.MedianRates(rates)
		m.PressesPerMinute, m.RebindsPerMinute = math.Round(m.PressesPerMinute), math.Round(m.RebindsPerMinute)
		out[k.pro][raceNames[k.race]] = m
	}
	return out
}

// measuredOddities reads rates from the replays, keyed by pro label and race name.
func measuredOddities(games []sideTraits) map[string]map[string][]propack.HotkeyOddity {
	byPro := map[proRace][]sideTraits{}
	for _, g := range games {
		byPro[proRace{g.Pro, g.Race}] = append(byPro[proRace{g.Pro, g.Race}], g)
	}
	medians := map[proRace]map[string]float64{}
	openings := map[proRace]map[string]float64{}
	for k, gs := range byPro {
		if len(gs) < minGamesPerPro {
			delete(byPro, k)
			continue
		}
		openings[k] = openingShares(gs)
		values := map[string][]float64{}
		for _, g := range gs {
			for m, v := range g.Measures {
				values[m] = append(values[m], v)
			}
		}
		medians[k] = map[string]float64{}
		for m, vs := range values {
			if len(vs) >= minGamesPerPro {
				medians[k][m] = median(vs)
			}
		}
	}
	out := map[string]map[string][]propack.HotkeyOddity{}
	for k := range byPro {
		var peers []proRace
		for j := range byPro {
			if j.race == k.race && j != k {
				peers = append(peers, j)
			}
		}
		found := measureOddities(k, peers, medians, byPro[k])
		if o, ok := openingOddity(k, peers, openings); ok {
			found = append(found, o)
		}
		if o, ok := shiftAddOddity(k, peers, medians); ok {
			found = append(found, o)
		}
		if len(found) == 0 {
			continue
		}
		if out[k.pro] == nil {
			out[k.pro] = map[string][]propack.HotkeyOddity{}
		}
		out[k.pro][raceNames[k.race]] = found
	}
	return out
}

// finalize orders a pro's oddities, the least shared first, and keeps the top ones.
func finalize(found []propack.HotkeyOddity) []propack.HotkeyOddity {
	sort.SliceStable(found, func(i, j int) bool {
		if len(found[i].SharedWith) != len(found[j].SharedWith) {
			return len(found[i].SharedWith) < len(found[j].SharedWith)
		}
		return priority(found[i].Kind) < priority(found[j].Kind)
	})
	if len(found) > maxOddities {
		found = found[:maxOddities]
	}
	return found
}

func priority(kind string) int {
	for i, k := range kindPriority {
		if k == kind {
			return i
		}
	}
	return len(kindPriority)
}

// dropRepeatedHall drops a main base line when the opening line already starts by binding the base to that key.
func dropRepeatedHall(found []propack.HotkeyOddity) []propack.HotkeyOddity {
	openingHall := map[int]bool{}
	for _, o := range found {
		if o.Kind == propack.OddityOpening {
			for _, b := range o.Binds {
				if b.Building != "" {
					openingHall[b.Key] = true
				}
			}
		}
	}
	kept := found[:0]
	for _, o := range found {
		if o.Kind == propack.OddityHallKey && openingHall[o.Value] {
			continue
		}
		kept = append(kept, o)
	}
	return kept
}

// dropRepeatedUnit drops a main-key line when a rare-unit line already names that unit on that key.
func dropRepeatedUnit(found []propack.HotkeyOddity) []propack.HotkeyOddity {
	rare := map[string]bool{}
	for _, o := range found {
		if o.Kind == propack.OddityRareUnit {
			rare[o.Unit] = true
		}
	}
	kept := found[:0]
	for _, o := range found {
		if o.Kind == propack.OddityUnitKey && rare[o.Unit] {
			continue
		}
		kept = append(kept, o)
	}
	return kept
}

// layoutKinds are the habits about what sits on which key; a card with one of them is not called standard.
var layoutKinds = map[string]bool{
	propack.OddityOpening: true, propack.OddityHallKey: true, propack.OddityWorkerKey: true, propack.OddityUnitKey: true,
	propack.OddityProdKey: true, propack.OddityRareUnit: true, propack.OddityFirstSupply: true, propack.OddityComsatKeys: true,
	propack.OddityFewKeys: true,
}

// dropContradictions removes lines that would read against another line on the same card: a standard layout
// next to an unusual key habit, and the share of commands that are hotkeys next to the hotkey press rate,
// which says nearly the same thing.
func dropContradictions(found []propack.HotkeyOddity) []propack.HotkeyOddity {
	unusual, fast := false, false
	for _, o := range found {
		unusual = unusual || layoutKinds[o.Kind]
		fast = fast || o.Kind == propack.OddityFastPresses
	}
	kept := found[:0]
	for _, o := range found {
		if (o.Kind == propack.OddityStandardLayout && unusual) || (o.Kind == propack.OddityHotkeyShare && fast) {
			continue
		}
		kept = append(kept, o)
	}
	return kept
}
