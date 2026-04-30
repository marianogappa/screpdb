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

	config.GameTypesMode = raw.GameTypesMode
	config.ExcludeShortGames = raw.ExcludeShortGames
	config.ExcludeComputers = raw.ExcludeComputers
	config.MapKindFilterMode = raw.MapKindFilterMode
	config.PlayerFilterMode = raw.PlayerFilterMode

	config.GameTypes, err = unmarshalStringSlice(raw.GameTypesJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse game types: %w", err)
	}
	config.MapKinds, err = unmarshalStringSlice(raw.MapKindsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse map kinds: %w", err)
	}
	config.Players, err = unmarshalStringSlice(raw.PlayersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse players: %w", err)
	}
	legacyIncludedPlayers, err := unmarshalStringSlice(raw.LegacyIncludedPlayersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy included players: %w", err)
	}
	legacyExcludedPlayers, err := unmarshalStringSlice(raw.LegacyExcludedPlayersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy excluded players: %w", err)
	}
	if len(config.GameTypes) == 0 {
		legacyGameType := strings.TrimSpace(strings.ToLower(raw.LegacyGameType))
		// 'ums' is no longer a valid game-type filter — drop it during legacy migration.
		if legacyGameType != "" && legacyGameType != "all" && legacyGameType != "ums" {
			config.GameTypes = []string{legacyGameType}
		}
	}
	if len(config.Players) == 0 {
		switch {
		case len(legacyIncludedPlayers) > 0:
			config.PlayerFilterMode = globalReplayFilterModeOnlyThese
			config.Players = legacyIncludedPlayers
		case len(legacyExcludedPlayers) > 0:
			config.PlayerFilterMode = globalReplayFilterModeAllExceptThese
			config.Players = legacyExcludedPlayers
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
	playersJSON, err := marshalStringSlice(normalized.Players)
	if err != nil {
		return normalized, err
	}

	err = d.dbStore.UpdateGlobalReplayFilterConfigRaw(
		ctx,
		globalReplayFilterConfigKey,
		legacyGlobalReplayFilterGameTypeValue(normalized),
		normalized.GameTypesMode,
		gameTypesJSON,
		normalized.ExcludeShortGames,
		normalized.ExcludeComputers,
		normalized.MapKindFilterMode,
		mapKindsJSON,
		normalized.PlayerFilterMode,
		playersJSON,
		nullableStringValue(normalized.CompiledReplaysFilterSQL),
	)
	if err != nil {
		return normalized, err
	}
	return normalized, nil
}

func legacyGlobalReplayFilterGameTypeValue(config globalReplayFilterConfig) string {
	if len(config.GameTypes) == 1 && config.GameTypesMode == globalReplayFilterModeOnlyThese {
		return config.GameTypes[0]
	}
	return "all"
}

func (d *Dashboard) listGlobalReplayFilterOptions(ctx context.Context) (globalReplayFilterOptionsResponse, error) {
	result := globalReplayFilterOptionsResponse{
		TopPlayers:   []globalReplayFilterOption{},
		OtherPlayers: []globalReplayFilterOption{},
	}

	playerRows, err := d.dbStore.ListGlobalReplayFilterPlayerOptions(ctx)
	if err != nil {
		return result, err
	}
	allPlayers := []globalReplayFilterOption{}
	for _, row := range playerRows {
		var option globalReplayFilterOption
		option.Label = row.Label
		option.Count = row.Count
		option.Value = normalizePlayerKey(option.Label)
		allPlayers = append(allPlayers, option)
	}
	result.TopPlayers, result.OtherPlayers = splitTopOptions(allPlayers, globalReplayFilterTopOptionLimit)
	return result, nil
}
