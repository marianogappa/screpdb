package markers

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// This file is the DSL for composing broad-match predicates. Each helper is a
// small, independently testable rule; Marker definitions in definitions.go
// combine them with All / Any / Not.
//
// Every helper returns a Predicate — a factory that produces a fresh
// PredicateState each time it's invoked. A PredicateState observes
// cmdenrich.EnrichedCommand as they arrive and reports Matched / Rejected as soon
// as the answer is determinate. At the RuleDeadline, any remaining Pending
// states collapse to Rejected via Finalize().
//
// Authors in definitions.go never see the state machinery: the combinators
// keep definitions reading as flat All(...) / Any(...) trees.

// -----------------------------------------------------------------------------
// Combinators
// -----------------------------------------------------------------------------

// All returns a Predicate that matches iff every child matches. Empty All
// matches.
func All(ps ...Predicate) Predicate {
	return func() PredicateState {
		children := make([]PredicateState, len(ps))
		for i, p := range ps {
			children[i] = p()
		}
		return &allState{children: children}
	}
}

type allState struct{ children []PredicateState }

func (a *allState) Observe(f cmdenrich.EnrichedCommand) {
	for _, c := range a.children {
		c.Observe(f)
	}
}

func (a *allState) Decision(now int) TriState {
	pending := false
	for _, c := range a.children {
		switch c.Decision(now) {
		case Rejected:
			return Rejected
		case Pending:
			pending = true
		}
	}
	if pending {
		return Pending
	}
	return Matched
}

func (a *allState) Finalize() TriState {
	for _, c := range a.children {
		if c.Finalize() == Rejected {
			return Rejected
		}
	}
	return Matched
}

// Any returns a Predicate that matches iff at least one child matches. Empty
// Any never matches.
func Any(ps ...Predicate) Predicate {
	return func() PredicateState {
		children := make([]PredicateState, len(ps))
		for i, p := range ps {
			children[i] = p()
		}
		return &anyState{children: children}
	}
}

type anyState struct{ children []PredicateState }

func (a *anyState) Observe(f cmdenrich.EnrichedCommand) {
	for _, c := range a.children {
		c.Observe(f)
	}
}

func (a *anyState) Decision(now int) TriState {
	pending := false
	for _, c := range a.children {
		switch c.Decision(now) {
		case Matched:
			return Matched
		case Pending:
			pending = true
		}
	}
	if pending {
		return Pending
	}
	return Rejected
}

func (a *anyState) Finalize() TriState {
	for _, c := range a.children {
		if c.Finalize() == Matched {
			return Matched
		}
	}
	return Rejected
}

// Not inverts a predicate's Matched/Rejected. Pending stays Pending.
func Not(p Predicate) Predicate {
	return func() PredicateState {
		return &notState{child: p()}
	}
}

type notState struct{ child PredicateState }

func (n *notState) Observe(f cmdenrich.EnrichedCommand) { n.child.Observe(f) }

func (n *notState) Decision(now int) TriState {
	switch n.child.Decision(now) {
	case Matched:
		return Rejected
	case Rejected:
		return Matched
	}
	return Pending
}

func (n *notState) Finalize() TriState {
	switch n.child.Finalize() {
	case Matched:
		return Rejected
	default:
		return Matched
	}
}

// -----------------------------------------------------------------------------
// Leaf predicates. Each is a small struct that observes facts monotonically
// and self-commits to Matched / Rejected as soon as its rule is decidable.
// -----------------------------------------------------------------------------

// commitState is a tiny mixin for predicates whose answer is a single
// Matched / Rejected commit once a distinguishing fact arrives.
type commitState struct{ done TriState }

func (c *commitState) finalizeDefaultRejected() TriState {
	if c.done == Pending {
		return Rejected
	}
	return c.done
}

// FirstBuildExists matches if the player ever Builds `subject`.
func FirstBuildExists(subject string) Predicate {
	return func() PredicateState {
		return &firstBuildExistsState{subject: subject}
	}
}

type firstBuildExistsState struct {
	commitState
	subject string
}

func (s *firstBuildExistsState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindMakeBuilding && f.Subject == s.subject {
		s.done = Matched
	}
}

