package storage

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
)

// ChatMessagesInserter implements BatchInserter for chat messages
type ChatMessagesInserter struct{}

// NewChatMessagesInserter creates a new chat messages inserter
func NewChatMessagesInserter() *ChatMessagesInserter {
	return &ChatMessagesInserter{}
}

// TableName returns the table name for chat messages
func (c *ChatMessagesInserter) TableName() string {
	return "chat_messages"
}

// ColumnNames returns the column names for chat messages
func (c *ChatMessagesInserter) ColumnNames() []string {
	return []string{
		"replay_id", "player_id", "sender_slot_id", "message", "frame", "time",
	}
}

// EntityCount returns the number of columns for chat messages
func (c *ChatMessagesInserter) EntityCount() int {
	return 6
}

// BuildArgs builds the arguments for a chat message entity
func (c *ChatMessagesInserter) BuildArgs(entity any, args []any, offset int) error {
	chatMsg, ok := entity.(*models.ChatMessage)
	if !ok {
		return fmt.Errorf("expected *models.ChatMessage, got %T", entity)
	}

	args[offset] = chatMsg.ReplayID
	args[offset+1] = chatMsg.PlayerID
	args[offset+2] = chatMsg.SenderSlotID
	args[offset+3] = chatMsg.Message
	args[offset+4] = chatMsg.Frame
	args[offset+5] = chatMsg.Time

	return nil
}
