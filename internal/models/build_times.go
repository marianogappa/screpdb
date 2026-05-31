package models

import "sort"

// Build times (seconds) at SC:BW "Fastest" game speed — the speed every
// competitive replay is played on. These are the canonical in-game values
// used by timing-based logic (e.g. build-order detection and expert timings)
// AND by the early-game economy table (internal/cmdenrich/costs.go), which
// references these consts so the two can never drift apart.
//
// Values are float64 because some are fractional (e.g. Zealot 25.2s). Round
// to int at the call site when combining with integer game seconds.
//
// Source: https://liquipedia.net/starcraft — Fastest game speed.
const (
	// Zerg
	BuildTimeSpawningPool     float64 = 50
	BuildTimeHatchery         float64 = 75
	BuildTimeExtractor        float64 = 25
	BuildTimeEvolutionChamber float64 = 25
	BuildTimeCreepColony      float64 = 12
	BuildTimeSunkenColony     float64 = 12
	BuildTimeSpire            float64 = 75
	BuildTimeOverlord         float64 = 25
	BuildTimeDrone            float64 = 12.6
	// BuildTimeZergling is per Zergling-pair (one Egg morphs into two lings).
	BuildTimeZergling float64 = 18
	BuildTimeMutalisk float64 = 25

	// Protoss
	BuildTimeNexus           float64 = 75
	BuildTimePylon           float64 = 19
	BuildTimeGateway         float64 = 38
	BuildTimeAssimilator     float64 = 25
	BuildTimeForge           float64 = 25
	BuildTimePhotonCannon    float64 = 31.5
	BuildTimeCyberneticsCore float64 = 38
	BuildTimeProbe           float64 = 12.6
	BuildTimeZealot          float64 = 25.2

	// Terran
	BuildTimeCommandCenter  float64 = 75
	BuildTimeSupplyDepot    float64 = 25
	BuildTimeBarracks       float64 = 50
	BuildTimeRefinery       float64 = 25
	BuildTimeEngineeringBay float64 = 38
	BuildTimeFactory        float64 = 50
	BuildTimeStarport       float64 = 44
	BuildTimeMachineShop    float64 = 25
	BuildTimeAcademy        float64 = 50
	BuildTimeBunker         float64 = 19
	BuildTimeMissileTurret  float64 = 18.9
	BuildTimeSCV            float64 = 12.6
	BuildTimeMarine         float64 = 15
)

// buildTimes is the canonical name → build-time index over the BuildTime*
// consts above. Keyed by the GeneralUnit* canonical name so callers and the
// SPECIFICATION.md generator can look up by the same string the replay emits.
var buildTimes = map[string]float64{
	// Zerg
	GeneralUnitSpawningPool:     BuildTimeSpawningPool,
	GeneralUnitHatchery:         BuildTimeHatchery,
	GeneralUnitExtractor:        BuildTimeExtractor,
	GeneralUnitEvolutionChamber: BuildTimeEvolutionChamber,
	GeneralUnitCreepColony:      BuildTimeCreepColony,
	GeneralUnitSunkenColony:     BuildTimeSunkenColony,
	GeneralUnitSpire:            BuildTimeSpire,
	GeneralUnitOverlord:         BuildTimeOverlord,
	GeneralUnitDrone:            BuildTimeDrone,
	GeneralUnitZergling:         BuildTimeZergling,
	GeneralUnitMutalisk:         BuildTimeMutalisk,

	// Protoss
	GeneralUnitNexus:           BuildTimeNexus,
	GeneralUnitPylon:           BuildTimePylon,
	GeneralUnitGateway:         BuildTimeGateway,
	GeneralUnitAssimilator:     BuildTimeAssimilator,
	GeneralUnitForge:           BuildTimeForge,
	GeneralUnitPhotonCannon:    BuildTimePhotonCannon,
	GeneralUnitCyberneticsCore: BuildTimeCyberneticsCore,
	GeneralUnitProbe:           BuildTimeProbe,
	GeneralUnitZealot:          BuildTimeZealot,

	// Terran
	GeneralUnitCommandCenter:  BuildTimeCommandCenter,
	GeneralUnitSupplyDepot:    BuildTimeSupplyDepot,
	GeneralUnitBarracks:       BuildTimeBarracks,
	GeneralUnitRefinery:       BuildTimeRefinery,
	GeneralUnitEngineeringBay: BuildTimeEngineeringBay,
	GeneralUnitFactory:        BuildTimeFactory,
	GeneralUnitStarport:       BuildTimeStarport,
	GeneralUnitMachineShop:    BuildTimeMachineShop,
	GeneralUnitAcademy:        BuildTimeAcademy,
	GeneralUnitBunker:         BuildTimeBunker,
	GeneralUnitMissileTurret:  BuildTimeMissileTurret,
	GeneralUnitSCV:            BuildTimeSCV,
	GeneralUnitMarine:         BuildTimeMarine,
}

// BuildTimeOf returns the canonical build time (seconds, Fastest speed) for a
// unit/building by its canonical GeneralUnit* name. The bool is false for
// names not in the table — callers should treat that as "unknown".
func BuildTimeOf(name string) (float64, bool) {
	t, ok := buildTimes[name]
	return t, ok
}

// BuildTimeEntry is one row of the canonical build-time table.
type BuildTimeEntry struct {
	Name    string
	Seconds float64
}

// AllBuildTimes returns every canonical build time, sorted by name. Used by the
// SPECIFICATION.md generator and cross-consistency tests.
func AllBuildTimes() []BuildTimeEntry {
	out := make([]BuildTimeEntry, 0, len(buildTimes))
	for name, sec := range buildTimes {
		out = append(out, BuildTimeEntry{Name: name, Seconds: sec})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
