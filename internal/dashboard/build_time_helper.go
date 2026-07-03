package dashboard

import "github.com/marianogappa/screpdb/internal/models"

// unitBuildTimesSec maps GeneralUnit* names to their build/morph durations in
// in-game seconds at "Fastest" game speed (the only speed used by competitive
// replays). Sources: Liquipedia BW unit data + models.BuildTime* constants
// where available.
//
// Used by the dashboard's trained-units overlay to compute "alive at second
// X" by shifting Train/Unit-Morph command timestamps forward by the unit's
// build time. Approximate by design — production cancellation, deaths, and
// morph source-deduction are not tracked.
var unitBuildTimesSec = map[string]float64{
	// Terran
	models.GeneralUnitMarine:                   24,
	models.GeneralUnitFirebat:                  24,
	models.GeneralUnitGhost:                    50,
	models.GeneralUnitMedic:                    30,
	models.GeneralUnitVulture:                  30,
	models.GeneralUnitGoliath:                  40,
	models.GeneralUnitSiegeTankTankMode:        50,
	models.GeneralUnitTerranSiegeTankSiegeMode: 50,
	models.GeneralUnitSCV:                      20,
	models.GeneralUnitWraith:                   60,
	models.GeneralUnitScienceVessel:            80,
	models.GeneralUnitDropship:                 50,
	models.GeneralUnitBattlecruiser:            133,
	models.GeneralUnitValkyrie:                 50,
	// Protoss
	models.GeneralUnitProbe:       20,
	models.GeneralUnitZealot:      models.BuildTimeZealot,
	models.GeneralUnitDragoon:     50,
	models.GeneralUnitHighTemplar: 50,
	models.GeneralUnitDarkTemplar: 50,
	models.GeneralUnitArchon:      20,
	models.GeneralUnitDarkArchon:  20,
	models.GeneralUnitShuttle:     60,
	models.GeneralUnitReaver:      70,
	models.GeneralUnitObserver:    40,
	models.GeneralUnitScout:       80,
	models.GeneralUnitCorsair:     40,
	models.GeneralUnitCarrier:     140,
	models.GeneralUnitArbiter:     160,
	// Zerg
	models.GeneralUnitDrone:     models.BuildTimeDrone,
	models.GeneralUnitZergling:  models.BuildTimeZergling,
	models.GeneralUnitHydralisk: 28,
	models.GeneralUnitLurker:    40,
	models.GeneralUnitUltralisk: 60,
	models.GeneralUnitDefiler:   50,
	models.GeneralUnitQueen:     50,
	models.GeneralUnitMutalisk:  models.BuildTimeMutalisk,
	models.GeneralUnitScourge:   30,
	models.GeneralUnitGuardian:  40,
	models.GeneralUnitDevourer:  40,
	models.GeneralUnitOverlord:  models.BuildTimeOverlord,
}

// buildTimeFor returns the build/morph duration in seconds for the named
// unit at Fastest game speed. Returns 0 for unrecognized names — callers
// should treat 0 as "count at command time" (over-counts slightly in the
// seconds just before completion).
func buildTimeFor(unitName string) float64 {
	if s, ok := unitBuildTimesSec[unitName]; ok {
		return s
	}
	return 0
}
