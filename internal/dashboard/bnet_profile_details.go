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

// Every numeric here is a pointer on purpose. Battle.net reports real zeroes
// (an account that laddered and lost every game has 0 wins), and a value we
// never received is a different fact from one we received as zero. A plain int
// with omitempty collapses the two, so nil means "we do not know" and a pointer
// to 0 means "Battle.net told us zero".
type bnetProfileDetail struct {
	Toon        string            `json:"toon"`
	AuroraID    int64             `json:"aurora_id,omitempty"`
	BattleTag   string            `json:"battle_tag,omitempty"`
	CountryCode string            `json:"country_code,omitempty"`
	Toons       []bnetProfileToon `json:"toons,omitempty"`
	// Found is false for a toon we looked up and Battle.net did not have. That
	// is an answer; a profile we never looked up is absent from the payload
	// entirely.
	Found bool `json:"found"`
	// FetchedAt is when this answer was obtained (RFC3339, UTC), so a stale
	// value can say how old it is instead of passing as current.
	FetchedAt string `json:"fetched_at,omitempty"`
	// GatewaysChecked counts the gateways swept for this toon and
	// GatewaysTotal the gateways worth sweeping. A miss only means "not on
	// Battle.net" once the two agree; before that the sweep is still running.
	GatewaysChecked int `json:"gateways_checked,omitempty"`
	GatewaysTotal   int `json:"gateways_total,omitempty"`

	PlaysLadder  *bool `json:"plays_ladder"`
	MMR          *int  `json:"mmr"`
	HighestMMR   *int  `json:"highest_mmr"`
	LadderWins   *int  `json:"ladder_wins"`
	LadderLosses *int  `json:"ladder_losses"`
	// Lifetime account totals summed from the per-race counters Battle.net
	// reports (wins/losses/draws/disconnects per race, plus per-game APM
	// sums, from which the average APM is derived).
	LifetimeGames       *int     `json:"lifetime_games"`
	LifetimeWins        *int     `json:"lifetime_wins"`
	LifetimeLosses      *int     `json:"lifetime_losses"`
	LifetimeDisconnects *int     `json:"lifetime_disconnects"`
	AverageAPM          *float64 `json:"average_apm"`
	// PlayTimeSeconds sums the per-race lifetime play_time counters.
	PlayTimeSeconds *int64 `json:"play_time_seconds"`
	// GamesLastWeek sums Battle.net's own games_last_week over the account's
	// toons; LastPlayedAt is the newest game in the archive (RFC3339, UTC).
	GamesLastWeek *int   `json:"games_last_week"`
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
		Found:       p.Found,
	}
	if !p.FetchedAt.IsZero() {
		detail.FetchedAt = p.FetchedAt.UTC().Format(time.RFC3339)
	}
	// A profile Battle.net does not have carries no stats to report. Leaving
	// every pointer nil is the whole point: the page can say "no account"
	// rather than inventing an unranked player with zero games.
	if !p.Found {
		return detail
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
	// Found means Battle.net answered about this account, so its totals are
	// known even when they add up to nothing. Distillation drops all-zero rows
	// as noise, so an empty slice here is "nothing on record", not "not asked";
	// the not-asked case returned above with every pointer nil.
	weekly := 0
	for _, t := range p.Toons {
		weekly += t.GamesLastWeek
	}
	detail.GamesLastWeek = &weekly
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
	// An empty Ladder slice is an answer, not a gap: Battle.net reported this
	// account holds no matchmaking record, so "does not ladder" is known and
	// the ratings genuinely do not apply.
	playsLadder := len(p.Ladder) > 0
	detail.PlaysLadder = &playsLadder
	if playsLadder {
		var mmr, highest, wins, losses int
		for _, stat := range p.Ladder {
			if stat.Rating > mmr {
				mmr = stat.Rating
			}
			if stat.HighestRating > highest {
				highest = stat.HighestRating
			}
			wins += stat.Wins
			losses += stat.Losses
		}
		detail.MMR, detail.HighestMMR = &mmr, &highest
		detail.LadderWins, detail.LadderLosses = &wins, &losses
	}
	// Lifetime totals: Battle.net reports per-race counters; sum them and
	// derive the average APM from the per-game APM sums.
	var lifetimeGames, lifetimeWins, lifetimeLosses, lifetimeDisconnects int
	var playTime int64
	apmSum := 0.0
	for _, row := range p.Lifetime {
		for _, race := range []string{"zerg", "terran", "protoss"} {
			totals := row.Race[race]
			lifetimeWins += totals.Wins
			lifetimeLosses += totals.Losses
			lifetimeDisconnects += totals.Disconnects
			lifetimeGames += totals.Wins + totals.Losses + totals.Draws + totals.Disconnects
			apmSum += totals.APMSum
			playTime += totals.PlayTimeSec
		}
	}
	detail.LifetimeGames, detail.LifetimeWins = &lifetimeGames, &lifetimeWins
	detail.LifetimeLosses, detail.LifetimeDisconnects = &lifetimeLosses, &lifetimeDisconnects
	detail.PlayTimeSeconds = &playTime
	// An average over no games is undefined, which is not the same as zero APM.
	if lifetimeGames > 0 {
		avg := apmSum / float64(lifetimeGames)
		detail.AverageAPM = &avg
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
	// A toon absent from every gateway produces one miss row per gateway. The
	// page needs the tally to tell a finished sweep that found nothing from one
	// still in progress, which look identical from a single row.
	missesByKey := map[string]int{}
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
		if !p.Found {
			missesByKey[key]++
		}
		// A toon can be cached under several gateways; keep the richest row.
		if existing, ok := out[key]; ok && bnetProfileDetailScore(existing) >= bnetProfileDetailScore(detail) {
			continue
		}
		out[key] = detail
	}
	for key, detail := range out {
		if detail.Found {
			continue
		}
		detail.GatewaysChecked = missesByKey[key]
		detail.GatewaysTotal = len(defaultGatewayOrder)
	}
	return out
}

// bnetProfileDetailScore ranks two cached rows for the same toon so the one
// carrying more information wins. Ladder data is the scarcest, then alternate
// toons, then a battle tag.
func bnetProfileDetailScore(detail *bnetProfileDetail) int {
	score := 0
	if detail.PlaysLadder != nil && *detail.PlaysLadder {
		score += 100
	}
	// A found profile always beats a miss, however little it carries.
	if detail.Found {
		score += 1000
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
