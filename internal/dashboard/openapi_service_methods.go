package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/dashboard/variables"
	"github.com/marianogappa/screpdb/internal/ingest"
)

var _ dashboardservice.DashboardService = (*Dashboard)(nil)

func (d *Dashboard) ListDashboards(ctx context.Context, _ apigen.ListDashboardsRequestObject) (any, error) {
	dashboards, err := d.listDashboards(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	type dashboardResponse struct {
		URL              string  `json:"url"`
		Name             string  `json:"name"`
		Description      *string `json:"description"`
		ReplaysFilterSQL *string `json:"replays_filter_sql"`
		CreatedAt        *string `json:"created_at"`
	}
	response := make([]dashboardResponse, len(dashboards))
	for i, dash := range dashboards {
		var createdAt *string
		if dash.CreatedAt != nil {
			ts := dash.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
			createdAt = &ts
		}
		response[i] = dashboardResponse{
			URL:              dash.URL,
			Name:             dash.Name,
			Description:      dash.Description,
			ReplaysFilterSQL: dash.ReplaysFilterSQL,
			CreatedAt:        createdAt,
		}
	}
	return response, nil
}

func (d *Dashboard) CreateDashboard(ctx context.Context, request apigen.CreateDashboardRequestObject) (any, error) {
	if request.Body == nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("request body is required"))
	}
	req := request.Body
	if strings.TrimSpace(req.Url) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("url cannot be empty"))
	}
	if strings.TrimSpace(req.Name) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("name cannot be empty"))
	}
	var replaysFilterSQL *string
	if req.ReplaysFilterSql != nil {
		normalized, err := d.validateReplayFilterSQL(req.ReplaysFilterSql)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, fmt.Errorf("invalid replays_filter_sql: %w", err))
		}
		if normalized != "" {
			replaysFilterSQL = &normalized
		}
	}
	dash, err := d.createDashboard(ctx, req.Url, req.Name, req.Description, replaysFilterSQL)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	response := map[string]any{
		"url":                dash.URL,
		"name":               dash.Name,
		"description":        dash.Description,
		"replays_filter_sql": dash.ReplaysFilterSQL,
	}
	if dash.CreatedAt != nil {
		response["created_at"] = dash.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return response, nil
}

func (d *Dashboard) DeleteDashboard(ctx context.Context, request apigen.DeleteDashboardRequestObject) (any, error) {
	if strings.TrimSpace(request.Url) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("dashboard url missing"))
	}
	if err := d.deleteDashboard(ctx, request.Url); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true}, nil
}

func (d *Dashboard) GetDashboard(ctx context.Context, request apigen.GetDashboardRequestObject) (any, error) {
	return d.getDashboardPayload(ctx, request.Url, nil)
}

func (d *Dashboard) GetDashboardPost(ctx context.Context, request apigen.GetDashboardPostRequestObject) (any, error) {
	var values map[string]any
	if request.Body != nil && request.Body.VariableValues != nil {
		values = *request.Body.VariableValues
	}
	return d.getDashboardPayload(ctx, request.Url, values)
}

