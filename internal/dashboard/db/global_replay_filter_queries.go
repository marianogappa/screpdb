package db

import (
	"context"
	"strings"

	"github.com/samber/lo"

	"github.com/marianogappa/screpdb/internal/dashboard/db/sqlcgen"
)

type GlobalReplayFilterConfigRaw struct {
	LegacyGameType            string
	LegacyIncludedPlayersJSON string
	LegacyExcludedPlayersJSON string
	GameTypesMode             string
	GameTypesJSON             string
	ExcludeShortGames         bool
	ExcludeComputers          bool
	MapKindFilterMode         string
	MapKindsJSON              string
	PlayerFilterMode          string
	PlayersJSON               string
	CompiledReplaysFilterSQL  *string
}

type GlobalReplayFilterOptionRow struct {
	Label string
	Count int64
}

func (s *Store) GetGlobalReplayFilterConfigRaw(ctx context.Context, configKey string) (GlobalReplayFilterConfigRaw, error) {
	sqlcRow, err := sqlcgen.New(Trace(s.defaultDB)).GetGlobalReplayFilterConfigRaw(ctx, configKey)
	var result GlobalReplayFilterConfigRaw
	if err != nil {
		return result, err
	}
	result.LegacyGameType = sqlcRow.GameType
	result.LegacyIncludedPlayersJSON = sqlcRow.IncludedPlayers
	result.LegacyExcludedPlayersJSON = sqlcRow.ExcludedPlayers
	result.GameTypesMode = sqlcRow.GameTypesMode
	result.GameTypesJSON = sqlcRow.GameTypes
	result.ExcludeShortGames = sqlcRow.ExcludeShortGames
	result.ExcludeComputers = sqlcRow.ExcludeComputers
	result.MapKindFilterMode = sqlcRow.MapKindFilterMode
	result.MapKindsJSON = sqlcRow.MapKinds
	result.PlayerFilterMode = sqlcRow.PlayerFilterMode
	result.PlayersJSON = sqlcRow.Players
	if sqlcRow.CompiledReplaysFilterSql != nil && strings.TrimSpace(*sqlcRow.CompiledReplaysFilterSql) != "" {
		result.CompiledReplaysFilterSQL = sqlcRow.CompiledReplaysFilterSql
	}
	return result, nil
}

func (s *Store) UpdateGlobalReplayFilterConfigRaw(
	ctx context.Context,
	configKey string,
	legacyGameType string,
	gameTypesMode string,
	gameTypesJSON string,
	excludeShortGames bool,
	excludeComputers bool,
	mapKindFilterMode string,
	mapKindsJSON string,
	playerFilterMode string,
	playersJSON string,
	compiledReplaysFilterSQL string,
) error {
	return sqlcgen.New(Trace(s.defaultDB)).UpdateGlobalReplayFilterConfigRaw(ctx, sqlcgen.UpdateGlobalReplayFilterConfigRawParams{
		GameType:                 legacyGameType,
		GameTypesMode:            gameTypesMode,
		GameTypes:                gameTypesJSON,
		ExcludeShortGames:        excludeShortGames,
		ExcludeComputers:         excludeComputers,
		MapKindFilterMode:        mapKindFilterMode,
		MapKinds:                 mapKindsJSON,
		PlayerFilterMode:         playerFilterMode,
		Players:                  playersJSON,
		CompiledReplaysFilterSql: lo.ToPtr(compiledReplaysFilterSQL),
		ConfigKey:                configKey,
	})
}

func (s *Store) ListGlobalReplayFilterPlayerOptions(ctx context.Context) ([]GlobalReplayFilterOptionRow, error) {
	sqlcRows, err := sqlcgen.New(Trace(s.defaultDB)).ListGlobalReplayFilterPlayerOptions(ctx)
	if err != nil {
		return nil, err
	}
	options := make([]GlobalReplayFilterOptionRow, 0, len(sqlcRows))
	for _, row := range sqlcRows {
		options = append(options, GlobalReplayFilterOptionRow{
			Label: row.Label,
			Count: row.Games,
		})
	}
	return options, nil
}
