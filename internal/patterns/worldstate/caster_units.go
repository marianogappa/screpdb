package worldstate

import "github.com/marianogappa/screpdb/internal/models"

// casterUnitForCast resolves a raw cast OrderName (e.g. "CastPsionicStorm",
// "CastRecall", "NukeLaunch") to the unit that issued the cast. Returns
// false when the order isn't a known cast → unit mapping.
//
// Used to derive ground-truth unit presence inside an attack window from
// cast evidence, since a cast at (x, y, t) proves the caster unit existed
// at that moment — stronger than the build/train history proxy.
func casterUnitForCast(orderName string) (string, bool) {
	if orderName == "" {
		return "", false
	}
	if u, ok := models.UnitOrderToUnit[orderName]; ok && u.Unit != "" {
		return u.Unit, true
	}
	return "", false
}
