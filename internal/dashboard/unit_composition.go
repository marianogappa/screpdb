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
// produces a flat list of (player, phase, units, spells) entries. The
// frontend renders per-player rows on each player strip and aggregates
// client-side into 3 replay-level bars for the per-game summary
// surface. Counts are sent raw — the frontend does the proportional
// fill on render so we don't lock into a presentation shape on the wire.
//
// Why request-time, not ingest-time: the rules (caster set, spell map,
// non-army units, excluded units) iterate without re-ingest. Persisting
// the histogram on every replay would lock those rules at ingest and
// force re-detection passes to update.

// compositionCasters: spellcaster units. Kept out of the Units
// histogram (their presence is captured via the spells they cast, not
// their headcount). Keep in sync with compositionSpells below.
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

// compositionNonArmy: units kept out of the Units histogram because they
// don't read as bulk army composition — transports (Dropship/Shuttle)
// and the Nuke.
//
// Battlecruiser is intentionally NOT here: it's primarily an attacking
// unit, so it always counts in the Units histogram (and additionally
// surfaces its Yamato Gun under Spellcasts when cast — a unit can appear
// in both, like the Vulture and its Spider Mine). Carrier, Reaver, Dark
// Templar, Guardian and Devourer stay in the histogram for the same
// reason.
var compositionNonArmy = map[string]struct{}{
	models.GeneralUnitDropship:       {},
	models.GeneralUnitShuttle:        {},
	models.GeneralUnitNuclearMissile: {},
}

// compositionSpell pairs the casting unit (for its icon) with the
// spell's display name. The same unit appears under multiple spells.
type compositionSpell struct {
	unit  string
	spell string
}

// compositionSpells maps an ability OrderName (a 'Targeted Order' command
// — Cast*, FireYamatoGun, PlaceMine, ...) to the (unit, spell) it
// represents. This map is the single source of truth for what counts as a
// spellcast: the SQL returns every Targeted Order and unmapped ones are
// dropped here, so adding/removing a cast is a one-line change with no SQL
// edit. Only meaningful player abilities are listed — unit morphs (Archon
// warp, Dark Archon meld, Guardian aspect), passives (Arbiter cloak),
// continuous Medic heal, Comsat scans and all Nuke orders are excluded
// (the Nuke has its own Featuring pill). Stasis Field is attributed to the
// Arbiter even though UnitOrderToUnit ties the order to the Science Vessel
// — the Arbiter is the unit that actually casts it.
var compositionSpells = map[string]compositionSpell{
	models.UnitOrderCastPsionicStorm:  {models.GeneralUnitHighTemplar, "Psionic Storm"},
	models.UnitOrderCastHallucination: {models.GeneralUnitHighTemplar, "Hallucination"},
	models.UnitOrderHallucination2:    {models.GeneralUnitHighTemplar, "Hallucination"},

	models.UnitOrderFireYamatoGun: {models.GeneralUnitBattlecruiser, "Yamato Gun"},

	models.UnitOrderVultureMine: {models.GeneralUnitVulture, "Spider Mine"},
	models.UnitOrderPlaceMine:   {models.GeneralUnitVulture, "Spider Mine"},

	models.UnitOrderCastLockdown: {models.GeneralUnitGhost, "Lockdown"},

	models.UnitOrderCastDarkSwarm: {models.GeneralUnitDefiler, "Dark Swarm"},
	models.UnitOrderCastPlague:    {models.GeneralUnitDefiler, "Plague"},
	models.UnitOrderCastConsume:   {models.GeneralUnitDefiler, "Consume"},

	models.UnitOrderCastEMPShockwave:    {models.GeneralUnitScienceVessel, "EMP Shockwave"},
	models.UnitOrderCastIrradiate:       {models.GeneralUnitScienceVessel, "Irradiate"},
	models.UnitOrderCastDefensiveMatrix: {models.GeneralUnitScienceVessel, "Defensive Matrix"},

	models.UnitOrderCastStasisField: {models.GeneralUnitArbiter, "Stasis Field"},
	models.UnitOrderCastRecall:      {models.GeneralUnitArbiter, "Recall"},

	models.UnitOrderCastDisruptionWeb: {models.GeneralUnitCorsair, "Disruption Web"},

	models.UnitOrderCastMindControl: {models.GeneralUnitDarkArchon, "Mind Control"},
	models.UnitOrderCastFeedback:    {models.GeneralUnitDarkArchon, "Feedback"},
	models.UnitOrderCastMaelstrom:   {models.GeneralUnitDarkArchon, "Maelstrom"},

	models.UnitOrderCastParasite:        {models.GeneralUnitQueen, "Parasite"},
	models.UnitOrderCastSpawnBroodlings: {models.GeneralUnitQueen, "Spawn Broodlings"},
	models.UnitOrderCastEnsnare:         {models.GeneralUnitQueen, "Ensnare"},
	models.UnitOrderCastInfestation:     {models.GeneralUnitQueen, "Infest Command Center"},

	models.UnitOrderCastRestoration:  {models.GeneralUnitMedic, "Restoration"},
	models.UnitOrderCastOpticalFlare: {models.GeneralUnitMedic, "Optical Flare"},
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
		spells          map[compositionSpell]struct{}
		productionCount int
	}
	buckets := map[accumKey]*accum{}
	getBucket := func(playerID int64, phase string) *accum {
		key := accumKey{playerID, phase}
		b, ok := buckets[key]
		if !ok {
			b = &accum{
				units:  map[string]int{},
				spells: map[compositionSpell]struct{}{},
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
					// Casters stay out of the histogram — captured via the
					// spells they cast (cast branch below), not headcount.
					continue
				}
				if _, isNonArmy := compositionNonArmy[name]; isNonArmy {
					// Transports / Nuke / Battlecruiser: not army composition.
					// BC surfaces only via its Yamato Gun spell.
					continue
				}
				b.units[name]++
			}
		default:
			// Cast / Nuke command — OrderName is the discriminator.
			if row.OrderName == nil {
				continue
			}
			spell, ok := compositionSpells[*row.OrderName]
			if !ok {
				continue
			}
			b := getBucket(row.PlayerID, phase)
			b.spells[spell] = struct{}{}
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
			Spells:   sortSpells(b.spells),
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

// sortSpells flattens the distinct (unit, spell) set into a stable list
// ordered by unit then spell, so the same caster's spells group together.
func sortSpells(spells map[compositionSpell]struct{}) []workflowUnitCompositionSpell {
	if len(spells) == 0 {
		return nil
	}
	out := make([]workflowUnitCompositionSpell, 0, len(spells))
	for s := range spells {
		out = append(out, workflowUnitCompositionSpell{Unit: s.unit, Spell: s.spell})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Unit != out[j].Unit {
			return out[i].Unit < out[j].Unit
		}
		return out[i].Spell < out[j].Spell
	})
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
