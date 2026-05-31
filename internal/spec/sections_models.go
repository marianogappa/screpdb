package spec

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/icza/screp/rep/repcore"
	"github.com/marianogappa/screpdb/internal/models"
)

// fnum formats a float without trailing zeros (e.g. 25.2, 50, 167.58).
func fnum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// sortedJoin returns the values sorted and comma-joined, for compact list cells.
func sortedJoin(values []string) string {
	cp := append([]string(nil), values...)
	sort.Strings(cp)
	return strings.Join(cp, ", ")
}

// knownNameSet returns the set of canonical unit + building names. Used by the
// model sections' Verify closures to assert that building/unit references point
// at real names.
func knownNameSet() map[string]bool {
	set := map[string]bool{}
	for _, n := range models.Units {
		set[n] = true
	}
	for _, n := range models.Buildings {
		set[n] = true
	}
	return set
}

func validRace(r string) bool {
	return r == models.RaceTerran || r == models.RaceZerg || r == models.RaceProtoss
}

func init() {
	registerUnitNames()
	registerOrderAttribution()
	registerBuildTimes()
	registerTechResearch()
	registerUpgrades()
	registerWorkerFlying()
	registerUnitGeometry()
	registerBuildingGeometry()
	registerActionTypes()
	registerReplayEnums()
}

func registerUnitNames() {
	Register(Section{
		Key:   "01-unit-names",
		Title: "Unit & building names",
		Intro: "The canonical names screpdb uses for every unit and building, grouped " +
			"by race. Every name shown in the UI is one of these strings.",
		Columns: []string{"Race", "Category", "Names"},
		Rows: func() [][]string {
			return [][]string{
				{models.RaceTerran, "Units", sortedJoin(models.TerranUnits)},
				{models.RaceTerran, "Buildings", sortedJoin(models.TerranBuildings)},
				{models.RaceZerg, "Units", sortedJoin(models.ZergUnits)},
				{models.RaceZerg, "Buildings", sortedJoin(models.ZergBuildings)},
				{models.RaceProtoss, "Units", sortedJoin(models.ProtossUnits)},
				{models.RaceProtoss, "Buildings", sortedJoin(models.ProtossBuildings)},
			}
		},
		Verify: func() error {
			if got, want := len(models.TerranUnits)+len(models.ZergUnits)+len(models.ProtossUnits), len(models.Units); got != want {
				return fmt.Errorf("per-race unit counts sum to %d, want len(Units)=%d", got, want)
			}
			if got, want := len(models.TerranBuildings)+len(models.ZergBuildings)+len(models.ProtossBuildings), len(models.Buildings); got != want {
				return fmt.Errorf("per-race building counts sum to %d, want len(Buildings)=%d", got, want)
			}
			return nil
		},
	})
}

func registerOrderAttribution() {
	// siegeModeAlias is the one attributed unit name that is an internal
	// alternate form (the sieged tank) rather than a member of the Units list.
	siegeModeAlias := map[string]bool{models.GeneralUnitTerranSiegeTankSiegeMode: true}

	Register(Section{
		Key:   "02-order-attribution",
		Title: "Order → unit attribution",
		Intro: "Orders/actions that belong to exactly one unit type (e.g. " +
			"`CastPsionicStorm` can only be a High Templar). screpdb uses this to " +
			"attribute a command to the unit that issued it. Generic orders (Move, " +
			"Attack, Hold) belong to many units and are omitted. `OrderName` and " +
			"`ActionType` are separate namespaces.",
		Columns: []string{"Key", "Namespace", "Issued by", "Race"},
		Rows: func() [][]string {
			var rows [][]string
			for k, v := range models.UnitOrderToUnit {
				rows = append(rows, []string{k, "OrderName", v.Unit, v.Race})
			}
			for k, v := range models.ActionTypeToUnit {
				rows = append(rows, []string{k, "ActionType", v.Unit, v.Race})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i][0] != rows[j][0] {
					return rows[i][0] < rows[j][0]
				}
				return rows[i][1] < rows[j][1]
			})
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			check := func(ns string, m map[string]models.UnitOrigin) error {
				for k, v := range m {
					if v.Unit == "" {
						return fmt.Errorf("%s %q has empty unit", ns, k)
					}
					if !validRace(v.Race) {
						return fmt.Errorf("%s %q has invalid race %q", ns, k, v.Race)
					}
					if !known[v.Unit] && !siegeModeAlias[v.Unit] {
						return fmt.Errorf("%s %q maps to unknown unit %q", ns, k, v.Unit)
					}
				}
				return nil
			}
			if err := check("OrderName", models.UnitOrderToUnit); err != nil {
				return err
			}
			return check("ActionType", models.ActionTypeToUnit)
		},
	})
}

