package spec

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/utils"
)

// endOfReplayThreshold mirrors markers.endOfReplaySentinel (private): rule
// deadlines at or above this are "resolved only at end of replay".
const endOfReplayThreshold = 7200 // 2 in-game hours; well past any real opener

func tolerance(t markers.Tolerance) string {
	if t.EarlySeconds == t.LateSeconds {
		return fmt.Sprintf("±%d", t.EarlySeconds)
	}
	return fmt.Sprintf("−%d / +%d", t.EarlySeconds, t.LateSeconds)
}

func openers() []markers.Marker {
	var out []markers.Marker
	for _, m := range markers.Markers() {
		if m.Kind == markers.KindInitialBuildOrder {
			out = append(out, m)
		}
	}
	return out
}

func init() {
	registerBuildOrders()
	registerBuildOrderDeadlines()
	registerAbsenceThresholds()
	registerDetectionScalars()
	registerUnitEconomics()
	registerGatherRates()
	registerTechTreeProducers()
	registerTechTreePrereqs()
	registerFeaturingOrder()
	registerGameEventFeatures()
}

func registerBuildOrders() {
	Register(Section{
		Key:   "20-build-orders",
		Title: "Build orders & expert timings",
		Intro: "The openings screpdb recognizes and each milestone's \"progamer ideal\" " +
			"timing. The Build Orders tab marks a milestone on-time if it lands within " +
			"the tolerance window around its target. Targets are seconds from game " +
			"start (\"Fastest\" speed); tolerance is the accepted early/late deviation.",
		Columns: []string{"Build order", "Race", "Milestone", "Target (s)", "Tolerance (s)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, m := range openers() {
				for _, e := range m.Expert {
					rows = append(rows, []string{
						m.Name, string(m.Race), e.Key,
						strconv.Itoa(e.TargetSecond), tolerance(e.Tolerance),
					})
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i][0] != rows[j][0] {
					return rows[i][0] < rows[j][0]
				}
				ti, _ := strconv.Atoi(rows[i][3])
				tj, _ := strconv.Atoi(rows[j][3])
				if ti != tj {
					return ti < tj
				}
				return rows[i][2] < rows[j][2]
			})
			return rows
		},
		Verify: func() error {
			ops := openers()
			if len(ops) == 0 {
				return fmt.Errorf("no initial-build-order markers registered")
			}
			known := knownNameSet()
			for _, m := range ops {
				for _, e := range m.Expert {
					if e.TargetSecond <= 0 {
						return fmt.Errorf("%s milestone %q has non-positive target", m.Name, e.Key)
					}
					if e.Tolerance.EarlySeconds < 0 || e.Tolerance.LateSeconds < 0 {
						return fmt.Errorf("%s milestone %q has negative tolerance", m.Name, e.Key)
					}
					if s := e.Match.Subject; s != "" && !known[s] {
						return fmt.Errorf("%s milestone %q matches unknown subject %q", m.Name, e.Key, s)
					}
				}
			}
			return nil
		},
	})
}

func registerBuildOrderDeadlines() {
	Register(Section{
		Key:   "21-build-order-deadlines",
		Title: "Build-order rule deadlines",
		Intro: "Each opener's detector commits its decision once the replay passes " +
			"this second — the last moment the rule could still flip. \"End of replay\" " +
			"means it's decided only when the game ends.",
		Columns: []string{"Build order", "Race", "Rule deadline (s)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, m := range openers() {
				dl := strconv.Itoa(m.RuleDeadline)
				if m.RuleDeadline >= endOfReplayThreshold {
					dl = "end of replay"
				}
				rows = append(rows, []string{m.Name, string(m.Race), dl})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
			return rows
		},
		Verify: func() error {
			for _, m := range openers() {
				if m.RuleDeadline <= 0 {
					return fmt.Errorf("opener %q has non-positive rule deadline", m.Name)
				}
			}
			return nil
		},
	})
}

func registerAbsenceThresholds() {
	Register(Section{
		Key:   "22-absence-thresholds",
		Title: "Absence-marker game-length thresholds",
		Intro: "\"Never X\" markers (e.g. *Never upgraded*) only fire on games long " +
			"enough for the absence to mean something. The minimum length is " +
			"matchup-specific — the 5th-percentile first-occurrence time across a large " +
			"progamer 1v1 corpus. Outer race is the player's, inner is the opponent's.",
		Columns: []string{"Marker", "Own race", "Opp race", "Min game length (s)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, m := range markers.Markers() {
				if m.MinReplaySecondsByMatchup == nil {
					continue
				}
				for own, byOpp := range m.MinReplaySecondsByMatchup {
					for opp, secs := range byOpp {
						rows = append(rows, []string{m.Name, string(own), string(opp), strconv.Itoa(secs)})
					}
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i][0] != rows[j][0] {
					return rows[i][0] < rows[j][0]
				}
				if rows[i][1] != rows[j][1] {
					return rows[i][1] < rows[j][1]
				}
				return rows[i][2] < rows[j][2]
			})
			return rows
		},
		Verify: func() error {
			count := 0
			for _, m := range markers.Markers() {
				for own, byOpp := range m.MinReplaySecondsByMatchup {
					for opp, secs := range byOpp {
						count++
						if secs <= 0 {
							return fmt.Errorf("%s [%s vs %s] threshold %d <= 0", m.Name, own, opp, secs)
						}
					}
				}
			}
			if count == 0 {
				return fmt.Errorf("no per-matchup absence thresholds found")
			}
			return nil
		},
	})
}

