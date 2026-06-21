package detectors

import (
	"encoding/json"
	"slices"

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
	// observed records every fact past dedup, in stream order. Used to resolve
	// Expert milestones once at save time so the dashboard doesn't re-resolve on
	// every page load. Only populated for markers with non-empty Expert.
	observed []cmdenrich.EnrichedCommand
	// lastObservedSecond tracks the replay second of the most recent fact fed into state.
	// On a Matched commit during streaming, this is the second that flipped the decision —
	// used as the marker's DetectedAtSecond.
	lastObservedSecond int

	// Custom path state:
	custom markers.CustomEvaluator
	// customResult is the result from CustomEvaluator.Finalize, cached so GetResult has access
	// to DetectedAtSecond + Payload.
	customResult markers.CustomResult

	matched          bool
	detectedAtSecond int
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
	if d.IsFinished() {
		// Rule already committed Matched/Rejected (or marker finalized at
		// deadline). Trailing commands must be ignored; in particular the
		// dedup map is nil after commit and would panic on insert.
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
	// NOTE: Money maps are NOT filtered at detection time for build
	// orders. BOs still detect so the per-player Build Orders tab +
	// per-player summary pills can show on Money games. The render layer
	// (games-list "Featuring" column and game-detail summary featuring
	// strip) is responsible for suppressing BO chips on Money maps — see
	// internal/dashboard/endpoint_main_games_players_list.go and the
	// frontend buildMainGameFeaturingPills helper. Markers that should
	// ONLY fire on a specific MapKind set the field above explicitly
	// (e.g. "10+ Scouts" on Money maps only).
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
// Also captures the fact into d.observed when the marker has Expert events,
// so GetResult can ResolveExpert against the same dedup'd stream the detector saw.
func (d *MarkerPlayerDetector) observeRuleFact(f cmdenrich.EnrichedCommand) {
	d.lastObservedSecond = f.Second
	d.state.Observe(f)
	if len(d.marker.Expert) > 0 {
		d.observed = append(d.observed, f)
	}
}

func (d *MarkerPlayerDetector) enqueueDedup(f cmdenrich.EnrichedCommand) {
	// After BuildDedupMaxSecond, skip dedup: flush any prior pending for this
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

// sameBuildTile reports whether two build facts target the same map tile.
// Dedup only collapses repeat placements of the same building at the same
// spot (double-tap / misclick); two same-type buildings at *different* tiles
// are genuinely distinct and must both be observed, even when placed seconds
// apart. Positions are required — if either is unknown we treat the pair as
// distinct rather than collapse on a guess.
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
		// First-time match: stamp the second the rule flipped. Subsequent
		// calls (post-match) keep the original second so DetectedAtSecond
		// reflects the build-decision moment, not later observations.
		if !d.matched {
			d.matched = true
			d.detectedAtSecond = d.lastObservedSecond
		}
		// Don't SetFinished yet: the marker may have Expert milestones
		// further out (e.g. "First Zealot" at ~108s for 2 Gate, after the
		// 2nd-Gateway commit at ~86s). Stay alive until RuleDeadline so
		// observeRuleFact keeps appending to d.observed and ResolveExpert
		// can resolve them at GetResult-time.
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
	// If the rule already committed Matched during streaming, keep that
	// verdict + DetectedAtSecond from the original commit. Re-running
	// Finalize on a Matched state would be a no-op anyway (Matched is
	// terminal), but we deliberately bypass overwriting DetectedAtSecond
	// with replay-end / RuleDeadline. That overwrite is only correct for
	// late-finalized rules that resolved at the deadline (absence
	// markers, etc.), not for rules that committed early.
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
// gate is satisfied. Rule markers with Expert milestones emit a payload of
// position-aligned actual seconds (so the dashboard doesn't re-resolve on
// every read); other rule markers emit nil payload; Custom markers emit
// whatever their evaluator returned.
func (d *MarkerPlayerDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	if d.state != nil {
		var payload json.RawMessage
		if len(d.marker.Expert) > 0 {
			resolutions := d.marker.ResolveExpert(d.observed)
			if encoded, err := markers.EncodeExpertActuals(resolutions); err == nil {
				payload = encoded
			}
		}
		return d.BuildPlayerResult(d.marker.PatternName, d.detectedAtSecond, payload)
	}
	return d.BuildPlayerResult(d.marker.PatternName, d.detectedAtSecond, d.customResult.Payload)
}

// ShouldSave is true iff the marker matched AND any duration gate is met.
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
		// Compare against the player's effective time-in-game, not the
		// whole replay's duration. In non-1v1 games a player can leave
		// or stop playing long before the replay ends — using the
		// replay duration would wrongly flag e.g. "never upgraded" on
		// a player who quit at 5min in a 30min FFA, even though they
		// never had a chance to research/upgrade past the 10-min floor.
		// Last command second is more robust than leaveSec — some replays
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

// matchupGateMinSeconds is a hard lower bound applied on top of any
// per-(own_race, opp_race) gate from Marker.MinReplaySecondsByMatchup.
// Several Upgrade matchups have a progamer p5 of first-Upgrade around
// 2:00-3:00 (e.g. ZvZ Speed, PvT Singularity Charge). Used alone, those
// gates would flag "never upgraded" on successful 4-pool / Bunker rush /
// Proxy gate finishes — exactly the rush-suppression case the matchup
// gate was meant to preserve. 4 min is above the typical successful-rush
// game-end time, so short rushes stay suppressed; longer games still get
// the matchup-aware gate raised above this floor when applicable
// (e.g. PvT first-Tech p5 = 8:16).
const matchupGateMinSeconds = 4 * 60

// resolveMinReplaySeconds picks the effective duration gate for this marker.
// For 1v1 replays we prefer a per-(own_race, opp_race) entry from
// MinReplaySecondsByMatchup so "never X" markers respect matchup-typical
// first-research / first-upgrade timings, lifted to at least
// matchupGateMinSeconds to keep short rushes suppressed. For non-1v1 or
// a missing bucket we fall back to the flat MinReplaySeconds.
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
