package dashboard

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

	clauses := []string{}
	if normalized.ExcludeShortGames {
		clauses = append(clauses, fmt.Sprintf("r.duration_seconds >= %d", globalReplayFilterShortGameSeconds))
	}
	if normalized.ExcludeComputers {
		clauses = append(clauses, `NOT EXISTS (
			SELECT 1
			FROM players p
			WHERE p.replay_id = r.id
				AND p.is_observer = 0
				AND lower(trim(coalesce(p.type, ''))) IN ('computer', 'computer controlled')
		)`)
	}

	appendModeClause(&clauses, normalized.GameTypesMode, gameTypePredicateSQL(normalized.GameTypes))
	appendModeClause(&clauses, normalized.MapFilterMode, mapPredicateSQL(normalized.Maps))
	appendModeClause(&clauses, normalized.PlayerFilterMode, playerPredicateSQL(normalized.Players))

	query := "SELECT r.* FROM replays r"
	if len(clauses) == 0 {
		return query, nil
	}
	return query + " WHERE " + strings.Join(clauses, " AND "), nil
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

func joinQuotedSQLStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		escaped := strings.ReplaceAll(value, "'", "''")
		quoted = append(quoted, "'"+escaped+"'")
	}
	return strings.Join(quoted, ", ")
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

func appendModeClause(clauses *[]string, mode string, predicate string) {
	predicate = normalizeSQLWhitespace(strings.TrimSpace(predicate))
	if predicate == "" {
		return
	}
	switch mode {
	case globalReplayFilterModeAllExceptThese:
		*clauses = append(*clauses, "NOT ("+predicate+")")
	default:
		*clauses = append(*clauses, "("+predicate+")")
	}
}

func gameTypePredicateSQL(values []string) string {
	predicates := make([]string, 0, len(values))
	for _, value := range values {
		switch value {
		case globalReplayFilterGameTypeTopVsBottom:
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'top vs bottom'")
		case globalReplayFilterGameTypeMelee:
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'melee'")
		case globalReplayFilterGameTypeOneOnOne:
			predicates = append(predicates, `(2 = (
				SELECT COUNT(*)
				FROM players p
				WHERE p.replay_id = r.id
					AND p.is_observer = 0
			) AND 2 = (
				SELECT COUNT(DISTINCT p.team)
				FROM players p
				WHERE p.replay_id = r.id
					AND p.is_observer = 0
			))`)
		case globalReplayFilterGameTypeUseMapSetting:
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) IN ('use map settings', 'ums')")
		case globalReplayFilterGameTypeFreeForAll:
			predicates = append(predicates, "lower(trim(coalesce(r.game_type, ''))) = 'free for all'")
		}
	}
	if len(predicates) == 0 {
		return ""
	}
	return strings.Join(predicates, " OR ")
}

func mapPredicateSQL(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return "lower(trim(coalesce(r.map_name, ''))) IN (" + joinQuotedSQLStrings(values) + ")"
}

func playerPredicateSQL(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return `EXISTS (
		SELECT 1
		FROM players p
		WHERE p.replay_id = r.id
			AND p.is_observer = 0
			AND lower(trim(coalesce(p.name, ''))) IN (` + joinQuotedSQLStrings(values) + `)
	)`
}

func composeReplayFilterSQL(globalFilterSQL *string, localFilterSQL *string) *string {
	globalNormalized := normalizeSQL(nullableStringValue(globalFilterSQL))
	localNormalized := normalizeSQL(nullableStringValue(localFilterSQL))
	switch {
	case globalNormalized == "" && localNormalized == "":
		return nil
	case globalNormalized == "":
		return &localNormalized
	case localNormalized == "":
		return &globalNormalized
	}
	composed := normalizeSQLWhitespace(fmt.Sprintf(
		"SELECT * FROM (%s) AS global_replays WHERE id IN (SELECT id FROM (%s) AS local_replays)",
		globalNormalized,
		localNormalized,
	))
	return &composed
}

func nullableStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