func registerBuildTimes() {
	Register(Section{
		Key:   "03-build-times",
		Title: "Build times",
		Intro: "Build time in seconds at \"Fastest\" speed (every competitive replay " +
			"uses it). Single source of truth for all timing logic — detection, expert " +
			"timings, and the economy table all read these values. Zerglings are timed " +
			"per pair (one egg makes two).",
		Columns: []string{"Unit / Building", "Build time (s)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range models.AllBuildTimes() {
				rows = append(rows, []string{e.Name, fnum(e.Seconds)})
			}
			return rows
		},
		Verify: func() error {
			for _, e := range models.AllBuildTimes() {
				if e.Seconds <= 0 {
					return fmt.Errorf("build time for %q is %v, want > 0", e.Name, e.Seconds)
				}
			}
			// Anchor the two values reconciled in issue #138.
			if models.BuildTimeZealot != 25.2 {
				return fmt.Errorf("BuildTimeZealot = %v, want 25.2", models.BuildTimeZealot)
			}
			if models.BuildTimeZergling != 18 {
				return fmt.Errorf("BuildTimeZergling = %v, want 18", models.BuildTimeZergling)
			}
			return nil
		},
	})
}

func registerTechResearch() {
	Register(Section{
		Key:   "04-tech-research",
		Title: "Tech research",
		Intro: "One-shot researches that unlock an ability or morph (Stim Packs, " +
			"Lurker Aspect, Psionic Storm, …): where each is researched, its cost, and " +
			"its duration at \"Fastest\" speed. screpdb uses the duration to time when " +
			"the ability becomes available.",
		Columns: []string{"Tech", "Race", "Researched at", "Minerals", "Gas", "Duration (s)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range models.AllTechMeta() {
				rows = append(rows, []string{
					e.Name, e.Meta.Race, e.Meta.BuildingSubject,
					strconv.Itoa(e.Meta.Minerals), strconv.Itoa(e.Meta.Gas), fnum(e.Meta.DurationS),
				})
			}
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			for _, e := range models.AllTechMeta() {
				if !validRace(e.Meta.Race) {
					return fmt.Errorf("tech %q has invalid race %q", e.Name, e.Meta.Race)
				}
				if !known[e.Meta.BuildingSubject] {
					return fmt.Errorf("tech %q researched at unknown building %q", e.Name, e.Meta.BuildingSubject)
				}
				if e.Meta.DurationS <= 0 {
					return fmt.Errorf("tech %q has non-positive duration %v", e.Name, e.Meta.DurationS)
				}
			}
			return nil
		},
	})
}

