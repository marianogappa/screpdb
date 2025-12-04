package models

const (
	GeneralUnitMarine                          = "Marine"
	GeneralUnitGhost                           = "Ghost"
	GeneralUnitVulture                         = "Vulture"
	GeneralUnitGoliath                         = "Goliath"
	GeneralUnitGoliathTurret                   = "Goliath Turret"
	GeneralUnitSiegeTankTankMode               = "Siege Tank (Tank Mode)"
	GeneralUnitSiegeTankTurretTankMode         = "Siege Tank Turret (Tank Mode)" // TODO this is probably Siege Tank (Siege Mode), fix on screp side
	GeneralUnitSCV                             = "SCV"
	GeneralUnitWraith                          = "Wraith"
	GeneralUnitScienceVessel                   = "Science Vessel"
	GeneralUnitGuiMotangFirebat                = "Gui Motang (Firebat)"
	GeneralUnitDropship                        = "Dropship"
	GeneralUnitBattlecruiser                   = "Battlecruiser"
	GeneralUnitSpiderMine                      = "Spider Mine"
	GeneralUnitNuclearMissile                  = "Nuclear Missile"
	GeneralUnitTerranCivilian                  = "Terran Civilian"
	GeneralUnitSarahKerriganGhost              = "Sarah Kerrigan (Ghost)"
	GeneralUnitAlanSchezarGoliath              = "Alan Schezar (Goliath)"
	GeneralUnitAlanSchezarTurret               = "Alan Schezar Turret)"
	GeneralUnitJimRaynorVulture                = "Jim Raynor (Vulture)"
	GeneralUnitJimRaynorMarine                 = "Jim Raynor (Marine)"
	GeneralUnitTomKazanskyWraith               = "Tom Kazansky (Wraith)"
	GeneralUnitMagellanScienceVessel           = "Magellan (Science Vessel)"
	GeneralUnitEdmundDukeTankMode              = "Edmund Duke (Tank Mode)"
	GeneralUnitEdmundDukeTurretTankMode        = "Edmund Duke Turret (Tank Mode)"
	GeneralUnitEdmundDukeSiegeMode             = "Edmund Duke (Siege Mode)"
	GeneralUnitEdmundDukeTurretSiegeMode       = "Edmund Duke Turret (Siege Mode)"
	GeneralUnitArcturusMengskBattlecruiser     = "Arcturus Mengsk (Battlecruiser)"
	GeneralUnitHyperionBattlecruiser           = "Hyperion (Battlecruiser)"
	GeneralUnitNoradIiBattlecruiser            = "Norad II (Battlecruiser)"
	GeneralUnitTerranSiegeTankSiegeMode        = "Terran Siege Tank (Siege Mode)"
	GeneralUnitSiegeTankTurretSiegeMode        = "Siege Tank Turret (Siege Mode)"
	GeneralUnitFirebat                         = "Firebat"
	GeneralUnitScannerSweep                    = "Scanner Sweep"
	GeneralUnitMedic                           = "Medic"
	GeneralUnitLarva                           = "Larva"
	GeneralUnitEgg                             = "Egg"
	GeneralUnitZergling                        = "Zergling"
	GeneralUnitHydralisk                       = "Hydralisk"
	GeneralUnitUltralisk                       = "Ultralisk"
	GeneralUnitDrone                           = "Drone"
	GeneralUnitOverlord                        = "Overlord"
	GeneralUnitMutalisk                        = "Mutalisk"
	GeneralUnitGuardian                        = "Guardian"
	GeneralUnitQueen                           = "Queen"
	GeneralUnitDefiler                         = "Defiler"
	GeneralUnitScourge                         = "Scourge"
	GeneralUnitTorrasqueUltralisk              = "Torrasque (Ultralisk)"
	GeneralUnitMatriarchQueen                  = "Matriarch (Queen)"
	GeneralUnitInfestedTerran                  = "Infested Terran"
	GeneralUnitInfestedKerriganInfestedTerran  = "Infested Kerrigan (Infested Terran)"
	GeneralUnitUncleanOneDefiler               = "Unclean One (Defiler)"
	GeneralUnitHunterKillerHydralisk           = "Hunter Killer (Hydralisk)"
	GeneralUnitDevouringOneZergling            = "Devouring One (Zergling)"
	GeneralUnitKukulzaMutalisk                 = "Kukulza (Mutalisk)"
	GeneralUnitKukulzaGuardian                 = "Kukulza (Guardian)"
	GeneralUnitYggdrasillOverlord              = "Yggdrasill (Overlord)"
	GeneralUnitValkyrie                        = "Valkyrie"
	GeneralUnitMutaliskCocoon                  = "Mutalisk Cocoon"
	GeneralUnitCorsair                         = "Corsair"
	GeneralUnitDarkTemplar                     = "Dark Templar"
	GeneralUnitDevourer                        = "Devourer"
	GeneralUnitDarkArchon                      = "Dark Archon"
	GeneralUnitProbe                           = "Probe"
	GeneralUnitZealot                          = "Zealot"
	GeneralUnitDragoon                         = "Dragoon"
	GeneralUnitHighTemplar                     = "High Templar"
	GeneralUnitArchon                          = "Archon"
	GeneralUnitShuttle                         = "Shuttle"
	GeneralUnitScout                           = "Scout"
	GeneralUnitArbiter                         = "Arbiter"
	GeneralUnitCarrier                         = "Carrier"
	GeneralUnitInterceptor                     = "Interceptor"
	GeneralUnitProtossDarkTemplarHero          = "Protoss Dark Templar (Hero)"
	GeneralUnitZeratulDarkTemplar              = "Zeratul (Dark Templar)"
	GeneralUnitTassadarZeratulArchon           = "Tassadar/Zeratul (Archon)"
	GeneralUnitFenixZealot                     = "Fenix (Zealot)"
	GeneralUnitFenixDragoon                    = "Fenix (Dragoon)"
	GeneralUnitTassadarTemplar                 = "Tassadar (Templar)"
	GeneralUnitMojoScout                       = "Mojo (Scout)"
	GeneralUnitWarbringerReaver                = "Warbringer (Reaver)"
	GeneralUnitGantrithorCarrier               = "Gantrithor (Carrier)"
	GeneralUnitReaver                          = "Reaver"
	GeneralUnitObserver                        = "Observer"
	GeneralUnitScarab                          = "Scarab"
	GeneralUnitDanimothArbiter                 = "Danimoth (Arbiter)"
	GeneralUnitAldarisTemplar                  = "Aldaris (Templar)"
	GeneralUnitArtanisScout                    = "Artanis (Scout)"
	GeneralUnitRhynadonBadlandsCritter         = "Rhynadon (Badlands Critter)"
	GeneralUnitBengalaasJungleCritter          = "Bengalaas (Jungle Critter)"
	GeneralUnitCargoShipUnused                 = "Cargo Ship (Unused)"
	GeneralUnitMercenaryGunshipUnused          = "Mercenary Gunship (Unused)"
	GeneralUnitScantidDesertCritter            = "Scantid (Desert Critter)"
	GeneralUnitKakaruTwilightCritter           = "Kakaru (Twilight Critter)"
	GeneralUnitRagnasaurAshworldCritter        = "Ragnasaur (Ashworld Critter)"
	GeneralUnitUrsadonIceWorldCritter          = "Ursadon (Ice World Critter)"
	GeneralUnitLurkerEgg                       = "Lurker Egg"
	GeneralUnitRaszagalCorsair                 = "Raszagal (Corsair)"
	GeneralUnitSamirDuranGhost                 = "Samir Duran (Ghost)"
	GeneralUnitAlexeiStukovGhost               = "Alexei Stukov (Ghost)"
	GeneralUnitMapRevealer                     = "Map Revealer"
	GeneralUnitGerardDuGalleBattleCruiser      = "Gerard DuGalle (BattleCruiser)"
	GeneralUnitLurker                          = "Lurker"
	GeneralUnitInfestedDuranInfestedTerran     = "Infested Duran (Infested Terran)"
	GeneralUnitDisruptionWeb                   = "Disruption Web"
	GeneralUnitCommandCenter                   = "Command Center"
	GeneralUnitComSat                          = "ComSat"
	GeneralUnitNuclearSilo                     = "Nuclear Silo"
	GeneralUnitSupplyDepot                     = "Supply Depot"
	GeneralUnitRefinery                        = "Refinery"
	GeneralUnitBarracks                        = "Barracks"
	GeneralUnitAcademy                         = "Academy"
	GeneralUnitFactory                         = "Factory"
	GeneralUnitStarport                        = "Starport"
	GeneralUnitControlTower                    = "Control Tower"
	GeneralUnitScienceFacility                 = "Science Facility"
	GeneralUnitCovertOps                       = "Covert Ops"
	GeneralUnitPhysicsLab                      = "Physics Lab"
	GeneralUnitMachineShop                     = "Machine Shop"
	GeneralUnitRepairBayUnused                 = "Repair Bay (Unused)"
	GeneralUnitEngineeringBay                  = "Engineering Bay"
	GeneralUnitArmory                          = "Armory"
	GeneralUnitMissileTurret                   = "Missile Turret"
	GeneralUnitBunker                          = "Bunker"
	GeneralUnitNoradIiCrashed                  = "Norad II (Crashed)"
	GeneralUnitIonCannon                       = "Ion Cannon"
	GeneralUnitUrajCrystal                     = "Uraj Crystal"
	GeneralUnitKhalisCrystal                   = "Khalis Crystal"
	GeneralUnitInfestedCc                      = "Infested CC"
	GeneralUnitHatchery                        = "Hatchery"
	GeneralUnitLair                            = "Lair"
	GeneralUnitHive                            = "Hive"
	GeneralUnitNydusCanal                      = "Nydus Canal"
	GeneralUnitHydraliskDen                    = "Hydralisk Den"
	GeneralUnitDefilerMound                    = "Defiler Mound"
	GeneralUnitGreaterSpire                    = "Greater Spire"
	GeneralUnitQueensNest                      = "Queens Nest"
	GeneralUnitEvolutionChamber                = "Evolution Chamber"
	GeneralUnitUltraliskCavern                 = "Ultralisk Cavern"
	GeneralUnitSpire                           = "Spire"
	GeneralUnitSpawningPool                    = "Spawning Pool"
	GeneralUnitCreepColony                     = "Creep Colony"
	GeneralUnitSporeColony                     = "Spore Colony"
	GeneralUnitUnusedZergBuilding1             = "Unused Zerg Building1"
	GeneralUnitSunkenColony                    = "Sunken Colony"
	GeneralUnitZergOvermindWithShell           = "Zerg Overmind (With Shell)"
	GeneralUnitOvermind                        = "Overmind"
	GeneralUnitExtractor                       = "Extractor"
	GeneralUnitMatureChrysalis                 = "Mature Chrysalis"
	GeneralUnitCerebrate                       = "Cerebrate"
	GeneralUnitCerebrateDaggoth                = "Cerebrate Daggoth"
	GeneralUnitUnusedZergBuilding2             = "Unused Zerg Building2"
	GeneralUnitNexus                           = "Nexus"
	GeneralUnitRoboticsFacility                = "Robotics Facility"
	GeneralUnitPylon                           = "Pylon"
	GeneralUnitAssimilator                     = "Assimilator"
	GeneralUnitUnusedProtossBuilding1          = "Unused Protoss Building1"
	GeneralUnitObservatory                     = "Observatory"
	GeneralUnitGateway                         = "Gateway"
	GeneralUnitUnusedProtossBuilding2          = "Unused Protoss Building2"
	GeneralUnitPhotonCannon                    = "Photon Cannon"
	GeneralUnitCitadelOfAdun                   = "Citadel of Adun"
	GeneralUnitCyberneticsCore                 = "Cybernetics Core"
	GeneralUnitTemplarArchives                 = "Templar Archives"
	GeneralUnitForge                           = "Forge"
	GeneralUnitStargate                        = "Stargate"
	GeneralUnitStasisCellPrison                = "Stasis Cell/Prison"
	GeneralUnitFleetBeacon                     = "Fleet Beacon"
	GeneralUnitArbiterTribunal                 = "Arbiter Tribunal"
	GeneralUnitRoboticsSupportBay              = "Robotics Support Bay"
	GeneralUnitShieldBattery                   = "Shield Battery"
	GeneralUnitKhaydarinCrystalFormation       = "Khaydarin Crystal Formation"
	GeneralUnitProtossTemple                   = "Protoss Temple"
	GeneralUnitXelNagaTemple                   = "Xel'Naga Temple"
	GeneralUnitMineralFieldType_1              = "Mineral Field (Type 1)"
	GeneralUnitMineralFieldType_2              = "Mineral Field (Type 2)"
	GeneralUnitMineralFieldType_3              = "Mineral Field (Type 3)"
	GeneralUnitCaveUnused                      = "Cave (Unused)"
	GeneralUnitCaveInUnused                    = "Cave-in (Unused)"
	GeneralUnitCantinaUnused                   = "Cantina (Unused)"
	GeneralUnitMiningPlatformUnused            = "Mining Platform (Unused)"
	GeneralUnitIndependentCommandCenterUnused  = "Independent Command Center (Unused)"
	GeneralUnitIndependentStarportUnused       = "Independent Starport (Unused)"
	GeneralUnitIndependentJumpGateUnused       = "Independent Jump Gate (Unused)"
	GeneralUnitRuinsUnused                     = "Ruins (Unused)"
	GeneralUnitKhaydarinCrystalFormationUnused = "Khaydarin Crystal Formation (Unused)"
	GeneralUnitVespeneGeyser                   = "Vespene Geyser"
	GeneralUnitWarpGate                        = "Warp Gate"
	GeneralUnitPsiDisrupter                    = "Psi Disrupter"
	GeneralUnitZergMarker                      = "Zerg Marker"
	GeneralUnitTerranMarker                    = "Terran Marker"
	GeneralUnitProtossMarker                   = "Protoss Marker"
	GeneralUnitZergBeacon                      = "Zerg Beacon"
	GeneralUnitTerranBeacon                    = "Terran Beacon"
	GeneralUnitProtossBeacon                   = "Protoss Beacon"
	GeneralUnitZergFlagBeacon                  = "Zerg Flag Beacon"
	GeneralUnitTerranFlagBeacon                = "Terran Flag Beacon"
	GeneralUnitProtossFlagBeacon               = "Protoss Flag Beacon"
	GeneralUnitPowerGenerator                  = "Power Generator"
	GeneralUnitOvermindCocoon                  = "Overmind Cocoon"
	GeneralUnitDarkSwarm                       = "Dark Swarm"
	GeneralUnitFloorMissileTrap                = "Floor Missile Trap"
	GeneralUnitFloorHatchUnused                = "Floor Hatch (Unused)"
	GeneralUnitLeftUpperLevelDoor              = "Left Upper Level Door"
	GeneralUnitRightUpperLevelDoor             = "Right Upper Level Door"
	GeneralUnitLeftPitDoor                     = "Left Pit Door"
	GeneralUnitRightPitDoor                    = "Right Pit Door"
	GeneralUnitFloorGunTrap                    = "Floor Gun Trap"
	GeneralUnitLeftWallMissileTrap             = "Left Wall Missile Trap"
	GeneralUnitLeftWallFlameTrap               = "Left Wall Flame Trap"
	GeneralUnitRightWallMissileTrap            = "Right Wall Missile Trap"
	GeneralUnitRightWallFlameTrap              = "Right Wall Flame Trap"
	GeneralUnitStartLocation                   = "Start Location"
	GeneralUnitFlag                            = "Flag"
	GeneralUnitYoungChrysalis                  = "Young Chrysalis"
	GeneralUnitPsiEmitter                      = "Psi Emitter"
	GeneralUnitDataDisc                        = "Data Disc"
	GeneralUnitKhaydarinCrystal                = "Khaydarin Crystal"
	GeneralUnitMineralClusterType_1            = "Mineral Cluster Type 1"
	GeneralUnitMineralClusterType_2            = "Mineral Cluster Type 2"
	GeneralUnitProtossVespeneGasOrbType_1      = "Protoss Vespene Gas Orb Type 1"
	GeneralUnitProtossVespeneGasOrbType_2      = "Protoss Vespene Gas Orb Type 2"
	GeneralUnitZergVespeneGasSacType_1         = "Zerg Vespene Gas Sac Type 1"
	GeneralUnitZergVespeneGasSacType_2         = "Zerg Vespene Gas Sac Type 2"
	GeneralUnitTerranVespeneGasTankType_1      = "Terran Vespene Gas Tank Type 1"
	GeneralUnitTerranVespeneGasTankType_2      = "Terran Vespene Gas Tank Type 2"
)

