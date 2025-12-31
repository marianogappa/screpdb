package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstMutaliskMorphTriggeredPlayerDetector detects the seconds to first Mutalisk morph triggered for a player
type SecondsToFirstMutaliskMorphTriggeredPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	seconds        *int
	finished       bool
}

// NewSecondsToFirstMutaliskMorphTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstMutaliskMorphTriggeredPlayerDetector() *SecondsToFirstMutaliskMorphTriggeredPlayerDetector {
	return &SecondsToFirstMutaliskMorphTriggeredPlayerDetector{
		finished: false,
	}
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) Name() string {
	return "Seconds to First Mutalisk Morph Triggered"
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
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

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *SecondsToFirstMutaliskMorphTriggeredPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}

