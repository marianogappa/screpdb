package detectors

import (
	"encoding/json"
	"slices"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

// MarkerPlayerDetector evaluates one Marker for one player; the orchestrator's
// markers-loop creates one per (player × marker). Exactly one of the Marker's
// Rule / Custom fields is expected to be set.
//
// The Rule path streams a PredicateState tree plus a small dedup tail buffer and
// commits as soon as determinate. The Custom path feeds every classified command
// to a CustomEvaluator and calls its Finalize at end-of-window for a richer value.
type MarkerPlayerDetector struct {
	BasePlayerDetector
	marker markers.Marker

	state   markers.PredicateState
	pending map[string]cmdenrich.EnrichedCommand // dedup tail for KindMakeBuilding facts
	// observed records every fact past dedup, in stream order, so Expert milestones
	// resolve once at save time instead of on every dashboard page load. Only
	// populated for markers with non-empty Expert.
	observed []cmdenrich.EnrichedCommand
	// On a Matched commit during streaming this is the second that flipped the
	// decision, which becomes the marker's DetectedAtSecond.
	lastObservedSecond int

	// One running predicate per Rule-based Modifier, fed the same dedup'd stream as
	// the BO rule. WorldstateEvent modifiers are checked separately in GetResult.
	modifierStates []modifierState
	// Such a result must not be emitted until the worldstate is finalized (the
	// orchestrator does so before the final detector pass), else reading it would
	// trigger a premature worldstate Finalize.
	hasWorldstateModifier bool

	custom markers.CustomEvaluator
	// Cached so GetResult can reach DetectedAtSecond and Payload.
	customResult markers.CustomResult

	matched          bool
	detectedAtSecond int
}

type modifierState struct {
	name  string
	state markers.PredicateState
}

func NewMarkerPlayerDetector(m markers.Marker) *MarkerPlayerDetector {
	d := &MarkerPlayerDetector{marker: m}
	if m.Rule != nil {
		d.state = m.Rule()
		d.pending = map[string]cmdenrich.EnrichedCommand{}
	} else if m.Custom != nil {
		d.custom = m.Custom()
	}
	for _, mod := range m.Modifiers {
		if mod.Rule != nil {
			d.modifierStates = append(d.modifierStates, modifierState{name: mod.Name, state: mod.Rule()})
		}
		if mod.WorldstateEvent != "" {
			d.hasWorldstateModifier = true
		}
	}
	return d
}

func (d *MarkerPlayerDetector) Name() string { return d.marker.PatternName }

func (d *MarkerPlayerDetector) ProcessCommand(command *models.Command) bool {
	if !d.ShouldProcessCommand(command) {
		return false
	}
	if d.IsFinished() {
		// Already committed (or finalized at deadline). Trailing commands must be
		// ignored: the dedup map is nil after commit and would panic on insert.
		return true
	}
	if d.marker.Race != "" && !isPlayerRace(d.GetPlayers(), d.GetReplayPlayerID(), string(d.marker.Race)) {
		d.commitRejected()
		return true
	}
	if len(d.marker.Matchup) > 0 {
		replay := d.GetReplay()
		if replay == nil || !markers.MatchupAdmits(d.marker.Matchup, replay.Matchup, replay.TeamFormat) {
			d.commitRejected()
			return true
		}
	}
	if len(d.marker.MapKind) > 0 {
		replay := d.GetReplay()
		if replay == nil || !slices.Contains(d.marker.MapKind, replay.MapKind) {
			d.commitRejected()
			return true
		}
	}
	// Money maps are deliberately NOT filtered at detection time for build orders:
	// BOs still detect so the Build Orders tab and per-player summary pills show on
	// Money games, and the render layer suppresses the BO chips instead (see
	// endpoint_main_games_players_list.go and buildMainGameFeaturingPills). Markers
	// that should fire ONLY on a MapKind set the field above explicitly.
	now := command.SecondsFromGameStart

	if d.state != nil {
		return d.processRule(command, now)
	}
	if d.custom != nil {
		return d.processCustom(command, now)
	}
	// Neither Rule nor Custom: misconfigured, so finalize as a no-op.
	d.SetFinished(true)
	return true
}

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
		// Upgrade/Tech/Hotkey facts bypass the subject gate: their subjects are
		// upgrade/tech names or hotkey groups rather than units, and their predicates
		// don't filter by subject.
		d.observeRuleFact(fact)
	}
	return d.checkRuleDecision(now)
}

