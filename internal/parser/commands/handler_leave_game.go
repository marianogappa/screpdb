package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// LeaveGameCommandHandler handles LeaveGame commands
type LeaveGameCommandHandler struct {
	BaseCommandHandler
}

func NewLeaveGameCommandHandler() *LeaveGameCommandHandler {
	return &LeaveGameCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "LeaveGame",
			actionID:   repcmd.TypeIDLeaveGame,
		},
	}
}

func (h *LeaveGameCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Extract leave reason from the command
	if leaveCmd, ok := cmd.(*repcmd.LeaveGameCmd); ok {
		reason := ""
		if leaveCmd.Reason != nil {
			reason = leaveCmd.Reason.String()
		}
		command.LeaveReason = stringPtr(reason)
	}

	return command
}
