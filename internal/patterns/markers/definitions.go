package markers

import (
	"fmt"
	"math"

	"github.com/marianogappa/screpdb/internal/models"
)

// secAfter rounds fractional SC:BW build times (Zealot 25.2s) to whole seconds.
func secAfter(anchor int, addends ...float64) int {
	total := float64(anchor)
	for _, a := range addends {
		total += a
	}
	return int(math.Round(total))
}

// zergPoolBO builds one rung of the pool-first ladder, keyed on the exact
// Drone-morph count so the rungs are mutually exclusive by construction.
// poolSec is the progamer-ideal Pool second for the UI golden compare.
func zergPoolBO(supply, poolSec int, expert ...ExpertEvent) Marker {
	drones := supply - 4
	label := fmt.Sprintf("%d Pool", supply)
	ev := expert
	if len(ev) == 0 {
		ev = []ExpertEvent{
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
		}
	}
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
		RuleDeadline:  180,
		Expert:        ev,
		SummaryPlayer: &Pill{Label: label, IconKey: "spawningpool"},
		GamesList:     &Pill{Label: label, IconKey: "spawningpool"},
	}
}

// zergHatchBO builds one rung of the hatch-first ladder, keyed on the exact
// Drone-morph count so the rungs are mutually exclusive by construction.
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

// zergHatchTechMarker layers "N Hatch <tech>" on top of the supply opener: the
// opening and the tech commitment are separate axes (issue #245).
func zergHatchTechMarker(tech zergTech, iconKey, featureKey string, matchup []string) Marker {
	name := "N Hatch " + tech.unit()
	pill := func() *Pill {
		return &Pill{Label: "{subject}", IconKey: iconKey, Subject: PayloadFieldSubject("label")}
	}
	return Marker{
		Name:          name,
		PatternName:   name,
		FeatureKey:    featureKey,
		Race:          RaceZerg,
		Kind:          KindMarker,
		Matchup:       matchup,
		Custom:        newZergHatchTech(tech),
		RuleDeadline:  600,
		SummaryPlayer: pill(),
		SummaryReplay: pill(),
		GamesList:     pill(),
	}
}

// Build order definitions. Everything else (detectors, game-list featuring, UI
// pills, Build Orders tab) picks up changes via the registry below. Times are
// seconds from game start; RuleDeadline is the last second the Rule could flip,
// after which the detector finalizes and frees its event buffer.

const (
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
	subjUltralisk        = models.GeneralUnitUltralisk
	subjGuardian         = models.GeneralUnitGuardian
	subjZergCarapace     = models.UpgradeZergCarapace

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
	subjObserver         = models.GeneralUnitObserver
	subjCitadelOfAdun    = models.GeneralUnitCitadelOfAdun
	subjTemplarArchives  = models.GeneralUnitTemplarArchives
	subjDarkTemplar      = models.GeneralUnitDarkTemplar
	subjLegEnhancement   = models.UpgradeLegEnhancementZealotSpeed

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
	subjValkyrie       = models.GeneralUnitValkyrie
	subjScienceVessel  = models.GeneralUnitScienceVessel
	subjBattlecruiser  = models.GeneralUnitBattlecruiser
	subjCloakingField  = models.TechCloakingField
)

// RuleDeadline for markers only resolvable at end-of-replay; well past any
// real replay, and the detector still finalizes when the replay ends.
const endOfReplaySentinel = 10 * 60 * 60 // 10 hours

var defaultTol = Sym(5)

