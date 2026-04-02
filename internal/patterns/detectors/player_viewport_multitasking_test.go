package detectors

import (
	"strconv"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestViewportMultitaskingPlayerDetector(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "Tester", "Protoss", 1)
	builder.WithCommand(1, 410, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 430, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 440, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 450, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 460, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay, players := builder.Build()
	replay.DurationSeconds = 1000
	commands := builder.GetCommands()
	commands[0].X = intPtr(0)
	commands[0].Y = intPtr(0)
	commands[1].X = intPtr(100)
	commands[1].Y = intPtr(100)
	commands[2].X = intPtr(950)
	commands[2].Y = intPtr(120)
	commands[3].X = intPtr(980)
	commands[3].Y = intPtr(150)
	commands[4].X = intPtr(1000)
	commands[4].Y = intPtr(170)

	detector := NewViewportMultitaskingPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, command := range commands {
		detector.ProcessCommand(command)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatal("expected viewport multitasking detector to save")
	}
	result := detector.GetResult()
	if result == nil || result.ValueString == nil {
		t.Fatalf("expected string payload result, got %+v", result)
	}

	value, err := strconv.ParseFloat(*result.ValueString, 64)
	if err != nil {
		t.Fatalf("parse switch rate: %v", err)
	}
	if value <= 0.15 || value >= 0.16 {
		t.Fatalf("expected switch rate around 0.158, got %v", value)
	}
}

func TestViewportMultitaskingPlayerDetectorSkipsShortWindow(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "Tester", "Protoss", 1).
		WithCommand(1, 430, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay, players := builder.Build()
	replay.DurationSeconds = 500
	commands := builder.GetCommands()
	commands[0].X = intPtr(0)
	commands[0].Y = intPtr(0)

	detector := NewViewportMultitaskingPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, command := range commands {
		detector.ProcessCommand(command)
	}
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatal("expected short replay window to be ignored")
	}
}
