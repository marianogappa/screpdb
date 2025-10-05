package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// AllianceCommandHandler handles Alliance commands
type AllianceCommandHandler struct {
	BaseCommandHandler
}

func NewAllianceCommandHandler() *AllianceCommandHandler {
	return &AllianceCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Alliance",
			actionID:   repcmd.TypeIDAlliance,
		},
	}
}

func (h *AllianceCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	allianceCmd := cmd.(*repcmd.AllianceCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.AlliancePlayerIDs = slotIDsToPlayerIDs(allianceCmd.SlotIDs, slotToPlayerMap)
	command.IsAlliedVictory = boolPtr(allianceCmd.AlliedVictory)

	return command
}
