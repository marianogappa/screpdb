package dashboard

import (
	"fmt"
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
		rows, err := d.dbStore.ReplayQueryContext(d.ctx,
			`SELECT c.player_id, COALESCE(c.leave_reason, '')
			 FROM commands c
			 WHERE c.replay_id = ?
			   AND c.action_type = 'Leave Game'
			   AND c.leave_reason IS NOT NULL
			 UNION ALL
			 SELECT c.player_id, COALESCE(c.leave_reason, '')
			 FROM commands_low_value c
			 WHERE c.replay_id = ?
			   AND c.action_type = 'Leave Game'
			   AND c.leave_reason IS NOT NULL`,
			detail.ReplayID, detail.ReplayID,
		)
		if err != nil {
			return fmt.Errorf("failed to query leave reasons: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var pid int64
			var reason string
			if err := rows.Scan(&pid, &reason); err != nil {
				return fmt.Errorf("failed to scan leave reason: %w", err)
			}
			reason = strings.TrimSpace(reason)
			if reason == "" {
				continue
			}
			if dep, ok := deparByPID[pid]; ok && dep.Reason == "Left" {
				dep.Reason = reason
				deparByPID[pid] = dep
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("failed to iterate leave reasons: %w", err)
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

	rows, err := d.dbStore.ReplayQueryContext(d.ctx,
		`SELECT c.seconds_from_game_start, c.player_id, COALESCE(c.chat_message, '')
		 FROM commands c
		 WHERE c.replay_id = ?
		   AND c.chat_message IS NOT NULL
		   AND trim(c.chat_message) <> ''
		 UNION ALL
		 SELECT c.seconds_from_game_start, c.player_id, COALESCE(c.chat_message, '')
		 FROM commands_low_value c
		 WHERE c.replay_id = ?
		   AND c.chat_message IS NOT NULL
		   AND trim(c.chat_message) <> ''
		 ORDER BY 1 ASC, 2 ASC`,
		detail.ReplayID, detail.ReplayID,
	)
	if err != nil {
		return fmt.Errorf("failed to query alliance-tab chat: %w", err)
	}
	defer rows.Close()
	out := []workflowAllianceChat{}
	for rows.Next() {
		var second int64
		var pid int64
		var message string
		if err := rows.Scan(&second, &pid, &message); err != nil {
			return fmt.Errorf("failed to scan chat row: %w", err)
		}
		out = append(out, workflowAllianceChat{
			Second:   second,
			PlayerID: pid,
			Message:  message,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("failed to iterate chat rows: %w", err)
	}
	if len(out) > 0 {
		detail.AllianceTabChat = out
	}
	return nil
}
