package markers

import (
	"math"

	"github.com/marianogappa/screpdb/internal/models"
)

// secAfter is shorthand for "second anchor + N build-time increments, rounded".
// Exists because some SC:BW build times are fractional (Zealot 25.2s) and
// target seconds must be ints.
func secAfter(anchor int, addends ...float64) int {
	total := float64(anchor)
	for _, a := range addends {
		total += a
	}
	return int(math.Round(total))
}

// -----------------------------------------------------------------------------
// Build order definitions. Add / edit / remove entries here — everything else
// (detectors, game-list featuring, UI pills, Build Orders tab) picks up
// changes via the registry below.
//
// Conventions:
//   - Subjects use the canonical in-game unit names from internal/models.
//   - Times are in seconds from game start.
//   - Tolerance defaults to ±5s when the user spec didn't give one.
//   - The expert "First X" event keys are named exactly how they should render
//     in the UI timeline.
//   - RuleDeadline is the last second the Rule could flip; the detector
//     finalizes past that point and frees its event buffer.
// -----------------------------------------------------------------------------

// Subject shorthand — kept in one place so typos are easy to catch.
const (
	// Zerg
	subjSpawningPool     = models.GeneralUnitSpawningPool
	subjHatchery         = models.GeneralUnitHatchery
	subjEvolutionChamber = models.GeneralUnitEvolutionChamber
	subjDrone            = models.GeneralUnitDrone
	subjOverlord         = models.GeneralUnitOverlord
	subjZergling         = models.GeneralUnitZergling
	subjSpire            = models.GeneralUnitSpire
	subjMutalisk         = models.GeneralUnitMutalisk

	// Protoss
	subjNexus           = models.GeneralUnitNexus
	subjPylon           = models.GeneralUnitPylon
	subjGateway         = models.GeneralUnitGateway
	subjAssimilator     = models.GeneralUnitAssimilator
	subjCyberneticsCore = models.GeneralUnitCyberneticsCore
	subjForge           = models.GeneralUnitForge
	subjPhotonCannon    = models.GeneralUnitPhotonCannon
	subjZealot          = models.GeneralUnitZealot
	subjScout           = models.GeneralUnitScout
	subjCarrier         = models.GeneralUnitCarrier

	// Terran
	subjCommandCenter  = models.GeneralUnitCommandCenter
	subjSupplyDepot    = models.GeneralUnitSupplyDepot
	subjBarracks       = models.GeneralUnitBarracks
	subjRefinery       = models.GeneralUnitRefinery
	subjAcademy        = models.GeneralUnitAcademy
	subjFactory        = models.GeneralUnitFactory
	subjStarport       = models.GeneralUnitStarport
	subjEngineeringBay = models.GeneralUnitEngineeringBay
	subjMissileTurret  = models.GeneralUnitMissileTurret
	subjBunker         = models.GeneralUnitBunker
	subjMedic          = models.GeneralUnitMedic
	subjVulture        = models.GeneralUnitVulture
	subjGoliath        = models.GeneralUnitGoliath
	subjSiegeTank      = models.GeneralUnitSiegeTankTankMode
	subjBattlecruiser  = models.GeneralUnitBattlecruiser
)

// endOfReplaySentinel is a RuleDeadline for markers whose answer can only
// be resolved at end-of-replay (e.g. "never upgraded", "Carriers produced
// at any point"). Well past any realistic SC:BW replay length; the detector
// will still Finalize when the replay actually ends.
const endOfReplaySentinel = 10 * 60 * 60 // 10 hours

// Default tolerance used when the user spec did not give one.
var defaultTol = Sym(5)

