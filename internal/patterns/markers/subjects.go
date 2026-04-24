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
	models.GeneralUnitNexus:   {},
	models.GeneralUnitGateway: {},
	models.GeneralUnitForge:   {},
	models.GeneralUnitZealot:  {},
	models.GeneralUnitCarrier: {},
	// Terran
	models.GeneralUnitFactory:       {},
	models.GeneralUnitBattlecruiser: {},
}

// IsSubjectOfInterest reports whether a canonical unit/building name matters
// for any registered marker.
func IsSubjectOfInterest(subject string) bool {
	_, ok := subjectsOfInterest[strings.TrimSpace(subject)]
	return ok
}
