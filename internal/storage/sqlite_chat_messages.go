package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// SQLiteChatMessagesInserter implements SQLiteBatchInserter for chat messages
type SQLiteChatMessagesInserter struct{}

// NewSQLiteChatMessagesInserter creates a new SQLite chat messages inserter
func NewSQLiteChatMessagesInserter() *SQLiteChatMessagesInserter {
	return &SQLiteChatMessagesInserter{}
}

// TableName returns the table name for chat messages
func (c *SQLiteChatMessagesInserter) TableName() string {
	return "chat_messages"
}

var sqliteChatMessagesColumnNames = []string{
	"replay_id", "player_id", "message", "frame", "time",
}

// ColumnNames returns the column names for chat messages
func (c *SQLiteChatMessagesInserter) ColumnNames() []string {
	return sqliteChatMessagesColumnNames
}

// EntityCount returns the number of columns for chat messages
func (c *SQLiteChatMessagesInserter) EntityCount() int {
	return len(sqliteChatMessagesColumnNames)
}

// BuildArgs builds the arguments for a chat message entity
func (c *SQLiteChatMessagesInserter) BuildArgs(entity any, args []any, offset int) error {
	chatMsg, ok := entity.(*models.ChatMessage)
	if !ok {
		return fmt.Errorf("expected *models.ChatMessage, got %T", entity)
	}

	args[offset] = chatMsg.ReplayID
	args[offset+1] = chatMsg.PlayerID
	args[offset+2] = chatMsg.Message
	args[offset+3] = chatMsg.Frame
	args[offset+4] = chatMsg.Time

	return nil
}
