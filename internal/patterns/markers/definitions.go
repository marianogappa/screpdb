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
	subjSpawningPool     = models.GeneralUnitSpawningPool
	subjHatchery         = models.GeneralUnitHatchery
	subjEvolutionChamber = models.GeneralUnitEvolutionChamber
	subjDrone            = models.GeneralUnitDrone
	subjOverlord         = models.GeneralUnitOverlord
	subjZergling         = models.GeneralUnitZergling

	subjNexus   = models.GeneralUnitNexus
	subjGateway = models.GeneralUnitGateway
	subjForge   = models.GeneralUnitForge
	subjZealot  = models.GeneralUnitZealot
	subjCarrier = models.GeneralUnitCarrier

	subjFactory        = models.GeneralUnitFactory
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
		{
			Name:        "4 Pool",
			PatternName: "Build Order: 4 Pool",
			FeatureKey:  "bo_4_pool",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// "no drones nor overlords before Spawning Pool"
				NoProduceBeforeBuild(subjDrone, subjSpawningPool),
				NoProduceBeforeBuild(subjOverlord, subjSpawningPool),
				// Pool-tech openings: no Hatch / Evo Chamber may precede the Pool.
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
				// "Spawning Pool built before 1 minute"
				FirstBuildBefore(subjSpawningPool, 60),
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
				// "player makes drones but no overlords before Spawning Pool"
				ProduceBeforeBuild(subjDrone, subjSpawningPool),
				NoProduceBeforeBuild(subjOverlord, subjSpawningPool),
				// Pool-tech opening: no Hatch / Evo Chamber before the Pool.
				// (Otherwise the replay is a hatch-first BO, not 9 Pool.)
				Not(BuildBefore(subjHatchery, subjSpawningPool)),
				Not(BuildBefore(subjEvolutionChamber, subjSpawningPool)),
				// No fast Hatchery follow-up — that's "9 Pool into Hatchery",
				// kept mutually exclusive from plain 9 Pool.
				Not(BuildAfterWithin(subjHatchery, subjSpawningPool, 60)),
				// Pool must be between the fast end of the expert range (73-3=70)
				// and 2 minutes. The lower bound prevents 4-Pool-ish timings
				// from being classified as 9 Pool.
				FirstBuildAtOrAfter(subjSpawningPool, 70),
				FirstBuildBefore(subjSpawningPool, 120),
			),
			RuleDeadline: 180, // covers Hatch follow-up check up to Pool+60s.
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
				// Hatch before Pool, within the hatch-first window.
				BuildBefore(subjHatchery, subjSpawningPool),
				// Upper bound = 12 Hatch's earliest acceptable Hatchery timing
				// (98 - 5 = 93). Keeps 9 Hatch and 12 Hatch mutually exclusive.
				FirstBuildBefore(subjHatchery, 93),
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
		{
			Name:        "12 Hatch",
			PatternName: "Build Order: 12 Hatch",
			FeatureKey:  "bo_12_hatch",
			Race:        RaceZerg,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// "hatchery is built before pool" — permissive: Pool may not exist yet,
				// but if it does, Hatch must precede it.
				BuildBefore(subjHatchery, subjSpawningPool),
				// Lower bound = 12 Hatch's earliest acceptable Hatchery timing
				// (98 - 5 = 93). Keeps 9 Hatch and 12 Hatch mutually exclusive.
				FirstBuildAtOrAfter(subjHatchery, 93),
				// "hatchery built within 2m30s"
				FirstBuildBefore(subjHatchery, 150),
			),
			RuleDeadline: 150,
			Expert: []ExpertEvent{
				{
					Key:          "Hatchery",
					Match:        MatchBuild(subjHatchery),
					TargetSecond: 98, // 1m38
					Tolerance:    defaultTol,
				},
				{
					Key:          "Spawning Pool",
					Match:        MatchBuild(subjSpawningPool),
					TargetSecond: 116, // 1m56
					Tolerance:    Asym(3, 10),
				},
			},
			SummaryPlayer: &Pill{Label: "12 Hatch", IconKey: "hatchery"},
			GamesList:     &Pill{Label: "12 Hatch", IconKey: "hatchery"},
		},
		{
			Name:        "Nexus First",
			PatternName: "Build Order: Nexus First",
			FeatureKey:  "bo_nexus_first",
			Race:        RaceProtoss,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// Nexus before Gateway AND before Forge — otherwise it's a
				// Forge Expand or Gateway-opener, not a true Nexus First.
				BuildBefore(subjNexus, subjGateway),
				BuildBefore(subjNexus, subjForge),
				// "Nexus built within 2m30s"
				FirstBuildBefore(subjNexus, 150),
			),
			RuleDeadline: 150,
			Expert: []ExpertEvent{
				{
					Key:          "Nexus",
					Match:        MatchBuild(subjNexus),
					TargetSecond: 106,
					Tolerance:    Sym(6),
				},
				{
					Key:          "Gateway",
					Match:        MatchBuild(subjGateway),
					TargetSecond: 126,
					Tolerance:    Sym(8),
				},
			},
			SummaryPlayer: &Pill{Label: "Nexus First", IconKey: "nexus"},
			GamesList:     &Pill{Label: "Nexus First", IconKey: "nexus"},
		},
		{
			Name:        "Forge Expand",
			PatternName: "Build Order: Forge Expand",
			FeatureKey:  "bo_forge_expa",
			Race:        RaceProtoss,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// "Forge is built before Gateway & Nexus within 1m30"
				FirstBuildBefore(subjForge, 90),
				BuildBefore(subjForge, subjGateway),
				BuildBefore(subjForge, subjNexus),
				// "then Nexus is built before Gateway before 3m"
				FirstBuildBefore(subjNexus, 180),
				BuildBefore(subjNexus, subjGateway),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "Forge",
					Match:        MatchBuild(subjForge),
					TargetSecond: 88,
					Tolerance:    Sym(6),
				},
				{
					Key:          "Nexus",
					Match:        MatchBuild(subjNexus),
					TargetSecond: 130,
					Tolerance:    Sym(8),
				},
			},
			SummaryPlayer: &Pill{Label: "Forge Expand", IconKey: "forge"},
			GamesList:     &Pill{Label: "Forge Expand", IconKey: "forge"},
		},
		{
			Name:        "2 Gate",
			PatternName: "Build Order: 2 Gate",
			FeatureKey:  "bo_2_gate",
			Race:        RaceProtoss,
			Kind:        KindInitialBuildOrder,
			Rule: All(
				// "2 Gateways are built before Nexus or Forge"
				NthBuildBeforeAll(subjGateway, 2, []string{subjNexus, subjForge}),
				// "both gateways built before 3 mins"
				CountBuildsBefore(subjGateway, 2, 180),
			),
			RuleDeadline: 180,
			Expert: []ExpertEvent{
				{
					Key:          "1st Gateway",
					Match:        MatchBuild(subjGateway),
					TargetSecond: 70, // 1m10
					Tolerance:    defaultTol,
				},
				{
					Key:          "2nd Gateway",
					Match:        MatchNthBuild(subjGateway, 2),
					TargetSecond: 86, // 1m26
					Tolerance:    defaultTol,
				},
				{
					// First Zealot can be queued the moment the 1st Gateway
					// completes: 70 + Gateway build time = 108.
					Key:          "First Zealot",
					Match:        MatchFirstProduce(subjZealot),
					TargetSecond: secAfter(70, models.BuildTimeGateway),
					Tolerance:    Sym(3),
				},
			},
			SummaryPlayer: &Pill{Label: "2 Gate", IconKey: "gateway"},
			GamesList:     &Pill{Label: "2 Gate", IconKey: "gateway"},
		},

		// -------------------------------------------------------------------
		// KindMarker entries. These may coexist with each other and with a
		// KindInitialBuildOrder. Bool-only via Rule; PatternName kept equal
		// to the old imperative detector's Name() so DB rows + frontend
		// checks stay compatible.
		// -------------------------------------------------------------------

		{
			Name:         "Quick factory",
			PatternName:  "Quick factory",
			FeatureKey:   "quick_factory",
			Kind:         KindMarker,
			Race:         RaceTerran,
			Rule:         FirstBuildBefore(subjFactory, 4*60),
			RuleDeadline: 4 * 60,
			// Inline pill: frontend renders "Quick <factory-icon>" — the unit icon
			// embeds as a sub-element rather than a plain text {subject}.
			SummaryPlayer: &Pill{Label: "Quick", IconKey: "factory", Style: PillStyleInline},
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
			GamesList:     &Pill{Label: "Carrier", IconKey: "carrier"},
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
			GamesList:     &Pill{Label: "Battlecruiser", IconKey: "battlecruiser"},
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
			Custom:        func() CustomEvaluator { return &worldstateFirstEventEvaluator{eventType: "recall"} },
			RuleDeadline:  endOfReplaySentinel,
			SummaryPlayer: &Pill{Label: "Made recalls at min {minute}"},
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
