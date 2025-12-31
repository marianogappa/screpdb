package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstGatewayBuildTriggeredPlayerDetector detects the seconds to first Gateway build triggered for a player
type SecondsToFirstGatewayBuildTriggeredPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	seconds        *int
	finished       bool
}

// NewSecondsToFirstGatewayBuildTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstGatewayBuildTriggeredPlayerDetector() *SecondsToFirstGatewayBuildTriggeredPlayerDetector {
	return &SecondsToFirstGatewayBuildTriggeredPlayerDetector{
		finished: false,
	}
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) Name() string {
	return "Seconds to First Gateway Build Triggered"
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
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

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *SecondsToFirstGatewayBuildTriggeredPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}
