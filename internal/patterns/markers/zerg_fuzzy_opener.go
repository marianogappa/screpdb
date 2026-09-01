package markers

import (
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// zergOpenerFuzzyEvaluator labels a Zerg pool/hatch opener whose exact supply
// rung is indeterminate. A larva-morph command records the selection size, not
// how many of the selected units were actually larvae, so a multi-unit-selection
// Drone morph before the Pool/Hatchery makes the drones-before-building count
// ambiguous (min = one Drone per morph, max = the capped selection size). When
// the two disagree no exact rung fires (each requires min==max==want), and this
// evaluator emits a fuzzy "~N Pool/Overpool/Hatch" label anchored at the floor.
//
// It is the opener of last resort for clean pool/hatch openings: it fires only
// when the count is ambiguous, so it never competes with an exact rung.
type zergOpenerFuzzyEvaluator struct {
	drones              []produceObservation
	poolSec, hatchSec   int
	evoSec, overlordSec int
}

func newZergOpenerFuzzyEvaluator() *zergOpenerFuzzyEvaluator {
	return &zergOpenerFuzzyEvaluator{poolSec: -1, hatchSec: -1, evoSec: -1, overlordSec: -1}
}

func (e *zergOpenerFuzzyEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	switch f.Kind {
	case cmdenrich.KindMakeUnit:
		switch f.Subject {
		case subjDrone:
			e.drones = append(e.drones, produceObservation{second: f.Second, count: factUnitCount(f)})
		case subjOverlord:
			if e.overlordSec < 0 {
				e.overlordSec = f.Second
			}
		}
	case cmdenrich.KindMakeBuilding:
		switch f.Subject {
		case subjSpawningPool:
			if e.poolSec < 0 {
				e.poolSec = f.Second
			}
		case subjHatchery:
			if e.hatchSec < 0 {
				e.hatchSec = f.Second
			}
		case subjEvolutionChamber:
			if e.evoSec < 0 {
				e.evoSec = f.Second
			}
		}
	}
}

func (e *zergOpenerFuzzyEvaluator) Finalize(_ CustomEvalContext) CustomResult {
	poolFirst := e.poolSec >= 0 &&
		(e.hatchSec < 0 || e.poolSec < e.hatchSec) &&
		(e.evoSec < 0 || e.poolSec < e.evoSec)
	hatchFirst := e.hatchSec >= 0 &&
		(e.poolSec < 0 || e.hatchSec < e.poolSec) &&
		(e.evoSec < 0 || e.hatchSec < e.evoSec)

	var defSec int
	var kind string
	switch {
	case poolFirst:
		defSec = e.poolSec
		if e.overlordSec >= 0 && e.overlordSec < e.poolSec {
			kind = "Overpool"
		} else {
			kind = "Pool"
		}
	case hatchFirst:
		defSec = e.hatchSec
		kind = "Hatch"
	default:
		return CustomResult{} // not a clean pool/hatch opener
	}

	minDrones, maxDrones := 0, 0
	for _, d := range e.drones {
		if d.second < defSec {
			minDrones++
			maxDrones += d.count
		}
	}
	if minDrones == maxDrones {
		return CustomResult{} // unambiguous — an exact rung owns this opener
	}

	label := fmt.Sprintf("~%d %s", 4+minDrones, kind)
	payload, _ := json.Marshal(struct {
		Label string `json:"label"`
	}{Label: label})
	return CustomResult{Matched: true, DetectedAtSecond: defSec, Payload: payload}
}

// fuzzyZergExpertByLabel carries the golden targets for the fuzzy-opener
// labels that clear the n >= 20 pro-corpus floor (see MEASUREMENT.md). The
// fuzzy marker's Expert list is empty — its rung is only known at detection
// time — so the Build Orders tab resolves the golden band from the persisted
// label through this table at render time, against the same raw-command
// timings it draws the actual ticks from. Each entry is the label's defining
// building only, mirroring the exact rungs' zergBOEventSchemas rows; labels
// without an entry render actual-only ticks.
var fuzzyZergExpertByLabel = map[string][]ExpertEvent{
	"~11 Hatch": {
		{Key: "Hatchery", Match: MatchBuild(subjHatchery), TargetSecond: 98, Tolerance: Sym(2)}, // n=446, p10/50/90 = 96/98/100
	},
	"~10 Hatch": {
		{Key: "Hatchery", Match: MatchBuild(subjHatchery), TargetSecond: 92, Tolerance: Sym(2)}, // n=349, p10/50/90 = 91/92/94
	},
	"~10 Overpool": {
		{Key: "Spawning Pool", Match: MatchBuild(subjSpawningPool), TargetSecond: 83, Tolerance: Asym(2, 3)}, // n=211, p10/50/90 = 81/83/86
	},
	"~5 Hatch": {
		{Key: "Hatchery", Match: MatchBuild(subjHatchery), TargetSecond: 54, Tolerance: Sym(2)}, // n=41, p10/50/90 = 53/54/55
	},
}

// FuzzyZergExpertEvents returns the golden targets for a resolved fuzzy
// Zerg opener label, or nil when the label is unmeasured.
func FuzzyZergExpertEvents(label string) []ExpertEvent {
	return fuzzyZergExpertByLabel[label]
}
