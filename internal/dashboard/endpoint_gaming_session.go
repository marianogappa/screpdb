package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// A gaming session is the run of games you have just played. Two constants
// define it:
//
//	gamingSessionGap     the quiet period that ends a session. Games closer
//	                     together than this belong to the same sitting.
//	gamingSessionRecency how recently the last game must have been for the
//	                     session to still count as current, which is what makes
//	                     the nav entry appear.
//
// Both are the same span deliberately: a session is "games no more than this
// far apart, ending no longer ago than this".
const (
	gamingSessionGap     = 3 * time.Hour
	gamingSessionRecency = 3 * time.Hour
)

// gamingSessionPlayer is someone met during the session. Opponents and allies
// are the same shape but are reported separately: a win/loss record only means
// something against an opponent, and mixing the two made the list read as if
// the user had beaten their own team-mates.
type gamingSessionPlayer struct {
	PlayerKey   string             `json:"player_key"`
	PlayerName  string             `json:"player_name"`
	CountryCode string             `json:"country_code,omitempty"`
	Games       int                `json:"games"`
	Wins        int                `json:"wins"`
	Losses      int                `json:"losses"`
	Races       []string           `json:"races,omitempty"`
	APM         int                `json:"apm,omitempty"`
	Profile     *bnetProfileDetail `json:"profile,omitempty"`
}

type gamingSessionStats struct {
	Games           int            `json:"games"`
	Wins            int            `json:"wins"`
	Losses          int            `json:"losses"`
	WinRate         float64        `json:"win_rate"`
	AverageAPM      float64        `json:"average_apm"`
	AverageEAPM     float64        `json:"average_eapm"`
	DurationSeconds int64          `json:"duration_seconds"`
	PlayedSeconds   int64          `json:"played_seconds"`
	StartedAt       string         `json:"started_at"`
	EndedAt         string         `json:"ended_at"`
	Matchups        map[string]int `json:"matchups"`
	RacesPlayed     map[string]int `json:"races_played"`
	Maps            map[string]int `json:"maps"`
}

type gamingSessionResponse struct {
	Active    bool                   `json:"active"`
	PlayerKey string                 `json:"player_key,omitempty"`
	Stats     gamingSessionStats     `json:"stats"`
	Opponents []gamingSessionPlayer  `json:"opponents"`
	Allies    []gamingSessionPlayer  `json:"allies"`
	Games     []workflowGameListItem `json:"games"`
}

// sessionGameRow is one of the user's games, as the session builder needs it.
type sessionGameRow struct {
	ReplayID   int64
	PlayedAt   time.Time
	PlayerKey  string
	PlayerName string
}

// gamingSessionWindow walks the user's games newest-first and returns the
// bounds of the current session: every game reachable from the most recent one
// without a gap longer than gamingSessionGap. It returns ok=false when the most
// recent game is older than gamingSessionRecency, which is how the whole
// feature stays invisible outside a play session.
func gamingSessionWindow(rows []sessionGameRow, now time.Time) (start, end time.Time, count int, ok bool) {
	if len(rows) == 0 {
		return time.Time{}, time.Time{}, 0, false
	}
	latest := rows[0].PlayedAt
	if now.Sub(latest) > gamingSessionRecency {
		return time.Time{}, time.Time{}, 0, false
	}
	earliest := latest
	count = 1
	for i := 1; i < len(rows); i++ {
		if earliest.Sub(rows[i].PlayedAt) > gamingSessionGap {
			break
		}
		earliest = rows[i].PlayedAt
		count++
	}
	return earliest, latest, count, true
}

// autosaveOnly reports whether a replay came from StarCraft's own Autosave
// folder. Only autosaved replays evidence that the user actually sat down and
// played: a replay they downloaded or were sent says nothing about their
// session.
func autosaveOnly(filePath string) bool {
	normalized := strings.ReplaceAll(filePath, "\\", "/")
	return strings.Contains(strings.ToLower(normalized), "/autosave/")
}

