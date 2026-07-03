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
