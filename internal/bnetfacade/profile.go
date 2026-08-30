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
	normalized, err := normalizeBridgeJSON(body)
	if err != nil {
		return nil, err
	}
	var p AuroraProfile
	if err := json.Unmarshal(normalized, &p); err != nil {
		return nil, fmt.Errorf("bnetfacade: parsing aurora profile: %w", err)
	}
	p.Raw = normalized
	return &p, nil
}
