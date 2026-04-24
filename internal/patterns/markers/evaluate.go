package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

// Matches reports whether a time-ordered slice of facts satisfies this BO's
// broad definition. Thin adapter over the streaming Predicate: replays the
// facts through a fresh state and asks Finalize for the verdict.
//
// Production detectors drive PredicateState directly from command flow; they
// don't go through Matches. This adapter exists for tests, fuzz, and
// one-shot callers (dashboard's broad re-checks) that already hold a slice.
func (bo Marker) Matches(facts []cmdenrich.EnrichedCommand) bool {
	return bo.Rule.Eval(facts)
}

// ExpertResolution is a per-event comparison between the player's actual
// timing and the expert template.
type ExpertResolution struct {
	Key          string
	Subject      string // canonical unit/building name (for icon lookup)
	TargetSecond int
	Tolerance    Tolerance
	ActualSecond int // valid only when Found == true
	Found        bool
	// DeltaSeconds = ActualSecond - TargetSecond. Positive = late, negative = early.
	// Only meaningful when Found is true.
	DeltaSeconds int
	// WithinTolerance reports whether Delta fits inside the declared tolerance.
	WithinTolerance bool
}

// ResolveExpert walks every expert event in the BO template and attempts to
// locate its actual occurrence inside the supplied facts. Events not present
// come back with Found == false; the UI can render them as "missing".
func (bo Marker) ResolveExpert(facts []cmdenrich.EnrichedCommand) []ExpertResolution {
	out := make([]ExpertResolution, 0, len(bo.Expert))
	for _, ev := range bo.Expert {
		res := ExpertResolution{
			Key:          ev.Key,
			Subject:      ev.Match.Subject,
			TargetSecond: ev.TargetSecond,
			Tolerance:    ev.Tolerance,
		}
		if sec, ok := ev.Match.Resolve(facts); ok {
			res.Found = true
			res.ActualSecond = sec
			res.DeltaSeconds = sec - ev.TargetSecond
			res.WithinTolerance = (res.DeltaSeconds >= -ev.Tolerance.EarlySeconds) &&
				(res.DeltaSeconds <= ev.Tolerance.LateSeconds)
		}
		out = append(out, res)
	}
	return out
}
