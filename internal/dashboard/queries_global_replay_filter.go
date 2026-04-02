package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (d *Dashboard) getGlobalReplayFilterConfig(ctx context.Context) (globalReplayFilterConfig, error) {
	row := d.db.QueryRowContext(ctx, `
		SELECT
			game_type,
			included_maps,
			excluded_maps,
			included_players,
			excluded_players,
			game_types_mode,
			game_types,
			exclude_short_games,
			exclude_computers,
			map_filter_mode,
			maps,
			player_filter_mode,
			players,
			compiled_replays_filter_sql
		FROM settings
		WHERE config_key = ?
	`, globalReplayFilterConfigKey)

	var config globalReplayFilterConfig
	var legacyGameType string
	var legacyIncludedMapsJSON string
	var legacyExcludedMapsJSON string
	var legacyIncludedPlayersJSON string
	var legacyExcludedPlayersJSON string
	var gameTypesJSON string
	var mapsJSON string
	var playersJSON string
	var compiled sql.NullString
	if err := row.Scan(
		&legacyGameType,
		&legacyIncludedMapsJSON,
		&legacyExcludedMapsJSON,
		&legacyIncludedPlayersJSON,
		&legacyExcludedPlayersJSON,
		&config.GameTypesMode,
		&gameTypesJSON,
		&config.ExcludeShortGames,
		&config.ExcludeComputers,
		&config.MapFilterMode,
		&mapsJSON,
		&config.PlayerFilterMode,
		&playersJSON,
		&compiled,
	); err != nil {
		return config, err
	}

	var err error
	config.GameTypes, err = unmarshalStringSlice(gameTypesJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse game types: %w", err)
	}
	config.Maps, err = unmarshalStringSlice(mapsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse maps: %w", err)
	}
	config.Players, err = unmarshalStringSlice(playersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse players: %w", err)
	}
	legacyIncludedMaps, err := unmarshalStringSlice(legacyIncludedMapsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy included maps: %w", err)
	}
	legacyExcludedMaps, err := unmarshalStringSlice(legacyExcludedMapsJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy excluded maps: %w", err)
	}
	legacyIncludedPlayers, err := unmarshalStringSlice(legacyIncludedPlayersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy included players: %w", err)
	}
	legacyExcludedPlayers, err := unmarshalStringSlice(legacyExcludedPlayersJSON)
	if err != nil {
		return config, fmt.Errorf("failed to parse legacy excluded players: %w", err)
	}
	if len(config.GameTypes) == 0 {
		legacyGameType = strings.TrimSpace(strings.ToLower(legacyGameType))
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
	if compiled.Valid && strings.TrimSpace(compiled.String) != "" {
		config.CompiledReplaysFilterSQL = &compiled.String
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

	_, err = d.db.ExecContext(ctx, `
		UPDATE settings
		SET
			game_type = ?,
			included_maps = '[]',
			excluded_maps = '[]',
			included_players = '[]',
			excluded_players = '[]',
			game_types_mode = ?,
			game_types = ?,
			exclude_short_games = ?,
			exclude_computers = ?,
			map_filter_mode = ?,
			maps = ?,
			player_filter_mode = ?,
			players = ?,
			compiled_replays_filter_sql = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE config_key = ?
	`,
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
		globalReplayFilterConfigKey,
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

	mapRows, err := d.db.QueryContext(ctx, `
		SELECT MIN(map_name) AS label, COUNT(*) AS games
		FROM replays
		GROUP BY lower(trim(map_name))
		ORDER BY games DESC, label ASC
	`)
	if err != nil {
		return result, err
	}
	defer mapRows.Close()
	allMaps := []globalReplayFilterOption{}
	for mapRows.Next() {
		var option globalReplayFilterOption
		if err := mapRows.Scan(&option.Label, &option.Count); err != nil {
			return result, err
		}
		option.Value = strings.ToLower(strings.TrimSpace(option.Label))
		allMaps = append(allMaps, option)
	}
	if err := mapRows.Err(); err != nil {
		return result, err
	}
	result.TopMaps, result.OtherMaps = splitTopOptions(allMaps, globalReplayFilterTopOptionLimit)

	playerRows, err := d.db.QueryContext(ctx, `
		SELECT MIN(name) AS label, COUNT(*) AS games
		FROM players
		WHERE is_observer = 0
		GROUP BY lower(trim(name))
		ORDER BY games DESC, label ASC
	`)
	if err != nil {
		return result, err
	}
	defer playerRows.Close()
	allPlayers := []globalReplayFilterOption{}
	for playerRows.Next() {
		var option globalReplayFilterOption
		if err := playerRows.Scan(&option.Label, &option.Count); err != nil {
			return result, err
		}
		option.Value = normalizePlayerKey(option.Label)
		allPlayers = append(allPlayers, option)
	}
	if err := playerRows.Err(); err != nil {
		return result, err
	}
	result.TopPlayers, result.OtherPlayers = splitTopOptions(allPlayers, globalReplayFilterTopOptionLimit)
	return result, nil
}
