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
	subjHydraliskDen     = models.GeneralUnitHydraliskDen
	subjHydralisk        = models.GeneralUnitHydralisk
	subjLurker           = models.GeneralUnitLurker

	// Protoss
	subjNexus            = models.GeneralUnitNexus
	subjPylon            = models.GeneralUnitPylon
	subjGateway          = models.GeneralUnitGateway
	subjAssimilator      = models.GeneralUnitAssimilator
	subjCyberneticsCore  = models.GeneralUnitCyberneticsCore
	subjForge            = models.GeneralUnitForge
	subjPhotonCannon     = models.GeneralUnitPhotonCannon
	subjZealot           = models.GeneralUnitZealot
	subjScout            = models.GeneralUnitScout
	subjCarrier          = models.GeneralUnitCarrier
	subjStargate         = models.GeneralUnitStargate
	subjCorsair          = models.GeneralUnitCorsair
	subjRoboticsFacility = models.GeneralUnitRoboticsFacility
	subjReaver           = models.GeneralUnitReaver
	subjCitadelOfAdun    = models.GeneralUnitCitadelOfAdun
	subjTemplarArchives  = models.GeneralUnitTemplarArchives
	subjDarkTemplar      = models.GeneralUnitDarkTemplar

	// Terran
	subjCommandCenter  = models.GeneralUnitCommandCenter
	subjSupplyDepot    = models.GeneralUnitSupplyDepot
	subjBarracks       = models.GeneralUnitBarracks
	subjRefinery       = models.GeneralUnitRefinery
	subjAcademy        = models.GeneralUnitAcademy
	subjFactory        = models.GeneralUnitFactory
	subjMachineShop    = models.GeneralUnitMachineShop
	subjArmory         = models.GeneralUnitArmory
	subjStarport       = models.GeneralUnitStarport
	subjEngineeringBay = models.GeneralUnitEngineeringBay
	subjMissileTurret  = models.GeneralUnitMissileTurret
	subjBunker         = models.GeneralUnitBunker
	subjMarine         = models.GeneralUnitMarine
	subjFirebat        = models.GeneralUnitFirebat
	subjMedic          = models.GeneralUnitMedic
	subjVulture        = models.GeneralUnitVulture
	subjGoliath        = models.GeneralUnitGoliath
	subjSiegeTank      = models.GeneralUnitSiegeTankTankMode
	subjWraith         = models.GeneralUnitWraith
	subjScienceVessel  = models.GeneralUnitScienceVessel
	subjBattlecruiser  = models.GeneralUnitBattlecruiser
)

// endOfReplaySentinel is a RuleDeadline for markers whose answer can only
// be resolved at end-of-replay (e.g. "never upgraded", "Carriers produced
// at any point"). Well past any realistic SC:BW replay length; the detector
// will still Finalize when the replay actually ends.
const endOfReplaySentinel = 10 * 60 * 60 // 10 hours