func (d *Dashboard) gamingSession(ctx context.Context) (*gamingSessionResponse, error) {
	empty := &gamingSessionResponse{
		Opponents: []gamingSessionPlayer{},
		Allies:    []gamingSessionPlayer{},
		Games:     []workflowGameListItem{},
		Stats:     gamingSessionStats{Matchups: map[string]int{}, RacesPlayed: map[string]int{}, Maps: map[string]int{}},
	}
	youKeys := d.loadYouKeys()
	if len(youKeys) == 0 {
		return empty, nil
	}
	rows, err := d.dbStore.ListRecentAutosaveGamesForPlayers(ctx, sortedKeys(youKeys), gamingSessionCandidateLimit)
	if err != nil {
		return nil, fmt.Errorf("loading session candidates: %w", err)
	}
	candidates := make([]sessionGameRow, 0, len(rows))
	for _, row := range rows {
		if !autosaveOnly(row.FilePath) {
			continue
		}
		playedAt, err := parseReplayDate(row.ReplayDate)
		if err != nil {
			continue
		}
		candidates = append(candidates, sessionGameRow{
			ReplayID:   row.ReplayID,
			PlayedAt:   playedAt,
			PlayerKey:  row.PlayerKey,
			PlayerName: row.PlayerName,
		})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].PlayedAt.After(candidates[j].PlayedAt) })

	_, _, count, ok := gamingSessionWindow(candidates, time.Now())
	if !ok || count == 0 {
		return empty, nil
	}
	sessionRows := candidates[:count]
	replayIDs := make([]int64, 0, len(sessionRows))
	for _, row := range sessionRows {
		replayIDs = append(replayIDs, row.ReplayID)
	}

	games, err := d.workflowGameListItemsByReplayIDs(replayIDs)
	if err != nil {
		return nil, err
	}
	apmByGamePlayer, err := d.sessionAPMByGamePlayer(ctx, replayIDs)
	if err != nil {
		return nil, err
	}
	resp := empty
	resp.Active = true
	resp.PlayerKey = sessionRows[0].PlayerKey
	resp.Games = games
	resp.Stats = summarizeGamingSession(sessionRows, games, apmByGamePlayer, youKeys)
	opponents, allies := gamingSessionPlayers(games, apmByGamePlayer, youKeys)
	resp.Opponents = d.withBnetProfiles(ctx, opponents)
	resp.Allies = d.withBnetProfiles(ctx, allies)
	return resp, nil
}

// gamingSessionCandidateLimit bounds the newest-first scan the session builder
// walks. A session cannot plausibly contain more games than this, and the walk
// stops at the first gap anyway, so it only ever caps pathological data.
const gamingSessionCandidateLimit = 300

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func summarizeGamingSession(rows []sessionGameRow, games []workflowGameListItem, apm map[gamePlayerKey]sessionAPM, youKeys map[string]struct{}) gamingSessionStats {
	stats := gamingSessionStats{
		Matchups:    map[string]int{},
		RacesPlayed: map[string]int{},
		Maps:        map[string]int{},
	}
	if len(rows) == 0 {
		return stats
	}
	stats.Games = len(games)
	stats.StartedAt = rows[len(rows)-1].PlayedAt.Format(time.RFC3339)
	stats.EndedAt = rows[0].PlayedAt.Format(time.RFC3339)
	stats.DurationSeconds = int64(rows[0].PlayedAt.Sub(rows[len(rows)-1].PlayedAt).Seconds())

	var apmSum, eapmSum float64
	var apmCount int
	for _, game := range games {
		stats.PlayedSeconds += game.DurationSeconds
		if game.MapName != "" {
			stats.Maps[game.MapName]++
		}
		if game.Matchup != "" {
			stats.Matchups[game.Matchup]++
		}
		for _, player := range game.Players {
			if _, mine := youKeys[normalizePlayerKey(player.PlayerKey)]; !mine {
				continue
			}
			if player.Race != "" {
				stats.RacesPlayed[player.Race]++
			}
			if player.IsWinner {
				stats.Wins++
			} else {
				stats.Losses++
			}
			if own, ok := apm[gamePlayerKey{ReplayID: game.ReplayID, PlayerKey: normalizePlayerKey(player.PlayerKey)}]; ok && own.APM > 0 {
				apmSum += float64(own.APM)
				eapmSum += float64(own.EAPM)
				apmCount++
			}
		}
	}
	if apmCount > 0 {
		stats.AverageAPM = apmSum / float64(apmCount)
		stats.AverageEAPM = eapmSum / float64(apmCount)
	}
	if decided := stats.Wins + stats.Losses; decided > 0 {
		stats.WinRate = float64(stats.Wins) / float64(decided)
	}
	return stats
}

