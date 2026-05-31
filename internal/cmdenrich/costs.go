package cmdenrich

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
)

// UnitEcon is the economic footprint of producing one instance of a Subject
// (a Train, Build, or Morph). It is the canonical answer for: "what does it
// cost, how long does it take, and how does it affect supply?"
//
// The data is keyed by canonical Subject string (e.g. "Pylon", "Probe",
// "Spawning Pool") — the same string the cmdenrich classifier emits as
// EnrichedCommand.Subject.
type UnitEcon struct {
	Minerals    int     // mineral cost
	Gas         int     // gas cost (0 for the early-game Tier-1 set we cover)
	BuildTimeS  float64 // build/train time in seconds at Fastest game speed
	SupplyDelta int     // +N for cap-increasers (Pylon/Depot/Overlord = +8); 0 otherwise
	SupplyCost  int     // population consumed when produced (1 for workers + most Tier-1 units; 0 for buildings)
}

// econTable is the source of truth for early-game economy. Extend as the
// filter's coverage grows beyond Tier-1.
//
// BuildTimeS references the canonical models.BuildTime* consts so the build
// times here can never drift from the build-order / expert-timing values in
// internal/models/build_times.go (enforced by a cross-consistency test).
var econTable = map[string]UnitEcon{
	// Protoss
	models.GeneralUnitPylon:           {Minerals: 100, BuildTimeS: models.BuildTimePylon, SupplyDelta: 8},
	models.GeneralUnitGateway:         {Minerals: 150, BuildTimeS: models.BuildTimeGateway},
	models.GeneralUnitAssimilator:     {Minerals: 100, BuildTimeS: models.BuildTimeAssimilator},
	models.GeneralUnitNexus:           {Minerals: 400, BuildTimeS: models.BuildTimeNexus},
	models.GeneralUnitForge:           {Minerals: 150, BuildTimeS: models.BuildTimeForge},
	models.GeneralUnitPhotonCannon:    {Minerals: 150, BuildTimeS: models.BuildTimePhotonCannon},
	models.GeneralUnitCyberneticsCore: {Minerals: 200, BuildTimeS: models.BuildTimeCyberneticsCore},
	models.GeneralUnitProbe:           {Minerals: 50, BuildTimeS: models.BuildTimeProbe, SupplyCost: 1},
	models.GeneralUnitZealot:          {Minerals: 100, BuildTimeS: models.BuildTimeZealot, SupplyCost: 2},

	// Terran
	models.GeneralUnitSupplyDepot:    {Minerals: 100, BuildTimeS: models.BuildTimeSupplyDepot, SupplyDelta: 8},
	models.GeneralUnitBarracks:       {Minerals: 150, BuildTimeS: models.BuildTimeBarracks},
	models.GeneralUnitRefinery:       {Minerals: 100, BuildTimeS: models.BuildTimeRefinery},
	models.GeneralUnitCommandCenter:  {Minerals: 400, BuildTimeS: models.BuildTimeCommandCenter},
	models.GeneralUnitEngineeringBay: {Minerals: 125, BuildTimeS: models.BuildTimeEngineeringBay},
	models.GeneralUnitFactory:        {Minerals: 200, BuildTimeS: models.BuildTimeFactory},
	models.GeneralUnitStarport:       {Minerals: 150, BuildTimeS: models.BuildTimeStarport},
	models.GeneralUnitMachineShop:    {Minerals: 50, BuildTimeS: models.BuildTimeMachineShop},
	models.GeneralUnitAcademy:        {Minerals: 150, BuildTimeS: models.BuildTimeAcademy},
	models.GeneralUnitBunker:         {Minerals: 100, BuildTimeS: models.BuildTimeBunker},
	models.GeneralUnitSCV:            {Minerals: 50, BuildTimeS: models.BuildTimeSCV, SupplyCost: 1},
	models.GeneralUnitMarine:         {Minerals: 50, BuildTimeS: models.BuildTimeMarine, SupplyCost: 1},

	// Zerg
	models.GeneralUnitOverlord:         {Minerals: 100, BuildTimeS: models.BuildTimeOverlord, SupplyDelta: 8},
	models.GeneralUnitSpawningPool:     {Minerals: 200, BuildTimeS: models.BuildTimeSpawningPool},
	models.GeneralUnitExtractor:        {Minerals: 50, BuildTimeS: models.BuildTimeExtractor},
	models.GeneralUnitHatchery:         {Minerals: 300, BuildTimeS: models.BuildTimeHatchery},
	models.GeneralUnitEvolutionChamber: {Minerals: 75, BuildTimeS: models.BuildTimeEvolutionChamber},
	models.GeneralUnitCreepColony:      {Minerals: 75, BuildTimeS: models.BuildTimeCreepColony},
	models.GeneralUnitSunkenColony:     {Minerals: 50, BuildTimeS: models.BuildTimeSunkenColony},
	models.GeneralUnitDrone:            {Minerals: 50, BuildTimeS: models.BuildTimeDrone, SupplyCost: 1},
	// Zerglings come in a pair from one Egg: 50m yields 2 lings, 2 supply. We
	// track them per-pair to match how the Train command appears in the replay
	// (one Morph command per pair); BuildTimeZergling is the per-pair morph time.
	models.GeneralUnitZergling: {Minerals: 50, BuildTimeS: models.BuildTimeZergling, SupplyCost: 2},
}

// EconOf returns the economic footprint for a Subject. Returns (zero, false)
// if the Subject is not in the table — callers should treat that as
// "unknown / pass-through" rather than free.
func EconOf(subject string) (UnitEcon, bool) {
	e, ok := econTable[subject]
	return e, ok
}

// gatherRates is the steady-state per-worker per-minute mineral income at a
// near base, sourced from Liquipedia. Values match the user spec.
var gatherRates = map[string]float64{
	models.GeneralUnitSCV:   65.0,
	models.GeneralUnitDrone: 67.1,
	models.GeneralUnitProbe: 68.1,
}

// GatherRatePerMinute returns the per-worker mineral gather rate for a worker
// Subject (SCV / Drone / Probe). Returns 0 for non-worker subjects.
func GatherRatePerMinute(workerSubject string) float64 {
	return gatherRates[workerSubject]
}

// EconEntry is one row of the early-game economy table (Subject + footprint).
type EconEntry struct {
	Subject string
	Econ    UnitEcon
}

// AllEcon returns every early-game economy entry, sorted by Subject. Used by
// the SPECIFICATION.md generator and cross-consistency tests.
func AllEcon() []EconEntry {
	out := make([]EconEntry, 0, len(econTable))
	for subject, econ := range econTable {
		out = append(out, EconEntry{Subject: subject, Econ: econ})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Subject < out[j].Subject })
	return out
}

// GatherRateEntry is one row of the worker gather-rate table.
type GatherRateEntry struct {
	Worker         string
	MineralsPerMin float64
}

// AllGatherRates returns the per-worker mineral gather rates, sorted by worker
// name. Used by the SPECIFICATION.md generator and cross-consistency tests.
func AllGatherRates() []GatherRateEntry {
	out := make([]GatherRateEntry, 0, len(gatherRates))
	for worker, rate := range gatherRates {
		out = append(out, GatherRateEntry{Worker: worker, MineralsPerMin: rate})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Worker < out[j].Worker })
	return out
}
