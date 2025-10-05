package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// GameSpeedCommandHandler handles GameSpeed commands
type GameSpeedCommandHandler struct {
	BaseCommandHandler
}

func NewGameSpeedCommandHandler() *GameSpeedCommandHandler {
	return &GameSpeedCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "GameSpeed",
			actionID:   repcmd.TypeIDGameSpeed,
		},
	}
}

func (h *GameSpeedCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	gameSpeedCmd := cmd.(*repcmd.GameSpeedCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if gameSpeedCmd.Speed != nil {
		command.GameSpeed = stringPtr(gameSpeedCmd.Speed.String())
	}

	return command
}
