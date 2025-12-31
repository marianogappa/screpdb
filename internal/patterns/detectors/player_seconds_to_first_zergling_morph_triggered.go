package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstZerglingMorphTriggeredPlayerDetector detects the seconds to first Zergling morph triggered for a player
type SecondsToFirstZerglingMorphTriggeredPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	seconds        *int
	finished       bool
}

// NewSecondsToFirstZerglingMorphTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstZerglingMorphTriggeredPlayerDetector() *SecondsToFirstZerglingMorphTriggeredPlayerDetector {
	return &SecondsToFirstZerglingMorphTriggeredPlayerDetector{
		finished: false,
	}
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) Name() string {
	return "Seconds to First Zergling Morph Triggered"
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
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

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *SecondsToFirstZerglingMorphTriggeredPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}
