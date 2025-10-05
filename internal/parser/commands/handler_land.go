package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// LandCommandHandler handles Land commands (virtual type)
type LandCommandHandler struct {
	BaseCommandHandler
}

func NewLandCommandHandler() *LandCommandHandler {
	return &LandCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Land",
			actionID:   repcmd.VirtualTypeIDLand,
		},
	}
}

func (h *LandCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	landCmd := cmd.(*repcmd.LandCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.X = int(landCmd.Pos.X)
	command.Y = int(landCmd.Pos.Y)

	if landCmd.Unit != nil {
		command.UnitID = bytePtr(byte(landCmd.Unit.ID))
		command.BuildUnitName = stringPtr(landCmd.Unit.Name)
	}

	if landCmd.Order != nil {
		command.OrderID = bytePtr(landCmd.Order.ID)
		command.OrderName = stringPtr(landCmd.Order.Name)
	}

	return command
}