// gamingSessionPlayers splits everyone the user shared a game with into the
// people they played against and the people they played with. Wins and losses
// are from the user's point of view and are only tallied for opponents; an ally
// shares the user's result, so a record against them would be meaningless.
func gamingSessionPlayers(games []workflowGameListItem, apm map[gamePlayerKey]sessionAPM, youKeys map[string]struct{}) (opponents, allies []gamingSessionPlayer) {
	type accumulator struct {
		player  *gamingSessionPlayer
		races   map[string]struct{}
		apmSum  int
		apmSeen int
	}
	opponentAcc := map[string]*accumulator{}
	allyAcc := map[string]*accumulator{}

	for _, game := range games {
		youWon := false
		var youTeam int64 = -1
		for _, player := range game.Players {
			if _, mine := youKeys[normalizePlayerKey(player.PlayerKey)]; !mine {
				continue
			}
			youWon = player.IsWinner
			youTeam = player.Team
			break
		}
		if youTeam < 0 {
			continue
		}
		for _, player := range game.Players {
			key := normalizePlayerKey(player.PlayerKey)
			if _, mine := youKeys[key]; mine || key == "" {
				continue
			}
			isAlly := player.Team == youTeam
			bucket := opponentAcc
			if isAlly {
				bucket = allyAcc
			}
			entry, ok := bucket[key]
			if !ok {
				entry = &accumulator{
					player: &gamingSessionPlayer{PlayerKey: key, PlayerName: player.Name, CountryCode: player.CountryCode},
					races:  map[string]struct{}{},
				}
				bucket[key] = entry
			}
			entry.player.Games++
			if entry.player.CountryCode == "" {
				entry.player.CountryCode = player.CountryCode
			}
			if player.Race != "" {
				entry.races[player.Race] = struct{}{}
			}
			if stat, ok := apm[gamePlayerKey{ReplayID: game.ReplayID, PlayerKey: key}]; ok && stat.APM > 0 {
				entry.apmSum += int(stat.APM)
				entry.apmSeen++
			}
			if isAlly {
				continue
			}
			if youWon {
				entry.player.Losses++
			} else {
				entry.player.Wins++
			}
		}
	}

	finish := func(bucket map[string]*accumulator) []gamingSessionPlayer {
		out := make([]gamingSessionPlayer, 0, len(bucket))
		for _, entry := range bucket {
			races := make([]string, 0, len(entry.races))
			for race := range entry.races {
				races = append(races, race)
			}
			sort.Strings(races)
			entry.player.Races = races
			if entry.apmSeen > 0 {
				entry.player.APM = entry.apmSum / entry.apmSeen
			}
			out = append(out, *entry.player)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Games != out[j].Games {
				return out[i].Games > out[j].Games
			}
			return strings.ToLower(out[i].PlayerName) < strings.ToLower(out[j].PlayerName)
		})
		return out
	}
	return finish(opponentAcc), finish(allyAcc)
}

