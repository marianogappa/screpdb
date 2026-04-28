package cmdenrich

import "github.com/marianogappa/screpdb/internal/models"

// producerByUnit answers "what building produces this unit?" for the early-game
// units the spam filter cares about. The StarCraft engine refuses to execute a
// Train order without its producer, so a kept Train command is strong evidence
// the producer existed.
var producerByUnit = map[string]string{
	// Protoss
	models.GeneralUnitZealot: models.GeneralUnitGateway,
	// Terran
	models.GeneralUnitMarine: models.GeneralUnitBarracks,
	// Zerg larva morphs come from Hatchery/Lair/Hive — workers and the
	// foundational Zergling have Hatchery as the minimum producer.
	models.GeneralUnitDrone:    models.GeneralUnitHatchery,
	models.GeneralUnitZergling: models.GeneralUnitSpawningPool,
	models.GeneralUnitOverlord: models.GeneralUnitHatchery,
	// Workers per resource centre
	models.GeneralUnitProbe: models.GeneralUnitNexus,
	models.GeneralUnitSCV:   models.GeneralUnitCommandCenter,
}

// ProducerOf returns the canonical producer building for a unit Subject.
// Returns ("", false) if the Subject isn't a tracked produced unit.
func ProducerOf(unitSubject string) (string, bool) {
	p, ok := producerByUnit[unitSubject]
	return p, ok
}

// prereqsByBuilding lists the Build-time prerequisites for a building, in
// addition to the producer. Pylon power for Protoss is the canonical case:
// no Gateway can warp in without an existing Pylon. Tech-tree children
// (Barracks → Academy, etc.) extend this map as the filter grows.
var prereqsByBuilding = map[string][]string{
	// Protoss buildings need Pylon for power. Photon Cannon also needs Forge.
	// Cybernetics Core needs Gateway. Robotics/Stargate gates not modelled
	// (out of 4-min window).
	models.GeneralUnitGateway:         {models.GeneralUnitPylon},
	models.GeneralUnitForge:           {models.GeneralUnitPylon},
	models.GeneralUnitAssimilator:     {models.GeneralUnitNexus},
	models.GeneralUnitPhotonCannon:    {models.GeneralUnitPylon, models.GeneralUnitForge},
	models.GeneralUnitCyberneticsCore: {models.GeneralUnitPylon, models.GeneralUnitGateway},

	// Terran tech tree
	models.GeneralUnitFactory:     {models.GeneralUnitBarracks},
	models.GeneralUnitStarport:    {models.GeneralUnitFactory},
	models.GeneralUnitAcademy:     {models.GeneralUnitBarracks},
	models.GeneralUnitBunker:      {models.GeneralUnitBarracks},
	models.GeneralUnitMachineShop: {models.GeneralUnitFactory},

	// Zerg tech tree (Hatchery is auto-completed at sim init)
	models.GeneralUnitSunkenColony: {models.GeneralUnitCreepColony, models.GeneralUnitSpawningPool},
}

// PrereqsOf returns the buildings that must already exist for the given
// building Subject to be placed. The producer is implicit and not repeated
// here. Returns (nil, false) if the Subject has no recorded prerequisites.
func PrereqsOf(buildingSubject string) ([]string, bool) {
	p, ok := prereqsByBuilding[buildingSubject]
	return p, ok
}

// workerSubjects is the set of worker units across races. Workers are the
// dominant source of fake early-game mineral spend (queued workers that never
// build because the queue overflows or the producer is busy), so the spam
// filter treats them as the first thing to drop when reconciling minerals
// during backtracking.
var workerSubjects = map[string]bool{
	models.GeneralUnitSCV:   true,
	models.GeneralUnitProbe: true,
	models.GeneralUnitDrone: true,
}

// IsWorker reports whether the Subject is a race's worker unit.
func IsWorker(subject string) bool { return workerSubjects[subject] }

// supplyStructureSubjects is the set of buildings that increase supply cap.
var supplyStructureSubjects = map[string]bool{
	models.GeneralUnitPylon:       true,
	models.GeneralUnitSupplyDepot: true,
	models.GeneralUnitOverlord:    true,
}

// IsSupplyStructure reports whether the Subject increases supply cap on
// completion.
func IsSupplyStructure(subject string) bool { return supplyStructureSubjects[subject] }