func (d *Dashboard) getDashboardPayload(ctx context.Context, dashboardURL string, variableValues map[string]any) (any, error) {
	if strings.TrimSpace(dashboardURL) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("dashboard url missing"))
	}
	if err := variables.ValidateReceivedVariableValues(variableValues); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, fmt.Errorf("invalid variable values supplied: %w", err))
	}
	dash, err := d.getDashboardByURL(ctx, dashboardURL)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, errors.New("unknown dashboard url"))
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	widgets, err := d.getDashboardWidgets(ctx, dashboardURL)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	allUsedVariables := map[string]variables.Variable{}
	widgetResponses := make([]map[string]any, 0, len(widgets))
	for _, widget := range widgets {
		if widget.Query != "" {
			for key, variable := range variables.FindVariables(widget.Query, variableValues) {
				allUsedVariables[key] = variable
			}
		}
		config, err := bytesToWidgetConfig([]byte(widget.Config))
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		row := map[string]any{
			"id":           widget.ID,
			"widget_order": widget.WidgetOrder,
			"name":         widget.Name,
			"description":  widget.Description,
			"config":       config,
			"query":        widget.Query,
		}
		if widget.CreatedAt != nil {
			row["created_at"] = widget.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if widget.UpdatedAt != nil {
			row["updated_at"] = widget.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		widgetResponses = append(widgetResponses, row)
	}
	var variableOptions map[string][]any
	if err := d.withFilteredConnection(dash.ReplaysFilterSQL, func(db *sql.DB) error {
		var runErr error
		variableOptions, runErr = variables.RunAllUsedVariableQueries(db, allUsedVariables)
		return runErr
	}); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	variablesResponse := map[string]any{}
	for varName, variable := range allUsedVariables {
		variablesResponse[varName] = map[string]any{
			"name":            variable.Name,
			"display_name":    variable.DisplayName,
			"description":     variable.Description,
			"possible_values": variableOptions[varName],
		}
	}
	response := map[string]any{
		"url":                dash.URL,
		"name":               dash.Name,
		"description":        dash.Description,
		"replays_filter_sql": dash.ReplaysFilterSQL,
		"widgets":            widgetResponses,
		"variables":          variablesResponse,
	}
	if dash.CreatedAt != nil {
		response["created_at"] = dash.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return response, nil
}

func (d *Dashboard) UpdateDashboard(ctx context.Context, request apigen.UpdateDashboardRequestObject) (any, error) {
	if strings.TrimSpace(request.Url) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("dashboard url missing"))
	}
	dash, err := d.getDashboardByURL(ctx, request.Url)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, errors.New("unknown dashboard"))
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	var req apigen.UpdateDashboardRequest
	if request.Body != nil {
		req = *request.Body
	}
	name := dash.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = *req.Name
	}
	description := dash.Description
	if req.Description != nil {
		description = req.Description
	}
	var replaysFilterSQL *string
	if req.ReplaysFilterSql != nil {
		normalized, err := d.validateReplayFilterSQL(req.ReplaysFilterSql)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, fmt.Errorf("invalid replays_filter_sql: %w", err))
		}
		replaysFilterSQL = &normalized
	}
	if err := d.updateDashboard(ctx, request.Url, name, description, replaysFilterSQL); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true}, nil
}

func (d *Dashboard) ListDashboardWidgets(ctx context.Context, request apigen.ListDashboardWidgetsRequestObject) (any, error) {
	if strings.TrimSpace(request.Url) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("dashboard url missing"))
	}
	widgets, err := d.getDashboardWidgets(ctx, request.Url)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	response := make([]map[string]any, 0, len(widgets))
	for _, widget := range widgets {
		row := map[string]any{
			"id":           widget.ID,
			"dashboard_id": widget.DashboardID,
			"widget_order": widget.WidgetOrder,
			"name":         widget.Name,
			"description":  widget.Description,
			"config":       []byte(widget.Config),
			"query":        widget.Query,
		}
		if widget.CreatedAt != nil {
			row["created_at"] = widget.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if widget.UpdatedAt != nil {
			row["updated_at"] = widget.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		response = append(response, row)
	}
	return response, nil
}

func (d *Dashboard) CreateDashboardWidget(ctx context.Context, request apigen.CreateDashboardWidgetRequestObject) (any, error) {
	if strings.TrimSpace(request.Url) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("dashboard url missing"))
	}
	var prompt string
	if request.Body != nil && request.Body.Prompt != nil {
		prompt = strings.TrimSpace(*request.Body.Prompt)
	}
	order, err := d.getNextWidgetOrder(ctx, request.Url)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	emptyConfig := WidgetConfig{Type: WidgetTypeTable}
	configBytes, err := widgetConfigToBytes(emptyConfig)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	widget, err := d.createDashboardWidget(ctx, request.Url, order, "New Widget", nil, configBytes, "")
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	if prompt != "" {
		if !d.ai.IsAvailable() {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("OpenAI API key not configured. Cannot create widget with prompt."))
		}
		conv, err := d.ai.NewConversation(widget.ID)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		d.conversations[int(widget.ID)] = conv
		resp, err := conv.Prompt(prompt)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		configBytes, err = widgetConfigToBytes(resp.Config)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
		var description *string
		if resp.Description != "" {
			description = &resp.Description
		}
		if err := d.updateDashboardWidget(ctx, widget.ID, resp.Title, description, configBytes, resp.SQLQuery, &order); err != nil {
			return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
		}
	}
	againWidget, err := d.getDashboardWidgetByID(ctx, widget.ID)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	config, err := bytesToWidgetConfig([]byte(againWidget.Config))
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	response := map[string]any{
		"id":           againWidget.ID,
		"dashboard_id": againWidget.DashboardID,
		"widget_order": againWidget.WidgetOrder,
		"name":         againWidget.Name,
		"description":  againWidget.Description,
		"config":       config,
		"query":        againWidget.Query,
	}
	if againWidget.CreatedAt != nil {
		response["created_at"] = againWidget.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if againWidget.UpdatedAt != nil {
		response["updated_at"] = againWidget.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return response, nil
}

func (d *Dashboard) DeleteDashboardWidget(ctx context.Context, request apigen.DeleteDashboardWidgetRequestObject) (any, error) {
	if err := d.deleteDashboardWidget(ctx, int64(request.Wid)); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true}, nil
}

