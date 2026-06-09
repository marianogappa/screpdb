package models

// Upgrade game-knowledge keyed by Upgrade* name constants.
//
// "Upgrade" here matches screp's classification: passive stat boosts. Some are
// one-shot (Singularity Charge, Carrier Capacity…) and some are tiered to
// three levels (Ground Weapons, Carapace…). Tiered upgrades share a single
// UpgradeName across all three levels — the level is implied by occurrence
// order in the replay.
//
// Source: https://liquipedia.net/starcraft/Upgrades — Fastest game speed.
// Times are in-game seconds (the same unit Command.SecondsFromGameStart uses).
//
// Tiered weapons & armor durations are uniform across races: 167.58 / 180.18 /
// 192.78 seconds for L1 / L2 / L3.

// UpgradeLevelMeta is the cost/timing footprint of a single upgrade level.
// For one-shot upgrades, only Levels[0] is populated.
type UpgradeLevelMeta struct {
	Minerals  int
	Gas       int
	DurationS float64 // research time in seconds at Fastest game speed
}

// UpgradeMeta is the static metadata for an upgrade name. MaxLevel is 1 for
// one-shot upgrades and 3 for tiered weapons/armor. Levels indexes >= MaxLevel
// are zero-valued and should not be read.
type UpgradeMeta struct {
	Race            string  // RaceTerran / RaceZerg / RaceProtoss
	BuildingSubject string  // GeneralUnit* constant naming the researching building
	Hotkey          string  // single-letter; "" when unknown / not yet populated
	MaxLevel        int     // 1 for one-shot upgrades, 3 for tiered
	Levels          [3]UpgradeLevelMeta
}

const (
	tieredWeaponArmorL1S = 167.58
	tieredWeaponArmorL2S = 180.18
	tieredWeaponArmorL3S = 192.78
)

