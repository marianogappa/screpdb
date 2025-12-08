package models

type Unit struct {
	Name         string
	WidthPixels  int
	HeightPixels int
}

type Building struct {
	Name             string
	BoxWidthPixels   int
	BoxHeightPixels  int
	RealWidthPixels  int
	RealHeightPixels int
	GapTopPixels     int
	GapLeftPixels    int
	GapRightPixels   int
	GapBottomPixels  int
}

var (
	UnitMarine = Unit{
		Name:         GeneralUnitMarine,
		WidthPixels:  17,
		HeightPixels: 20,
	}
	UnitGhost = Unit{
		Name:         GeneralUnitGhost,
		WidthPixels:  15,
		HeightPixels: 22,
	}
	UnitVulture = Unit{
		Name:         GeneralUnitVulture,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitGoliath = Unit{
		Name:         GeneralUnitGoliath,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitSiegeTankTankMode = Unit{
		Name:         GeneralUnitSiegeTankTankMode,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitSiegeTankTurretTankMode = Unit{
		Name:         GeneralUnitSiegeTankTurretTankMode,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitSCV = Unit{
		Name:         GeneralUnitSCV,
		WidthPixels:  23,
		HeightPixels: 23,
	}
	UnitWraith = Unit{
		Name:         GeneralUnitWraith,
		WidthPixels:  38,
		HeightPixels: 30,
	}
	UnitScienceVessel = Unit{
		Name:         GeneralUnitScienceVessel,
		WidthPixels:  65,
		HeightPixels: 50,
	}
	UnitDropship = Unit{
		Name:         GeneralUnitDropship,
		WidthPixels:  49,
		HeightPixels: 37,
	}
	UnitBattlecruiser = Unit{
		Name:         GeneralUnitBattlecruiser,
		WidthPixels:  75,
		HeightPixels: 59,
	}
	UnitFirebat = Unit{
		Name:         GeneralUnitFirebat,
		WidthPixels:  23,
		HeightPixels: 22,
	}
	UnitMedic = Unit{
		Name:         GeneralUnitMedic,
		WidthPixels:  17,
		HeightPixels: 20,
	}
	UnitValkyrie = Unit{
		Name:         GeneralUnitValkyrie,
		WidthPixels:  49,
		HeightPixels: 37,
	}
	UnitZergling = Unit{
		Name:         GeneralUnitZergling,
		WidthPixels:  16,
		HeightPixels: 16,
	}
	UnitHydralisk = Unit{
		Name:         GeneralUnitHydralisk,
		WidthPixels:  21,
		HeightPixels: 23,
	}
	UnitUltralisk = Unit{
		Name:         GeneralUnitUltralisk,
		WidthPixels:  38,
		HeightPixels: 32,
	}
	UnitDrone = Unit{
		Name:         GeneralUnitDrone,
		WidthPixels:  23,
		HeightPixels: 23,
	}
	UnitOverlord = Unit{
		Name:         GeneralUnitOverlord,
		WidthPixels:  50,
		HeightPixels: 50,
	}
	UnitMutalisk = Unit{
		Name:         GeneralUnitMutalisk,
		WidthPixels:  44,
		HeightPixels: 44,
	}
	UnitGuardian = Unit{
		Name:         GeneralUnitGuardian,
		WidthPixels:  44,
		HeightPixels: 44,
	}
	UnitQueen = Unit{
		Name:         GeneralUnitQueen,
		WidthPixels:  48,
		HeightPixels: 48,
	}
	UnitDefiler = Unit{
		Name:         GeneralUnitDefiler,
		WidthPixels:  27,
		HeightPixels: 25,
	}
	UnitScourge = Unit{
		Name:         GeneralUnitScourge,
		WidthPixels:  24,
		HeightPixels: 24,
	}
	UnitDevourer = Unit{
		Name:         GeneralUnitDevourer,
		WidthPixels:  44,
		HeightPixels: 44,
	}
	UnitLurker = Unit{
		Name:         GeneralUnitLurker,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitInfestedTerran = Unit{
		Name:         GeneralUnitInfestedTerran,
		WidthPixels:  17,
		HeightPixels: 20,
	}
	UnitCorsair = Unit{
		Name:         GeneralUnitCorsair,
		WidthPixels:  36,
		HeightPixels: 32,
	}
	UnitDarkTemplar = Unit{
		Name:         GeneralUnitDarkTemplar,
		WidthPixels:  24,
		HeightPixels: 26,
	}
	UnitDarkArchon = Unit{
		Name:         GeneralUnitDarkArchon,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitProbe = Unit{
		Name:         GeneralUnitProbe,
		WidthPixels:  23,
		HeightPixels: 23,
	}
	UnitZealot = Unit{
		Name:         GeneralUnitZealot,
		WidthPixels:  23,
		HeightPixels: 19,
	}
	UnitDragoon = Unit{
		Name:         GeneralUnitDragoon,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitHighTemplar = Unit{
		Name:         GeneralUnitHighTemplar,
		WidthPixels:  24,
		HeightPixels: 24,
	}
	UnitArchon = Unit{
		Name:         GeneralUnitArchon,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitShuttle = Unit{
		Name:         GeneralUnitShuttle,
		WidthPixels:  40,
		HeightPixels: 32,
	}
	UnitScout = Unit{
		Name:         GeneralUnitScout,
		WidthPixels:  36,
		HeightPixels: 32,
	}
	UnitArbiter = Unit{
		Name:         GeneralUnitArbiter,
		WidthPixels:  44,
		HeightPixels: 44,
	}
	UnitCarrier = Unit{
		Name:         GeneralUnitCarrier,
		WidthPixels:  64,
		HeightPixels: 64,
	}
	UnitReaver = Unit{
		Name:         GeneralUnitReaver,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	UnitObserver = Unit{
		Name:         GeneralUnitObserver,
		WidthPixels:  32,
		HeightPixels: 32,
	}
	BuildingCommandCenter = Building{
		Name:             GeneralUnitCommandCenter,
		GapTopPixels:     7,
		GapLeftPixels:    6,
		GapRightPixels:   5,
		GapBottomPixels:  6,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  117,
		RealHeightPixels: 83,
	}
	BuildingComSat = Building{
		Name:             GeneralUnitComSat,
		GapTopPixels:     16,
		GapLeftPixels:    -5,
		GapRightPixels:   0,
		GapBottomPixels:  6,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  69,
		RealHeightPixels: 42,
	}
	BuildingNuclearSilo = Building{
		Name:             GeneralUnitNuclearSilo,
		GapTopPixels:     16,
		GapLeftPixels:    -5,
		GapRightPixels:   0,
		GapBottomPixels:  6,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  69,
		RealHeightPixels: 42,
	}
	BuildingSupplyDepot = Building{
		Name:             GeneralUnitSupplyDepot,
		GapTopPixels:     10,
		GapLeftPixels:    10,
		GapRightPixels:   9,
		GapBottomPixels:  5,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  77,
		RealHeightPixels: 49,
	}
	BuildingRefinery = Building{
		Name:             GeneralUnitRefinery,
		GapTopPixels:     0,
		GapLeftPixels:    8,
		GapRightPixels:   7,
		GapBottomPixels:  0,
		BoxWidthPixels:   128,
		BoxHeightPixels:  64,
		RealWidthPixels:  113,
		RealHeightPixels: 64,
	}
	BuildingBarracks = Building{
		Name:             GeneralUnitBarracks,
		GapTopPixels:     8,
		GapLeftPixels:    16,
		GapRightPixels:   7,
		GapBottomPixels:  15,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  105,
		RealHeightPixels: 73,
	}
	BuildingAcademy = Building{
		Name:             GeneralUnitAcademy,
		GapTopPixels:     0,
		GapLeftPixels:    8,
		GapRightPixels:   3,
		GapBottomPixels:  7,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  85,
		RealHeightPixels: 57,
	}
	BuildingFactory = Building{
		Name:             GeneralUnitFactory,
		GapTopPixels:     8,
		GapLeftPixels:    8,
		GapRightPixels:   7,
		GapBottomPixels:  7,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  113,
		RealHeightPixels: 81,
	}
	BuildingStarport = Building{
		Name:             GeneralUnitStarport,
		GapTopPixels:     8,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  9,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  97,
		RealHeightPixels: 79,
	}
	BuildingControlTower = Building{
		Name:             GeneralUnitControlTower,
		GapTopPixels:     8,
		GapLeftPixels:    -15,
		GapRightPixels:   3,
		GapBottomPixels:  9,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  76,
		RealHeightPixels: 47,
	}
	BuildingScienceFacility = Building{
		Name:             GeneralUnitScienceFacility,
		GapTopPixels:     10,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  9,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  97,
		RealHeightPixels: 77,
	}
	BuildingCovertOps = Building{
		Name:             GeneralUnitCovertOps,
		GapTopPixels:     8,
		GapLeftPixels:    -15,
		GapRightPixels:   3,
		GapBottomPixels:  9,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  76,
		RealHeightPixels: 47,
	}
	BuildingPhysicsLab = Building{
		Name:             GeneralUnitPhysicsLab,
		GapTopPixels:     8,
		GapLeftPixels:    -15,
		GapRightPixels:   3,
		GapBottomPixels:  9,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  76,
		RealHeightPixels: 47,
	}
	BuildingMachineShop = Building{
		Name:             GeneralUnitMachineShop,
		GapTopPixels:     8,
		GapLeftPixels:    -7,
		GapRightPixels:   0,
		GapBottomPixels:  7,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  71,
		RealHeightPixels: 49,
	}
	BuildingEngineeringBay = Building{
		Name:             GeneralUnitEngineeringBay,
		GapTopPixels:     16,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  19,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  97,
		RealHeightPixels: 61,
	}
	BuildingArmory = Building{
		Name:             GeneralUnitArmory,
		GapTopPixels:     0,
		GapLeftPixels:    0,
		GapRightPixels:   0,
		GapBottomPixels:  9,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  96,
		RealHeightPixels: 55,
	}
	BuildingMissileTurret = Building{
		Name:             GeneralUnitMissileTurret,
		GapTopPixels:     0,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  15,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  33,
		RealHeightPixels: 49,
	}
	BuildingBunker = Building{
		Name:             GeneralUnitBunker,
		GapTopPixels:     0,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  15,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  33,
		RealHeightPixels: 49,
	}

	BuildingInfestedCc = Building{
		Name:             GeneralUnitInfestedCc,
		GapTopPixels:     7,
		GapLeftPixels:    6,
		GapRightPixels:   5,
		GapBottomPixels:  6,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  117,
		RealHeightPixels: 83,
	}
	BuildingHatchery = Building{
		Name:             GeneralUnitHatchery,
		GapTopPixels:     16,
		GapLeftPixels:    15,
		GapRightPixels:   14,
		GapBottomPixels:  15,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  99,
		RealHeightPixels: 65,
	}
	BuildingLair = Building{
		Name:             GeneralUnitLair,
		GapTopPixels:     16,
		GapLeftPixels:    15,
		GapRightPixels:   14,
		GapBottomPixels:  15,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  99,
		RealHeightPixels: 65,
	}
	BuildingHive = Building{
		Name:             GeneralUnitHive,
		GapTopPixels:     16,
		GapLeftPixels:    15,
		GapRightPixels:   14,
		GapBottomPixels:  15,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  99,
		RealHeightPixels: 65,
	}
	BuildingNydusCanal = Building{
		Name:             GeneralUnitNydusCanal,
		GapTopPixels:     0,
		GapLeftPixels:    0,
		GapRightPixels:   0,
		GapBottomPixels:  0,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  64,
		RealHeightPixels: 64,
	}
	BuildingHydraliskDen = Building{
		Name:             GeneralUnitHydraliskDen,
		GapTopPixels:     0,
		GapLeftPixels:    8,
		GapRightPixels:   7,
		GapBottomPixels:  7,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  81,
		RealHeightPixels: 57,
	}
	BuildingDefilerMound = Building{
		Name:             GeneralUnitDefilerMound,
		GapTopPixels:     0,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  27,
		BoxWidthPixels:   128,
		BoxHeightPixels:  64,
		RealWidthPixels:  97,
		RealHeightPixels: 37,
	}
	BuildingGreaterSpire = Building{
		Name:             GeneralUnitGreaterSpire,
		GapTopPixels:     0,
		GapLeftPixels:    4,
		GapRightPixels:   3,
		GapBottomPixels:  7,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  57,
		RealHeightPixels: 57,
	}
	BuildingQueensNest = Building{
		Name:             GeneralUnitQueensNest,
		GapTopPixels:     4,
		GapLeftPixels:    10,
		GapRightPixels:   15,
		GapBottomPixels:  3,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  71,
		RealHeightPixels: 57,
	}
	BuildingEvolutionChamber = Building{
		Name:             GeneralUnitEvolutionChamber,
		GapTopPixels:     0,
		GapLeftPixels:    4,
		GapRightPixels:   15,
		GapBottomPixels:  11,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  77,
		RealHeightPixels: 53,
	}
	BuildingUltraliskCavern = Building{
		Name:             GeneralUnitUltraliskCavern,
		GapTopPixels:     0,
		GapLeftPixels:    8,
		GapRightPixels:   15,
		GapBottomPixels:  0,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  73,
		RealHeightPixels: 64,
	}
	BuildingSpire = Building{
		Name:             GeneralUnitSpire,
		GapTopPixels:     0,
		GapLeftPixels:    4,
		GapRightPixels:   3,
		GapBottomPixels:  7,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  57,
		RealHeightPixels: 57,
	}
	BuildingSpawningPool = Building{
		Name:             GeneralUnitSpawningPool,
		GapTopPixels:     4,
		GapLeftPixels:    12,
		GapRightPixels:   7,
		GapBottomPixels:  13,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  77,
		RealHeightPixels: 47,
	}
	BuildingCreepColony = Building{
		Name:             GeneralUnitCreepColony,
		GapTopPixels:     8,
		GapLeftPixels:    8,
		GapRightPixels:   8,
		GapBottomPixels:  8,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  48,
		RealHeightPixels: 48,
	}
	BuildingSporeColony = Building{
		Name:             GeneralUnitSporeColony,
		GapTopPixels:     8,
		GapLeftPixels:    8,
		GapRightPixels:   8,
		GapBottomPixels:  8,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  48,
		RealHeightPixels: 48,
	}
	BuildingSunkenColony = Building{
		Name:             GeneralUnitSunkenColony,
		GapTopPixels:     8,
		GapLeftPixels:    8,
		GapRightPixels:   8,
		GapBottomPixels:  8,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  48,
		RealHeightPixels: 48,
	}
	BuildingExtractor = Building{
		Name:             GeneralUnitExtractor,
		GapTopPixels:     0,
		GapLeftPixels:    0,
		GapRightPixels:   0,
		GapBottomPixels:  0,
		BoxWidthPixels:   128,
		BoxHeightPixels:  64,
		RealWidthPixels:  128,
		RealHeightPixels: 64,
	}

	BuildingNexus = Building{
		Name:             GeneralUnitNexus,
		GapTopPixels:     9,
		GapLeftPixels:    8,
		GapRightPixels:   7,
		GapBottomPixels:  8,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  113,
		RealHeightPixels: 79,
	}
	BuildingRoboticsFacility = Building{
		Name:             GeneralUnitRoboticsFacility,
		GapTopPixels:     16,
		GapLeftPixels:    12,
		GapRightPixels:   7,
		GapBottomPixels:  11,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  77,
		RealHeightPixels: 37,
	}
	BuildingPylon = Building{
		Name:             GeneralUnitPylon,
		GapTopPixels:     20,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  11,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  33,
		RealHeightPixels: 33,
	}
	BuildingAssimilator = Building{
		Name:             GeneralUnitAssimilator,
		GapTopPixels:     0,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  7,
		BoxWidthPixels:   128,
		BoxHeightPixels:  64,
		RealWidthPixels:  97,
		RealHeightPixels: 57,
	}
	BuildingObservatory = Building{
		Name:             GeneralUnitObservatory,
		GapTopPixels:     16,
		GapLeftPixels:    4,
		GapRightPixels:   3,
		GapBottomPixels:  3,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  89,
		RealHeightPixels: 45,
	}
	BuildingGateway = Building{
		Name:             GeneralUnitGateway,
		GapTopPixels:     16,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  7,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  97,
		RealHeightPixels: 73,
	}
	BuildingPhotonCannon = Building{
		Name:             GeneralUnitPhotonCannon,
		GapTopPixels:     16,
		GapLeftPixels:    12,
		GapRightPixels:   11,
		GapBottomPixels:  15,
		BoxWidthPixels:   64,
		BoxHeightPixels:  64,
		RealWidthPixels:  41,
		RealHeightPixels: 33,
	}
	BuildingCitadelOfAdun = Building{
		Name:             GeneralUnitCitadelOfAdun,
		GapTopPixels:     8,
		GapLeftPixels:    24,
		GapRightPixels:   7,
		GapBottomPixels:  7,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  65,
		RealHeightPixels: 49,
	}
	BuildingCyberneticsCore = Building{
		Name:             GeneralUnitCyberneticsCore,
		GapTopPixels:     8,
		GapLeftPixels:    8,
		GapRightPixels:   7,
		GapBottomPixels:  7,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  81,
		RealHeightPixels: 49,
	}
	BuildingTemplarArchives = Building{
		Name:             GeneralUnitTemplarArchives,
		GapTopPixels:     8,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  7,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  65,
		RealHeightPixels: 49,
	}
	BuildingForge = Building{
		Name:             GeneralUnitForge,
		GapTopPixels:     8,
		GapLeftPixels:    12,
		GapRightPixels:   11,
		GapBottomPixels:  11,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  73,
		RealHeightPixels: 45,
	}
	BuildingStargate = Building{
		Name:             GeneralUnitStargate,
		GapTopPixels:     8,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  15,
		BoxWidthPixels:   128,
		BoxHeightPixels:  96,
		RealWidthPixels:  97,
		RealHeightPixels: 73,
	}
	BuildingFleetBeacon = Building{
		Name:             GeneralUnitFleetBeacon,
		GapTopPixels:     0,
		GapLeftPixels:    8,
		GapRightPixels:   0,
		GapBottomPixels:  7,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  88,
		RealHeightPixels: 57,
	}
	BuildingArbiterTribunal = Building{
		Name:             GeneralUnitArbiterTribunal,
		GapTopPixels:     4,
		GapLeftPixels:    4,
		GapRightPixels:   3,
		GapBottomPixels:  3,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  89,
		RealHeightPixels: 57,
	}
	BuildingRoboticsSupportBay = Building{
		Name:             GeneralUnitRoboticsSupportBay,
		GapTopPixels:     0,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  11,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  65,
		RealHeightPixels: 53,
	}
	BuildingShieldBattery = Building{
		Name:             GeneralUnitShieldBattery,
		GapTopPixels:     16,
		GapLeftPixels:    16,
		GapRightPixels:   15,
		GapBottomPixels:  15,
		BoxWidthPixels:   96,
		BoxHeightPixels:  64,
		RealWidthPixels:  65,
		RealHeightPixels: 33,
	}
)
