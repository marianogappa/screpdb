package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// ChatCommandHandler handles Chat commands
type ChatCommandHandler struct {
	BaseCommandHandler
}

func NewChatCommandHandler() *ChatCommandHandler {
	return &ChatCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Chat",
			actionID:   repcmd.TypeIDChat,
		},
	}
}

func (h *ChatCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Chat data is stored in dedicated chat_messages table, not in commands table
	return command
}
