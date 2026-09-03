package dashboard

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
)

// The profile payload the SC:R bridge returns is large and mostly avatars and
// per-race lifetime counters. These are the parts worth surfacing: who the
// account belongs to, what else they play as, and whether they ladder.
type bnetProfileToon struct {
	Toon          string `json:"toon"`
	Gateway       int    `json:"gateway,omitempty"`
	GatewayName   string `json:"gateway_name,omitempty"`
	GamesLastWeek int    `json:"games_last_week"`
}

type bnetProfileDetail struct {
	Toon         string            `json:"toon"`
	AuroraID     int64             `json:"aurora_id,omitempty"`
	BattleTag    string            `json:"battle_tag,omitempty"`
	CountryCode  string            `json:"country_code,omitempty"`
	Toons        []bnetProfileToon `json:"toons,omitempty"`
	PlaysLadder  bool              `json:"plays_ladder"`
	MMR          int               `json:"mmr,omitempty"`
	HighestMMR   int               `json:"highest_mmr,omitempty"`
	LadderWins   int               `json:"ladder_wins,omitempty"`
	LadderLosses int               `json:"ladder_losses,omitempty"`
	// Lifetime account totals summed from the per-race counters Battle.net
	// reports (wins/losses/draws/disconnects per race, plus per-game APM
	// sums, from which the average APM is derived).
	LifetimeGames       int     `json:"lifetime_games,omitempty"`
	LifetimeWins        int     `json:"lifetime_wins,omitempty"`
	LifetimeLosses      int     `json:"lifetime_losses,omitempty"`
	LifetimeDisconnects int     `json:"lifetime_disconnects,omitempty"`
	AverageAPM          float64 `json:"average_apm,omitempty"`
	// PlayTimeSeconds sums the per-race lifetime play_time counters.
	PlayTimeSeconds int64 `json:"play_time_seconds,omitempty"`
	// GamesLastWeek sums Battle.net's own games_last_week over the account's
	// toons; LastPlayedAt is the newest game in game_results (RFC3339, UTC).
	GamesLastWeek int    `json:"games_last_week"`
	LastPlayedAt  string `json:"last_played_at,omitempty"`
	// RecentGames is the account's game_results list, newest first: the last
	// ~20 games Battle.net remembers, whichever toon they were played on.
	RecentGames []bnetRecentGame `json:"recent_games,omitempty"`
	// Habits is filled by the player page from the rolling game cache.
	Habits *bnetPlayHabits `json:"habits,omitempty"`
}

// bnetRecentGame is one entry of the profile's game_results, seen from this
// account's side. Nothing here allows downloading the replay.
type bnetRecentGame struct {
	PlayedAt        string               `json:"played_at"`
	GameID          string               `json:"game_id,omitempty"`
	MatchGUID       string               `json:"match_guid,omitempty"`
	Gateway         int                  `json:"gateway,omitempty"`
	GatewayName     string               `json:"gateway_name,omitempty"`
	MapName         string               `json:"map_name"`
	Toon            string               `json:"toon,omitempty"`
	Race            string               `json:"race,omitempty"`
	Result          string               `json:"result"`
	APM             int                  `json:"apm,omitempty"`
	DurationSeconds int                  `json:"duration_seconds,omitempty"`
	Opponents       []bnetRecentOpponent `json:"opponents,omitempty"`
}

type bnetRecentOpponent struct {
	Toon string `json:"toon"`
	Race string `json:"race,omitempty"`
}

// rawBnetProfile is the subset of the bridge payload we decode. Everything else
// (avatars, per-race lifetime counters, replay lists) is ignored.
type rawBnetProfile struct {
	AuroraID    int64  `json:"aurora_id"`
	BattleTag   string `json:"battle_tag"`
	CountryCode string `json:"country_code"`
	Toons       []struct {
		Toon          string `json:"toon"`
		GatewayID     int    `json:"gateway_id"`
		GamesLastWeek int    `json:"games_last_week"`
	} `json:"toons"`
	MatchmakedStats []struct {
		Rating        int `json:"rating"`
		HighestRating int `json:"highest_rating"`
		Wins          int `json:"wins"`
		Losses        int `json:"losses"`
	} `json:"matchmaked_stats"`
	Stats []struct {
		Raw map[string]float64 `json:"raw"`
	} `json:"stats"`
	GameResults []rawBnetGameResult `json:"game_results"`
}

