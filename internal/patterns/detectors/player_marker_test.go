package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/models"
)

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
	// 9 drones before pool, no overlord, pool at 73.
	for i := 0; i < 9; i++ {
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
