package sbfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

const maxUsersPerBatch = 50

var ErrBadUserID = errors.New("sbfacade: malformed user id")

// WinLoss maps are keyed by race: "p", "t", "z" and "r" (random).
type WinLoss struct {
	Wins   int
	Losses int
}

type LadderStats struct {
	Rank          int
	Rating        float64
	Wins          int
	Losses        int
	LifetimeGames int
	LastPlayed    time.Time
	ByRace        map[string]WinLoss
}

type RecentPlayer struct {
	UserID uint32
	Name   string
	// Race is the race played when the game is settled, otherwise the race
	// picked, which may be "r".
	Race string
	Team int
	// Result is empty until the game is settled.
	Result Result
}

type RecentGame struct {
	ID              string
	StartTime       time.Time
	GameLength      time.Duration
	MapName         string
	GameSource      string
	MatchmakingType string
	Canceled        bool
	Players         []RecentPlayer
}

// Profile is a player's public ShieldBattery profile.
type Profile struct {
	UserID  uint32
	Name    string
	Created time.Time
	// Ladder is keyed by matchmaking type ("1v1", "2v2", "2v2bgh"...) and
	// holds the current season only.
	Ladder   map[string]LadderStats
	SeasonID int
	// Lifetime is keyed by race and counts every rated game ever.
	Lifetime    map[string]WinLoss
	RecentGames []RecentGame
}

type raceStats struct {
	PWins   int `json:"pWins"`
	PLosses int `json:"pLosses"`
	TWins   int `json:"tWins"`
	TLosses int `json:"tLosses"`
	ZWins   int `json:"zWins"`
	ZLosses int `json:"zLosses"`
	RWins   int `json:"rWins"`
	RLosses int `json:"rLosses"`
}

func (r raceStats) byRace() map[string]WinLoss {
	return map[string]WinLoss{
		"p": {r.PWins, r.PLosses},
		"t": {r.TWins, r.TLosses},
		"z": {r.ZWins, r.ZLosses},
		"r": {r.RWins, r.RLosses},
	}
}

type userInfo struct {
	ID      uint32 `json:"id"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
}

type profileResponse struct {
	User    userInfo `json:"user"`
	Profile struct {
		Ladder map[string]struct {
			raceStats
			Rank           int     `json:"rank"`
			Rating         float64 `json:"rating"`
			Wins           int     `json:"wins"`
			Losses         int     `json:"losses"`
			LifetimeGames  int     `json:"lifetimeGames"`
			LastPlayedDate int64   `json:"lastPlayedDate"`
		} `json:"ladder"`
		SeasonID  int       `json:"seasonId"`
		UserStats raceStats `json:"userStats"`
	} `json:"profile"`
	MatchHistory struct {
		Games []gameRecord `json:"games"`
		Maps  []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"maps"`
		Users []userInfo `json:"users"`
	} `json:"matchHistory"`
}

// FetchProfile fetches a player's public profile, or ErrNotFound.
func FetchProfile(ctx context.Context, userID uint32) (*Profile, error) {
	if userID == 0 {
		return nil, ErrBadUserID
	}
	if err := thePacer.wait(ctx); err != nil {
		return nil, err
	}
	return fetchProfileFrom(ctx, apiURL+"users/"+strconv.FormatUint(uint64(userID), 10)+"/profile")
}

func fetchProfileFrom(ctx context.Context, url string) (*Profile, error) {
	body, err := getJSON(ctx, url)
	if err != nil {
		return nil, err
	}
	return parseProfile(body)
}

func parseProfile(body []byte) (*Profile, error) {
	var resp profileResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("sbfacade: parsing profile: %w", err)
	}
	profile := &Profile{
		UserID:   resp.User.ID,
		Name:     resp.User.Name,
		Created:  unixMilli(resp.User.Created),
		Ladder:   make(map[string]LadderStats, len(resp.Profile.Ladder)),
		SeasonID: resp.Profile.SeasonID,
		Lifetime: resp.Profile.UserStats.byRace(),
	}
	for kind, l := range resp.Profile.Ladder {
		profile.Ladder[kind] = LadderStats{
			Rank:          l.Rank,
			Rating:        l.Rating,
			Wins:          l.Wins,
			Losses:        l.Losses,
			LifetimeGames: l.LifetimeGames,
			LastPlayed:    unixMilli(l.LastPlayedDate),
			ByRace:        l.byRace(),
		}
	}
	mapNames := make(map[string]string, len(resp.MatchHistory.Maps))
	for _, m := range resp.MatchHistory.Maps {
		mapNames[m.ID] = m.Name
	}
	names := make(map[uint32]string, len(resp.MatchHistory.Users)+1)
	for _, u := range resp.MatchHistory.Users {
		names[u.ID] = u.Name
	}
	names[resp.User.ID] = resp.User.Name
	for _, g := range resp.MatchHistory.Games {
		game, err := recentGame(g, mapNames, names)
		if err != nil {
			return nil, err
		}
		profile.RecentGames = append(profile.RecentGames, game)
	}
	return profile, nil
}

func recentGame(g gameRecord, mapNames map[string]string, names map[uint32]string) (RecentGame, error) {
	results, err := parseResults(g.Results)
	if err != nil {
		return RecentGame{}, err
	}
	game := RecentGame{
		ID:              g.ID,
		StartTime:       unixMilli(g.StartTime),
		GameLength:      millis(g.GameLength),
		MapName:         mapNames[g.MapID],
		GameSource:      g.Config.GameSource,
		MatchmakingType: g.Config.GameSourceExtra.Type,
		Canceled:        g.CanceledAt != nil,
	}
	for team, slots := range g.Config.Teams {
		for _, slot := range slots {
			if slot.IsComputer {
				continue
			}
			player := RecentPlayer{UserID: slot.ID, Name: names[slot.ID], Race: slot.Race, Team: team}
			if r, ok := results[slot.ID]; ok {
				player.Result = r.Result
				if r.Race != "" {
					player.Race = r.Race
				}
			}
			game.Players = append(game.Players, player)
		}
	}
	return game, nil
}

// FetchUsers resolves ShieldBattery user ids to names. Ids ShieldBattery does
// not know are absent from the result.
func FetchUsers(ctx context.Context, ids []uint32) (map[uint32]string, error) {
	for _, id := range ids {
		if id == 0 {
			return nil, ErrBadUserID
		}
	}
	return fetchUsersFrom(ctx, apiURL+"users/batch-info", ids, thePacer.wait)
}

func fetchUsersFrom(ctx context.Context, endpoint string, ids []uint32, wait func(context.Context) error) (map[uint32]string, error) {
	names := make(map[uint32]string, len(ids))
	for start := 0; start < len(ids); start += maxUsersPerBatch {
		chunk := ids[start:min(start+maxUsersPerBatch, len(ids))]
		query := url.Values{}
		for _, id := range chunk {
			query.Add("u", strconv.FormatUint(uint64(id), 10))
		}
		if err := wait(ctx); err != nil {
			return nil, err
		}
		body, err := getJSON(ctx, endpoint+"?"+query.Encode())
		if err != nil {
			return nil, err
		}
		var resp struct {
			UserInfos []userInfo `json:"userInfos"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("sbfacade: parsing user infos: %w", err)
		}
		for _, u := range resp.UserInfos {
			names[u.ID] = u.Name
		}
	}
	return names, nil
}
