package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// runPhaseBoundary drives a phase-boundary detector over a worldstate built
// from builder's commands and returns the result (nil when nothing saved).
func runPhaseBoundary(t *testing.T, d *PhaseBoundaryDetector, builder *TestReplayBuilder) *core.PatternResult {
	t.Helper()
	replay, players := builder.Build()
	ws := worldstate.NewEngine(replay, players, &models.ReplayMapContext{})
	for _, cmd := range builder.GetCommands() {
		ws.ProcessCommand(cmd)
	}
	ws.Finalize()

	d.SetWorldState(ws)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		if d.ProcessCommand(cmd) {
			t.Fatalf("ProcessCommand should be a no-op returning false")
		}
	}
	d.Finalize()
	return d.GetResult()
}

func phaseBuilder() *TestReplayBuilder {
	// Mutalisk completion → early-game end signal.
	// Carrier completion → mid-game end signal.
	return NewTestReplayBuilder().
		WithPlayer(1, "Z", "Zerg", 1).
		WithPlayer(2, "P", "Protoss", 2).
		WithDurationSeconds(1200).
		WithCommand(1, 300, models.ActionTypeTrain, models.GeneralUnitMutalisk).
		WithCommand(2, 700, models.ActionTypeTrain, models.GeneralUnitCarrier)
}

func TestPhaseBoundary_MidGameStarts_EmitsEarlyEnd(t *testing.T) {
	d := NewMidGameStartsDetector().(*PhaseBoundaryDetector)
	res := runPhaseBoundary(t, d, phaseBuilder())
	if res == nil {
		t.Fatalf("expected a mid_game_starts row")
	}
	if res.PatternName != PhaseBoundaryMidGameStartsEventType {
		t.Fatalf("unexpected pattern name %q", res.PatternName)
	}
	if res.Level != core.LevelReplay {
		t.Fatalf("expected replay-level result, got %v", res.Level)
	}
	if res.ReplayPlayerID != nil {
		t.Fatalf("expected nil ReplayPlayerID (replay-level), got %v", res.ReplayPlayerID)
	}
	if res.DetectedAtSecond != 300 {
		t.Fatalf("expected earlyEnd second 300 (first Mutalisk), got %d", res.DetectedAtSecond)
	}
}

func TestPhaseBoundary_LateGameStarts_EmitsMidEnd(t *testing.T) {
	d := NewLateGameStartsDetector().(*PhaseBoundaryDetector)
	res := runPhaseBoundary(t, d, phaseBuilder())
	if res == nil {
		t.Fatalf("expected a late_game_starts row")
	}
	if res.PatternName != PhaseBoundaryLateGameStartsEventType {
		t.Fatalf("unexpected pattern name %q", res.PatternName)
	}
	if res.DetectedAtSecond != 700 {
		t.Fatalf("expected midEnd second 700 (first Carrier), got %d", res.DetectedAtSecond)
	}
}

// No early/mid signal in the stream → both detectors finish but emit nothing.
func TestPhaseBoundary_NoSignalEmitsNothing(t *testing.T) {
	build := func() *TestReplayBuilder {
		return NewTestReplayBuilder().
			WithPlayer(1, "Z", "Zerg", 1).
			WithDurationSeconds(600).
			WithCommand(1, 100, models.ActionTypeTrain, models.GeneralUnitZergling)
	}

	mid := NewMidGameStartsDetector().(*PhaseBoundaryDetector)
	if res := runPhaseBoundary(t, mid, build()); res != nil {
		t.Fatalf("expected no mid_game_starts row without an early signal, got %+v", res)
	}
	if !mid.IsFinished() {
		t.Fatalf("expected detector finished after Finalize")
	}
	if mid.ShouldSave() {
		t.Fatalf("expected ShouldSave false without a boundary")
	}

	late := NewLateGameStartsDetector().(*PhaseBoundaryDetector)
	if res := runPhaseBoundary(t, late, build()); res != nil {
		t.Fatalf("expected no late_game_starts row without a mid signal, got %+v", res)
	}
}

// Finalize with no worldstate wired must not panic and must emit nothing.
func TestPhaseBoundary_NilWorldStateEmitsNothing(t *testing.T) {
	d := NewMidGameStartsDetector()
	d.Initialize(&models.Replay{ID: 1, DurationSeconds: 600}, nil)
	d.Finalize()
	if d.ShouldSave() {
		t.Fatalf("expected ShouldSave false with nil worldstate")
	}
	if d.GetResult() != nil {
		t.Fatalf("expected nil result with nil worldstate")
	}
	if d.Name() != PhaseBoundaryMidGameStartsEventType {
		t.Fatalf("unexpected name %q", d.Name())
	}
}

func TestPhaseBoundary_Level(t *testing.T) {
	if got := NewMidGameStartsDetector().Level(); got != core.LevelReplay {
		t.Fatalf("expected replay level, got %v", got)
	}
}
