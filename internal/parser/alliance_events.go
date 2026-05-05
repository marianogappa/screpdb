package parser

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// BuildAllianceDerivedEvents converts the alliance analyzer's outputs into
// replay events for the storage / dashboard pipeline. Three event types:
//
//   - player_stopped_playing: per-player, at the moment they went silent.
//     Suppressed when the player also has a Leave Game (the leave already
//     covers "gone"; ComputeActivity removes those from StoppedSecByPID).
//
//   - late_alliance: one event per topology-changing snapshot whose Sec is
//     past LateAllianceThresholdSec (10 min). Carries source = the issuer
//     of the most recent alliance command at that snapshot, target = a
//     primary new ally (best-effort).
//
//   - team_stacking_detected: at most one per replay, at the start of the
//     qualifying band. Description summarizes the team-sizes that earned
//     the flag.
func BuildAllianceDerivedEvents(players []*models.Player, ar AllianceResult) []worldstate.ReplayEvent {
	events := make([]worldstate.ReplayEvent, 0)
	pidToName := map[byte]string{}
	for _, p := range players {
		if p == nil {
			continue
		}
		pidToName[p.PlayerID] = p.Name
	}

	// player_stopped_playing — sorted by sec for stable test output.
	stopPIDs := make([]byte, 0, len(ar.StoppedSecByPID))
	for pid := range ar.StoppedSecByPID {
		stopPIDs = append(stopPIDs, pid)
	}
	sort.Slice(stopPIDs, func(i, j int) bool {
		return ar.StoppedSecByPID[stopPIDs[i]] < ar.StoppedSecByPID[stopPIDs[j]]
	})
	for _, pid := range stopPIDs {
		sec := ar.StoppedSecByPID[pid]
		pidCopy := pid
		events = append(events, worldstate.ReplayEvent{
			EventType:            "player_stopped_playing",
			Second:               sec,
			SourceReplayPlayerID: &pidCopy,
		})
	}

	// late_alliance — per topology-changing snapshot after sec=600.
	for _, snap := range ar.LateAllianceTransitions {
		issuer, primaryAlly := pickAllianceIssuerAndAlly(snap.Teams)
		event := worldstate.ReplayEvent{
			EventType: "late_alliance",
			Second:    snap.Sec,
		}
		if issuer != 0 {
			i := issuer
			event.SourceReplayPlayerID = &i
		}
		if primaryAlly != 0 {
			a := primaryAlly
			event.TargetReplayPlayerID = &a
		}
		// Payload carries all team groupings so the frontend can render the
		// full picture if it wants ("X is now allied with Y, Z").
		if payload, ok := marshalAllianceTopology(snap.Teams, pidToName); ok {
			payloadCopy := payload
			event.Payload = &payloadCopy
		}
		events = append(events, event)
	}

	// team_stacking_detected — at most one per replay.
	if ar.TeamStackingFlag {
		event := worldstate.ReplayEvent{
			EventType: "team_stacking_detected",
			Second:    ar.StackingBandStartSec,
		}
		if payload, ok := marshalStackingPayload(ar.StackingBandTeams, pidToName); ok {
			payloadCopy := payload
			event.Payload = &payloadCopy
		}
		events = append(events, event)
	}

	return events
}

// pickAllianceIssuerAndAlly picks a representative (issuer, ally) pair
// from a topology snapshot. We pick the largest non-solo team (ties broken
// by smallest min-pid) and use its first two members. The frontend has
// the full team list in payload; this is just for the structured columns
// so player-name colorization works for the headline names.
func pickAllianceIssuerAndAlly(teams [][]byte) (byte, byte) {
	var biggest []byte
	for _, t := range teams {
		if len(t) < 2 {
			continue
		}
		if len(t) > len(biggest) {
			biggest = t
		}
	}
	if len(biggest) < 2 {
		return 0, 0
	}
	return biggest[0], biggest[1]
}

// marshalAllianceTopology turns the team list into a small JSON object
// like {"teams":[["Alice","Bob"],["Carol"]]} for the frontend.
func marshalAllianceTopology(teams [][]byte, pidToName map[byte]string) (string, bool) {
	if len(teams) == 0 {
		return "", false
	}
	named := make([][]string, 0, len(teams))
	for _, team := range teams {
		row := make([]string, 0, len(team))
		for _, pid := range team {
			name, ok := pidToName[pid]
			if !ok || name == "" {
				name = fmt.Sprintf("Player %d", pid)
			}
			row = append(row, name)
		}
		if len(row) > 0 {
			named = append(named, row)
		}
	}
	if len(named) == 0 {
		return "", false
	}
	payload := map[string]any{"teams": named}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(b), true
}

// marshalStackingPayload encodes the team-sizes summary (e.g. "3v2") and
// the named team groupings for the team_stacking_detected event.
func marshalStackingPayload(teams [][]byte, pidToName map[byte]string) (string, bool) {
	if len(teams) == 0 {
		return "", false
	}
	sizes := make([]int, 0, len(teams))
	named := make([][]string, 0, len(teams))
	for _, team := range teams {
		row := make([]string, 0, len(team))
		for _, pid := range team {
			name, ok := pidToName[pid]
			if !ok || name == "" {
				name = fmt.Sprintf("Player %d", pid)
			}
			row = append(row, name)
		}
		if len(row) == 0 {
			continue
		}
		named = append(named, row)
		sizes = append(sizes, len(team))
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	sizeStrs := make([]string, len(sizes))
	for i, s := range sizes {
		sizeStrs[i] = fmt.Sprintf("%d", s)
	}
	payload := map[string]any{
		"team_sizes":      strings.Join(sizeStrs, "v"),
		"teams":           named,
		"threshold_sec":   StackingThresholdSec,
		"band_started_at": teams,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(b), true
}