func (d *Dashboard) UpdateDashboardWidget(ctx context.Context, request apigen.UpdateDashboardWidgetRequestObject) (any, error) {
	widget, err := d.getDashboardWidgetByID(ctx, int64(request.Wid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, errors.New("unknown dashboard widget"))
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	var req apigen.UpdateDashboardWidgetRequest
	if request.Body != nil {
		req = *request.Body
	}
	name := widget.Name
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name = *req.Name
	}
	description := widget.Description
	if req.Description != nil {
		description = req.Description
	}
	query := widget.Query
	if req.Query != nil {
		query = *req.Query
	}
	currentConfig, err := bytesToWidgetConfig([]byte(widget.Config))
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	configToUse := currentConfig
	if req.Config != nil {
		configBytes, err := json.Marshal(req.Config)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
		}
		if err := json.Unmarshal(configBytes, &configToUse); err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
		}
	}
	configBytes, err := widgetConfigToBytes(configToUse)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	widgetOrder := widget.WidgetOrder
	if widgetOrder == nil {
		zero := int64(0)
		widgetOrder = &zero
	}
	if err := d.updateDashboardWidget(ctx, int64(request.Wid), name, description, configBytes, query, widgetOrder); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{"ok": true}, nil
}

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
		GameTypesMode:     string(body.GameTypesMode),
		ExcludeShortGames: body.ExcludeShortGames,
		ExcludeComputers:  body.ExcludeComputers,
		Maps:              body.Maps,
		MapFilterMode:     string(body.MapFilterMode),
		Players:           body.Players,
		PlayerFilterMode:  string(body.PlayerFilterMode),
	}
	for _, gameType := range body.GameTypes {
		config.GameTypes = append(config.GameTypes, string(gameType))
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

func (d *Dashboard) GetGlobalReplayFilterOptions(ctx context.Context, _ apigen.GetGlobalReplayFilterOptionsRequestObject) (any, error) {
	options, err := d.listGlobalReplayFilterOptions(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return options, nil
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
		Watch:            nullableBoolValue(body.Watch),
		StoreRightClicks: nullableBoolValue(body.StoreRightClicks),
		SkipHotkeys:      nullableBoolValue(body.SkipHotkeys),
		StopAfterN:       nullableIntValue(body.StopAfterNReps),
		UpToDate:         strings.TrimSpace(nullableStringValue(body.UpToYyyyMmDd)),
		UpToMonths:       nullableIntValue(body.UpToNMonths),
		Clean:            nullableBoolValue(body.Clean),
		CleanDashboard:   nullableBoolValue(body.CleanDashboard),
		HandleSignals:    false,
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

func (d *Dashboard) ExecuteQuery(ctx context.Context, request apigen.ExecuteQueryRequestObject) (any, error) {
	if request.Body == nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("request body is required"))
	}
	req := *request.Body
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("query is required"))
	}
	if !isSelectQuery(query) {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("only SELECT queries are allowed"))
	}
	var variableValues map[string]any
	if req.VariableValues != nil {
		variableValues = *req.VariableValues
	}
	if err := variables.ValidateReceivedVariableValues(variableValues); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, fmt.Errorf("invalid variable values supplied: %w", err))
	}
	var replaysFilterSQL *string
	if req.DashboardUrl != nil && strings.TrimSpace(*req.DashboardUrl) != "" {
		dash, err := d.getDashboardByURL(ctx, *req.DashboardUrl)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("unknown dashboard url"))
		}
		replaysFilterSQL = dash.ReplaysFilterSQL
	}
	results, columns, err := d.executeQuery(query, variables.FindVariables(query, variableValues), replaysFilterSQL)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, err)
	}
	return map[string]any{"results": results, "columns": columns}, nil
}