// rawBnetGameResult is one game_results entry. Numbers arrive as strings.
type rawBnetGameResult struct {
	Attributes struct {
		MapName string `json:"mapName"`
	} `json:"attributes"`
	CreateTime string `json:"create_time"`
	GameID     string `json:"game_id"`
	GatewayID  int    `json:"gateway_id"`
	MatchGUID  string `json:"match_guid"`
	Players    []struct {
		Attributes struct {
			Race string `json:"race"`
			Type string `json:"type"`
		} `json:"attributes"`
		Result string            `json:"result"`
		Stats  map[string]string `json:"stats"`
		Toon   string            `json:"toon"`
	} `json:"players"`
}

// parseBnetProfileDetail extracts the displayable parts of a cached profile
// payload. A payload we cannot decode yields no detail rather than an error:
// this is decoration on a page that must render regardless.
func parseBnetProfileDetail(toon string, payload []byte) *bnetProfileDetail {
	if len(payload) == 0 {
		return nil
	}
	var raw rawBnetProfile
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil
	}
	detail := &bnetProfileDetail{
		Toon:        toon,
		AuroraID:    raw.AuroraID,
		BattleTag:   strings.TrimSpace(raw.BattleTag),
		CountryCode: strings.TrimSpace(raw.CountryCode),
	}
	for _, t := range raw.Toons {
		name := strings.TrimSpace(t.Toon)
		if name == "" {
			continue
		}
		detail.Toons = append(detail.Toons, bnetProfileToon{
			Toon:          name,
			Gateway:       t.GatewayID,
			GatewayName:   bnetfacade.GatewayNames[t.GatewayID],
			GamesLastWeek: t.GamesLastWeek,
		})
	}
	// Most played first, so the account someone actually uses leads.
	sort.SliceStable(detail.Toons, func(i, j int) bool {
		if detail.Toons[i].GamesLastWeek != detail.Toons[j].GamesLastWeek {
			return detail.Toons[i].GamesLastWeek > detail.Toons[j].GamesLastWeek
		}
		return strings.ToLower(detail.Toons[i].Toon) < strings.ToLower(detail.Toons[j].Toon)
	})

	// A player can hold several matchmaking records (per season and mode). The
	// best current rating is the meaningful headline; wins/losses are summed
	// across records so "laddered at all" is not hidden by an empty season.
	for _, stat := range raw.MatchmakedStats {
		detail.PlaysLadder = true
		if stat.Rating > detail.MMR {
			detail.MMR = stat.Rating
		}
		if stat.HighestRating > detail.HighestMMR {
			detail.HighestMMR = stat.HighestRating
		}
		detail.LadderWins += stat.Wins
		detail.LadderLosses += stat.Losses
	}
	// Lifetime totals: Battle.net reports per-race counters; sum them and
	// derive the average APM from the per-game APM sums.
	apmSum := 0.0
	for _, stat := range raw.Stats {
		for _, race := range []string{"zerg", "terran", "protoss"} {
			wins := int(stat.Raw[race+"_wins_sum"])
			losses := int(stat.Raw[race+"_losses_sum"])
			draws := int(stat.Raw[race+"_draws_sum"])
			disconnects := int(stat.Raw[race+"_disconnects_sum"])
			detail.LifetimeWins += wins
			detail.LifetimeLosses += losses
			detail.LifetimeDisconnects += disconnects
			detail.LifetimeGames += wins + losses + draws + disconnects
			apmSum += stat.Raw[race+"_apm_sum"]
		}
	}
	if detail.LifetimeGames > 0 {
		detail.AverageAPM = apmSum / float64(detail.LifetimeGames)
	}
	for _, stat := range raw.Stats {
		for _, race := range []string{"zerg", "terran", "protoss"} {
			detail.PlayTimeSeconds += int64(stat.Raw[race+"_play_time_sum"])
		}
	}
	for _, t := range raw.Toons {
		detail.GamesLastWeek += t.GamesLastWeek
	}
	detail.RecentGames = parseBnetRecentGames(raw)
	if len(detail.RecentGames) > 0 {
		detail.LastPlayedAt = detail.RecentGames[0].PlayedAt
	}
	return detail
}

