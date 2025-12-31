package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// HadCarriersReplayDetector detects if any player in the replay created at least 3 Carriers
type HadCarriersReplayDetector struct {
	replay       *models.Replay
	players      []*models.Player
	playerCounts map[int64]int // player ID -> carrier count
	finished     bool
}

// NewHadCarriersReplayDetector creates a new replay-level Carriers detector
func NewHadCarriersReplayDetector() *HadCarriersReplayDetector {
	return &HadCarriersReplayDetector{
		playerCounts: make(map[int64]int),
		finished:     false,
	}
}

func (d *HadCarriersReplayDetector) Name() string {
	return "Had Carriers"
}

func (d *HadCarriersReplayDetector) Level() core.DetectorLevel {
	return core.LevelReplay
}

func (d *HadCarriersReplayDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *HadCarriersReplayDetector) ProcessCommand(command *models.Command) bool {
	// Process commands from any player
	if command.Player == nil {
		return false
	}

	// Check if this is a unit creation command with Carrier
	if command.UnitType != nil && *command.UnitType == "Carrier" {
		replayPlayerID := command.Player.PlayerID // Use replay player ID (byte)
		d.playerCounts[int64(replayPlayerID)]++
		// Finish when any player has 3 or more Carriers
		if d.playerCounts[int64(replayPlayerID)] >= 3 {
			d.finished = true
			return true
		}
	}

	return false
}

func (d *HadCarriersReplayDetector) IsFinished() bool {
	return d.finished
}

func (d *HadCarriersReplayDetector) GetResult() *core.PatternResult {
	if !d.finished {
		return nil
	}

	// Only return a result if any player has 3+ Carriers (result is true)
	for _, count := range d.playerCounts {
		if count >= 3 {
			valueBool := true
			return &core.PatternResult{
				PatternName: d.Name(),
				Level:       d.Level(),
				ReplayID:    d.replay.ID,
				ValueBool:   &valueBool,
			}
		}
	}

	// No player has 3+ carriers, don't return a result
	return nil
}

func (d *HadCarriersReplayDetector) ShouldSave() bool {
	// Only save if we have a true result (at least 3 carriers detected by any player)
	// This ensures we never save false results to the database
	if !d.finished {
		return false
	}
	for _, count := range d.playerCounts {
		if count >= 3 {
			return true
		}
	}
	return false
}
