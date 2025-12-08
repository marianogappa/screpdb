package commands

import (
	"fmt"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// CommandHandler defines the interface for handling specific command types
type CommandHandler interface {
	Handle(cmd repcmd.Cmd, base *repcmd.Base, slotToPlayerMap map[uint16]int64) *models.Command
	GetActionType() string
	GetActionID() byte
}

// BaseCommandHandler provides common functionality for command handlers
type BaseCommandHandler struct {
	actionType string
	actionID   byte
}

func (h *BaseCommandHandler) GetActionType() string {
	return h.actionType
}

func (h *BaseCommandHandler) GetActionID() byte {
	return h.actionID
}

// slotIDsToPlayerIDs converts slot IDs to player IDs using the provided mapping
func slotIDsToPlayerIDs(slotIDs repcmd.Bytes, slotToPlayerMap map[uint16]int64) *[]int64 {
	if len(slotIDs) == 0 {
		return nil
	}

	playerIDs := make([]int64, 0, len(slotIDs))
	for _, slotID := range slotIDs {
		if playerID, exists := slotToPlayerMap[uint16(slotID)]; exists {
			playerIDs = append(playerIDs, playerID)
		}
	}

	if len(playerIDs) == 0 {
		return nil
	}

	return &playerIDs
}

func dataToHexString(data []byte) *string {
	if len(data) == 0 {
		return nil
	}

	hex := fmt.Sprintf("%x", data)
	return &hex
}

func boolPtr(b bool) *bool {
	return &b
}

func bytePtr(b byte) *byte {
	return &b
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Create base command from base command info
func createBaseCommand(base *repcmd.Base, replayID int64, startTime int64) *models.Command {
	return &models.Command{
		ReplayID:             replayID,
		PlayerID:             int64(base.PlayerID),
		Frame:                int32(base.Frame),
		SecondsFromGameStart: int(base.Frame.Seconds()),
		RunAt:                time.Unix(startTime+int64(base.Frame.Duration().Seconds()), 0),
		ActionType:           base.Type.String(),
	}
}
