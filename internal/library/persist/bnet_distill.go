package persist

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// bnetCreateTimeSentinel is what replays[].create_time carries when Battle.net
// has no timestamp (8.3% of entries measured). Treating it as a real time
// would date those games in 2106.
const bnetCreateTimeSentinel = 0xFFFFFFFF

// bnetGameRefTolerance joins a replays[] entry with an empty link to its
// game_results[] row by create time. Measured skew between the two sections'
// stamps of one game is seconds; one account's games are minutes apart.
const bnetGameRefToleranceSeconds = 120

// rawAuroraProfile mirrors the parts of the bridge's scr_profile payload the
// distillation keeps. Everything else (the five avatar catalogues, end-screen
// score counters, per-lobby counters) is dropped here, once, at fetch time.
type rawAuroraProfile struct {
	AuroraID    int64  `json:"aurora_id"`
	BattleTag   string `json:"battle_tag"`
	CountryCode string `json:"country_code"`
	Profiles    []struct {
		Toon     string `json:"toon"`
		AvatarID string `json:"avatar_id"`
	} `json:"profiles"`
	Toons []struct {
		Toon          string `json:"toon"`
		GatewayID     int    `json:"gateway_id"`
		GamesLastWeek int    `json:"games_last_week"`
	} `json:"toons"`
	MatchmakedStats []struct {
		SeasonID      int    `json:"season_id"`
		Toon          string `json:"toon"`
		Rating        int    `json:"rating"`
		HighestRating int    `json:"highest_rating"`
		Wins          int    `json:"wins"`
		Losses        int    `json:"losses"`
		Disconnects   int    `json:"disconnects"`
		Bucket        int    `json:"bucket"`
	} `json:"matchmaked_stats"`
	Stats []struct {
		SeasonID  int                `json:"season_id"`
		GatewayID int                `json:"gateway_id"`
		Toon      string             `json:"toon"`
		Raw       map[string]float64 `json:"raw"`
	} `json:"stats"`
	GameResults []rawAuroraGameResult `json:"game_results"`
	Replays     []rawAuroraReplay     `json:"replays"`
}

// rawAuroraGameResult is one game_results[] entry. Numbers arrive as strings.
type rawAuroraGameResult struct {
	Attributes struct {
		MapName string `json:"mapName"`
		TileSet string `json:"tileset"`
	} `json:"attributes"`
	CreateTime string `json:"create_time"`
	GameID     string `json:"game_id"`
	GatewayID  int    `json:"gateway_id"`
	MatchGUID  string `json:"match_guid"`
	Players    []struct {
		Attributes struct {
			Race string `json:"race"`
			Team string `json:"team"`
			Type string `json:"type"`
			Left string `json:"left"`
		} `json:"attributes"`
		Result string            `json:"result"`
		Stats  map[string]string `json:"stats"`
		Toon   string            `json:"toon"`
	} `json:"players"`
}

// rawAuroraReplay is one replays[] entry: the only copy of the game's type,
// lobby name, host and map dimensions, plus md5/url on the fetching account's
// own games. Joined to game_results on link == game_id.
type rawAuroraReplay struct {
	MD5        string            `json:"md5"`
	URL        string            `json:"url"`
	Link       string            `json:"link"`
	CreateTime int64             `json:"create_time"`
	Attributes map[string]string `json:"attributes"`
}

var bnetRaces = [...]string{"zerg", "terran", "protoss", "random"}

// DistillBnetProfile parses an scr_profile payload once and keeps only what
// the app reads: the typed profile record and the game records its
// game_results[] and replays[] arrays describe. An undecodable payload yields
// a not-found record, exactly like an unknown toon.
func DistillBnetProfile(toon string, gateway int64, fetchedAt time.Time, payload []byte) (BnetProfile, []BnetGame) {
	profile := BnetProfile{Toon: toon, Gateway: gateway, FetchedAt: fetchedAt.UTC()}
	var raw rawAuroraProfile
	if err := json.Unmarshal(payload, &raw); err != nil {
		return profile, nil
	}
	profile.Found = raw.AuroraID != 0
	profile.AuroraID = raw.AuroraID
	profile.BattleTag = strings.TrimSpace(raw.BattleTag)
	profile.CountryCode = strings.TrimSpace(raw.CountryCode)
	for _, p := range raw.Profiles {
		if p.AvatarID != "" {
			profile.AvatarID = p.AvatarID
			break
		}
	}
	for _, t := range raw.Toons {
		name := strings.TrimSpace(t.Toon)
		if name == "" {
			continue
		}
		profile.Toons = append(profile.Toons, BnetToon{Toon: name, Gateway: t.GatewayID, GamesLastWeek: t.GamesLastWeek})
	}
	for _, m := range raw.MatchmakedStats {
		profile.Ladder = append(profile.Ladder, BnetLadder{
			Season: m.SeasonID, Toon: m.Toon, Rating: m.Rating, HighestRating: m.HighestRating,
			Wins: m.Wins, Losses: m.Losses, Disconnects: m.Disconnects, Bucket: m.Bucket,
		})
	}
	for _, s := range raw.Stats {
		row := BnetLifetime{Season: s.SeasonID, Gateway: s.GatewayID, Toon: s.Toon}
		for _, race := range bnetRaces {
			totals := BnetRaceTotals{
				Wins:        int(s.Raw[race+"_wins_sum"]),
				Losses:      int(s.Raw[race+"_losses_sum"]),
				Draws:       int(s.Raw[race+"_draws_sum"]),
				Disconnects: int(s.Raw[race+"_disconnects_sum"]),
				APMSum:      s.Raw[race+"_apm_sum"],
				PlayTimeSec: int64(s.Raw[race+"_play_time_sum"]),
			}
			if totals == (BnetRaceTotals{}) {
				continue
			}
			if row.Race == nil {
				row.Race = map[string]BnetRaceTotals{}
			}
			row.Race[race] = totals
		}
		if row.Race != nil {
			profile.Lifetime = append(profile.Lifetime, row)
		}
	}
	if !profile.Found {
		return profile, nil
	}
	return profile, distillBnetGames(raw)
}

