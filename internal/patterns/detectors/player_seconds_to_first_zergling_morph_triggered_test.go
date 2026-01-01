package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

func TestSecondsToFirstZerglingMorphTriggeredPlayerDetector(t *testing.T) {
	tests := []struct {
		name           string
		playerID       byte
		commands       []commandSpec
		wantFinished   bool
		wantResult     *int
		wantShouldSave bool
	}{
		{
			name:     "detects first zergling morph at 45 seconds",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeUnitMorph, unit: models.GeneralUnitZergling, seconds: 45},
			},
			wantFinished:   true,
			wantResult:     intPtr(45),
			wantShouldSave: true,
		},
		{
			name:     "ignores build action",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeBuild, unit: models.GeneralUnitZergling, seconds: 45},
			},
			wantFinished:   false,
			wantResult:     nil,
			wantShouldSave: false,
		},
		{
			name:     "takes first occurrence when multiple zerglings",
			playerID: 1,
			commands: []commandSpec{
				{action: models.ActionTypeUnitMorph, unit: models.GeneralUnitZergling, seconds: 40},
				{action: models.ActionTypeUnitMorph, unit: models.GeneralUnitZergling, seconds: 50},
			},
			wantFinished:   true,
			wantResult:     intPtr(40),
			wantShouldSave: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewTestReplayBuilder().
				WithPlayer(tt.playerID, "Player1", "Zerg", 1)

			for _, cmd := range tt.commands {
				playerID := cmd.playerID
				if playerID == 0 {
					playerID = tt.playerID
				}
				builder.WithCommand(playerID, cmd.seconds, cmd.action, cmd.unit)
			}

			replay, players := builder.Build()
			detector := NewSecondsToFirstZerglingMorphTriggeredPlayerDetector()
			detector.SetReplayPlayerID(tt.playerID)
			detector.Initialize(replay, players)

			for _, cmd := range builder.GetCommands() {
				detector.ProcessCommand(cmd)
			}

			if detector.IsFinished() != tt.wantFinished {
				t.Errorf("IsFinished() = %v, want %v", detector.IsFinished(), tt.wantFinished)
			}

			result := detector.GetResult()
			if tt.wantResult == nil {
				if result != nil {
					t.Errorf("GetResult() = %v, want nil", result)
				}
			} else {
				if result == nil || result.ValueInt == nil || *result.ValueInt != *tt.wantResult {
					t.Errorf("GetResult().ValueInt = %v, want %d", result, *tt.wantResult)
				}
				if result.Level != core.LevelPlayer {
					t.Errorf("GetResult().Level = %v, want %v", result.Level, core.LevelPlayer)
				}
			}

			if detector.ShouldSave() != tt.wantShouldSave {
				t.Errorf("ShouldSave() = %v, want %v", detector.ShouldSave(), tt.wantShouldSave)
			}
		})
	}
}


