package cmdenrich

import "github.com/marianogappa/screpdb/internal/models"

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
var econTable = map[string]UnitEcon{
	// Protoss
	models.GeneralUnitPylon:           {Minerals: 100, BuildTimeS: 19, SupplyDelta: 8},
	models.GeneralUnitGateway:         {Minerals: 150, BuildTimeS: 38},
	models.GeneralUnitAssimilator:     {Minerals: 100, BuildTimeS: 25},
	models.GeneralUnitNexus:           {Minerals: 400, BuildTimeS: 75},
	models.GeneralUnitForge:           {Minerals: 150, BuildTimeS: 25},
	models.GeneralUnitPhotonCannon:    {Minerals: 150, BuildTimeS: 31.5},
	models.GeneralUnitCyberneticsCore: {Minerals: 200, BuildTimeS: 38},
	models.GeneralUnitProbe:           {Minerals: 50, BuildTimeS: 12.6, SupplyCost: 1},
	models.GeneralUnitZealot:          {Minerals: 100, BuildTimeS: 25, SupplyCost: 2},

	// Terran
	models.GeneralUnitSupplyDepot:    {Minerals: 100, BuildTimeS: 25, SupplyDelta: 8},
	models.GeneralUnitBarracks:       {Minerals: 150, BuildTimeS: 50},
	models.GeneralUnitRefinery:       {Minerals: 100, BuildTimeS: 25},
	models.GeneralUnitCommandCenter:  {Minerals: 400, BuildTimeS: 75},
	models.GeneralUnitEngineeringBay: {Minerals: 125, BuildTimeS: 38},
	models.GeneralUnitFactory:        {Minerals: 200, BuildTimeS: 50},
	models.GeneralUnitStarport:       {Minerals: 150, BuildTimeS: 44},
	models.GeneralUnitMachineShop:    {Minerals: 50, BuildTimeS: 25},
	models.GeneralUnitAcademy:        {Minerals: 150, BuildTimeS: 50},
	models.GeneralUnitBunker:         {Minerals: 100, BuildTimeS: 19},
	models.GeneralUnitSCV:            {Minerals: 50, BuildTimeS: 12.6, SupplyCost: 1},
	models.GeneralUnitMarine:         {Minerals: 50, BuildTimeS: 15, SupplyCost: 1},

	// Zerg
	models.GeneralUnitOverlord:         {Minerals: 100, BuildTimeS: 25, SupplyDelta: 8},
	models.GeneralUnitSpawningPool:     {Minerals: 200, BuildTimeS: 50},
	models.GeneralUnitExtractor:        {Minerals: 50, BuildTimeS: 25},
	models.GeneralUnitHatchery:         {Minerals: 300, BuildTimeS: 75},
	models.GeneralUnitEvolutionChamber: {Minerals: 75, BuildTimeS: 25},
	models.GeneralUnitCreepColony:      {Minerals: 75, BuildTimeS: 12},
	models.GeneralUnitSunkenColony:     {Minerals: 50, BuildTimeS: 12},
	models.GeneralUnitDrone:            {Minerals: 50, BuildTimeS: 12.6, SupplyCost: 1},
	// Zerglings come in a pair from one Egg: 50m/17.1s yields 2 lings, 2 supply.
	// We track them per-pair to match how the Train command appears in the replay
	// (one Morph command per pair).
	models.GeneralUnitZergling: {Minerals: 50, BuildTimeS: 17.1, SupplyCost: 2},
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
