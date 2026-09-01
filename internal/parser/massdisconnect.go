package parser

import "github.com/marianogappa/screpdb/internal/models"

// MassDisconnectEnd describes a replay that ended when the replay saver lost
// their connection (issue #358): the saver's client records a simultaneous
// "Dropped" Leave Game for every other remaining player and stops recording
// seconds later. Winner attribution would otherwise read that as "everyone
// else left, the saver was the last one standing" and credit the saver's team
// a phantom win — the tie-break in winningTeamByLeaves fires by construction
// because the virtual saver leave is always last.
type MassDisconnectEnd struct {
	SaverPID      byte
	ClusterSecond int
}

// massDisconnectEndWindowSeconds bounds how close to the end of the replay
// the drop cluster must sit. Measured clusters land 2-4s before the end; a
// genuine mid-game drop (which can share a frame with another player's) sits
// minutes earlier.
const massDisconnectEndWindowSeconds = 5

// DetectMassDisconnectEnd classifies a saver disconnect. All must hold:
//
//  1. the replay saver is known and a non-observer,
//  2. the saver has no real Leave Game command,
//  3. >=2 non-observer Leave Game commands share a frame with reason Dropped,
//  4. every non-observer except the saver has a Leave Game command — exactly
//     the condition that makes the last-leaver tie-break fire,
//  5. the cluster sits within massDisconnectEndWindowSeconds of the end.
//
// The cluster size is bounded by how many opponents were still alive, so a
// count threshold is deliberately NOT part of the rule (a 39-minute game where
// everyone else already quit ends in a genuine 2-drop cluster). A mass
// simultaneous drop means either the saver's link died or the whole lobby lost
// the host; either way the game never resolved, so "no valid result" is
// correct regardless of cause. Returns nil when the replay is not a saver
// disconnect.
func DetectMassDisconnectEnd(players []*models.Player, cmds []*models.Command, repSaverPID *byte, durationSeconds int) *MassDisconnectEnd {
	if repSaverPID == nil {
		return nil
	}
	nonObs := map[byte]bool{}
	for _, p := range players {
		if p == nil || p.IsObserver {
			continue
		}
		nonObs[p.PlayerID] = true
	}
	if !nonObs[*repSaverPID] {
		return nil
	}

	leaverPIDs := map[byte]bool{}
	droppedByFrame := map[int32][]*models.Command{}
	for _, cmd := range cmds {
		if cmd == nil || cmd.ActionType != "Leave Game" || cmd.Player == nil || !nonObs[cmd.Player.PlayerID] {
			continue
		}
		if cmd.Player.PlayerID == *repSaverPID {
			return nil
		}
		leaverPIDs[cmd.Player.PlayerID] = true
		if cmd.LeaveReason != nil && *cmd.LeaveReason == "Dropped" {
			droppedByFrame[cmd.Frame] = append(droppedByFrame[cmd.Frame], cmd)
		}
	}
	if len(leaverPIDs) != len(nonObs)-1 {
		return nil
	}

	for _, cluster := range droppedByFrame {
		if len(cluster) < 2 {
			continue
		}
		sec := cluster[0].SecondsFromGameStart
		if durationSeconds-sec <= massDisconnectEndWindowSeconds {
			return &MassDisconnectEnd{SaverPID: *repSaverPID, ClusterSecond: sec}
		}
	}
	return nil
}