var upgradeTable = map[string]UpgradeMeta{
	// ===== Terran tiered (Engineering Bay / Armory) =====
	UpgradeTerranInfantryArmor: {
		Race: RaceTerran, BuildingSubject: GeneralUnitEngineeringBay, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeTerranInfantryWeapons: {
		Race: RaceTerran, BuildingSubject: GeneralUnitEngineeringBay, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeTerranVehiclePlating: {
		Race: RaceTerran, BuildingSubject: GeneralUnitArmory, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeTerranVehicleWeapons: {
		Race: RaceTerran, BuildingSubject: GeneralUnitArmory, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeTerranShipPlating: {
		Race: RaceTerran, BuildingSubject: GeneralUnitArmory, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL1S},
			{Minerals: 225, Gas: 225, DurationS: tieredWeaponArmorL2S},
			{Minerals: 300, Gas: 300, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeTerranShipWeapons: {
		Race: RaceTerran, BuildingSubject: GeneralUnitArmory, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL2S},
			{Minerals: 200, Gas: 200, DurationS: tieredWeaponArmorL3S},
		},
	},

	// ===== Terran one-shot =====
	UpgradeU238ShellsMarineRange:              oneShot(RaceTerran, GeneralUnitAcademy, 150, 150, 63),
	UpgradeMoebiusReactorGhostEnergy:          oneShot(RaceTerran, GeneralUnitCovertOps, 150, 150, 105),
	UpgradeOcularImplantsGhostSight:           oneShot(RaceTerran, GeneralUnitCovertOps, 100, 100, 105),
	UpgradeIonThrustersVultureSpeed:           oneShot(RaceTerran, GeneralUnitMachineShop, 100, 100, 63),
	UpgradeCharonBoostersGoliathRange:         oneShot(RaceTerran, GeneralUnitMachineShop, 100, 100, 84),
	UpgradeTitanReactorScienceVesselEnergy:    oneShot(RaceTerran, GeneralUnitScienceFacility, 150, 150, 105),
	UpgradeApolloReactorWraithEnergy:          oneShot(RaceTerran, GeneralUnitControlTower, 200, 200, 105),
	UpgradeColossusReactorBattleCruiserEnergy: oneShot(RaceTerran, GeneralUnitPhysicsLab, 150, 150, 105),
	UpgradeCaduceusReactorMedicEnergy:         oneShot(RaceTerran, GeneralUnitAcademy, 150, 150, 105),

	// ===== Zerg tiered (Evolution Chamber / Spire) =====
	UpgradeZergCarapace: {
		Race: RaceZerg, BuildingSubject: GeneralUnitEvolutionChamber, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL1S},
			{Minerals: 225, Gas: 225, DurationS: tieredWeaponArmorL2S},
			{Minerals: 300, Gas: 300, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeZergMeleeAttacks: {
		Race: RaceZerg, BuildingSubject: GeneralUnitEvolutionChamber, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL2S},
			{Minerals: 200, Gas: 200, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeZergMissileAttacks: {
		Race: RaceZerg, BuildingSubject: GeneralUnitEvolutionChamber, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL2S},
			{Minerals: 200, Gas: 200, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeZergFlyerCarapace: {
		Race: RaceZerg, BuildingSubject: GeneralUnitSpire, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL1S},
			{Minerals: 225, Gas: 225, DurationS: tieredWeaponArmorL2S},
			{Minerals: 300, Gas: 300, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeZergFlyerAttacks: {
		Race: RaceZerg, BuildingSubject: GeneralUnitSpire, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},

	// ===== Zerg one-shot =====
	UpgradeMetabolicBoostZerglingSpeed:      oneShot(RaceZerg, GeneralUnitSpawningPool, 100, 100, 63),
	UpgradeAdrenalGlandsZerglingAttack:      oneShot(RaceZerg, GeneralUnitSpawningPool, 200, 200, 63),
	UpgradeMuscularAugmentsHydraliskSpeed:   oneShot(RaceZerg, GeneralUnitHydraliskDen, 150, 150, 63),
	UpgradeGroovedSpinesHydraliskRange:      oneShot(RaceZerg, GeneralUnitHydraliskDen, 150, 150, 63),
	UpgradePneumatizedCarapaceOverlordSpeed: oneShot(RaceZerg, GeneralUnitLair, 150, 150, 83.79),
	UpgradeAntennaeOverlordSight:            oneShot(RaceZerg, GeneralUnitLair, 150, 150, 83.79),
	UpgradeVentralSacsOverlordTransport:     oneShot(RaceZerg, GeneralUnitLair, 200, 200, 100.8),
	UpgradeChitinousPlatingUltraliskArmor:   oneShot(RaceZerg, GeneralUnitUltraliskCavern, 150, 150, 83.79),
	UpgradeAnabolicSynthesisUltraliskSpeed:  oneShot(RaceZerg, GeneralUnitUltraliskCavern, 200, 200, 83.79),
	UpgradeGameteMeiosisQueenEnergy:         oneShot(RaceZerg, GeneralUnitQueensNest, 150, 150, 104.58),
	UpgradeDefilerEnergy:                    oneShot(RaceZerg, GeneralUnitDefilerMound, 150, 150, 104.58),

	// ===== Protoss tiered (Forge / Cybernetics Core) =====
	UpgradeProtossGroundArmor: {
		Race: RaceProtoss, BuildingSubject: GeneralUnitForge, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeProtossGroundWeapons: {
		Race: RaceProtoss, BuildingSubject: GeneralUnitForge, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL2S},
			{Minerals: 200, Gas: 200, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeProtossPlasmaShields: {
		Race: RaceProtoss, BuildingSubject: GeneralUnitForge, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 200, Gas: 200, DurationS: tieredWeaponArmorL1S},
			{Minerals: 300, Gas: 300, DurationS: tieredWeaponArmorL2S},
			{Minerals: 400, Gas: 400, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeProtossAirArmor: {
		Race: RaceProtoss, BuildingSubject: GeneralUnitCyberneticsCore, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 150, Gas: 150, DurationS: tieredWeaponArmorL1S},
			{Minerals: 225, Gas: 225, DurationS: tieredWeaponArmorL2S},
			{Minerals: 300, Gas: 300, DurationS: tieredWeaponArmorL3S},
		},
	},
	UpgradeProtossAirWeapons: {
		Race: RaceProtoss, BuildingSubject: GeneralUnitCyberneticsCore, MaxLevel: 3,
		Levels: [3]UpgradeLevelMeta{
			{Minerals: 100, Gas: 100, DurationS: tieredWeaponArmorL1S},
			{Minerals: 175, Gas: 175, DurationS: tieredWeaponArmorL2S},
			{Minerals: 250, Gas: 250, DurationS: tieredWeaponArmorL3S},
		},
	},

	// ===== Protoss one-shot =====
	UpgradeSingularityChargeDragoonRange: oneShot(RaceProtoss, GeneralUnitCyberneticsCore, 150, 150, 105),
	UpgradeLegEnhancementZealotSpeed:     oneShot(RaceProtoss, GeneralUnitCitadelOfAdun, 150, 150, 84),
	UpgradeGraviticDriveShuttleSpeed:     oneShot(RaceProtoss, GeneralUnitRoboticsSupportBay, 200, 200, 105),
	UpgradeReaverCapacity:                oneShot(RaceProtoss, GeneralUnitRoboticsSupportBay, 200, 200, 105),
	UpgradeScarabDamage:                  oneShot(RaceProtoss, GeneralUnitRoboticsSupportBay, 200, 200, 105),
	UpgradeSensorArrayObserverSight:      oneShot(RaceProtoss, GeneralUnitObservatory, 150, 150, 84),
	UpgradeGraviticBoosterObserverSpeed:  oneShot(RaceProtoss, GeneralUnitObservatory, 150, 150, 84),
	UpgradeKhaydarinAmuletTemplarEnergy:  oneShot(RaceProtoss, GeneralUnitTemplarArchives, 150, 150, 105),
	UpgradeArgusTalismanDarkArchonEnergy: oneShot(RaceProtoss, GeneralUnitTemplarArchives, 150, 150, 105),
	UpgradeKhaydarinCoreArbiterEnergy:    oneShot(RaceProtoss, GeneralUnitArbiterTribunal, 150, 150, 105),
	UpgradeApialSensorsScoutSight:        oneShot(RaceProtoss, GeneralUnitFleetBeacon, 100, 100, 105),
	UpgradeGraviticThrustersScoutSpeed:   oneShot(RaceProtoss, GeneralUnitFleetBeacon, 200, 200, 105),
	UpgradeCarrierCapacity:               oneShot(RaceProtoss, GeneralUnitFleetBeacon, 100, 100, 63),
	UpgradeArgusJewelCorsairEnergy:       oneShot(RaceProtoss, GeneralUnitFleetBeacon, 100, 100, 105),
}

func oneShot(race, building string, minerals, gas int, durationS float64) UpgradeMeta {
	return UpgradeMeta{
		Race: race, BuildingSubject: building, MaxLevel: 1,
		Levels: [3]UpgradeLevelMeta{{Minerals: minerals, Gas: gas, DurationS: durationS}},
	}
}

// LookupUpgrade returns the static metadata for an upgrade. Returns false for
// unknown names — callers should treat that as "unknown / pass-through".
func LookupUpgrade(name string) (UpgradeMeta, bool) {
	m, ok := upgradeTable[name]
	return m, ok
}

// IsHPUpgrade reports whether an upgrade name is a tiered weapon/armor/shield
// upgrade — the "HP Upgrades" group. These are the only upgrades that count
// toward the Never-Upgraded marker; every other (one-shot) upgrade is a
// research and counts toward Never-Researched instead. Unknown names are not
// treated as HP upgrades.
func IsHPUpgrade(name string) bool {
	m, ok := upgradeTable[name]
	return ok && m.MaxLevel == 3
}
