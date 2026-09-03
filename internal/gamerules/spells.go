// Package gamerules holds game-knowledge constants shared by replay compaction
// and the dashboard so the two cannot drift.
package gamerules

import "github.com/marianogappa/screpdb/internal/models"

// CompositionSpell ties an ability OrderName (a 'Targeted Order' command:
// Cast*, FireYamatoGun, PlaceMine, ...) to the unit that casts it and the
// human-readable spell name.
type CompositionSpell struct {
	Order string
	Unit  string
	Spell string
}

// CompositionSpells is the single source of truth for what counts as a
// spellcast. Only meaningful player abilities are listed: unit morphs (Archon
// warp, Dark Archon meld, Guardian aspect), passives (Arbiter cloak),
// continuous Medic heal, Comsat scans and all Nuke orders are excluded (the
// Nuke has its own Featuring pill). Stasis Field is attributed to the Arbiter
// even though UnitOrderToUnit ties the order to the Science Vessel, because
// the Arbiter is the unit that actually casts it.
var CompositionSpells = []CompositionSpell{
	{models.UnitOrderCastPsionicStorm, models.GeneralUnitHighTemplar, "Psionic Storm"},
	{models.UnitOrderCastHallucination, models.GeneralUnitHighTemplar, "Hallucination"},
	{models.UnitOrderHallucination2, models.GeneralUnitHighTemplar, "Hallucination"},

	{models.UnitOrderFireYamatoGun, models.GeneralUnitBattlecruiser, "Yamato Gun"},

	{models.UnitOrderVultureMine, models.GeneralUnitVulture, "Spider Mine"},
	{models.UnitOrderPlaceMine, models.GeneralUnitVulture, "Spider Mine"},

	{models.UnitOrderCastLockdown, models.GeneralUnitGhost, "Lockdown"},

	{models.UnitOrderCastDarkSwarm, models.GeneralUnitDefiler, "Dark Swarm"},
	{models.UnitOrderCastPlague, models.GeneralUnitDefiler, "Plague"},
	{models.UnitOrderCastConsume, models.GeneralUnitDefiler, "Consume"},

	{models.UnitOrderCastEMPShockwave, models.GeneralUnitScienceVessel, "EMP Shockwave"},
	{models.UnitOrderCastIrradiate, models.GeneralUnitScienceVessel, "Irradiate"},
	{models.UnitOrderCastDefensiveMatrix, models.GeneralUnitScienceVessel, "Defensive Matrix"},

	{models.UnitOrderCastStasisField, models.GeneralUnitArbiter, "Stasis Field"},
	{models.UnitOrderCastRecall, models.GeneralUnitArbiter, "Recall"},

	{models.UnitOrderCastDisruptionWeb, models.GeneralUnitCorsair, "Disruption Web"},

	{models.UnitOrderCastMindControl, models.GeneralUnitDarkArchon, "Mind Control"},
	{models.UnitOrderCastFeedback, models.GeneralUnitDarkArchon, "Feedback"},
	{models.UnitOrderCastMaelstrom, models.GeneralUnitDarkArchon, "Maelstrom"},

	{models.UnitOrderCastParasite, models.GeneralUnitQueen, "Parasite"},
	{models.UnitOrderCastSpawnBroodlings, models.GeneralUnitQueen, "Spawn Broodlings"},
	{models.UnitOrderCastEnsnare, models.GeneralUnitQueen, "Ensnare"},
	{models.UnitOrderCastInfestation, models.GeneralUnitQueen, "Infest Command Center"},

	{models.UnitOrderCastRestoration, models.GeneralUnitMedic, "Restoration"},
	{models.UnitOrderCastOpticalFlare, models.GeneralUnitMedic, "Optical Flare"},
}

var compositionSpellOrders = func() map[string]struct{} {
	set := make(map[string]struct{}, len(CompositionSpells))
	for _, s := range CompositionSpells {
		set[s.Order] = struct{}{}
	}
	return set
}()

// IsCompositionSpellOrder reports whether a Targeted Order name is one of the
// spellcasts kept for unit composition and production analysis.
func IsCompositionSpellOrder(orderName string) bool {
	_, ok := compositionSpellOrders[orderName]
	return ok
}