// A func, not a var, so initialization order can't trip on cross-file refs.
func allMarkers() []Marker {
	// Shared opener rules. Each is declared once and referenced by both its BO
	// entry and its race's residual "… (Other)", defined as the exact complement
	// Not(Any(named…)) so the residual can never drift out of sync.

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
	// The Nexus-before-Cyber guard is what keeps this disjoint from 1 Gate Core.
	pRuleGateExpand := All(
		BuildBefore(subjGateway, subjForge),
		BuildBefore(subjGateway, subjNexus),
		FirstBuildBefore(subjNexus, 220),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjNexus})),
		Not(BuildBefore(subjCyberneticsCore, subjNexus)),
	)
	// Windows are loose because the corpus shows progamer FFEs with the Forge up
	// to ~140s and the Nexus up to ~260s.
	pRuleForgeExpand := All(
		FirstBuildBefore(subjForge, 140),
		BuildBefore(subjForge, subjGateway),
		BuildBefore(subjForge, subjNexus),
		FirstBuildBefore(subjNexus, 260),
		BuildBefore(subjNexus, subjGateway),
	)
	// "no expa" is baked in: a Gateway build that does expand is Gate Expand.
	pRule1GateNoExpa := All(
		FirstBuildExists(subjGateway),
		Not(FirstBuildExists(subjForge)),
		Not(FirstBuildBefore(subjNexus, 300)),
		Not(FirstBuildBefore(subjCyberneticsCore, 180)),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore, subjNexus, subjForge})),
	)
	// Keys on the build sequence only — proxy-vs-defensive cannons are the
	// separate cannon_rush marker's spatial distinction.
	pRuleForgeCannonNoExpa := All(
		FirstBuildExists(subjForge),
		FirstBuildExists(subjPhotonCannon),
		Not(FirstBuildBefore(subjNexus, 300)),
		Not(FirstBuildBefore(subjCyberneticsCore, 180)),
		Not(NthBuildBeforeAll(subjGateway, 2, []string{subjCyberneticsCore, subjNexus, subjForge})),
	)
	pNamed := Any(pRule1GateCore, pRule2Gate, pRuleNexusFirst, pRuleGateExpand, pRuleForgeExpand, pRule1GateNoExpa, pRuleForgeCannonNoExpa)

	// Terran. Everything but CC First and BBS is reclassified by army composition
	// at 10:00 (issue #155), so tCohort excludes only those two.
	tRuleCCFirst := All(
		BuildBefore(subjCommandCenter, subjBarracks),
		FirstBuildBefore(subjCommandCenter, 200),
		// A 2nd depot before the CC means the player floated or teched first.
		Not(NthBuildBeforeAll(subjSupplyDepot, 2, []string{subjCommandCenter})),
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
	// Location and commitment define a bunker rush, not the economy. A SINGLE
	// early bunker is usually a poke or wall (it stole 2 Port Wraith, Bio and
	// Factory-Expand builds), so the 2nd forward bunker is the commitment
	// signal (issue #226/#227).
	tRuleBunkerRush := All(
		CountBuildsBefore(subjBunker, 2, 240),
		BuildBefore(subjBarracks, subjBunker),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjBunker})),
	)

	// Only CC First and BBS are excluded: Bunker Rush needs a spatial gate, so its
	// players stay eligible here and lose to it by tier precedence instead.
	tCohort := All(Not(tRuleCCFirst), Not(tRuleBBS))

	// "Predominant" = strict majority of produced army.
	bioUnits := []string{subjMarine, subjMedic, subjFirebat}
	mechUnits := []string{subjVulture, subjGoliath, subjSiegeTank}
	tcBioPred := Predominant(bioUnits, mechUnits, 600)
	tcMechPred := Predominant(mechUnits, bioUnits, 600)
	// The 8-Marine floor screens out players who make a few Marines on the way
	// into mech or air; those always have a Factory or Starport, so a
	// no-transition opening below the floor (died / left early) is still Bio.
	tcBioNoTransition := All(
		ProduceCountAtLeastBefore(subjMarine, 1, 600),
		Not(FirstBuildBefore(subjFactory, 600)),
		Not(FirstBuildBefore(subjStarport, 600)),
	)
	tcBio := All(tcBioPred, Any(ProduceCountAtLeastBefore(subjMarine, 8, 600), tcBioNoTransition))
	// The bio/mech transition that often follows lands after the two Starports, so
	// extra Barracks/Factories must not disqualify the opener.
	tcWraith := All(
		CountBuildsBefore(subjStarport, 2, 600),
		Not(NthBuildBeforeAll(subjBarracks, 2, []string{subjStarport})),
		Not(NthBuildBeforeAll(subjFactory, 2, []string{subjStarport})),
		ProduceCountAtLeastBefore(subjWraith, 5, 600),
	)
	tcTank1 := ProduceCountAtLeastBefore(subjSiegeTank, 1, 600)
	tcTank0 := ProduceCountAtMostBefore(subjSiegeTank, 0, 600)
	// Goliath is a composition flavor of mech, not its own opener (issue #227).
	tcGoliathDom := All(tcTank0, Predominant([]string{subjGoliath}, []string{subjVulture}, 600))
	tcOneOneOne := All(FirstBuildBefore(subjStarport, 420), ProduceCountAtLeastBefore(subjWraith, 1, 600))
	// Ignores what came before the Starports. The Wraith floor is 3, not 5: a
	// 3-Starport opener pumping only 3 Wraiths by 10:00 is still a Wraith build.
	tc2Starport := NthBuildWithinGapOfFirst(subjStarport, 2, 90)
	tc3Starport := NthBuildWithinGapOfFirst(subjStarport, 3, 120)
	tcWraithAir := ProduceCountAtLeastBefore(subjWraith, 3, 600)
	tcValkAir := All(ProduceCountAtLeastBefore(subjValkyrie, 2, 600), Not(tcWraithAir))
	tc3StarportWraith := All(tc3Starport, tcWraithAir)
	tc3StarportValkyrie := All(tc3Starport, tcValkAir)
	tc2StarportWraith := All(tc2Starport, Not(tc3Starport), tcWraithAir)
	tc2StarportValkyrie := All(tc2Starport, Not(tc3Starport), tcValkAir)

	// The matchup-free union of every Terran BO, so the residual is its exact
	// complement. The composition BOs are matchup-gated but this union is not, so
	// an off-matchup composition is subtracted from the residual with no BO firing
	// — a deliberate, rare coverage gap.
	tNamed := Any(
		tRuleCCFirst, tRuleBBS,
		All(tCohort, tc2Starport), // covers 2 and 3 Starport (3 implies the 2-cluster)
		All(tCohort, tcBio),
		All(tCohort, tcMechPred), // every mech-predominant build maps to a Mech / Goliath / Tankless bucket
		All(tCohort, tcOneOneOne),
	)

	// Measured per MEASUREMENT.md over the progamer corpus (issue #362). Split in
	// two because factory-before-expansion takes gas a full minute earlier than
	// expand-first, and one pooled table modelled both badly. The one-base buckets
	// pool with fact-first: their backbone medians agree.
	tMechOpeningFactFirst := []ExpertEvent{
		{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 54, Tolerance: Asym(2, 3)}, // n=671, p10/50/90 = 53/54/57
		{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 84, Tolerance: Asym(3, 4)},        // n=671, p10/50/90 = 81/84/88
		{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 97, Tolerance: Asym(2, 4)},        // n=668, p10/50/90 = 95/97/101
		{Key: "1st Factory", Match: MatchBuild(subjFactory), TargetSecond: 148, Tolerance: Asym(3, 4)},     // n=671, p10/50/90 = 145/148/152
	}
	tMechOpeningExpandFirst := []ExpertEvent{
		{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 54, Tolerance: Asym(2, 3)}, // n=154, p10/50/90 = 53/54/57
		{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 85, Tolerance: Asym(2, 10)},       // n=154, p10/50/90 = 83/85/95
		{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 164, Tolerance: Asym(6, 9)},       // n=154, p10/50/90 = 158/164/173
		{Key: "1st Factory", Match: MatchBuild(subjFactory), TargetSecond: 216, Tolerance: Asym(7, 27)},    // n=154, p10/50/90 = 209/216/243
	}

	// Pairwise-disjoint via the bio/mech predominance split, the Tank
	// present/absent split, the exact count, and Not() guards against the
	// higher-signal Wraith/Goliath/1-1-1 rules.
	mkPill := func(label, icon string) *Pill { return &Pill{Label: label, IconKey: icon} }
	// Split by base count, not Barracks count: the rax count keeps growing all
	// game with no clean "the opening ended here" moment, so 1-Rax…6-Rax were
	// fragile. One-base bio is all-in/pressure, two-base is macro. Per-bucket
	// expert tables because 1-base takes its Refinery ~40s earlier than 2-base.
	bioBase := func(name, fkey string, expandRule Predicate, expert []ExpertEvent) Marker {
		return Marker{
			Name: name, PatternName: InitialBuildOrderPatternNamePrefix + name, FeatureKey: fkey,
			Race: RaceTerran, Kind: KindInitialBuildOrder, Matchup: []string{"TvZ", MatchupNon1v1},
			Rule:          All(tCohort, tcBio, Not(tcWraith), Not(tcGoliathDom), expandRule),
			RuleDeadline:  600,
			Modifiers:     []Modifier{{Name: "proxy", WorldstateEvent: "proxy_rax"}},
			Expert:        expert,
			SummaryPlayer: mkPill(name, "marine"), GamesList: mkPill(name, "marine"),
		}
	}
	// Named by Factories built STRICTLY BEFORE the first expansion, which is
	// deterministic — a by-deadline count conflated pre- and post-expansion ones.
	type mechComp struct {
		suffix, key, icon string
		gate              Predicate
	}
	mechComps := []mechComp{
		{"Mech", "mech", "siegetank", tcTank1},
		{"Goliath", "goliath", "goliath", tcGoliathDom},
		{"Tankless Mech", "tankless", "vulture", All(tcTank0, Not(tcGoliathDom))},
	}
	mechEv := func(c mechComp, factFirst bool, extra ...ExpertEvent) []ExpertEvent {
		backbone := tMechOpeningExpandFirst
		if factFirst {
			backbone = tMechOpeningFactFirst
		}
		ev := append([]ExpertEvent{}, backbone...)
		ev = append(ev, extra...)
		switch c.key {
		case "goliath":
			// unmeasured legacy (n=14 fact-first, n=4 expand-first)
			return append(ev, ExpertEvent{Key: "First Goliath", Match: MatchFirstProduce(subjGoliath), TargetSecond: 339, Tolerance: Asym(75, 110)})
		case "tankless":
			if factFirst {
				return append(ev, ExpertEvent{Key: "First Vulture", Match: MatchFirstProduce(subjVulture), TargetSecond: 201, Tolerance: Asym(3, 23)}) // n=49, p10/50/90 = 198/201/224
			}
			// unmeasured legacy (n=6): expand-first vultures land ~294s, but the sample
			// is below the floor.
			return append(ev, ExpertEvent{Key: "First Vulture", Match: MatchFirstProduce(subjVulture), TargetSecond: 205, Tolerance: Asym(20, 50)})
		default:
			if factFirst {
				return append(ev, ExpertEvent{Key: "First Siege Tank", Match: MatchFirstProduce(subjSiegeTank), TargetSecond: 276, Tolerance: Asym(44, 135)}) // n=608, p10/50/90 = 232/276/411
			}
			return append(ev, ExpertEvent{Key: "First Siege Tank", Match: MatchFirstProduce(subjSiegeTank), TargetSecond: 301, Tolerance: Asym(10, 129)}) // n=144, p10/50/90 = 291/301/430
		}
	}
	// The backbone is count-invariant but the expansion CC is not: each extra
	// pre-expansion Factory pushes it ~110s later. n>=3 is below the sample floor
	// and reuses the 2-Fact table.
	tMechExpansionCC := func(n int) ExpertEvent {
		if n >= 2 {
			return ExpertEvent{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 321, Tolerance: Asym(52, 99)} // n=35, p10/50/90 = 269/321/420
		}
		return ExpertEvent{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 211, Tolerance: Asym(14, 62)} // n=593, p10/50/90 = 197/211/273
	}
	mechCore := func(name, fkey string, facRule Predicate, c mechComp, ev []ExpertEvent) Marker {
		return Marker{
			Name: name, PatternName: InitialBuildOrderPatternNamePrefix + name, FeatureKey: fkey,
			Race: RaceTerran, Kind: KindInitialBuildOrder,
			Rule:         All(tCohort, facRule, tcMechPred, c.gate, Not(tc2Starport), Not(tcOneOneOne)),
			RuleDeadline: 600,
			// "proxy" flags a forward Factory at the enemy; orthogonal to the mech
			// flavour and factory count.
			Modifiers:     []Modifier{{Name: "proxy", WorldstateEvent: "proxy_factory"}},
			Expert:        ev,
			SummaryPlayer: mkPill(name, c.icon), GamesList: mkPill(name, c.icon),
		}
	}
	// n==6 is the "6+" top rung ("at least").
	mechExpa := func(n int, c mechComp) Marker {
		head := fmt.Sprintf("%d Fact Expa", n)
		facRule := BuildCountBeforeFirstBuildOf(subjFactory, subjCommandCenter, n)
		if n >= 6 {
			head = "6+ Fact Expa"
			facRule = BuildCountAtLeastBeforeFirstBuildOf(subjFactory, subjCommandCenter, 6)
		}
		name := head + " " + c.suffix
		fkey := fmt.Sprintf("bo_t_%s_expa_%dfac", c.key, n)
		return mechCore(name, fkey, facRule, c, mechEv(c, true, tMechExpansionCC(n)))
	}
	mechPlain := func(c mechComp) Marker {
		fkey := fmt.Sprintf("bo_t_%s_expand", c.key)
		return mechCore(c.suffix, fkey, BuildCountBeforeFirstBuildOf(subjFactory, subjCommandCenter, 0), c, mechEv(c, false))
	}
	mechNoExpa := func(c mechComp) Marker {
		fkey := fmt.Sprintf("bo_t_%s_noexpa", c.key)
		facRule := All(Not(FirstBuildBefore(subjCommandCenter, 600)), CountBuildsBefore(subjFactory, 2, 600))
		ev := mechEv(c, true)
		if c.key == "mech" {
			// One-base pools with fact-first on the backbone but not the tank: skipping
			// the expansion buys a much earlier First Siege Tank than the family's 276.
			ev[len(ev)-1] = ExpertEvent{Key: "First Siege Tank", Match: MatchFirstProduce(subjSiegeTank), TargetSecond: 229, Tolerance: Asym(3, 53)} // n=20, p10/50/90 = 226/229/282
		}
		return mechCore("1-Base "+c.suffix, fkey, facRule, c, ev)
	}
	oneOneOne := func(name, fkey, icon string, comp Predicate) Marker {
		return Marker{
			Name: name, PatternName: "Build Order: " + name, FeatureKey: fkey,
			Race: RaceTerran, Kind: KindInitialBuildOrder,
			Rule:         All(tCohort, tcOneOneOne, comp, Not(tc2Starport)),
			RuleDeadline: 600,
			// Pooled over the three 1-1-1 buckets (n=129). The wide late sides are real
			// spread, not a missing split: TvT stretches its Starport to ~340s while TvZ
			// lands ~218s, with no clean seam to cut on.
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 54, Tolerance: Asym(2, 22)}, // n=129, p10/50/90 = 53/54/76
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 85, Tolerance: Asym(25, 3)},        // n=129, p10/50/90 = 60/85/88
				{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 98, Tolerance: Asym(3, 62)},        // n=129, p10/50/90 = 95/98/160
				{Key: "Factory", Match: MatchBuild(subjFactory), TargetSecond: 148, Tolerance: Asym(3, 58)},         // n=129, p10/50/90 = 145/148/206
				{Key: "Starport", Match: MatchBuild(subjStarport), TargetSecond: 230, Tolerance: Asym(28, 89)},      // n=129, p10/50/90 = 202/230/319
			},
			SummaryPlayer: &Pill{Label: name, IconKey: icon}, GamesList: &Pill{Label: name, IconKey: icon},
		}
	}

	ms := []Marker{
		// TIER-1 PREFERRED OPENERS (issue #182). Pairwise-disjoint WITHIN tier 1 per
		// (race, matchup); across tiers overlap is expected and resolved by tier
		// precedence. Deliberately stays MORE SPECIFIC than tier 2 — where the broad
		// composition bucket is already the better classification (issue #155), no
		// blanket tier-1 opener is added.

		// Zerg tech-composition markers, named by the town halls standing at the
		// economy→army transition rather than a fixed clock. Base count comes from
		// town-hall tag evidence, so a cancelled or re-placed Hatchery doesn't inflate
		// N (issue #245). See zergHatchTechEvaluator.
		zergHatchTechMarker(techHydra, "hydralisk", "nhatch_hydra", []string{"PvZ"}),
		zergHatchTechMarker(techMuta, "mutalisk", "nhatch_muta", []string{"TvZ"}),
		zergHatchTechMarker(techLurker, "lurker", "nhatch_lurker", []string{"TvZ"}),

		{
			Name: "1 Gate Reaver", PatternName: InitialBuildOrderPatternNamePrefix + "1 Gate Reaver", FeatureKey: "bo_p_1gate_reaver",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvT"},
			Rule: All(
				FirstBuildExists(subjRoboticsFacility),
				ProduceCountAtLeast(subjReaver, 1),
				// A 2nd Gateway before the first Reaver makes it a 2-Gate build.
				Not(NthBuildBeforeFirstProduce(subjGateway, 2, subjReaver)),
				Not(ProduceCountAtLeast(subjDarkTemplar, 1)),
			),
			// "expand" separates the economic variant from one-base reaver pressure.
			Modifiers:    []Modifier{{Name: "expand", Rule: NthBuildBeforeFirstProduce(subjNexus, 1, subjReaver)}},
			RuleDeadline: 600,
			Expert: []ExpertEvent{
				{Key: "Robotics Facility", Match: MatchBuild(subjRoboticsFacility), TargetSecond: 243, Tolerance: Asym(22, 38)}, // n=34, p10/50/90 = 221/243/281
				{Key: "First Reaver", Match: MatchFirstProduce(subjReaver), TargetSecond: 339, Tolerance: Asym(27, 36)},         // n=34, p10/50/90 = 312/339/375
			},
			SummaryPlayer: mkPill("1 Gate Reaver", "reaver"), GamesList: mkPill("1 Gate Reaver", "reaver"),
		},
		// 2 Gate DT, 2 Gate Reaver and Sair/Speedlot were removed as openers: they
		// classified the post-opening tech composition, not the opening itself. They
		// are timing / composition markers below instead.

		// Protoss cannon-contain openers (PvZ). The three permutations differ only in
		// the build order of {Gate, Forge, Cannon}, so they are disjoint by ordering.
		// TierPreferred so they refine the base opener when the shape is present.
		{
			Name: "Gate Forge Cannon before expa", PatternName: InitialBuildOrderPatternNamePrefix + "Gate Forge Cannon before expa", FeatureKey: "bo_p_gate_forge_cannon",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvZ"},
			Rule: All(
				BuildBefore(subjGateway, subjForge),
				BuildBefore(subjForge, subjPhotonCannon),
				FirstBuildExists(subjPhotonCannon),
				BuildBefore(subjPhotonCannon, subjCyberneticsCore),
				BuildBefore(subjPhotonCannon, subjNexus),
			),
			RuleDeadline: 320,
			Expert: []ExpertEvent{
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 73, Tolerance: Asym(2, 3)},                // n=27, p10/50/90 = 71/73/76
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 137, Tolerance: Asym(21, 117)},                // n=27, p10/50/90 = 116/137/254
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 171, Tolerance: Asym(19, 117)}, // n=27, p10/50/90 = 152/171/288
			},
			SummaryPlayer: mkPill("Gate Forge Cannon (before expa)", "photoncannon"), GamesList: mkPill("Gate Forge Cannon (before expa)", "photoncannon"),
		},
		{
			Name: "Forge Cannon Gate before expa", PatternName: InitialBuildOrderPatternNamePrefix + "Forge Cannon Gate before expa", FeatureKey: "bo_p_forge_cannon_gate",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvZ"},
			Rule: All(
				BuildBefore(subjForge, subjPhotonCannon),
				BuildBefore(subjPhotonCannon, subjGateway),
				FirstBuildExists(subjGateway),
				BuildBefore(subjGateway, subjCyberneticsCore),
				BuildBefore(subjGateway, subjNexus),
			),
			RuleDeadline: 320,
			// unmeasured legacy (n=7)
			Expert: []ExpertEvent{
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 96, Tolerance: Asym(30, 60)},
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 126, Tolerance: Asym(30, 60)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 144, Tolerance: Asym(30, 80)},
			},
			SummaryPlayer: mkPill("Forge Cannon Gate (before expa)", "photoncannon"), GamesList: mkPill("Forge Cannon Gate (before expa)", "photoncannon"),
		},
		{
			Name: "Forge Gate Cannon before expa", PatternName: InitialBuildOrderPatternNamePrefix + "Forge Gate Cannon before expa", FeatureKey: "bo_p_forge_gate_cannon",
			Race: RaceProtoss, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"PvZ"},
			Rule: All(
				BuildBefore(subjForge, subjGateway),
				BuildBefore(subjGateway, subjPhotonCannon),
				FirstBuildExists(subjPhotonCannon),
				BuildBefore(subjPhotonCannon, subjCyberneticsCore),
				BuildBefore(subjPhotonCannon, subjNexus),
			),
			RuleDeadline: 320,
			// unmeasured legacy (n=1)
			Expert: []ExpertEvent{
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 96, Tolerance: Asym(30, 60)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 130, Tolerance: Asym(30, 70)},
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 160, Tolerance: Asym(30, 80)},
			},
			SummaryPlayer: mkPill("Forge Gate Cannon (before expa)", "photoncannon"), GamesList: mkPill("Forge Gate Cannon (before expa)", "photoncannon"),
		},

		// Terran air opener. Absorbs the former "2 Port Wraith" and, unlike the old
		// rule, ignores what was built before the Starports. The matchup-gated
		// "Factory Expand" and "2 Fact before Expa" openers were retired: they are 1-
		// and 2-Factory expands and now fall to the "N Fact Expa Mech" buckets.
		{
			Name: "2 Starport Wraith", PatternName: InitialBuildOrderPatternNamePrefix + "2 Starport Wraith", FeatureKey: "bo_t_2starport_wraith",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvT", "TvZ"},
			Rule:         All(tCohort, tc2StarportWraith),
			RuleDeadline: 600,
			// "expand" is the economic variant; "proxy" is forward Starports at the enemy.
			Modifiers: []Modifier{
				{Name: "expand", Rule: BuildBefore(subjCommandCenter, subjStarport)},
				{Name: "proxy", WorldstateEvent: "proxy_starport"},
			},
			// The wide late sides are the "expand" variant, not a missing split: 1-base
			// games sit tight while expand games land ~70s later, and already carry the
			// modifier pill that explains it.
			Expert: []ExpertEvent{
				{Key: "1st Starport", Match: MatchBuild(subjStarport), TargetSecond: 201, Tolerance: Asym(12, 126)},       // n=77, p10/50/90 = 189/201/327
				{Key: "2nd Starport", Match: MatchNthBuild(subjStarport, 2), TargetSecond: 210, Tolerance: Asym(20, 117)}, // n=77, p10/50/90 = 190/210/327
				{Key: "First Wraith", Match: MatchFirstProduce(subjWraith), TargetSecond: 248, Tolerance: Asym(12, 131)},  // n=77, p10/50/90 = 236/248/379
			},
			SummaryPlayer: mkPill("2 Starport Wraith", "wraith"), GamesList: mkPill("2 Starport Wraith", "wraith"),
		},
		{
			Name: "2 Starport Valkyrie", PatternName: InitialBuildOrderPatternNamePrefix + "2 Starport Valkyrie", FeatureKey: "bo_t_2starport_valk",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvT", "TvZ"},
			Rule:         All(tCohort, tc2StarportValkyrie),
			RuleDeadline: 600,
			Modifiers: []Modifier{
				{Name: "expand", Rule: BuildBefore(subjCommandCenter, subjStarport)},
				{Name: "proxy", WorldstateEvent: "proxy_starport"},
			},
			// unmeasured legacy (n=8)
			Expert: []ExpertEvent{
				{Key: "1st Starport", Match: MatchBuild(subjStarport), TargetSecond: 205, Tolerance: Asym(40, 90)},
				{Key: "2nd Starport", Match: MatchNthBuild(subjStarport, 2), TargetSecond: 212, Tolerance: Asym(40, 90)},
				{Key: "First Valkyrie", Match: MatchFirstProduce(subjValkyrie), TargetSecond: 300, Tolerance: Asym(60, 120)},
			},
			SummaryPlayer: mkPill("2 Starport Valkyrie", "valkyrie"), GamesList: mkPill("2 Starport Valkyrie", "valkyrie"),
		},
		{
			// Same family as 2 Starport, but the cluster size names it (issue #227).
			Name: "3 Starport Wraith", PatternName: InitialBuildOrderPatternNamePrefix + "3 Starport Wraith", FeatureKey: "bo_t_3starport_wraith",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvT", "TvZ"},
			Rule:         All(tCohort, tc3StarportWraith),
			RuleDeadline: 600,
			Modifiers: []Modifier{
				{Name: "expand", Rule: BuildBefore(subjCommandCenter, subjStarport)},
				{Name: "proxy", WorldstateEvent: "proxy_starport"},
			},
			// unmeasured legacy (n=6)
			Expert: []ExpertEvent{
				{Key: "1st Starport", Match: MatchBuild(subjStarport), TargetSecond: 205, Tolerance: Asym(40, 90)},
				{Key: "3rd Starport", Match: MatchNthBuild(subjStarport, 3), TargetSecond: 225, Tolerance: Asym(40, 90)},
				{Key: "First Wraith", Match: MatchFirstProduce(subjWraith), TargetSecond: 253, Tolerance: Asym(40, 90)},
			},
			SummaryPlayer: mkPill("3 Starport Wraith", "wraith"), GamesList: mkPill("3 Starport Wraith", "wraith"),
		},
		{
			Name: "3 Starport Valkyrie", PatternName: InitialBuildOrderPatternNamePrefix + "3 Starport Valkyrie", FeatureKey: "bo_t_3starport_valk",
			Race: RaceTerran, Kind: KindInitialBuildOrder, Tier: TierPreferred, Matchup: []string{"TvT", "TvZ"},
			Rule:         All(tCohort, tc3StarportValkyrie),
			RuleDeadline: 600,
			Modifiers: []Modifier{
				{Name: "expand", Rule: BuildBefore(subjCommandCenter, subjStarport)},
				{Name: "proxy", WorldstateEvent: "proxy_starport"},
			},
			// unmeasured legacy (n=1)
			Expert: []ExpertEvent{
				{Key: "1st Starport", Match: MatchBuild(subjStarport), TargetSecond: 205, Tolerance: Asym(40, 90)},
				{Key: "3rd Starport", Match: MatchNthBuild(subjStarport, 3), TargetSecond: 225, Tolerance: Asym(40, 90)},
				{Key: "First Valkyrie", Match: MatchFirstProduce(subjValkyrie), TargetSecond: 300, Tolerance: Asym(60, 120)},
			},
			SummaryPlayer: mkPill("3 Starport Valkyrie", "valkyrie"), GamesList: mkPill("3 Starport Valkyrie", "valkyrie"),
		},

		// Pool-first BOs key off exact pre-Pool morph counts: the early-game spam
		// filter (internal/earlyfilter) strips engine-impossible morphs, so the
		// surviving stream is a faithful supply count.
		{
			Name:        "4 Pool",
			PatternName: "Build Order: 4 Pool",
			FeatureKey:  "bo_4_pool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Supply 4 at Pool placement = 0 Drone morphs, 0 Overlords.
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 0),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 0),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
			),
			RuleDeadline: 60,
			// unmeasured legacy (n=7)
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 33,
					Tolerance:    Sym(4),
				},
				{
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: secAfter(33, models.BuildTimeSpawningPool),
					Tolerance:    Sym(3),
				},
			},
			SummaryPlayer: &Pill{Label: "4 Pool", IconKey: "spawningpool"},
			GamesList:     &Pill{Label: "4 Pool", IconKey: "spawningpool"},
		},
		// 5–8 Pool needs no Overlord gate: supply <9 needs no Overlord, and the exact
		// Drone count alone keeps each rung disjoint from every other pool BO.
		zergPoolBO(5, 45), // unmeasured legacy (n=3)
		zergPoolBO(6, 52), // unmeasured legacy (n=1)
		zergPoolBO(7, 60), // unmeasured legacy (n=1)
		zergPoolBO(8, 67), // unmeasured legacy (n=0)
		{
			Name:        "9 Pool",
			PatternName: "Build Order: 9 Pool",
			FeatureKey:  "bo_9_pool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Supply 9 = 5 Drone morphs with no Overlord yet; the same count WITH the
				// Overlord already morphed is 9 Overpool, its own BO.
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 5),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 0),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
				// Mutex with "9 Pool into Hatchery", which owns the fast follow-up Hatch.
				Not(BuildAfterWithin(subjHatchery, subjSpawningPool, 60)),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 62, // n=348, p10/50/90 = 60/62/63
					Tolerance:    Sym(2),
				},
				{
					// Measured directly, not derived from the Pool: build time does not include
					// larva availability.
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: 114, // n=348, p10/50/90 = 113/114/116
					Tolerance:    Sym(2),
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
				// Supply 9 with the Overlord morphed BEFORE the Pool (vs plain 9 Pool).
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
					TargetSecond: 73, // n=514, p10/50/90 = 71/73/75
					Tolerance:    Sym(2),
				},
				{
					Key:          "First Zerglings",
					Match:        MatchFirstProduce(subjZergling),
					TargetSecond: 126, // n=513, p10/50/90 = 124/126/128
					Tolerance:    Sym(2),
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
				// Supply 12 = 8 Drone morphs + 1 Overlord (needed to lift the cap past 9).
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 8),
				ProduceCountBeforeBuild(subjOverlord, subjSpawningPool, 1),
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
			),
			RuleDeadline: 180,
			// unmeasured legacy (n=0)
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
		// 10–11 Pool force an Overlord first (cap 9), but the exact Drone-morph count
		// already makes them disjoint, so no explicit Overlord gate is needed.
		zergPoolBO(10, 84,
			ExpertEvent{Key: "Spawning Pool", Match: MatchBuild(subjSpawningPool), TargetSecond: 84, Tolerance: Sym(2)},           // n=20, p10/50/90 = 83/84/85
			ExpertEvent{Key: "First Zerglings", Match: MatchFirstProduce(subjZergling), TargetSecond: 136, Tolerance: Asym(2, 3)}, // n=20, p10/50/90 = 136/136/139
		),
		zergPoolBO(11, 98), // unmeasured legacy (n=12)
		{
			Name:        "9 Pool into Hatchery",
			PatternName: "Build Order: 9 Pool into Hatchery",
			FeatureKey:  "bo_9_pool_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// The exact 5-Drone count, not a loose "≥1 Drone", keeps this disjoint from
				// the 5–8/10–11 Pool rungs.
				ProduceCountBeforeBuild(subjDrone, subjSpawningPool, 5),
				NoProduceBeforeBuild(subjOverlord, subjSpawningPool),
				FirstBuildAtOrAfter(subjSpawningPool, 70),
				FirstBuildBefore(subjSpawningPool, 120),
				BuildAfterWithin(subjHatchery, subjSpawningPool, 60),
			),
			RuleDeadline: 180, // pool ≤120 + hatch ≤60 after pool
			// unmeasured legacy (n=0)
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
			SummaryPlayer: &Pill{Label: "9 Pool 9 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "9 Pool 9 Hatch", IconKey: "hatchery"},
		},
		// 4–8 Hatch: a Hatchery costs 300 minerals, so placing one at supply 4–8 is a
		// real greedy/fast expansion rather than noise.
		zergHatchBO(4, 40), // unmeasured legacy (n=8)
		zergHatchBO(5, 50), // unmeasured legacy (n=17)
		zergHatchBO(6, 58), // unmeasured legacy (n=11)
		zergHatchBO(7, 66), // unmeasured legacy (n=13)
		zergHatchBO(8, 70), // unmeasured legacy (n=3)
		{
			Name:        "9 Hatch",
			PatternName: "Build Order: 9 Hatch",
			FeatureKey:  "bo_9_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Supply 9 = 5 Drone morphs, no Overlord (the cap blocks further morphs).
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 5),
				ProduceCountBeforeBuild(subjOverlord, subjHatchery, 0),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 150,
			// unmeasured legacy (n=4)
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
		// Hatch-first BOs key off exact pre-Hatch morph counts. 10/11/12 Hatch all
		// require an Overlord first because reaching supply >9 demands cap expansion.
		{
			Name:        "10 Hatch",
			PatternName: "Build Order: 10 Hatch",
			FeatureKey:  "bo_10_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// No Overlord gate: reaching the 10th Drone before an Overlord is legal via
				// the extractor trick, which the early-game economy sim models, so the Drone
				// count alone stays supply-faithful.
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 6),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 180,
			// unmeasured legacy (n=2)
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
			// unmeasured legacy (n=3)
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
			// unmeasured legacy (n=2)
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
		{
			Name:        "13 Hatch",
			PatternName: "Build Order: 13 Hatch",
			FeatureKey:  "bo_13_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				ProduceCountBeforeBuild(subjDrone, subjHatchery, 9),
				ProduceCountBeforeBuild(subjOverlord, subjHatchery, 1),
				Not(BuildBefore(subjSpawningPool, subjHatchery)),
				Not(BuildBefore(subjEvolutionChamber, subjHatchery)),
			),
			RuleDeadline: 180,
			// unmeasured legacy (n=0)
			Expert: []ExpertEvent{
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 104,
					Tolerance:    defaultTol,
				},
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 122,
					Tolerance:    Asym(3, 10),
				},
			},
			SummaryPlayer: &Pill{Label: "13 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "13 Hatch", IconKey: "hatchery"},
		},
		{
			// Fires only when no exact rung does: a multi-unit-selection Drone morph makes
			// the count ambiguous, because the replay records selection size rather than
			// how many of them were larvae.
			Name:          "Zerg opening (approximate)",
			PatternName:   "Build Order: Zerg opening (approximate)",
			FeatureKey:    "bo_z_fuzzy",
			Race:          RaceZerg,
			Kind:          KindInitialBuildOrder,
			Tier:          TierBackup,
			Custom:        func() CustomEvaluator { return newZergOpenerFuzzyEvaluator() },
			RuleDeadline:  180,
			SummaryPlayer: &Pill{Label: "{subject}", IconKey: "drone", Subject: PayloadFieldSubject("label")},
			GamesList:     &Pill{Label: "{subject}", IconKey: "drone", Subject: PayloadFieldSubject("label")},
		},
		// Protoss openers, mutually exclusive within (Protoss, matchup) by build-order
		// topology — each requires its defining building before the others'.

		{
			Name:         "1 Gate Core",
			PatternName:  "Build Order: 1 Gate Core",
			FeatureKey:   "bo_1_gate_core",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRule1GateCore,
			RuleDeadline: 180,
			// Assimilator / Cybernetics Core are genuinely bimodal in PvT (gas-first ~90s
			// vs gas-later ~120s), so their late tolerance is wide by design.
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 46, Tolerance: Sym(2)},                            // n=435, p10/50/90 = 46/46/48
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 73, Tolerance: Sym(2)},                        // n=435, p10/50/90 = 73/73/75
				{Key: "Assimilator", Match: MatchBuild(subjAssimilator), TargetSecond: 93, Tolerance: Asym(5, 27)},           // n=392, p10/50/90 = 88/93/120
				{Key: "Cybernetics Core", Match: MatchBuild(subjCyberneticsCore), TargetSecond: 116, Tolerance: Asym(2, 29)}, // n=435, p10/50/90 = 114/116/145
			},
			SummaryPlayer: &Pill{Label: "1 Gate Core", IconKey: "cyberneticscore"},
			GamesList:     &Pill{Label: "1 Gate Core", IconKey: "cyberneticscore"},
		},
		{
			Name:         "2 Gate",
			PatternName:  "Build Order: 2 Gate",
			FeatureKey:   "bo_2_gate",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRule2Gate,
			RuleDeadline: 180,
			Modifiers:    []Modifier{{Name: "proxy", WorldstateEvent: "proxy_gate"}},
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 47, Tolerance: Asym(12, 3)},                // n=124, p10/50/90 = 35/47/50
				{Key: "1st Gateway", Match: MatchBuild(subjGateway), TargetSecond: 73, Tolerance: Asym(7, 3)},         // n=124, p10/50/90 = 66/73/76
				{Key: "2nd Gateway", Match: MatchNthBuild(subjGateway, 2), TargetSecond: 90, Tolerance: Asym(15, 12)}, // n=124, p10/50/90 = 75/90/102
				{
					// Measured directly, not derived from the 1st Gateway: pros cut the probe
					// before the Zealot, landing it ~8s after the arithmetic says.
					Key:          "First Zealot",
					Match:        MatchFirstProduce(subjZealot),
					TargetSecond: 116, // n=105, p10/50/90 = 113/116/128
					Tolerance:    Asym(3, 12),
				},
			},
			SummaryPlayer: &Pill{Label: "2 Gate", IconKey: "gateway"},
			GamesList:     &Pill{Label: "2 Gate", IconKey: "gateway"},
		},
		{
			// Upper bound loosened from the legacy 150s: the data shows Nexus placement
			// up to ~170s in PvT.
			Name:         "Nexus First",
			PatternName:  "Build Order: Nexus First",
			FeatureKey:   "bo_nexus_first",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleNexusFirst,
			RuleDeadline: 200,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 47, Tolerance: Sym(2)},           // n=160, p10/50/90 = 46/47/48
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 105, Tolerance: Asym(2, 6)},      // n=160, p10/50/90 = 104/105/111
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 126, Tolerance: Asym(4, 36)}, // n=158, p10/50/90 = 122/126/162
			},
			SummaryPlayer: &Pill{Label: "Nexus First", IconKey: "nexus"},
			GamesList:     &Pill{Label: "Nexus First", IconKey: "nexus"},
		},
		{
			Name:         "Gate Expand",
			PatternName:  "Build Order: Gate Expand",
			FeatureKey:   "bo_gate_expand",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleGateExpand,
			RuleDeadline: 220,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 47, Tolerance: Sym(2)},        // n=206, p10/50/90 = 45/47/48
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 72, Tolerance: Sym(4)},    // n=206, p10/50/90 = 68/72/76
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 164, Tolerance: Asym(43, 12)}, // n=206, p10/50/90 = 121/164/176
			},
			SummaryPlayer: &Pill{Label: "Gate Expand", IconKey: "nexus"},
			GamesList:     &Pill{Label: "Gate Expand", IconKey: "nexus"},
		},
		{
			// Forge upper bound loosened from the legacy 90s to admit slower openers.
			Name:         "Forge Expand",
			PatternName:  "Build Order: Forge Expand",
			FeatureKey:   "bo_forge_expa",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleForgeExpand,
			RuleDeadline: 260,
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 47, Tolerance: Sym(2)},                       // n=175, p10/50/90 = 46/47/49
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 91, Tolerance: Asym(7, 10)},                  // n=175, p10/50/90 = 84/91/101
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 142, Tolerance: Asym(15, 28)}, // n=163, p10/50/90 = 127/142/170
				{Key: "Nexus", Match: MatchBuild(subjNexus), TargetSecond: 135, Tolerance: Asym(7, 47)},                 // n=175, p10/50/90 = 128/135/182
			},
			SummaryPlayer: &Pill{Label: "FFE", IconKey: "forge"},
			GamesList:     &Pill{Label: "FFE", IconKey: "forge"},
		},
		{
			// Proxy vs in-base cannons is the cannon_rush marker's distinction.
			Name:         "Forge Cannon (no expa)",
			PatternName:  "Build Order: Forge Cannon (no expa)",
			FeatureKey:   "bo_forge_cannon_no_expa",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRuleForgeCannonNoExpa,
			RuleDeadline: 320,
			// unmeasured legacy (n=0)
			Expert: []ExpertEvent{
				{Key: "Forge", Match: MatchBuild(subjForge), TargetSecond: 90, Tolerance: Sym(20)},
				{Key: "Photon Cannon", Match: MatchBuild(subjPhotonCannon), TargetSecond: 130, Tolerance: Sym(30)},
			},
			SummaryPlayer: &Pill{Label: "Forge Cannon (no expa)", IconKey: "photoncannon"},
			GamesList:     &Pill{Label: "Forge Cannon (no expa)", IconKey: "photoncannon"},
		},
		{
			Name:         "1 Gate (no expa)",
			PatternName:  "Build Order: 1 Gate (no expa)",
			FeatureKey:   "bo_1_gate_no_expa",
			Race:         RaceProtoss,
			Kind:         KindInitialBuildOrder,
			Rule:         pRule1GateNoExpa,
			RuleDeadline: 320,
			// unmeasured legacy (n=7)
			Expert: []ExpertEvent{
				{Key: "Pylon", Match: MatchBuild(subjPylon), TargetSecond: 48, Tolerance: Sym(6)},
				{Key: "Gateway", Match: MatchBuild(subjGateway), TargetSecond: 88, Tolerance: Sym(15)},
			},
			SummaryPlayer: &Pill{Label: "1 Gate (no expa)", IconKey: "gateway"},
			GamesList:     &Pill{Label: "1 Gate (no expa)", IconKey: "gateway"},
		},

		// Terran openers, mutually exclusive within (Terran, matchup): BBS commits at
		// the 2nd Rax before any Depot, CC First needs the CC before any Rax, and the
		// rest split on Refinery-vs-Factory-vs-CC ordering.

		// Composition-based Terran BOs (issue #155): the old 1 Rax 1 Fac / 1 Rax FE /
		// 2 Rax CC / 1 Rax Bio openers collapse into this set, classified by army
		// composition at 10:00. The TvZ "Wraith" opener folded into 2 Port Wraith
		// (same build) and the standalone "Goliath" into the mech flavors (#227).
		bioBase("1-Base Bio", "bo_t_bio_1base", Not(FirstBuildBefore(subjCommandCenter, 360)), []ExpertEvent{
			{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 55, Tolerance: Asym(8, 50)}, // n=40, p10/50/90 = 47/55/105
			{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 77, Tolerance: Asym(23, 12)},       // n=40, p10/50/90 = 54/77/89
			{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 152, Tolerance: Asym(53, 109)},     // n=35, p10/50/90 = 99/152/261
			{Key: "Academy", Match: MatchBuild(subjAcademy), TargetSecond: 200, Tolerance: Asym(37, 172)},       // n=32, p10/50/90 = 163/200/372
		}),
		bioBase("2-Base Bio", "bo_t_bio_2base", FirstBuildBefore(subjCommandCenter, 360), []ExpertEvent{
			{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 55, Tolerance: Asym(6, 7)},        // n=405, p10/50/90 = 49/55/62
			{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 84, Tolerance: Asym(7, 2)},               // n=405, p10/50/90 = 77/84/86
			{Key: "Refinery", Match: MatchBuild(subjRefinery), TargetSecond: 190, Tolerance: Asym(91, 37)},            // n=404, p10/50/90 = 99/190/227
			{Key: "Academy", Match: MatchBuild(subjAcademy), TargetSecond: 230, Tolerance: Asym(23, 183)},             // n=388, p10/50/90 = 207/230/413
			{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 166, Tolerance: Asym(19, 41)}, // n=405, p10/50/90 = 147/166/207
		}),
		// 1-1-1 (early Starport + Wraith), named by the transition. Mech vs
		// Tankless Mech split by tank presence; balanced "1-1-1" is neither bio-
		// nor mech-predominant (the classic Vulture/Tank/Wraith opener).
		oneOneOne("1-1-1 Mech", "bo_t_111_mech", "siegetank", All(tcMechPred, tcTank1)),
		oneOneOne("1-1-1 Tankless Mech", "bo_t_111_tankless", "vulture", All(tcMechPred, tcTank0)),
		oneOneOne("1-1-1", "bo_t_111", "starport", All(Not(tcBio), Not(tcMechPred))),
		// Mech named by Factories built strictly before the first expansion CC, which
		// is deterministic. Generated just below this slice via mechFamily().
		{
			// True CC First is Depot, CC, Rax — the D-B-C-D-R variant falls under Rax-CC.
			Name:         "CC First",
			PatternName:  "Build Order: CC First",
			FeatureKey:   "bo_cc_first",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRuleCCFirst,
			RuleDeadline: 200,
			Expert: []ExpertEvent{
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 54, Tolerance: Sym(2)},          // n=49, p10/50/90 = 52/54/56
				{Key: "Command Center", Match: MatchBuild(subjCommandCenter), TargetSecond: 115, Tolerance: Asym(2, 5)}, // n=50, p10/50/90 = 113/115/120
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 135, Tolerance: Asym(5, 3)},            // n=50, p10/50/90 = 130/135/138
			},
			SummaryPlayer: &Pill{Label: "CC First", IconKey: "commandcenter"},
			GamesList:     &Pill{Label: "CC First", IconKey: "commandcenter"},
		},
		{
			// All-in 2-Rax before any other Terran building. Rare in modern pro play but
			// a recognizable signature.
			Name:         "BBS",
			PatternName:  "Build Order: BBS",
			FeatureKey:   "bo_bbs",
			Race:         RaceTerran,
			Kind:         KindInitialBuildOrder,
			Rule:         tRuleBBS,
			RuleDeadline: 120,
			Modifiers:    []Modifier{{Name: "proxy", WorldstateEvent: "proxy_rax"}},
			// unmeasured legacy (n=7)
			Expert: []ExpertEvent{
				{Key: "1st Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 60, Tolerance: Sym(8)},
				{Key: "2nd Barracks", Match: MatchNthBuild(subjBarracks, 2), TargetSecond: 80, Tolerance: Sym(8)},
				{Key: "Supply Depot", Match: MatchBuild(subjSupplyDepot), TargetSecond: 100, Tolerance: Sym(10)},
			},
			SummaryPlayer: &Pill{Label: "BBS", IconKey: "barracks"},
			GamesList:     &Pill{Label: "BBS", IconKey: "barracks"},
		},
		{
			// TierPreferred so a genuine rush — which also matches its composition BO now
			// that tCohort no longer excludes bunker topology — wins by precedence.
			// endOfReplaySentinel because the spatial gate's worldstate event only exists
			// once the full stream is processed.
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
				{Key: "Barracks", Match: MatchBuild(subjBarracks), TargetSecond: 57, Tolerance: Asym(15, 27)}, // n=30, p10/50/90 = 42/57/84
				{Key: "Bunker", Match: MatchBuild(subjBunker), TargetSecond: 164, Tolerance: Asym(26, 23)},    // n=30, p10/50/90 = 138/164/187
			},
			SummaryPlayer: &Pill{Label: "Bunker Rush", IconKey: "bunker"},
			GamesList:     &Pill{Label: "Bunker Rush", IconKey: "bunker"},
		},

		// Residual "… (Other)" catch-alls, one per race: the EXACT complement of the
		// race's named openers, gated on the player having placed a defining opener
		// building, so every classifiable player lands on exactly one initial BO.
		// Players who never place one fall to "Opener unresolved" below.
		{
			// The greedy tail of the Drone ladder: a Pool or expansion Hatchery placed at
			// supply ≥13, which no exact rung claims.
			Name:        "Pool/Hatch (Other)",
			PatternName: "Build Order: Pool/Hatch (Other)",
			FeatureKey:  "bo_zerg_other",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Tier:        TierResidual,
			Rule: Any(
				All(
					FirstBuildExists(subjSpawningPool),
					ProduceCountAtLeastBeforeBuild(subjDrone, subjSpawningPool, 9),
					Not(BuildBefore(subjHatchery, subjSpawningPool)),
					Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
				),
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
			// In practice: too-short / tiny-army games, one-Factory builds, and
			// composition-balanced openers that are clearly neither bio nor mech.
			// RuleDeadline matches the composition window so the residual sees the same
			// facts as the buckets it complements.
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
			// The player never placed a defining opener building — almost always a
			// sub-2-minute abort, instant leave or dodge. Race-agnostic, and stored so the
			// dashboard can render "—" and coverage can be reported over classifiable
			// players instead of counting these as misses.
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

		// KindMarker entries, which may coexist with each other and with a
		// KindInitialBuildOrder. PatternName is kept equal to the old imperative
		// detector's Name() so DB rows and frontend checks stay compatible.

		{
			// Fires iff the opponent also matches the turret-timing burst, via a shared
			// cross-player gate in mutalisk_turret_timing.go that walks the worldstate
			// engine's full enriched stream at Finalize. No Expert bands: the only
			// progamer reference is the muta-vs-turret completion gap, surfaced on the
			// Mutalisk Timing tab (see populateMutaliskTimingForGameDetail).
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
			// Replaces the former 1/2 Gate Reaver openers: Reaver tech is common in
			// PvP/PvT, so a timing signal reads better than a build order.
			Name:          "First Reaver",
			PatternName:   "First Reaver",
			FeatureKey:    "first_reaver",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvP", "PvT"},
			Custom:        firstUnitTiming(subjReaver, 600),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "1st Reaver {timestamp}", IconKey: "reaver"},
			SummaryReplay: &Pill{Label: "1st Reaver {timestamp}", IconKey: "reaver"},
			GamesList:     &Pill{Label: "1st Reaver {timestamp}", IconKey: "reaver"},
			EventsList:    &Pill{Label: "trains first Reaver", IconKey: "reaver"},
		},
		{
			Name:          "First Corsair",
			PatternName:   "First Corsair",
			FeatureKey:    "first_corsair",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvZ"},
			Custom:        firstUnitTiming(subjCorsair, 600),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "1st Corsair {timestamp}", IconKey: "corsair"},
			SummaryReplay: &Pill{Label: "1st Corsair {timestamp}", IconKey: "corsair"},
			GamesList:     &Pill{Label: "1st Corsair {timestamp}", IconKey: "corsair"},
			EventsList:    &Pill{Label: "trains first Corsair", IconKey: "corsair"},
		},
		{
			// Reports the second Cloaking Field research FINISHES (start + 63s), not the
			// start: a research the game ends before completing yields no cloaked Wraiths.
			// Not gated to the opener — Cloaking Field in TvZ/TvT is a wraith build's tell.
			Name:          "Wraith Cloak timing",
			PatternName:   "Wraith Cloak timing",
			FeatureKey:    "wraith_cloak_timing",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Matchup:       []string{"TvZ", "TvT"},
			Custom:        firstTechCompletionTiming(subjCloakingField, 0),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Wraith Cloak {timestamp}", IconKey: "wraith"},
			SummaryReplay: &Pill{Label: "Wraith Cloak {timestamp}", IconKey: "wraith"},
			GamesList:     &Pill{Label: "Wraith Cloak {timestamp}", IconKey: "wraith"},
			EventsList:    &Pill{Label: "finishes Wraith Cloak research", IconKey: "wraith"},
		},
		{
			// Reports the second Leg Enhancement FINISHES, not the start: a research the
			// game ends before completing produces no Speedlots, so no marker.
			Name:          "Speedlot timing",
			PatternName:   "Speedlot timing",
			FeatureKey:    "speedlot_timing",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvZ"},
			Custom:        firstUpgradeCompletionTiming(subjLegEnhancement, 600),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Zealot Speed {timestamp}", IconKey: "zealot"},
			SummaryReplay: &Pill{Label: "Zealot Speed {timestamp}", IconKey: "zealot"},
			GamesList:     &Pill{Label: "Zealot Speed {timestamp}", IconKey: "zealot"},
			EventsList:    &Pill{Label: "finishes Zealot Speed research", IconKey: "zealot"},
		},
		{
			// Per-player pill only: the point is comparing the two players' Observer
			// timings in the mirror or vs Terran.
			Name:          "First Observer",
			PatternName:   "First Observer",
			FeatureKey:    "first_observer",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvP", "PvT"},
			Custom:        firstUnitTiming(subjObserver, 0),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "1st Observer {timestamp}", IconKey: "observer"},
		},
		{
			// Per-player pill only: the point is the distance between the two players'
			// first-mine timings.
			Name:          "First Mine",
			PatternName:   "First Mine",
			FeatureKey:    "first_mine",
			Kind:          KindMarker,
			Race:          RaceTerran,
			Matchup:       []string{"PvT"},
			Custom:        firstMineTiming(0),
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "1st Mine {timestamp}", IconKey: "vulture"},
		},
		{
			// The former Sair/Speedlot opener, demoted to a presence-only composition
			// marker because the opening underneath is FFE or Gate Expand.
			Name:          "Sair/Speedlot",
			PatternName:   "Sair/Speedlot",
			FeatureKey:    "sair_speedlot",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvZ"},
			Custom:        func() CustomEvaluator { return &sairSpeedlotEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Sair/Speedlot", IconKey: "corsair"},
			SummaryReplay: &Pill{Label: "Sair/Speedlot", IconKey: "corsair"},
			GamesList:     &Pill{Label: "Sair/Speedlot", IconKey: "corsair"},
		},
		{
			// Presence-only and high-confidence: the worldstate engine applies the
			// conservative per-player volley bar, so this marker just surfaces the flag.
			// No timeline pill — per-window timing is too error-prone to render.
			Name:          "Muta hit-n-run",
			PatternName:   "Muta hit-n-run",
			FeatureKey:    "muta_hitnrun",
			Kind:          KindMarker,
			Race:          RaceZerg,
			Custom:        func() CustomEvaluator { return &mutaHitnRunEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Muta hit-n-run", IconKey: "mutalisk"},
			SummaryReplay: &Pill{Label: "Muta hit-n-run", IconKey: "mutalisk"},
			GamesList:     &Pill{Label: "Muta hit-n-run", IconKey: "mutalisk"},
		},
		{
			// Map gating happens inside the evaluator's Finalize via IsBigGameHuntersMap.
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
			// Time-bounded (2nd Stargate + 6th Corsair by 7:30) because a corpus survey
			// found an 8-35min tail that is really a Carrier transition, not the
			// double-Stargate technique — false positives when the rule was unbounded.
			Name:          "Double Stargate",
			PatternName:   "Double Stargate",
			FeatureKey:    "double_stargate",
			Kind:          KindMarker,
			Race:          RaceProtoss,
			Matchup:       []string{"PvZ"},
			Rule:          All(CountBuildsBefore(subjStargate, 2, 450), ProduceCountAtLeastBefore(subjCorsair, 6, 450)),
			RuleDeadline:  450,
			SummaryPlayer: &Pill{Label: "Double Stargate", IconKey: "corsair", Style: PillStyleStrong, Title: "2 Stargates + 6 Corsairs by 7:30 (PvZ)"},
			GamesList:     &Pill{Label: "Double Stargate", IconKey: "corsair", Style: PillStyleStrong, Title: "2 Stargates + 6 Corsairs by 7:30 (PvZ)"},
		},
		{
			// Money-map signature: Scouts are uneconomic on Regular maps (275m + 125g +
			// Stargate), so the MapKind gate keeps the pill noise-free on standard games
			// even if a player accidentally produces a few.
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
			// The "mass air" threshold scales with team format (3+ in 1v1, 10+ in team
			// games, where a low count is incidental harass rather than signal); it lives
			// in the evaluator via ctx.Replay.TeamFormat.
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
			// p5 of the first HP-upgrade command over the progamer corpus
			// (MEASUREMENT.md), HP-only to match this marker's rule. The previous floors
			// were measured over all upgrades, so fast non-HP researches (Zergling speed
			// at ~2:05) dragged the Zerg rows below any real carapace timing.
			MinReplaySecondsByMatchup: map[Race]map[Race]int{
				RaceTerran: {
					RaceTerran:  399, // n=207, p5=6:39
					RaceProtoss: 325, // n=461, p5=5:25
					RaceZerg:    231, // n=525, p5=3:51
				},
				RaceProtoss: {
					RaceTerran:  236, // n=227, p5=3:56
					RaceProtoss: 346, // n=95,  p5=5:46
					RaceZerg:    245, // n=382, p5=4:05
				},
				RaceZerg: {
					RaceTerran:  312, // n=481, p5=5:12
					RaceProtoss: 348, // n=492, p5=5:48
					RaceZerg:    297, // n=92,  p5=4:57
				},
			},
			SummaryPlayer: &Pill{
				Label: "🚫 upgrades",
				Style: PillStyleNegative,
				Title: "No weapon/armor/shield upgrades in this replay for this player.",
			},
		},
		{
			Name:             "Never researched",
			PatternName:      "Never researched",
			FeatureKey:       "never_researched",
			Kind:             KindMarker,
			Rule:             Not(Any(TechExists(), NonHPUpgradeExists())),
			RuleDeadline:     endOfReplaySentinel,
			MinReplaySeconds: 10 * 60, // fallback for non-1v1
			// p5 of the first tech-or-non-HP-upgrade command over the progamer corpus
			// (MEASUREMENT.md), matching this marker's rule exactly. The previous floors
			// counted Tech commands only, so early non-HP upgrades didn't count and the
			// floors over-suppressed — PvT sat at 8:16 when pros research by ~2:46.
			MinReplaySecondsByMatchup: map[Race]map[Race]int{
				RaceTerran: {
					RaceTerran:  234, // n=322, p5=3:54
					RaceProtoss: 229, // n=616, p5=3:49
					RaceZerg:    230, // n=602, p5=3:50
				},
				RaceProtoss: {
					RaceTerran:  166, // n=369, p5=2:46
					RaceProtoss: 164, // n=274, p5=2:44
					RaceZerg:    309, // n=341, p5=5:09
				},
				RaceZerg: {
					RaceTerran:  181, // n=592, p5=3:01
					RaceProtoss: 158, // n=717, p5=2:38
					RaceZerg:    124, // n=557, p5=2:04
				},
			},
			SummaryPlayer: &Pill{
				Label: "🚫 researches",
				Style: PillStyleNegative,
				Title: "No tech or non-HP upgrade commands in this replay for this player.",
			},
		},

		{
			Name:         "Made drops",
			PatternName:  "Made drops",
			FeatureKey:   "made_drops",
			Kind:         KindMarker,
			Custom:       func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "drop"} },
			RuleDeadline: endOfReplaySentinel,
			// Suppressed on the summary player row when the backend already emits a drop
			// game_event (the frontend de-dupes via trustGameEventsForDrops); the pill
			// still exists for the Events-list and raw consumers.
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
			// Manner pylon: a Pylon placed inside the enemy's starting base to
			// block worker mining. Sourced from the worldstate spatial pass.
			Name:         "Manner pylon",
			PatternName:  "Manner pylon",
			FeatureKey:   "manner_pylon",
			Kind:         KindMarker,
			Race:         RaceProtoss,
			Custom:       func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "manner_pylon"} },
			RuleDeadline: endOfReplaySentinel,
			SummaryPlayer: &Pill{
				Label:   "Manner pylon",
				IconKey: "pylon",
				Style:   PillStyleStrong,
				Title:   "Placed a Pylon in the enemy's mineral line to block worker mining",
			},
			SummaryReplay: &Pill{Label: "Manner pylon", IconKey: "pylon", Style: PillStyleStrong, Title: "Placed a Pylon in the enemy's mineral line to block worker mining"},
			GamesList:     &Pill{Label: "Manner pylon", IconKey: "pylon", Style: PillStyleStrong, Title: "Placed a Pylon in the enemy's mineral line to block worker mining"},
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
			// Made Maelstrom (PvZ): the Dark Archon Maelstrom cast — a strong tell
			// of a Protoss commitment to mass templar tech vs Zerg. Surfaced only
			// as a games-list pill + filter; spell casting is already shown in the
			// game-summary spell-cast surface, so no summary pills here.
			Name:         "Made Maelstrom",
			PatternName:  "Made Maelstrom",
			FeatureKey:   "made_maelstrom",
			Kind:         KindMarker,
			Race:         RaceProtoss,
			Matchup:      []string{"PvZ"},
			Custom:       func() CustomEvaluator { return &firstCastEvaluator{subject: "Maelstrom"} },
			RuleDeadline: endOfReplaySentinel,
			GamesList:    &Pill{Label: "Maelstrom", IconKey: "darkarchon"},
		},
		{
			// Crazy Zerg (TvZ): the player goes from Mutalisk straight into
			// Ultralisk — with Zerg Carapace upgraded — and never morphs a Lurker
			// before the first Ultralisk. The "crazy" read is skipping the standard
			// Lurker midgame for an all-in ground transition.
			Name:          "Crazy Zerg",
			PatternName:   "Crazy Zerg",
			FeatureKey:    "crazy_zerg",
			Kind:          KindMarker,
			Race:          RaceZerg,
			Matchup:       []string{"TvZ"},
			Custom:        func() CustomEvaluator { return &crazyZergEvaluator{} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Crazy Zerg", IconKey: "ultralisk"},
			SummaryReplay: &Pill{Label: "Crazy Zerg", IconKey: "ultralisk"},
			GamesList:     &Pill{Label: "Crazy Zerg", IconKey: "ultralisk"},
		},
		{
			// Guardians (TvZ): the player morphs at least one Guardian (Greater
			// Spire air-to-ground tech). Game-level signal — game summary + games
			// list, but no per-player user pill and not a filter chip.
			Name:          "Guardians",
			PatternName:   "Guardians",
			FeatureKey:    "guardians",
			Kind:          KindMarker,
			Race:          RaceZerg,
			Matchup:       []string{"TvZ"},
			Rule:          ProduceCountAtLeast(subjGuardian, 1),
			RuleDeadline:  endOfReplaySentinel,
			SummaryReplay: &Pill{Label: "Guardians", IconKey: "guardian"},
			GamesList:     &Pill{Label: "Guardians", IconKey: "guardian"},
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
			// Deliberately no pill surfaces: this feeds the viewport-multitasking widget.
		},

		// Positive hotkey usage lives in players.hotkey_stream, so only the negative
		// signal remains a marker.

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
		// Phase-boundary markers: registry-only stubs so the storage layer's
		// markers.ByPatternName() lookup resolves their FeatureKey on insert. The data
		// comes from detectors/phase_boundary_detector.go, which emits PatternResults
		// sharing these PatternNames. No pills: these are queried server-side only.
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
	// Generated here so adding a flavor is one line.
	for _, c := range mechComps {
		for n := 1; n <= 6; n++ {
			ms = append(ms, mechExpa(n, c))
		}
		ms = append(ms, mechPlain(c), mechNoExpa(c))
	}
	return ms
}
