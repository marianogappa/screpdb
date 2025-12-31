package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstMutaliskMorphTriggeredReplayDetector detects the seconds to first Mutalisk morph triggered in the replay
type SecondsToFirstMutaliskMorphTriggeredReplayDetector struct {
	replay   *models.Replay
	players  []*models.Player
	seconds  *int
	finished bool
}

// NewSecondsToFirstMutaliskMorphTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstMutaliskMorphTriggeredReplayDetector() *SecondsToFirstMutaliskMorphTriggeredReplayDetector {
	return &SecondsToFirstMutaliskMorphTriggeredReplayDetector{
		finished: false,
	}
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) Name() string {
	return "Seconds to First Mutalisk Morph Triggered"
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a Mutalisk morph command
	if command.ActionType == models.ActionTypeUnitMorph &&
		command.UnitType != nil && *command.UnitType == models.GeneralUnitMutalisk {
		seconds := command.SecondsFromGameStart
		d.seconds = &seconds
		d.finished = true
		return true
	}

	return false
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstMutaliskMorphTriggeredReplayDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

