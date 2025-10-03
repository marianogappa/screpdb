package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// LeaveGamesInserter implements BatchInserter for leave games
type LeaveGamesInserter struct{}

// NewLeaveGamesInserter creates a new leave games inserter
func NewLeaveGamesInserter() *LeaveGamesInserter {
	return &LeaveGamesInserter{}
}

// TableName returns the table name for leave games
func (l *LeaveGamesInserter) TableName() string {
	return "leave_games"
}

// ColumnNames returns the column names for leave games
func (l *LeaveGamesInserter) ColumnNames() []string {
	return []string{
		"replay_id", "player_id", "reason", "frame", "time",
	}
}

// EntityCount returns the number of columns for leave games
func (l *LeaveGamesInserter) EntityCount() int {
	return 5
}

// BuildArgs builds the arguments for a leave game entity
func (l *LeaveGamesInserter) BuildArgs(entity any, args []any, offset int) error {
	leaveGame, ok := entity.(*models.LeaveGame)
	if !ok {
		return fmt.Errorf("expected *models.LeaveGame, got %T", entity)
	}

	args[offset] = leaveGame.ReplayID
	args[offset+1] = leaveGame.PlayerID
	args[offset+2] = leaveGame.Reason
	args[offset+3] = leaveGame.Frame
	args[offset+4] = leaveGame.Time

	return nil
}
