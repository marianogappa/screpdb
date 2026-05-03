package dashboard

import (
	"context"
	"fmt"
	"strings"
)

func (d *Dashboard) getGlobalReplayFilterConfig(ctx context.Context) (globalReplayFilterConfig, error) {
	var config globalReplayFilterConfig
	raw, err := d.dbStore.GetGlobalReplayFilterConfigRaw(ctx, globalReplayFilterConfigKey)
	if err != nil {
		return config, err
	}

	config.ExcludeShortGames = raw.ExcludeShortGames
	config.ExcludeComputers = raw.ExcludeComputers

	config.GameTypes, err = unmarshalStringSlice(raw.GameTypesJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse game types: %w", err)
	}
	config.MapKinds, err = unmarshalStringSlice(raw.MapKindsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse map kinds: %w", err)
	}
	if len(config.GameTypes) == 0 {
		legacyGameType := strings.TrimSpace(strings.ToLower(raw.LegacyGameType))
		// 'ums' is no longer a valid game-type filter — drop it during legacy migration.
		if legacyGameType != "" && legacyGameType != "all" && legacyGameType != "ums" {
			config.GameTypes = []string{legacyGameType}
		}
	}
	if raw.CompiledReplaysFilterSQL != nil {
		config.CompiledReplaysFilterSQL = raw.CompiledReplaysFilterSQL
	}
	return normalizeGlobalReplayFilterConfig(config)
}

func (d *Dashboard) updateGlobalReplayFilterConfig(ctx context.Context, config globalReplayFilterConfig) (globalReplayFilterConfig, error) {
	normalized, err := normalizeGlobalReplayFilterConfig(config)
	if err != nil {
		return normalized, err
	}

	gameTypesJSON, err := marshalStringSlice(normalized.GameTypes)
	if err != nil {
		return normalized, err
	}
	mapKindsJSON, err := marshalStringSlice(normalized.MapKinds)
	if err != nil {
		return normalized, err
	}

	// player_filter_mode / players / *_mode columns still exist in the
	// settings table — write fixed defaults so legacy DBs don't choke on
	// missing values. The simplified config no longer surfaces these to
	// callers.
	err = d.dbStore.UpdateGlobalReplayFilterConfigRaw(
		ctx,
		globalReplayFilterConfigKey,
		legacyGlobalReplayFilterGameTypeValue(normalized),
		globalReplayFilterModeOnlyThese,
		gameTypesJSON,
		normalized.ExcludeShortGames,
		normalized.ExcludeComputers,
		globalReplayFilterModeOnlyThese,
		mapKindsJSON,
		globalReplayFilterModeOnlyThese,
		"[]",
		nullableStringValue(normalized.CompiledReplaysFilterSQL),
	)
	if err != nil {
		return normalized, err
	}
	return normalized, nil
}

func legacyGlobalReplayFilterGameTypeValue(config globalReplayFilterConfig) string {
	if len(config.GameTypes) == 1 {
		return config.GameTypes[0]
	}
	return "all"
}
