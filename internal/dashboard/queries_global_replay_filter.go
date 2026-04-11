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
	config.MapFilterMode = raw.MapFilterMode
	config.PlayerFilterMode = raw.PlayerFilterMode

	config.GameTypes, err = unmarshalStringSlice(raw.GameTypesJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse game types: %w", err)
	}
	config.Maps, err = unmarshalStringSlice(raw.MapsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse maps: %w", err)
	}
	config.Players, err = unmarshalStringSlice(raw.PlayersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse players: %w", err)
	}
	legacyIncludedMaps, err := unmarshalStringSlice(raw.LegacyIncludedMapsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy included maps: %w", err)
	}
	legacyExcludedMaps, err := unmarshalStringSlice(raw.LegacyExcludedMapsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy excluded maps: %w", err)
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
		if legacyGameType != "" && legacyGameType != "all" {
			config.GameTypes = []string{legacyGameType}
		}
	}
	if len(config.Maps) == 0 {
		switch {
		case len(legacyIncludedMaps) > 0:
			config.MapFilterMode = globalReplayFilterModeOnlyThese
			config.Maps = legacyIncludedMaps
		case len(legacyExcludedMaps) > 0:
			config.MapFilterMode = globalReplayFilterModeAllExceptThese
			config.Maps = legacyExcludedMaps
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
	mapsJSON, err := marshalStringSlice(normalized.Maps)
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
		normalized.MapFilterMode,
		mapsJSON,
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
		TopMaps:      []globalReplayFilterOption{},
		OtherMaps:    []globalReplayFilterOption{},
		TopPlayers:   []globalReplayFilterOption{},
		OtherPlayers: []globalReplayFilterOption{},
	}

	mapRows, err := d.dbStore.ListGlobalReplayFilterMapOptions(ctx)
	if err != nil {
		return result, err
	}
	allMaps := []globalReplayFilterOption{}
	for _, row := range mapRows {
		var option globalReplayFilterOption
		option.Label = row.Label
		option.Count = row.Count
		option.Value = strings.ToLower(strings.TrimSpace(option.Label))
		allMaps = append(allMaps, option)
	}
	result.TopMaps, result.OtherMaps = splitTopOptions(allMaps, globalReplayFilterTopOptionLimit)

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
