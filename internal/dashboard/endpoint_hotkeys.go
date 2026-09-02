package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"sort"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

// hotkeySignatureMinGames gates the player hotkey-signature card: below this
// many games of a race the aggregation is noise, not a pattern.
const hotkeySignatureMinGames = 3

// hotkeySignatureMaxGamesPerRace caps how many recent games feed one card, so
// the signature tracks the player's current habits.
const hotkeySignatureMaxGamesPerRace = 40

type hotkeyTimelinePlayer struct {
	PlayerID int64  `json:"player_id"`
	Name     string `json:"name"`
	Race     string `json:"race"`
	Team     int64  `json:"team"`
	// Events are [sec, type, group, count, buildingID, tileX, tileY] tuples;
	// type is the internal/hotkeystream wire type (0=Select, 1=Assign units,
	// 2=Add, 3=Assign building).
	Events [][7]int `json:"events"`
	// Legacy marks a pre-v2 stream: times are real, annotations are absent.
	Legacy bool `json:"legacy,omitempty"`
}

type hotkeyTimelinePayload struct {
	DurationSeconds int64                  `json:"duration_seconds"`
	Players         []hotkeyTimelinePlayer `json:"players"`
	// Buildings maps the building IDs used in events to display names.
	Buildings map[byte]string `json:"buildings"`
}

// GameHotkeys returns both players' decoded hotkey streams for the game
// report's hotkey timeline tab.
func (d *Dashboard) GameHotkeys(ctx context.Context, request apigen.GameHotkeysRequestObject) (any, error) {
	replayID := int64(request.ReplayID)
	summary, err := d.dbStore.GetReplaySummary(ctx, replayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	rows, err := d.dbStore.ListReplayPlayerHotkeyStreams(ctx, replayID)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	payload := hotkeyTimelinePayload{
		DurationSeconds: summary.DurationSeconds,
		Players:         []hotkeyTimelinePlayer{},
		Buildings:       map[byte]string{},
	}
	for _, row := range rows {
		events, err := hotkeystream.Decode(row.HotkeyStream)
		if err != nil {
			log.Printf("game hotkeys replay=%d player=%d: %v", replayID, row.PlayerID, err)
			continue
		}
		p := hotkeyTimelinePlayer{
			PlayerID: row.PlayerID,
			Name:     row.Name,
			Race:     row.Race,
			Team:     row.Team,
			Events:   make([][7]int, 0, len(events)),
			Legacy:   isLegacyStream(row.HotkeyStream),
		}
		for _, e := range events {
			p.Events = append(p.Events, [7]int{int(e.Sec), int(e.Type), int(e.Group), int(e.Count), int(e.Building), int(e.TileX), int(e.TileY)})
			if e.Type == hotkeystream.TypeAssignBuilding && e.Building != 0 {
				payload.Buildings[e.Building] = hotkeystream.BuildingName(e.Building)
			}
		}
		payload.Players = append(payload.Players, p)
	}
	return payload, nil
}

// isLegacyStream reports whether a stored blob predates wire format v2 (no
// annotations; see internal/hotkeystream).
func isLegacyStream(blob []byte) bool {
	return len(blob) > 0 && !(len(blob) >= 2 && blob[0] == 0xFF && blob[1] == 0x02)
}

type hotkeySignaturePayload struct {
	Cards []*hotkeystream.Signature `json:"cards"`
	// GamesByRace reports every race seen, including ones below the card
	// threshold, so the frontend can explain why a race has no card.
	GamesByRace map[string]int `json:"games_by_race"`
}

// PlayerHotkeySignature aggregates a player's stored hotkey streams into one
// temporal signature card per race with at least hotkeySignatureMinGames
// games. A Random player naturally yields one card per race played.
func (d *Dashboard) PlayerHotkeySignature(ctx context.Context, request apigen.PlayerHotkeySignatureRequestObject) (any, error) {
	playerKey := normalizePlayerKey(string(request.PlayerKey))
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	rows, err := d.dbStore.ListPlayerHotkeyStreamsByKey(ctx, playerKey)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	displayName := playerKey
	byRace := map[string][][]hotkeystream.Event{}
	gamesByRace := map[string]int{}
	for _, row := range rows {
		if displayName == playerKey && row.Name != "" {
			displayName = row.Name
		}
		gamesByRace[row.Race]++
		if isLegacyStream(row.HotkeyStream) {
			continue
		}
		if len(byRace[row.Race]) >= hotkeySignatureMaxGamesPerRace {
			continue
		}
		events, err := hotkeystream.Decode(row.HotkeyStream)
		if err != nil || len(events) == 0 {
			continue
		}
		byRace[row.Race] = append(byRace[row.Race], events)
	}
	payload := hotkeySignaturePayload{Cards: []*hotkeystream.Signature{}, GamesByRace: gamesByRace}
	for race, games := range byRace {
		if len(games) < hotkeySignatureMinGames {
			continue
		}
		payload.Cards = append(payload.Cards, hotkeystream.ComputeSignature(displayName, race, games))
	}
	sort.Slice(payload.Cards, func(i, j int) bool { return payload.Cards[i].Games > payload.Cards[j].Games })
	return payload, nil
}