func (d *Dashboard) GetQueryVariables(ctx context.Context, request apigen.GetQueryVariablesRequestObject) (any, error) {
	if request.Body == nil {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("request body is required"))
	}
	req := *request.Body
	usedVariables := variables.FindVariables(req.Query, nil)
	var replaysFilterSQL *string
	if req.DashboardUrl != nil && strings.TrimSpace(*req.DashboardUrl) != "" {
		dash, err := d.getDashboardByURL(ctx, *req.DashboardUrl)
		if err != nil {
			return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("unknown dashboard url"))
		}
		replaysFilterSQL = dash.ReplaysFilterSQL
	}
	var variableOptions map[string][]any
	if err := d.withFilteredConnection(replaysFilterSQL, func(db *sql.DB) error {
		var runErr error
		variableOptions, runErr = variables.RunAllUsedVariableQueries(db, usedVariables)
		return runErr
	}); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	response := map[string]any{}
	for varName, variable := range usedVariables {
		response[varName] = map[string]any{
			"name":            variable.Name,
			"display_name":    variable.DisplayName,
			"description":     variable.Description,
			"possible_values": variableOptions[varName],
		}
	}
	return map[string]any{"variables": response}, nil
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
			ReplayID:        row.ReplayID,
			ReplayDate:      row.ReplayDate,
			FileName:        row.FileName,
			MapName:         row.MapName,
			DurationSeconds: row.DurationSeconds,
			GameType:        row.GameType,
			Players:         []workflowGameListPlayer{},
			Featuring:       []string{},
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

func (d *Dashboard) GameAsk(_ context.Context, request apigen.GameAskRequestObject) (any, error) {
	if request.Body == nil || strings.TrimSpace(request.Body.Question) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("question is required"))
	}
	if !d.ai.IsAvailable() {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("AI is not configured"))
	}
	detail, err := d.buildWorkflowGameDetail(request.ReplayID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	scope := fmt.Sprintf("The answer MUST be scoped to replay_id=%d. Prefer SQL WHERE replay_id = %d when querying replay/player/command tables.", request.ReplayID, request.ReplayID)
	answer, err := d.ai.AnswerWorkflowQuestion(strings.TrimSpace(request.Body.Question), detail, scope)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	results := []map[string]any{}
	columns := []string{}
	if answer.Config.Type != WidgetTypeText && strings.TrimSpace(answer.SQLQuery) != "" {
		filter := dashboarddb.ReplayIDFilterSQL(request.ReplayID)
		qResults, qColumns, queryErr := d.executeQuery(answer.SQLQuery, map[string]variables.Variable{}, &filter)
		if queryErr == nil {
			results = qResults
			columns = qColumns
		}
	}
	return map[string]any{
		"title":       answer.Title,
		"description": answer.Description,
		"config":      answer.Config,
		"sql_query":   answer.SQLQuery,
		"text_answer": answer.TextAnswer,
		"results":     results,
		"columns":     columns,
	}, nil
}

