package models

// Tech (research) game-knowledge keyed by Tech* name constants.
//
// "Tech" here matches screp's classification: one-shot researches that enable
// a cast ability or unit morph (Stim Packs, Lurker Aspect, Plague…) — as
// distinct from passive Upgrades which can be tiered.
//
// Source: https://liquipedia.net/starcraft/Upgrades — Fastest game speed.
// Times are in-game seconds (the same unit Command.SecondsFromGameStart uses).
//
// Default abilities that don't require research (Defensive Matrix, Scanner
// Sweep, Parasite, Infestation, Dark Swarm, Archon Warp, Dark Archon Meld,
// Healing, Feedback) are intentionally omitted — they may appear in techs.go
// for completeness of the screp enum but are not researched by players.

// TechMeta is the static cost/timing footprint of a Tech research.
type TechMeta struct {
	Race            string  // RaceTerran / RaceZerg / RaceProtoss
	BuildingSubject string  // GeneralUnit* constant naming the building that researches it
	Hotkey          string  // single-letter; "" when unknown / not yet populated
	Minerals        int
	Gas             int
	DurationS       float64 // research time in seconds at Fastest game speed
}

// TechTable is the lookup table from Tech* name → TechMeta. Kept private; use
// LookupTech to read.
var techTable = map[string]TechMeta{
	// Terran
	TechStimPacks:         {Race: RaceTerran, BuildingSubject: GeneralUnitAcademy, Minerals: 100, Gas: 100, DurationS: 50.4},
	TechRestoration:       {Race: RaceTerran, BuildingSubject: GeneralUnitAcademy, Minerals: 100, Gas: 100, DurationS: 50.4},
	TechOpticalFlare:      {Race: RaceTerran, BuildingSubject: GeneralUnitAcademy, Minerals: 100, Gas: 100, DurationS: 75.6},
	TechLockdown:          {Race: RaceTerran, BuildingSubject: GeneralUnitCovertOps, Minerals: 200, Gas: 200, DurationS: 63},
	TechPersonnelCloaking: {Race: RaceTerran, BuildingSubject: GeneralUnitCovertOps, Minerals: 100, Gas: 100, DurationS: 50},
	TechSpiderMines:       {Race: RaceTerran, BuildingSubject: GeneralUnitMachineShop, Minerals: 100, Gas: 100, DurationS: 50.4},
	TechTankSiegeMode:     {Race: RaceTerran, BuildingSubject: GeneralUnitMachineShop, Minerals: 150, Gas: 150, DurationS: 50.4},
	TechEMPShockwave:      {Race: RaceTerran, BuildingSubject: GeneralUnitScienceFacility, Minerals: 200, Gas: 200, DurationS: 75.6},
	TechIrradiate:         {Race: RaceTerran, BuildingSubject: GeneralUnitScienceFacility, Minerals: 200, Gas: 200, DurationS: 50.4},
	TechCloakingField:     {Race: RaceTerran, BuildingSubject: GeneralUnitControlTower, Minerals: 150, Gas: 150, DurationS: 63},
	TechYamatoGun:         {Race: RaceTerran, BuildingSubject: GeneralUnitPhysicsLab, Minerals: 100, Gas: 100, DurationS: 75.6},

	// Zerg
	TechBurrowing:       {Race: RaceZerg, BuildingSubject: GeneralUnitHatchery, Minerals: 100, Gas: 100, DurationS: 80},
	TechLurkerAspect:    {Race: RaceZerg, BuildingSubject: GeneralUnitHydraliskDen, Minerals: 200, Gas: 200, DurationS: 75.6},
	TechSpawnBroodlings: {Race: RaceZerg, BuildingSubject: GeneralUnitQueensNest, Minerals: 100, Gas: 100, DurationS: 50},
	TechEnsnare:         {Race: RaceZerg, BuildingSubject: GeneralUnitQueensNest, Minerals: 100, Gas: 100, DurationS: 50},
	TechPlague:          {Race: RaceZerg, BuildingSubject: GeneralUnitDefilerMound, Minerals: 200, Gas: 200, DurationS: 63},
	TechConsume:         {Race: RaceZerg, BuildingSubject: GeneralUnitDefilerMound, Minerals: 100, Gas: 100, DurationS: 63},

	// Protoss
	TechPsionicStorm:  {Race: RaceProtoss, BuildingSubject: GeneralUnitTemplarArchives, Minerals: 200, Gas: 200, DurationS: 75.6},
	TechHallucination: {Race: RaceProtoss, BuildingSubject: GeneralUnitTemplarArchives, Minerals: 150, Gas: 150, DurationS: 50.4},
	TechMaelstrom:     {Race: RaceProtoss, BuildingSubject: GeneralUnitTemplarArchives, Minerals: 100, Gas: 100, DurationS: 63},
	TechMindControl:   {Race: RaceProtoss, BuildingSubject: GeneralUnitTemplarArchives, Minerals: 200, Gas: 200, DurationS: 75.6},
	TechStasisField:   {Race: RaceProtoss, BuildingSubject: GeneralUnitArbiterTribunal, Minerals: 150, Gas: 150, DurationS: 63},
	TechRecall:        {Race: RaceProtoss, BuildingSubject: GeneralUnitArbiterTribunal, Minerals: 150, Gas: 150, DurationS: 76},
	TechDisruptionWeb: {Race: RaceProtoss, BuildingSubject: GeneralUnitFleetBeacon, Minerals: 200, Gas: 200, DurationS: 50},
}

// LookupTech returns the static metadata for a researched tech. The bool is
// false for default abilities (Feedback, Parasite, …) and unused enum values
// (Unused 26, Unused 33, Archon Warp, …) — callers should treat that as
// "unknown / pass-through" rather than free.
func LookupTech(name string) (TechMeta, bool) {
	m, ok := techTable[name]
	return m, ok
}