// observeRuleFact records the fact's second so a subsequent Matched commit can
// report the flipping fact's timestamp, and captures it into d.observed when the
// marker has Expert events so ResolveExpert sees the same dedup'd stream.
func (d *MarkerPlayerDetector) observeRuleFact(f cmdenrich.EnrichedCommand) {
	d.lastObservedSecond = f.Second
	d.state.Observe(f)
	for i := range d.modifierStates {
		d.modifierStates[i].state.Observe(f)
	}
	if len(d.marker.Expert) > 0 {
		d.observed = append(d.observed, f)
	}
}

func (d *MarkerPlayerDetector) enqueueDedup(f cmdenrich.EnrichedCommand) {
	// Past BuildDedupMaxSecond, skip dedup: flush any prior pending for this
	// subject as a real observation, then observe the current fact too.
	if f.Second >= markers.BuildDedupMaxSecond {
		if prior, ok := d.pending[f.Subject]; ok {
			d.observeRuleFact(prior)
			delete(d.pending, f.Subject)
		}
		d.observeRuleFact(f)
		return
	}
	if prior, ok := d.pending[f.Subject]; ok {
		if f.Second-prior.Second < markers.BuildDedupGapSeconds && sameBuildTile(prior, f) {
			d.pending[f.Subject] = f
			return
		}
		d.observeRuleFact(prior)
	}
	d.pending[f.Subject] = f
}

// Dedup only collapses repeat placements of the same building at the same spot
// (double-tap / misclick); two same-type buildings at DIFFERENT tiles are
// genuinely distinct and must both be observed, even seconds apart. Positions
// are required — an unknown one makes the pair distinct rather than collapsing
// on a guess.
func sameBuildTile(a, b cmdenrich.EnrichedCommand) bool {
	return a.X != nil && a.Y != nil && b.X != nil && b.Y != nil && *a.X == *b.X && *a.Y == *b.Y
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
		// Post-match calls keep the original second, so DetectedAtSecond reflects the
		// build-decision moment rather than later observations.
		if !d.matched {
			d.matched = true
			d.detectedAtSecond = d.lastObservedSecond
		}
		// Don't SetFinished yet: Expert milestones can lie further out ("First Zealot"
		// ~108s for 2 Gate, after the 2nd-Gateway commit at ~86s), so stay alive until
		// RuleDeadline to keep appending to d.observed for ResolveExpert.
		return false
	case markers.Rejected:
		d.commitRejected()
		return true
	}
	return false
}

func (d *MarkerPlayerDetector) finalizeRuleAtDeadline() {
	d.SetFinished(true)
	d.flushAllPending()
	// Keep the streaming commit's verdict and DetectedAtSecond. Re-running Finalize
	// on a Matched state is a no-op anyway, but this deliberately bypasses
	// overwriting DetectedAtSecond with replay-end / RuleDeadline — correct only for
	// rules that actually resolved at the deadline, like absence markers.
	if d.matched {
		d.pending = nil
		return
	}
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
}

// Finalize handles end-of-replay for detectors that never tripped their
// deadline, forcing a commitment on whichever path is active.
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

// GetResult returns a result only when the marker matched AND any duration gate
// is satisfied. Rule markers with Expert milestones emit a payload of
// position-aligned actual seconds so the dashboard doesn't re-resolve on every
// read; Custom markers emit whatever their evaluator returned.
func (d *MarkerPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	// A worldstate-backed modifier can only be read after the batch pipeline runs.
	// If this detector finished mid-stream, defer emission until the orchestrator
	// finalizes the worldstate — reading the event now would trigger a premature
	// Finalize that locks in an incomplete event stream.
	if d.hasWorldstateModifier {
		if ws := d.GetWorldState(); ws != nil && !ws.Finalized() {
			return nil
		}
	}
	if d.state != nil {
		var payload json.RawMessage
		modifiers := d.matchedModifiers()
		if len(d.marker.Expert) > 0 || len(modifiers) > 0 {
			resolutions := d.marker.ResolveExpert(d.observed)
			if encoded, err := markers.EncodeBuildOrderPayload(resolutions, modifiers); err == nil {
				payload = encoded
			}
		}
		return d.BuildPlayerResult(d.marker.PatternName, d.detectedAtSecond, payload)
	}
	return d.BuildPlayerResult(d.marker.PatternName, d.detectedAtSecond, d.customResult.Payload)
}

