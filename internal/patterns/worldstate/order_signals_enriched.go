package worldstate

import (
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// EnrichedCommand-shaped attack-pressure classifiers. Mirror of the
// existing attackOpeningPressure / attackSustainAfterOpen here, but
// consume cmdenrich.EnrichedCommand directly. Used by attacks_pass.go.
//
// Same semantics: AttackMove, AttackUnit, Patrol, Hold, and aggressive
// Casts open pressure ranges. Right-click alone never opens (only sustains).

// AttackOpeningPressure reports whether this enriched command counts
// toward starting an attack/scout pressure range at an enemy base.
func AttackOpeningPressure(ec cmdenrich.EnrichedCommand) bool {
	switch ec.Kind {
	case cmdenrich.KindRightClick:
		return false
	case cmdenrich.KindAttackMove, cmdenrich.KindAttackUnit:
		return true
	case cmdenrich.KindPatrol, cmdenrich.KindHold:
		return true
	case cmdenrich.KindCast:
		return castIsAggressive(ec.OrderName)
	case cmdenrich.KindMove:
		return false
	}
	return false
}

// AttackSustainAfterOpen extends an open attack range: opening-class
// commands or any right-click while in enemy territory.
func AttackSustainAfterOpen(ec cmdenrich.EnrichedCommand) bool {
	if AttackOpeningPressure(ec) {
		return true
	}
	return ec.Kind == cmdenrich.KindRightClick
}

// castIsAggressive mirrors the donor's classifyCastByName. Returns false
// for utility casts (Restoration, Hallucination, Recall, ScannerSweep,
// DefensiveMatrix). Returns true for the standard offensive casts and
// for the nuke-launch family.
func castIsAggressive(orderName string) bool {
	o := normalizeCastKey(orderName)
	if o == "" {
		return false
	}
	if strings.Contains(o, "scanner") ||
		strings.Contains(o, "defensivematrix") ||
		strings.Contains(o, "restoration") ||
		strings.Contains(o, "hallucination") ||
		strings.Contains(o, "recall") {
		return false
	}
	if strings.HasPrefix(o, "cast") {
		return true
	}
	if strings.Contains(o, "psionicstorm") ||
		strings.Contains(o, "sieging") ||
		strings.Contains(o, "fireyamato") ||
		strings.Contains(o, "nukelaunch") ||
		strings.Contains(o, "nuclearstrike") {
		return true
	}
	return false
}

func normalizeCastKey(s string) string {
	x := strings.ToLower(s)
	x = strings.ReplaceAll(x, " ", "")
	x = strings.ReplaceAll(x, "_", "")
	return x
}
