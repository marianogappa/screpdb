package markers

import (
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// zergHatchTechEvaluator names a Zerg tech-unit army (Hydralisk / Mutalisk /
// Lurker) "N Hatch <tech>" by the number of bases standing at the economy→army
// transition, rather than a fixed clock. It is a composition MARKER that layers
// on top of the supply opener (11/12 Hatch, Overpool, …) — the opening and the
// tech commitment are separate axes (issue #245).
//
// The transition is when the player STARTS producing the classifying tech unit
// (its 1st morph, as Drone production is cut). N = real town halls standing at
// that transition, plus (for Hydra/Lurker) a short grace window: an expansion
// placed right as tech production begins is part of the same commit, while later
// macro expansions during sustained production are not (issue #227).
//
// Muta uses NO grace: it's a timing attack — money and larvae are pre-saved so
// the Mutalisks pop from every existing Hatchery the instant the Spire finishes,
// so the base count is exactly what stands at that moment and a grace would
// wrongly count a Hatchery added seconds later.
//
// Base counting uses the worldstate town-hall build seconds, which come from the
// RAW command stream with re-placements collapsed (a cancelled / re-dropped
// Hatchery whose footprint overlaps another isn't a distinct base). See
// worldstate.TownHallBuildSeconds / unittags.TownHallBuildSeconds.
//
// Confirmation is per tech:
//   - Hydra: 6+ Hydralisks, Hydra-dominant over Muta (a Spire for Scourge / a
//     few Mutas is fine).
//   - Muta: 4+ Mutalisks, muta-first (Spire built before any Hydralisk Den) so a
//     lurker build that also makes Mutas isn't relabelled.
//   - Lurker: 2+ Lurkers, lurker-first (Hydralisk Den built before any Spire).
const hatchTechTransitionGraceSec = 30

type zergTech int

const (
	techHydra zergTech = iota
	techMuta
	techLurker
)

func (t zergTech) unit() string {
	switch t {
	case techMuta:
		return "Muta"
	case techLurker:
		return "Lurker"
	default:
		return "Hydra"
	}
}

// graceSec is the window after the transition in which a fresh expansion still
// counts toward N. Zero for Muta (a sharp timing attack — see above).
func (t zergTech) graceSec() int {
	if t == techMuta {
		return 0
	}
	return hatchTechTransitionGraceSec
}

type zergHatchTechEvaluator struct {
	tech zergTech

	muta, hydra, lurker int
	firstTech           int // second of the classifying tech unit's 1st morph; -1 until seen
	spireSec, denSec    int // first Spire / Hydralisk Den build second; -1 if none
}

func newZergHatchTech(tech zergTech) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &zergHatchTechEvaluator{tech: tech, firstTech: -1, spireSec: -1, denSec: -1}
	}
}

func (e *zergHatchTechEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch {
	case f.Kind == cmdenrich.KindMakeBuilding && f.Subject == subjSpire:
		if e.spireSec < 0 {
			e.spireSec = f.Second
		}
	case f.Kind == cmdenrich.KindMakeBuilding && f.Subject == subjHydraliskDen:
		if e.denSec < 0 {
			e.denSec = f.Second
		}
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjMutalisk:
		e.muta += factUnitCount(f)
		e.noteTransition(techMuta, f.Second)
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjHydralisk:
		e.hydra += factUnitCount(f)
		e.noteTransition(techHydra, f.Second)
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjLurker:
		e.lurker += factUnitCount(f)
		e.noteTransition(techLurker, f.Second)
	}
}

func (e *zergHatchTechEvaluator) noteTransition(tech zergTech, second int) {
	if tech == e.tech && e.firstTech < 0 {
		e.firstTech = second
	}
}

func (e *zergHatchTechEvaluator) Finalize(ctx CustomEvalContext) CustomResult {
	if e.firstTech < 0 || !e.confirmed() || ctx.WorldState == nil {
		return CustomResult{}
	}
	cutoff := e.firstTech + e.tech.graceSec()
	bases := 1 // the starting town hall (no Build command; not in the build evidence)
	for _, s := range ctx.WorldState.TownHallBuildSeconds(ctx.ReplayPlayerID) {
		if s < cutoff {
			bases++
		}
	}
	label := fmt.Sprintf("%d Hatch %s", bases, e.tech.unit())
	payload, _ := json.Marshal(struct {
		Label string `json:"label"`
	}{Label: label})
	return CustomResult{Matched: true, DetectedAtSecond: e.firstTech, Payload: payload}
}

// confirmed reports whether the army is a real instance of the classifying tech
// build (enough units + the muta/lurker ordering that keeps the two disjoint).
func (e *zergHatchTechEvaluator) confirmed() bool {
	switch e.tech {
	case techHydra:
		return e.hydra >= 6 && e.hydra > e.muta
	case techMuta:
		return e.muta >= 4 && e.spireBeforeDen()
	case techLurker:
		return e.lurker >= 2 && e.denBeforeSpire()
	}
	return false
}

func (e *zergHatchTechEvaluator) spireBeforeDen() bool {
	return e.spireSec >= 0 && (e.denSec < 0 || e.spireSec < e.denSec)
}

func (e *zergHatchTechEvaluator) denBeforeSpire() bool {
	return e.denSec >= 0 && (e.spireSec < 0 || e.denSec < e.spireSec)
}
