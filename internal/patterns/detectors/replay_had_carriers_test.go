package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func TestHadCarriersReplayDetector(t *testing.T) {
	tests := []struct {
		name           string
		commands       []commandSpec // unit type -> seconds
		wantFinished   bool
		wantResult     bool // true if should return result
		wantShouldSave bool
	}{
		{
			name: "detects 3 carriers from any player",
			commands: []commandSpec{
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 100},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 200},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 300},
			},
			wantFinished:   true,
			wantResult:     true,
			wantShouldSave: true,
		},
		{
			name: "detects 3 carriers from single player (not across multiple)",
			commands: []commandSpec{
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 100},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 200},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 300},
			},
			wantFinished:   true,
			wantResult:     true,
			wantShouldSave: true,
		},
		{
			name: "does not detect with only 2 carriers",
			commands: []commandSpec{
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 100},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 200},
			},
			wantFinished:   false,
			wantResult:     false,
			wantShouldSave: false,
		},
		{
			name: "ignores other unit types",
			commands: []commandSpec{
				{playerID: 1, action: "Train", unit: "Zealot", seconds: 100},
				{playerID: 1, action: "Train", unit: "Zealot", seconds: 200},
				{playerID: 1, action: "Train", unit: "Zealot", seconds: 300},
			},
			wantFinished:   false,
			wantResult:     false,
			wantShouldSave: false,
		},
		{
			name: "finishes on third carrier from single player",
			commands: []commandSpec{
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 100},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 200},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 300},
				{playerID: 1, action: "Train", unit: "Carrier", seconds: 400}, // Should not be processed
			},
			wantFinished:   true,
			wantResult:     true,
			wantShouldSave: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewTestReplayBuilder().
				WithPlayer(1, "Player1", "Protoss", 1).
				WithPlayer(2, "Player2", "Protoss", 2)

			for _, cmd := range tt.commands {
				if cmd.playerID == 0 {
					cmd.playerID = 1
				}
				builder.WithCommand(cmd.playerID, cmd.seconds, cmd.action, cmd.unit)
			}

			replay, players := builder.Build()
			detector := NewHadCarriersReplayDetector()
			detector.Initialize(replay, players)

			// Process commands
			for _, cmd := range builder.GetCommands() {
				detector.ProcessCommand(cmd)
			}

			// Check finished state
			if detector.IsFinished() != tt.wantFinished {
				t.Errorf("IsFinished() = %v, want %v", detector.IsFinished(), tt.wantFinished)
			}

			// Check result
			result := detector.GetResult()
			if tt.wantResult {
				if result == nil {
					t.Errorf("GetResult() = nil, want result")
				} else {
					if result.ValueBool == nil || !*result.ValueBool {
						t.Errorf("GetResult().ValueBool = %v, want true", result.ValueBool)
					}
					if result.PatternName != detector.Name() {
						t.Errorf("GetResult().PatternName = %v, want %v", result.PatternName, detector.Name())
					}
					if result.Level != core.LevelReplay {
						t.Errorf("GetResult().Level = %v, want %v", result.Level, core.LevelReplay)
					}
				}
			} else {
				if result != nil {
					t.Errorf("GetResult() = %v, want nil", result)
				}
			}

			// Check ShouldSave
			if detector.ShouldSave() != tt.wantShouldSave {
				t.Errorf("ShouldSave() = %v, want %v", detector.ShouldSave(), tt.wantShouldSave)
			}
		})
	}
}

