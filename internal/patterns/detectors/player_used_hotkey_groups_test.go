package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func TestUsedHotkeyGroupsPlayerDetector(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "Player1", "Terran", 1).
		WithPlayer(2, "Player2", "Zerg", 2).
		WithDurationSeconds(secondsSevenMinutes)
	replay, players := builder.Build()

	detector := NewUsedHotkeyGroupsPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)

	// Player 1 uses hotkeys: 3, 1, 3
	c1 := builder.WithCommand(1, 10, "Hotkey", "").GetCommands()[0]
	g1 := byte(3)
	c1.HotkeyGroup = &g1

	c2 := builder.WithCommand(1, 20, "Hotkey", "").GetCommands()[1]
	g2 := byte(1)
	c2.HotkeyGroup = &g2

	c3 := builder.WithCommand(1, 30, "Hotkey", "").GetCommands()[2]
	g3 := byte(3)
	c3.HotkeyGroup = &g3

	// Another player hotkey should be ignored.
	c4 := builder.WithCommand(2, 40, "Hotkey", "").GetCommands()[3]
	g4 := byte(9)
	c4.HotkeyGroup = &g4

	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}

	// End-of-replay finalize phase.
	detector.Finalize()

	if !detector.IsFinished() {
		t.Fatalf("expected detector to be finished after finalize")
	}
	if !detector.ShouldSave() {
		t.Fatalf("expected detector.ShouldSave() to be true")
	}
	result := detector.GetResult()
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.Level != core.LevelPlayer {
		t.Fatalf("unexpected level: %v", result.Level)
	}
	if result.ValueString == nil || *result.ValueString != "1,3" {
		t.Fatalf("unexpected value string: %v", result.ValueString)
	}
}

func TestNeverUsedHotkeysPlayerDetector(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "Player1", "Terran", 1).
		WithDurationSeconds(secondsSevenMinutes)
	replay, players := builder.Build()

	detector := NewNeverUsedHotkeysPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected never used hotkeys detection")
	}
}

func TestNeverUsedHotkeysPlayerDetector_RequiresMinimumDuration(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "Player1", "Terran", 1).
		WithDurationSeconds(secondsSevenMinutes - 1)
	replay, players := builder.Build()

	detector := NewNeverUsedHotkeysPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatalf("did not expect never used hotkeys detection for sub-7-minute replay")
	}
}

func TestNeverUsedHotkeysPlayerDetector_SkipsWhenHotkeysWereUsed(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "Player1", "Terran", 1).
		WithDurationSeconds(secondsSevenMinutes).
		WithCommand(1, 30, "Hotkey", "")
	replay, players := builder.Build()

	hotkeyCommand := builder.GetCommands()[0]
	group := byte(4)
	hotkeyCommand.HotkeyGroup = &group

	detector := NewNeverUsedHotkeysPlayerDetector()
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatalf("did not expect never used hotkeys detection when a hotkey was used")
	}
}
