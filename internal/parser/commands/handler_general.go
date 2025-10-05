package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// GeneralCommandHandler handles general/unhandled commands
type GeneralCommandHandler struct {
	BaseCommandHandler
}

func NewGeneralCommandHandler(actionType string, actionID byte) *GeneralCommandHandler {
	return &GeneralCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: actionType,
			actionID:   actionID,
		},
	}
}

func (h *GeneralCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Try to extract data from GeneralCmd if it's that type, otherwise use empty data
	if generalCmd, ok := cmd.(*repcmd.GeneralCmd); ok {
		command.GeneralData = dataToHexString(generalCmd.Data)
	} else {
		// For other command types that don't have specific handlers,
		// we'll store basic information without the raw data
		command.GeneralData = nil
	}

	return command
}
