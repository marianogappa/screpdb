package commands

import (
	"github.com/marianogappa/screpdb/internal/models"
)

const (
	// Synthesized action types — not present in icza/screp's command type
	// table. They replace Right Click on commands whose target unit is a
	// transport (Dropship/Shuttle/Overlord post-Ventral-Sacs) or a Bunker.
	// Worldstate drop detection (see internal/patterns/worldstate/drops_pass.go)
	// keys off these.
	ActionTypeLoad       = "Load"
	ActionTypeLoadBunker = "LoadBunker"
)

// ClassifyLoads walks the command stream and rewrites Right Click actions
// whose TargetUnitType is a transport (or Bunker) into Load / LoadBunker
// actions. Overlord loads are gated on Ventral Sacs research completion —
// pre-Ventral-Sacs right-clicks on Overlords are just Move orders.
//
// Mutates commands in place. Safe to call multiple times: only commands with
// action_type == "Right Click" are candidates.
func ClassifyLoads(commands []*models.Command) {
	// Pass 1: find per-player Ventral Sacs completion second.
	ventralSacsDoneSec := map[int64]int{}
	for _, c := range commands {
		if c == nil {
			continue
		}
		if c.UpgradeName == nil || *c.UpgradeName != models.UpgradeVentralSacsOverlordTransport {
			continue
		}
		done, ok := models.UpgradeCompletionSec(c.SecondsFromGameStart, *c.UpgradeName)
		if !ok {
			continue
		}
		// First Ventral Sacs command per player wins (subsequent are no-ops).
		if existing, seen := ventralSacsDoneSec[c.PlayerID]; !seen || done < existing {
			ventralSacsDoneSec[c.PlayerID] = done
		}
	}

	// Pass 2: rewrite right-clicks on transports.
	for _, c := range commands {
		if c == nil {
			continue
		}
		if c.ActionType != "Right Click" {
			continue
		}
		if c.TargetUnitType == nil {
			continue
		}
		switch *c.TargetUnitType {
		case models.GeneralUnitDropship, models.GeneralUnitShuttle:
			c.ActionType = ActionTypeLoad
		case models.GeneralUnitBunker:
			c.ActionType = ActionTypeLoadBunker
		case models.GeneralUnitOverlord:
			done, ok := ventralSacsDoneSec[c.PlayerID]
			if ok && c.SecondsFromGameStart >= done {
				c.ActionType = ActionTypeLoad
			}
		}
	}
}
