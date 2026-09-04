package service

import (
	"context"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
)

// DashboardService is generated from apigen.StrictServerInterface.
type DashboardService interface {
	GetGlobalReplayFilterConfig(ctx context.Context, request apigen.GetGlobalReplayFilterConfigRequestObject) (HandlerResult, error)
	UpdateGlobalReplayFilterConfig(ctx context.Context, request apigen.UpdateGlobalReplayFilterConfigRequestObject) (HandlerResult, error)
	GetLibrarySettings(ctx context.Context, request apigen.GetLibrarySettingsRequestObject) (HandlerResult, error)
	UpdateLibrarySettings(ctx context.Context, request apigen.UpdateLibrarySettingsRequestObject) (HandlerResult, error)
	GamesList(ctx context.Context, request apigen.GamesListRequestObject) (HandlerResult, error)
	GameDetail(ctx context.Context, request apigen.GameDetailRequestObject) (HandlerResult, error)
	GameHotkeys(ctx context.Context, request apigen.GameHotkeysRequestObject) (HandlerResult, error)
	GameSee(ctx context.Context, request apigen.GameSeeRequestObject) (HandlerResult, error)
	Healthcheck(ctx context.Context, request apigen.HealthcheckRequestObject) (HandlerResult, error)
	PlayerColors(ctx context.Context, request apigen.PlayerColorsRequestObject) (HandlerResult, error)
	PlayersList(ctx context.Context, request apigen.PlayersListRequestObject) (HandlerResult, error)
	PlayersApmHistogram(ctx context.Context, request apigen.PlayersApmHistogramRequestObject) (HandlerResult, error)
	PlayersUnitCadence(ctx context.Context, request apigen.PlayersUnitCadenceRequestObject) (HandlerResult, error)
	PlayersViewportMultitasking(ctx context.Context, request apigen.PlayersViewportMultitaskingRequestObject) (HandlerResult, error)
	PlayerDetail(ctx context.Context, request apigen.PlayerDetailRequestObject) (HandlerResult, error)
	PlayerChatSummary(ctx context.Context, request apigen.PlayerChatSummaryRequestObject) (HandlerResult, error)
	PlayerHotkeySignature(ctx context.Context, request apigen.PlayerHotkeySignatureRequestObject) (HandlerResult, error)
	PlayerInsight(ctx context.Context, request apigen.PlayerInsightRequestObject) (HandlerResult, error)
	PlayerApmHistogram(ctx context.Context, request apigen.PlayerApmHistogramRequestObject) (HandlerResult, error)
	PlayerUnitCadence(ctx context.Context, request apigen.PlayerUnitCadenceRequestObject) (HandlerResult, error)
	PlayerLastGames(ctx context.Context, request apigen.PlayerLastGamesRequestObject) (HandlerResult, error)
	ScrepColors(ctx context.Context, request apigen.ScrepColorsRequestObject) (HandlerResult, error)
}
