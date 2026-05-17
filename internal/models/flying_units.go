package models

// flyingUnits enumerates units that cannot be loaded into transports. Used by
// the worldstate drop detector to exclude flyers from the participant unit
// estimate it surfaces on drop events.
var flyingUnits = map[string]struct{}{
	// Terran
	GeneralUnitWraith:        {},
	GeneralUnitScienceVessel: {},
	GeneralUnitDropship:      {},
	GeneralUnitValkyrie:      {},
	GeneralUnitBattlecruiser: {},
	// Zerg
	GeneralUnitOverlord:      {},
	GeneralUnitMutalisk:      {},
	GeneralUnitMutaliskCocoon: {},
	GeneralUnitGuardian:      {},
	GeneralUnitDevourer:      {},
	GeneralUnitScourge:       {},
	GeneralUnitQueen:         {},
	// Protoss
	GeneralUnitShuttle:  {},
	GeneralUnitObserver: {},
	GeneralUnitScout:    {},
	GeneralUnitCorsair:  {},
	GeneralUnitCarrier:  {},
	GeneralUnitArbiter:  {},
}

// IsFlyingUnit reports whether a unit type is a flyer (cannot be loaded into
// a transport).
func IsFlyingUnit(name string) bool {
	_, ok := flyingUnits[name]
	return ok
}
