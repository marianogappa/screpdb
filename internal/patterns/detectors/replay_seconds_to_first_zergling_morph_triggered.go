package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstZerglingMorphTriggeredReplayDetector detects the seconds to first Zergling morph triggered in the replay
type SecondsToFirstZerglingMorphTriggeredReplayDetector struct {
	replay   *models.Replay
	players  []*models.Player
	seconds  *int
	finished bool
}

// NewSecondsToFirstZerglingMorphTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstZerglingMorphTriggeredReplayDetector() *SecondsToFirstZerglingMorphTriggeredReplayDetector {
	return &SecondsToFirstZerglingMorphTriggeredReplayDetector{
		finished: false,
	}
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) Name() string {
	return "Seconds to First Zergling Morph Triggered"
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a Zergling morph command
	if command.ActionType == models.ActionTypeUnitMorph &&
		command.UnitType != nil && *command.UnitType == models.GeneralUnitZergling {
		seconds := command.SecondsFromGameStart
		d.seconds = &seconds
		d.finished = true
		return true
	}

	return false
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) GetResult() *core.PatternResult {
	// Only return a result if we detected the command
	if !d.finished || d.seconds == nil {
		return nil
	}

	return &core.PatternResult{
		PatternName: d.Name(),
		Level:       d.Level(),
		ReplayID:    d.replay.ID,
		ValueInt:    d.seconds,
	}
}

func (d *SecondsToFirstZerglingMorphTriggeredReplayDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

