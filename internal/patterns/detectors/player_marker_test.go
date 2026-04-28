package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/models"
)

// recordingState is a test PredicateState that captures every observed fact.
// Stays Pending forever so the detector drives the full stream through
// enqueueDedup / flushDedupBefore without early-committing.
type recordingState struct {
	observed []cmdenrich.EnrichedCommand
}

func (s *recordingState) Observe(f cmdenrich.EnrichedCommand) {
	s.observed = append(s.observed, f)
}
func (s *recordingState) Decision(int) markers.TriState { return markers.Pending }
func (s *recordingState) Finalize() markers.TriState    { return markers.Rejected }

func findBOForTest(t *testing.T, name string) markers.Marker {
	t.Helper()
	for _, bo := range markers.Markers() {
		if bo.Name == name {
			return bo
		}
	}
	t.Fatalf("BO %q not registered", name)
	return markers.Marker{}
}

func TestMarkerDetector_9Pool_Positive(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "Z", "Zerg", 1)
	// 9 Pool BO: 5 drone morphs (4 starting + 5 = 9 supply), no Overlord
	// before Pool, Pool placed at second 73.
	for i := 0; i < 5; i++ {
		builder.WithCommand(1, 5+i*3, models.ActionTypeUnitMorph, models.GeneralUnitDrone)
	}
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool)
	builder.WithCommand(1, 125, models.ActionTypeUnitMorph, models.GeneralUnitZergling)
	replay, players := builder.Build()

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected 9 pool detection to save")
	}
	result := detector.GetResult()
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.PatternName != "Build Order: 9 Pool" {
		t.Fatalf("unexpected pattern name: %q", result.PatternName)
	}
}

func TestMarkerDetector_9Pool_NegativeWhenOverlordBeforePool(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "Z", "Zerg", 1)
	builder.WithCommand(1, 10, models.ActionTypeUnitMorph, models.GeneralUnitDrone)
	builder.WithCommand(1, 30, models.ActionTypeUnitMorph, models.GeneralUnitOverlord) // disqualifying
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool)
	replay, players := builder.Build()

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatalf("expected 9 pool to NOT match when overlord produced before pool")
	}
}

func TestMarkerDetector_SkipsOtherRaces(t *testing.T) {
	// A Protoss player should not trigger the Zerg 9 pool detector.
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1)
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool) // nonsensical but proves race gating
	replay, players := builder.Build()

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatalf("Protoss player must not match a Zerg BO")
	}
}

// Dedup still collapses same-subject Build facts within the 3s gap during the
// opening window (pre-BuildDedupMaxSecond). Only the latest observation lands.
func TestBuildDedup_WithinOpeningCollapses(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFact("Gateway", 70))
	d.enqueueDedup(buildFact("Gateway", 72)) // inside gap → replaces pending
	d.flushAllPending()

	if got := len(rec.observed); got != 1 {
		t.Fatalf("expected 1 observation (dedup), got %d", got)
	}
	if rec.observed[0].Second != 72 {
		t.Fatalf("expected latest-wins at second=72, got %d", rec.observed[0].Second)
	}
}

// Past BuildDedupMaxSecond (240s), dedup is off: every same-subject Build fact
// reaches the predicate even when they arrive within the 3s gap.
func TestBuildDedup_PastCapDoesNotCollapse(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFact("Gateway", 300))
	d.enqueueDedup(buildFact("Gateway", 301)) // 1s apart but past cap → both land

	if got := len(rec.observed); got != 2 {
		t.Fatalf("expected 2 observations past cap, got %d", got)
	}
}

// A pending entry from before the cap must flush (as its own observation) the
// moment the first post-cap fact arrives, so nothing is silently dropped.
func TestBuildDedup_PendingFromBeforeCapFlushesAtCap(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFact("Gateway", 238))  // pending, pre-cap
	d.enqueueDedup(buildFact("Gateway", 240))  // at cap → flushes prior, observes self

	if got := len(rec.observed); got != 2 {
		t.Fatalf("expected 2 observations (pre-cap pending flushed + post-cap), got %d", got)
	}
	if rec.observed[0].Second != 238 || rec.observed[1].Second != 240 {
		t.Fatalf("unexpected order: %+v", rec.observed)
	}
}

func newRecordingBuildDetector() (*MarkerPlayerDetector, *recordingState) {
	rec := &recordingState{}
	d := &MarkerPlayerDetector{
		state:   rec,
		pending: map[string]cmdenrich.EnrichedCommand{},
	}
	return d, rec
}

func buildFact(subject string, second int) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{
		Kind:    cmdenrich.KindMakeBuilding,
		Subject: subject,
		Second:  second,
	}
}

func TestMarkerDetector_2Gate_Positive(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1)
	builder.WithCommand(1, 70, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 130, models.ActionTypeTrain, "Zealot")
	replay, players := builder.Build()

	bo := findBOForTest(t, "2 Gate")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected 2 gate detection to save")
	}
}

// Ensures the persisted payload carries Expert milestone seconds in
// position-aligned order with bo.Expert. The dashboard now reads this
// payload directly instead of re-resolving on every page load.
func TestMarkerDetector_2Gate_PayloadHasExpertActuals(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1)
	// Anti-spam double-click on the 1st Gateway: two Build commands within
	// BuildDedupGapSeconds. The detector should dedup → keep the later one,
	// then a real 2nd Gateway, then a Zealot. Payload should reflect the
	// post-dedup timings, not the raw Build stream.
	builder.WithCommand(1, 70, models.ActionTypeBuild, models.GeneralUnitGateway) // dropped by dedup
	builder.WithCommand(1, 71, models.ActionTypeBuild, models.GeneralUnitGateway) // wins dedup window → 1st Gateway @ 71
	builder.WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway) // 2nd Gateway @ 86
	builder.WithCommand(1, 130, models.ActionTypeTrain, "Zealot")                 // First Zealot @ 130
	replay, players := builder.Build()

	bo := findBOForTest(t, "2 Gate")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	result := detector.GetResult()
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if len(result.Payload) == 0 {
		t.Fatalf("expected payload with expert_actuals, got empty")
	}
	actuals := markers.DecodeExpertActuals(result.Payload)
	if len(actuals) != len(bo.Expert) {
		t.Fatalf("expected %d expert actuals (one per bo.Expert), got %d", len(bo.Expert), len(actuals))
	}
	// bo.Expert order: 1st Gateway, 2nd Gateway, First Zealot.
	if !actuals[0].Found || actuals[0].Second != 71 {
		t.Fatalf("1st Gateway actual wrong (expected found@71): %+v", actuals[0])
	}
	if !actuals[1].Found || actuals[1].Second != 86 {
		t.Fatalf("2nd Gateway actual wrong (expected found@86): %+v", actuals[1])
	}
	if !actuals[2].Found || actuals[2].Second != 130 {
		t.Fatalf("First Zealot actual wrong (expected found@130): %+v", actuals[2])
	}
}
