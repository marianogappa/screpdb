package db

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
