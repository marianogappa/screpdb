package service

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
)

// DashboardService is generated from apigen.StrictServerInterface.
type DashboardService interface {
	ListDashboards(ctx context.Context, request apigen.ListDashboardsRequestObject) (HandlerResult, error)
	CreateDashboard(ctx context.Context, request apigen.CreateDashboardRequestObject) (HandlerResult, error)
	DeleteDashboard(ctx context.Context, request apigen.DeleteDashboardRequestObject) (HandlerResult, error)
	GetDashboard(ctx context.Context, request apigen.GetDashboardRequestObject) (HandlerResult, error)
	GetDashboardPost(ctx context.Context, request apigen.GetDashboardPostRequestObject) (HandlerResult, error)
	UpdateDashboard(ctx context.Context, request apigen.UpdateDashboardRequestObject) (HandlerResult, error)
	ListDashboardWidgets(ctx context.Context, request apigen.ListDashboardWidgetsRequestObject) (HandlerResult, error)
	CreateDashboardWidget(ctx context.Context, request apigen.CreateDashboardWidgetRequestObject) (HandlerResult, error)
	DeleteDashboardWidget(ctx context.Context, request apigen.DeleteDashboardWidgetRequestObject) (HandlerResult, error)
	UpdateDashboardWidget(ctx context.Context, request apigen.UpdateDashboardWidgetRequestObject) (HandlerResult, error)
	GetGlobalReplayFilterConfig(ctx context.Context, request apigen.GetGlobalReplayFilterConfigRequestObject) (HandlerResult, error)
	UpdateGlobalReplayFilterConfig(ctx context.Context, request apigen.UpdateGlobalReplayFilterConfigRequestObject) (HandlerResult, error)
	GetGlobalReplayFilterOptions(ctx context.Context, request apigen.GetGlobalReplayFilterOptionsRequestObject) (HandlerResult, error)
	Ingest(ctx context.Context, request apigen.IngestRequestObject) (HandlerResult, error)
	IngestLogs(ctx context.Context, request apigen.IngestLogsRequestObject) (HandlerResult, error)
	GetIngestSettings(ctx context.Context, request apigen.GetIngestSettingsRequestObject) (HandlerResult, error)
	UpdateIngestSettings(ctx context.Context, request apigen.UpdateIngestSettingsRequestObject) (HandlerResult, error)
	ExecuteQuery(ctx context.Context, request apigen.ExecuteQueryRequestObject) (HandlerResult, error)
	GetQueryVariables(ctx context.Context, request apigen.GetQueryVariablesRequestObject) (HandlerResult, error)
	GamesList(ctx context.Context, request apigen.GamesListRequestObject) (HandlerResult, error)
	GameDetail(ctx context.Context, request apigen.GameDetailRequestObject) (HandlerResult, error)
	GameAsk(ctx context.Context, request apigen.GameAskRequestObject) (HandlerResult, error)
	GameSee(ctx context.Context, request apigen.GameSeeRequestObject) (HandlerResult, error)
	Healthcheck(ctx context.Context, request apigen.HealthcheckRequestObject) (HandlerResult, error)
	PlayerColors(ctx context.Context, request apigen.PlayerColorsRequestObject) (HandlerResult, error)
	PlayersList(ctx context.Context, request apigen.PlayersListRequestObject) (HandlerResult, error)
	PlayersApmHistogram(ctx context.Context, request apigen.PlayersApmHistogramRequestObject) (HandlerResult, error)
	PlayersDelayHistogram(ctx context.Context, request apigen.PlayersDelayHistogramRequestObject) (HandlerResult, error)
	PlayersUnitCadence(ctx context.Context, request apigen.PlayersUnitCadenceRequestObject) (HandlerResult, error)
	PlayersViewportMultitasking(ctx context.Context, request apigen.PlayersViewportMultitaskingRequestObject) (HandlerResult, error)
	PlayerDetail(ctx context.Context, request apigen.PlayerDetailRequestObject) (HandlerResult, error)
	PlayerAsk(ctx context.Context, request apigen.PlayerAskRequestObject) (HandlerResult, error)
	PlayerChatSummary(ctx context.Context, request apigen.PlayerChatSummaryRequestObject) (HandlerResult, error)
	PlayerInsight(ctx context.Context, request apigen.PlayerInsightRequestObject) (HandlerResult, error)
	PlayerApmHistogram(ctx context.Context, request apigen.PlayerApmHistogramRequestObject) (HandlerResult, error)
	PlayerDelayInsight(ctx context.Context, request apigen.PlayerDelayInsightRequestObject) (HandlerResult, error)
	PlayerUnitCadence(ctx context.Context, request apigen.PlayerUnitCadenceRequestObject) (HandlerResult, error)
	PlayerMetrics(ctx context.Context, request apigen.PlayerMetricsRequestObject) (HandlerResult, error)
	PlayerOutliers(ctx context.Context, request apigen.PlayerOutliersRequestObject) (HandlerResult, error)
	PlayerRecentGames(ctx context.Context, request apigen.PlayerRecentGamesRequestObject) (HandlerResult, error)
}
