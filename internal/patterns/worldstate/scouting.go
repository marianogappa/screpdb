package worldstate

import (
	"math"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Worker / non-combat units. A player whose only trained units are workers +
// queue overhead (Larva, Egg) is still in scout-only territory — scouts
// happen before any combat unit exists.
//
// Used by the donor-derived attacks pass (attacks_pass.go) to detect
// lone-worker scout incursions that the rolling-pressure tracker misses.
var scoutNonCombatUnits = map[string]bool{
	"SCV":             true,
	"Probe":           true,
	"Drone":           true,
	"Overlord":        true,
	"Larva":           true,
	"Egg":             true,
	"Lurker Egg":      true,
	"Mutalisk Cocoon": true,
}

// firstCombatUnitSec returns, per player byte PlayerID, the second at which
// that player first issued a KindMakeUnit for a combat-class unit. If a
// player never trains a combat unit, the entry is math.MaxInt (always
// considered "still in scout-only mode").
//
// Pre-pass over the enriched stream — linear, single walk. Cheap.
// Time math is in seconds (test fixtures populate Second but leave Frame
// at zero, so frame-based comparisons would be wrong).
func firstCombatUnitSec(stream []cmdenrich.EnrichedCommand) map[byte]int {
	out := map[byte]int{}
	for _, ec := range stream {
		if ec.Kind != cmdenrich.KindMakeUnit || ec.Subject == "" {
			continue
		}
		if scoutNonCombatUnits[ec.Subject] {
			continue
		}
		pid := byte(ec.PlayerID)
		if _, seen := out[pid]; seen {
			continue
		}
		out[pid] = ec.Second
	}
	return out
}

// firstCombatSecOr returns the player's first-combat-unit second, or
// math.MaxInt if they never trained one.
func firstCombatSecOr(m map[byte]int, pid byte) int {
	if s, ok := m[pid]; ok {
		return s
	}
	return math.MaxInt
}

// IsScoutPolygon decides whether a polygon is an interesting scout target.
// Workers visiting random middle expansions don't tell a story — only the
// enemy's main or natural carry the scouting narrative.
func IsScoutPolygon(p PolygonGeom) bool {
	return p.Kind == "start" || p.Kind == "natural"
}
