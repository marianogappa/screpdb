package bnetfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

var ErrUnknownGateway = errors.New("bnetfacade: unknown gateway")

// GatewayNames maps SC:R gateway ids to their display names, per
// web-api/v1/gateways. The gateway matters: querying a toon on the wrong
// gateway returns the same empty response as a nonexistent toon.
var GatewayNames = map[int]string{
	10: "U.S. West",
	11: "U.S. East",
	20: "Europe",
	30: "Korea",
	45: "Asia",
}

// AuroraProfile is the parsed scr_profile response for one (toon, gateway).
// Only the scalar identity fields are typed; the array sections keep their
// raw JSON so consumers can parse exactly what they need, and Raw carries the
// full UTF-8-normalized payload for caching.
type AuroraProfile struct {
	AuroraID        int64           `json:"aurora_id"`
	BattleTag       string          `json:"battle_tag"`
	CountryCode     string          `json:"country_code"`
	GameResults     json.RawMessage `json:"game_results"`
	Replays         json.RawMessage `json:"replays"`
	Toons           json.RawMessage `json:"toons"`
	MatchmakedStats json.RawMessage `json:"matchmaked_stats"`
	Stats           json.RawMessage `json:"stats"`

	Raw []byte `json:"-"`
}

// ProfileReplay is one replays[] entry of an aurora profile. MD5, URL and Link
// (the game id) are present only on entries belonging to the requesting
// account's own toons; other players' entries carry attributes and create_time
// alone. The list holds the account's last 25 games and rolls fast, so game
// ids must be harvested while a game is still listed.
type ProfileReplay struct {
	MD5        string            `json:"md5"`
	URL        string            `json:"url"`
	Link       string            `json:"link"`
	CreateTime int64             `json:"create_time"`
	Attributes map[string]string `json:"attributes"`
}

// ProfileReplaysFromPayload extracts the replays[] entries from a raw
// scr_profile payload (fresh or cached). A payload that cannot be decoded
// yields nil.
func ProfileReplaysFromPayload(payload []byte) []ProfileReplay {
	var parsed struct {
		Replays []ProfileReplay `json:"replays"`
	}
	if err := json.Unmarshal(normalizeBridgeJSON(payload), &parsed); err != nil {
		return nil
	}
	return parsed.Replays
}

// ProfileGameRef is one game_results[] entry's identity. Some replays[]
// entries arrive with an empty link; the game id can still be recovered by
// matching create times against these, which come in the same payload.
type ProfileGameRef struct {
	GameID     string
	CreateTime int64
}

// ProfileGameRefsFromPayload extracts (game id, create time) pairs from a raw
// scr_profile payload's game_results. Entries without both are skipped; an
// undecodable payload yields nil.
func ProfileGameRefsFromPayload(payload []byte) []ProfileGameRef {
	var parsed struct {
		GameResults []struct {
			GameID     string `json:"game_id"`
			CreateTime string `json:"create_time"`
		} `json:"game_results"`
	}
	if err := json.Unmarshal(normalizeBridgeJSON(payload), &parsed); err != nil {
		return nil
	}
	out := make([]ProfileGameRef, 0, len(parsed.GameResults))
	for _, g := range parsed.GameResults {
		createTime, err := strconv.ParseInt(strings.TrimSpace(g.CreateTime), 10, 64)
		if err != nil || g.GameID == "" {
			continue
		}
		out = append(out, ProfileGameRef{GameID: g.GameID, CreateTime: createTime})
	}
	return out
}

// Found reports whether the toon exists on the queried gateway. The bridge
// returns HTTP 200 with empty arrays and aurora_id 0 for an unknown toon (or
// a known toon on the wrong gateway) — there is no 404.
func (p *AuroraProfile) Found() bool {
	return p.AuroraID != 0
}

// FetchAuroraProfile fetches the scr_profile payload for one (toon, gateway)
// through the rate-limited bridge facade. The response is normalized to valid
// UTF-8 (map titles may carry raw cp949/latin-1 bytes) before parsing; the
// normalized payload is returned in Raw. Callers must check Found().
func FetchAuroraProfile(ctx context.Context, addr, toon string, gateway int, prio Priority) (*AuroraProfile, error) {
	if _, ok := GatewayNames[gateway]; !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownGateway, gateway)
	}
	path := fmt.Sprintf("/web-api/v2/aurora-profile-by-toon/%s/%d?request_flags=scr_profile", url.PathEscape(toon), gateway)
	body, err := BridgeGet(ctx, addr, path, prio)
	if err != nil {
		return nil, err
	}
	normalized := normalizeBridgeJSON(body)
	var p AuroraProfile
	if err := json.Unmarshal(normalized, &p); err != nil {
		return nil, fmt.Errorf("bnetfacade: parsing aurora profile: %w", err)
	}
	p.Raw = normalized
	return &p, nil
}