func registerDetectionScalars() {
	Register(Section{
		Key:   "23-detection-scalars",
		Title: "Detection scalars & versioning",
		Intro: "Standalone constants the detectors depend on — dedup windows, " +
			"muta/turret burst thresholds, cliff-drop corner boxes, the viewport " +
			"window, and the algorithm version (bump it to force re-detection).",
		Columns: []string{"Constant", "Value", "Meaning"},
		Rows: func() [][]string {
			return [][]string{
				{"Algorithm version", strconv.Itoa(core.AlgorithmVersion), "Detection algorithm revision; incremented to trigger re-detection."},
				{"Build dedup gap (s)", strconv.Itoa(markers.BuildDedupGapSeconds), "Repeat Build orders of the same building closer than this are one event."},
				{"Build dedup max second (s)", strconv.Itoa(markers.BuildDedupMaxSecond), "Past this second, dedup stops and every Build is observed as-is."},
				{"Mutalisk burst window (s)", strconv.Itoa(markers.MutaBurstWindowSec), "Window within which the Mutalisk morphs must cluster."},
				{"Mutalisk burst min count", strconv.Itoa(markers.MutaBurstMinCount), "Minimum Mutalisks in the window to count as a burst."},
				{"Turret burst window (s)", strconv.Itoa(markers.TurretBurstWindowSec), "Window within which the Missile Turrets must cluster."},
				{"Turret burst min count", strconv.Itoa(markers.TurretBurstMinCount), "Minimum Missile Turrets in the window to count as a burst."},
				{"Cliff-drop corner width (px)", strconv.Itoa(utils.CliffDropCornerWidthPx), "Width of the corner box a drop must land in to count as a cliff drop."},
				{"Cliff-drop corner height (px)", strconv.Itoa(utils.CliffDropCornerHeightPx), "Height of that corner box."},
				{"Viewport window start (s)", strconv.Itoa(models.ViewportMultitaskingWindowStartSecond), "Second from which viewport-multitasking is measured."},
				{"Viewport width (px)", strconv.Itoa(models.ViewportWidthPixels), "Width of the screen viewport in map pixels."},
				{"Viewport height (px)", strconv.Itoa(models.ViewportHeightPixels), "Height of the screen viewport in map pixels."},
			}
		},
		Verify: func() error {
			if core.AlgorithmVersion <= 0 {
				return fmt.Errorf("AlgorithmVersion = %d, want > 0", core.AlgorithmVersion)
			}
			if markers.BuildDedupGapSeconds <= 0 || markers.BuildDedupMaxSecond <= 0 {
				return fmt.Errorf("dedup constants must be positive")
			}
			if markers.MutaBurstMinCount <= 0 || markers.TurretBurstMinCount <= 0 {
				return fmt.Errorf("burst counts must be positive")
			}
			if utils.CliffDropCornerWidthPx <= 0 || utils.CliffDropCornerHeightPx <= 0 {
				return fmt.Errorf("cliff-drop corner box must be positive")
			}
			if models.ViewportWidthPixels <= 0 || models.ViewportHeightPixels <= 0 {
				return fmt.Errorf("viewport dimensions must be positive")
			}
			return nil
		},
	})
}

func registerUnitEconomics() {
	Register(Section{
		Key:   "24-unit-economics",
		Title: "Early-game unit economics",
		Intro: "What producing each early-game unit/building costs and does to supply. " +
			"Supply Δ is the cap increase from supply structures (Pylon/Depot/Overlord " +
			"= +8); supply cost is what a unit consumes. Build times match the Build " +
			"times section.",
		Columns: []string{"Subject", "Minerals", "Gas", "Build time (s)", "Supply Δ", "Supply cost"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range cmdenrich.AllEcon() {
				rows = append(rows, []string{
					e.Subject, strconv.Itoa(e.Econ.Minerals), strconv.Itoa(e.Econ.Gas),
					fnum(e.Econ.BuildTimeS), strconv.Itoa(e.Econ.SupplyDelta), strconv.Itoa(e.Econ.SupplyCost),
				})
			}
			return rows
		},
		Verify: func() error {
			for _, e := range cmdenrich.AllEcon() {
				bt, ok := models.BuildTimeOf(e.Subject)
				if !ok {
					return fmt.Errorf("econ subject %q has no canonical build time", e.Subject)
				}
				if bt != e.Econ.BuildTimeS {
					return fmt.Errorf("econ build time for %q is %v, canonical is %v", e.Subject, e.Econ.BuildTimeS, bt)
				}
				if e.Econ.Minerals <= 0 {
					return fmt.Errorf("econ subject %q has non-positive minerals", e.Subject)
				}
			}
			return nil
		},
	})
}

