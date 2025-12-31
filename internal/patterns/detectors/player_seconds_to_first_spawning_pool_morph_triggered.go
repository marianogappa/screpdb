package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector detects the seconds to first Spawning Pool morph triggered for a player
type SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	seconds        *int
	finished       bool
}

// NewSecondsToFirstSpawningPoolMorphTriggeredPlayerDetector creates a new player-level detector
func NewSecondsToFirstSpawningPoolMorphTriggeredPlayerDetector() *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector {
	return &SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector{
		finished: false,
	}
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) Name() string {
	return "Seconds to First Spawning Pool Morph Triggered"
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
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

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) GetResult() *core.PatternResult {
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

func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) ShouldSave() bool {
	// Only save if we detected the command
	return d.finished && d.seconds != nil
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *SecondsToFirstSpawningPoolMorphTriggeredPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}

