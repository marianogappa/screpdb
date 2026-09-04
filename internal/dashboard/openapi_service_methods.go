package dashboard

import (
	"context"
	"errors"
	"fmt"
	"github.com/marianogappa/screpdb/internal/propack"
	"net/http"
	"path"
	"strings"

	"github.com/icza/screp/rep/repcore"
	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/buildinfo"
	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/winsandbox"
)

var _ dashboardservice.DashboardService = (*Dashboard)(nil)

func (d *Dashboard) GetGlobalReplayFilterConfig(ctx context.Context, _ apigen.GetGlobalReplayFilterConfigRequestObject) (any, error) {
	config, err := d.getGlobalReplayFilterConfig(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return config, nil
}

func (d *Dashboard) UpdateGlobalReplayFilterConfig(ctx context.Context, request apigen.UpdateGlobalReplayFilterConfigRequestObject) (any, error) {
	if request.Body == nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("request body is required"))
	}
	body := request.Body
	config := globalReplayFilterConfig{
		GameTypes:         make([]string, 0, len(body.GameTypes)),
		ExcludeShortGames: body.ExcludeShortGames,
		ExcludeComputers:  body.ExcludeComputers,
		MapKinds:          make([]string, 0, len(body.MapKinds)),
	}
	for _, gameType := range body.GameTypes {
		config.GameTypes = append(config.GameTypes, string(gameType))
	}
	for _, mapKind := range body.MapKinds {
		config.MapKinds = append(config.MapKinds, string(mapKind))
	}
	updated, err := d.updateGlobalReplayFilterConfig(ctx, config)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	d.invalidateFingerprintCache()
	return updated, nil
}

func (d *Dashboard) GetLibrarySettings(ctx context.Context, _ apigen.GetLibrarySettingsRequestObject) (any, error) {
	return d.librarySettings(ctx), nil
}

func (d *Dashboard) UpdateLibrarySettings(ctx context.Context, request apigen.UpdateLibrarySettingsRequestObject) (any, error) {
	var replayDir string
	if request.Body != nil && request.Body.ReplayDir != nil {
		replayDir = *request.Body.ReplayDir
	}
	if err := d.setReplayDir(ctx, replayDir); err != nil {
		return nil, err
	}
	return d.librarySettings(ctx), nil
}

func (d *Dashboard) GamesList(ctx context.Context, request apigen.GamesListRequestObject) (any, error) {
	limit, offset := 20, 0
	if request.Params.Limit != nil && *request.Params.Limit > 0 {
		limit = int(*request.Params.Limit)
		if limit > 200 {
			limit = 200
		}
	}
	if request.Params.Offset != nil && *request.Params.Offset >= 0 {
		offset = int(*request.Params.Offset)
	}
	filters := workflowGamesListFilters{}
	if request.Params.Player != nil {
		filters.PlayerKeys = parseCSVQueryValues(*request.Params.Player, true)
	}
	if request.Params.Map != nil {
		filters.MapNames = parseCSVQueryValues(*request.Params.Map, false)
	}
	if request.Params.Duration != nil {
		filters.DurationBuckets = parseCSVQueryValues(*request.Params.Duration, true)
	}
	if request.Params.Featuring != nil {
		filters.FeaturingKeys = parseCSVQueryValues(*request.Params.Featuring, true)
	}
	if request.Params.Matchup != nil {
		filters.MatchupKeys = parseCSVQueryValues(*request.Params.Matchup, true)
	}
	if request.Params.MapKind != nil {
		filters.MapKindKeys = parseCSVQueryValues(*request.Params.MapKind, true)
	}
	query := buildGamesQuery(filters)
	total, err := d.dbStore.CountGames(ctx, query)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	listRows, err := d.dbStore.ListGames(ctx, query, limit, offset)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	items := []workflowGameListItem{}
	for _, row := range listRows {
		items = append(items, workflowGameListItem{
			ReplayID:           row.ReplayID,
			ReplayDate:         row.ReplayDate,
			FileName:           row.FileName,
			MapName:            row.MapName,
			MapKind:            row.MapKind,
			GameSource:         row.GameSource,
			LobbyKind:          row.LobbyKind,
			DurationSeconds:    row.DurationSeconds,
			GameType:           row.GameType,
			Matchup:            row.Matchup,
			TeamStacking:       row.TeamStacking,
			TeamInfoIncomplete: row.TeamInfoIncomplete,
			Players:            []workflowGameListPlayer{},
			Featuring:          []string{},
		})
	}
	if err := d.populateWorkflowGameListPlayers(items); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	if err := d.populateWorkflowGameListFeaturing(items); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	filterOptions, err := d.workflowGamesListFilterOptions()
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"summary_version": workflowSummaryVersion,
		"items":           items,
		"limit":           limit,
		"offset":          offset,
		"total":           total,
		"filter_options":  filterOptions,
	}, nil
}

