package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// QueueableCommandHandler handles queueable commands (Stop, HoldPosition, etc.)
type QueueableCommandHandler struct {
	BaseCommandHandler
}

func NewQueueableCommandHandler(actionType string, actionID byte) *QueueableCommandHandler {
	return &QueueableCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *QueueableCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Try to extract queued status from QueueableCmd if it's that type
	if queueableCmd, ok := cmd.(*repcmd.QueueableCmd); ok {
		command.IsQueued = boolPtr(queueableCmd.Queued)
	} else {
		// For other command types, we'll store basic information without queued status
		command.IsQueued = nil
	}

	return command
}
