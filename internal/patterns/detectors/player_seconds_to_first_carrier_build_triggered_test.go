package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func TestSecondsToFirstCarrierBuildTriggeredPlayerDetector(t *testing.T) {
	tests := []struct {
		name           string
		playerID       byte
		commands       []commandSpec // unit type -> seconds
		wantFinished   bool
		wantResult     *int
		wantShouldSave bool
	}{
		{
			name:     "detects first carrier train at 120 seconds",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 120},
			},
			wantFinished:   true,
			wantResult:     intPtr(120),
			wantShouldSave: true,
		},
		{
			name:     "ignores other player's carrier",
			playerID: 1,
			commands: []commandSpec{
				{playerID: 2, action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 120},
			},
			wantFinished:   false,
			wantResult:     nil,
			wantShouldSave: false,
		},
		{
			name:     "ignores other unit types",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeTrain, unit: "Zealot", seconds: 120},
			},
			wantFinished:   false,
			wantResult:     nil,
			wantShouldSave: false,
		},
		{
			name:     "ignores other action types",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeBuild, unit: models.GeneralUnitCarrier, seconds: 120},
			},
			wantFinished:   false,
			wantResult:     nil,
			wantShouldSave: false,
		},
		{
			name:     "takes first occurrence when multiple carriers",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 100},
				{action: models.ActionTypeTrain, unit: models.GeneralUnitCarrier, seconds: 200},
			},
			wantFinished:   true,
			wantResult:     intPtr(100),
			wantShouldSave: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewTestReplayBuilder().
				WithPlayer(tt.playerID, "Player1", "Protoss", 1).
				WithPlayer(2, "Player2", "Protoss", 2)

			for _, cmd := range tt.commands {
				playerID := cmd.playerID
				if playerID == 0 {
					playerID = tt.playerID
				}
				builder.WithCommand(playerID, cmd.seconds, cmd.action, cmd.unit)
			}

			replay, players := builder.Build()
			detector := NewSecondsToFirstCarrierBuildTriggeredPlayerDetector()
			detector.SetReplayPlayerID(tt.playerID)
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
				// Verify result structure
				if result.PatternName != detector.Name() {
					t.Errorf("GetResult().PatternName = %v, want %v", result.PatternName, detector.Name())
				}
				if result.Level != core.LevelPlayer {
					t.Errorf("GetResult().Level = %v, want %v", result.Level, core.LevelPlayer)
				}
			}

			// Check ShouldSave
			if detector.ShouldSave() != tt.wantShouldSave {
				t.Errorf("ShouldSave() = %v, want %v", detector.ShouldSave(), tt.wantShouldSave)
			}
		})
	}
}

type commandSpec struct {
	playerID byte
	action   string
	unit     string
	seconds  int
}

func intPtr(i int) *int {
	return &i
}
