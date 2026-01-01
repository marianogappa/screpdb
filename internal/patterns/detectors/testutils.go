package detectors

import (
	"time"

	"github.com/marianogappa/screpdb/internal/models"
)

// TestReplayBuilder helps build test replays
type TestReplayBuilder struct {
	replay   *models.Replay
	players  []*models.Player
	commands []*models.Command
}

// NewTestReplayBuilder creates a new test replay builder
func NewTestReplayBuilder() *TestReplayBuilder {
	return &TestReplayBuilder{
		replay: &models.Replay{
			ID:        1,
			ReplayDate: time.Now(),
		},
		players:  []*models.Player{},
		commands: []*models.Command{},
	}
}

// WithPlayer adds a player to the replay
func (b *TestReplayBuilder) WithPlayer(playerID byte, name, race string, team byte) *TestReplayBuilder {
	player := &models.Player{
		ID:       int64(len(b.players) + 1),
		ReplayID: b.replay.ID,
		PlayerID: playerID,
		Name:     name,
		Race:     race,
		Type:     models.PlayerTypeHuman,
		Team:     team,
	}
	b.players = append(b.players, player)
	return b
}

// WithCommand adds a command to the replay
func (b *TestReplayBuilder) WithCommand(playerID byte, seconds int, actionType, unitType string) *TestReplayBuilder {
	cmd := &models.Command{
		ID:                   int64(len(b.commands) + 1),
		ReplayID:             b.replay.ID,
		SecondsFromGameStart: seconds,
		ActionType:           actionType,
		UnitType:             stringPtr(unitType),
		Player:               b.findPlayer(playerID),
	}
	b.commands = append(b.commands, cmd)
	return b
}

// Build returns the replay and players
func (b *TestReplayBuilder) Build() (*models.Replay, []*models.Player) {
	// Link players to replay
	for _, player := range b.players {
		player.Replay = b.replay
	}
	b.replay.Players = b.players
	return b.replay, b.players
}

// GetCommands returns the commands
func (b *TestReplayBuilder) GetCommands() []*models.Command {
	return b.commands
}

func (b *TestReplayBuilder) findPlayer(playerID byte) *models.Player {
	for _, player := range b.players {
		if player.PlayerID == playerID {
			return player
		}
	}
	return nil
}

func stringPtr(s string) *string {
	return &s
}