func (d *Dashboard) GameSee(ctx context.Context, request apigen.GameSeeRequestObject) (any, error) {
	const seeReplayFilename = "_watch_me.rep"
	sourceFilePath, err := d.dbStore.GetReplayFilePathByID(ctx, request.ReplayID)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
	}
	destinationDirPath, err := d.dbStore.GetIngestInputDir(ctx, globalReplayFilterConfigKey)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	if destinationDirPath == "" {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, errors.New("Replay ingestion directory is not set; cannot move replay file"))
	}
	destinationFilePath := path.Join(destinationDirPath, seeReplayFilename)
	input, err := os.ReadFile(sourceFilePath)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	if err := os.WriteFile(destinationFilePath, input, 0644); err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"sourceFilePath":      sourceFilePath,
		"destinationFilePath": destinationFilePath,
		"destinationFileName": seeReplayFilename,
		"success":             true,
	}, nil
}

func (d *Dashboard) Healthcheck(ctx context.Context, _ apigen.HealthcheckRequestObject) (any, error) {
	totalReplays, err := d.dbStore.CountReplays(ctx)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return map[string]any{
		"ok":                        true,
		"total_replays":             totalReplays,
		"openai_enabled":            d.ai != nil && d.ai.llm != nil,
		"custom_dashboards_enabled": customDashboardsEnabled(),
	}, nil
}

// customDashboardsEnabled is the poor-man's feature flag for the Custom
// Dashboards UI. Off by default; opt-in via env. Backend API endpoints stay
// reachable so existing bookmarked dashboards keep working when the flag is
// flipped on later — this only controls UI discoverability.
func customDashboardsEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SCREPDB_ENABLE_CUSTOM_DASHBOARDS")))
	return v == "1" || v == "true" || v == "yes" || v == "on"
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

func (d *Dashboard) PlayerAsk(_ context.Context, request apigen.PlayerAskRequestObject) (any, error) {
	if request.Body == nil || strings.TrimSpace(request.Body.Question) == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("question is required"))
	}
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	if !d.ai.IsAvailable() {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("AI is not configured"))
	}
	player, err := d.buildWorkflowPlayerOverview(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	scope := fmt.Sprintf("The answer MUST be scoped to player_key=%q (normalized player name). Prefer SQL WHERE lower(trim(name)) = %q for player-specific analysis.", playerKey, playerKey)
	answer, err := d.ai.AnswerWorkflowQuestion(strings.TrimSpace(request.Body.Question), player, scope)
	if err != nil {
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	results := []map[string]any{}
	columns := []string{}
	if answer.Config.Type != WidgetTypeText && strings.TrimSpace(answer.SQLQuery) != "" {
		qResults, qColumns, queryErr := d.executeQuery(answer.SQLQuery, map[string]variables.Variable{}, nil)
		if queryErr == nil {
			results = qResults
			columns = qColumns
		}
	}
	return map[string]any{
		"title":       answer.Title,
		"description": answer.Description,
		"config":      answer.Config,
		"sql_query":   answer.SQLQuery,
		"text_answer": answer.TextAnswer,
		"results":     results,
		"columns":     columns,
	}, nil
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

func (d *Dashboard) PlayerMetrics(_ context.Context, request apigen.PlayerMetricsRequestObject) (any, error) {
	playerKey := normalizePlayerKey(request.PlayerKey)
	if playerKey == "" {
		return nil, dashboardservice.WithStatus(http.StatusBadRequest, errors.New("player key missing"))
	}
	metrics, err := d.buildWorkflowPlayerMetrics(playerKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dashboardservice.WithStatus(http.StatusNotFound, err)
		}
		return nil, dashboardservice.WithStatus(http.StatusInternalServerError, err)
	}
	return metrics, nil
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
