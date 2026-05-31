package service

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
)

// DashboardService is generated from apigen.StrictServerInterface.
type DashboardService interface {
	ListAliases(ctx context.Context, request apigen.ListAliasesRequestObject) (HandlerResult, error)
	ImportAliases(ctx context.Context, request apigen.ImportAliasesRequestObject) (HandlerResult, error)
	UpsertAliasEntry(ctx context.Context, request apigen.UpsertAliasEntryRequestObject) (HandlerResult, error)
	DeleteAliasEntry(ctx context.Context, request apigen.DeleteAliasEntryRequestObject) (HandlerResult, error)
	GetGlobalReplayFilterConfig(ctx context.Context, request apigen.GetGlobalReplayFilterConfigRequestObject) (HandlerResult, error)
	UpdateGlobalReplayFilterConfig(ctx context.Context, request apigen.UpdateGlobalReplayFilterConfigRequestObject) (HandlerResult, error)
	GetGlobalReplayFilterOptions(ctx context.Context, request apigen.GetGlobalReplayFilterOptionsRequestObject) (HandlerResult, error)
	Ingest(ctx context.Context, request apigen.IngestRequestObject) (HandlerResult, error)
	IngestLogs(ctx context.Context, request apigen.IngestLogsRequestObject) (HandlerResult, error)
	GetIngestSettings(ctx context.Context, request apigen.GetIngestSettingsRequestObject) (HandlerResult, error)
	UpdateIngestSettings(ctx context.Context, request apigen.UpdateIngestSettingsRequestObject) (HandlerResult, error)
	GetStaleReplaysCount(ctx context.Context, request apigen.GetStaleReplaysCountRequestObject) (HandlerResult, error)
	GamesList(ctx context.Context, request apigen.GamesListRequestObject) (HandlerResult, error)
	GameDetail(ctx context.Context, request apigen.GameDetailRequestObject) (HandlerResult, error)
	GameSee(ctx context.Context, request apigen.GameSeeRequestObject) (HandlerResult, error)
	Healthcheck(ctx context.Context, request apigen.HealthcheckRequestObject) (HandlerResult, error)
	PlayerColors(ctx context.Context, request apigen.PlayerColorsRequestObject) (HandlerResult, error)
	PlayersList(ctx context.Context, request apigen.PlayersListRequestObject) (HandlerResult, error)
	PlayersApmHistogram(ctx context.Context, request apigen.PlayersApmHistogramRequestObject) (HandlerResult, error)
	PlayersDelayHistogram(ctx context.Context, request apigen.PlayersDelayHistogramRequestObject) (HandlerResult, error)
	PlayersUnitCadence(ctx context.Context, request apigen.PlayersUnitCadenceRequestObject) (HandlerResult, error)
	PlayersViewportMultitasking(ctx context.Context, request apigen.PlayersViewportMultitaskingRequestObject) (HandlerResult, error)
	PlayerDetail(ctx context.Context, request apigen.PlayerDetailRequestObject) (HandlerResult, error)
	PlayerChatSummary(ctx context.Context, request apigen.PlayerChatSummaryRequestObject) (HandlerResult, error)
	PlayerInsight(ctx context.Context, request apigen.PlayerInsightRequestObject) (HandlerResult, error)
	PlayerApmHistogram(ctx context.Context, request apigen.PlayerApmHistogramRequestObject) (HandlerResult, error)
	PlayerDelayInsight(ctx context.Context, request apigen.PlayerDelayInsightRequestObject) (HandlerResult, error)
	PlayerUnitCadence(ctx context.Context, request apigen.PlayerUnitCadenceRequestObject) (HandlerResult, error)
	PlayerOutliers(ctx context.Context, request apigen.PlayerOutliersRequestObject) (HandlerResult, error)
	PlayerRecentGames(ctx context.Context, request apigen.PlayerRecentGamesRequestObject) (HandlerResult, error)
	PlayerSummaryOutliers(ctx context.Context, request apigen.PlayerSummaryOutliersRequestObject) (HandlerResult, error)
	PlayerSummaryPerMatchup(ctx context.Context, request apigen.PlayerSummaryPerMatchupRequestObject) (HandlerResult, error)
	PlayerSummarySpecial(ctx context.Context, request apigen.PlayerSummarySpecialRequestObject) (HandlerResult, error)
	ScrepColors(ctx context.Context, request apigen.ScrepColorsRequestObject) (HandlerResult, error)
}