// Order follows Marker.Modifiers so the payload is deterministic.
func (d *MarkerPlayerDetector) matchedModifiers() []string {
	if len(d.marker.Modifiers) == 0 {
		return nil
	}
	ruleVerdict := map[string]bool{}
	for i := range d.modifierStates {
		ruleVerdict[d.modifierStates[i].name] = d.modifierStates[i].state.Finalize() == markers.Matched
	}
	ws := d.GetWorldState()
	var out []string
	for _, mod := range d.marker.Modifiers {
		held := false
		if mod.Rule != nil {
			held = ruleVerdict[mod.Name]
		} else if mod.WorldstateEvent != "" {
			held = ws != nil && ws.FirstEventSecondForPlayer(d.GetReplayPlayerID(), mod.WorldstateEvent) != nil
		}
		if held {
			out = append(out, mod.Name)
		}
	}
	return out
}

func (d *MarkerPlayerDetector) ShouldSave() bool {
	if !d.IsFinished() || !d.matched {
		return false
	}
	if d.marker.RequireWorldstateEvent != "" {
		ws := d.GetWorldState()
		if ws == nil || ws.FirstEventSecondForPlayer(d.GetReplayPlayerID(), d.marker.RequireWorldstateEvent) == nil {
			return false
		}
	}
	gate := d.resolveMinReplaySeconds()
	if gate > 0 {
		replay := d.GetReplay()
		if replay == nil {
			return false
		}
		// Compare against the player's effective time in game, not the replay's
		// duration: in non-1v1 games a player can leave long before the replay ends, and
		// the replay duration would wrongly flag "never upgraded" on someone who quit at
		// 5min of a 30min FFA. Last-command second beats leaveSec here — some replays
		// record a spurious early leave_game for players who keep playing.
		playerTime := replay.DurationSeconds
		if ws := d.GetWorldState(); ws != nil {
			if lastSec, ok := ws.LastCommandSecond(d.GetReplayPlayerID()); ok && lastSec < playerTime {
				playerTime = lastSec
			}
		}
		if playerTime < gate {
			return false
		}
	}
	return true
}

// A hard floor on top of any per-matchup gate. Several Upgrade matchups have a
// progamer p5 of first-Upgrade around 2:00-3:00 (ZvZ Speed, PvT Singularity
// Charge), and used alone those gates flag "never upgraded" on successful 4-pool
// / Bunker rush finishes — the exact rush-suppression case the matchup gate
// exists to preserve. 4 min sits above the typical successful-rush game end, so
// short rushes stay suppressed while longer games still get a higher gate.
const matchupGateMinSeconds = 4 * 60

// resolveMinReplaySeconds prefers a per-(own race, opp race) entry for 1v1 so
// "never X" markers respect matchup-typical first-research timings, lifted to at
// least matchupGateMinSeconds. Non-1v1 or a missing bucket falls back to the
// flat MinReplaySeconds.
func (d *MarkerPlayerDetector) resolveMinReplaySeconds() int {
	if len(d.marker.MinReplaySecondsByMatchup) == 0 {
		return d.marker.MinReplaySeconds
	}
	replay := d.GetReplay()
	if replay == nil || replay.TeamFormat != "1v1" {
		return d.marker.MinReplaySeconds
	}
	players := d.GetPlayers()
	own := getPlayerByReplayPlayerID(players, d.GetReplayPlayerID())
	if own == nil {
		return d.marker.MinReplaySeconds
	}
	opp := getOpponentInOneVOne(players, own)
	if opp == nil {
		return d.marker.MinReplaySeconds
	}
	byOpp, ok := d.marker.MinReplaySecondsByMatchup[markers.Race(own.Race)]
	if !ok {
		return d.marker.MinReplaySeconds
	}
	v, ok := byOpp[markers.Race(opp.Race)]
	if !ok {
		return d.marker.MinReplaySeconds
	}
	if v < matchupGateMinSeconds {
		return matchupGateMinSeconds
	}
	return v
}