func (d *Dashboard) GameDetail(_ context.Context, request apigen.GameDetailRequestObject) (any, error) {
	detail, err := d.buildWorkflowGameDetail(request.ReplayID)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return detail, nil
}

func (d *Dashboard) GameSee(ctx context.Context, request apigen.GameSeeRequestObject) (any, error) {
	// The folder name starts with "000_" so it sorts above other folders, and folders
	// sort above files in StarCraft's replay browser — making the staged replay easy
	// to find. The file inside is just "watch_me.rep" since the folder already carries
	// the screpdb prefix.
	const seeReplayFolderName = fileops.WatchMeDirName
	const seeReplayFilename = "watch_me.rep"
	sourceFilePath, err := d.dbStore.GetReplayFilePathByID(ctx, request.ReplayID)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
	}
	ingestDirPath := d.library.Folder()
	if ingestDirPath == "" {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, errors.New("Replay ingestion directory is not set; cannot move replay file"))
	}
	destinationDirPath := path.Join(ingestDirPath, seeReplayFolderName)
	destinationFilePath := path.Join(destinationDirPath, seeReplayFilename)
	if winsandbox.IsWorker() {
		// The replays folder is read-only to the Low-integrity worker; ask the
		// Medium launcher to stage the file there on our behalf (issue #237).
		appDir, err := appdata.Dir()
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		dest, err := winsandbox.BrokerSeeReplay(appDir, sourceFilePath, ingestDirPath)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		destinationFilePath = dest
	} else {
		if err := iofacade.MkdirAll(destinationDirPath, 0755); err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		input, err := iofacade.ReadFile(sourceFilePath)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		if err := iofacade.WriteFile(destinationFilePath, input, 0644); err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
	}
	return map[string]any{
		"sourceFilePath":      sourceFilePath,
		"destinationFilePath": destinationFilePath,
		"destinationFileName": seeReplayFilename,
		"destinationFolder":   seeReplayFolderName,
		"success":             true,
	}, nil
}

