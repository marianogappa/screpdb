package library

import (
	"fmt"
	"slices"
	"strings"
)

// ShortGameSeconds is the floor below which "exclude short games" drops a replay.
const ShortGameSeconds = 120

const (
	GameTypeTopVsBottom = "top_vs_bottom"
	GameTypeMelee       = "melee"
	GameTypeOneOnOne    = "one_on_one"
	GameTypeFreeForAll  = "free_for_all"

	MapKindFilterRegular = "regular"
	MapKindFilterMoney   = "money"
)

// FilterConfig is the global replay filter. Semantics mirror the dashboard's
// former SQL: UseMapSettings replays are always excluded; each list is an OR
// over its recognised values and imposes nothing when it has none.
type FilterConfig struct {
	GameTypes         []string `json:"game_types"`
	ExcludeShortGames bool     `json:"exclude_short_games"`
	ExcludeComputers  bool     `json:"exclude_computers"`
	MapKinds          []string `json:"map_kinds"`
}

// DefaultFilterConfig includes every game type and map kind and excludes
// short games and games with computer players.
func DefaultFilterConfig() FilterConfig {
	return FilterConfig{
		GameTypes:         []string{GameTypeTopVsBottom, GameTypeMelee, GameTypeOneOnOne, GameTypeFreeForAll},
		ExcludeShortGames: true,
		ExcludeComputers:  true,
		MapKinds:          []string{MapKindFilterRegular, MapKindFilterMoney},
	}
}

// Normalize trims, lowercases and dedups the lists, rejecting unknown values.
func (c FilterConfig) Normalize() (FilterConfig, error) {
	c.GameTypes = normalizeValues(c.GameTypes)
	for _, v := range c.GameTypes {
		switch v {
		case GameTypeTopVsBottom, GameTypeMelee, GameTypeOneOnOne, GameTypeFreeForAll:
		default:
			return c, fmt.Errorf("invalid global replay filter game type: %s", v)
		}
	}
	c.MapKinds = normalizeValues(c.MapKinds)
	for _, v := range c.MapKinds {
		switch v {
		case MapKindFilterRegular, MapKindFilterMoney:
		default:
			return c, fmt.Errorf("invalid global replay filter map kind: %s", v)
		}
	}
	return c, nil
}

func normalizeValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, raw := range values {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v == "" || slices.Contains(out, v) {
			continue
		}
		out = append(out, v)
	}
	return out
}

func (c FilterConfig) Equal(o FilterConfig) bool {
	return c.ExcludeShortGames == o.ExcludeShortGames &&
		c.ExcludeComputers == o.ExcludeComputers &&
		slices.Equal(c.GameTypes, o.GameTypes) &&
		slices.Equal(c.MapKinds, o.MapKinds)
}

// Matches is the filter predicate. Unrecognised list values are ignored, as
// the SQL builder dropped them.
func (c FilterConfig) Matches(r *Replay) bool {
	if r == nil || r.MapKind == MapKindUseMapSettings {
		return false
	}
	if c.ExcludeShortGames && int(r.Duration) < ShortGameSeconds {
		return false
	}
	if c.ExcludeComputers && r.HasNonObserverComputer() {
		return false
	}
	if !matchesGameTypes(c.GameTypes, r) {
		return false
	}
	return matchesMapKinds(c.MapKinds, r)
}

func matchesGameTypes(values []string, r *Replay) bool {
	constrained, matched := false, false
	for _, v := range values {
		switch v {
		case GameTypeTopVsBottom:
			constrained = true
			matched = matched || gameTypeIs(r, "top vs bottom")
		case GameTypeMelee:
			constrained = true
			matched = matched || gameTypeIs(r, "melee")
		case GameTypeOneOnOne:
			constrained = true
			matched = matched || r.OneOnOne()
		case GameTypeFreeForAll:
			constrained = true
			matched = matched || gameTypeIs(r, "free for all")
		}
	}
	return !constrained || matched
}

func matchesMapKinds(values []string, r *Replay) bool {
	constrained, matched := false, false
	for _, v := range values {
		switch v {
		case MapKindFilterRegular:
			constrained = true
			matched = matched || r.MapKind == MapKindRegular
		case MapKindFilterMoney:
			constrained = true
			matched = matched || r.MapKind == MapKindMoney
		}
	}
	return !constrained || matched
}

func gameTypeIs(r *Replay, want string) bool {
	return strings.ToLower(strings.TrimSpace(Strings.Name(r.GameType))) == want
}