var (
	Units = []string{
		GeneralUnitMarine,
		GeneralUnitGhost,
		GeneralUnitVulture,
		GeneralUnitGoliath,
		GeneralUnitSiegeTankTankMode,
		GeneralUnitSiegeTankTurretTankMode,
		GeneralUnitSCV,
		GeneralUnitWraith,
		GeneralUnitScienceVessel,
		GeneralUnitDropship,
		GeneralUnitBattlecruiser,
		GeneralUnitFirebat,
		GeneralUnitMedic,
		GeneralUnitValkyrie,
		GeneralUnitZergling,
		GeneralUnitHydralisk,
		GeneralUnitUltralisk,
		GeneralUnitDrone,
		GeneralUnitOverlord,
		GeneralUnitMutalisk,
		GeneralUnitGuardian,
		GeneralUnitQueen,
		GeneralUnitDefiler,
		GeneralUnitScourge,
		GeneralUnitDevourer,
		GeneralUnitLurker,
		GeneralUnitInfestedTerran,
		GeneralUnitCorsair,
		GeneralUnitDarkTemplar,
		GeneralUnitDarkArchon,
		GeneralUnitProbe,
		GeneralUnitZealot,
		GeneralUnitDragoon,
		GeneralUnitHighTemplar,
		GeneralUnitArchon,
		GeneralUnitShuttle,
		GeneralUnitScout,
		GeneralUnitArbiter,
		GeneralUnitCarrier,
		GeneralUnitReaver,
		GeneralUnitObserver,
	}
	TerranUnits = []string{
		GeneralUnitMarine,
		GeneralUnitGhost,
		GeneralUnitVulture,
		GeneralUnitGoliath,
		GeneralUnitSiegeTankTankMode,
		GeneralUnitSiegeTankTurretTankMode,
		GeneralUnitSCV,
		GeneralUnitWraith,
		GeneralUnitScienceVessel,
		GeneralUnitDropship,
		GeneralUnitBattlecruiser,
		GeneralUnitFirebat,
		GeneralUnitMedic,
		GeneralUnitValkyrie,
	}
	ZergUnits = []string{
		GeneralUnitZergling,
		GeneralUnitHydralisk,
		GeneralUnitUltralisk,
		GeneralUnitDrone,
		GeneralUnitOverlord,
		GeneralUnitMutalisk,
		GeneralUnitGuardian,
		GeneralUnitQueen,
		GeneralUnitDefiler,
		GeneralUnitScourge,
		GeneralUnitDevourer,
		GeneralUnitLurker,
		GeneralUnitInfestedTerran,
	}
	ProtossUnits = []string{
		GeneralUnitCorsair,
		GeneralUnitDarkTemplar,
		GeneralUnitDarkArchon,
		GeneralUnitProbe,
		GeneralUnitZealot,
		GeneralUnitDragoon,
		GeneralUnitHighTemplar,
		GeneralUnitArchon,
		GeneralUnitShuttle,
		GeneralUnitScout,
		GeneralUnitArbiter,
		GeneralUnitCarrier,
		GeneralUnitReaver,
		GeneralUnitObserver,
	}
	Buildings = []string{
		GeneralUnitCommandCenter,
		GeneralUnitComSat,
		GeneralUnitNuclearSilo,
		GeneralUnitSupplyDepot,
		GeneralUnitRefinery,
		GeneralUnitBarracks,
		GeneralUnitAcademy,
		GeneralUnitFactory,
		GeneralUnitStarport,
		GeneralUnitControlTower,
		GeneralUnitScienceFacility,
		GeneralUnitCovertOps,
		GeneralUnitPhysicsLab,
		GeneralUnitMachineShop,
		GeneralUnitEngineeringBay,
		GeneralUnitArmory,
		GeneralUnitMissileTurret,
		GeneralUnitBunker,
		GeneralUnitInfestedCc,
		GeneralUnitHatchery,
		GeneralUnitLair,
		GeneralUnitHive,
		GeneralUnitNydusCanal,
		GeneralUnitHydraliskDen,
		GeneralUnitDefilerMound,
		GeneralUnitGreaterSpire,
		GeneralUnitQueensNest,
		GeneralUnitEvolutionChamber,
		GeneralUnitUltraliskCavern,
		GeneralUnitSpire,
		GeneralUnitSpawningPool,
		GeneralUnitCreepColony,
		GeneralUnitSporeColony,
		GeneralUnitSunkenColony,
		GeneralUnitExtractor,
		GeneralUnitNexus,
		GeneralUnitRoboticsFacility,
		GeneralUnitPylon,
		GeneralUnitAssimilator,
		GeneralUnitObservatory,
		GeneralUnitGateway,
		GeneralUnitPhotonCannon,
		GeneralUnitCitadelOfAdun,
		GeneralUnitCyberneticsCore,
		GeneralUnitTemplarArchives,
		GeneralUnitForge,
		GeneralUnitStargate,
		GeneralUnitFleetBeacon,
		GeneralUnitArbiterTribunal,
		GeneralUnitRoboticsSupportBay,
		GeneralUnitShieldBattery,
	}
	TerranBuildings = []string{
		GeneralUnitCommandCenter,
		GeneralUnitComSat,
		GeneralUnitNuclearSilo,
		GeneralUnitSupplyDepot,
		GeneralUnitRefinery,
		GeneralUnitBarracks,
		GeneralUnitAcademy,
		GeneralUnitFactory,
		GeneralUnitStarport,
		GeneralUnitControlTower,
		GeneralUnitScienceFacility,
		GeneralUnitCovertOps,
		GeneralUnitPhysicsLab,
		GeneralUnitMachineShop,
		GeneralUnitEngineeringBay,
		GeneralUnitArmory,
		GeneralUnitMissileTurret,
		GeneralUnitBunker,
	}
	ZergBuildings = []string{
		GeneralUnitInfestedCc,
		GeneralUnitHatchery,
		GeneralUnitLair,
		GeneralUnitHive,
		GeneralUnitNydusCanal,
		GeneralUnitHydraliskDen,
		GeneralUnitDefilerMound,
		GeneralUnitGreaterSpire,
		GeneralUnitQueensNest,
		GeneralUnitEvolutionChamber,
		GeneralUnitUltraliskCavern,
		GeneralUnitSpire,
		GeneralUnitSpawningPool,
		GeneralUnitCreepColony,
		GeneralUnitSporeColony,
		GeneralUnitSunkenColony,
		GeneralUnitExtractor,
	}
	ProtossBuildings = []string{
		GeneralUnitNexus,
		GeneralUnitRoboticsFacility,
		GeneralUnitPylon,
		GeneralUnitAssimilator,
		GeneralUnitObservatory,
		GeneralUnitGateway,
		GeneralUnitPhotonCannon,
		GeneralUnitCitadelOfAdun,
		GeneralUnitCyberneticsCore,
		GeneralUnitTemplarArchives,
		GeneralUnitForge,
		GeneralUnitStargate,
		GeneralUnitFleetBeacon,
		GeneralUnitArbiterTribunal,
		GeneralUnitRoboticsSupportBay,
		GeneralUnitShieldBattery,
	}
)
