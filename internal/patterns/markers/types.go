// Package markers is the single source of truth for Marker definitions.
//
// A Marker is a classifier attached to a (replay × player) that reports
// something interesting about the player's play — an opening build order, a
// late-game signature (Carriers, Battlecruisers), an absence (Never
// researched), or a worldstate-sourced event (Made drops, Became Zerg).
//
// Two layers of definition are expressed for each Marker:
//
//  1. Rule or Custom — the match + value extraction. Bool-only markers use
//     a composable Predicate over a player's stream of cmdenrich.EnrichedCommand.
//     Value-producing markers (e.g. "second at which drop happened") use a
//     CustomEvaluator that can read worldstate at Finalize.
//  2. Expert (opener-only) — a list of named milestones with target second +
//     tolerance describing the "progamer ideal". Used by the UI's Build
//     Orders tab to compare actual player timings against the gold
//     standard. Only KindInitialBuildOrder markers populate this.
//
// Both definitions live here so consumers (pattern detectors, dashboard UI
// tab) share the same knowledge. Adding a new marker, or tweaking an
// existing one, is a single-file change in definitions.go.
package markers

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// Race is a narrow race identifier used to gate detectors.
type Race string

const (
	RaceZerg    Race = "Zerg"
	RaceProtoss Race = "Protoss"
	RaceTerran  Race = "Terran"
)

// BuildDedupGapSeconds collapses rapid repeat Build events of the same subject.
// Progamers often double-tap a building placement (spam / misclick); the earlier
// order is effectively cancelled. Build facts of the same subject less than
// this many seconds apart are treated as a single event (later wins).
const BuildDedupGapSeconds = 3

// TriState is the monotone decision a PredicateState reports.
//
// Once a state commits to Matched or Rejected it must stay there — Observe
// never un-commits a prior decision. Pending means "not enough information
// yet"; the caller's Finalize() pass collapses any remaining Pending to
// Rejected per the plan's deadline contract.
type TriState int

const (
	// Pending: the predicate cannot yet decide.
	Pending TriState = iota
	// Matched: the predicate is satisfied. Final.
	Matched
	// Rejected: the predicate cannot be satisfied. Final.
	Rejected
)

// PredicateState is the streaming evaluator a Predicate produces. Each
// marker's broad rule compiles into a tree of PredicateStates that together
// observe a player's commands as they arrive and report Matched / Rejected
// as soon as determinate.
//
// Implementation contract:
//
//   - Observe must be idempotent once committed (Matched or Rejected).
//     Further facts are ignored.
//   - Decision(now) reports the current best answer at the given in-game
//     second. It may return Pending before the predicate's intrinsic deadline.
//   - Finalize forces a final commitment. Pending collapses to the
//     "event-never-happened" answer for that predicate (usually Rejected, but
//     Not wraps children and inverts).
//
// All combinators (All / Any / Not) and every leaf DSL helper implement this
// interface.
type PredicateState interface {
	Observe(f cmdenrich.EnrichedCommand)
	Decision(now int) TriState
	Finalize() TriState
}

// Predicate is a factory that produces a fresh PredicateState each time it is
// called. Marker authors in definitions.go compose Predicates with All / Any /
// Not — never seeing the underlying state machinery.
type Predicate func() PredicateState

// Eval runs this predicate over a slice of facts (time-ordered) and returns
// whether the marker's broad rule ultimately matches. Used by tests and
// one-shot callers. The streaming detector path never calls Eval; it drives
// PredicateState directly.
func (p Predicate) Eval(facts []cmdenrich.EnrichedCommand) bool {
	if p == nil {
		return false
	}
	st := p()
	for _, f := range facts {
		st.Observe(f)
	}
	return st.Finalize() == Matched
}

// -----------------------------------------------------------------------------
// Custom evaluators — for markers that can't be expressed as a bool predicate
// (worldstate-sourced events, spatial/ratio stats). A Custom evaluator sees
// every classified command in order and emits a MarkerResult at Finalize.
// -----------------------------------------------------------------------------

// MarkerValue is the value a Custom evaluator emits on match. Mirrors the
// value columns on detected_patterns_replay_player. Exactly one of the
// pointer fields is expected to be set (or none if the marker is pure
// bool-by-match).
type MarkerValue struct {
	Bool   *bool
	Int    *int
	String *string
	Time   *int64
}

// CustomEvalContext carries the replay-scoped state a Custom evaluator may
// need at Finalize. Rule-based (Predicate) markers don't see this — they
// only observe the command stream.
type CustomEvalContext struct {
	ReplayPlayerID byte
	Replay         *models.Replay
	WorldState     *worldstate.Engine
}

// CustomResult is the verdict + optional value a Custom evaluator returns
// at Finalize.
type CustomResult struct {
	Matched bool
	Value   MarkerValue
}

// CustomEvaluator is the streaming evaluator for a Custom marker.
//
//   - Observe is called once per EnrichedCommand in replay time order.
//     Implementations that don't care about commands (purely worldstate-
//     sourced markers) can leave Observe a no-op.
//   - Finalize is called at end-of-window (RuleDeadline exceeded OR
//     end-of-replay) and reports the final verdict + value.
type CustomEvaluator interface {
	Observe(f cmdenrich.EnrichedCommand)
	Finalize(ctx CustomEvalContext) CustomResult
}

