package models

// Build times (seconds) at SC:BW "Fastest" game speed — the speed every
// competitive replay is played on. These are the canonical in-game values
// used by timing-based logic (e.g. build-order detection and expert timings).
//
// Values are float64 because some are fractional (e.g. Zealot 25.2s). Round
// to int at the call site when combining with integer game seconds.
const (
	BuildTimeSpawningPool float64 = 50
	BuildTimeGateway      float64 = 38
	BuildTimeZealot       float64 = 25.2
	BuildTimeHatchery     float64 = 75
	BuildTimeNexus        float64 = 75
	BuildTimeForge        float64 = 25
	BuildTimeOverlord     float64 = 25
	BuildTimeDrone        float64 = 12.6
	BuildTimeZergling     float64 = 18
)
