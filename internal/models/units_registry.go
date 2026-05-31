package models

import "sort"

// unitGeometry / buildingGeometry are the registries over the Unit* / Building*
// geometry vars declared in units.go. They exist so the SPECIFICATION.md
// generator can enumerate every pixel box. A completeness guard test
// (units_registry_test.go) parses units.go and fails if any Unit*/Building*
// var is missing here, so the registries can't silently fall out of sync.
var unitGeometry = []Unit{
	UnitMarine, UnitGhost, UnitVulture, UnitGoliath, UnitSiegeTankTankMode,
	UnitSiegeTankTurretTankMode, UnitSCV, UnitWraith, UnitScienceVessel,
	UnitDropship, UnitBattlecruiser, UnitFirebat, UnitMedic, UnitValkyrie,
	UnitZergling, UnitHydralisk, UnitUltralisk, UnitDrone, UnitOverlord,
	UnitMutalisk, UnitGuardian, UnitQueen, UnitDefiler, UnitScourge,
	UnitDevourer, UnitLurker, UnitInfestedTerran, UnitCorsair, UnitDarkTemplar,
	UnitDarkArchon, UnitProbe, UnitZealot, UnitDragoon, UnitHighTemplar,
	UnitArchon, UnitShuttle, UnitScout, UnitArbiter, UnitCarrier, UnitReaver,
	UnitObserver,
}

var buildingGeometry = []Building{
	BuildingCommandCenter, BuildingComSat, BuildingNuclearSilo,
	BuildingSupplyDepot, BuildingRefinery, BuildingBarracks, BuildingAcademy,
	BuildingFactory, BuildingStarport, BuildingControlTower,
	BuildingScienceFacility, BuildingCovertOps, BuildingPhysicsLab,
	BuildingMachineShop, BuildingEngineeringBay, BuildingArmory,
	BuildingMissileTurret, BuildingBunker, BuildingInfestedCc, BuildingHatchery,
	BuildingLair, BuildingHive, BuildingNydusCanal, BuildingHydraliskDen,
	BuildingDefilerMound, BuildingGreaterSpire, BuildingQueensNest,
	BuildingEvolutionChamber, BuildingUltraliskCavern, BuildingSpire,
	BuildingSpawningPool, BuildingCreepColony, BuildingSporeColony,
	BuildingSunkenColony, BuildingExtractor, BuildingNexus,
	BuildingRoboticsFacility, BuildingPylon, BuildingAssimilator,
	BuildingObservatory, BuildingGateway, BuildingPhotonCannon,
	BuildingCitadelOfAdun, BuildingCyberneticsCore, BuildingTemplarArchives,
	BuildingForge, BuildingStargate, BuildingFleetBeacon,
	BuildingArbiterTribunal, BuildingRoboticsSupportBay, BuildingShieldBattery,
}

// AllUnitGeometry returns every unit pixel box, sorted by Name. Used by the
// SPECIFICATION.md generator.
func AllUnitGeometry() []Unit {
	out := make([]Unit, len(unitGeometry))
	copy(out, unitGeometry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// AllBuildingGeometry returns every building pixel box, sorted by Name. Used by
// the SPECIFICATION.md generator.
func AllBuildingGeometry() []Building {
	out := make([]Building, len(buildingGeometry))
	copy(out, buildingGeometry)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