// Tolerance describes acceptable early/late deviation around an expert target
// second. Use Sym or Asym to construct.
type Tolerance struct {
	EarlySeconds int
	LateSeconds  int
}

// Sym constructs a symmetric tolerance (± v).
func Sym(v int) Tolerance { return Tolerance{EarlySeconds: v, LateSeconds: v} }

// Asym constructs an asymmetric tolerance (early, late).
func Asym(early, late int) Tolerance { return Tolerance{EarlySeconds: early, LateSeconds: late} }

// ExpertEvent describes one milestone in the progamer template.
//
//   - Key is a human-readable label used in the UI (e.g. "Spawning Pool",
//     "First Zergling").
//   - Match selects which fact counts as the actual occurrence.
//   - TargetSecond is the ideal second from game start.
//   - Tolerance is the acceptable deviation around TargetSecond.
type ExpertEvent struct {
	Key          string
	Match        FactMatcher
	TargetSecond int
	Tolerance    Tolerance
}

// FactMatcher selects a specific occurrence of a specific kind+subject.
type FactMatcher struct {
	Kind            cmdenrich.Kind
	Subject         string
	OccurrenceIndex int // 1-indexed; defaults to 1 when zero.
}

// MatchBuild is shorthand for the first Build of a subject.
func MatchBuild(subject string) FactMatcher {
	return FactMatcher{Kind: cmdenrich.KindMakeBuilding, Subject: subject, OccurrenceIndex: 1}
}

// MatchNthBuild is shorthand for the n-th Build of a subject.
func MatchNthBuild(subject string, n int) FactMatcher {
	return FactMatcher{Kind: cmdenrich.KindMakeBuilding, Subject: subject, OccurrenceIndex: n}
}

// MatchFirstProduce is shorthand for the first Produce of a unit.
func MatchFirstProduce(unit string) FactMatcher {
	return FactMatcher{Kind: cmdenrich.KindMakeUnit, Subject: unit, OccurrenceIndex: 1}
}

// Resolve finds the Second at which this matcher's fact actually occurred in
// the given slice. Returns (second, true) or (0, false) if not present.
func (m FactMatcher) Resolve(facts []cmdenrich.EnrichedCommand) (int, bool) {
	n := m.OccurrenceIndex
	if n <= 0 {
		n = 1
	}
	count := 0
	for _, f := range facts {
		if f.Kind != m.Kind || f.Subject != m.Subject {
			continue
		}
		count++
		if count == n {
			return f.Second, true
		}
	}
	return 0, false
}

// Kind categorizes a marker so that mutually-exclusive families (openers)
// can coexist in the registry alongside overlap-permitted ones (signatures,
// absences, worldstate-sourced events).
type Kind string

const (
	// KindInitialBuildOrder is an opening build order: the player's first
	// few actions from game start. At most one initial BO may match per
	// player (fuzz-enforced mutex).
	KindInitialBuildOrder Kind = "initial_build_order"
	// KindMarker is everything else. Multiple KindMarker entries may match
	// the same player simultaneously, including alongside a KindInitialBuildOrder.
	KindMarker Kind = "marker"
)

// Marker bundles both the classification rule and (for openers) the expert
// timings used by the Build Orders UI tab.
type Marker struct {
	// Name is the user-facing short name ("4 Pool", "Carriers", etc.). It
	// also doubles as the pattern name suffix stored in the DB for openers.
	Name string

	// Kind classifies the marker. Openers use KindInitialBuildOrder (mutex);
	// everything else uses KindMarker (overlap permitted).
	Kind Kind

	// PatternName is the name stored in detected_patterns_replay_player.
	// Openers use the form "Build Order: <Name>"; KindMarker entries use
	// bare names ("Carriers", "Quick factory", …) to preserve existing
	// frontend checks and DB-row compatibility.
	PatternName string

	// FeatureKey is the stable identifier used on the games-list
	// "Featuring" filter and in the frontend pill registry
	// (e.g. "bo_9_pool", "carriers", "made_drops").
	FeatureKey string

	// Race is the race this marker applies to. Empty string means "any race".
	Race Race

	// MinReplaySeconds gates this marker on replay duration. 0 = no gate.
	// Used by "never X" markers that would otherwise trip on short games.
	MinReplaySeconds int

	// Rule is the predicate-DSL path — tree of PredicateState factories.
	// If non-nil, the marker emits ValueBool:true on match.
	Rule Predicate

	// Custom is the alternative evaluator path for markers that can't be
	// expressed as a bool predicate — produces richer values (int / string
	// / time) typically sourced from worldstate. Exactly one of Rule /
	// Custom is expected to be non-nil.
	Custom func() CustomEvaluator

	// RuleDeadline is the last in-game second that could still change the
	// answer. Once a replay passes this second, the detector finalizes.
	// Set to the tightest upper-bound across all rule sub-predicates;
	// Custom markers that need the full replay use a large value (e.g.
	// end-of-replay sentinel).
	RuleDeadline int

	// Expert is the ordered list of ideal timings used by the Build Orders
	// UI tab to compare actual vs. gold-standard. Only populated for
	// KindInitialBuildOrder markers.
	Expert []ExpertEvent
}
