package dashboard

import (
	"database/sql"
	"sort"

	db "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/models"
)

// Per-game attacker-composition computation, request-time.
//
// At request time the per-game endpoint loads the persisted phase
// boundaries (mid_game_starts / late_game_starts replay-level markers)
// and the Train / Unit Morph / Cast command stream for the replay, then
// produces a flat list of (player, phase, units, casters) entries. The
// frontend renders per-player rows on each player strip and aggregates
// client-side into 3 replay-level pills for the per-game summary
// surface. Counts are sent raw — the frontend does the 10-slot
// proportional fill on render so we don't lock into a presentation
// shape on the wire.
//
// Why request-time, not ingest-time: the rules (caster set, signature
// non-casters, excluded units) iterate without re-ingest. Persisting
// the histogram on every replay would lock those rules at ingest and
// force re-detection passes to update.

// compositionCasters: units that can cast spells. Excluded from the
// units histogram. Surfaced in the right strip iff they actually cast a
// spell during the phase. Keep in sync with
// internal/models/order_unit_associations.go (UnitOrderToUnit).
var compositionCasters = map[string]struct{}{
	models.GeneralUnitHighTemplar:   {},
	models.GeneralUnitDefiler:       {},
	models.GeneralUnitMedic:         {},
	models.GeneralUnitGhost:         {},
	models.GeneralUnitQueen:         {},
	models.GeneralUnitScienceVessel: {},
	models.GeneralUnitArbiter:       {},
	models.GeneralUnitDarkArchon:    {},
	models.GeneralUnitCorsair:       {},
}

// compositionSignatureNonCasters: notable non-spellcaster units that
// warrant a "once if built" chip in the right strip rather than
// counting toward the units histogram. Reserved for units that aren't
// really part of bulk-army composition — transports, nukes, late
// morphs, signature stealth/harassers — where seeing them appear at
// all is the interesting signal, not their count.
//
// Note: Carrier and Reaver are intentionally NOT in this set. Both are
// produced in meaningful numbers (4-12 Carriers, 2-6 Reavers) and read
// as primary army composition, so they belong in the slot strip on the
// left where the proportional fill reflects their actual share.
var compositionSignatureNonCasters = map[string]struct{}{
	models.GeneralUnitDarkTemplar:    {},
	models.GeneralUnitBattlecruiser:  {},
	models.GeneralUnitDropship:       {},
	models.GeneralUnitNuclearMissile: {},
	models.GeneralUnitGuardian:       {},
	models.GeneralUnitDevourer:       {},
}

// compositionExcluded: workers + supply. Don't appear anywhere.
var compositionExcluded = map[string]struct{}{
	models.UnitNameDrone:    {},
	models.UnitNameProbe:    {},
	models.UnitNameSCV:      {},
	models.UnitNameOverlord: {},
}

// computeCompositionForReplay walks the production+cast rows for one
// replay and returns one workflowGameUnitComposition entry per (player,
// phase) where the player produced ≥1 non-excluded attacking unit. A
// phase with only orphan casts (caster built earlier, casts now, no
// other production this phase) does not render — the cast info reads
// poorly without composition context.
//
// 0-valued boundaries mean "not detected" and collapse the adjacent
// phase, matching dashboard.populatePhaseMarkersForGameDetail.
func computeCompositionForReplay(rows []db.UnitProductionOrCastRow, boundaries db.PhaseBoundaries) []workflowGameUnitComposition {
	if len(rows) == 0 {
		return nil
	}
	earlyEnd := int(boundaries.EarlyEndsAtSecond)
	midEnd := int(boundaries.MidEndsAtSecond)

	type accumKey struct {
		playerID int64
		phase    string
	}
	type accum struct {
		units           map[string]int
		castersThatCast map[string]struct{}
		signaturesBuilt map[string]struct{}
		productionCount int
	}
	buckets := map[accumKey]*accum{}
	getBucket := func(playerID int64, phase string) *accum {
		key := accumKey{playerID, phase}
		b, ok := buckets[key]
		if !ok {
			b = &accum{
				units:           map[string]int{},
				castersThatCast: map[string]struct{}{},
				signaturesBuilt: map[string]struct{}{},
			}
			buckets[key] = b
		}
		return b
	}

	for _, row := range rows {
		phase := phaseForSecond(int(row.SecondsFromGameStart), earlyEnd, midEnd)
		if phase == "" {
			continue
		}
		switch row.ActionType {
		case "Train", "Unit Morph":
			for _, name := range commandUnitNamesFromPtrs(row.UnitType, row.UnitTypes) {
				if _, excluded := compositionExcluded[name]; excluded {
					continue
				}
				b := getBucket(row.PlayerID, phase)
				b.productionCount++
				if _, isCaster := compositionCasters[name]; isCaster {
					// Caster builds count toward the gate but show in the
					// strip only if a spell was actually cast — handled
					// by the cast branch below.
					continue
				}
				if _, isSig := compositionSignatureNonCasters[name]; isSig {
					b.signaturesBuilt[name] = struct{}{}
					continue
				}
				b.units[name]++
			}
		default:
			// Cast / Nuke command — OrderName is the discriminator.
			if row.OrderName == nil {
				continue
			}
			origin, ok := models.UnitOrderToUnit[*row.OrderName]
			if !ok {
				continue
			}
			if _, isCaster := compositionCasters[origin.Unit]; !isCaster {
				continue
			}
			b := getBucket(row.PlayerID, phase)
			b.castersThatCast[origin.Unit] = struct{}{}
		}
	}

	out := make([]workflowGameUnitComposition, 0, len(buckets))
	for key, b := range buckets {
		if b.productionCount == 0 {
			continue
		}
		out = append(out, workflowGameUnitComposition{
			PlayerID: key.playerID,
			Phase:    key.phase,
			Units:    sortUnitsDesc(b.units),
			Casters:  unionStrip(b.castersThatCast, b.signaturesBuilt),
		})
	}
	sortGameCompositionRows(out)
	return out
}

