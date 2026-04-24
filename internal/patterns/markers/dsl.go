package markers

import "github.com/marianogappa/screpdb/internal/cmdenrich"

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

// UpgradeExists matches as soon as any KindUpgrade fact arrives, regardless
// of subject. Used via Not(...) for the Never-Upgraded marker.
func UpgradeExists() Predicate {
	return func() PredicateState { return &upgradeExistsState{} }
}

type upgradeExistsState struct{ commitState }

func (s *upgradeExistsState) Observe(f cmdenrich.EnrichedCommand) {
	if s.done != Pending {
		return
	}
	if f.Kind == cmdenrich.KindUpgrade {
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
