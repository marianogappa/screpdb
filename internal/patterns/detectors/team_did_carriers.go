package detectors

import (
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
)

// DidCarriersTeamDetector detects if any player on a team created at least 3 Carriers
type DidCarriersTeamDetector struct {
	replay       *models.Replay
	players      []*models.Player
	team         byte
	playerCounts map[int64]int // player ID -> carrier count
	finished     bool
}

// NewDidCarriersTeamDetector creates a new team-level Carriers detector
func NewDidCarriersTeamDetector() *DidCarriersTeamDetector {
	return &DidCarriersTeamDetector{
		playerCounts: make(map[int64]int),
		finished:     false,
	}
}

func (d *DidCarriersTeamDetector) Name() string {
	return "Did Carriers"
}

func (d *DidCarriersTeamDetector) Level() core.DetectorLevel {
	return core.LevelTeam
}

func (d *DidCarriersTeamDetector) Initialize(replay *models.Replay, players []*models.Player) {
	d.replay = replay
	d.players = players
}

func (d *DidCarriersTeamDetector) ProcessCommand(command *models.Command) bool {
	// Only process commands for players on this team
	if command.Player == nil || command.Player.Team != d.team {
		return false
	}

	// Check if this is a unit creation command with Carrier
	if command.UnitType != nil && *command.UnitType == "Carrier" {
		replayPlayerID := command.Player.PlayerID // Use replay player ID (byte)
		d.playerCounts[int64(replayPlayerID)]++
		// Finish when any player on the team has 3 or more Carriers
		if d.playerCounts[int64(replayPlayerID)] >= 3 {
			d.finished = true
			return true
		}
	}

	return false
}

func (d *DidCarriersTeamDetector) IsFinished() bool {
	return d.finished
}

func (d *DidCarriersTeamDetector) GetResult() *core.PatternResult {
	if !d.finished {
		return nil
	}

	// Only return a result if any player on the team has 3+ Carriers (result is true)
	for _, count := range d.playerCounts {
		if count >= 3 {
			valueBool := true
			return &core.PatternResult{
				PatternName: d.Name(),
				Level:       d.Level(),
				ReplayID:    d.replay.ID,
				Team:        &d.team,
				ValueBool:   &valueBool,
			}
		}
	}

	// No player on the team has 3+ carriers, don't return a result
	return nil
}

func (d *DidCarriersTeamDetector) ShouldSave() bool {
	// Only save if we have a true result (at least 3 carriers detected by any player on the team)
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

// SetTeam sets the team this detector is monitoring
func (d *DidCarriersTeamDetector) SetTeam(team byte) {
	d.team = team
}
