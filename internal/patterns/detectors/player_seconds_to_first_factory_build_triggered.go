package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstFactoryBuildTriggeredPlayerDetector detects the seconds to first Factory build triggered for a player
type SecondsToFirstFactoryBuildTriggeredPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	seconds        *int
	finished       bool
}

// NewSecondsToFirstFactoryBuildTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstFactoryBuildTriggeredPlayerDetector() *SecondsToFirstFactoryBuildTriggeredPlayerDetector {
	return &SecondsToFirstFactoryBuildTriggeredPlayerDetector{
		finished: false,
	}
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) Name() string {
	return "Seconds to First Factory Build Triggered"
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
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

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) GetResult() *core.PatternResult {
	// Only return a result if we detected the command
	if !d.finished || d.seconds == nil {
		return nil
	}

	replayPlayerID := d.replayPlayerID
	return &core.PatternResult{
		PatternName:    d.Name(),
		Level:          d.Level(),
		ReplayID:       d.replay.ID,
		PlayerID:       nil, // Will be set when converting to database IDs
		ReplayPlayerID: &replayPlayerID,
		ValueInt:       d.seconds,
	}
}

func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *SecondsToFirstFactoryBuildTriggeredPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}
