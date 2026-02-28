package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// UnloadCommandHandler handles Unload commands (including 121 version)
type UnloadCommandHandler struct {
	BaseCommandHandler
}

func NewUnloadCommandHandler(actionType string, actionID byte) *UnloadCommandHandler {
	return &UnloadCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *UnloadCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller
	return command
}
