package commands

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// UpgradeCommandHandler handles Upgrade commands
type UpgradeCommandHandler struct {
	BaseCommandHandler
}

func NewUpgradeCommandHandler() *UpgradeCommandHandler {
	return &UpgradeCommandHandler{
		BaseCommandHandler: BaseCommandHandler{
			actionType: "Upgrade",
			actionID:   repcmd.TypeIDUpgrade,
		},
	}
}

func (h *UpgradeCommandHandler) Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command {
	upgradeCmd := cmd.(*repcmd.UpgradeCmd)
	command := createBaseCommand(base, 0, 0) // replayID and startTime will be set by caller

	if upgradeCmd.Upgrade != nil {
		command.UpgradeName = stringPtr(upgradeCmd.Upgrade.Name)
	}

	return command
}
