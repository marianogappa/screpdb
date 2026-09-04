package db

import "context"

func (s *LibStore) GetIngestInputDir(ctx context.Context, configKey string) (string, error) {
	return s.settings.GetIngestInputDir(ctx, configKey)
}

func (s *LibStore) SetIngestInputDir(ctx context.Context, configKey, inputDir string) error {
	return s.settings.SetIngestInputDir(ctx, configKey, inputDir)
}

func (s *LibStore) GetFeatureFlagsJSON(ctx context.Context, configKey string) (string, error) {
	return s.settings.GetFeatureFlagsJSON(ctx, configKey)
}

func (s *LibStore) SetFeatureFlagsJSON(ctx context.Context, configKey, raw string) error {
	return s.settings.SetFeatureFlagsJSON(ctx, configKey, raw)
}

func (s *LibStore) GetGlobalReplayFilterConfigRaw(ctx context.Context, configKey string) (GlobalReplayFilterConfigRaw, error) {
	return s.settings.GetGlobalReplayFilterConfigRaw(ctx, configKey)
}

func (s *LibStore) UpdateGlobalReplayFilterConfigRaw(
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
	return s.settings.UpdateGlobalReplayFilterConfigRaw(ctx, configKey, legacyGameType, gameTypesMode,
		gameTypesJSON, excludeShortGames, excludeComputers, mapKindFilterMode, mapKindsJSON,
		playerFilterMode, playersJSON, compiledReplaysFilterSQL)
}
