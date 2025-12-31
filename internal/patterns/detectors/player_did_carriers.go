package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// DidCarriersPlayerDetector detects if a player created at least 3 Carriers
type DidCarriersPlayerDetector struct {
	replay         *models.Replay
	players        []*models.Player
	replayPlayerID byte // Replay's player ID (byte), not database ID
	carrierCount   int
	finished       bool
}

// NewDidCarriersPlayerDetector creates a new player-level Carriers detector
func NewDidCarriersPlayerDetector() *DidCarriersPlayerDetector {
	return &DidCarriersPlayerDetector{
		carrierCount: 0,
		finished:     false,
	}
}

func (d *DidCarriersPlayerDetector) Name() string {
	return "Did Carriers"
}

func (d *DidCarriersPlayerDetector) Level() core.DetectorLevel {
	return core.LevelPlayer
}

func (d *DidCarriersPlayerDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *DidCarriersPlayerDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for this specific player (by replay player ID)
	if command.Player == nil || command.Player.PlayerID != d.replayPlayerID {
		return false
	}

	// Check if this is a unit creation command with Carrier
	if command.UnitType != nil && *command.UnitType == "Carrier" {
		d.carrierCount++
		// Finish when we detect 3 or more Carriers
		if d.carrierCount >= 3 {
			d.finished = true
			return true
		}
	}

	return false
}

func (d *DidCarriersPlayerDetector) IsFinished() bool {
	return d.finished
}

func (d *DidCarriersPlayerDetector) GetResult() *core.PatternResult {
	// Only return a result if we detected at least 3 carriers (result is true)
	if !d.finished || d.carrierCount < 3 {
		return nil
	}

	// Store replay player ID temporarily, will be converted to database ID later
	valueBool := true
	replayPlayerID := d.replayPlayerID
	return &core.PatternResult{
		PatternName:    d.Name(),
		Level:          d.Level(),
		ReplayID:       d.replay.ID,
		PlayerID:       nil, // Will be set when converting to database IDs
		ReplayPlayerID: &replayPlayerID,
		ValueBool:      &valueBool,
	}
}

func (d *DidCarriersPlayerDetector) ShouldSave() bool {
	// Only save if we have a true result (at least 3 carriers detected)
	// This ensures we never save false results to the database
	return d.finished && d.carrierCount >= 3
}

// SetReplayPlayerID sets the replay player ID (byte) this detector is monitoring
func (d *DidCarriersPlayerDetector) SetReplayPlayerID(replayPlayerID byte) {
	d.replayPlayerID = replayPlayerID
}

// GetReplayPlayerID returns the replay player ID this detector is monitoring
func (d *DidCarriersPlayerDetector) GetReplayPlayerID() byte {
	return d.replayPlayerID
}
