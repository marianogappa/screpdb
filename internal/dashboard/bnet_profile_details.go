package dashboard

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

// The distilled profile record still carries more than the pages surface.
// These are the parts worth showing: who the account belongs to, what else
// they play as, and whether they ladder.
type bnetProfileToon struct {
	Toon           string `json:"toon"`
	Gateway        int    `json:"gateway,omitempty"`
	GatewayName    string `json:"gateway_name,omitempty"`
	GamesLastWeek  int    `json:"games_last_week"`
	LocalPlayerKey string `json:"local_player_key,omitempty"`
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
	// toons; LastPlayedAt is the newest game in the archive (RFC3339, UTC).
	GamesLastWeek int    `json:"games_last_week"`
	LastPlayedAt  string `json:"last_played_at,omitempty"`
	// RecentGames is the account's slice of the game archive, newest first. It
	// accumulates across fetches, so it is not capped at the ~20 games one
	// Battle.net response carries.
	RecentGames []bnetRecentGame `json:"recent_games,omitempty"`
	// Habits is filled by the player page from the game archive.
	Habits *bnetPlayHabits `json:"habits,omitempty"`
}

// bnetRecentGame is one archived game, seen from this account's side. Nothing
// here allows downloading the replay.
type bnetRecentGame struct {
	PlayedAt        string               `json:"played_at"`
	GameID          string               `json:"game_id,omitempty"`
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

// bnetProfileDetailFromRecord assembles the displayable parts of a distilled
// profile and the account's archived games.
func bnetProfileDetailFromRecord(p persist.BnetProfile, games []persist.BnetGame) *bnetProfileDetail {
	detail := &bnetProfileDetail{
		Toon:        p.Toon,
		AuroraID:    p.AuroraID,
		BattleTag:   p.BattleTag,
		CountryCode: p.CountryCode,
	}
	// The bridge lists a toon once per gateway it exists on, so the same name
	// arrives several times. Collapse them onto the normalised key, keeping the
	// first gateway seen and summing the per-gateway weekly counts.
	toonIndex := map[string]int{}
	for _, t := range p.Toons {
		if i, ok := toonIndex[normalizePlayerKey(t.Toon)]; ok {
			detail.Toons[i].GamesLastWeek += t.GamesLastWeek
			continue
		}
		toonIndex[normalizePlayerKey(t.Toon)] = len(detail.Toons)
		detail.Toons = append(detail.Toons, bnetProfileToon{
			Toon:          t.Toon,
			Gateway:       t.Gateway,
			GatewayName:   bnetfacade.GatewayNames[t.Gateway],
			GamesLastWeek: t.GamesLastWeek,
		})
	}
	for _, t := range p.Toons {
		detail.GamesLastWeek += t.GamesLastWeek
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
	for _, stat := range p.Ladder {
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
	for _, row := range p.Lifetime {
		for _, race := range []string{"zerg", "terran", "protoss"} {
			totals := row.Race[race]
			detail.LifetimeWins += totals.Wins
			detail.LifetimeLosses += totals.Losses
			detail.LifetimeDisconnects += totals.Disconnects
			detail.LifetimeGames += totals.Wins + totals.Losses + totals.Draws + totals.Disconnects
			apmSum += totals.APMSum
			detail.PlayTimeSeconds += totals.PlayTimeSec
		}
	}
	if detail.LifetimeGames > 0 {
		detail.AverageAPM = apmSum / float64(detail.LifetimeGames)
	}
	detail.RecentGames = bnetRecentGamesFromArchive(p, games)
	if len(detail.RecentGames) > 0 {
		detail.LastPlayedAt = detail.RecentGames[0].PlayedAt
	}
	return detail
}

// bnetRecentGamesFromArchive reads the account's games from its side: the
// player whose toon is one of the account's toons is "us", every other human
// is an opponent. Games where no toon of ours appears are kept without a side.
func bnetRecentGamesFromArchive(p persist.BnetProfile, games []persist.BnetGame) []bnetRecentGame {
	ours := map[string]bool{}
	for _, t := range p.Toons {
		if key := normalizePlayerKey(t.Toon); key != "" {
			ours[key] = true
		}
	}
	out := make([]bnetRecentGame, 0, len(games))
	for _, g := range games {
		if g.CreateTime.IsZero() {
			continue
		}
		game := bnetRecentGame{
			PlayedAt:    g.CreateTime.UTC().Format(time.RFC3339),
			GameID:      g.GameID,
			Gateway:     g.Gateway,
			GatewayName: bnetfacade.GatewayNames[g.Gateway],
			MapName:     g.MapName,
			Result:      persist.BnetResultString(persist.BnetResultUnknown),
		}
		for _, player := range g.Players {
			if player.Computer {
				continue
			}
			if player.Seconds > game.DurationSeconds {
				game.DurationSeconds = player.Seconds
			}
			if ours[normalizePlayerKey(player.Toon)] && game.Toon == "" {
				game.Toon = player.Toon
				game.Race = prettyBnetRace(player.Race)
				if player.Result != persist.BnetResultUnknown {
					game.Result = persist.BnetResultString(player.Result)
				}
				game.APM = player.APM
				continue
			}
			game.Opponents = append(game.Opponents, bnetRecentOpponent{Toon: player.Toon, Race: prettyBnetRace(player.Race)})
		}
		out = append(out, game)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].PlayedAt > out[j].PlayedAt })
	return out
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

// bnetProfileDetailsByPlayerKeys reads the caches only. It never fetches, so
// calling it costs no bridge budget and cannot block on the network or disk.
func (d *Dashboard) bnetProfileDetailsByPlayerKeys(ctx context.Context, playerKeys []string) map[string]*bnetProfileDetail {
	out := map[string]*bnetProfileDetail{}
	if len(playerKeys) == 0 {
		return out
	}
	profiles, err := d.dbStore.ListBnetProfilesByPlayerKeys(ctx, playerKeys)
	if err != nil {
		return out
	}
	gamesByAccount := map[int64][]persist.BnetGame{}
	for _, p := range profiles {
		if _, ok := gamesByAccount[p.AuroraID]; !ok {
			games, gamesErr := d.dbStore.ListBnetGamesByAccount(ctx, p.AuroraID)
			if gamesErr != nil {
				games = nil
			}
			gamesByAccount[p.AuroraID] = games
		}
		detail := bnetProfileDetailFromRecord(p, gamesByAccount[p.AuroraID])
		key := normalizePlayerKey(p.Toon)
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

func (d *Dashboard) fillLocalPlayerKeys(ctx context.Context, profile *bnetProfileDetail) {
	if profile == nil || len(profile.Toons) == 0 {
		return
	}
	names := make([]string, 0, len(profile.Toons))
	for _, toon := range profile.Toons {
		names = append(names, toon.Toon)
	}
	local, _ := d.dbStore.HasLocalPlayers(ctx, names)
	for i := range profile.Toons {
		key := strings.ToLower(strings.TrimSpace(profile.Toons[i].Toon))
		if _, ok := local[key]; ok {
			profile.Toons[i].LocalPlayerKey = key
		}
	}
}
