package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

// ApplyBuildDedup mirrors the streaming dedup applied by MarkerPlayerDetector
// when feeding facts into the Predicate state. It exists so non-streaming
// callers (dashboard's ResolveExpert path) see the same Build commands the
// detector did — without it, the UI re-resolves expert milestones against
// raw Build commands and surfaces double-clicked / spam-placed buildings as
// "first" / "second" of their kind, which is wrong.
//
// Behavior matches enqueueDedup + flushDedupBefore + flushAllPending:
//   - Only KindMakeBuilding facts whose Subject is "of interest" are subject
//     to dedup; everything else passes through untouched.
//   - Same-Subject Build facts arriving within BuildDedupGapSeconds collapse
//     to the latest occurrence (later wins, anti-spam).
//   - After BuildDedupMaxSecond (4 min), the gate is off; same-Subject facts
//     after that point all pass through.
//   - End-of-stream: any pending fact is emitted as a real observation.
//
// Input is assumed time-ordered (Second non-decreasing); the detector relies
// on this and so does this helper.
func ApplyBuildDedup(facts []cmdenrich.EnrichedCommand) []cmdenrich.EnrichedCommand {
	if len(facts) == 0 {
		return facts
	}
	out := make([]cmdenrich.EnrichedCommand, 0, len(facts))
	pending := map[string]cmdenrich.EnrichedCommand{}

	flushExpired := func(now int) {
		for subj, f := range pending {
			if now-f.Second >= BuildDedupGapSeconds {
				out = append(out, f)
				delete(pending, subj)
			}
		}
	}

	for _, f := range facts {
		flushExpired(f.Second)

		if f.Kind != cmdenrich.KindMakeBuilding || !IsSubjectOfInterest(f.Subject) {
			out = append(out, f)
			continue
		}

		if f.Second >= BuildDedupMaxSecond {
			if prior, ok := pending[f.Subject]; ok {
				out = append(out, prior)
				delete(pending, f.Subject)
			}
			out = append(out, f)
			continue
		}

		if prior, ok := pending[f.Subject]; ok {
			if f.Second-prior.Second < BuildDedupGapSeconds {
				pending[f.Subject] = f
				continue
			}
			out = append(out, prior)
		}
		pending[f.Subject] = f
	}
	for _, f := range pending {
		out = append(out, f)
	}
	sortByOriginalOrder(out)
	return out
}

// sortByOriginalOrder sorts in-place by Second (ascending). The detector emits
// observations in stream order; we reproduce that for callers that depend on
// time order (FactMatcher.Resolve walks linearly and trusts ordering).
func sortByOriginalOrder(facts []cmdenrich.EnrichedCommand) {
	for i := 1; i < len(facts); i++ {
		for j := i; j > 0 && facts[j].Second < facts[j-1].Second; j-- {
			facts[j], facts[j-1] = facts[j-1], facts[j]
		}
	}
}
