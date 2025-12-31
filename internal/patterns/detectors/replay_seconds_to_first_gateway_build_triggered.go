package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstGatewayBuildTriggeredReplayDetector detects the seconds to first Gateway build triggered in the replay
type SecondsToFirstGatewayBuildTriggeredReplayDetector struct {
	replay   *models.Replay
	players  []*models.Player
	seconds  *int
	finished bool
}

// NewSecondsToFirstGatewayBuildTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstGatewayBuildTriggeredReplayDetector() *SecondsToFirstGatewayBuildTriggeredReplayDetector {
	return &SecondsToFirstGatewayBuildTriggeredReplayDetector{
		finished: false,
	}
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) Name() string {
	return "Seconds to First Gateway Build Triggered"
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a Gateway build command
	if command.ActionType == models.ActionTypeBuild &&
		command.UnitType != nil && *command.UnitType == models.GeneralUnitGateway {
		seconds := command.SecondsFromGameStart
		d.seconds = &seconds
		d.finished = true
		return true
	}

	return false
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstGatewayBuildTriggeredReplayDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