// parseBnetRecentGames reads game_results from the account's side: the entry
// whose toon is one of the account's toons is "us", every other human is an
// opponent. Games where no toon of ours appears are kept without a side.
func parseBnetRecentGames(raw rawBnetProfile) []bnetRecentGame {
	ours := map[string]bool{}
	for _, t := range raw.Toons {
		if key := normalizePlayerKey(t.Toon); key != "" {
			ours[key] = true
		}
	}
	games := make([]bnetRecentGame, 0, len(raw.GameResults))
	for _, g := range raw.GameResults {
		createTime, err := strconv.ParseInt(strings.TrimSpace(g.CreateTime), 10, 64)
		if err != nil || createTime <= 0 {
			continue
		}
		game := bnetRecentGame{
			PlayedAt:    time.Unix(createTime, 0).UTC().Format(time.RFC3339),
			GameID:      g.GameID,
			MatchGUID:   g.MatchGUID,
			Gateway:     g.GatewayID,
			GatewayName: bnetfacade.GatewayNames[g.GatewayID],
			MapName:     stripBnetControlChars(g.Attributes.MapName),
			Result:      "unknown",
		}
		for _, p := range g.Players {
			if p.Attributes.Type != "player" {
				continue
			}
			race := prettyBnetRace(p.Attributes.Race)
			playTime, _ := strconv.Atoi(p.Stats[p.Attributes.Race+"_play_time"])
			if playTime > game.DurationSeconds {
				game.DurationSeconds = playTime
			}
			if ours[normalizePlayerKey(p.Toon)] && game.Toon == "" {
				game.Toon = p.Toon
				game.Race = race
				if p.Result != "" {
					game.Result = p.Result
				}
				game.APM, _ = strconv.Atoi(p.Stats[p.Attributes.Race+"_apm"])
				continue
			}
			game.Opponents = append(game.Opponents, bnetRecentOpponent{Toon: p.Toon, Race: race})
		}
		games = append(games, game)
	}
	sort.SliceStable(games, func(i, j int) bool { return games[i].PlayedAt > games[j].PlayedAt })
	return games
}

// stripBnetControlChars drops the colour-code bytes (0x01-0x1f) Battle.net
// leaves in map titles ("\x07KnockOut \x051.4").
func stripBnetControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func prettyBnetRace(race string) string {
	switch strings.ToLower(strings.TrimSpace(race)) {
	case "zerg":
		return "Zerg"
	case "terran":
		return "Terran"
	case "protoss":
		return "Protoss"
	case "random":
		return "Random"
	}
	return ""
}

// bnetProfileDetailsByPlayerKeys reads cached profiles only. It never fetches,
// so calling it costs no bridge budget and cannot block on the network.
func (d *Dashboard) bnetProfileDetailsByPlayerKeys(ctx context.Context, playerKeys []string) map[string]*bnetProfileDetail {
	out := map[string]*bnetProfileDetail{}
	if len(playerKeys) == 0 {
		return out
	}
	rows, err := d.dbStore.ListBnetProfilePayloadsByPlayerKeys(ctx, playerKeys)
	if err != nil {
		return out
	}
	for _, row := range rows {
		detail := parseBnetProfileDetail(row.Toon, []byte(row.Payload))
		if detail == nil {
			continue
		}
		key := normalizePlayerKey(row.Toon)
		// A toon can be cached under several gateways; keep the richest row.
		if existing, ok := out[key]; ok && bnetProfileDetailScore(existing) >= bnetProfileDetailScore(detail) {
			continue
		}
		out[key] = detail
	}
	return out
}

// bnetProfileDetailScore ranks two cached rows for the same toon so the one
// carrying more information wins. Ladder data is the scarcest, then alternate
// toons, then a battle tag.
func bnetProfileDetailScore(detail *bnetProfileDetail) int {
	score := 0
	if detail.PlaysLadder {
		score += 100
	}
	score += len(detail.Toons)
	if detail.BattleTag != "" {
		score++
	}
	return score
}
