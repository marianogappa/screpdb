package detectors

import (
	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// MarkerPlayerDetector evaluates one Marker for one player. Instances are
// created per (player × marker) by the orchestrator's markers-loop.
//
// Two dispatch modes:
//
//   - Rule (predicate DSL): streaming PredicateState tree + small dedup tail
//     buffer. Commits Matched/Rejected as soon as the state is determinate.
//   - Custom (escape-hatch evaluator): feeds every classified command to a
//     CustomEvaluator; calls evaluator.Finalize at end-of-window to obtain
//     a richer value (int / string / time) alongside the match verdict.
//
// Exactly one of the Marker's Rule / Custom fields is expected to be set.
type MarkerPlayerDetector struct {
	BasePlayerDetector
	marker markers.Marker

	// Rule path state:
	state   markers.PredicateState
	pending map[string]cmdenrich.EnrichedCommand // dedup tail for KindMakeBuilding facts
	// lastObservedSecond tracks the replay second of the most recent fact fed into state.
	// On a Matched commit during streaming, this is the second that flipped the decision —
	// used as the marker's DetectedAtSecond.
	lastObservedSecond int

	// Custom path state:
	custom markers.CustomEvaluator
	// customResult is the result from CustomEvaluator.Finalize, cached so GetResult has access
	// to DetectedAtSecond + Payload (not just the legacy Value fields).
	customResult markers.CustomResult

	matched          bool
	detectedAtSecond int
	result           markers.MarkerValue
}

// NewMarkerPlayerDetector creates a detector for the given marker.
func NewMarkerPlayerDetector(m markers.Marker) *MarkerPlayerDetector {
	d := &MarkerPlayerDetector{marker: m}
	if m.Rule != nil {
		d.state = m.Rule()
		d.pending = map[string]cmdenrich.EnrichedCommand{}
	} else if m.Custom != nil {
		d.custom = m.Custom()
	}
	return d
}

// Name returns the stored pattern name (e.g. "Build Order: 9 Pool", "Carriers").
func (d *MarkerPlayerDetector) Name() string { return d.marker.PatternName }

// ProcessCommand dispatches to the Rule or Custom path.
func (d *MarkerPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}
	if d.marker.Race != "" && !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), string(d.marker.Race)) {
		d.commitRejected()
		return true
	}
	now := command.SecondsFromGameStart

	if d.state != nil {
		return d.processRule(command, now)
	}
	if d.custom != nil {
		return d.processCustom(command, now)
	}
	// Misconfigured marker — neither Rule nor Custom. Finalize as a no-op.
	d.SetFinished(true)
	return true
}

// -----------------------------------------------------------------------------
// Rule path
// -----------------------------------------------------------------------------

func (d *MarkerPlayerDetector) processRule(command *models.Command, now int) bool {
	d.flushDedupBefore(now)

	if now > d.marker.RuleDeadline {
		d.finalizeRuleAtDeadline()
		return true
	}

	fact, ok := cmdenrich.Classify(command)
	if !ok {
		return d.checkRuleDecision(now)
	}
	switch fact.Kind {
	case cmdenrich.KindMakeBuilding:
		if markers.IsSubjectOfInterest(fact.Subject) {
			d.enqueueDedup(fact)
		}
	case cmdenrich.KindMakeUnit:
		if markers.IsSubjectOfInterest(fact.Subject) {
			d.observeRuleFact(fact)
		}
	case cmdenrich.KindUpgrade, cmdenrich.KindTech, cmdenrich.KindHotkey:
		// Upgrade/Tech/Hotkey facts bypass the subject gate — their
		// subjects are upgrade/tech names or hotkey groups, not
		// units/buildings, and their predicates don't filter by subject.
		d.observeRuleFact(fact)
	}
	return d.checkRuleDecision(now)
}

// observeRuleFact funnels a fact into the predicate state and records its second
// so a subsequent Matched commit can report the flipping fact's timestamp.
func (d *MarkerPlayerDetector) observeRuleFact(f cmdenrich.EnrichedCommand) {
	d.lastObservedSecond = f.Second
	d.state.Observe(f)
}