func registerUpgrades() {
	lvl := func(m models.UpgradeLevelMeta) string {
		return fmt.Sprintf("%d / %d / %s", m.Minerals, m.Gas, fnum(m.DurationS))
	}
	Register(Section{
		Key:   "05-upgrades",
		Title: "Upgrades",
		Intro: "Passive upgrades: where each is researched, its max level (1 one-shot, " +
			"3 tiered) and each level's cost. Level cells read `minerals / gas / " +
			"seconds`; unused levels show an em dash. Used to time when an upgrade " +
			"completes.",
		Columns: []string{"Upgrade", "Race", "Researched at", "Max level", "L1 (m/g/s)", "L2 (m/g/s)", "L3 (m/g/s)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, e := range models.AllUpgradeMeta() {
				row := []string{e.Name, e.Meta.Race, e.Meta.BuildingSubject, strconv.Itoa(e.Meta.MaxLevel)}
				for i := 0; i < 3; i++ {
					if i < e.Meta.MaxLevel {
						row = append(row, lvl(e.Meta.Levels[i]))
					} else {
						row = append(row, "")
					}
				}
				rows = append(rows, row)
			}
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			for _, e := range models.AllUpgradeMeta() {
				if e.Meta.MaxLevel != 1 && e.Meta.MaxLevel != 3 {
					return fmt.Errorf("upgrade %q has MaxLevel %d, want 1 or 3", e.Name, e.Meta.MaxLevel)
				}
				if !validRace(e.Meta.Race) {
					return fmt.Errorf("upgrade %q has invalid race %q", e.Name, e.Meta.Race)
				}
				if !known[e.Meta.BuildingSubject] {
					return fmt.Errorf("upgrade %q researched at unknown building %q", e.Name, e.Meta.BuildingSubject)
				}
				if e.Meta.Levels[0].DurationS <= 0 {
					return fmt.Errorf("upgrade %q L1 has non-positive duration", e.Name)
				}
			}
			return nil
		},
	})
}

func registerWorkerFlying() {
	Register(Section{
		Key:   "06-worker-flying",
		Title: "Worker & flying units",
		Intro: "Two unit sets the detectors special-case, both excluded from " +
			"drop-composition estimates: workers, and flying units (can't be carried " +
			"in a transport).",
		Columns: []string{"Category", "Units"},
		Rows: func() [][]string {
			return [][]string{
				{"Workers", strings.Join(models.WorkerUnitNames(), ", ")},
				{"Flying (non-transportable)", strings.Join(models.FlyingUnitNames(), ", ")},
			}
		},
		Verify: func() error {
			known := knownNameSet()
			if len(models.WorkerUnitNames()) == 0 || len(models.FlyingUnitNames()) == 0 {
				return fmt.Errorf("worker/flying set is empty")
			}
			for _, n := range models.WorkerUnitNames() {
				if !known[n] {
					return fmt.Errorf("worker %q is not a known unit name", n)
				}
			}
			// Flying set may include transient forms (e.g. "Mutalisk Cocoon")
			// that aren't in the playable Units list, so only require non-empty.
			for _, n := range models.FlyingUnitNames() {
				if n == "" {
					return fmt.Errorf("flying unit set contains an empty name")
				}
			}
			return nil
		},
	})
}

func registerUnitGeometry() {
	Register(Section{
		Key:     "07-unit-geometry",
		Title:   "Unit geometry",
		Intro:   "Pixel dimensions screpdb uses to draw unit overlays on the map.",
		Columns: []string{"Unit", "Width (px)", "Height (px)"},
		Rows: func() [][]string {
			var rows [][]string
			for _, u := range models.AllUnitGeometry() {
				rows = append(rows, []string{u.Name, strconv.Itoa(u.WidthPixels), strconv.Itoa(u.HeightPixels)})
			}
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			for _, u := range models.AllUnitGeometry() {
				if u.WidthPixels <= 0 || u.HeightPixels <= 0 {
					return fmt.Errorf("unit %q has non-positive dimensions", u.Name)
				}
				if !known[u.Name] {
					return fmt.Errorf("unit geometry %q is not a known unit name", u.Name)
				}
			}
			return nil
		},
	})
}

