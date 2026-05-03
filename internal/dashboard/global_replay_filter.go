package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
)

const (
	globalReplayFilterConfigKey           = "global"
	globalReplayFilterModeOnlyThese       = "only_these"
	globalReplayFilterGameTypeTopVsBottom = "top_vs_bottom"
	globalReplayFilterGameTypeMelee       = "melee"
	globalReplayFilterGameTypeOneOnOne    = "one_on_one"
	globalReplayFilterGameTypeFreeForAll  = "free_for_all"
	globalReplayFilterMapKindRegular      = "regular"
	globalReplayFilterMapKindMoney        = "money"
	globalReplayFilterShortGameSeconds    = 120
)

type globalReplayFilterConfig struct {
	GameTypes                []string `json:"game_types"`
	ExcludeShortGames        bool     `json:"exclude_short_games"`
	ExcludeComputers         bool     `json:"exclude_computers"`
	MapKinds                 []string `json:"map_kinds"`
	CompiledReplaysFilterSQL *string  `json:"compiled_replays_filter_sql,omitempty"`
}

func defaultGlobalReplayFilterConfig() globalReplayFilterConfig {
	config := globalReplayFilterConfig{
		// Default to including everything. The user toggles off whatever
		// they don't want — there's no "exclude" mode anymore, just a
		// presence-based whitelist.
		GameTypes: []string{
			globalReplayFilterGameTypeTopVsBottom,
			globalReplayFilterGameTypeMelee,
			globalReplayFilterGameTypeOneOnOne,
			globalReplayFilterGameTypeFreeForAll,
		},
		ExcludeShortGames: true,
		ExcludeComputers:  true,
		MapKinds:          []string{globalReplayFilterMapKindRegular, globalReplayFilterMapKindMoney},
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
	config.GameTypes = normalizeGlobalReplayFilterValues(config.GameTypes, true)
	for _, value := range config.GameTypes {
		switch value {
		case globalReplayFilterGameTypeTopVsBottom,
			globalReplayFilterGameTypeMelee,
			globalReplayFilterGameTypeOneOnOne,
			globalReplayFilterGameTypeFreeForAll:
		default:
			return config, fmt.Errorf("invalid global replay filter game type: %s", value)
		}
	}
	config.MapKinds = normalizeGlobalReplayFilterValues(config.MapKinds, true)
	for _, value := range config.MapKinds {
		switch value {
		case globalReplayFilterMapKindRegular, globalReplayFilterMapKindMoney:
		default:
			return config, fmt.Errorf("invalid global replay filter map kind: %s", value)
		}
	}

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
		normalized.GameTypes,
		normalized.MapKinds,
	), nil
}

func normalizeGlobalReplayFilterConfigWithoutSQL(config globalReplayFilterConfig) (globalReplayFilterConfig, error) {
	config.GameTypes = normalizeGlobalReplayFilterValues(config.GameTypes, true)
	for _, value := range config.GameTypes {
		switch value {
		case globalReplayFilterGameTypeTopVsBottom,
			globalReplayFilterGameTypeMelee,
			globalReplayFilterGameTypeOneOnOne,
			globalReplayFilterGameTypeFreeForAll:
		default:
			return config, fmt.Errorf("invalid global replay filter game type: %s", value)
		}
	}
	config.MapKinds = normalizeGlobalReplayFilterValues(config.MapKinds, true)
	for _, value := range config.MapKinds {
		switch value {
		case globalReplayFilterMapKindRegular, globalReplayFilterMapKindMoney:
		default:
			return config, fmt.Errorf("invalid global replay filter map kind: %s", value)
		}
	}
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

func ptrToString(value string) *string {
	return &value
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

