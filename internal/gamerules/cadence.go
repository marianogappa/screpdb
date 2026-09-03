package gamerules

// Unit cadence analysis window and thresholds. The window starts once the
// opening is over and stops before the endgame, where production collapses
// for reasons unrelated to macro habits.
const (
	UnitCadenceStartSeconds      = 7 * 60
	UnitCadenceEndFraction       = 0.8
	UnitCadenceIdleGapSeconds    = 20
	UnitCadenceMinUnitsPerReplay = 12
	UnitCadenceMinGapsPerReplay  = 8
)

// UnitCadenceExcludedUnits lists the unit types that do not count as army
// production for cadence: workers, supply, and support casters whose
// production rhythm says nothing about macro.
var UnitCadenceExcludedUnits = []string{
	"SCV",
	"Probe",
	"Drone",
	"Overlord",
	"Observer",
	"Shuttle",
	"Science Vessel",
	"Medic",
	"Dropship",
	"Defiler",
	"Queen",
	"Nuclear Missile",
}
