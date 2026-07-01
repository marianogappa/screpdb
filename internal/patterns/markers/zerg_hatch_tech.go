package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

// zergHatchHydraEvaluator classifies a hydralisk-based Zerg army by the number
// of bases standing at the economy→army transition, rather than a fixed clock.
//
// The transition is when the player STARTS producing Hydralisks (the 1st
// Hydralisk morph, as Drone production is cut). N = Hatcheries standing at that
// transition, counting Build(Hatchery) commands up to firstHydra + a short grace
// window (an expansion placed right as hydra production begins is part of the
// same commit) plus the starting Hatchery. The grace excludes later macro
// expansions during sustained hydra production (issue #227): 2jd's 3rd Hatchery
// lands 14s into hydra (counts → 3 Hatch), while SYC's 4th lands 52s in (doesn't
// → stays 4 Hatch). A total of 6+ Hydralisks, Hydra-dominant over Muta (a Spire
// for Scourge is fine), confirms a real hydra build.
const hydraTransitionGraceSec = 30

type zergHatchHydraEvaluator struct {
	targetBases  int
	hatchSecs    []int
	muta         int
	hydra        int
	firstHydra   int
	committed    bool
	committedSec int
}

func newZergHatchHydra(targetBases int) func() CustomEvaluator {
	return func() CustomEvaluator {
		return &zergHatchHydraEvaluator{targetBases: targetBases, firstHydra: -1}
	}
}

func (e *zergHatchHydraEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch {
	case f.Kind == cmdenrich.KindMakeBuilding && f.Subject == subjHatchery:
		e.hatchSecs = append(e.hatchSecs, f.Second)
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjMutalisk:
		e.muta += factUnitCount(f)
	case f.Kind == cmdenrich.KindMakeUnit && f.Subject == subjHydralisk:
		e.hydra += factUnitCount(f)
		if !e.committed {
			e.committed = true
			e.committedSec = f.Second
			e.firstHydra = f.Second
		}
	}
}

func (e *zergHatchHydraEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	// Not a real hydra build unless it produced a mass of Hydralisks, and it must
	// be hydra-dominant (Hydra > Muta — a Spire for Scourge / a few Mutas is fine).
	if !e.committed || e.hydra < 6 || e.hydra <= e.muta {
		return CustomResult{}
	}
	bases := 1 // the starting Hatchery
	for _, s := range e.hatchSecs {
		if s < e.firstHydra+hydraTransitionGraceSec {
			bases++
		}
	}
	if bases != e.targetBases {
		return CustomResult{}
	}
	return CustomResult{Matched: true, DetectedAtSecond: e.committedSec}
}
