package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/icza/screp/rep/repcore"
	"github.com/marianogappa/screpdb/internal/buildinfo"
	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/ingest"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/storage"
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
	config.CompiledReplaysFilterSQL = body.CompiledReplaysFilterSql
	updated, err := d.updateGlobalReplayFilterConfig(ctx, config)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	if err := d.refreshReplayScopedDB(); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return updated, nil
}

func (d *Dashboard) GetGlobalReplayFilterOptions(_ context.Context, _ apigen.GetGlobalReplayFilterOptionsRequestObject) (any, error) {
	// Player options + mode toggles were removed; the endpoint still
	// exists in the OpenAPI spec for backward compatibility but returns
	// an empty payload. Frontend no longer calls it.
	return map[string]any{
		"top_players":   []any{},
		"other_players": []any{},
	}, nil
}

func (d *Dashboard) Ingest(ctx context.Context, request apigen.IngestRequestObject) (any, error) {
	body := apigen.IngestRequest{}
	if request.Body != nil {
		body = *request.Body
	}
	inputDir := strings.TrimSpace(nullableStringValue(body.InputDir))
	if inputDir != "" {
		if err := d.setIngestInputDir(ctx, inputDir); err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
		}
	} else {
		var err error
		inputDir, err = d.getIngestInputDir(ctx)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		if inputDir == "" {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("replay folder is not configured"))
		}
	}
	cfg := ingest.Config{
		InputDir:         inputDir,
		SQLitePath:       strings.TrimSpace(nullableStringValue(body.SqlitePath)),
		StoreRightClicks: nullableBoolValue(body.StoreRightClicks),
		SkipHotkeys:      nullableBoolValue(body.SkipHotkeys),
		StopAfterN:       nullableIntValue(body.StopAfterNReps),
		UpToDate:         strings.TrimSpace(nullableStringValue(body.UpToYyyyMmDd)),
		UpToMonths:       nullableIntValue(body.UpToNMonths),
		Clean:            nullableBoolValue(body.Clean),
		CleanDashboard:   nullableBoolValue(body.CleanDashboard),
		UseColor:         false,
		Logger:           d.newIngestLogger(),
	}
	if cfg.SQLitePath == "" {
		cfg.SQLitePath = d.sqlitePath
	}
	if !d.tryStartIngest(cfg.InputDir) {
		return map[string]any{
			"ok":          true,
			"started":     false,
			"in_progress": true,
			"input_dir":   inputDir,
			"sqlitePath":  cfg.SQLitePath,
		}, nil
	}
	go func() {
		runErr := ingest.Run(d.ctx, cfg)
		if runErr != nil {
			cfg.Logger.Errorf("Ingestion failed: %v", runErr)
		}
		d.finishIngest(runErr)
	}()
	return map[string]any{
		"ok":         true,
		"started":    true,
		"input_dir":  cfg.InputDir,
		"sqlitePath": cfg.SQLitePath,
	}, nil
}

func (d *Dashboard) IngestLogs(_ context.Context, _ apigen.IngestLogsRequestObject) (any, error) {
	return map[string]any{"upgraded": true}, nil
}

// GetStaleReplaysCount reports how many replays were analyzed under an algorithm version
// older than core.AlgorithmVersion. Used by the dashboard to decide whether to surface
// the bulk re-analyze banner.
func (d *Dashboard) GetStaleReplaysCount(ctx context.Context, _ apigen.GetStaleReplaysCountRequestObject) (any, error) {
	store, err := storage.NewSQLiteStorage(d.sqlitePath)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	defer store.Close()

	count, err := store.CountStaleReplays(ctx, core.AlgorithmVersion)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"count":           count,
		"current_version": core.AlgorithmVersion,
	}, nil
}


