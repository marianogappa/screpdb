package db

import (
	"context"
	"time"
)

// GamesQuery is the games-list selection the dashboard asks for. Values are
// OR-ed within a field and AND-ed across fields.
type GamesQuery struct {
	PlayerKeys      []string
	MapNames        []string
	DurationBuckets []string
	Featuring       []string
	MatchupKeys     []string
	MapKindKeys     []string
}

// PlayersQuery is the players-list selection, including its sort, because the
// count and the page must agree on the same aggregate.
type PlayersQuery struct {
	NameFilter   string
	OnlyFivePlus bool
	LastPlayed   []string
	SortColumn   string
	SortDir      string
}

// ReplayLeaveReasonRow is one player's screp leave-reason enum for a replay.
type ReplayLeaveReasonRow struct {
	PlayerID int64
	Reason   string
}

// ReplayChatRow is one chat line of a replay, ordered by second then player.
type ReplayChatRow struct {
	Second   int64
	PlayerID int64
	Message  string
}

// PlayerFingerprintVectorRow is one game's hotkey-habit feature vector for a
// player.
type PlayerFingerprintVectorRow struct {
	Vector []byte
	Race   string
}

// Reader is every corpus read the dashboard performs. It exists so the
// library-backed implementation can be built and swapped in per use case
// without the SQL store and the endpoints having to change together.
type Reader interface {
	// Per game: the game detail page and its tabs.
	GetReplaySummary(ctx context.Context, replayID int64) (*ReplaySummaryRow, error)
	GetReplayFilePathByID(ctx context.Context, replayID int64) (string, error)
	ListReplayPlayersForDetail(ctx context.Context, replayID int64) ([]ReplayPlayerDetailRow, error)
	ListReplayPlayersForAlliance(ctx context.Context, replayID int64) ([]ReplayPlayerForAllianceRow, error)
	ListReplayAllianceCommands(ctx context.Context, replayID int64) ([]ReplayAllianceCommandRow, error)
	ListReplayPatterns(ctx context.Context, replayID int64) ([]PatternValueRow, error)
	ListPlayerPatterns(ctx context.Context, replayID int64) ([]PlayerPatternValueRow, error)
	ListReplayEvents(ctx context.Context, replayID int64) ([]ReplayEventRow, error)
	GetPhaseBoundariesForReplay(ctx context.Context, replayID int64) (PhaseBoundaries, error)
	ListGameUnitProductionAndCasts(ctx context.Context, replayID int64) ([]UnitProductionOrCastRow, error)
	ListUnitSliceCommandRows(ctx context.Context, replayID int64) ([]UnitSliceCommandRow, error)
	ListFirstUnitCommandRows(ctx context.Context, replayID int64) ([]FirstUnitCommandRow, error)
	ListGameUnitCadenceRows(ctx context.Context, replayID int64, durationSeconds int64, excludedUnits []string, startSeconds int64, endFraction float64, idleGapSeconds int64) ([]GameUnitCadenceRow, error)
	ListGasTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error)
	ListUpgradeTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error)
	ListTechTimingRows(ctx context.Context, replayID int64) ([]TimingRow, error)
	LoadEarlyZergTimings(ctx context.Context, replayID int64) ([]EarlyZergTimingsRow, error)
	ListViewportGameRows(ctx context.Context, replayID int64, eventType string) ([]ViewportGameRow, error)
	ListReplayLeaveReasons(ctx context.Context, replayID int64) ([]ReplayLeaveReasonRow, error)
	ListReplayChat(ctx context.Context, replayID int64) ([]ReplayChatRow, error)

	// Games list: the paged list plus the filter chips above it.
	CountGames(ctx context.Context, query GamesQuery) (int64, error)
	ListGames(ctx context.Context, query GamesQuery, limit, offset int) ([]WorkflowGameListRow, error)
	ListReplayPlayers(ctx context.Context, replayIDs []int64) ([]WorkflowGamePlayerRow, error)
	ListFeaturingPlayerPatternRows(ctx context.Context, replayIDs []int64) ([]WorkflowPlayerPatternRow, error)
	ListFeaturingReplayEventRows(ctx context.Context, replayIDs []int64) ([]WorkflowReplayEventRow, error)
	ListCurrentPlayersForReplayIDs(ctx context.Context, playerKey string, replayIDs []int64) ([]WorkflowCurrentPlayerRow, error)
	ListPatternValuesForPlayerIDs(ctx context.Context, playerIDs []int64) ([]WorkflowCurrentPlayerPatternRow, error)
	ListDroppedPlayerIDs(ctx context.Context, playerIDs []int64) (map[int64]bool, error)
	ListWorkflowFilterPlayers(ctx context.Context) ([]WorkflowFilterOptionRow, error)
	ListWorkflowFilterMaps(ctx context.Context) ([]WorkflowFilterOptionRow, error)
	ListDistinctMarkerLabels(ctx context.Context, featureKey string) ([]string, error)
	CountWorkflowDurationBuckets(ctx context.Context) (int64, int64, int64, int64, int64, error)
	CountWorkflowFeaturingGames(ctx context.Context, featureKeys []string) (map[string]int64, error)
	CountWorkflowMatchupGames(ctx context.Context) (map[string]int64, error)
	CountWorkflowMapKindGames(ctx context.Context) (map[string]int64, error)

	// Per player: the player page and its insights.
	GetPlayerNameByKey(ctx context.Context, playerKey string) (string, error)
	GetPlayerOverviewSummary(ctx context.Context, playerKey string) (*PlayerOverviewSummaryRow, error)
	ListPlayerRecentGames(ctx context.Context, playerKey string) ([]PlayerRecentGameRow, error)
	ListPlayerMatchups(ctx context.Context, playerKey string) ([]PlayerMatchupRow, error)
	ListRaceSections(ctx context.Context, playerKey string) ([]RaceSectionRow, error)
	ListRaceOrderRows(ctx context.Context, playerKey string) ([]RaceOrderRow, error)
	ListMatchupOrderRows(ctx context.Context, playerKey string) ([]MatchupOrderRow, error)
	ListPlayerChatRows(ctx context.Context, playerKey string) ([]PlayerChatRow, error)
	ListPlayerFirstExpansionTimings(ctx context.Context, playerKey string) ([]PlayerFirstExpansionTimingRow, error)
	GetPlayerFingerprintCoverage(ctx context.Context, playerKey string, featureVersion int64) (int64, error)
	ListPlayerFingerprintVectors(ctx context.Context, playerKey string, featureVersion int64) ([]PlayerFingerprintVectorRow, error)
	CountPlayerBnetGames(ctx context.Context, playerKey string) (int64, error)

	// Populations: corpus-wide aggregates the players list and the percentile
	// bands are built from.
	CountReplays(ctx context.Context) (int64, error)
	CountPlayers(ctx context.Context, query PlayersQuery) (int64, error)
	ListPlayers(ctx context.Context, query PlayersQuery, limit, offset int) ([]WorkflowPlayersListRow, error)
	CountPlayersLastPlayedBuckets(ctx context.Context, query PlayersQuery) (int64, int64, error)
	ListPlayerApmAggregates(ctx context.Context, minGames int64) ([]PlayerApmAggregateRow, error)
	ListUnitCadenceReplayMetrics(ctx context.Context, excludedUnits []string, onlyPlayerKey string, startSeconds int64, endFraction float64, idleGapSeconds int64, minUnitsPerReplay int64, minGapsPerReplay int64) ([]UnitCadenceReplayMetricRow, error)
	ListViewportAggregateRows(ctx context.Context, patternName string) ([]ViewportAggregateRow, error)
	ListHotkeyGamesRateByPlayer(ctx context.Context) (map[string]float64, error)
	ListTopPlayerColorRows(ctx context.Context) ([]PlayerColorRow, error)

	// Session: the gaming-session view.
	ListRecentAutosaveGamesForPlayers(ctx context.Context, playerKeys []string, limit int) ([]SessionCandidateRow, error)
	ListReplaysByIDs(ctx context.Context, replayIDs []int64) ([]SessionReplayRow, error)
	ListPlayerAPMByReplayIDs(ctx context.Context, replayIDs []int64) ([]SessionPlayerAPMRow, error)

	// Hotkeys: the hotkey signature, timeline and map surfaces.
	ListReplayPlayerHotkeyStreams(ctx context.Context, replayID int64) ([]ReplayPlayerHotkeyStreamRow, error)
	ListPlayerHotkeyStreamsByKey(ctx context.Context, playerKey string) ([]PlayerHotkeyStreamRow, error)
	GetReplayPlayerHotkeyStream(ctx context.Context, replayID, playerID int64) (*ReplayPlayerHotkeyStream, error)

	// Bnet: the Battle.net profile cache and its game-result history.
	GetBnetProfile(ctx context.Context, toon string, gateway int64) (*BnetProfileRow, error)
	UpsertBnetProfile(ctx context.Context, row BnetProfileRow) error
	GetBnetCountryCodesByPlayerKeys(ctx context.Context, playerKeys []string) (map[string]string, error)
	ListBnetProfilePayloadsByPlayerKeys(ctx context.Context, playerKeys []string) ([]BnetProfilePayloadRow, error)
	ListBnetAuroraIDsByPlayerKeys(ctx context.Context, playerKeys []string) ([]int64, error)
	UpsertBnetGameResults(ctx context.Context, rows []BnetGameResultRow) error
	ListBnetGameTimes(ctx context.Context, auroraID int64, since time.Time) ([]time.Time, error)

	// Settings: the persisted replay folder, feature flags and global filter.
	GetIngestInputDir(ctx context.Context, configKey string) (string, error)
	SetIngestInputDir(ctx context.Context, configKey, inputDir string) error
	GetFeatureFlagsJSON(ctx context.Context, configKey string) (string, error)
	SetFeatureFlagsJSON(ctx context.Context, configKey, raw string) error
	GetGlobalReplayFilterConfigRaw(ctx context.Context, configKey string) (GlobalReplayFilterConfigRaw, error)
	UpdateGlobalReplayFilterConfigRaw(ctx context.Context, configKey string, legacyGameType string, gameTypesMode string, gameTypesJSON string, excludeShortGames bool, excludeComputers bool, mapKindFilterMode string, mapKindsJSON string, playerFilterMode string, playersJSON string, compiledReplaysFilterSQL string) error
}