func registerBuildingGeometry() {
	Register(Section{
		Key:   "08-building-geometry",
		Title: "Building geometry",
		Intro: "Pixel dimensions for building overlays. `Box` is the placement " +
			"footprint, `Real` is the visible sprite, and the gaps are the insets from " +
			"box to sprite on each side.",
		Columns: []string{"Building", "Box W", "Box H", "Real W", "Real H", "Gap T", "Gap L", "Gap R", "Gap B"},
		Rows: func() [][]string {
			var rows [][]string
			for _, b := range models.AllBuildingGeometry() {
				rows = append(rows, []string{
					b.Name,
					strconv.Itoa(b.BoxWidthPixels), strconv.Itoa(b.BoxHeightPixels),
					strconv.Itoa(b.RealWidthPixels), strconv.Itoa(b.RealHeightPixels),
					strconv.Itoa(b.GapTopPixels), strconv.Itoa(b.GapLeftPixels),
					strconv.Itoa(b.GapRightPixels), strconv.Itoa(b.GapBottomPixels),
				})
			}
			return rows
		},
		Verify: func() error {
			known := knownNameSet()
			for _, b := range models.AllBuildingGeometry() {
				if b.BoxWidthPixels <= 0 || b.BoxHeightPixels <= 0 {
					return fmt.Errorf("building %q has non-positive box dimensions", b.Name)
				}
				if !known[b.Name] {
					return fmt.Errorf("building geometry %q is not a known building name", b.Name)
				}
			}
			return nil
		},
	})
}

func registerActionTypes() {
	Register(Section{
		Key:   "09-action-types",
		Title: "Higher-level action types",
		Intro: "screpdb collapses many raw commands into a few higher-level action " +
			"types (train, build, morph). These are the exact string values it stores " +
			"and matches on.",
		Columns: []string{"Action type", "Value"},
		Rows: func() [][]string {
			return [][]string{
				{"Train", models.ActionTypeTrain},
				{"Build", models.ActionTypeBuild},
				{"Unit Morph", models.ActionTypeUnitMorph},
			}
		},
		Verify: func() error {
			for _, v := range []string{models.ActionTypeTrain, models.ActionTypeBuild, models.ActionTypeUnitMorph} {
				if v == "" {
					return fmt.Errorf("empty action type constant")
				}
			}
			return nil
		},
	})
}

func registerReplayEnums() {
	speedNames := func() []string {
		var out []string
		for _, s := range repcore.Speeds {
			out = append(out, s.Name)
		}
		return out
	}
	colorNames := func() []string {
		seen := map[string]bool{}
		var out []string
		for _, c := range repcore.Colors {
			if c.Name == "" || seen[c.Name] {
				continue
			}
			seen[c.Name] = true
			out = append(out, c.Name)
		}
		return out
	}
	Register(Section{
		Key:   "10-replay-enums",
		Title: "Replay enums",
		Intro: "Fixed value sets screpdb reads from a parsed replay. Races are " +
			"screpdb's; speeds and colors come from the screp parser " +
			"(`github.com/icza/screp/rep/repcore`); matchups are derived from race " +
			"initials.",
		Columns: []string{"Enum", "Values", "Notes"},
		Rows: func() [][]string {
			return [][]string{
				{"Races", strings.Join([]string{models.RaceTerran, models.RaceProtoss, models.RaceZerg}, ", "), "Random resolves to the actual race at game start."},
				{"Game speeds", strings.Join(speedNames(), ", "), "Competitive replays are always played on Fastest."},
				{"Player colors", strings.Join(colorNames(), ", "), "The player's slot color as parsed from the replay."},
				{"1v1 matchups", "PvP, PvT, PvZ, TvT, TvZ, ZvZ", "Race initials sorted alphabetically; team/FFA games use the same scheme (e.g. PPvZZ)."},
				{"Start location (clock)", "1–12", "The o'clock position of the player's start location on the map."},
			}
		},
		Verify: func() error {
			has := func(list []string, want string) bool {
				for _, v := range list {
					if v == want {
						return true
					}
				}
				return false
			}
			if !has(speedNames(), "Fastest") {
				return fmt.Errorf("game speeds missing Fastest")
			}
			if !has(colorNames(), "Red") {
				return fmt.Errorf("player colors missing Red")
			}
			return nil
		},
	})
}