func (d *Dashboard) GetIngestSettings(ctx context.Context, _ apigen.GetIngestSettingsRequestObject) (any, error) {
	inputDir, err := d.getIngestInputDir(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return ingestSettingsResponse{InputDir: inputDir}, nil
}

func (d *Dashboard) UpdateIngestSettings(ctx context.Context, request apigen.UpdateIngestSettingsRequestObject) (any, error) {
	var inputDir string
	if request.Body != nil && request.Body.InputDir != nil {
		inputDir = *request.Body.InputDir
	}
	if err := d.setIngestInputDir(ctx, inputDir); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	return ingestSettingsResponse{InputDir: strings.TrimSpace(inputDir)}, nil
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
	whereSQL, whereArgs := buildWorkflowGamesListWhere(filters)
	total, err := d.dbStore.CountGamesWithWhere(ctx, whereSQL, whereArgs)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	listRows, err := d.dbStore.ListGamesWithWhere(ctx, whereSQL, whereArgs, limit, offset)
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
		if errors.Is(err, sql.ErrNoRows) {
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
	const seeReplayFolderName = "000_screpdb_watch_me"
	const seeReplayFilename = "watch_me.rep"
	sourceFilePath, err := d.dbStore.GetReplayFilePathByID(ctx, request.ReplayID)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
	}
	ingestDirPath, err := d.getIngestInputDir(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	if ingestDirPath == "" {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, errors.New("Replay ingestion directory is not set; cannot move replay file"))
	}
	destinationDirPath := path.Join(ingestDirPath, seeReplayFolderName)
	if err := iofacade.MkdirAll(destinationDirPath, 0755); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	destinationFilePath := path.Join(destinationDirPath, seeReplayFilename)
	input, err := iofacade.ReadFile(sourceFilePath)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	if err := iofacade.WriteFile(destinationFilePath, input, 0644); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
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
	return map[string]any{
		"ok":            true,
		"total_replays": totalReplays,
		"version":       buildinfo.Version,
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
	return map[string]any{
		"summary_version": workflowSummaryVersion,
		"items":           items,
		"limit":           limit,
		"offset":          offset,
		"total":           total,
		"filter_options":  filterOptions,
	}, nil
}

func (d *Dashboard) PlayersApmHistogram(_ context.Context, _ apigen.PlayersApmHistogramRequestObject) (any, error) {
	histogram, err := d.buildWorkflowPlayerApmHistogram("")
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return histogram, nil
}

func (d *Dashboard) PlayersDelayHistogram(_ context.Context, _ apigen.PlayersDelayHistogramRequestObject) (any, error) {
	histogram, err := d.buildWorkflowPlayerDelayHistogram()
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
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
	return result, nil
}

func (d *Dashboard) PlayersViewportMultitasking(_ context.Context, _ apigen.PlayersViewportMultitaskingRequestObject) (any, error) {
	result, err := d.buildWorkflowPlayerViewportMultitaskingDistribution()
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerDetail(_ context.Context, request apigen.PlayerDetailRequestObject) (any, error) {
	if strings.TrimSpace(request.PlayerKey) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	player, err := d.buildWorkflowPlayerOverview(normalizePlayerKey(request.PlayerKey))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	chatSummary, err := d.buildPlayerChatSummary(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	result, err := d.buildWorkflowPlayerAsyncInsight(playerKey, insightType)
	if err != nil {
		if errors.Is(err, errUnsupportedWorkflowPlayerInsightType) {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
		}
		if errors.Is(err, sql.ErrNoRows) {
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
	histogram, err := d.buildWorkflowPlayerApmHistogram(playerKey)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return histogram, nil
}

func (d *Dashboard) PlayerDelayInsight(_ context.Context, request apigen.PlayerDelayInsightRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	result, err := d.buildWorkflowPlayerDelayInsight(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
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
	result, err := d.buildWorkflowPlayerUnitCadenceInsight(playerKey, filterMode)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerOutliers(_ context.Context, request apigen.PlayerOutliersRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	outliers, err := d.buildWorkflowPlayerOutliers(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return outliers, nil
}

func (d *Dashboard) PlayerSummaryPerMatchup(_ context.Context, request apigen.PlayerSummaryPerMatchupRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	result, err := d.buildWorkflowPlayerSummaryPerMatchup(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerSummarySpecial(_ context.Context, request apigen.PlayerSummarySpecialRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	result, err := d.buildWorkflowPlayerSummarySpecial(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerSummaryOutliers(_ context.Context, request apigen.PlayerSummaryOutliersRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	category := strings.TrimSpace(request.Params.Category)
	if category == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("category is required"))
	}
	result, err := d.buildWorkflowPlayerSummaryOutliersForCategory(playerKey, category)
	if err != nil {
		if errors.Is(err, errUnknownOutlierCategory) {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
		}
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return result, nil
}

func (d *Dashboard) PlayerRecentGames(_ context.Context, request apigen.PlayerRecentGamesRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	games, err := d.buildWorkflowPlayerRecentGames(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"player_key":      playerKey,
		"recent_games":    games,
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
