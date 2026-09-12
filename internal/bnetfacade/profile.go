package bnetfacade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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
// Only the scalar identity fields are typed; Raw carries the full
// UTF-8-normalized payload for the caller to distil.
type AuroraProfile struct {
	AuroraID    int64  `json:"aurora_id"`
	BattleTag   string `json:"battle_tag"`
	CountryCode string `json:"country_code"`

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

// ProfileGameRef is one game_results[] entry's identity. Some replays[]
// entries arrive with an empty link; the game id can still be recovered by
// matching create times against these, which come in the same payload.
type ProfileGameRef struct {
	GameID     string
	CreateTime int64
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
