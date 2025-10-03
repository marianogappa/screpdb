package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLiteLeaveGamesInserter implements SQLiteBatchInserter for leave games
type SQLiteLeaveGamesInserter struct{}

// NewSQLiteLeaveGamesInserter creates a new SQLite leave games inserter
func NewSQLiteLeaveGamesInserter() *SQLiteLeaveGamesInserter {
	return &SQLiteLeaveGamesInserter{}
}

// TableName returns the table name for leave games
func (l *SQLiteLeaveGamesInserter) TableName() string {
	return "leave_games"
}

// ColumnNames returns the column names for leave games
func (l *SQLiteLeaveGamesInserter) ColumnNames() []string {
	return []string{
		"replay_id", "player_id", "reason", "frame", "time",
	}
}

// EntityCount returns the number of columns for leave games
func (l *SQLiteLeaveGamesInserter) EntityCount() int {
	return 5
}

// BuildArgs builds the arguments for a leave game entity
func (l *SQLiteLeaveGamesInserter) BuildArgs(entity any, args []any, offset int) error {
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
