package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func TestSecondsToFirstCarrierBuildTriggeredReplayDetector(t *testing.T) {
	tests := []struct {
		name           string
		commands       []commandSpec // unit type -> seconds
		wantFinished   bool
		wantResult     *int
		wantShouldSave bool
	}{
		{
			name: "detects first carrier train from any player at 120 seconds",
			commands: []commandSpec{
				{playerID: 1, action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 120},
			},
			wantFinished:   true,
			wantResult:     intPtr(120),
			wantShouldSave: true,
		},
		{
			name: "takes first occurrence across multiple players",
			commands: []commandSpec{
				{playerID: 2, action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 100},
				{playerID: 1, action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 200},
			},
			wantFinished:   true,
			wantResult:     intPtr(100),
			wantShouldSave: true,
		},
		{
			name: "ignores other unit types",
			commands: []commandSpec{
				{playerID: 1, action: models.ActionTypeTrain, unit: "Zealot", seconds: 120},
			},
			wantFinished:   false,
			wantResult:     nil,
			wantShouldSave: false,
		},
		{
			name: "ignores other action types",
			commands: []commandSpec{
				{playerID: 1, action: models.ActionTypeBuild, unit: models.GeneralUnitCarrier, seconds: 120},
			},
			wantFinished:   false,
			wantResult:     nil,
			wantShouldSave: false,
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
			detector := NewSecondsToFirstCarrierBuildTriggeredReplayDetector()
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
			if tt.wantResult == nil {
				if result != nil {
					t.Errorf("GetResult() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("GetResult() = nil, want result with ValueInt = %d", *tt.wantResult)
				} else if result.ValueInt == nil || *result.ValueInt != *tt.wantResult {
					t.Errorf("GetResult().ValueInt = %v, want %d", result.ValueInt, *tt.wantResult)
				}
				if result.PatternName != detector.Name() {
					t.Errorf("GetResult().PatternName = %v, want %v", result.PatternName, detector.Name())
				}
				if result.Level != core.LevelReplay {
					t.Errorf("GetResult().Level = %v, want %v", result.Level, core.LevelReplay)
				}
			}

			// Check ShouldSave
			if detector.ShouldSave() != tt.wantShouldSave {
				t.Errorf("ShouldSave() = %v, want %v", detector.ShouldSave(), tt.wantShouldSave)
			}
		})
	}
}

