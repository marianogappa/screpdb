package moments

import (
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

type Cost = engine.Cost

func CostOf(t uint16) Cost { return engine.CostOf(t) }

// Footprint is a building's placement size in tiles; units have none.
type Footprint struct{ W, H int32 }

var footprints = map[bwgame.UnitTypes]Footprint{
	bwgame.UnitTypesTerranCommandCenter: {4, 3}, bwgame.UnitTypesTerranComsatStation: {2, 2}, bwgame.UnitTypesTerranNuclearSilo: {2, 2},
	bwgame.UnitTypesTerranSupplyDepot: {3, 2}, bwgame.UnitTypesTerranRefinery: {4, 2}, bwgame.UnitTypesTerranBarracks: {4, 3},
	bwgame.UnitTypesTerranAcademy: {3, 2}, bwgame.UnitTypesTerranFactory: {4, 3}, bwgame.UnitTypesTerranStarport: {4, 3},
	bwgame.UnitTypesTerranControlTower: {2, 2}, bwgame.UnitTypesTerranScienceFacility: {4, 3}, bwgame.UnitTypesTerranCovertOps: {2, 2},
	bwgame.UnitTypesTerranPhysicsLab: {2, 2}, bwgame.UnitTypesTerranMachineShop: {2, 2}, bwgame.UnitTypesTerranEngineeringBay: {4, 3},
	bwgame.UnitTypesTerranArmory: {3, 2}, bwgame.UnitTypesTerranMissileTurret: {2, 2}, bwgame.UnitTypesTerranBunker: {3, 2},
	bwgame.UnitTypesZergInfestedCommandCenter: {4, 3}, bwgame.UnitTypesZergHatchery: {4, 3}, bwgame.UnitTypesZergLair: {4, 3},
	bwgame.UnitTypesZergHive: {4, 3}, bwgame.UnitTypesZergNydusCanal: {2, 2}, bwgame.UnitTypesZergHydraliskDen: {3, 2},
	bwgame.UnitTypesZergDefilerMound: {4, 2}, bwgame.UnitTypesZergGreaterSpire: {2, 2}, bwgame.UnitTypesZergQueensNest: {3, 2},
	bwgame.UnitTypesZergEvolutionChamber: {3, 2}, bwgame.UnitTypesZergUltraliskCavern: {3, 2}, bwgame.UnitTypesZergSpire: {2, 2},
	bwgame.UnitTypesZergSpawningPool: {3, 2}, bwgame.UnitTypesZergCreepColony: {2, 2}, bwgame.UnitTypesZergSporeColony: {2, 2},
	bwgame.UnitTypesZergSunkenColony: {2, 2}, bwgame.UnitTypesZergExtractor: {4, 2},
	bwgame.UnitTypesProtossNexus: {4, 3}, bwgame.UnitTypesProtossRoboticsFacility: {3, 2}, bwgame.UnitTypesProtossPylon: {2, 2},
	bwgame.UnitTypesProtossAssimilator: {4, 2}, bwgame.UnitTypesProtossObservatory: {3, 2}, bwgame.UnitTypesProtossGateway: {4, 3},
	bwgame.UnitTypesProtossPhotonCannon: {2, 2}, bwgame.UnitTypesProtossCitadelOfAdun: {3, 2}, bwgame.UnitTypesProtossCyberneticsCore: {3, 2},
	bwgame.UnitTypesProtossTemplarArchives: {3, 2}, bwgame.UnitTypesProtossForge: {3, 2}, bwgame.UnitTypesProtossStargate: {4, 3},
	bwgame.UnitTypesProtossFleetBeacon: {3, 2}, bwgame.UnitTypesProtossArbiterTribunal: {3, 2}, bwgame.UnitTypesProtossRoboticsSupportBay: {3, 2},
	bwgame.UnitTypesProtossShieldBattery: {3, 2},
}

func FootprintOf(t uint16) (Footprint, bool) {
	f, ok := footprints[bwgame.UnitTypes(t)]
	return f, ok
}

var townHalls = map[bwgame.UnitTypes]bool{
	bwgame.UnitTypesTerranCommandCenter: true, bwgame.UnitTypesProtossNexus: true, bwgame.UnitTypesZergHatchery: true,
	bwgame.UnitTypesZergLair: true, bwgame.UnitTypesZergHive: true,
}

func IsTownHall(t uint16) bool { return townHalls[bwgame.UnitTypes(t)] }