func (d *Dashboard) Healthcheck(ctx context.Context, _ apigen.HealthcheckRequestObject) (any, error) {
	totalReplays, err := d.dbStore.CountReplays(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	progress := d.libraryHub.Progress()
	return map[string]any{
		"app":           "screpdb",
		"ok":            true,
		"total_replays": totalReplays,
		"version":       buildinfo.Version,
		"commit":        buildinfo.Commit,
		"library": map[string]any{
			"status":     statusForPhase(progress.Phase),
			"phase":      string(progress.Phase),
			"generation": progress.Generation,
			"version":    progress.Version,
			"loaded":     progress.Loaded,
			"total":      progress.Total,
			"failed":     progress.Failed,
			"skipped":    progress.Skipped,
			"replay_dir": progress.Folder,
			"complete":   progress.Complete(),
		},
	}, nil
}

func (d *Dashboard) PlayerColors(ctx context.Context, _ apigen.PlayerColorsRequestObject) (any, error) {
	rows, err := d.dbStore.ListTopPlayerColorRows(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	playerColors := map[string]string{}
	for i, row := range rows {
		if i >= len(topPlayerPalette) {
			break
		}
		playerColors[row.PlayerKey] = topPlayerPalette[i]
	}
	return map[string]any{"player_colors": playerColors, "palette": topPlayerPalette}, nil
}

// ScrepColors returns the canonical screp player-color palette: a map from
// normalized name (lowercased, spaces stripped — matches the frontend's lookup
// key) to the engine RGB as a #rrggbb string. Sourced from repcore.Colors so
// the values track whatever screp version the binary links against.
func (d *Dashboard) ScrepColors(_ context.Context, _ apigen.ScrepColorsRequestObject) (any, error) {
	out := make(map[string]string, len(repcore.Colors))
	for _, c := range repcore.Colors {
		key := strings.ReplaceAll(strings.ToLower(c.Name), " ", "")
		out[key] = fmt.Sprintf("#%06x", c.RGB)
	}
	return out, nil
}

func (d *Dashboard) PlayersList(_ context.Context, request apigen.PlayersListRequestObject) (any, error) {
	limit, offset := 20, 0
	if request.Params.Limit != nil && *request.Params.Limit > 0 {
		limit = int(*request.Params.Limit)
		if limit > 200 {
			limit = 200
		}
	}
	if request.Params.Offset != nil && *request.Params.Offset >= 0 {
		offset = int(*request.Params.Offset)
	}
	filters := workflowPlayersListFilters{}
	if request.Params.Name != nil {
		filters.NameContains = strings.TrimSpace(*request.Params.Name)
	}
	if request.Params.Only5Plus != nil {
		raw := strings.ToLower(strings.TrimSpace(*request.Params.Only5Plus))
		filters.OnlyFivePlus = raw == "1" || raw == "true" || raw == "on" || raw == "yes"
	}
	if request.Params.LastPlayed != nil {
		filters.LastPlayedBuckets = parseCSVQueryValues(*request.Params.LastPlayed, true)
	}
	sortSpec := workflowPlayersListSort{Column: "games_played", Desc: true}
	if request.Params.SortBy != nil {
		switch *request.Params.SortBy {
		case apigen.Name:
			sortSpec.Column = "player_name"
		case apigen.Race:
			sortSpec.Column = "race"
		case apigen.Games:
			sortSpec.Column = "games_played"
		case apigen.Apm:
			sortSpec.Column = "average_apm"
		case apigen.LastPlayed:
			sortSpec.Column = "last_played_days_ago"
		}
	}
	if request.Params.SortDir != nil {
		sortSpec.Desc = *request.Params.SortDir != apigen.Asc
	}
	items, total, filterOptions, err := d.listWorkflowPlayers(limit, offset, filters, sortSpec)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	// Built-in progamer profiles are not rows of the paginated local list (they
	// have no games-played or last-played to sort by), so they travel as their
	// own group, only with the first page, filtered by the same name search.
	featured := []workflowFeaturedPlayerItem{}
	if offset == 0 {
		featured = d.featuredPlayersList(filters.NameContains)
	}
	return map[string]any{
		"summary_version":  workflowSummaryVersion,
		"items":            items,
		"featured_players": featured,
		"limit":            limit,
		"offset":           offset,
		"total":            total,
		"filter_options":   filterOptions,
	}, nil
}

func (d *Dashboard) PlayersApmHistogram(_ context.Context, _ apigen.PlayersApmHistogramRequestObject) (any, error) {
	histogram, err := d.buildWorkflowPlayerApmHistogram("")
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	histogram.FeaturedPlayers = d.featuredApmPoints()
	return histogram, nil
}

func (d *Dashboard) PlayersUnitCadence(_ context.Context, request apigen.PlayersUnitCadenceRequestObject) (any, error) {
	filterMode, err := parseWorkflowUnitCadenceFilterMode(nullableStringValue(request.Params.Filter))
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	minGames := workflowUnitCadenceMinGames
	if request.Params.MinGames != nil && *request.Params.MinGames > 0 {
		minGames = *request.Params.MinGames
	}
	limit := workflowUnitCadenceDefaultLimit
	if request.Params.Limit != nil {
		if *request.Params.Limit < 0 {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("limit must be >= 0"))
		}
		limit = *request.Params.Limit
	}
	if limit > workflowUnitCadenceMaxLimit {
		limit = workflowUnitCadenceMaxLimit
	}
	result, err := d.buildWorkflowPlayerUnitCadenceLeaderboard(filterMode, minGames, limit)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	result.FeaturedPlayers = d.featuredCadencePoints()
	return result, nil
}

func (d *Dashboard) PlayersViewportMultitasking(_ context.Context, _ apigen.PlayersViewportMultitaskingRequestObject) (any, error) {
	result, err := d.buildWorkflowPlayerViewportMultitaskingDistribution()
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	result.FeaturedPlayers = d.featuredViewportPoints()
	return result, nil
}

func (d *Dashboard) PlayerDetail(_ context.Context, request apigen.PlayerDetailRequestObject) (any, error) {
	if strings.TrimSpace(request.PlayerKey) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	playerKey := normalizePlayerKey(request.PlayerKey)
	build := d.buildWorkflowPlayerOverview
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		build = d.buildFeaturedPlayerOverview
	}
	player, err := build(playerKey)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return player, nil
}