func (s *firstBuildExistsState) Decision(int) TriState { return s.done }
func (s *firstBuildExistsState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// FirstProduceExists matches if the player ever produces a unit of `subject`
// (Train or Unit Morph). Backs the Carriers / Battlecruisers markers.
func FirstProduceExists(subject string) Predicate {
	return func() PredicateState {
		return &firstProduceExistsState{subject: subject}
	}
}

type firstProduceExistsState struct {
	commitState
	subject string
}

func (s *firstProduceExistsState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindMakeUnit && f.Subject == s.subject {
		s.done = Matched
	}
}

func (s *firstProduceExistsState) Decision(int) TriState { return s.done }
func (s *firstProduceExistsState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// ProduceCountAtLeast matches once the player has produced at least n units
// of `subject`. Time-agnostic: anchored only to the running tally, not to a
// build / second. Backs unit-mass signatures like "10+ Scouts".
func ProduceCountAtLeast(subject string, n int) Predicate {
	return func() PredicateState {
		return &produceCountAtLeastState{subject: subject, want: n}
	}
}

type produceCountAtLeastState struct {
	commitState
	subject string
	want    int
	count   int
}

func (s *produceCountAtLeastState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindMakeUnit && f.Subject == s.subject {
		s.count++
		if s.count >= s.want {
			s.done = Matched
		}
	}
}

func (s *produceCountAtLeastState) Decision(int) TriState { return s.done }
func (s *produceCountAtLeastState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// BuildCountAtLeast matches once at least n Build(subject) facts have been
// observed, with no time bound. The building analogue of ProduceCountAtLeast,
// used by whole-game signatures (e.g. "2+ Stargates") whose verdict is
// independent of when the builds land. Tile-level double-tap spam in the
// opening minutes is already collapsed upstream (see BuildDedupGapSeconds).
func BuildCountAtLeast(subject string, n int) Predicate {
	return func() PredicateState {
		return &buildCountAtLeastState{subject: subject, want: n}
	}
}

type buildCountAtLeastState struct {
	commitState
	subject string
	want    int
	count   int
}

func (s *buildCountAtLeastState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindMakeBuilding && f.Subject == s.subject {
		s.count++
		if s.count >= s.want {
			s.done = Matched
		}
	}
}

func (s *buildCountAtLeastState) Decision(int) TriState { return s.done }
func (s *buildCountAtLeastState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// HPUpgradeExists matches as soon as any tiered weapon/armor/shield upgrade
// (the "HP Upgrades" group) arrives. Used via Not(...) for the Never-Upgraded
// marker: only HP upgrades count as "upgrading"; every other upgrade is a
// research (see NonHPUpgradeExists).
func HPUpgradeExists() Predicate {
	return func() PredicateState { return &upgradeExistsState{wantHP: true} }
}

// NonHPUpgradeExists matches as soon as any non-HP upgrade (range, speed,
// energy, capacity/cooldown/damage — every upgrade tab except HP Upgrades)
// arrives. Combined with TechExists via Any(...) to back the Never-Researched
// marker: these upgrades are conceptually researches, split across tabs only
// for chart readability.
func NonHPUpgradeExists() Predicate {
	return func() PredicateState { return &upgradeExistsState{wantHP: false} }
}

type upgradeExistsState struct {
	commitState
	wantHP bool
}

func (s *upgradeExistsState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindUpgrade && models.IsHPUpgrade(f.Subject) == s.wantHP {
		s.done = Matched
	}
}

func (s *upgradeExistsState) Decision(int) TriState { return s.done }
func (s *upgradeExistsState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// TechExists matches as soon as any KindTech fact arrives. Used via Not(...)
// for the Never-Researched marker.
func TechExists() Predicate {
	return func() PredicateState { return &techExistsState{} }
}

type techExistsState struct{ commitState }

func (s *techExistsState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindTech {
		s.done = Matched
	}
}

func (s *techExistsState) Decision(int) TriState { return s.done }
func (s *techExistsState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// HotkeyExists matches as soon as any KindHotkey fact arrives. Used via
// Not(...) for the Never-Used-Hotkeys marker.
func HotkeyExists() Predicate {
	return func() PredicateState { return &hotkeyExistsState{} }
}

type hotkeyExistsState struct{ commitState }

func (s *hotkeyExistsState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindHotkey {
		s.done = Matched
	}
}

func (s *hotkeyExistsState) Decision(int) TriState { return s.done }
func (s *hotkeyExistsState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// FirstBuildBefore matches if the first Build(subject) happens strictly
// before maxSecond.
func FirstBuildBefore(subject string, maxSecond int) Predicate {
	return func() PredicateState {
		return &firstBuildBeforeState{subject: subject, max: maxSecond}
	}
}

type firstBuildBeforeState struct {
	commitState
	subject string
	max     int
}

func (s *firstBuildBeforeState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindMakeBuilding && f.Subject == s.subject {
		if f.Second < s.max {
			s.done = Matched
		} else {
			s.done = Rejected
		}
	}
}

func (s *firstBuildBeforeState) Decision(now int) TriState {
	if s.done != Pending {
		return s.done
	}
	if now >= s.max {
		return Rejected
	}
	return Pending
}

func (s *firstBuildBeforeState) Finalize() TriState { return s.finalizeDefaultRejected() }

// FirstBuildAtOrAfter matches if the first Build(subject) happens at or after
// minSecond.
func FirstBuildAtOrAfter(subject string, minSecond int) Predicate {
	return func() PredicateState {
		return &firstBuildAtOrAfterState{subject: subject, min: minSecond}
	}
}

type firstBuildAtOrAfterState struct {
	commitState
	subject string
	min     int
}

func (s *firstBuildAtOrAfterState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindMakeBuilding && f.Subject == s.subject {
		if f.Second >= s.min {
			s.done = Matched
		} else {
			s.done = Rejected
		}
	}
}

func (s *firstBuildAtOrAfterState) Decision(int) TriState { return s.done }
func (s *firstBuildAtOrAfterState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// BuildBefore matches if `a` is built AND either `b` is never built OR a < b.
// Leverages monotonic command arrival: whichever of a/b is observed first
// wins.
func BuildBefore(a, b string) Predicate {
	return func() PredicateState {
		return &buildBeforeState{a: a, b: b}
	}
}

type buildBeforeState struct {
	commitState
	a, b string
}

func (s *buildBeforeState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeBuilding {
		return
	}
	switch f.Subject {
	case s.a:
		s.done = Matched
	case s.b:
		s.done = Rejected
	}
}

func (s *buildBeforeState) Decision(int) TriState { return s.done }
func (s *buildBeforeState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// BuildAfterWithin matches if the first Build(after) falls in the half-open
// window (firstBuild(ref), firstBuild(ref)+maxGap]. Requires both to exist.
func BuildAfterWithin(after, ref string, maxGap int) Predicate {
	return func() PredicateState {
		return &buildAfterWithinState{after: after, ref: ref, maxGap: maxGap, refSec: -1}
	}
}

type buildAfterWithinState struct {
	commitState
	after, ref string
	maxGap     int
	refSec     int // -1 until observed
}

func (s *buildAfterWithinState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeBuilding {
		return
	}
	switch f.Subject {
	case s.ref:
		if s.refSec < 0 {
			s.refSec = f.Second
		}
	case s.after:
		if s.refSec < 0 {
			s.done = Rejected
			return
		}
		gap := f.Second - s.refSec
		if gap > 0 && gap <= s.maxGap {
			s.done = Matched
		} else {
			s.done = Rejected
		}
	}
}

func (s *buildAfterWithinState) Decision(now int) TriState {
	if s.done != Pending {
		return s.done
	}
	if s.refSec >= 0 && now-s.refSec > s.maxGap {
		return Rejected
	}
	return Pending
}

func (s *buildAfterWithinState) Finalize() TriState { return s.finalizeDefaultRejected() }

// NoProduceBeforeBuild matches if refSubject is eventually built AND no
// Produce(unit) happened strictly before the first Build(refSubject).
func NoProduceBeforeBuild(unit, refSubject string) Predicate {
	return func() PredicateState {
		return &noProduceBeforeBuildState{unit: unit, ref: refSubject}
	}
}

type noProduceBeforeBuildState struct {
	commitState
	unit, ref        string
	sawProduceBefore bool
}

func (s *noProduceBeforeBuildState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	switch f.Kind {
	case cmdenrich.KindMakeUnit:
		if f.Subject == s.unit {
			s.sawProduceBefore = true
		}
	case cmdenrich.KindMakeBuilding:
		if f.Subject == s.ref {
			if s.sawProduceBefore {
				s.done = Rejected
			} else {
				s.done = Matched
			}
		}
	}
}

func (s *noProduceBeforeBuildState) Decision(int) TriState { return s.done }
func (s *noProduceBeforeBuildState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// NthBuildBeforeFirstProduce matches if the n-th Build(buildSubject) happens
// strictly before the first Produce(produceUnit). Used negated to keep a
// single-production-structure opener honest, e.g.
// Not(NthBuildBeforeFirstProduce(Gateway, 2, Reaver)) rejects a 2nd Gateway
// laid before the Reaver ever pops — a "1 Gate Reaver" that already has two
// Gateways by reaver time is really a 2-Gate build.
func NthBuildBeforeFirstProduce(buildSubject string, n int, produceUnit string) Predicate {
	return func() PredicateState {
		return &nthBuildBeforeFirstProduceState{build: buildSubject, n: n, unit: produceUnit}
	}
}

type nthBuildBeforeFirstProduceState struct {
	commitState
	build, unit string
	n, builds   int
}

func (s *nthBuildBeforeFirstProduceState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	switch f.Kind {
	case cmdenrich.KindMakeBuilding:
		if f.Subject == s.build {
			s.builds++
			if s.builds >= s.n {
				s.done = Matched
			}
		}
	case cmdenrich.KindMakeUnit:
		if f.Subject == s.unit && s.builds < s.n {
			s.done = Rejected
		}
	}
}

func (s *nthBuildBeforeFirstProduceState) Decision(int) TriState { return s.done }
func (s *nthBuildBeforeFirstProduceState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// ProduceBeforeBuild matches if at least one Produce(unit) happened strictly
// before the first Build(refSubject). Requires refSubject to be built.
func ProduceBeforeBuild(unit, refSubject string) Predicate {
	return func() PredicateState {
		return &produceBeforeBuildState{unit: unit, ref: refSubject}
	}
}

type produceBeforeBuildState struct {
	commitState
	unit, ref  string
	sawProduce bool
}

func (s *produceBeforeBuildState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	switch f.Kind {
	case cmdenrich.KindMakeUnit:
		if f.Subject == s.unit {
			s.sawProduce = true
		}
	case cmdenrich.KindMakeBuilding:
		if f.Subject == s.ref {
			if s.sawProduce {
				s.done = Matched
			} else {
				s.done = Rejected
			}
		}
	}
}

func (s *produceBeforeBuildState) Decision(int) TriState { return s.done }
func (s *produceBeforeBuildState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// ProduceCountBeforeBuild matches if EXACTLY n Produce(unit) facts arrive
// strictly before the first Build(refSubject). The early-game spam filter
// (internal/earlyfilter) drops engine-impossible morphs so the surviving
// stream is a faithful count: this predicate keys Zerg build orders off
// that count alone — no timing windows, no race-against-clock heuristics.
//
// Rejects on observing the (n+1)-th Produce(unit) before refSubject, and
// on observing refSubject with a count != n.
func ProduceCountBeforeBuild(unit, refSubject string, n int) Predicate {
	return func() PredicateState {
		return &produceCountBeforeBuildState{unit: unit, ref: refSubject, want: n}
	}
}

type produceCountBeforeBuildState struct {
	commitState
	unit, ref string
	want      int
	// produces buffers each Produce(unit) as (second, count) until the ref
	// Build is observed. We resolve by the produce's *game second* relative to
	// the build's second rather than by observation order: the build-dedup tail
	// (see player_marker.go enqueueDedup) holds a Build fact for a few seconds,
	// during which a unit morphed just *after* the building would otherwise be
	// miscounted as before it (e.g. the 6th Drone morphed 2s after a 9-supply
	// Pool, inflating "9 Overpool" into "10 Pool").
	produces []produceObservation
}

type produceObservation struct {
	second int
	count  int
}

func (s *produceCountBeforeBuildState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	switch f.Kind {
	case cmdenrich.KindMakeUnit:
		if f.Subject == s.unit {
			s.produces = append(s.produces, produceObservation{second: f.Second, count: factUnitCount(f)})
		}
	case cmdenrich.KindMakeBuilding:
		if f.Subject == s.ref {
			count := 0
			for _, p := range s.produces {
				if p.second < f.Second {
					count += p.count
				}
			}
			if count == s.want {
				s.done = Matched
			} else {
				s.done = Rejected
			}
		}
	}
}

// factUnitCount is how many units a Produce fact represents — normally 1, but a
// single Zerg larva-morph command can morph several selected larvae at once
// (see cmdenrich.EnrichedCommand.Count). Facts built without a count (DB-side
// FromAction, test literals) report 0 and are treated as 1.
func factUnitCount(f cmdenrich.EnrichedCommand) int {
	if f.Count > 1 {
		return f.Count
	}
	return 1
}

func (s *produceCountBeforeBuildState) Decision(int) TriState { return s.done }
func (s *produceCountBeforeBuildState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// ProduceCountAtLeastBeforeBuild matches if AT LEAST n Produce(unit) facts
// arrive strictly before the first Build(refSubject). Like
// ProduceCountBeforeBuild but with a >= threshold instead of an exact count —
// used by the residual "Other Pool / Other Hatch" catch-alls, which claim the
// greedy tail of the drone ladder (supply >= 13) that the exact rungs don't.
//
// Matches as soon as the n-th Produce(unit) arrives before refSubject; rejects
// on observing refSubject with fewer than n produced.
func ProduceCountAtLeastBeforeBuild(unit, refSubject string, n int) Predicate {
	return func() PredicateState {
		return &produceCountAtLeastBeforeBuildState{unit: unit, ref: refSubject, want: n}
	}
}

type produceCountAtLeastBeforeBuildState struct {
	commitState
	unit, ref string
	want      int
	count     int
}

func (s *produceCountAtLeastBeforeBuildState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	switch f.Kind {
	case cmdenrich.KindMakeUnit:
		if f.Subject == s.unit {
			s.count += factUnitCount(f)
			if s.count >= s.want {
				s.done = Matched
			}
		}
	case cmdenrich.KindMakeBuilding:
		if f.Subject == s.ref {
			// First ref build with too few produced so far — can never reach
			// the threshold "before the first ref build".
			if s.count < s.want {
				s.done = Rejected
			} else {
				s.done = Matched
			}
		}
	}
}

func (s *produceCountAtLeastBeforeBuildState) Decision(int) TriState { return s.done }
func (s *produceCountAtLeastBeforeBuildState) Finalize() TriState    { return s.finalizeDefaultRejected() }

// CountBuildsBefore matches if at least n Build(subject) facts happen with
// second strictly less than maxSecond.
func CountBuildsBefore(subject string, n, maxSecond int) Predicate {
	return func() PredicateState {
		return &countBuildsBeforeState{subject: subject, n: n, max: maxSecond}
	}
}

type countBuildsBeforeState struct {
	commitState
	subject string
	n, max  int
	count   int
}

func (s *countBuildsBeforeState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeBuilding || f.Subject != s.subject {
		return
	}
	if f.Second >= s.max {
		if s.count < s.n {
			s.done = Rejected
		}
		return
	}
	s.count++
	if s.count >= s.n {
		s.done = Matched
	}
}

func (s *countBuildsBeforeState) Decision(now int) TriState {
	if s.done != Pending {
		return s.done
	}
	if now >= s.max {
		return Rejected
	}
	return Pending
}

func (s *countBuildsBeforeState) Finalize() TriState { return s.finalizeDefaultRejected() }

// BuildCountEqualsBefore matches if EXACTLY n Build(subject) facts happen with
// second strictly less than maxSecond. Backs the per-Factory / per-Barracks
// composition buckets (e.g. "exactly 3 Factories by 10:00"). Rejects early as
// soon as the (n+1)-th build lands before the deadline; otherwise defers the
// verdict to the deadline (fewer than n is only knowable once time is up).
func BuildCountEqualsBefore(subject string, n, maxSecond int) Predicate {
	return func() PredicateState {
		return &countBuildsEqualsBeforeState{subject: subject, n: n, max: maxSecond}
	}
}

type countBuildsEqualsBeforeState struct {
	commitState
	subject string
	n, max  int
	count   int
}

func (s *countBuildsEqualsBeforeState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeBuilding || f.Subject != s.subject || f.Second >= s.max {
		return
	}
	s.count++
	if s.count > s.n {
		s.done = Rejected
	}
}

func (s *countBuildsEqualsBeforeState) Decision(now int) TriState {
	if s.done != Pending {
		return s.done
	}
	if now >= s.max {
		if s.count == s.n {
			return Matched
		}
		return Rejected
	}
	return Pending
}

func (s *countBuildsEqualsBeforeState) Finalize() TriState {
	if s.done == Rejected {
		return Rejected
	}
	if s.count == s.n {
		return Matched
	}
	return Rejected
}

// ProduceCountAtLeastBefore matches if at least n units of `unit` are produced
// (Train / Unit Morph) with second strictly less than maxSecond. Commits early
// once the threshold is reached; rejects at the deadline if it never is.
func ProduceCountAtLeastBefore(unit string, n, maxSecond int) Predicate {
	return func() PredicateState {
		return &produceCountAtLeastBeforeState{unit: unit, n: n, max: maxSecond}
	}
}

type produceCountAtLeastBeforeState struct {
	commitState
	unit   string
	n, max int
	count  int
}

func (s *produceCountAtLeastBeforeState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeUnit || f.Subject != s.unit || f.Second >= s.max {
		return
	}
	s.count++
	if s.count >= s.n {
		s.done = Matched
	}
}

func (s *produceCountAtLeastBeforeState) Decision(now int) TriState {
	if s.done != Pending {
		return s.done
	}
	if now >= s.max {
		return Rejected
	}
	return Pending
}

func (s *produceCountAtLeastBeforeState) Finalize() TriState { return s.finalizeDefaultRejected() }

// ProduceCountAtMostBefore matches if AT MOST n units of `unit` are produced
// with second strictly less than maxSecond. Rejects early as soon as the
// (n+1)-th unit lands before the deadline; otherwise matches (an upper bound is
// only confirmable once the window closes, so it defaults to Matched).
func ProduceCountAtMostBefore(unit string, n, maxSecond int) Predicate {
	return func() PredicateState {
		return &produceCountAtMostBeforeState{unit: unit, n: n, max: maxSecond}
	}
}

type produceCountAtMostBeforeState struct {
	commitState
	unit   string
	n, max int
	count  int
}

func (s *produceCountAtMostBeforeState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeUnit || f.Subject != s.unit || f.Second >= s.max {
		return
	}
	s.count++
	if s.count > s.n {
		s.done = Rejected
	}
}

func (s *produceCountAtMostBeforeState) Decision(now int) TriState {
	if s.done != Pending {
		return s.done
	}
	if now >= s.max {
		return Matched
	}
	return Pending
}

func (s *produceCountAtMostBeforeState) Finalize() TriState {
	if s.done == Rejected {
		return Rejected
	}
	return Matched
}

// Predominant matches if the total count of units in `units` produced before
// maxSecond strictly exceeds the total count of units in `over`. Backs the
// bio-vs-mech composition split. The verdict can only be known once the window
// closes (either side can still grow), so it always defers to the deadline.
func Predominant(units, over []string, maxSecond int) Predicate {
	return func() PredicateState {
		inA := make(map[string]struct{}, len(units))
		for _, u := range units {
			inA[u] = struct{}{}
		}
		inB := make(map[string]struct{}, len(over))
		for _, u := range over {
			inB[u] = struct{}{}
		}
		return &predominantState{inA: inA, inB: inB, max: maxSecond}
	}
}

type predominantState struct {
	inA, inB   map[string]struct{}
	max        int
	aCnt, bCnt int
}

func (s *predominantState) Observe(f cmdenrich.EnrichedCommand) {
	if f.Kind != cmdenrich.KindMakeUnit || f.Second >= s.max {
		return
	}
	if _, ok := s.inA[f.Subject]; ok {
		s.aCnt++
	} else if _, ok := s.inB[f.Subject]; ok {
		s.bCnt++
	}
}

func (s *predominantState) verdict() TriState {
	if s.aCnt > s.bCnt {
		return Matched
	}
	return Rejected
}

func (s *predominantState) Decision(now int) TriState {
	if now >= s.max {
		return s.verdict()
	}
	return Pending
}

func (s *predominantState) Finalize() TriState { return s.verdict() }

// NthBuildBeforeAll matches if the n-th Build(subject) exists AND its second
// is strictly less than the first Build of every member of `others`.
func NthBuildBeforeAll(subject string, n int, others []string) Predicate {
	return func() PredicateState {
		set := make(map[string]struct{}, len(others))
		for _, o := range others {
			set[o] = struct{}{}
		}
		return &nthBuildBeforeAllState{subject: subject, n: n, others: set}
	}
}

type nthBuildBeforeAllState struct {
	commitState
	subject   string
	n         int
	others    map[string]struct{}
	subjCount int
	otherSeen bool
}

func (s *nthBuildBeforeAllState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind != cmdenrich.KindMakeBuilding {
		return
	}
	if _, isOther := s.others[f.Subject]; isOther {
		s.otherSeen = true
		return
	}
	if f.Subject != s.subject {
		return
	}
	s.subjCount++
	if s.subjCount >= s.n {
		if s.otherSeen {
			s.done = Rejected
		} else {
			s.done = Matched
		}
	}
}

func (s *nthBuildBeforeAllState) Decision(int) TriState { return s.done }
func (s *nthBuildBeforeAllState) Finalize() TriState    { return s.finalizeDefaultRejected() }
