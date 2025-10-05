package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// MinimapPingCommandHandler handles MinimapPing commands
type MinimapPingCommandHandler struct {
	BaseCommandHandler
}

func NewMinimapPingCommandHandler() *MinimapPingCommandHandler {
	return &MinimapPingCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "MinimapPing",
			actionID:   repcmd.TypeIDMinimapPing,
		},
	}
}

func (h *MinimapPingCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	minimapPingCmd := cmd.(*repcmd.MinimapPingCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(minimapPingCmd.Pos.X)
	command.Y = int(minimapPingCmd.Pos.Y)

	return command
}
