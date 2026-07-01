package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

// zergHatchHydraEvaluator classifies a hydralisk-based Zerg army by the number
// of bases standing at the economy→army transition, rather than a fixed clock.
//
// The transition is the moment the player STARTS producing Hydralisks (the 1st
// Hydralisk morph, when Drone production is cut) — the user's definition. N =
// Build(Hatchery) count observed strictly before that first morph, plus the
// starting Hatchery. Later Hatcheries / Drone rounds after the transition don't
// change the BO name (issue #227). A total of 6+ Hydralisks confirms it is a
// real hydra build, not a stray Hydralisk.
type zergHatchHydraEvaluator struct {
	targetBases   int
	hatchBuilds   int
	muta          int
	hydra         int
	committed     bool
	committedSec  int
	basesAtCommit int
}

func newZergHatchHydra(targetBases int) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &zergHatchHydraEvaluator{targetBases: targetBases}
	}
}

func (e *zergHatchHydraEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch {
	case f.Kind == cmdenrich.KindMakeBuilding && f.Subject == subjHatchery:
		e.hatchBuilds++
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjMutalisk:
		e.muta += factUnitCount(f)
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjHydralisk:
		e.hydra += factUnitCount(f)
		if !e.committed {
			e.committed = true
			e.committedSec = f.Second
			e.basesAtCommit = e.hatchBuilds + 1 // + the starting Hatchery
		}
	}
}

func (e *zergHatchHydraEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	// Not a real hydra build unless it produced a mass of Hydralisks.
	if !e.committed || e.hydra < 6 {
		return CustomResult{}
	}
	// Hydra-DOMINANT, not a muta build. A Spire for Scourge (Overlord control)
	// or a handful of Mutas is fine — the point is Hydralisks are the army — so
	// gate on Hydra outnumbering Muta, not Den-vs-Spire ordering or a low muta cap.
	if e.hydra <= e.muta {
		return CustomResult{}
	}
	if e.basesAtCommit != e.targetBases {
		return CustomResult{}
	}
	return CustomResult{Matched: true, DetectedAtSecond: e.committedSec}
}
