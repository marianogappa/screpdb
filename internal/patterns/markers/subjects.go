package markers

import (
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
)

// subjectsOfInterest is the set of canonical unit/building names every marker
// (that's subject-gated) cares about. The detector filters EnrichedCommands
// through IsSubjectOfInterest before dispatching Build/Produce facts to
// predicate state — so downstream predicates only see subjects that could
// affect some registered marker.
//
// Upgrade / Tech facts bypass this gate (their subject is an upgrade or
// tech name, not a unit/building — see the detector logic).
var subjectsOfInterest = map[string]struct{}{
	// Zerg
	models.GeneralUnitSpawningPool:     {},
	models.GeneralUnitHatchery:         {},
	models.GeneralUnitEvolutionChamber: {},
	models.GeneralUnitDrone:            {},
	models.GeneralUnitOverlord:         {},
	models.GeneralUnitZergling:         {},
	// Protoss
	models.GeneralUnitNexus:           {},
	models.GeneralUnitPylon:           {},
	models.GeneralUnitGateway:         {},
	models.GeneralUnitAssimilator:     {},
	models.GeneralUnitCyberneticsCore: {},
	models.GeneralUnitForge:           {},
	models.GeneralUnitPhotonCannon:    {},
	models.GeneralUnitZealot:          {},
	models.GeneralUnitScout:           {},
	models.GeneralUnitCarrier:         {},
	// Terran
	models.GeneralUnitCommandCenter:        {},
	models.GeneralUnitSupplyDepot:          {},
	models.GeneralUnitBarracks:             {},
	models.GeneralUnitRefinery:             {},
	models.GeneralUnitAcademy:              {},
	models.GeneralUnitFactory:              {},
	models.GeneralUnitStarport:             {},
	models.GeneralUnitEngineeringBay:       {},
	models.GeneralUnitBunker:               {},
	models.GeneralUnitMedic:                {},
	models.GeneralUnitVulture:              {},
	models.GeneralUnitGoliath:              {},
	models.GeneralUnitSiegeTankTankMode:    {},
	models.GeneralUnitBattlecruiser:        {},
}

// IsSubjectOfInterest reports whether a canonical unit/building name matters
// for any registered marker.
func IsSubjectOfInterest(subject string) bool {
	_, ok := subjectsOfInterest[strings.TrimSpace(subject)]
	return ok
}
