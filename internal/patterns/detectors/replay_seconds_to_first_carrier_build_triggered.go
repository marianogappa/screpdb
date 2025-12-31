package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstCarrierBuildTriggeredReplayDetector detects the seconds to first Carrier build triggered in the replay
type SecondsToFirstCarrierBuildTriggeredReplayDetector struct {
	replay   *models.Replay
	players  []*models.Player
	seconds  *int
	finished bool
}

// NewSecondsToFirstCarrierBuildTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstCarrierBuildTriggeredReplayDetector() *SecondsToFirstCarrierBuildTriggeredReplayDetector {
	return &SecondsToFirstCarrierBuildTriggeredReplayDetector{
		finished: false,
	}
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) Name() string {
	return "Seconds to First Carrier Build Triggered"
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a Carrier train command
	if command.ActionType == models.ActionTypeTrain &&
		command.UnitType != nil && *command.UnitType == models.GeneralUnitCarrier {
		seconds := command.SecondsFromGameStart
		d.seconds = &seconds
		d.finished = true
		return true
	}

	return false
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstCarrierBuildTriggeredReplayDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

