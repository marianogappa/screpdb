package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstSpawningPoolMorphTriggeredReplayDetector detects the seconds to first Spawning Pool morph triggered in the replay
type SecondsToFirstSpawningPoolMorphTriggeredReplayDetector struct {
	replay   *models.Replay
	players  []*models.Player
	seconds  *int
	finished bool
}

// NewSecondsToFirstSpawningPoolMorphTriggeredReplayDetector creates a new replay-level detector
func NewSecondsToFirstSpawningPoolMorphTriggeredReplayDetector() *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector {
	return &SecondsToFirstSpawningPoolMorphTriggeredReplayDetector{
		finished: false,
	}
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) Name() string {
	return "Seconds to First Spawning Pool Morph Triggered"
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a Spawning Pool build command (it's built, not morphed)
	if command.ActionType == models.ActionTypeBuild &&
		command.UnitType != nil && *command.UnitType == models.GeneralUnitSpawningPool {
		seconds := command.SecondsFromGameStart
		d.seconds = &seconds
		d.finished = true
		return true
	}

	return false
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstSpawningPoolMorphTriggeredReplayDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

