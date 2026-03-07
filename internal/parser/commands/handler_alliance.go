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

func (h *AllianceCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	allianceCmd := cmd.(*repcmd.AllianceCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.AlliancePlayerIDs = bytsToInts(allianceCmd.SlotIDs) // TODO these need to be mapped after insertion
	command.IsAlliedVictory = boolPtr(allianceCmd.AlliedVictory)

	return command
}
