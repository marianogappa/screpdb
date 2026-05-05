package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/phases"
)

// Phase-boundary detectors emit replay-level markers carrying the second
// at which mid-game and late-game start. They exist so request-time
// composition computation (per-game dashboard endpoint) can read fixed
// thresholds from replay_events instead of re-walking the whole command
// stream — and so future consumers (per-user view aggregations,
// downstream analyses) get a single source of truth for the split.
//
// These are NOT user-visible "signature" markers: the registry has no
// Pill defined for them, so renderPatternPill yields nothing. They live
// only to be queried by feature code that needs the boundaries.
//
// Boundary algorithm: phases.Compute (internal/patterns/phases) — same
// thresholds the per-game events list already uses (first
// Muta/Lurker/Wraith/SiegeArmed/DragoonArmed for Early-end; first
// Defiler/Arbiter/Carrier/BC/Ultra/TerranPlus2 for Mid-end). When a
// boundary is not detected (game ends before reaching it) the
// corresponding detector emits no row.

// PatternName / FeatureKey / event_type strings persisted in
// replay_events. Stable identifiers: changing requires an
// AlgorithmVersion bump and re-ingest.
const (
	PhaseBoundaryMidGameStartsEventType  = "mid_game_starts"
	PhaseBoundaryLateGameStartsEventType = "late_game_starts"
)

// boundaryPicker selects either the early-end or mid-end second from
// (earlyEnd, midEnd). Lets the two detector instances share the same
// struct without runtime branching.
type boundaryPicker func(earlyEnd, midEnd int) int

// PhaseBoundaryDetector is a replay-level detector that emits a single
// marker row carrying the boundary second in
// replay_events.seconds_from_game_start.
type PhaseBoundaryDetector struct {
	BaseReplayDetector
	eventType string
	pick      boundaryPicker

	matched bool
	second  int
}

// NewMidGameStartsDetector emits "mid_game_starts" at second = earlyEnd.
func NewMidGameStartsDetector() core.Detector {
	return &PhaseBoundaryDetector{
		eventType: PhaseBoundaryMidGameStartsEventType,
		pick:      func(earlyEnd, _ int) int { return earlyEnd },
	}
}

// NewLateGameStartsDetector emits "late_game_starts" at second = midEnd.
func NewLateGameStartsDetector() core.Detector {
	return &PhaseBoundaryDetector{
		eventType: PhaseBoundaryLateGameStartsEventType,
		pick:      func(_, midEnd int) int { return midEnd },
	}
}

func (d *PhaseBoundaryDetector) Name() string { return d.eventType }

// ProcessCommand is a no-op: phase boundaries derive from the full
// per-replay enriched stream, which we read once at Finalize via the
// orchestrator-owned worldstate engine. Mirrors mutaTimingEvaluator's
// approach.
func (d *PhaseBoundaryDetector) ProcessCommand(_ *models.Command) bool { return false }

func (d *PhaseBoundaryDetector) Finalize() {
	d.SetFinished(true)
	ws := d.GetWorldState()
	if ws == nil {
		return
	}
	earlyEnd, midEnd := phases.Compute(ws.EnrichedStream())
	sec := d.pick(earlyEnd, midEnd)
	if sec > 0 {
		d.matched = true
		d.second = sec
	}
}

func (d *PhaseBoundaryDetector) ShouldSave() bool {
	return d.IsFinished() && d.matched
}

func (d *PhaseBoundaryDetector) GetResult() *core.PatternResult {
	if !d.ShouldSave() {
		return nil
	}
	// Replay-level result: nil PlayerID → persists with source_player_id IS NULL.
	return d.BuildReplayResult(d.eventType, d.second, nil)
}