// zergOpeningHatchDeadline is the second by which the tier-1 Zerg tech-pathway
// openers count opening Hatcheries to tell "2 base" from "3 base". Counting
// relative to the tech building (Spire/Den) under-counts: the 3rd base usually
// lands AT or just after the Spire (~4:50), so before-Spire is 1 hatch for both
// 2- and 3-hatch muta. By 6:00 the 3-base opening has its 2 expansion Hatcheries
// down while the 2-base one still has 1 — a clean split in the cwal-dl corpus
// (1 hatch: 80 players, 2 hatch: 225). One Build(Hatchery) = 2 bases (the
// starting Hatchery is not a command); two = 3 bases.
const zergOpeningHatchDeadline = 360

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

	// Terran. CC First and BBS stay keyed on build-order topology; Bunker Rush
	// adds a spatial gate on top of topology (see its definition below).
	// Everything else — what used to be 1 Rax 1 Fac, 1 Rax FE, 2 Rax CC and the
	// 1 Rax Bio residual — is reclassified by army composition at 10:00 (issue
	// #155): Bio / Mech / Wraith / Goliath / 1-1-1, split by Barracks or Factory
	// count. tCohort excludes only CC First and BBS so the composition BOs share
	// their (complementary) space.
	tRuleCCFirst := All(
		BuildBefore(subjCommandCenter, subjBarracks),
		FirstBuildBefore(subjCommandCenter, 200),
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
	// CC by 300s) and NO Factory tech (none by 240s). This topology alone can't
	// tell an offensive bunker rush from a defensive sim-city bunker: on Money
	// maps (BGH) nobody takes a second CC, so a player who walls their own base
	// with an early Bunker matches every guard here (issue #164). The Bunker
	// Rush marker pairs this topology with a spatial gate (RequireWorldstateEvent
	// "bunker_rush") that only fires for a Bunker placed at the enemy's base, so
	// the topology stays the all-in shape while the location decides the verdict.
	tRuleBunkerRush := All(
		FirstBuildBefore(subjBunker, 240),
		BuildBefore(subjBarracks, subjBunker),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjBunker})),
		Not(FirstBuildBefore(subjCommandCenter, 300)),
		Not(FirstBuildBefore(subjFactory, 240)),
	)

	// Composition cohort: any Terran opener that is NOT a topology opener. Only
	// CC First and BBS are excluded — Bunker Rush is no longer a pure-topology
	// opener (it needs the spatial gate), so its bunker-topology players must
	// stay eligible for the composition / residual buckets. A genuine offensive
	// bunker rush still matches a composition BO too, but Bunker Rush is
	// TierPreferred and wins by precedence (selectBestTierOpeners).
	tCohort := All(Not(tRuleCCFirst), Not(tRuleBBS))

	// Composition signals, all measured over the opening window (10:00 = 600s;
	// early caps at 7:00 = 420s). bio = Marine/Medic/Firebat, mech = Vulture/
	// Goliath/Siege Tank. "Predominant" = strict majority of produced army.
	bioUnits := []string{subjMarine, subjMedic, subjFirebat}
	mechUnits := []string{subjVulture, subjGoliath, subjSiegeTank}
	tcBioPred := Predominant(bioUnits, mechUnits, 600)
	tcMechPred := Predominant(mechUnits, bioUnits, 600)
	// Bio is Marine-dominant and either committed (8+ Marines by 10:00) or a
	// pure-Barracks opening with no Factory/Starport transition by 10:00. The
	// 8-Marine floor exists to screen out players who make a few Marines on the
	// way into mech/air — those players always have a Factory or Starport, so a
	// no-transition opening that fell short of the floor (e.g. died / left early
	// under attack) is still a Bio opening, not a residual.
	tcBioNoTransition := All(
		ProduceCountAtLeastBefore(subjMarine, 1, 600),
		Not(FirstBuildBefore(subjFactory, 600)),
		Not(FirstBuildBefore(subjStarport, 600)),
	)
	tcBio := All(tcBioPred, Any(ProduceCountAtLeastBefore(subjMarine, 8, 600), tcBioNoTransition))
	tcWraith := All(CountBuildsBefore(subjStarport, 2, 600), ProduceCountAtLeastBefore(subjWraith, 5, 600))
	tcTank1 := ProduceCountAtLeastBefore(subjSiegeTank, 1, 600)
	tcTank0 := ProduceCountAtMostBefore(subjSiegeTank, 0, 600)
	// Goliath opener is Goliath-dominant with no tank tech: ≤2 Vultures and
	// ≤4 Marines by 7:00, 4+ Goliaths by 10:00. Tanks make it Mech instead, so
	// tcTank0 guards against tank-heavy mech being misread as a Goliath build.
	tcGoliath := All(
		ProduceCountAtMostBefore(subjVulture, 2, 420),
		ProduceCountAtMostBefore(subjMarine, 4, 420),
		ProduceCountAtLeastBefore(subjGoliath, 4, 600),
		tcTank0,
	)
	tcOneOneOne := All(FirstBuildBefore(subjStarport, 420), ProduceCountAtLeastBefore(subjWraith, 1, 600))
	tcFac2 := CountBuildsBefore(subjFactory, 2, 600)

	// tNamed: the matchup-free union of everything any Terran BO can match, used
	// to define the residual as its exact complement (mutual-exclusion is then
	// guaranteed by construction). NOTE: the composition BOs are matchup-gated
	// (Wraith/Goliath TvZ, Bio TvZ-or-non-1v1) but tNamed is matchup-free, so a
	// game whose composition matches a gated BO in the "wrong" matchup is
	// subtracted from the residual without any BO firing — a deliberate, rare
	// coverage gap for off-matchup compositions (e.g. a TvP mass-bio game).
	tNamed := Any(
		tRuleCCFirst, tRuleBBS,
		All(tCohort, tcWraith),
		All(tCohort, tcGoliath),
		All(tCohort, tcBio),
		All(tCohort, tcFac2, tcMechPred), // mech + tankless mech + 1-1-1-into-mech (all need ≥2 Factories)
		All(tCohort, tcOneOneOne),        // plain 1-1-1
	)

	// Expert milestone timings for the composition-based Terran BOs (issue #158),
	// mined from a corpus of ~2,860 expert/progamer replays. Each target is the
	// median actual second across that bucket's detected games; tolerances span
	// roughly the p10–p90 expert range (wider, and skewed late, for the macro
	// buildings that arrive on a sliding economy timing). The opening backbone
	// (Depot / 1st Barracks) is shared across the Terran macro openers, but gas
	// timing splits the families — bio delays its Refinery (~185s) while mech
	// takes early gas (~100s) — so each family carries its own opening table.
	// Family tech (Academy for bio, 1st Factory for mech) and the bucket-defining
	// Nth building carry the per-build signal. Nth-building tables are indexed by
	// count (2..6); indices 0,1 are unused.
	tBioOpening := []ExpertEvent{
		{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 56, Tolerance: Asym(10, 24)},
		{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 84, Tolerance: Asym(28, 20)},
		{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 185, Tolerance: Asym(80, 50)},
		{Key: "Academy", Match: MatchBuild(subjAcademy), TargetSecond: 230, Tolerance: Asym(45, 90)},
	}
	tMechOpening := []ExpertEvent{
		{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 55, Tolerance: Asym(10, 24)},
		{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 85, Tolerance: Asym(26, 18)},
		{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 100, Tolerance: Asym(12, 70)},
		{Key: "1st Factory", Match: MatchBuild(subjFactory), TargetSecond: 152, Tolerance: Asym(12, 80)},
	}
	tBioNthRax := [7]int{0, 0, 220, 342, 355, 476, 512}
	tMechNthFac := [7]int{0, 0, 275, 458, 480, 501, 528}
	tMechTank := 290
	tTanklessNthFac := [7]int{0, 0, 243, 349, 422, 471, 560}
	tTanklessVulture := 205

	// Compact builders for the per-count composition buckets (issue #155). Each
	// is an initial BO inside tCohort, made pairwise-disjoint by the predominance
	// split (bio vs mech), the Tank present/absent split, the exact count, and
	// Not(...) guards against the higher-signal Wraith/Goliath/1-1-1 rules.
	mkPill := func(label, icon string) *Pill { return &Pill{Label: label, IconKey: icon} }
	// countPred builds the bucket's count predicate: an exact count for the
	// 1..5 rungs, "at least" for the 6+ top rung.
	countPred := func(subj string, n int, exact bool) Predicate {
		if exact {
			return BuildCountEqualsBefore(subj, n, 600)
		}
		return CountBuildsBefore(subj, n, 600)
	}
	// nthLabel renders the bucket-defining building milestone label, e.g.
	// "2nd Barracks", "6th Factory".
	nthLabel := func(n int, building string) string {
		suffix := "th"
		switch n {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
		return fmt.Sprintf("%d%s %s", n, suffix, building)
	}
	bioBucket := func(name, fkey string, n int, exact bool) Marker {
		ev := append([]ExpertEvent{}, tBioOpening...)
		if n >= 2 {
			ev = append(ev, ExpertEvent{Key: nthLabel(n, "Barracks"), Match: MatchNthBuild(subjBarracks, n), TargetSecond: tBioNthRax[n], Tolerance: Asym(90, 130)})
		}
		return Marker{
			Name: name, PatternName: InitialBuildOrderPatternNamePrefix + name, FeatureKey: fkey,
			Race: RaceTerran, Kind: KindInitialBuildOrder, Matchup: []string{"TvZ", MatchupNon1v1},
			Rule:          All(tCohort, tcBio, Not(tcWraith), Not(tcGoliath), countPred(subjBarracks, n, exact)),
			RuleDeadline:  600,
			Expert:        ev,
			SummaryPlayer: mkPill(name, "marine"), GamesList: mkPill(name, "marine"),
		}
	}
	mechBucket := func(name, fkey string, n int, exact bool) Marker {
		ev := append([]ExpertEvent{}, tMechOpening...)
		ev = append(ev,
			ExpertEvent{Key: nthLabel(n, "Factory"), Match: MatchNthBuild(subjFactory, n), TargetSecond: tMechNthFac[n], Tolerance: Asym(90, 130)},
			ExpertEvent{Key: "First Siege Tank", Match: MatchFirstProduce(subjSiegeTank), TargetSecond: tMechTank, Tolerance: Asym(60, 120)},
		)
		return Marker{
			Name: name, PatternName: InitialBuildOrderPatternNamePrefix + name, FeatureKey: fkey,
			Race: RaceTerran, Kind: KindInitialBuildOrder,
			Rule:          All(tCohort, countPred(subjFactory, n, exact), tcMechPred, tcTank1, Not(tcOneOneOne), Not(tcWraith), Not(tcGoliath)),
			RuleDeadline:  600,
			Expert:        ev,
			SummaryPlayer: mkPill(name, "siegetank"), GamesList: mkPill(name, "siegetank"),
		}
	}
	tanklessBucket := func(name, fkey string, n int, exact bool) Marker {
		ev := append([]ExpertEvent{}, tMechOpening...)
		ev = append(ev,
			ExpertEvent{Key: nthLabel(n, "Factory"), Match: MatchNthBuild(subjFactory, n), TargetSecond: tTanklessNthFac[n], Tolerance: Asym(90, 130)},
			ExpertEvent{Key: "First Vulture", Match: MatchFirstProduce(subjVulture), TargetSecond: tTanklessVulture, Tolerance: Asym(20, 50)},
		)
		return Marker{
			Name: name, PatternName: InitialBuildOrderPatternNamePrefix + name, FeatureKey: fkey,
			Race: RaceTerran, Kind: KindInitialBuildOrder,
			Rule:          All(tCohort, countPred(subjFactory, n, exact), tcMechPred, tcTank0, Not(tcWraith), Not(tcGoliath)),
			RuleDeadline:  600,
			Expert:        ev,
			SummaryPlayer: mkPill(name, "vulture"), GamesList: mkPill(name, "vulture"),
		}
	}

	return []Marker{
		// -------------------------------------------------------------------
		// TIER-1 PREFERRED OPENERS (issue #182). Specific, scene-named openings
		// sourced from current BW pro / Liquipedia knowledge (docs/build-orders-
		// research/). Each is keyed on the building SEQUENCE + defining tech
		// unit the parser detects reliably (supply is not simulated for T/P),
		// matchup-gated, and made pairwise-disjoint WITHIN tier 1 per (race,
		// matchup) — across tiers, overlap is expected and resolved by tier
		// precedence (a matched preferred opener suppresses the broad tier-2
		// bucket it overlaps). Expert milestone timings are intentionally left
		// empty pending corpus-derived medians (issue #182, same method as #158);
		// the UI shows the opener label without an actual-vs-ideal compare until
		// then.
		//
		// Tier-1 deliberately stays MORE SPECIFIC than tier 2: where the broad
		// bucket is already the better classification (notably TvZ army
		// composition, #155), no blanket tier-1 opener is added, so those games
		// keep their composition label.
		// -------------------------------------------------------------------

		// --- Zerg tech-pathway openers: hatchery count + first tech building. ---
		{
			// 3 Hatch Muta (ZvT): muta-first (Spire before any Hydralisk Den)
			// off a 3-base opening — 2 expansion Hatcheries placed before the
			// Spire. The defining modern ZvT macro-muta build.
			Name: "3 Hatch Muta", PatternName: InitialBuildOrderPatternNamePrefix + "3 Hatch Muta", FeatureKey: "bo_z_3hatch_muta",
			Race: RaceZerg, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvZ"},
			Rule: All(
				BuildBefore(subjSpire, subjHydraliskDen),                     // muta-first: Spire before any Den
				ProduceCountAtLeast(subjMutalisk, 4),                         // it's a muta build, not just a Spire
				CountBuildsBefore(subjHatchery, 2, zergOpeningHatchDeadline), // 3 bases (2 expansions)
			),
			RuleDeadline: 600,
			// Expert targets: medians across the cwal-dl corpus's detected games
			// (issue #182), tolerances ~ the observed spread.
			Expert: []ExpertEvent{
				{Key: "Spire", Match: MatchBuild(subjSpire), TargetSecond: 240, Tolerance: Asym(30, 80)},
				{Key: "First Mutalisks", Match: MatchFirstProduce(subjMutalisk), TargetSecond: 320, Tolerance: Asym(40, 90)},
			},
			SummaryPlayer: mkPill("3 Hatch Muta", "mutalisk"), GamesList: mkPill("3 Hatch Muta", "mutalisk"),
		},
		{
			// 2 Hatch Muta (ZvT): muta-first off a 2-base opening — fewer than 2
			// expansion Hatcheries before the Spire. Faster muta when a 3rd is
			// hard to hold.
			Name: "2 Hatch Muta", PatternName: InitialBuildOrderPatternNamePrefix + "2 Hatch Muta", FeatureKey: "bo_z_2hatch_muta",
			Race: RaceZerg, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvZ"},
			Rule: All(
				BuildBefore(subjSpire, subjHydraliskDen),                          // muta-first: Spire before any Den
				ProduceCountAtLeast(subjMutalisk, 4),                              // it's a muta build, not just a Spire
				Not(CountBuildsBefore(subjHatchery, 2, zergOpeningHatchDeadline)), // 2 bases only
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Spire", Match: MatchBuild(subjSpire), TargetSecond: 249, Tolerance: Asym(35, 70)},
				{Key: "First Mutalisks", Match: MatchFirstProduce(subjMutalisk), TargetSecond: 327, Tolerance: Asym(40, 90)},
			},
			SummaryPlayer: mkPill("2 Hatch Muta", "mutalisk"), GamesList: mkPill("2 Hatch Muta", "mutalisk"),
		},
		{
			// 3 Hatch Lurker (ZvT): lurker-first (Hydralisk Den before Spire)
			// off a 3-base opening, with Lurkers actually morphed. Defensive
			// fast-tech vs +1 5-Rax.
			Name: "3 Hatch Lurker", PatternName: InitialBuildOrderPatternNamePrefix + "3 Hatch Lurker", FeatureKey: "bo_z_3hatch_lurker",
			Race: RaceZerg, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvZ"},
			Rule: All(
				BuildBefore(subjHydraliskDen, subjSpire),                     // lurker-first: Den before any Spire
				ProduceCountAtLeast(subjLurker, 2),                           // lurkers are the point of the build
				CountBuildsBefore(subjHatchery, 2, zergOpeningHatchDeadline), // 3 bases
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Hydralisk Den", Match: MatchBuild(subjHydraliskDen), TargetSecond: 270, Tolerance: Asym(50, 60)},
				{Key: "First Lurkers", Match: MatchFirstProduce(subjLurker), TargetSecond: 417, Tolerance: Asym(80, 120)},
			},
			SummaryPlayer: mkPill("3 Hatch Lurker", "lurker"), GamesList: mkPill("3 Hatch Lurker", "lurker"),
		},
		{
			// 2 Hatch Hydra (ZvP): a 2-base Hydralisk Den opening (Den before any
			// Spire) — hydra pressure / bust vs cannon-light FFE.
			Name: "2 Hatch Hydra", PatternName: InitialBuildOrderPatternNamePrefix + "2 Hatch Hydra", FeatureKey: "bo_z_2hatch_hydra",
			Race: RaceZerg, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvZ"},
			Rule: All(
				BuildBefore(subjHydraliskDen, subjSpire),                          // hydra-tech, not muta
				ProduceCountAtLeastBefore(subjHydralisk, 6, 420),                  // a hydra mass / bust
				Not(CountBuildsBefore(subjHatchery, 2, zergOpeningHatchDeadline)), // 2 bases only
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Hydralisk Den", Match: MatchBuild(subjHydraliskDen), TargetSecond: 214, Tolerance: Asym(25, 90)},
				{Key: "First Hydralisks", Match: MatchFirstProduce(subjHydralisk), TargetSecond: 250, Tolerance: Asym(40, 120)},
			},
			SummaryPlayer: mkPill("2 Hatch Hydra", "hydralisk"), GamesList: mkPill("2 Hatch Hydra", "hydralisk"),
		},

		// --- Protoss tech-pathway openers: opening topology + first tech unit. ---
		{
			// 1 Gate Reaver (PvT): single Gateway into Robotics + Reaver harass
			// (no Dark Templar). Reaver-shuttle harass into expand.
			Name: "1 Gate Reaver", PatternName: InitialBuildOrderPatternNamePrefix + "1 Gate Reaver", FeatureKey: "bo_p_1gate_reaver",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvT"},
			Rule: All(
				FirstBuildExists(subjRoboticsFacility),
				ProduceCountAtLeast(subjReaver, 1),
				Not(NthBuildBeforeAll(subjGateway, 2, []string{subjRoboticsFacility})),
				Not(ProduceCountAtLeast(subjDarkTemplar, 1)),
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Robotics Facility", Match: MatchBuild(subjRoboticsFacility), TargetSecond: 252, Tolerance: Asym(60, 70)},
				{Key: "First Reaver", Match: MatchFirstProduce(subjReaver), TargetSecond: 408, Tolerance: Asym(90, 120)},
			},
			SummaryPlayer: mkPill("1 Gate Reaver", "reaver"), GamesList: mkPill("1 Gate Reaver", "reaver"),
		},
		{
			// 2 Gate DT (PvT): Citadel + Templar Archives into Dark Templar
			// (no Reaver). Cloaked harass vs Siege-FE Terran lacking detection.
			Name: "2 Gate DT", PatternName: InitialBuildOrderPatternNamePrefix + "2 Gate DT", FeatureKey: "bo_p_2gate_dt",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvT"},
			Rule: All(
				CountBuildsBefore(subjGateway, 2, 360), // it's a 2-Gate build
				FirstBuildExists(subjCitadelOfAdun),    // Citadel → Templar Archives path
				FirstBuildExists(subjTemplarArchives),
				ProduceCountAtLeast(subjDarkTemplar, 2), // DTs are the point (harass pair)
				Not(ProduceCountAtLeast(subjReaver, 1)), // disjoint from 1 Gate Reaver
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Templar Archives", Match: MatchBuild(subjTemplarArchives), TargetSecond: 327, Tolerance: Asym(80, 100)},
				{Key: "First Dark Templar", Match: MatchFirstProduce(subjDarkTemplar), TargetSecond: 379, Tolerance: Asym(90, 120)},
			},
			SummaryPlayer: mkPill("2 Gate DT", "darktemplar"), GamesList: mkPill("2 Gate DT", "darktemplar"),
		},
		{
			// 2 Gate Reaver (PvP): ≥2 Gateways into Robotics + Reaver — the
			// gate-robo-gate PvP standard.
			Name: "2 Gate Reaver", PatternName: InitialBuildOrderPatternNamePrefix + "2 Gate Reaver", FeatureKey: "bo_p_2gate_reaver",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvP"},
			Rule: All(
				FirstBuildExists(subjRoboticsFacility),
				ProduceCountAtLeast(subjReaver, 1),
				CountBuildsBefore(subjGateway, 2, 600),
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Robotics Facility", Match: MatchBuild(subjRoboticsFacility), TargetSecond: 260, Tolerance: Asym(60, 100)},
				{Key: "First Reaver", Match: MatchFirstProduce(subjReaver), TargetSecond: 383, Tolerance: Asym(90, 120)},
			},
			SummaryPlayer: mkPill("2 Gate Reaver", "reaver"), GamesList: mkPill("2 Gate Reaver", "reaver"),
		},
		{
			// Sair/Speedlot (PvZ): Stargate Corsairs + Citadel zealot-speed off a
			// Forge/Gate expand — the dominant modern PvZ ground/air opener.
			Name: "Sair/Speedlot", PatternName: InitialBuildOrderPatternNamePrefix + "Sair/Speedlot", FeatureKey: "bo_p_sair_speedlot",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvZ"},
			Rule: All(
				FirstBuildExists(subjStargate),
				ProduceCountAtLeast(subjCorsair, 2), // the Sair half
				FirstBuildExists(subjCitadelOfAdun), // Citadel → zealot leg speed
				// Speedlot = the ground-zealot Sair variant: not Sair/DT, not
				// Sair/Reaver (those are distinct openers in the research doc).
				Not(ProduceCountAtLeast(subjDarkTemplar, 1)),
				Not(ProduceCountAtLeast(subjReaver, 1)),
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Stargate", Match: MatchBuild(subjStargate), TargetSecond: 280, Tolerance: Asym(60, 90)},
				{Key: "Citadel of Adun", Match: MatchBuild(subjCitadelOfAdun), TargetSecond: 332, Tolerance: Asym(60, 120)},
			},
			SummaryPlayer: mkPill("Sair/Speedlot", "corsair"), GamesList: mkPill("Sair/Speedlot", "corsair"),
		},

		// --- Terran opening-sequence openers (the new axis vs #155 composition). ---
		{
			// Siege Expand (TvP): 1 Rax → Factory (+ Machine Shop) → natural CC —
			// the safest TvP mech opener. Stays disjoint from a 2-Rax opening
			// (single Barracks before the Factory).
			Name: "Siege Expand", PatternName: InitialBuildOrderPatternNamePrefix + "Siege Expand", FeatureKey: "bo_t_siege_expand",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvT"},
			Rule: All(
				BuildBefore(subjBarracks, subjFactory),
				Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjFactory})),
				FirstBuildExists(subjMachineShop),
				BuildBefore(subjFactory, subjCommandCenter),
				FirstBuildBefore(subjCommandCenter, 360),
			),
			RuleDeadline: 360,
			Expert: []ExpertEvent{
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 150, Tolerance: Asym(20, 60)},
				{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 229, Tolerance: Asym(30, 80)},
			},
			SummaryPlayer: mkPill("Siege Expand", "siegetank"), GamesList: mkPill("Siege Expand", "siegetank"),
		},
		{
			// 2 Port Wraith (TvT): two Starports before any expansion — cloaked-
			// wraith harass into mech.
			Name: "2 Port Wraith", PatternName: InitialBuildOrderPatternNamePrefix + "2 Port Wraith", FeatureKey: "bo_t_2port_wraith",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvT"},
			Rule: All(
				CountBuildsBefore(subjStarport, 2, 600),
				NthBuildBeforeAll(subjStarport, 2, []string{subjCommandCenter}),
				ProduceCountAtLeast(subjWraith, 4), // it's a wraith build, not just 2 Starports
				// Disjoint from 2 Fact Vults: a wraith opener is 1 Factory into
				// 2 Starports, not a 2-Factory vulture opening.
				Not(NthBuildBeforeAll(subjFactory, 2, []string{subjStarport})),
			),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "1st Starport", Match: MatchBuild(subjStarport), TargetSecond: 201, Tolerance: Asym(25, 50)},
				{Key: "2nd Starport", Match: MatchNthBuild(subjStarport, 2), TargetSecond: 208, Tolerance: Asym(25, 60)},
			},
			SummaryPlayer: mkPill("2 Port Wraith", "wraith"), GamesList: mkPill("2 Port Wraith", "wraith"),
		},
		{
			// 2 Fact Vults (TvT): two Factories before any expansion / Starport —
			// the aggressive vulture/mine timing. Disjoint from 2 Port Wraith
			// (no early Starport).
			Name: "2 Fact Vults", PatternName: InitialBuildOrderPatternNamePrefix + "2 Fact Vults", FeatureKey: "bo_t_2fact_vults",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvT"},
			Rule: All(
				CountBuildsBefore(subjFactory, 2, 300),
				NthBuildBeforeAll(subjFactory, 2, []string{subjCommandCenter, subjStarport}),
				Not(FirstBuildBefore(subjStarport, 300)),
				// It's a vulture build — require early Vultures, which also keeps
				// it disjoint from any wraith/air opening that took 2 Factories.
				ProduceCountAtLeastBefore(subjVulture, 3, 360),
			),
			RuleDeadline: 360,
			Expert: []ExpertEvent{
				{Key: "1st Factory", Match: MatchBuild(subjFactory), TargetSecond: 147, Tolerance: Asym(25, 30)},
				{Key: "2nd Factory", Match: MatchNthBuild(subjFactory, 2), TargetSecond: 183, Tolerance: Asym(40, 50)},
			},
			SummaryPlayer: mkPill("2 Fact Vults", "vulture"), GamesList: mkPill("2 Fact Vults", "vulture"),
		},

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

		// --- Composition-based Terran BOs (issue #155). The old 1 Rax 1 Fac /
		// 1 Rax FE / 2 Rax CC / 1 Rax Bio openers collapse into this set,
		// classified by army composition at 10:00. ---
		{
			// Wraith: TvZ air build — 2+ Starports and 5+ Wraiths by 10:00.
			Name: "Wraith", PatternName: "Build Order: Wraith", FeatureKey: "bo_t_wraith",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Matchup: []string{"TvZ"},
			Rule:         All(tCohort, tcWraith),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 56, Tolerance: Asym(10, 24)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 84, Tolerance: Asym(28, 18)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 98, Tolerance: Asym(10, 60)},
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 152, Tolerance: Asym(12, 60)},
				{Key: "Starport", Match: MatchBuild(subjStarport), TargetSecond: 205, Tolerance: Asym(15, 60)},
				{Key: "First Wraith", Match: MatchFirstProduce(subjWraith), TargetSecond: 253, Tolerance: Asym(20, 70)},
			},
			SummaryPlayer: &Pill{Label: "Wraith", IconKey: "wraith"},
			GamesList:     &Pill{Label: "Wraith", IconKey: "wraith"},
		},
		{
			// Goliath: TvZ Goliath-dominant — ≤2 Vultures & ≤4 Marines by 7:00,
			// 4+ Goliaths by 10:00 (with tanks it's Mech instead).
			Name: "Goliath", PatternName: "Build Order: Goliath", FeatureKey: "bo_t_goliath",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Matchup: []string{"TvZ"},
			Rule:         All(tCohort, tcGoliath, Not(tcWraith)),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 55, Tolerance: Asym(10, 24)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 86, Tolerance: Asym(28, 18)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 102, Tolerance: Asym(12, 70)},
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 161, Tolerance: Asym(15, 120)},
				{Key: "Armory", Match: MatchBuild(subjArmory), TargetSecond: 242, Tolerance: Asym(40, 100)},
				{Key: "First Goliath", Match: MatchFirstProduce(subjGoliath), TargetSecond: 339, Tolerance: Asym(75, 70)},
			},
			SummaryPlayer: &Pill{Label: "Goliath", IconKey: "goliath"},
			GamesList:     &Pill{Label: "Goliath", IconKey: "goliath"},
		},
		// Bio (Marine/Medic predominant, 8+ Marines; TvZ or non-1v1), split by
		// Barracks count by 10:00.
		bioBucket("1-Rax Bio", "bo_t_bio_1rax", 1, true),
		bioBucket("2-Rax Bio", "bo_t_bio_2rax", 2, true),
		bioBucket("3-Rax Bio", "bo_t_bio_3rax", 3, true),
		bioBucket("4-Rax Bio", "bo_t_bio_4rax", 4, true),
		bioBucket("5-Rax Bio", "bo_t_bio_5rax", 5, true),
		bioBucket("6+ Rax Bio", "bo_t_bio_6rax", 6, false),
		{
			// 1-1-1 into Mech: early Starport + Wraith, then mech (≥2 Factories,
			// ≥1 Tank, mech-predominant).
			Name: "1-1-1 into Mech", PatternName: "Build Order: 1-1-1 into Mech", FeatureKey: "bo_t_111_mech",
			Race: RaceTerran, Kind: KindInitialBuildOrder,
			Rule:         All(tCohort, tcOneOneOne, tcFac2, tcMechPred, tcTank1, Not(tcWraith), Not(tcGoliath)),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 55, Tolerance: Asym(10, 24)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 85, Tolerance: Asym(26, 18)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 99, Tolerance: Asym(12, 70)},
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 153, Tolerance: Asym(12, 80)},
				{Key: "Starport", Match: MatchBuild(subjStarport), TargetSecond: 254, Tolerance: Asym(50, 70)},
				{Key: "First Siege Tank", Match: MatchFirstProduce(subjSiegeTank), TargetSecond: 312, Tolerance: Asym(80, 120)},
			},
			SummaryPlayer: &Pill{Label: "1-1-1 into Mech", IconKey: "siegetank"},
			GamesList:     &Pill{Label: "1-1-1 into Mech", IconKey: "siegetank"},
		},
		// Mech (mech-predominant, ≥1 Tank), split by Factory count by 10:00.
		mechBucket("2-Fac Mech", "bo_t_mech_2fac", 2, true),
		mechBucket("3-Fac Mech", "bo_t_mech_3fac", 3, true),
		mechBucket("4-Fac Mech", "bo_t_mech_4fac", 4, true),
		mechBucket("5-Fac Mech", "bo_t_mech_5fac", 5, true),
		mechBucket("6+ Fac Mech", "bo_t_mech_6fac", 6, false),
		// Tankless Mech (mech-predominant, no Tank by 10:00 — pure Vulture/Goliath).
		tanklessBucket("2-Fac Tankless Mech", "bo_t_tankless_2fac", 2, true),
		tanklessBucket("3-Fac Tankless Mech", "bo_t_tankless_3fac", 3, true),
		tanklessBucket("4-Fac Tankless Mech", "bo_t_tankless_4fac", 4, true),
		tanklessBucket("5-Fac Tankless Mech", "bo_t_tankless_5fac", 5, true),
		tanklessBucket("6+ Fac Tankless Mech", "bo_t_tankless_6fac", 6, false),
		{
			// 1-1-1: early Starport + Wraith that stays balanced (neither bio-
			// nor mech-predominant) — the classic Vulture/Tank/Wraith opener.
			Name: "1-1-1", PatternName: "Build Order: 1-1-1", FeatureKey: "bo_t_111",
			Race: RaceTerran, Kind: KindInitialBuildOrder,
			Rule:         All(tCohort, tcOneOneOne, Not(tcBio), Not(tcMechPred), Not(tcWraith), Not(tcGoliath)),
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 57, Tolerance: Asym(10, 24)},
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 85, Tolerance: Asym(28, 20)},
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 98, Tolerance: Asym(15, 70)},
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 160, Tolerance: Asym(15, 70)},
				{Key: "Starport", Match: MatchBuild(subjStarport), TargetSecond: 226, Tolerance: Asym(40, 80)},
				{Key: "First Wraith", Match: MatchFirstProduce(subjWraith), TargetSecond: 271, Tolerance: Asym(40, 90)},
			},
			SummaryPlayer: &Pill{Label: "1-1-1", IconKey: "starport"},
			GamesList:     &Pill{Label: "1-1-1", IconKey: "starport"},
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
			// (no CC by 300s) and no Factory tech (none by 240s), AND the spatial
			// gate (see tRuleBunkerRush). TierPreferred so a genuine rush — which
			// also matches its composition BO once tCohort stopped excluding
			// bunker topology — wins by precedence. endOfReplaySentinel because
			// the worldstate event only exists after the full stream is processed.
			Name:                   "Bunker Rush",
			PatternName:            "Build Order: Bunker Rush",
			FeatureKey:             "bo_bunker_rush",
			Race:                   RaceTerran,
			Kind:                   KindInitialBuildOrder,
			Tier:                   TierPreferred,
			Rule:                   tRuleBunkerRush,
			RequireWorldstateEvent: "bunker_rush",
			RuleDeadline:           endOfReplaySentinel,
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
			Tier:        TierResidual,
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
			Tier:        TierResidual,
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
			// Terran residual: a Terran opener that matches none of the named
			// builds — the kept topology openers (CC First / BBS / Bunker Rush)
			// nor any composition BO (Bio / Mech / Wraith / Goliath / 1-1-1).
			// In practice: too-short / tiny-army games, one-Factory builds, and
			// composition-balanced openers that aren't clearly bio or mech.
			// Defined as the exact complement Not(tNamed). RuleDeadline matches
			// the composition window so the residual sees the same facts.
			Name:        "Terran (Other)",
			PatternName: "Build Order: Terran (Other)",
			FeatureKey:  "bo_terran_other",
			Race:        RaceTerran,
			Kind:        KindInitialBuildOrder,
			Tier:        TierResidual,
			Rule: All(
				Any(
					FirstBuildExists(subjBarracks),
					FirstBuildExists(subjCommandCenter),
					FirstBuildExists(subjFactory),
				),
				Not(tNamed),
			),
			RuleDeadline:  600,
			SummaryPlayer: &Pill{Label: "Terran (Other)", IconKey: "marine"},
			GamesList:     &Pill{Label: "Terran (Other)", IconKey: "marine"},
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

		// NOTE: the former Terran style markers — "Mech", "1-1-1", "SK Terran"
		// and "Mech transition" — were promoted to first-class composition
		// initial BOs above (issue #155) and removed here.
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
			SummaryPlayer: &Pill{Label: "Cliff drop", IconKey: "dropship"},
			SummaryReplay: &Pill{Label: "Cliff drop", IconKey: "dropship"},
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
			// Double Stargate (PvZ): the Protoss player commits to 2 Stargates
			// (rather than the standard single Stargate, or none) and pumps a
			// significant Corsair count. 6+ Corsairs is well past the 2-3 a
			// one-base Sair opener produces for Overlord control, signalling a
			// dedicated air investment that only two Stargates sustain. Gated to
			// PvZ — the build is matchup-specific and meaningless elsewhere.
			Name:          "Double Stargate",
			PatternName:   "Double Stargate",
			FeatureKey:    "double_stargate",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvZ"},
			Rule:          All(BuildCountAtLeast(subjStargate, 2), ProduceCountAtLeast(subjCorsair, 6)),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Double Stargate", IconKey: "corsair", Style: PillStyleStrong, Title: "2 Stargates + 6 Corsairs (PvZ)"},
			GamesList:     &Pill{Label: "Double Stargate", IconKey: "corsair", Style: PillStyleStrong, Title: "2 Stargates + 6 Corsairs (PvZ)"},
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
			// Wraiths: air-heavy Terran play. The "mass air" threshold scales
			// with team format — 3+ Wraiths reads as a deliberate air opener in
			// a 1v1, whereas team games need 10+ (like 10+ Scouts) before the
			// count is signal rather than incidental harass. The format-aware
			// threshold lives in the evaluator (ctx.Replay.TeamFormat).
			Name:          "Wraiths",
			PatternName:   "Wraiths",
			FeatureKey:    "wraiths",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Custom:        func() CustomEvaluator { return &wraithCountEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Wraiths", IconKey: "wraith", Style: PillStyleStrong, Title: "Wraiths"},
			GamesList:     &Pill{Label: "Wraiths", IconKey: "wraith", Style: PillStyleStrong, Title: "Wraiths"},
		},
		{
			Name:             "Never upgraded",
			PatternName:      "Never upgraded",
			FeatureKey:       "never_upgraded",
			Kind:             KindMarker,
			Rule:             Not(HPUpgradeExists()),
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
				Title: "No weapon/armor/shield upgrades in this replay for this player (suppressed on games shorter than matchup-typical first upgrade).",
			},
		},
		{
			Name:             "Never researched",
			PatternName:      "Never researched",
			FeatureKey:       "never_researched",
			Kind:             KindMarker,
			Rule:             Not(Any(TechExists(), NonHPUpgradeExists())),
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
				Title: "No tech or non-HP upgrade commands in this replay for this player (suppressed on games shorter than matchup-typical first research).",
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
			SummaryPlayer: &Pill{Label: "Made drops"},
		},
		{
			Name:         "Offensive nydus canal",
			PatternName:  "Offensive nydus canal",
			FeatureKey:   "offensive_nydus",
			Kind:         KindMarker,
			Race:         RaceZerg,
			Custom:       func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "nydus_attack"} },
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{
				Label:   "Offensive nydus",
				IconKey: "nyduscanal",
				Style:   PillStyleStrong,
				Title:   "Built a forward Nydus Canal and teleported an army into enemy territory",
			},
			GamesList: &Pill{
				Label:   "Offensive nydus",
				IconKey: "nyduscanal",
				Style:   PillStyleStrong,
				Title:   "Built a forward Nydus Canal and teleported an army into enemy territory",
			},
		},
		{
			Name:          "Made recalls",
			PatternName:   "Made recalls",
			FeatureKey:    "made_recalls",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Custom:        func() CustomEvaluator { return &firstCastEvaluator{subject: "Recall"} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Recalls", IconKey: "arbiter"},
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
			SummaryPlayer: &Pill{Label: "Threw Nukes", IconKey: "ghost"},
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
				Label:   "Became Terran",
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
				Label:   "Became Zerg",
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