func distillBnetGames(raw rawAuroraProfile) []BnetGame {
	ours := map[string]bool{}
	for _, t := range raw.Toons {
		if key := strings.ToLower(strings.TrimSpace(t.Toon)); key != "" {
			ours[key] = true
		}
	}
	games := make([]BnetGame, 0, len(raw.GameResults))
	index := map[string]int{}
	var refs []struct {
		id string
		at int64
	}
	for _, g := range raw.GameResults {
		createTime, err := strconv.ParseInt(strings.TrimSpace(g.CreateTime), 10, 64)
		if err != nil || createTime <= 0 || g.GameID == "" {
			continue
		}
		tileset, _ := strconv.Atoi(g.Attributes.TileSet)
		game := BnetGame{
			GameID:     g.GameID,
			CreateTime: time.Unix(createTime, 0).UTC(),
			Gateway:    g.GatewayID,
			Ladder:     g.MatchGUID != "",
			MapName:    stripBnetControlChars(g.Attributes.MapName),
			TileSet:    tileset,
			Accounts:   []int64{raw.AuroraID},
		}
		for _, p := range g.Players {
			slotType := strings.TrimSpace(p.Attributes.Type)
			toon := strings.TrimSpace(p.Toon)
			if slotType == "none" || slotType == "" || toon == "" {
				continue
			}
			race := strings.ToLower(strings.TrimSpace(p.Attributes.Race))
			team, _ := strconv.Atoi(p.Attributes.Team)
			apm, _ := strconv.Atoi(p.Stats[race+"_apm"])
			seconds, _ := strconv.Atoi(p.Stats[race+"_play_time"])
			player := BnetGamePlayer{
				Toon:     toon,
				Race:     race,
				Team:     team,
				Computer: slotType != "player",
				Left:     p.Attributes.Left == "1",
				APM:      apm,
				Seconds:  seconds,
			}
			// Provenance: game_results results are reliable only for the
			// fetched account's own toons (a third of other-player rows are
			// missing, and wins are under-reported 3:1). Everyone else stays
			// unknown until their own profile is seen.
			if ours[strings.ToLower(toon)] {
				player.Result = BnetResultFromString(p.Result)
			}
			game.Players = append(game.Players, player)
		}
		index[game.GameID] = len(games)
		games = append(games, game)
		refs = append(refs, struct {
			id string
			at int64
		}{game.GameID, createTime})
	}

	for _, rep := range raw.Replays {
		link := rep.Link
		if link == "" && rep.CreateTime > 0 && rep.CreateTime != bnetCreateTimeSentinel {
			// Some replays[] entries arrive with an empty link; the same
			// payload's game_results still carries the game id, recoverable
			// by create time.
			best, bestDiff := "", int64(bnetGameRefToleranceSeconds+1)
			for _, ref := range refs {
				diff := ref.at - rep.CreateTime
				if diff < 0 {
					diff = -diff
				}
				if diff < bestDiff {
					best, bestDiff = ref.id, diff
				}
			}
			link = best
		}
		i, ok := index[link]
		if !ok {
			continue
		}
		game := &games[i]
		attrs := rep.Attributes
		game.GameName = stripBnetControlChars(attrs["game_name"])
		game.Host = strings.TrimSpace(attrs["game_creator"])
		game.Type, _ = strconv.Atoi(attrs["game_type"])
		game.SubType, _ = strconv.Atoi(attrs["game_sub_type"])
		game.MapWidth, _ = strconv.Atoi(attrs["map_width"])
		game.MapHeight, _ = strconv.Atoi(attrs["map_height"])
		game.MD5 = rep.MD5
		game.URL = rep.URL
		if game.MapName == "" {
			game.MapName = stripBnetControlChars(attrs["map_title"])
		}
		// replay_result is the outcome from the fetching account's side,
		// already in the archive's encoding (1 win, 2 loss, 3 draw, 4
		// disconnect). Prefer it over the own player row's string.
		if result, err := strconv.Atoi(attrs["replay_result"]); err == nil && result >= BnetResultWin && result <= BnetResultDisconnect {
			for j := range game.Players {
				if ours[strings.ToLower(game.Players[j].Toon)] {
					game.Players[j].Result = result
				}
			}
		}
	}
	return games
}

func BnetResultFromString(result string) int {
	switch strings.ToLower(strings.TrimSpace(result)) {
	case "win":
		return BnetResultWin
	case "loss":
		return BnetResultLoss
	case "draw":
		return BnetResultDraw
	case "disconnect":
		return BnetResultDisconnect
	}
	return BnetResultUnknown
}

// BnetResultString is the display form of a BnetGamePlayer.Result.
func BnetResultString(result int) string {
	switch result {
	case BnetResultWin:
		return "win"
	case BnetResultLoss:
		return "loss"
	case BnetResultDraw:
		return "draw"
	case BnetResultDisconnect:
		return "disconnect"
	}
	return "unknown"
}

// stripBnetControlChars drops the colour-code bytes (0x01-0x1f) Battle.net
// leaves in map titles and lobby names ("\x07KnockOut \x051.4").
func stripBnetControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
