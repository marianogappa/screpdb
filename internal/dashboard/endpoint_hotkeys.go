package dashboard

import (
	"context"
	"errors"
	"github.com/marianogappa/screpdb/internal/propack"
	"log"
	"net/http"
	"sort"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
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
	// Events are [sec, type, group, count, buildingID, tileX, tileY, unitType, unitCount] tuples;
	// type is the internal/hotkeystream wire type (0=Select, 1=Assign units,
	// 2=Add, 3=Assign building). unitType and unitCount come from the engine
	// (0 when the replay has no engine data): the group's main unit and how
	// many units it held.
	Events [][9]int `json:"events"`
	// Compositions maps a unit assign's event index to every unit type the group held, as
	// [unitType, count] pairs, most valuable first. Only engine digests from EngineVersion 14 have it.
	Compositions map[int][][2]int `json:"compositions,omitempty"`
	// Legacy marks a pre-v2 stream: times are real, annotations are absent.
	Legacy bool `json:"legacy,omitempty"`
}

type hotkeyTimelinePayload struct {
	DurationSeconds int64                  `json:"duration_seconds"`
	Players         []hotkeyTimelinePlayer `json:"players"`
	// Buildings maps the building IDs used in events to display names.
	Buildings map[byte]string `json:"buildings"`
	// Units maps the unit types used in events to display names.
	Units map[uint16]string `json:"units"`
	// UnitTypesExact is true for an engine game: its unit assigns carry the real units the engine saw.
	UnitTypesExact bool `json:"unit_types_exact"`
}

const (
	evSec = iota
	evType
	evGroup
	evCount
	evBuilding
	evTileX
	evTileY
	evUnitType
	evUnitCount
)

// GameHotkeys returns both players' decoded hotkey streams for the game
// report's hotkey timeline tab.
func (d *Dashboard) GameHotkeys(ctx context.Context, request apigen.GameHotkeysRequestObject) (any, error) {
	replayID := int64(request.ReplayID)
	summary, err := d.dbStore.GetReplaySummary(ctx, replayID)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
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
		Units:           map[uint16]string{},
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
			Events:   make([][9]int, 0, len(events)),
			Legacy:   isLegacyStream(row.HotkeyStream),
		}
		for _, e := range events {
			p.Events = append(p.Events, [9]int{int(e.Sec), int(e.Type), int(e.Group), int(e.Count), int(e.Building), int(e.TileX), int(e.TileY)})
			if e.Type == hotkeystream.TypeAssignBuilding && e.Building != 0 {
				payload.Buildings[e.Building] = hotkeystream.BuildingName(e.Building)
			}
		}
		payload.Players = append(payload.Players, p)
	}
	d.applyEngineHotkeys(replayID, &payload)
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
	// Twins is, on a built-in profile, the other built-in pro whose hotkey layout is closest to each card's,
	// by race. Local players get none: a pro in the user's own replays would match their own built-in profile.
	Twins map[string]hotkeyTwin `json:"twins,omitempty"`
	// Oddities are a built-in pro's conspicuous hotkey habits, by race.
	Oddities map[string][]propack.HotkeyOddity `json:"oddities,omitempty"`
	// Rates are the player's median hotkey tempo, by race.
	Rates map[string]hotkeystream.Rates `json:"rates,omitempty"`
}

type hotkeyTwin struct {
	PlayerKey  string  `json:"player_key"`
	PlayerName string  `json:"player_name"`
	PhotoURL   string  `json:"photo_url,omitempty"`
	Similarity float64 `json:"similarity"`
}

// hotkeyTwinMinSimilarity keeps the twin to layouts that genuinely match. Pros' nearest other pro has a median
// similarity of 0.86, and two accounts of the same player score above 0.9.
const hotkeyTwinMinSimilarity = 0.9