func registerGatherRates() {
	Register(Section{
		Key:   "25-gather-rates",
		Title: "Worker gather rates",
		Intro: "Mineral income per worker per minute at a near base, used in " +
			"early-game economy math.",
		Columns: []string{"Worker", "Minerals / min"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range cmdenrich.AllGatherRates() {
				rows = append(rows, []string{e.Worker, fnum(e.MineralsPerMin)})
			}
			return rows
		},
		Verify: func() error {
			rates := cmdenrich.AllGatherRates()
			if len(rates) == 0 {
				return fmt.Errorf("no gather rates")
			}
			known := knownNameSet()
			for _, e := range rates {
				if e.MineralsPerMin <= 0 {
					return fmt.Errorf("gather rate for %q is non-positive", e.Worker)
				}
				if !known[e.Worker] {
					return fmt.Errorf("gather-rate worker %q is not a known unit", e.Worker)
				}
			}
			return nil
		},
	})
}

func registerTechTreeProducers() {
	Register(Section{
		Key:   "26-tech-tree-producers",
		Title: "Tech tree: producers",
		Intro: "Which building produces each early-game unit. The engine won't run a " +
			"Train order without its producer, so a surviving Train is strong evidence " +
			"the producer existed — the spam filter relies on this.",
		Columns: []string{"Unit", "Produced by"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range cmdenrich.AllProducers() {
				rows = append(rows, []string{e.Unit, e.Producer})
			}
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			for _, e := range cmdenrich.AllProducers() {
				if !known[e.Unit] {
					return fmt.Errorf("producer entry unit %q unknown", e.Unit)
				}
				if !known[e.Producer] {
					return fmt.Errorf("producer %q unknown for unit %q", e.Producer, e.Unit)
				}
			}
			return nil
		},
	})
}

func registerTechTreePrereqs() {
	Register(Section{
		Key:   "27-tech-tree-prereqs",
		Title: "Tech tree: prerequisites",
		Intro: "Buildings that must already exist before another can be placed (beyond " +
			"the producer) — e.g. a Photon Cannon needs a Pylon and a Forge.",
		Columns: []string{"Building", "Requires"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range cmdenrich.AllPrereqs() {
				rows = append(rows, []string{e.Building, strings.Join(e.Prereqs, ", ")})
			}
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			for _, e := range cmdenrich.AllPrereqs() {
				if !known[e.Building] {
					return fmt.Errorf("prereq entry building %q unknown", e.Building)
				}
				if len(e.Prereqs) == 0 {
					return fmt.Errorf("building %q has empty prerequisites", e.Building)
				}
				for _, p := range e.Prereqs {
					if !known[p] {
						return fmt.Errorf("prerequisite %q unknown for %q", p, e.Building)
					}
				}
			}
			return nil
		},
	})
}

func registerFeaturingOrder() {
	Register(Section{
		Key:   "28-featuring-order",
		Title: "Featuring strip order",
		Intro: "The fixed left-to-right order of chips in the games-list \"Featuring\" " +
			"strip — a mix of marker keys and game-event keys. Every key resolves to a " +
			"marker or a game-event feature (next section), enforced by a test.",
		Columns: []string{"#", "Feature key"},
		Rows: func() [][]string {
			var rows [][]string
			for i, key := range dashboard.FeaturingOrder() {
				rows = append(rows, []string{strconv.Itoa(i + 1), key})
			}
			return rows
		},
		Verify: func() error {
			order := dashboard.FeaturingOrder()
			if len(order) == 0 {
				return fmt.Errorf("featuring order is empty")
			}
			seen := map[string]bool{}
			for _, k := range order {
				if k == "" {
					return fmt.Errorf("empty featuring key")
				}
				if seen[k] {
					return fmt.Errorf("duplicate featuring key %q", k)
				}
				seen[k] = true
			}
			return nil
		},
	})
}

func registerGameEventFeatures() {
	Register(Section{
		Key:   "29-game-event-features",
		Title: "Game-event featuring chips",
		Intro: "Featuring chips for narrative game events that aren't markers (cannon " +
			"rush, drop, mind control, …): each with a label and one or more unit icons.",
		Columns: []string{"Key", "Label", "Icons"},
		Rows: func() [][]string {
			var rows [][]string
			for _, f := range dashboard.AllGameEventFeatures() {
				rows = append(rows, []string{f.Key, f.Label, strings.Join(f.IconKeys, ", ")})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
			return rows
		},
		Verify: func() error {
			feats := dashboard.AllGameEventFeatures()
			if len(feats) == 0 {
				return fmt.Errorf("no game-event features")
			}
			for _, f := range feats {
				if f.Key == "" || f.Label == "" {
					return fmt.Errorf("game-event feature with empty key/label")
				}
			}
			return nil
		},
	})
}