// withBnetProfiles attaches whatever cached Battle.net profile we already hold
// for each player. Cache-only, so it spends no bridge budget.
func (d *Dashboard) withBnetProfiles(ctx context.Context, players []gamingSessionPlayer) []gamingSessionPlayer {
	if len(players) == 0 {
		return players
	}
	keys := make([]string, 0, len(players))
	for _, player := range players {
		keys = append(keys, player.PlayerKey)
	}
	details := d.bnetProfileDetailsByPlayerKeys(ctx, keys)
	for i := range players {
		if detail, ok := details[players[i].PlayerKey]; ok {
			players[i].Profile = detail
			if players[i].CountryCode == "" {
				players[i].CountryCode = detail.CountryCode
			}
		}
	}
	return players
}

func (d *Dashboard) handlerGamingSession(w http.ResponseWriter, r *http.Request) {
	if !d.featureFlagEnabled(r.Context(), featureFlagGamingSession) {
		http.Error(w, "gaming session is not enabled", http.StatusNotFound)
		return
	}
	session, err := d.gamingSession(r.Context())
	if err != nil {
		http.Error(w, "failed to build gaming session", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(session)
}

// sessionAPM is one player's APM in one game.
type sessionAPM struct {
	APM  int64
	EAPM int64
}

// gamePlayerKey identifies one player in one game, which is the grain the
// session needs: the same person appears in several games, and the same game
// holds several people.
type gamePlayerKey struct {
	ReplayID  int64
	PlayerKey string
}

func (d *Dashboard) sessionAPMByGamePlayer(ctx context.Context, replayIDs []int64) (map[gamePlayerKey]sessionAPM, error) {
	rows, err := d.dbStore.ListPlayerAPMByReplayIDs(ctx, replayIDs)
	if err != nil {
		return nil, fmt.Errorf("loading session APM: %w", err)
	}
	out := make(map[gamePlayerKey]sessionAPM, len(rows))
	for _, row := range rows {
		out[gamePlayerKey{ReplayID: row.ReplayID, PlayerKey: row.PlayerKey}] = sessionAPM{APM: row.APM, EAPM: row.EAPM}
	}
	return out, nil
}

// workflowGameListItemsByReplayIDs builds the same game rows the games list
// renders, for an explicit set of replays, so the session view can reuse that
// rendering wholesale instead of growing a second shape for the same thing.
func (d *Dashboard) workflowGameListItemsByReplayIDs(replayIDs []int64) ([]workflowGameListItem, error) {
	rows, err := d.dbStore.ListReplaysByIDs(d.ctx, replayIDs)
	if err != nil {
		return nil, fmt.Errorf("loading session games: %w", err)
	}
	items := make([]workflowGameListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, workflowGameListItem{
			ReplayID:           row.ReplayID,
			ReplayDate:         row.ReplayDate,
			FileName:           row.FileName,
			MapName:            row.MapName,
			MapKind:            row.MapKind,
			GameSource:         row.GameSource,
			LobbyKind:          row.LobbyKind,
			DurationSeconds:    row.DurationSeconds,
			GameType:           row.GameType,
			Matchup:            row.Matchup,
			TeamStacking:       row.TeamStacking,
			TeamInfoIncomplete: row.TeamInfoIncomplete,
			Players:            []workflowGameListPlayer{},
			Featuring:          []string{},
		})
	}
	if err := d.populateWorkflowGameListPlayers(items); err != nil {
		return nil, err
	}
	if err := d.populateWorkflowGameListFeaturing(items); err != nil {
		return nil, err
	}
	return items, nil
}

// replayDateLayouts covers how replay_date is written to SQLite. The first is
// Go's time.Time.String() form, which is what the ingest path stores; the rest
// are there so a hand-edited or older row still parses rather than silently
// dropping a game out of the session.
var replayDateLayouts = []string{
	"2006-01-02 15:04:05 -0700 MST",
	"2006-01-02 15:04:05 -0700",
	time.RFC3339,
	"2006-01-02 15:04:05",
}

func parseReplayDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	for _, layout := range replayDateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised replay date %q", value)
}