func (d *MarkerPlayerDetector) enqueueDedup(f cmdenrich.EnrichedCommand) {
	if prior, ok := d.pending[f.Subject]; ok {
		if f.Second-prior.Second < markers.BuildDedupGapSeconds {
			d.pending[f.Subject] = f
			return
		}
		d.observeRuleFact(prior)
	}
	d.pending[f.Subject] = f
}

func (d *MarkerPlayerDetector) flushDedupBefore(now int) {
	for subj, f := range d.pending {
		if now-f.Second >= markers.BuildDedupGapSeconds {
			d.observeRuleFact(f)
			delete(d.pending, subj)
		}
	}
}

func (d *MarkerPlayerDetector) flushAllPending() {
	for subj, f := range d.pending {
		d.observeRuleFact(f)
		delete(d.pending, subj)
	}
}

func (d *MarkerPlayerDetector) checkRuleDecision(now int) bool {
	switch d.state.Decision(now) {
	case markers.Matched:
		d.matched = true
		d.detectedAtSecond = d.lastObservedSecond
		d.SetFinished(true)
		d.pending = nil
		return true
	case markers.Rejected:
		d.commitRejected()
		return true
	}
	return false
}

func (d *MarkerPlayerDetector) finalizeRuleAtDeadline() {
	d.SetFinished(true)
	d.flushAllPending()
	d.matched = d.state.Finalize() == markers.Matched
	if d.matched {
		// Absence markers and deadline-finalized rules commit at end-of-replay.
		if replay := d.GetReplay(); replay != nil {
			d.detectedAtSecond = replay.DurationSeconds
		} else {
			d.detectedAtSecond = d.marker.RuleDeadline
		}
	}
	d.pending = nil
}

// -----------------------------------------------------------------------------
// Custom path
// -----------------------------------------------------------------------------

func (d *MarkerPlayerDetector) processCustom(command *models.Command, now int) bool {
	if now > d.marker.RuleDeadline {
		d.finalizeCustomAtDeadline()
		return true
	}
	fact, ok := cmdenrich.Classify(command)
	if !ok {
		return false
	}
	d.custom.Observe(fact)
	return false
}

func (d *MarkerPlayerDetector) finalizeCustomAtDeadline() {
	d.SetFinished(true)
	if d.custom == nil {
		return
	}
	res := d.custom.Finalize(markers.CustomEvalContext{
		ReplayPlayerID: d.GetReplayPlayerID(),
		Replay:         d.GetReplay(),
		WorldState:     d.GetWorldState(),
	})
	d.matched = res.Matched
	d.detectedAtSecond = res.DetectedAtSecond
	d.customResult = res
	d.result = res.Value
}

// Finalize handles end-of-replay for detectors that never tripped their
// deadline. It forces a final commitment on whichever path is active.
func (d *MarkerPlayerDetector) Finalize() {
	if d.IsFinished() {
		return
	}
	if d.state != nil {
		d.finalizeRuleAtDeadline()
		return
	}
	if d.custom != nil {
		d.finalizeCustomAtDeadline()
		return
	}
	d.SetFinished(true)
}

func (d *MarkerPlayerDetector) commitRejected() {
	d.matched = false
	d.SetFinished(true)
	d.pending = nil
}

// GetResult returns a PatternResult when the marker matched AND any duration
// gate is satisfied. Uses the Custom evaluator's value when present; falls
// back to ValueBool:true for pure Rule markers.
func (d *MarkerPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	if d.state != nil {
		valueBool := true
		return d.BuildPlayerResult(d.marker.PatternName, d.detectedAtSecond, nil, &valueBool, nil, nil, nil)
	}
	return d.BuildPlayerResult(
		d.marker.PatternName,
		d.detectedAtSecond,
		d.customResult.Payload,
		d.result.Bool,
		d.result.Int,
		d.result.String,
		d.result.Time,
	)
}

// ShouldSave is true iff the marker matched AND any duration gate is met.
func (d *MarkerPlayerDetector) ShouldSave() bool {
	if !d.IsFinished() || !d.matched {
		return false
	}
	if d.marker.MinReplaySeconds > 0 {
		replay := d.GetReplay()
		if replay == nil || replay.DurationSeconds < d.marker.MinReplaySeconds {
			return false
		}
	}
	return true
}
