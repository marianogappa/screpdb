package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstFactoryBuildTriggeredReplayDetector detects the seconds to first Factory build triggered in the replay
type SecondsToFirstFactoryBuildTriggeredReplayDetector struct {
	replay   *models.Replay
	players  []*models.Player
	seconds  *int
	finished bool
}

// NewSecondsToFirstFactoryBuildTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstFactoryBuildTriggeredReplayDetector() *SecondsToFirstFactoryBuildTriggeredReplayDetector {
	return &SecondsToFirstFactoryBuildTriggeredReplayDetector{
		finished: false,
	}
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) Name() string {
	return "Seconds to First Factory Build Triggered"
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a Factory build command
	if command.ActionType == models.ActionTypeBuild &&
		command.UnitType != nil && *command.UnitType == models.GeneralUnitFactory {
		seconds := command.SecondsFromGameStart
		d.seconds = &seconds
		d.finished = true
		return true
	}

	return false
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstFactoryBuildTriggeredReplayDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

