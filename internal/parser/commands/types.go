package commands

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/models"
)

// CommandHandler defines the interface for handling specific command types
type CommandHandler interface {
	Handle(cmd repcmd.Cmd, base *repcmd.Base) *models.Command
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

// Helper functions for common operations
func unitTagsToJSON(unitTags []repcmd.UnitTag) *string {
	if len(unitTags) == 0 {
		return nil
	}

	tags := make([]uint16, len(unitTags))
	for i, tag := range unitTags {
		tags[i] = uint16(tag)
	}

	data, err := json.Marshal(tags)
	if err != nil {
		return nil
	}

	result := string(data)
	return &result
}

func slotIDsToJSON(slotIDs repcmd.Bytes) *string {
	if len(slotIDs) == 0 {
		return nil
	}

	ids := make([]byte, len(slotIDs))
	for i, id := range slotIDs {
		ids[i] = byte(id)
	}

	data, err := json.Marshal(ids)
	if err != nil {
		return nil
	}

	result := string(data)
	return &result
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

func uint16Ptr(u uint16) *uint16 {
	return &u
}

func intPtr(i int) *int {
	return &i
}

// Create base command from base command info
func createBaseCommand(base *repcmd.Base, replayID int64, startTime int64) *models.Command {
	return &models.Command{
		ReplayID:   replayID,
		PlayerID:   int64(base.PlayerID),
		Frame:      int32(base.Frame),
		Time:       time.Unix(startTime+int64(base.Frame.Duration().Seconds()), 0),
		ActionType: base.Type.String(),
		ActionID:   base.Type.ID,
		Effective:  base.IneffKind.Effective(),
	}
}
