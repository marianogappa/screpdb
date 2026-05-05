// Package phases computes per-replay early/mid/late game phase boundaries
// from an in-memory enriched command stream.
//
// The boundary logic mirrors populatePhaseMarkersForGameDetail
// (internal/dashboard/endpoint_main_game_detail.go) so the early/mid/late
// split shown on the per-game events list matches what attacker-composition
// markers see at ingest. The dashboard helper consumes already-persisted
// SQL rows; this helper consumes the in-memory enriched stream during
// ingest. The algorithm is identical — kept in two implementations only
// because the input shapes differ. De-duplicating these is a known
// follow-up.
//
// Returned values: replay-second at which Early ends and Mid ends. A value
// of 0 means "boundary not detected" — callers treat that section as
// open-ended (early covers all events; or mid extends to end-of-replay
// when only late is missing). Mirrors the dashboard's convention.
package phases

import (
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Compute walks an enriched command stream and returns (earlyEnd, midEnd)
// in seconds-from-game-start. Either may be 0 when the corresponding
// boundary signal is absent from the replay.
//
// Early ends at the EARLIEST of, across all players:
//   - first Mutalisk completion (Train/Morph)
//   - first Lurker completion
//   - first Wraith completion
//   - max(first Siege Tank, Tank Siege Mode tech researched)
//   - max(first Dragoon, Singularity Charge upgrade researched)
//
// Mid ends at the EARLIEST mid-game candidate that is also >= Early:
//   - first Defiler/Arbiter/Carrier/Battlecruiser/Ultralisk completion
//   - any Terran ground/armor +2 finished (per-player level == nth
//     occurrence of the upgrade command for that player+label)
func Compute(stream []cmdenrich.EnrichedCommand) (earlyEnd, midEnd int) {
	// Earliest first-occurrence per normalized unit key (across all players).
	firstUnit := map[string]int{}
	rememberFirstUnit := func(key string, second int) {
		if existing, ok := firstUnit[key]; !ok || second < existing {
			firstUnit[key] = second
		}
	}

	// Earliest Tank Siege Mode tech across all players.
	siegeModeSec := -1

	// Earliest Singularity Charge upgrade (any occurrence) and per-(player,label)
	// occurrences for Terran +2 detection.
	dragoonRangeSec := -1
	type plKey struct {
		player int64
		label  string
	}
	terran2Names := map[string]struct{}{
		"terran infantry weapons": {},
		"terran vehicle weapons":  {},
		"terran infantry armor":   {},
		"terran vehicle plating":  {},
	}
	upgradeOccurrences := map[plKey][]int{}

	for _, f := range stream {
		switch f.Kind {
		case cmdenrich.KindMakeUnit:
			for _, alias := range unitAliases(f.Subject) {
				rememberFirstUnit(alias, f.Second)
			}
		case cmdenrich.KindTech:
			label := strings.TrimSpace(f.Subject)
			if strings.EqualFold(label, "Tank Siege Mode") {
				if siegeModeSec < 0 || f.Second < siegeModeSec {
					siegeModeSec = f.Second
				}
			}
		case cmdenrich.KindUpgrade:
			labelLower := strings.ToLower(strings.TrimSpace(f.Subject))
			if strings.Contains(labelLower, "singularity charge") {
				if dragoonRangeSec < 0 || f.Second < dragoonRangeSec {
					dragoonRangeSec = f.Second
				}
			}
			if _, ok := terran2Names[labelLower]; ok {
				key := plKey{player: f.PlayerID, label: labelLower}
				upgradeOccurrences[key] = append(upgradeOccurrences[key], f.Second)
			}
		}
	}

	// Resolve Terran +2: across all (player,label) groups, find the earliest
	// 2nd-occurrence second. That's the first time *any* Terran ground/armor
	// reached level 2.
	terranPlus2Sec := -1
	for _, secs := range upgradeOccurrences {
		if len(secs) < 2 {
			continue
		}
		sort.Ints(secs)
		if terranPlus2Sec < 0 || secs[1] < terranPlus2Sec {
			terranPlus2Sec = secs[1]
		}
	}

	mutaSec := lookupFirst(firstUnit, "mutalisk")
	lurkerSec := lookupFirst(firstUnit, "lurker")
	wraithSec := lookupFirst(firstUnit, "wraith")
	siegeTankSec := lookupFirst(firstUnit, "siegetank", "siegetanktankmode", "siegetanksiegemode")
	dragoonSec := lookupFirst(firstUnit, "dragoon")
	// Additional Protoss tier-2 ground signals (no upgrade gate). The
	// dashboard's original algorithm only considers Dragoon+Singularity
	// Charge for Protoss, which misses common openings where Toss
	// transitions through Reaver or DT first (Robotics or Citadel/Templar
	// Archives path) before researching range — and PvP games where
	// Singularity Charge is often skipped entirely. Without these, a fast
	// Carrier game where neither player ever hits Muta/Lurker/Wraith/
	// SiegeTank+Mode/Dragoon+Range produces earlyEnd=0 and Carriers fall
	// into the "early" phase pill on the per-game composition surface.
	reaverSec := lookupFirst(firstUnit, "reaver")
	darkTemplarSec := lookupFirst(firstUnit, "darktemplar")

	siegeArmedSec := maxOf(siegeTankSec, siegeModeSec)
	dragoonArmedSec := maxOf(dragoonSec, dragoonRangeSec)

	earlyRaw := earliest(mutaSec, lurkerSec, wraithSec, siegeArmedSec, dragoonArmedSec, reaverSec, darkTemplarSec)

	defilerSec := lookupFirst(firstUnit, "defiler")
	arbiterSec := lookupFirst(firstUnit, "arbiter")
	carrierSec := lookupFirst(firstUnit, "carrier")
	bcSec := lookupFirst(firstUnit, "battlecruiser")
	ultraSec := lookupFirst(firstUnit, "ultralisk")

	midCandidate := earliest(defilerSec, arbiterSec, carrierSec, bcSec, ultraSec, terranPlus2Sec)
	midRaw := midCandidate
	if midRaw >= 0 && earlyRaw >= 0 && midRaw < earlyRaw {
		midRaw = earlyRaw
	}

	if earlyRaw > 0 {
		earlyEnd = earlyRaw
	}
	if midRaw > 0 {
		midEnd = midRaw
	}
	return earlyEnd, midEnd
}

// unitAliases mirrors dashboard.unitNameAliases — normalizes to lowercase
// alphanumeric and emits both the full name and the race-prefix-stripped
// form so callers can match either "siegetanktankmode" or
// "siegetanksiegemode" → "siegetank" alias variants.
func unitAliases(name string) []string {
	base := normalize(name)
	if base == "" {
		return nil
	}
	out := []string{base}
	for _, prefix := range []string{"terran", "zerg", "protoss"} {
		if strings.HasPrefix(base, prefix) && len(base) > len(prefix) {
			out = append(out, strings.TrimPrefix(base, prefix))
		}
	}
	return out
}

func normalize(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	if raw == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(raw))
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func lookupFirst(m map[string]int, keys ...string) int {
	best := -1
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if best < 0 || v < best {
				best = v
			}
		}
	}
	return best
}

func earliest(seconds ...int) int {
	best := -1
	for _, s := range seconds {
		if s < 0 {
			continue
		}
		if best < 0 || s < best {
			best = s
		}
	}
	return best
}

func maxOf(a, b int) int {
	if a < 0 || b < 0 {
		return -1
	}
	if a > b {
		return a
	}
	return b
}
