package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// HotkeyCommandHandler handles Hotkey commands
type HotkeyCommandHandler struct {
	BaseCommandHandler
}

func NewHotkeyCommandHandler() *HotkeyCommandHandler {
	return &HotkeyCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Hotkey",
			actionID:   repcmd.TypeIDHotkey,
		},
	}
}

func (h *HotkeyCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	hotkeyCmd := cmd.(*repcmd.HotkeyCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if hotkeyCmd.HotkeyType != nil {
		command.HotkeyType = stringPtr(hotkeyCmd.HotkeyType.Name)
	}

	command.HotkeyGroup = bytePtr(hotkeyCmd.Group)

	return command
}
