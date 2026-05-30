package markers

import (
	"fmt"
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

// zergPoolBO builds one rung of the pool-first ladder. Supply at Pool = 4
// starting Drones + N Drone morphs, so the rule keys off the exact morph count
// (the early-game spam filter makes the surviving Drone stream supply-faithful
// — see the 4/9/12 Pool comments). Each rung uses a distinct exact count, so
// the rungs are mutually exclusive by construction. Hatchery / Evolution
// Chamber must not precede the Pool (else it's a hatch-first opener).
// poolSec is the progamer-ideal Pool placement second for the UI golden
// compare; zerglings pop one Pool build-time later.
func zergPoolBO(supply, poolSec int) Marker {
	drones := supply - 4
	label := fmt.Sprintf("%d Pool", supply)
	return Marker{
		Name:        label,
		PatternName: "Build Order: " + label,
		FeatureKey:  fmt.Sprintf("bo_%d_pool", supply),
		Race:        RaceZerg,
		Kind:        KindInitialBuildOrder,
		Rule: All(
			ProduceCountBeforeBuild(subjDrone, subjSpawningPool, drones),
			Not(BuildBefore(subjHatchery, subjSpawningPool)),
			Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
		),
		RuleDeadline: 180,
		Expert: []ExpertEvent{
			{
				Key:          "Spawning Pool",
				Match:        MatchBuild(subjSpawningPool),
				TargetSecond: poolSec,
				Tolerance:    Sym(5),
			},
			{
				Key:          "First Zerglings",
				Match:        MatchFirstProduce(subjZergling),
				TargetSecond: secAfter(poolSec, models.BuildTimeSpawningPool),
				Tolerance:    Sym(4),
			},
		},
		SummaryPlayer: &Pill{Label: label, IconKey: "spawningpool"},
		GamesList:     &Pill{Label: label, IconKey: "spawningpool"},
	}
}

// zergHatchBO builds one rung of the hatch-first ladder. Supply at the
// expansion Hatchery = 4 starting Drones + N Drone morphs (the first
// Build(Hatchery) is the expansion — the starting Hatchery isn't a command).
// Keyed on the exact morph count, so rungs are mutually exclusive. A Pool /
// Evolution Chamber before the Hatchery makes it a Pool-tech opening, not
// hatch-first. The lower rungs (4–8 Hatch) need no Overlord gate — a Hatchery
// costs 300 minerals, so a Drone morphing or not before it is the whole signal,
// and the build can't be faked (the spam filter keeps the morph stream
// supply-faithful). hatchSec is the progamer-ideal Hatchery second.
func zergHatchBO(supply, hatchSec int) Marker {
	drones := supply - 4
	label := fmt.Sprintf("%d Hatch", supply)
	return Marker{
		Name:        label,
		PatternName: "Build Order: " + label,
		FeatureKey:  fmt.Sprintf("bo_%d_hatch", supply),
		Race:        RaceZerg,
		Kind:        KindInitialBuildOrder,
		Rule: All(
			ProduceCountBeforeBuild(subjDrone, subjHatchery, drones),
			Not(BuildBefore(subjSpawningPool, subjHatchery)),
			Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
		),
		RuleDeadline: 180,
		Expert: []ExpertEvent{
			{
				Key:          "Hatchery",
				Match:        MatchBuild(subjHatchery),
				TargetSecond: hatchSec,
				Tolerance:    Sym(6),
			},
			{
				Key:          "Spawning Pool",
				Match:        MatchBuild(subjSpawningPool),
				TargetSecond: hatchSec + 30,
				Tolerance:    Asym(6, 12),
			},
		},
		SummaryPlayer: &Pill{Label: label, IconKey: "hatchery"},
		GamesList:     &Pill{Label: label, IconKey: "hatchery"},
	}
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
	// -------------------------------------------------------------------
	// Shared opener rules. Each named Protoss / Terran opener's Rule is
	// declared once here and referenced both by its BO entry below and by
	// the per-race residual "… (Other)" catch-all, which is defined as the
	// EXACT complement Not(Any(named…)). Keeping a single source of truth
	// means the residual can never drift out of sync with the named set, and
	// the FuzzInitialBOsMutualExclusion test guarantees the named rules stay
	// pairwise disjoint. Drone-count-keyed Zerg rungs don't need this — their
	// exact counts are disjoint by construction — so their residual uses the
	// ProduceCountAtLeastBeforeBuild primitive directly.
	// -------------------------------------------------------------------

	// Protoss.
	pRule1GateCore := All(
		BuildBefore(subjGateway, subjCyberneticsCore),
		FirstBuildBefore(subjCyberneticsCore, 180),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore})),
		BuildBefore(subjCyberneticsCore, subjNexus),
	)
	pRule2Gate := All(
		NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore, subjNexus, subjForge}),
		CountBuildsBefore(subjGateway, 2, 180),
	)
	pRuleNexusFirst := All(
		BuildBefore(subjNexus, subjGateway),
		BuildBefore(subjNexus, subjForge),
		FirstBuildBefore(subjNexus, 200),
	)
	// Gate Expand: single Gateway then Nexus. The Nexus-before-Cyber guard is
	// what keeps it disjoint from 1 Gate Core once both run in PvT/PvP
	// (1 Gate Core is Cyber-before-Nexus). Nexus window loosened 200→220.
	pRuleGateExpand := All(
		BuildBefore(subjGateway, subjForge),
		BuildBefore(subjGateway, subjNexus),
		FirstBuildBefore(subjNexus, 220),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjNexus})),
		Not(BuildBefore(subjCyberneticsCore, subjNexus)),
	)
	// Forge Expand (FFE): Forge window loosened 100→140 and Nexus 200→260 —
	// the corpus shows progamer FFEs with Forge up to ~140s and Nexus up to
	// ~260s that the legacy bounds missed.
	pRuleForgeExpand := All(
		FirstBuildBefore(subjForge, 140),
		BuildBefore(subjForge, subjGateway),
		BuildBefore(subjForge, subjNexus),
		FirstBuildBefore(subjNexus, 260),
		BuildBefore(subjNexus, subjGateway),
	)
	// 1 Gate (no expa): a single-Gateway opener that doesn't rush Cyber
	// (1 Gate Core), doesn't go 2 Gate, and never expands early — a slow /
	// contain Gateway. "no expa" is baked in (no Nexus by 300s); a Gateway
	// build that DOES expand is Gate Expand instead.
	pRule1GateNoExpa := All(
		FirstBuildExists(subjGateway),
		Not(FirstBuildExists(subjForge)),
		Not(FirstBuildBefore(subjNexus, 300)),
		Not(FirstBuildBefore(subjCyberneticsCore, 180)),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore, subjNexus, subjForge})),
	)
	// Forge Cannon (no expa): a Forge + Photon Cannon defensive opening with no
	// early expansion (and not a Cyber-rush). NOTE: whether the Cannons are
	// defensive (own base) or a proxy Cannon Rush is a spatial distinction the
	// separate cannon_rush marker already makes; this opener keys on the build
	// sequence only.
	pRuleForgeCannonNoExpa := All(
		FirstBuildExists(subjForge),
		FirstBuildExists(subjPhotonCannon),
		Not(FirstBuildBefore(subjNexus, 300)),
		Not(FirstBuildBefore(subjCyberneticsCore, 180)),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore, subjNexus, subjForge})),
	)
	pNamed := Any(pRule1GateCore, pRule2Gate, pRuleNexusFirst, pRuleGateExpand, pRuleForgeExpand, pRule1GateNoExpa, pRuleForgeCannonNoExpa)

	// Terran.
	tRule1Rax1Fac := All(
		BuildBefore(subjRefinery, subjFactory),
		FirstBuildBefore(subjFactory, 240),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjFactory})),
		Not(BuildBefore(subjCommandCenter, subjBarracks)),
		BuildBefore(subjRefinery, subjCommandCenter),
		BuildBefore(subjFactory, subjCommandCenter),
		// A defensive Bunker is fine here — Bunker Rush is now separated by
		// expansion/tech absence (no CC, no Factory), not by Bunker timing.
	)
	tRuleCCFirst := All(
		BuildBefore(subjCommandCenter, subjBarracks),
		FirstBuildBefore(subjCommandCenter, 200),
	)
	// 1 Rax FE: 1 Barracks then CC (CC before Factory, CC<270). Gas-first
	// expands land here, and so do defensive Bunkers — Bunker Rush is now
	// separated by "no expansion" (below), not by Bunker timing. Disjoint
	// from 1 Rax 1 Fac (which is Factory-before-CC).
	tRule1RaxFE := All(
		BuildBefore(subjBarracks, subjCommandCenter),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjCommandCenter})),
		BuildBefore(subjCommandCenter, subjFactory),
		FirstBuildBefore(subjCommandCenter, 270),
	)
	// 2 Rax CC: two Barracks before the expansion CC (a safer, pressure-first
	// FE). Disjoint from 1 Rax FE (one Rax before CC) and from BBS (the Depot
	// precedes the 2nd Rax here, so it isn't the no-Depot BBS topology).
	tRule2RaxCC := All(
		NthBuildBeforeAll(subjBarracks, 2, []string{subjCommandCenter}),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjSupplyDepot})),
		BuildBefore(subjCommandCenter, subjFactory),
		FirstBuildBefore(subjCommandCenter, 300),
	)
	tRuleBBS := All(
		NthBuildBeforeAll(subjBarracks, 2, []string{
			subjSupplyDepot, subjRefinery, subjCommandCenter,
			subjFactory, subjStarport, subjAcademy,
			subjEngineeringBay, subjBunker,
		}),
		CountBuildsBefore(subjBarracks, 2, 120),
		FirstBuildBefore(subjBarracks, 100),
	)
	// Bunker Rush: an all-in — an early Bunker (≤240s) with NO expansion (no
	// CC by 270s) and NO Factory tech (none by 240s). Defined by commitment to
	// one base + the Bunker, not by Bunker-vs-gas timing, so it's disjoint from
	// 1 Rax FE (has a CC) and 1 Rax 1 Fac (has a Factory). NOTE: a true
	// proxy/forward Bunker would also require the Bunker to sit on the enemy's
	// base/natural — that spatial check (reusing the worldstate the cannon_rush
	// marker uses) is a follow-up; this rule keys on the all-in topology only.
	tRuleBunkerRush := All(
		FirstBuildBefore(subjBunker, 240),
		BuildBefore(subjBarracks, subjBunker),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjBunker})),
		// No expansion in the opener window — using 300s (matching 2 Rax CC's
		// CC bound) so a 2-Rax build that expands at 270–300 is 2 Rax CC, not
		// a Bunker Rush.
		Not(FirstBuildBefore(subjCommandCenter, 300)),
		Not(FirstBuildBefore(subjFactory, 240)),
	)
	tNamed := Any(tRule1Rax1Fac, tRuleCCFirst, tRule1RaxFE, tRule2RaxCC, tRuleBBS, tRuleBunkerRush)

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
		// 5–8 Pool: the lower rungs of the ladder. Keyed purely on the exact
		// pre-Pool Drone-morph count (1/2/3/4 → supply 5/6/7/8); no Overlord
		// gate needed (supply <9 needs no Overlord, and the exact Drone count
		// alone keeps each rung disjoint from every other pool BO).
		zergPoolBO(5, 45),
		zergPoolBO(6, 52),
		zergPoolBO(7, 60),
		zergPoolBO(8, 67),
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
		// 10–11 Pool: between 9 and 12. Supply 10/11 forces an Overlord first
		// (cap 9), but the exact Drone-morph count (6/7) already makes these
		// disjoint from every other rung, so no explicit Overlord gate is
		// needed — keeping them parallel to the 5–8 rungs.
		zergPoolBO(10, 92),
		zergPoolBO(11, 98),
		{
			Name:        "9 Pool into Hatchery",
			PatternName: "Build Order: 9 Pool into Hatchery",
			FeatureKey:  "bo_9_pool_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Same supply as 9 Pool (exactly 5 Drone morphs, no Overlord
				// yet). Keyed on the exact count — not a loose "≥1 Drone" — so
				// it stays disjoint from the 5–8/10–11 Pool rungs (only the
				// 5-Drone stream can match here).
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 5),
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
		// 4–8 Hatch: the fast hatch-first ladder below 9 Hatch. A Hatchery
		// costs 300 minerals, so a player placing one at supply 4–8 genuinely
		// waited that long with that few Drones — it's a real (greedy/fast)
		// expansion, not noise. Keyed on exact pre-Hatch Drone count (0–4).
		zergHatchBO(4, 40),
		zergHatchBO(5, 50),
		zergHatchBO(6, 58),
		zergHatchBO(7, 66),
		zergHatchBO(8, 70),
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
			// Extended to PvZ: the Gate→Cyber→Stargate (Corsair / Sair-DT)
			// opener is a standard PvZ build that previously fell through.
			Kind:         KindInitialBuildOrder,
			Rule:         pRule1GateCore,
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
			Name:         "2 Gate",
			PatternName:  "Build Order: 2 Gate",
			FeatureKey:   "bo_2_gate",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRule2Gate,
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
			Name:         "Nexus First",
			PatternName:  "Build Order: Nexus First",
			FeatureKey:   "bo_nexus_first",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleNexusFirst,
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
			// Extended beyond PvZ: the Gate→Nexus expand (e.g. Gate→Nexus→Cyber)
			// is common in PvT/PvP and previously had no opener.
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleGateExpand,
			RuleDeadline: 220,
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
			// Extended beyond PvZ for the rare FFE-style opener in other
			// matchups; the topology (Forge→Nexus→Gate) is matchup-agnostic.
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleForgeExpand,
			RuleDeadline: 260,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(4)},
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 86, Tolerance: Sym(8)},
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 130, Tolerance: Sym(20)},
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 152, Tolerance: Sym(15)},
			},
			SummaryPlayer: &Pill{Label: "FFE", IconKey: "forge"},
			GamesList:     &Pill{Label: "FFE", IconKey: "forge"},
		},
		{
			// Forge Cannon (no expa): defensive Forge + Cannon, no early
			// expansion. Cannon icon. (Proxy vs in-base cannons → cannon_rush
			// marker.)
			Name:         "Forge Cannon (no expa)",
			PatternName:  "Build Order: Forge Cannon (no expa)",
			FeatureKey:   "bo_forge_cannon_no_expa",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleForgeCannonNoExpa,
			RuleDeadline: 320,
			Expert: []ExpertEvent{
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 90, Tolerance: Sym(20)},
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 130, Tolerance: Sym(30)},
			},
			SummaryPlayer: &Pill{Label: "Forge Cannon (no expa)", IconKey: "photoncannon"},
			GamesList:     &Pill{Label: "Forge Cannon (no expa)", IconKey: "photoncannon"},
		},
		{
			// 1 Gate (no expa): slow / contain single Gateway, no fast Cyber,
			// no expansion.
			Name:         "1 Gate (no expa)",
			PatternName:  "Build Order: 1 Gate (no expa)",
			FeatureKey:   "bo_1_gate_no_expa",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRule1GateNoExpa,
			RuleDeadline: 320,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(6)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 88, Tolerance: Sym(15)},
			},
			SummaryPlayer: &Pill{Label: "1 Gate (no expa)", IconKey: "gateway"},
			GamesList:     &Pill{Label: "1 Gate (no expa)", IconKey: "gateway"},
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
			Name:         "1 Rax 1 Fac",
			PatternName:  "Build Order: 1 Rax 1 Fac",
			FeatureKey:   "bo_1_rax_1_fac",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRule1Rax1Fac,
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
			Name:         "CC First",
			PatternName:  "Build Order: CC First",
			FeatureKey:   "bo_cc_first",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRuleCCFirst,
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
			Name:         "1 Rax FE",
			PatternName:  "Build Order: 1 Rax FE",
			FeatureKey:   "bo_rax_cc",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRule1RaxFE,
			RuleDeadline: 270,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 88, Tolerance: Sym(10)},
				{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 180, Tolerance: Sym(18)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 195, Tolerance: Sym(18)},
			},
			SummaryPlayer: &Pill{Label: "1 Rax FE", IconKey: "commandcenter"},
			GamesList:     &Pill{Label: "1 Rax FE", IconKey: "commandcenter"},
		},
		{
			// 2 Rax CC: two Barracks before the expansion CC — a safer,
			// pressure-first FE. Depot precedes the 2nd Rax (not BBS), and
			// 2 Rax precede the CC (not 1 Rax FE).
			Name:         "2 Rax CC",
			PatternName:  "Build Order: 2 Rax CC",
			FeatureKey:   "bo_2_rax_cc",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRule2RaxCC,
			RuleDeadline: 300,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "1st Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 88, Tolerance: Sym(10)},
				{Key: "2nd Barracks", Match: MatchNthBuild(subjBarracks, 2), TargetSecond: 120, Tolerance: Sym(15)},
				{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 200, Tolerance: Sym(25)},
			},
			SummaryPlayer: &Pill{Label: "2 Rax CC", IconKey: "commandcenter"},
			GamesList:     &Pill{Label: "2 Rax CC", IconKey: "commandcenter"},
		},
		{
			// BBS: confirmed in the dataset (e.g. SST_JumJaJungJi opens
			// BBS in many TvZs: Rax @58s, Rax @79s, Depot @100s, Bunker
			// @157s). All-in 2-Rax before any other Terran building. Rare
			// in modern pro play but a recognizable signature.
			Name:         "BBS",
			PatternName:  "Build Order: BBS",
			FeatureKey:   "bo_bbs",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRuleBBS,
			RuleDeadline: 120,
			Expert: []ExpertEvent{
				{Key: "1st Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "2nd Barracks", Match: MatchNthBuild(subjBarracks, 2), TargetSecond: 80, Tolerance: Sym(8)},
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 100, Tolerance: Sym(10)},
			},
			SummaryPlayer: &Pill{Label: "BBS", IconKey: "barracks"},
			GamesList:     &Pill{Label: "BBS", IconKey: "barracks"},
		},
		{
			// Bunker Rush: an all-in — early Bunker (≤240s) with no expansion
			// (no CC by 270s) and no Factory tech (none by 240s). Separated
			// from 1 Rax FE (has a CC) and 1 Rax 1 Fac (has a Factory) by that
			// commitment, so defensive Bunkers in macro builds no longer land
			// here. (Spatial "Bunker on enemy base" is a follow-up refinement.)
			Name:         "Bunker Rush",
			PatternName:  "Build Order: Bunker Rush",
			FeatureKey:   "bo_bunker_rush",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRuleBunkerRush,
			RuleDeadline: 300,
			Expert: []ExpertEvent{
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 60, Tolerance: Sym(10)},
				{Key: "Bunker", Match: MatchBuild(subjBunker), TargetSecond: 130, Tolerance: Sym(20)},
			},
			SummaryPlayer: &Pill{Label: "Bunker Rush", IconKey: "bunker"},
			GamesList:     &Pill{Label: "Bunker Rush", IconKey: "bunker"},
		},

		// -------------------------------------------------------------------
		// Residual "… (Other)" catch-alls (one per race). Each is the EXACT
		// complement of its race's named openers, gated on the player having
		// actually placed a defining opener building — so every classifiable
		// player-replay lands on exactly one initial BO (a named one or its
		// race's residual). Players who never place a defining building are
		// left to the "Opener unresolved" marker below. Mutual exclusion with
		// the named openers is guaranteed by construction (Not(Any(named))).
		// -------------------------------------------------------------------
		{
			// Zerg residual: the greedy tail of the Drone ladder — a Pool or
			// expansion Hatchery placed at supply ≥13 (≥9 Drone morphs), which
			// no exact rung claims. Pool-first vs hatch-first guards keep the
			// two arms disjoint from each other and from the named rungs.
			Name:        "Pool/Hatch (Other)",
			PatternName: "Build Order: Pool/Hatch (Other)",
			FeatureKey:  "bo_zerg_other",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: Any(
				// Pool-first greedy tail: supply ≥13 (≥9 Drone morphs).
				All(
					FirstBuildExists(subjSpawningPool),
					ProduceCountAtLeastBeforeBuild(subjDrone, subjSpawningPool, 9),
					Not(BuildBefore(subjHatchery, subjSpawningPool)),
					Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
				),
				// Hatch-first greedy tail: supply ≥13 (≥9 Drone morphs). The
				// low tail (supply 4–8) is now covered by the named 4–8 Hatch
				// rungs.
				All(
					FirstBuildExists(subjHatchery),
					ProduceCountAtLeastBeforeBuild(subjDrone, subjHatchery, 9),
					Not(BuildBefore(subjSpawningPool, subjHatchery)),
					Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
				),
			),
			RuleDeadline:  240,
			SummaryPlayer: &Pill{Label: "Other", IconKey: "spawningpool"},
			GamesList:     &Pill{Label: "Other", IconKey: "spawningpool"},
		},
		{
			// Protoss residual: a Gateway / Nexus / Forge opener that matches
			// none of the five named Protoss builds.
			Name:        "Gateway (Other)",
			PatternName: "Build Order: Gateway (Other)",
			FeatureKey:  "bo_protoss_other",
			Race:        RaceProtoss,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				Any(
					FirstBuildExists(subjGateway),
					FirstBuildExists(subjNexus),
					FirstBuildExists(subjForge),
				),
				Not(pNamed),
			),
			RuleDeadline:  320,
			SummaryPlayer: &Pill{Label: "Other", IconKey: "gateway"},
			GamesList:     &Pill{Label: "Other", IconKey: "gateway"},
		},
		{
			// 1 Rax Bio: a Barracks opener that matches none of the named
			// Terran builds — the Dep-Rax-Gas bio path (Academy/Marine-Medic,
			// no early expansion or Factory), plus short games that only got a
			// Barracks down. Defined as the complement of the named openers,
			// so it stays mutually exclusive with them.
			Name:        "1 Rax Bio",
			PatternName: "Build Order: 1 Rax Bio",
			FeatureKey:  "bo_1_rax_bio",
			Race:        RaceTerran,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				Any(
					FirstBuildExists(subjBarracks),
					FirstBuildExists(subjCommandCenter),
					FirstBuildExists(subjFactory),
				),
				Not(tNamed),
			),
			RuleDeadline:  300,
			SummaryPlayer: &Pill{Label: "1 Rax Bio", IconKey: "marine"},
			GamesList:     &Pill{Label: "1 Rax Bio", IconKey: "marine"},
		},
		{
			// Opener unresolved (N/A): the player never placed a defining
			// opener building — Pool/Hatchery (Z), Gateway/Forge/Nexus (P), or
			// Barracks/CC/Factory (T). Almost always a <2-minute abort / instant
			// leave / dodge where no build order ever happened. Race-agnostic
			// (no Race gate) so one entry covers all three; disjoint from every
			// initial BO, which all require a defining building. Stored so the
			// dashboard can render "—" and coverage can be reported over
			// classifiable players only, instead of counting these as misses.
			Name:        "Opener unresolved",
			PatternName: "Opener unresolved",
			FeatureKey:  "opener_unresolved",
			Kind:        KindMarker,
			Rule: Not(Any(
				FirstBuildExists(subjSpawningPool),
				FirstBuildExists(subjHatchery),
				FirstBuildExists(subjGateway),
				FirstBuildExists(subjForge),
				FirstBuildExists(subjNexus),
				FirstBuildExists(subjBarracks),
				FirstBuildExists(subjCommandCenter),
				FirstBuildExists(subjFactory),
			)),
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{
				Label: "🚫 Opener unresolved",
				Style: PillStyleNegative,
				Title: "No opening build order resolved (game too short / left early).",
			},
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
			Name:        "Mech",
			PatternName: "Mech",
			FeatureKey:  "mech",
			Kind:        KindMarker,
			Race:        RaceTerran,
			Rule: All(
				CountBuildsBefore(subjFactory, 2, 390),
				Not(FirstBuildExists(subjAcademy)),
			),
			RuleDeadline:  390,
			SummaryPlayer: &Pill{Label: "Mech", IconKey: "siegetank", Style: PillStyleStrong, Title: "Mech"},
			GamesList:     &Pill{Label: "Mech", IconKey: "siegetank", Style: PillStyleStrong, Title: "Mech"},
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
			SummaryPlayer: &Pill{Label: "SK Terran", IconKey: "marine", Style: PillStyleStrong, Title: "SK Terran"},
			GamesList:     &Pill{Label: "SK Terran", IconKey: "marine", Style: PillStyleStrong, Title: "SK Terran"},
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
			SummaryPlayer: &Pill{Label: "Mech Transition", IconKey: "siegetank"},
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
			Name:          "Mutalisk timing",
			PatternName:   "Mutalisk timing",
			FeatureKey:    "mutalisk_timing",
			Kind:          KindMarker,
			Race:          RaceZerg,
			Matchup:       []string{"TvZ"},
			Custom:        func() CustomEvaluator { return &mutaTimingEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Mutalisk timing {timestamp}", IconKey: "mutalisk"},
			SummaryReplay: &Pill{Label: "Mutalisk timing {timestamp}", IconKey: "mutalisk"},
			GamesList:     &Pill{Label: "Mutalisk timing {timestamp}", IconKey: "mutalisk"},
		},
		{
			Name:          "Turret timing",
			PatternName:   "Turret timing",
			FeatureKey:    "turret_timing",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Matchup:       []string{"TvZ"},
			Custom:        func() CustomEvaluator { return &turretTimingEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Turret timing {timestamp}", IconKey: "missileturret"},
			SummaryReplay: &Pill{Label: "Turret timing {timestamp}", IconKey: "missileturret"},
			GamesList:     &Pill{Label: "Turret timing {timestamp}", IconKey: "missileturret"},
		},
		{
			// Cliff drop (Big Game Hunters only): Terran player produces a
			// Siege Tank, then UnloadAll fires within the 256×128px corner
			// box at top-left or bottom-right. Map gating happens inside
			// the evaluator's Finalize via IsBigGameHuntersMap.
			Name:          "Cliff drop",
			PatternName:   "Cliff drop",
			FeatureKey:    "cliff_drop",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Custom:        func() CustomEvaluator { return &cliffDropEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Cliff drop at min {minute}", IconKey: "dropship"},
			SummaryReplay: &Pill{Label: "Cliff drop at min {minute}", IconKey: "dropship"},
			GamesList:     &Pill{Label: "Cliff drop", IconKey: "dropship"},
		},
		{
			Name:          "Carriers",
			PatternName:   "Carriers",
			FeatureKey:    "carriers",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Rule:          FirstProduceExists(subjCarrier),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{IconKey: "carrier", Style: PillStyleStrong, Title: "Carriers"},
			GamesList:     &Pill{IconKey: "carrier", Style: PillStyleStrong, Title: "Carriers"},
		},
		{
			Name:          "Battlecruisers",
			PatternName:   "Battlecruisers",
			FeatureKey:    "battlecruisers",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Rule:          FirstProduceExists(subjBattlecruiser),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{IconKey: "battlecruiser", Style: PillStyleStrong, Title: "Battlecruisers"},
			GamesList:     &Pill{IconKey: "battlecruiser", Style: PillStyleStrong, Title: "Battlecruisers"},
		},
		{
			// 10+ Scouts: Money-map signature. Scouts are uneconomic on
			// Regular maps (275m + 125g + Stargate prerequisite), so a
			// 10-Scout count almost never happens outside Money games.
			// MapKind gate keeps the chip / pill noise-free on standard
			// games even if a player accidentally produces a few Scouts.
			Name:          "10+ Scouts",
			PatternName:   "10+ Scouts",
			FeatureKey:    "ten_plus_scouts",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			MapKind:       []string{"Money"},
			Rule:          ProduceCountAtLeast(subjScout, 10),
			RuleDeadline:  endOfReplaySentinel,
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
			MinReplaySeconds: 10 * 60, // fallback for non-1v1
			// 1v1 floor per (own_race, opp_race): p5 of first-Upgrade time
			// across 5708 progamer 1v1 player-games. Buckets with <20 samples
			// are omitted and fall through to MinReplaySeconds.
			MinReplaySecondsByMatchup: map[Race]map[Race]int{
				RaceTerran: {
					RaceTerran:  264, // n=118, p5=4:24
					RaceProtoss: 293, // n=315, p5=4:53
					RaceZerg:    233, // n=358, p5=3:53
				},
				RaceProtoss: {
					RaceTerran:  163, // n=366, p5=2:43
					RaceProtoss: 169, // n=235, p5=2:49
					RaceZerg:    237, // n=536, p5=3:57
				},
				RaceZerg: {
					RaceTerran:  175, // n=407, p5=2:55
					RaceProtoss: 128, // n=592, p5=2:08
					RaceZerg:    125, // n=444, p5=2:05
				},
			},
			SummaryPlayer: &Pill{
				Label: "🚫 upgrades",
				Style: PillStyleNegative,
				Title: "No Upgrade commands in this replay for this player (suppressed on games shorter than matchup-typical first upgrade).",
			},
		},
		{
			Name:             "Never researched",
			PatternName:      "Never researched",
			FeatureKey:       "never_researched",
			Kind:             KindMarker,
			Rule:             Not(TechExists()),
			RuleDeadline:     endOfReplaySentinel,
			MinReplaySeconds: 10 * 60, // fallback for non-1v1 (and ZvZ in 1v1, omitted below for n<20)
			// 1v1 floor per (own_race, opp_race): p5 of first-Tech time
			// across 5708 progamer 1v1 player-games. Buckets with <20 samples
			// are omitted (ZvZ has n=17 → falls through to MinReplaySeconds).
			MinReplaySecondsByMatchup: map[Race]map[Race]int{
				RaceTerran: {
					RaceTerran:  245, // n=141, p5=4:05
					RaceProtoss: 238, // n=341, p5=3:58
					RaceZerg:    252, // n=368, p5=4:12
				},
				RaceProtoss: {
					RaceTerran:  496, // n=176, p5=8:16
					RaceProtoss: 332, // n=74,  p5=5:32
					RaceZerg:    396, // n=407, p5=6:36
				},
				RaceZerg: {
					RaceTerran:  243, // n=264, p5=4:03
					RaceProtoss: 339, // n=372, p5=5:39
				},
			},
			SummaryPlayer: &Pill{
				Label: "🚫 researches",
				Style: PillStyleNegative,
				Title: "No Tech commands in this replay for this player (suppressed on games shorter than matchup-typical first research).",
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
				// The keyboard emoji (enlarged) and a "HOTKEYS" top-border
				// legend are added by the frontend; the stored label is just
				// the group list.
				Label:   "{subject}",
				Subject: PayloadFieldSubject("groups"),
			},
		},
		// Phase-boundary markers: registry-only stubs so the storage layer's
		// markers.ByPatternName() lookup resolves their FeatureKey on insert.
		// They have NO Rule and NO Custom — the orchestrator's auto-registered
		// MarkerPlayerDetector becomes a no-op for them. The actual data is
		// produced by the replay-level detectors in
		// internal/patterns/detectors/phase_boundary_detector.go which emit
		// PatternResults that share these PatternNames. Pills are
		// intentionally absent on every surface: these markers exist only
		// to be queried server-side by feature code that needs the
		// early/mid/late split, never rendered as a chip.
		{
			Name:         "Mid game starts",
			PatternName:  "mid_game_starts",
			FeatureKey:   "mid_game_starts",
			Kind:         KindMarker,
			RuleDeadline: endOfReplaySentinel,
		},
		{
			Name:         "Late game starts",
			PatternName:  "late_game_starts",
			FeatureKey:   "late_game_starts",
			Kind:         KindMarker,
			RuleDeadline: endOfReplaySentinel,
		},
	}
}
