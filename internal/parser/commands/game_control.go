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

func (h *GameSpeedCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	gameSpeedCmd := cmd.(*repcmd.GameSpeedCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if gameSpeedCmd.Speed != nil {
		command.GameSpeed = stringPtr(gameSpeedCmd.Speed.String())
	}

	return command
}

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

func (h *HotkeyCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	hotkeyCmd := cmd.(*repcmd.HotkeyCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if hotkeyCmd.HotkeyType != nil {
		command.HotkeyType = stringPtr(hotkeyCmd.HotkeyType.Name)
	}

	command.HotkeyGroup = bytePtr(hotkeyCmd.Group)

	return command
}

// ChatCommandHandler handles Chat commands
type ChatCommandHandler struct {
	BaseCommandHandler
}

func NewChatCommandHandler() *ChatCommandHandler {
	return &ChatCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Chat",
			actionID:   repcmd.TypeIDChat,
		},
	}
}

func (h *ChatCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Chat data is stored in dedicated chat_messages table, not in commands table
	return command
}

// VisionCommandHandler handles Vision commands
type VisionCommandHandler struct {
	BaseCommandHandler
}

func NewVisionCommandHandler() *VisionCommandHandler {
	return &VisionCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Vision",
			actionID:   repcmd.TypeIDVision,
		},
	}
}

func (h *VisionCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	visionCmd := cmd.(*repcmd.VisionCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	command.VisionSlotIDs = slotIDsToIntSlice(visionCmd.SlotIDs)

	return command
}

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

	command.AllianceSlotIDs = slotIDsToIntSlice(allianceCmd.SlotIDs)
	command.IsAlliedVictory = boolPtr(allianceCmd.AlliedVictory)

	return command
}

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

func (h *LeaveGameCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command {
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	// Leave game data is stored in dedicated leave_games table, not in commands table
	return command
}
