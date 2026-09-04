package dashboard

import (
	"strings"
)

// populatePlayerDepartureForGameDetail backfills per-player LeftSecond /
// LeaveReason from the already-populated GameEvents (leave_game and
// player_stopped_playing) plus the raw commands.leave_reason for an exact
// reason string. This is shipped on detail.Players so the Alliances tab can
// truncate player lines at the moment a player stopped playing.
func (d *Dashboard) populatePlayerDepartureForGameDetail(detail *workflowGameDetail) error {
	if len(detail.Players) == 0 {
		return nil
	}

	// Index player rows by ID for fast lookup / write-back.
	playerByID := map[int64]*workflowGamePlayer{}
	for i := range detail.Players {
		p := &detail.Players[i]
		playerByID[p.PlayerID] = p
	}

	// Pull the earliest leave_game / player_stopped_playing second from the
	// already-populated GameEvents slice. This avoids a second round-trip and
	// guarantees consistency with what the Events list shows.
	type departure struct {
		Second int64
		Reason string
	}
	deparByPID := map[int64]departure{}
	for _, ev := range detail.GameEvents {
		if ev.Actor == nil {
			continue
		}
		pid := ev.Actor.PlayerID
		switch ev.Type {
		case "leave_game":
			cur, ok := deparByPID[pid]
			if !ok || ev.Second < cur.Second {
				deparByPID[pid] = departure{Second: ev.Second, Reason: "Left"}
			}
		// player_dropped / mass_disconnect carry the reason in the type; the
		// mass_disconnect actor is the replay saver, whose connection died.
		case "player_dropped", "mass_disconnect":
			cur, ok := deparByPID[pid]
			if !ok || ev.Second < cur.Second {
				deparByPID[pid] = departure{Second: ev.Second, Reason: "Dropped"}
			}
		case "player_stopped_playing":
			cur, ok := deparByPID[pid]
			if !ok || ev.Second < cur.Second {
				deparByPID[pid] = departure{Second: ev.Second, Reason: "Stopped"}
			}
		}
	}

	// Replace the generic "Left" with the screp leave-reason enum (Quit,
	// Defeat, Dropped, Finished, Draw, Victory, UNKNOWN). The data lives on
	// commands.leave_reason — we only fetch when there's at least one
	// leave_game event to enrich.
	hasLeaveGame := false
	for _, dep := range deparByPID {
		if dep.Reason == "Left" {
			hasLeaveGame = true
			break
		}
	}
	if hasLeaveGame {
		rows, err := d.dbStore.ListReplayLeaveReasons(d.ctx, detail.ReplayID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			reason := strings.TrimSpace(row.Reason)
			if reason == "" {
				continue
			}
			if dep, ok := deparByPID[row.PlayerID]; ok && dep.Reason == "Left" {
				dep.Reason = reason
				deparByPID[row.PlayerID] = dep
			}
		}
	}

	for pid, dep := range deparByPID {
		player, ok := playerByID[pid]
		if !ok {
			continue
		}
		s := dep.Second
		player.LeftSecond = &s
		player.LeaveReason = dep.Reason
	}
	return nil
}

// populateAllianceTabChatForGameDetail attaches the per-replay chat stream to
// the Alliances tab response. Only runs for Melee games with more than two
// active players (the same gate the alliance timeline uses, so the field is
// present together with the topology timeline that needs it).
func (d *Dashboard) populateAllianceTabChatForGameDetail(detail *workflowGameDetail) error {
	if detail.GameType != "Melee" {
		return nil
	}
	if len(detail.AllianceTimeline) == 0 {
		// AllianceTimeline is the source of truth for "does the alliance tab
		// even render": it stays empty when the alliance populator's
		// ≤2-active-player gate trips. Mirror that condition here so we don't
		// ship chat for games that aren't surfacing the tab.
		return nil
	}

	rows, err := d.dbStore.ListReplayChat(d.ctx, detail.ReplayID)
	if err != nil {
		return err
	}
	out := []workflowAllianceChat{}
	for _, row := range rows {
		out = append(out, workflowAllianceChat{
			Second:   row.Second,
			PlayerID: row.PlayerID,
			Message:  row.Message,
		})
	}
	if len(out) > 0 {
		detail.AllianceTabChat = out
	}
	return nil
}
