package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstCarrierBuildTriggeredPlayerDetector detects the seconds to first Carrier build triggered for a player
type SecondsToFirstCarrierBuildTriggeredPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	seconds        *int
	finished       bool
}

// NewSecondsToFirstCarrierBuildTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstCarrierBuildTriggeredPlayerDetector() *SecondsToFirstCarrierBuildTriggeredPlayerDetector {
	return &SecondsToFirstCarrierBuildTriggeredPlayerDetector{
		finished: false,
	}
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) Name() string {
	return "Seconds to First Carrier Build Triggered"
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
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

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *SecondsToFirstCarrierBuildTriggeredPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}
