package dashboard

import (
	"encoding/json"
	"fmt"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
)

// populateAllianceTimelineForGameDetail attaches the alliance topology
// timeline to a game detail when the game qualifies (Melee + >2 active
// players). For non-qualifying games it leaves AllianceTimeline empty.
//
// We reconstruct screp-style models.Player and models.Command values from the
// stored rows so the parser.AnalyzeAlliances function — the same one used at
// ingest — can run unmodified.
func (d *Dashboard) populateAllianceTimelineForGameDetail(detail *workflowGameDetail) error {
	if detail.GameType != "Melee" {
		return nil
	}

	playerRows, err := d.dbStore.ListReplayPlayersForAlliance(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to list players for alliance: %w", err)
	}
	cmdRows, err := d.dbStore.ListReplayAllianceCommands(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to list alliance commands: %w", err)
	}
	// Activity is sourced from replay_events (leave_game + player_stopped_playing)
	// rather than rescanning commands — the per-game inactivity calculation
	// happens once at ingest and is persisted there.
	eventRows, err := d.dbStore.ListReplayEvents(d.ctx, detail.ReplayID)
	if err != nil {
		return fmt.Errorf("failed to list replay events: %w", err)
	}

	// Active player set: non-observer, non-computer (mirrors screp).
	activeCount := 0
	for _, p := range playerRows {
		if p.IsObserver || p.Type == "Computer" {
			continue
		}
		activeCount++
	}
	if activeCount <= 2 {
		return nil
	}

	// PlayerID-as-byte mapping: the analyzer uses byte-keyed IDs. We use the
	// players-table id (truncated to byte) — collisions are impossible inside
	// a single replay since slot_id maxes at 11 and ID assignment is per-row.
	// To be safe we assign synthetic byte IDs in iteration order and remember
	// the reverse mapping back to int64 DB IDs for the API payload.
	dbIDByByte := map[byte]int64{}
	playerByDBID := map[int64]models.Player{}
	playerBySlot := map[byte]*models.Player{}
	syntheticPID := byte(1)
	players := make([]*models.Player, 0, len(playerRows))
	for _, row := range playerRows {
		// Skip purely observer rows from the analyzer's view (already excluded
		// from activeCount). Computers stay so the analyzer can ignore them.
		p := models.Player{
			SlotID:     uint16(row.SlotID),
			PlayerID:   syntheticPID,
			Name:       row.Name,
			Race:       row.Race,
			Type:       row.Type,
			Team:       byte(row.Team),
			IsObserver: row.IsObserver,
		}
		dbIDByByte[syntheticPID] = row.PlayerID
		playerByDBID[row.PlayerID] = p
		players = append(players, &p)
		playerBySlot[byte(row.SlotID)] = players[len(players)-1]
		syntheticPID++
		if syntheticPID == 0 {
			// Defensive: ID byte wraparound. Shouldn't happen with melee
			// player counts (≤8), but bail rather than alias IDs.
			return fmt.Errorf("alliance: too many players to assign byte IDs (%d)", len(playerRows))
		}
	}

	// Map the DB-id-keyed alliance command rows to byte-pid analyzer
	// commands. The issuer's DB id is in row.PlayerID; the alliance targets
	// are slot IDs in the JSON array.
	playerByteByDBID := map[int64]byte{}
	for b, dbID := range dbIDByByte {
		playerByteByDBID[dbID] = b
	}
	commands := make([]*models.Command, 0, len(cmdRows))
	for _, row := range cmdRows {
		issuerByte, ok := playerByteByDBID[row.PlayerID]
		if !ok {
			continue
		}
		issuerPlayer := players[issuerByte-1]
		var slotIDs []int64
		if row.AlliancePlayerIDs != "" {
			if err := json.Unmarshal([]byte(row.AlliancePlayerIDs), &slotIDs); err != nil {
				// Log and skip rather than fail the whole game-detail call;
				// a malformed JSON row shouldn't break the page.
				continue
			}
		}
		cmd := &models.Command{
			ActionType:           "Alliance",
			SecondsFromGameStart: int(row.SecondsFromGameStart),
			Player:               issuerPlayer,
			AlliancePlayerIDs:    &slotIDs,
		}
		commands = append(commands, cmd)
	}

	// Build the activity maps (leave + stop) from replay_events for the
	// effective-team filter inside the analyzer.
	activity := parser.Activity{
		StoppedSecByPID: map[byte]int{},
		LeaveSecByPID:   map[byte]int{},
	}
	for _, ev := range eventRows {
		if ev.SourcePlayerID == nil {
			continue
		}
		bytePID, ok := playerByteByDBID[*ev.SourcePlayerID]
		if !ok {
			continue
		}
		switch ev.EventType {
		case "leave_game":
			if existing, exists := activity.LeaveSecByPID[bytePID]; !exists || int(ev.Second) < existing {
				activity.LeaveSecByPID[bytePID] = int(ev.Second)
			}
		case "player_stopped_playing":
			if existing, exists := activity.StoppedSecByPID[bytePID]; !exists || int(ev.Second) < existing {
				activity.StoppedSecByPID[bytePID] = int(ev.Second)
			}
		}
	}

	res := parser.AnalyzeAlliances(players, commands, int(detail.DurationSeconds), activity)

	// Translate snapshots from synthetic byte IDs back to DB int64 IDs.
	out := make([]workflowAllianceSnapshot, 0, len(res.Snapshots))
	for _, snap := range res.Snapshots {
		teams := make([][]int64, 0, len(snap.Teams))
		for _, team := range snap.Teams {
			ids := make([]int64, 0, len(team))
			for _, b := range team {
				if dbID, ok := dbIDByByte[b]; ok {
					ids = append(ids, dbID)
				}
			}
			if len(ids) > 0 {
				teams = append(teams, ids)
			}
		}
		out = append(out, workflowAllianceSnapshot{
			Sec:      int64(snap.Sec),
			Teams:    teams,
			Stacking: snap.Stacking,
		})
	}
	detail.AllianceTimeline = out
	detail.AllianceStackingThresholdSeconds = parser.StackingThresholdSec
	return nil
}