// All orders in a stable, UI-facing order. Defined as a func so initialization
// order doesn't trip on cross-file references.
func allMarkers() []Marker {
	return []Marker{
		// Pool-first BOs are keyed off exact pre-Pool Drone-morph and
		// Overlord-morph counts. The early-game spam filter (internal/
		// earlyfilter) strips engine-impossible morphs so the surviving
		// stream is a faithful supply count: 4 starting drones + N kept
		// Drone morphs. Hatchery / Evolution Chamber must not precede
		// the Pool, else it's a hatch-first BO. Timings live in the
		// Expert events (UI golden compare) only.
		{
			Name:        "4 Pool",
			PatternName: "Build Order: 4 Pool",
			FeatureKey:  "bo_4_pool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 4 Pool = supply 4 at Pool placement: 0 drones, 0 overlords.
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 0),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 0),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
			),
			RuleDeadline: 60,
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 33,
					Tolerance:    Sym(4),
				},
				{
					// First Zergling pops one Pool build-time after the Pool.
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: secAfter(33, models.BuildTimeSpawningPool),
					Tolerance:    Sym(3),
				},
			},
			SummaryPlayer: &Pill{Label: "4 Pool", IconKey: "spawningpool"},
			GamesList:     &Pill{Label: "4 Pool", IconKey: "spawningpool"},
		},
		{
			Name:        "9 Pool",
			PatternName: "Build Order: 9 Pool",
			FeatureKey:  "bo_9_pool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 9 Pool = supply 9 at Pool placement: 5 drone morphs and
				// no Overlord yet (Overlord follows the Pool). The 9-
				// Overpool variant — same drone count but with the
				// Overlord already morphed — is its own BO.
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 5),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 0),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
				// Mutex with "9 Pool into Hatchery" — fast follow-up Hatch
				// belongs to that BO, not plain 9 Pool.
				Not(BuildAfterWithin(subjHatchery, subjSpawningPool, 60)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 73,
					Tolerance:    Sym(4),
				},
				{
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: secAfter(73, models.BuildTimeSpawningPool),
					Tolerance:    Sym(3),
				},
			},
			SummaryPlayer: &Pill{Label: "9 Pool", IconKey: "spawningpool"},
			GamesList:     &Pill{Label: "9 Pool", IconKey: "spawningpool"},
		},
		{
			Name:        "9 Overpool",
			PatternName: "Build Order: 9 Overpool",
			FeatureKey:  "bo_9_overpool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 9 Overpool = supply 9 at Pool placement, but the
				// Overlord was morphed before the Pool (vs plain 9 Pool
				// where Pool comes first). Same 5 Drone morphs.
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 5),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 1),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 80,
					Tolerance:    Sym(5),
				},
				{
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: secAfter(80, models.BuildTimeSpawningPool),
					Tolerance:    Sym(4),
				},
			},
			SummaryPlayer: &Pill{Label: "9 Overpool", IconKey: "spawningpool"},
			GamesList:     &Pill{Label: "9 Overpool", IconKey: "spawningpool"},
		},
		{
			Name:        "12 Pool",
			PatternName: "Build Order: 12 Pool",
			FeatureKey:  "bo_12_pool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 12 Pool = supply 12 at Pool: 4 starting + 8 drone
				// morphs + 1 Overlord (Overlord required to lift the
				// supply cap past 9 to reach 12).
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 8),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 1),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 104,
					Tolerance:    Sym(5),
				},
				{
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: secAfter(104, models.BuildTimeSpawningPool),
					Tolerance:    Sym(4),
				},
			},
			SummaryPlayer: &Pill{Label: "12 Pool", IconKey: "spawningpool"},
			GamesList:     &Pill{Label: "12 Pool", IconKey: "spawningpool"},
		},
		{
			Name:        "9 Pool into Hatchery",
			PatternName: "Build Order: 9 Pool into Hatchery",
			FeatureKey:  "bo_9_pool_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Same prefix as 9 Pool...
				ProduceBeforeBuild(subjDrone, subjSpawningPool),
				NoProduceBeforeBuild(subjOverlord, subjSpawningPool),
				FirstBuildAtOrAfter(subjSpawningPool, 70),
				FirstBuildBefore(subjSpawningPool, 120),
				// ...plus: "hatchery is built within 1 minute after pool"
				BuildAfterWithin(subjHatchery, subjSpawningPool, 60),
			),
			RuleDeadline: 180, // pool ≤120 + hatch ≤60 after pool
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 73,
					Tolerance:    Sym(4),
				},
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 118,
					Tolerance:    Sym(5),
				},
				{
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: secAfter(73, models.BuildTimeSpawningPool),
					Tolerance:    Sym(3),
				},
			},
			SummaryPlayer: &Pill{Label: "9 Pool → Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "9 Pool → Hatch", IconKey: "hatchery"},
		},
		{
			Name:        "9 Hatch",
			PatternName: "Build Order: 9 Hatch",
			FeatureKey:  "bo_9_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 9 Hatch first = supply 9 at Hatch placement: 5 drone
				// morphs, no Overlord yet (supply cap 9 blocks further
				// morphs anyway). Pool / Evo Chamber must not precede.
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 5),
				ProduceCountBeforeBuild(subjOverlord, subjHatchery, 0),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 150,
			Expert: []ExpertEvent{
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 73, // 1m13
					Tolerance:    Sym(4),
				},
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 103, // 1m43
					Tolerance:    Asym(6, 10),
				},
			},
			SummaryPlayer: &Pill{Label: "9 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "9 Hatch", IconKey: "hatchery"},
		},
		// Hatch-first BOs are keyed off exact pre-Hatch drone / overlord
		// counts. Spawning Pool / Evolution Chamber must not precede the
		// expansion Hatchery (else it'd be a Pool-tech opening). All
		// three of 10 / 11 / 12 Hatch require an Overlord first because
		// reaching supply >9 demands cap expansion.
		{
			Name:        "10 Hatch",
			PatternName: "Build Order: 10 Hatch",
			FeatureKey:  "bo_10_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 6),
				ProduceCountBeforeBuild(subjOverlord, subjHatchery, 1),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 80,
					Tolerance:    defaultTol,
				},
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 110,
					Tolerance:    Asym(3, 10),
				},
			},
			SummaryPlayer: &Pill{Label: "10 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "10 Hatch", IconKey: "hatchery"},
		},
		{
			Name:        "11 Hatch",
			PatternName: "Build Order: 11 Hatch",
			FeatureKey:  "bo_11_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 7),
				ProduceCountBeforeBuild(subjOverlord, subjHatchery, 1),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 94,
					Tolerance:    defaultTol,
				},
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 116,
					Tolerance:    Asym(3, 10),
				},
			},
			SummaryPlayer: &Pill{Label: "11 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "11 Hatch", IconKey: "hatchery"},
		},
		{
			Name:        "12 Hatch",
			PatternName: "Build Order: 12 Hatch",
			FeatureKey:  "bo_12_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 8),
				ProduceCountBeforeBuild(subjOverlord, subjHatchery, 1),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 98,
					Tolerance:    defaultTol,
				},
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 116,
					Tolerance:    Asym(3, 10),
				},
			},
			SummaryPlayer: &Pill{Label: "12 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "12 Hatch", IconKey: "hatchery"},
		},
		// -------------------------------------------------------------------
		// Protoss openers (matchup-gated). Sourced from a 3000-replay
		// progamer mining run (1v1 melee). Frequencies cited per matchup
		// in the per-BO comments are pre-detection raw building-sequence
		// frequencies; expected post-detector hit rates will be similar.
		//
		// Mutex within (Protoss, matchup):
		//   * 1 Gate Core requires Cyber before Nexus AND before 2nd Gate.
		//   * 2 Gate requires 2 Gateways before Cyber/Nexus/Forge.
		//   * Nexus First requires Nexus before Gateway AND before Forge.
		//   * Gate Expand (PvZ only) requires Gateway before Forge AND Nexus.
		//   * Forge Expand (PvZ only) requires Forge before Gateway AND Nexus.
		// -------------------------------------------------------------------

		{
			// 1 Gate Core: ~47% of PvP and ~47% of PvT in the dataset.
			// Pylon, Gateway, Assimilator, Cybernetics Core. Foundation for
			// Goon Range / DT / Reaver tech. Not used as the dominant
			// PvZ opener (FFE / Gate FE are preferred), so PvZ excluded.
			Name:        "1 Gate Core",
			PatternName: "Build Order: 1 Gate Core",
			FeatureKey:  "bo_1_gate_core",
			Race:        RaceProtoss,
			Matchup:     []string{"PvP", "PvT"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 1 Gate before Cyber Core (canonical).
				BuildBefore(subjGateway, subjCyberneticsCore),
				// Cyber Core within window — distinguishes from "Gateway
				// only" or 2-Gate (where Cyber is delayed past 180s).
				FirstBuildBefore(subjCyberneticsCore, 180),
				// vs 2 Gate: Cyber arrives before any 2nd Gateway.
				Not(NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore})),
				// vs Nexus First: Cyber before Nexus.
				BuildBefore(subjCyberneticsCore, subjNexus),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(4)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 86, Tolerance: Sym(6)},
				{Key: "Assimilator", Match: MatchBuild(subjAssimilator), TargetSecond: 116, Tolerance: Sym(10)},
				{Key: "Cybernetics Core", Match: MatchBuild(subjCyberneticsCore), TargetSecond: 138, Tolerance: Sym(10)},
			},
			SummaryPlayer: &Pill{Label: "1 Gate Core", IconKey: "cyberneticscore"},
			GamesList:     &Pill{Label: "1 Gate Core", IconKey: "cyberneticscore"},
		},
		{
			// 2 Gate: ~11% of PvP, ~4% of PvZ in the dataset (rarer in PvT).
			// Pylon, Gateway, Gateway, Pylon. Pressure / Zealot rush. Both
			// Gateways must precede Cyber Core, Nexus, and Forge.
			Name:        "2 Gate",
			PatternName: "Build Order: 2 Gate",
			FeatureKey:  "bo_2_gate",
			Race:        RaceProtoss,
			Matchup:     []string{"PvP", "PvT", "PvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore, subjNexus, subjForge}),
				CountBuildsBefore(subjGateway, 2, 180),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(4)},
				{Key: "1st Gateway", Match: MatchBuild(subjGateway), TargetSecond: 70, Tolerance: Sym(6)},
				{Key: "2nd Gateway", Match: MatchNthBuild(subjGateway, 2), TargetSecond: 86, Tolerance: Sym(10)},
				{
					// First Zealot can be queued the moment the 1st Gateway
					// completes: 70 + Gateway build time = ~108s.
					Key:          "First Zealot",
					Match:        MatchFirstProduce(subjZealot),
					TargetSecond: secAfter(70, models.BuildTimeGateway),
					Tolerance:    Sym(3),
				},
			},
			SummaryPlayer: &Pill{Label: "2 Gate", IconKey: "gateway"},
			GamesList:     &Pill{Label: "2 Gate", IconKey: "gateway"},
		},
		{
			// Nexus First: ~10% of PvP, ~14% of PvT, smaller in PvZ.
			// Pioneered by Bisu (NeoSair-style greedy expand). Pylon, Nexus,
			// Gateway. Loosened upper bound from the legacy 150s rule because
			// the data shows Nexus placement up to ~170s in PvT.
			Name:        "Nexus First",
			PatternName: "Build Order: Nexus First",
			FeatureKey:  "bo_nexus_first",
			Race:        RaceProtoss,
			Matchup:     []string{"PvP", "PvT", "PvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				BuildBefore(subjNexus, subjGateway),
				BuildBefore(subjNexus, subjForge),
				FirstBuildBefore(subjNexus, 200),
			),
			RuleDeadline: 200,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(4)},
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 145, Tolerance: Sym(20)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 175, Tolerance: Sym(20)},
			},
			SummaryPlayer: &Pill{Label: "Nexus First", IconKey: "nexus"},
			GamesList:     &Pill{Label: "Nexus First", IconKey: "nexus"},
		},
		{
			// Gate Expand (Gate FE): ~24% of PvZ (data combines variants
			// like P-G-P-N-P/F/A/N). Single Gateway then Nexus, no Forge
			// yet. Mirrors FFE in PvZ frequency at progamer level.
			Name:        "Gate Expand",
			PatternName: "Build Order: Gate Expand",
			FeatureKey:  "bo_gate_expand",
			Race:        RaceProtoss,
			Matchup:     []string{"PvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Gateway before Forge AND Nexus.
				BuildBefore(subjGateway, subjForge),
				BuildBefore(subjGateway, subjNexus),
				// Nexus built within window.
				FirstBuildBefore(subjNexus, 200),
				// Mutex with 2 Gate: only ONE Gateway before Nexus.
				Not(NthBuildBeforeAll(subjGateway, 2, []string{subjNexus})),
			),
			RuleDeadline: 200,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(4)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 88, Tolerance: Sym(10)},
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 165, Tolerance: Sym(15)},
			},
			SummaryPlayer: &Pill{Label: "Gate Expand", IconKey: "nexus"},
			GamesList:     &Pill{Label: "Gate Expand", IconKey: "nexus"},
		},
		{
			// Forge Expand (FFE): ~20% of PvZ (data combines defensive
			// variants P-F-N-H-G, P-F-F-N-H, P-F-H-H-N, P-F-H-N-G).
			// Pylon, Forge, optional Cannon, Nexus, then Gateway. Loosened
			// the legacy upper bound from 90s to 100s for the Forge to
			// admit slower openers.
			Name:        "Forge Expand",
			PatternName: "Build Order: Forge Expand",
			FeatureKey:  "bo_forge_expa",
			Race:        RaceProtoss,
			Matchup:     []string{"PvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				FirstBuildBefore(subjForge, 100),
				BuildBefore(subjForge, subjGateway),
				BuildBefore(subjForge, subjNexus),
				FirstBuildBefore(subjNexus, 200),
				BuildBefore(subjNexus, subjGateway),
			),
			RuleDeadline: 200,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(4)},
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 86, Tolerance: Sym(8)},
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 130, Tolerance: Sym(20)},
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 152, Tolerance: Sym(15)},
			},
			SummaryPlayer: &Pill{Label: "FFE", IconKey: "forge"},
			GamesList:     &Pill{Label: "FFE", IconKey: "forge"},
		},

		// -------------------------------------------------------------------
		// Terran openers (matchup-gated). Sourced from the same dataset as
		// the Protoss block. TvT is heavily monolithic (~57% 1 Rax 1 Fac);
		// TvZ is the diverse matchup with at least 4 distinct families.
		//
		// Mutex within (Terran, matchup):
		//   * BBS commits at 2nd Rax before any Depot — disjoint from all
		//     others (which all have Depot first).
		//   * CC First requires CC before any Rax — disjoint from Rax-CC
		//     and 1 Rax 1 Fac.
		//   * Rax-CC requires CC before Refinery AND Factory, with ≤1 Rax
		//     before CC — disjoint from 1 Rax 1 Fac (Refinery before Factory)
		//     and from BBS (2 Rax before Depot).
		//   * 1 Rax 1 Fac requires Refinery before Factory and CC neither
		//     before Refinery nor before Factory.
		// -------------------------------------------------------------------

		{
			// 1 Rax 1 Fac: ~48% of TvT and ~45% of TvP, ~12% of TvZ.
			// Depot, Rax, Refinery, Depot, Factory. Foundation for 1-1-1,
			// 2-Fac Vulture, SK Terran. Refinery precedes Factory and CC
			// (gas-before-CC discriminates from Rax-CC).
			Name:        "1 Rax 1 Fac",
			PatternName: "Build Order: 1 Rax 1 Fac",
			FeatureKey:  "bo_1_rax_1_fac",
			Race:        RaceTerran,
			Matchup:     []string{"TvT", "PvT", "TvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Refinery built before Factory (gas before tech).
				BuildBefore(subjRefinery, subjFactory),
				FirstBuildBefore(subjFactory, 240),
				// ≤1 Rax before Factory (else it's 2-Rax style).
				Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjFactory})),
				// Not CC-first: Barracks must precede CC if CC happens.
				Not(BuildBefore(subjCommandCenter, subjBarracks)),
				// Mutex with Rax-CC: gas before CC, factory before CC.
				BuildBefore(subjRefinery, subjCommandCenter),
				BuildBefore(subjFactory, subjCommandCenter),
			),
			RuleDeadline: 240,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 88, Tolerance: Sym(8)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 115, Tolerance: Sym(12)},
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 165, Tolerance: Sym(15)},
			},
			SummaryPlayer: &Pill{Label: "1 Rax 1 Fac", IconKey: "factory"},
			GamesList:     &Pill{Label: "1 Rax 1 Fac", IconKey: "factory"},
		},
		{
			// CC First: ~6% of TvT, ~9% of TvP, ~10% of TvZ (combining
			// D-C-B-D-R and D-B-C-D-R variants — actually only the former
			// is true CC-first; the latter falls under Rax-CC). True CC
			// First is Depot, CC, Rax. Risky vs Protoss without map help;
			// canonical for greedy macro vs Z.
			Name:        "CC First",
			PatternName: "Build Order: CC First",
			FeatureKey:  "bo_cc_first",
			Race:        RaceTerran,
			Matchup:     []string{"TvT", "PvT", "TvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				BuildBefore(subjCommandCenter, subjBarracks),
				FirstBuildBefore(subjCommandCenter, 200),
			),
			RuleDeadline: 200,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 62, Tolerance: Sym(8)},
				{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 145, Tolerance: Sym(20)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 165, Tolerance: Sym(20)},
			},
			SummaryPlayer: &Pill{Label: "CC First", IconKey: "commandcenter"},
			GamesList:     &Pill{Label: "CC First", IconKey: "commandcenter"},
		},
		{
			// Rax-CC: ~11% of TvP, ~15% of TvZ (combining D-B-C-D-R and
			// D-B-D-C-R/B variants). Depot, Rax, CC before any gas, before
			// Factory. The 2nd Rax — when present — typically arrives
			// AFTER the CC (e.g. D-B-D-C-B), which is why a separate
			// "2-Rax CC" BO would over-fragment the data.
			Name:        "Rax-CC",
			PatternName: "Build Order: Rax-CC",
			FeatureKey:  "bo_rax_cc",
			Race:        RaceTerran,
			Matchup:     []string{"TvT", "PvT", "TvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				BuildBefore(subjBarracks, subjCommandCenter),
				// 1 Rax before CC (else it's a 2-Rax variant).
				Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjCommandCenter})),
				// CC before any gas / tech.
				BuildBefore(subjCommandCenter, subjFactory),
				BuildBefore(subjCommandCenter, subjRefinery),
				FirstBuildBefore(subjCommandCenter, 220),
			),
			RuleDeadline: 220,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 88, Tolerance: Sym(10)},
				{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 180, Tolerance: Sym(18)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 195, Tolerance: Sym(18)},
			},
			SummaryPlayer: &Pill{Label: "Rax-CC", IconKey: "commandcenter"},
			GamesList:     &Pill{Label: "Rax-CC", IconKey: "commandcenter"},
		},
		{
			// BBS: confirmed in the dataset (e.g. SST_JumJaJungJi opens
			// BBS in many TvZs: Rax @58s, Rax @79s, Depot @100s, Bunker
			// @157s). All-in 2-Rax before any other Terran building. Rare
			// in modern pro play but a recognizable signature.
			Name:        "BBS",
			PatternName: "Build Order: BBS",
			FeatureKey:  "bo_bbs",
			Race:        RaceTerran,
			Matchup:     []string{"TvT", "PvT", "TvZ"},
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// 2 Barracks before any other Terran building. The "all
				// other buildings" guard is what locks BBS topology: no
				// Depot, no Refinery, no CC, no Factory until both Rax
				// are down. (Mutex with Rax-CC, which permits a CC after
				// the 1st Rax.)
				NthBuildBeforeAll(subjBarracks, 2, []string{
					subjSupplyDepot, subjRefinery, subjCommandCenter,
					subjFactory, subjStarport, subjAcademy,
					subjEngineeringBay, subjBunker,
				}),
				// 2nd Rax built before 120s — disambiguates from a slow
				// "delayed 2nd Rax" stream (where the player simply skips
				// production for a long time then drops a 2nd Rax).
				CountBuildsBefore(subjBarracks, 2, 120),
				// First Rax built before 100s — BBS pivot timings.
				FirstBuildBefore(subjBarracks, 100),
			),
			RuleDeadline: 120,
			Expert: []ExpertEvent{
				{Key: "1st Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "2nd Barracks", Match: MatchNthBuild(subjBarracks, 2), TargetSecond: 80, Tolerance: Sym(8)},
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 100, Tolerance: Sym(10)},
			},
			SummaryPlayer: &Pill{Label: "BBS", IconKey: "barracks"},
			GamesList:     &Pill{Label: "BBS", IconKey: "barracks"},
		},

		// -------------------------------------------------------------------
		// KindMarker entries. These may coexist with each other and with a
		// KindInitialBuildOrder. Bool-only via Rule; PatternName kept equal
		// to the old imperative detector's Name() so DB rows + frontend
		// checks stay compatible.
		// -------------------------------------------------------------------

		{
			// Mech: pure-Mech signal. ≥2 Factories built before 6:30 AND
			// no Academy ever (Academy means SK / bio path). Replaces the
			// older "Quick factory" marker which fired too eagerly on
			// 1-Fac TvP openers that immediately go bio anyway.
			Name:         "Mech",
			PatternName:  "Mech",
			FeatureKey:   "mech",
			Kind:         KindMarker,
			Race:         RaceTerran,
			Rule: All(
				CountBuildsBefore(subjFactory, 2, 390),
				Not(FirstBuildExists(subjAcademy)),
			),
			RuleDeadline:  390,
			SummaryPlayer: &Pill{IconKey: "siegetank", Style: PillStyleStrong, Title: "Mech"},
			GamesList:     &Pill{IconKey: "siegetank", Style: PillStyleStrong, Title: "Mech"},
		},
		{
			// 1-1-1: Barracks → Factory → Starport transition. Starport is
			// the discriminator; first Starport averages 206-220s in this
			// dataset, so we extend the deadline to 6 minutes to capture
			// stragglers. Independent of opener — fires on top of
			// "1 Rax 1 Fac", "Rax-CC", etc.
			//
			// MapKind gate: on Money games every Terran builds Rax+Fac+Starport
			// because resources are free, so the marker carries no strategic
			// signal. Restrict to non-Money maps.
			Name:        "1-1-1",
			PatternName: "1-1-1",
			FeatureKey:  "one_one_one",
			Kind:        KindMarker,
			Race:        RaceTerran,
			MapKind:     []string{"Regular", "UseMapSettings"},
			Rule: All(
				FirstBuildExists(subjBarracks),
				FirstBuildExists(subjFactory),
				FirstBuildBefore(subjStarport, 360),
			),
			RuleDeadline:  360,
			SummaryPlayer: &Pill{Label: "1-1-1", IconKey: "starport"},
			GamesList:     &Pill{Label: "1-1-1", IconKey: "starport"},
		},
		{
			// SK Terran: Marine-Medic-heavy bio with Engineering Bay
			// upgrades. Signals: Academy + Medic produced + Engineering
			// Bay all built before 8 minutes. Most relevant in TvP and
			// TvZ — TvT rarely goes bio.
			Name:        "SK Terran",
			PatternName: "SK Terran",
			FeatureKey:  "sk_terran",
			Kind:        KindMarker,
			Race:        RaceTerran,
			Matchup:     []string{"PvT", "TvZ"},
			Rule: All(
				FirstBuildBefore(subjAcademy, 480),
				FirstBuildBefore(subjEngineeringBay, 480),
				FirstProduceExists(subjMedic),
			),
			RuleDeadline:  480,
			SummaryPlayer: &Pill{IconKey: "medic", Style: PillStyleStrong, Title: "SK Terran"},
			GamesList:     &Pill{IconKey: "medic", Style: PillStyleStrong, Title: "SK Terran"},
		},
		{
			// Mech transition (TvZ only): bio start (Medic produced ≤330s)
			// followed by ≥2 Factories AND a Vulture/Tank/Goliath produced
			// after the 2nd Factory. Separate from "Mech" (which excludes
			// Academy) — this fires when the player committed to bio first
			// then pivoted.
			Name:          "Mech transition",
			PatternName:   "Mech transition",
			FeatureKey:    "mech_transition",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Matchup:       []string{"TvZ"},
			Custom:        func() CustomEvaluator { return &mechTransitionEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Mech transition at min {minute}", IconKey: "siegetank"},
			GamesList:     &Pill{Label: "Mech transition", IconKey: "siegetank"},
		},
		{
			// Mutalisk timing (Z side) — fires iff opponent (T) also
			// matches the turret-timing burst. Coupled with the
			// turret_timing marker below via a shared cross-player gate
			// in mutalisk_turret_timing.go that walks the worldstate
			// engine's full enriched stream at Finalize.
			//
			// No per-event Expert tolerance bands: the only progamer
			// reference baked into this marker is the muta-vs-turret
			// completion gap (median + p25/p75 from the cwal-dl 1v1 TvZ
			// corpus), surfaced on the Mutalisk Timing tab — see
			// populateMutaliskTimingForGameDetail.
			Name:         "Mutalisk timing",
			PatternName:  "Mutalisk timing",
			FeatureKey:   "mutalisk_timing",
			Kind:         KindMarker,
			Race:         RaceZerg,
			Matchup:      []string{"TvZ"},
			Custom:       func() CustomEvaluator { return &mutaTimingEvaluator{} },
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Mutalisk timing {timestamp}", IconKey: "mutalisk"},
			SummaryReplay: &Pill{Label: "Mutalisk timing {timestamp}", IconKey: "mutalisk"},
			GamesList:     &Pill{Label: "Mutalisk timing {timestamp}", IconKey: "mutalisk"},
		},
		{
			Name:         "Turret timing",
			PatternName:  "Turret timing",
			FeatureKey:   "turret_timing",
			Kind:         KindMarker,
			Race:         RaceTerran,
			Matchup:      []string{"TvZ"},
			Custom:       func() CustomEvaluator { return &turretTimingEvaluator{} },
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Turret timing {timestamp}", IconKey: "missileturret"},
			SummaryReplay: &Pill{Label: "Turret timing {timestamp}", IconKey: "missileturret"},
			GamesList:     &Pill{Label: "Turret timing {timestamp}", IconKey: "missileturret"},
		},
		{
			Name:         "Carriers",
			PatternName:  "Carriers",
			FeatureKey:   "carriers",
			Kind:         KindMarker,
			Race:         RaceProtoss,
			Rule:         FirstProduceExists(subjCarrier),
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{IconKey: "carrier", Style: PillStyleStrong, Title: "Carriers"},
			GamesList:     &Pill{IconKey: "carrier", Style: PillStyleStrong, Title: "Carriers"},
		},
		{
			Name:         "Battlecruisers",
			PatternName:  "Battlecruisers",
			FeatureKey:   "battlecruisers",
			Kind:         KindMarker,
			Race:         RaceTerran,
			Rule:         FirstProduceExists(subjBattlecruiser),
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{IconKey: "battlecruiser", Style: PillStyleStrong, Title: "Battlecruisers"},
			GamesList:     &Pill{IconKey: "battlecruiser", Style: PillStyleStrong, Title: "Battlecruisers"},
		},
		{
			// 10+ Scouts: Money-map signature. Scouts are uneconomic on
			// Regular maps (275m + 125g + Stargate prerequisite), so a
			// 10-Scout count almost never happens outside Money games.
			// MapKind gate keeps the chip / pill noise-free on standard
			// games even if a player accidentally produces a few Scouts.
			Name:         "10+ Scouts",
			PatternName:  "10+ Scouts",
			FeatureKey:   "ten_plus_scouts",
			Kind:         KindMarker,
			Race:         RaceProtoss,
			MapKind:      []string{"Money"},
			Rule:         ProduceCountAtLeast(subjScout, 10),
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "10+ Scouts", IconKey: "scout", Style: PillStyleStrong, Title: "10+ Scouts"},
			GamesList:     &Pill{Label: "10+ Scouts", IconKey: "scout", Style: PillStyleStrong, Title: "10+ Scouts"},
		},
		{
			Name:             "Never upgraded",
			PatternName:      "Never upgraded",
			FeatureKey:       "never_upgraded",
			Kind:             KindMarker,
			Rule:             Not(UpgradeExists()),
			RuleDeadline:     endOfReplaySentinel,
			MinReplaySeconds: 10 * 60,
			SummaryPlayer: &Pill{
				Label: "🚫 upgrades",
				Style: PillStyleNegative,
				Title: "No Upgrade commands in this replay for this player (10+ minute games).",
			},
		},
		{
			Name:             "Never researched",
			PatternName:      "Never researched",
			FeatureKey:       "never_researched",
			Kind:             KindMarker,
			Rule:             Not(TechExists()),
			RuleDeadline:     endOfReplaySentinel,
			MinReplaySeconds: 10 * 60,
			SummaryPlayer: &Pill{
				Label: "🚫 researches",
				Style: PillStyleNegative,
				Title: "No Tech commands in this replay for this player (10+ minute games).",
			},
		},

		// Custom evaluator markers — worldstate-sourced events + spatial stat.

		{
			Name:         "Made drops",
			PatternName:  "Made drops",
			FeatureKey:   "made_drops",
			Kind:         KindMarker,
			Custom:       func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "drop"} },
			RuleDeadline: endOfReplaySentinel,
			// Suppressed on the summary player row when the backend already emits a
			// drop game_event (the frontend de-dupes via trustGameEventsForDrops);
			// we still expose the pill for the Events-list / raw consumers.
			SummaryPlayer: &Pill{Label: "Made drops at min {minute}"},
		},
		{
			Name:          "Made recalls",
			PatternName:   "Made recalls",
			FeatureKey:    "made_recalls",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Custom:        func() CustomEvaluator { return &firstCastEvaluator{subject: "Recall"} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Recalls at min {minute}", IconKey: "arbiter"},
			GamesList:     &Pill{Label: "Recalls", IconKey: "arbiter"},
		},
		{
			Name:          "Threw Nukes",
			PatternName:   "Threw Nukes",
			FeatureKey:    "threw_nukes",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Custom:        func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "nuke"} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Threw Nukes at {minute} mins"},
			GamesList:     &Pill{Label: "Nukes", IconKey: "ghost"},
		},
		{
			Name:         "Became Terran",
			PatternName:  "Became Terran",
			FeatureKey:   "became_terran",
			Kind:         KindMarker,
			Custom:       func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "became_terran"} },
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{
				Label:   "Terran at {minute} mins",
				IconKey: "darkarchon",
				Style:   PillStyleStrong,
				Title:   "Became Terran",
			},
		},
		{
			Name:         "Became Zerg",
			PatternName:  "Became Zerg",
			FeatureKey:   "became_zerg",
			Kind:         KindMarker,
			Custom:       func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "became_zerg"} },
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{
				Label:   "Zerg at {minute} mins",
				IconKey: "darkarchon",
				Style:   PillStyleStrong,
				Title:   "Became Zerg",
			},
		},
		{
			Name:             "Viewport Multitasking",
			PatternName:      models.PatternNameViewportMultitasking,
			FeatureKey:       "viewport_multitasking",
			Kind:             KindMarker,
			Custom:           newViewportMultitaskingEvaluator,
			RuleDeadline:     endOfReplaySentinel,
			MinReplaySeconds: models.ViewportMultitaskingWindowStartSecond, // 7m
			// Deliberately no pill surfaces: this marker feeds the dedicated
			// viewport-multitasking widget, not the summary pill row.
		},

		// Hotkey markers. Migrated from the imperative detectors; same
		// PatternNames so DB + FE stay compatible.

		{
			Name:             "Never used hotkeys",
			PatternName:      "Never used hotkeys",
			FeatureKey:       "never_used_hotkeys",
			Kind:             KindMarker,
			Rule:             Not(HotkeyExists()),
			RuleDeadline:     endOfReplaySentinel,
			MinReplaySeconds: 7 * 60,
			SummaryPlayer: &Pill{
				Label: "🚫 hotkeys",
				Style: PillStyleNegative,
				Title: "No hotkey-group commands in this replay (same 7+ minute gate as the detector).",
			},
		},
		{
			Name:         "Used Hotkey Groups",
			PatternName:  "Used Hotkey Groups",
			FeatureKey:   "used_hotkey_groups",
			Kind:         KindMarker,
			Custom:       newUsedHotkeyGroupsEvaluator,
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{
				Label:   "Hotkeys {subject}",
				Subject: PayloadFieldSubject("groups"),
			},
		},
	}
}
