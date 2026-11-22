package main

import (
	"math"
	"sort"

	"github.com/marianogappa/screpdb/internal/propack"
)

// measureRule flags a pro whose median is far above their race's median and among its highest. The line
// states the rank, so a cluster of similar pros reads as a cluster rather than as one unique player.
// Every measure is something a pro does, never a skill judgement, and only the flattering or neutral side of
// each is ever shown.
type measureRule struct {
	measure string
	kind    string
	// low flags the bottom of the race instead of the top.
	low bool
	// factor is how far from the race median the player must be: factor times above it, or below it divided
	// by factor when low.
	factor float64
	// scale converts the measure to the integer shown (100 for shares shown as percentages).
	scale float64
}

// measureMaxRank is the lowest rank a rate line is shown for.
const measureMaxRank = 3

var measureRules = []measureRule{
	{measure: "rebinds_min", kind: propack.OddityRebinds, factor: 3, scale: 1},
	{measure: "presses_min", kind: propack.OddityFastPresses, factor: 1.5, scale: 1},
	{measure: "hotkey_share", kind: propack.OddityHotkeyShare, factor: 1.2, scale: 100},
	{measure: "deselects_min", kind: propack.OddityDeselects, factor: 1.4, scale: 10},
	{measure: "tech_binds", kind: propack.OddityTechBinds, factor: 3, scale: 1},
	{measure: "hall_binds", kind: propack.OddityFewHallBinds, low: true, factor: 1.5, scale: 1},
}

// measureConsistency is the share of a player's games that must sit on the same side of the race median as
// their own median, so a line never rests on a few extreme games.
const measureConsistency = 0.75

// measureOddities ranks by the value the line shows, so two players who read the same are tied rather than
// ranked apart by a hidden decimal.
func measureOddities(k proRace, peers []proRace, medians map[proRace]map[string]float64, games []sideTraits) []propack.HotkeyOddity {
	var out []propack.HotkeyOddity
	for _, rule := range measureRules {
		raw, ok := medians[k][rule.measure]
		if !ok || math.IsNaN(raw) {
			continue
		}
		shown := func(v float64) float64 { return math.Round(v * rule.scale) }
		own := shown(raw)
		all := []float64{raw}
		rank, tied := 1, false
		for _, j := range peers {
			v, ok := medians[j][rule.measure]
			if !ok || math.IsNaN(v) {
				continue
			}
			all = append(all, v)
			if (!rule.low && shown(v) > own) || (rule.low && shown(v) < own) {
				rank++
			}
			tied = tied || shown(v) == own
		}
		race := median(all)
		if race <= 0 || rank > measureMaxRank {
			continue
		}
		if (!rule.low && raw < rule.factor*race) || (rule.low && raw > race/rule.factor) {
			continue
		}
		beyond, measured := 0, 0
		for _, g := range games {
			if v, ok := g.Measures[rule.measure]; ok {
				measured++
				if (!rule.low && v > race) || (rule.low && v < race) {
					beyond++
				}
			}
		}
		if measured == 0 || float64(beyond) < measureConsistency*float64(measured) {
			continue
		}
		out = append(out, propack.HotkeyOddity{Kind: rule.kind, Value: int(own), Baseline: int(math.Round(race * rule.scale)), Tenths: rule.scale == 10, Rank: rank, Tied: tied, Population: len(all)})
	}
	return out
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	if len(s)%2 == 1 {
		return s[len(s)/2]
	}
	return (s[len(s)/2-1] + s[len(s)/2]) / 2
}