func (d *Dashboard) hotkeyTwins(cards []*hotkeystream.Signature, selfProID string) map[string]hotkeyTwin {
	pack := d.loadProPack()
	if pack == nil {
		return nil
	}
	excluded := d.excludedProIDs(pack)
	twins := map[string]hotkeyTwin{}
	for _, card := range cards {
		best := hotkeyTwin{Similarity: hotkeyTwinMinSimilarity}
		for i := range pack.Pros {
			pro := &pack.Pros[i]
			if pro.ID == selfProID || excluded[pro.ID] {
				continue
			}
			for _, proCard := range pro.Hotkeys {
				if proCard.Race != card.Race {
					continue
				}
				if sim := hotkeystream.LayoutSimilarity(card, proCard); sim > best.Similarity {
					best = hotkeyTwin{PlayerKey: pro.Key(), PlayerName: pro.Label, PhotoURL: proPhotoURL(pro), Similarity: sim}
				}
			}
		}
		if best.PlayerKey != "" {
			twins[card.Race] = best
		}
	}
	return twins
}

// PlayerHotkeySignature aggregates a player's stored hotkey streams into one
// temporal signature card per race with at least hotkeySignatureMinGames
// games. A Random player naturally yields one card per race played.
func (d *Dashboard) PlayerHotkeySignature(ctx context.Context, request apigen.PlayerHotkeySignatureRequestObject) (any, error) {
	playerKey := normalizePlayerKey(string(request.PlayerKey))
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		pro := d.featuredPro(playerKey)
		if pro == nil {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, errors.New("unknown built-in profile"))
		}
		payload := d.featuredHotkeySignature(pro)
		payload.Twins = d.hotkeyTwins(payload.Cards, pro.ID)
		payload.Oddities = pro.HotkeyOddities
		payload.Rates = pro.HotkeyRates
		return payload, nil
	}
	payload, err := d.localHotkeySignature(ctx, playerKey)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return payload, nil
}

// localHotkeySignature aggregates the stored hotkey streams of a player in the
// database. The pro pack generator calls it too, on its scratch corpus.
func (d *Dashboard) localHotkeySignature(ctx context.Context, playerKey string) (hotkeySignaturePayload, error) {
	rows, err := d.dbStore.ListPlayerHotkeyStreamsByKey(ctx, playerKey)
	if err != nil {
		return hotkeySignaturePayload{}, err
	}
	displayName := playerKey
	byRace := map[string][][]hotkeystream.Event{}
	ratesByRace := map[string][]hotkeystream.Rates{}
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
		d.engineHotkeyUnits(row.ReplayID, row.Slot, events)
		byRace[row.Race] = append(byRace[row.Race], events)
		if r, ok := hotkeystream.GameRates(events, int(row.DurationSeconds)); ok {
			ratesByRace[row.Race] = append(ratesByRace[row.Race], r)
		}
	}
	payload := hotkeySignaturePayload{Cards: hotkeySignatureCards(displayName, byRace), GamesByRace: gamesByRace, Rates: map[string]hotkeystream.Rates{}}
	for _, card := range payload.Cards {
		if rates := ratesByRace[card.Race]; len(rates) > 0 {
			payload.Rates[card.Race] = hotkeystream.MedianRates(rates)
		}
	}
	return payload, nil
}

// hotkeySignatureCards is one card per race with enough games, most played first. A race whose games never
// touch a hotkey has nothing to show, so it gets no card, and a player with no cards gets no Hotkeys tab.
func hotkeySignatureCards(displayName string, byRace map[string][][]hotkeystream.Event) []*hotkeystream.Signature {
	cards := []*hotkeystream.Signature{}
	for race, games := range byRace {
		if len(games) < hotkeySignatureMinGames {
			continue
		}
		if card := hotkeystream.ComputeSignature(displayName, race, games); len(card.Keys) > 0 {
			cards = append(cards, card)
		}
	}
	sort.Slice(cards, func(i, j int) bool { return cards[i].Games > cards[j].Games })
	return cards
}
