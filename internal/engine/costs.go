package engine

import "github.com/marianogappa/openbw-go/bwgame"

// Cost is a unit's price, with what it morphed from included: a Lurker is a Hydralisk plus its morph.
// Melee Brood War prices are fixed, so the table needs no game data.
type Cost struct{ Minerals, Gas int32 }

func (c Cost) Value() int32 { return c.Minerals + c.Gas }

var costs = map[bwgame.UnitTypes]Cost{
	bwgame.UnitTypesTerranMarine: {50, 0}, bwgame.UnitTypesTerranGhost: {25, 75}, bwgame.UnitTypesTerranVulture: {75, 0},
	bwgame.UnitTypesTerranGoliath: {100, 50}, bwgame.UnitTypesTerranSiegeTankTankMode: {150, 100}, bwgame.UnitTypesTerranSiegeTankSiegeMode: {150, 100},
	bwgame.UnitTypesTerranSCV: {50, 0}, bwgame.UnitTypesTerranWraith: {150, 100}, bwgame.UnitTypesTerranScienceVessel: {100, 225},
	bwgame.UnitTypesTerranDropship: {100, 100}, bwgame.UnitTypesTerranBattlecruiser: {400, 300}, bwgame.UnitTypesTerranFirebat: {50, 25},
	bwgame.UnitTypesTerranMedic: {50, 25}, bwgame.UnitTypesTerranValkyrie: {250, 125},
	bwgame.UnitTypesTerranCommandCenter: {400, 0}, bwgame.UnitTypesTerranComsatStation: {50, 50}, bwgame.UnitTypesTerranNuclearSilo: {100, 100},
	bwgame.UnitTypesTerranSupplyDepot: {100, 0}, bwgame.UnitTypesTerranRefinery: {100, 0}, bwgame.UnitTypesTerranBarracks: {150, 0},
	bwgame.UnitTypesTerranAcademy: {150, 0}, bwgame.UnitTypesTerranFactory: {200, 100}, bwgame.UnitTypesTerranStarport: {150, 100},
	bwgame.UnitTypesTerranControlTower: {50, 50}, bwgame.UnitTypesTerranScienceFacility: {100, 150}, bwgame.UnitTypesTerranCovertOps: {50, 50},
	bwgame.UnitTypesTerranPhysicsLab: {50, 50}, bwgame.UnitTypesTerranMachineShop: {50, 50}, bwgame.UnitTypesTerranEngineeringBay: {125, 0},
	bwgame.UnitTypesTerranArmory: {100, 50}, bwgame.UnitTypesTerranMissileTurret: {75, 0}, bwgame.UnitTypesTerranBunker: {100, 0},

	bwgame.UnitTypesZergZergling: {25, 0}, bwgame.UnitTypesZergHydralisk: {75, 25}, bwgame.UnitTypesZergUltralisk: {200, 200},
	bwgame.UnitTypesZergDrone: {50, 0}, bwgame.UnitTypesZergOverlord: {100, 0}, bwgame.UnitTypesZergMutalisk: {100, 100},
	bwgame.UnitTypesZergGuardian: {150, 200}, bwgame.UnitTypesZergQueen: {100, 100}, bwgame.UnitTypesZergDefiler: {50, 150},
	bwgame.UnitTypesZergScourge: {12, 38}, bwgame.UnitTypesZergInfestedTerran: {100, 50}, bwgame.UnitTypesZergDevourer: {250, 150},
	bwgame.UnitTypesZergLurker:   {125, 125},
	bwgame.UnitTypesZergHatchery: {300, 0}, bwgame.UnitTypesZergLair: {450, 100}, bwgame.UnitTypesZergHive: {650, 250},
	bwgame.UnitTypesZergNydusCanal: {150, 0}, bwgame.UnitTypesZergHydraliskDen: {100, 50}, bwgame.UnitTypesZergDefilerMound: {100, 100},
	bwgame.UnitTypesZergGreaterSpire: {300, 300}, bwgame.UnitTypesZergQueensNest: {150, 100}, bwgame.UnitTypesZergEvolutionChamber: {75, 0},
	bwgame.UnitTypesZergUltraliskCavern: {150, 200}, bwgame.UnitTypesZergSpire: {200, 150}, bwgame.UnitTypesZergSpawningPool: {200, 0},
	bwgame.UnitTypesZergCreepColony: {75, 0}, bwgame.UnitTypesZergSporeColony: {125, 0}, bwgame.UnitTypesZergSunkenColony: {150, 0},
	bwgame.UnitTypesZergExtractor: {50, 0},

	bwgame.UnitTypesProtossProbe: {50, 0}, bwgame.UnitTypesProtossZealot: {100, 0}, bwgame.UnitTypesProtossDragoon: {125, 50},
	bwgame.UnitTypesProtossHighTemplar: {50, 150}, bwgame.UnitTypesProtossDarkTemplar: {125, 100}, bwgame.UnitTypesProtossArchon: {100, 300},
	bwgame.UnitTypesProtossDarkArchon: {250, 200}, bwgame.UnitTypesProtossShuttle: {200, 0}, bwgame.UnitTypesProtossScout: {275, 125},
	bwgame.UnitTypesProtossArbiter: {100, 350}, bwgame.UnitTypesProtossCarrier: {350, 250}, bwgame.UnitTypesProtossInterceptor: {25, 0},
	bwgame.UnitTypesProtossReaver: {200, 100}, bwgame.UnitTypesProtossObserver: {25, 75}, bwgame.UnitTypesProtossCorsair: {150, 100},
	bwgame.UnitTypesProtossNexus: {400, 0}, bwgame.UnitTypesProtossRoboticsFacility: {200, 200}, bwgame.UnitTypesProtossPylon: {100, 0},
	bwgame.UnitTypesProtossAssimilator: {100, 0}, bwgame.UnitTypesProtossObservatory: {50, 100}, bwgame.UnitTypesProtossGateway: {150, 0},
	bwgame.UnitTypesProtossPhotonCannon: {150, 0}, bwgame.UnitTypesProtossCitadelOfAdun: {150, 100}, bwgame.UnitTypesProtossCyberneticsCore: {200, 0},
	bwgame.UnitTypesProtossTemplarArchives: {150, 200}, bwgame.UnitTypesProtossForge: {150, 0}, bwgame.UnitTypesProtossStargate: {150, 150},
	bwgame.UnitTypesProtossFleetBeacon: {300, 200}, bwgame.UnitTypesProtossArbiterTribunal: {200, 150}, bwgame.UnitTypesProtossRoboticsSupportBay: {150, 100},
	bwgame.UnitTypesProtossShieldBattery: {100, 0},
}

func CostOf(t uint16) Cost { return costs[bwgame.UnitTypes(t)] }