// phaseForSecond maps a replay second to "early" / "mid" / "late"
// given (earlyEnd, midEnd) boundaries. Either or both may be 0 when
// the corresponding signal didn't fire.
//
// Cases:
//   - both 0: no transition observed at all → whole game is "early".
//   - midEnd 0, earlyEnd > 0: tier-2 fired but no tier-3 → "early"
//     before earlyEnd, "mid" after. No late phase.
//   - earlyEnd 0, midEnd > 0: tier-3 fired without a tier-2 signal
//     (e.g. Protoss skipped Singularity Charge and went straight to
//     Carriers, with Terran on pure mech without Siege Mode). Split at
//     midEnd: pre-midEnd is "early", post-midEnd is "late". Skipping
//     "mid" is correct here — we never observed a tier-2 inflection,
//     so claiming "mid game starts" would be misleading.
//   - both > 0: standard early < earlyEnd ≤ mid < midEnd ≤ late.
func phaseForSecond(second, earlyEnd, midEnd int) string {
	if earlyEnd <= 0 && midEnd <= 0 {
		return "early"
	}
	if earlyEnd <= 0 {
		// Only tier-3 boundary observed. Everything before it is "early"
		// (we never confirmed tier-2), everything at-or-after is "late".
		if second < midEnd {
			return "early"
		}
		return "late"
	}
	if second < earlyEnd {
		return "early"
	}
	if midEnd <= 0 || second < midEnd {
		return "mid"
	}
	return "late"
}

// commandUnitNamesFromPtrs adapts the *string columns from the sqlc-
// generated row to the existing parseCommandUnitNames helper that takes
// sql.NullString.
func commandUnitNamesFromPtrs(unitType *string, unitTypes *string) []string {
	var n1, n2 sql.NullString
	if unitType != nil {
		n1 = sql.NullString{String: *unitType, Valid: true}
	}
	if unitTypes != nil {
		n2 = sql.NullString{String: *unitTypes, Valid: true}
	}
	return parseCommandUnitNames(n1, n2)
}

func sortUnitsDesc(counts map[string]int) []workflowUnitCompositionUnit {
	out := make([]workflowUnitCompositionUnit, 0, len(counts))
	for name, count := range counts {
		out = append(out, workflowUnitCompositionUnit{Name: name, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// unionStrip merges casters-that-cast and signature-non-casters into a
// single right-side strip, deduplicated and alphabetically sorted.
func unionStrip(a, b map[string]struct{}) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	merged := map[string]struct{}{}
	for k := range a {
		merged[k] = struct{}{}
	}
	for k := range b {
		merged[k] = struct{}{}
	}
	out := make([]string, 0, len(merged))
	for k := range merged {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortGameCompositionRows orders rows by (player_id, phase-rank) for a
// deterministic wire payload across requests.
func sortGameCompositionRows(rows []workflowGameUnitComposition) {
	rank := map[string]int{"early": 0, "mid": 1, "late": 2}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].PlayerID != rows[j].PlayerID {
			return rows[i].PlayerID < rows[j].PlayerID
		}
		return rank[rows[i].Phase] < rank[rows[j].Phase]
	})
}
