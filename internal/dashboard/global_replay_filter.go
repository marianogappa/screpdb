package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

const (
	globalReplayFilterConfigKey                  = "global"
	globalReplayFilterModeOnlyThese             = "only_these"
	globalReplayFilterModeAllExceptThese        = "all_except_these"
	globalReplayFilterGameTypeTopVsBottom       = "top_vs_bottom"
	globalReplayFilterGameTypeMelee             = "melee"
	globalReplayFilterGameTypeOneOnOne          = "one_on_one"
	globalReplayFilterGameTypeUseMapSetting     = "ums"
	globalReplayFilterGameTypeFreeForAll        = "free_for_all"
	globalReplayFilterShortGameSeconds          = 120
	globalReplayFilterTopOptionLimit            = 10
)

type globalReplayFilterConfig struct {
	GameTypes               []string `json:"game_types"`
	GameTypesMode           string   `json:"game_types_mode"`
	ExcludeShortGames       bool     `json:"exclude_short_games"`
	ExcludeComputers        bool     `json:"exclude_computers"`
	Maps                    []string `json:"maps"`
	MapFilterMode           string   `json:"map_filter_mode"`
	Players                 []string `json:"players"`
	PlayerFilterMode        string   `json:"player_filter_mode"`
	CompiledReplaysFilterSQL *string `json:"compiled_replays_filter_sql,omitempty"`
}

type globalReplayFilterOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

type globalReplayFilterOptionsResponse struct {
	TopMaps      []globalReplayFilterOption `json:"top_maps"`
	OtherMaps    []globalReplayFilterOption `json:"other_maps"`
	TopPlayers   []globalReplayFilterOption `json:"top_players"`
	OtherPlayers []globalReplayFilterOption `json:"other_players"`
}

func defaultGlobalReplayFilterConfig() globalReplayFilterConfig {
	config := globalReplayFilterConfig{
		GameTypes:         []string{},
		GameTypesMode:     globalReplayFilterModeOnlyThese,
		ExcludeShortGames: true,
		ExcludeComputers:  true,
		Maps:              []string{},
		MapFilterMode:     globalReplayFilterModeOnlyThese,
		Players:           []string{},
		PlayerFilterMode:  globalReplayFilterModeOnlyThese,
	}
	config.CompiledReplaysFilterSQL = ptrToString(mustCompileGlobalReplayFilterSQL(config))
	return config
}

func mustCompileGlobalReplayFilterSQL(config globalReplayFilterConfig) string {
	compiled, err := compileGlobalReplayFilterSQL(config)
	if err != nil {
		panic(err)
	}
	return compiled
}

func normalizeGlobalReplayFilterConfig(config globalReplayFilterConfig) (globalReplayFilterConfig, error) {
	config.GameTypesMode = normalizeGlobalReplayFilterMode(config.GameTypesMode)
	config.MapFilterMode = normalizeGlobalReplayFilterMode(config.MapFilterMode)
	config.PlayerFilterMode = normalizeGlobalReplayFilterMode(config.PlayerFilterMode)

	config.GameTypes = normalizeGlobalReplayFilterValues(config.GameTypes, true)
	for _, value := range config.GameTypes {
		switch value {
		case globalReplayFilterGameTypeTopVsBottom,
		globalReplayFilterGameTypeMelee,
		globalReplayFilterGameTypeOneOnOne,
			globalReplayFilterGameTypeUseMapSetting,
			globalReplayFilterGameTypeFreeForAll:
		default:
			return config, fmt.Errorf("invalid global replay filter game type: %s", value)
		}
	}
	config.Maps = normalizeGlobalReplayFilterValues(config.Maps, true)
	config.Players = normalizeGlobalReplayFilterValues(config.Players, true)

	compiled, err := compileGlobalReplayFilterSQL(config)
	if err != nil {
		return config, err
	}
	config.CompiledReplaysFilterSQL = &compiled
	return config, nil
}

func normalizeGlobalReplayFilterValues(values []string, forceLower bool) []string {
	dedup := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if forceLower {
			value = strings.ToLower(value)
		}
		if _, ok := dedup[value]; ok {
			continue
		}
		dedup[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func compileGlobalReplayFilterSQL(config globalReplayFilterConfig) (string, error) {
	normalized, err := normalizeGlobalReplayFilterConfigWithoutSQL(config)
	if err != nil {
		return "", err
	}
	return dashboarddb.BuildGlobalReplayFilterSQL(
		normalized.ExcludeShortGames,
		globalReplayFilterShortGameSeconds,
		normalized.ExcludeComputers,
		normalized.GameTypesMode,
		normalized.GameTypes,
		normalized.MapFilterMode,
		normalized.Maps,
		normalized.PlayerFilterMode,
		normalized.Players,
	), nil
}

func normalizeGlobalReplayFilterConfigWithoutSQL(config globalReplayFilterConfig) (globalReplayFilterConfig, error) {
	config.GameTypesMode = normalizeGlobalReplayFilterMode(config.GameTypesMode)
	config.MapFilterMode = normalizeGlobalReplayFilterMode(config.MapFilterMode)
	config.PlayerFilterMode = normalizeGlobalReplayFilterMode(config.PlayerFilterMode)
	config.GameTypes = normalizeGlobalReplayFilterValues(config.GameTypes, true)
	for _, value := range config.GameTypes {
		switch value {
		case globalReplayFilterGameTypeTopVsBottom,
			globalReplayFilterGameTypeMelee,
			globalReplayFilterGameTypeOneOnOne,
			globalReplayFilterGameTypeUseMapSetting,
			globalReplayFilterGameTypeFreeForAll:
		default:
			return config, fmt.Errorf("invalid global replay filter game type: %s", value)
		}
	}
	config.Maps = normalizeGlobalReplayFilterValues(config.Maps, true)
	config.Players = normalizeGlobalReplayFilterValues(config.Players, true)
	return config, nil
}

func marshalStringSlice(values []string) (string, error) {
	bs, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

func unmarshalStringSlice(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

func splitTopOptions(options []globalReplayFilterOption, topLimit int) ([]globalReplayFilterOption, []globalReplayFilterOption) {
	if topLimit < 0 {
		topLimit = 0
	}
	if len(options) <= topLimit {
		return options, []globalReplayFilterOption{}
	}
	top := append([]globalReplayFilterOption{}, options[:topLimit]...)
	rest := append([]globalReplayFilterOption{}, options[topLimit:]...)
	return top, rest
}

func ptrToString(value string) *string {
	return &value
}

func normalizeGlobalReplayFilterMode(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", globalReplayFilterModeOnlyThese:
		return globalReplayFilterModeOnlyThese
	case globalReplayFilterModeAllExceptThese:
		return globalReplayFilterModeAllExceptThese
	default:
		return globalReplayFilterModeOnlyThese
	}
}

func composeReplayFilterSQL(globalFilterSQL *string, localFilterSQL *string) *string {
	composed := dashboarddb.ComposeReplayFilterSQL(nullableStringValue(globalFilterSQL), nullableStringValue(localFilterSQL))
	if composed == "" {
		return nil
	}
	return &composed
}

func nullableStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