func (d *Dashboard) PlayerChatSummary(_ context.Context, request apigen.PlayerChatSummaryRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, errFeaturedPlayerHasNoLocalData)
	}
	chatSummary, err := d.buildPlayerChatSummary(playerKey)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"player_key":      playerKey,
		"chat_summary":    chatSummary,
		"summary_version": workflowSummaryVersion,
	}, nil
}

func (d *Dashboard) PlayerInsight(_ context.Context, request apigen.PlayerInsightRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	insightType := workflowPlayerInsightType(nullableStringValue(request.Params.Type))
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		pro := d.featuredPro(playerKey)
		if pro == nil {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, dashboarddb.ErrNotFound)
		}
		result, err := d.featuredAsyncInsight(pro, insightType)
		if err != nil {
			if errors.Is(err, errUnsupportedWorkflowPlayerInsightType) {
				return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
			}
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		return result, nil
	}
	result, err := d.buildWorkflowPlayerAsyncInsight(playerKey, insightType)
	if err != nil {
		if errors.Is(err, errUnsupportedWorkflowPlayerInsightType) {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
		}
		if errors.Is(err, dashboarddb.ErrNotFound) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerApmHistogram(_ context.Context, request apigen.PlayerApmHistogramRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, errFeaturedPlayerHasNoLocalData)
	}
	histogram, err := d.buildWorkflowPlayerApmHistogram(playerKey)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return histogram, nil
}

func (d *Dashboard) PlayerUnitCadence(_ context.Context, request apigen.PlayerUnitCadenceRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	filterMode, err := parseWorkflowUnitCadenceFilterMode(nullableStringValue(request.Params.Filter))
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, errFeaturedPlayerHasNoLocalData)
	}
	result, err := d.buildWorkflowPlayerUnitCadenceInsight(playerKey, filterMode)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerLastGames(_ context.Context, request apigen.PlayerLastGamesRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	if _, isPro := propack.IDFromKey(playerKey); isPro {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, errFeaturedPlayerHasNoLocalData)
	}
	games, err := d.buildWorkflowPlayerLastGames(playerKey)
	if err != nil {
		if errors.Is(err, dashboarddb.ErrNotFound) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"player_key":      playerKey,
		"last_games":      games,
		"summary_version": workflowSummaryVersion,
	}, nil
}

func nullableIntValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func nullableBoolValue(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}
