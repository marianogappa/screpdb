package dashboard

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

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
	return detail
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
