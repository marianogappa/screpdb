package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/marianogappa/screpdb/internal/dashboard/dashdb"
	"github.com/marianogappa/screpdb/internal/storage"
)

type Dashboard struct {
	ctx           context.Context
	pool          *pgxpool.Pool
	queries       *dashdb.Queries
	conversations map[int]*Conversation
	ai            *AI
}

func New(ctx context.Context, store storage.Storage, postgresConnectionString string, openAIAPIKey string) (*Dashboard, error) {
	if err := runMigrations(postgresConnectionString); err != nil {
		return nil, fmt.Errorf("failed to run migration routine: %w", err)
	}

	pool, err := pgxpool.New(ctx, postgresConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := dashdb.New(pool)

	ai, err := NewAI(ctx, openAIAPIKey, store)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	return &Dashboard{ctx: ctx, pool: pool, queries: queries, ai: ai, conversations: map[int]*Conversation{}}, nil
}

func (d *Dashboard) Run() error {
	r := mux.NewRouter()
	r.Use(mux.CORSMethodMiddleware(r))
	r.HandleFunc("/api/dashboard", d.handlerListDashboards).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerGetDashboard).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerDeleteDashboard).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard", d.handlerCreateDashboard).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerUpdateDashboard).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget", d.handlerListDashboardWidgets).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget/{wid}", d.handlerDeleteDashboardWidget).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget", d.handlerCreateDashboardWidget).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget/{wid}", d.handlerUpdateDashboardWidget).Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/api/health", d.handlerHealthcheck).Methods(http.MethodGet, http.MethodOptions)
	http.Handle("/", r)

	srv := &http.Server{
		Handler:      r,
		Addr:         "localhost:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Println("Server listening on localhost:8000...")
	return srv.ListenAndServe()
}

func (d *Dashboard) handlerGetDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard url missing")
		return
	}

	dash, err := d.queries.GetDashboard(d.ctx, dashboardURL)
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard url")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error getting dashboard url from db: %v", err.Error())
		return
	}

	widgets, err := d.queries.ListDashboardWidgets(d.ctx, pgText(dashboardURL))
	if err != nil && err != sql.ErrNoRows {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error listing dashboard widgets: %v", err.Error())
		return
	}

	type WidgetWithResults struct {
		ID          int64            `json:"id"`
		WidgetOrder pgtype.Int8      `json:"widget_order"`
		Name        string           `json:"name"`
		Description pgtype.Text      `json:"description"`
		Content     string           `json:"content"`
		Query       string           `json:"query"`
		Results     []map[string]any `json:"results"`
		CreatedAt   pgtype.Timestamp `json:"created_at"`
		UpdatedAt   pgtype.Timestamp `json:"updated_at"`
	}

	widgetsWithResults := make([]WidgetWithResults, 0, len(widgets))
	for _, widget := range widgets {
		var results []map[string]any
		if widget.Query != "" {
			queryResults, err := d.executeQuery(widget.Query)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "error executing query for widget %d: %v", widget.ID, err.Error())
				return
			}
			results = queryResults
		}

		widgetsWithResults = append(widgetsWithResults, WidgetWithResults{
			ID:          widget.ID,
			WidgetOrder: widget.WidgetOrder,
			Name:        widget.Name,
			Description: widget.Description,
			Content:     widget.Content,
			Query:       widget.Query,
			Results:     results,
			CreatedAt:   widget.CreatedAt,
			UpdatedAt:   widget.UpdatedAt,
		})
	}

	type DashboardResponse struct {
		dashdb.Dashboard
		Widgets []WidgetWithResults `json:"widgets"`
	}

	response := DashboardResponse{
		Dashboard: dash,
		Widgets:   widgetsWithResults,
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

func (d *Dashboard) handlerDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard url missing")
		return
	}

	err := d.queries.DeleteDashboardWidgetsOfDashboard(d.ctx, pgText(dashboardURL))
	if err != nil && err != sql.ErrNoRows && err != pgx.ErrNoRows {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error deleting dashboard widgets: %v", err.Error())
		return
	}

	err = d.queries.DeleteDashboard(d.ctx, dashboardURL)
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard url")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error deleting dashboard: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (d *Dashboard) handlerDeleteDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardWidgetID, err := strconv.Atoi(vars["wid"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard widget id missing or should be numeric")
		return
	}

	err = d.queries.DeleteDashboardWidget(d.ctx, int64(dashboardWidgetID))
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard widget id")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error deleting dashboard widget: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// func (d *Dashboard) handlerDeleteDashboardWidgetPromptHistory(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)

// 	dashboardWidgetID, err := strconv.Atoi(vars["id"])
// 	if err != nil {
// 		w.WriteHeader(http.StatusBadRequest)
// 		fmt.Fprintf(w, "dashboard widget id missing or should be numeric")
// 		return
// 	}

// 	err = d.queries.DeleteDashboardWidgetPromptHistory(d.ctx, pgInt(dashboardWidgetID))
// 	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
// 		w.WriteHeader(http.StatusNotFound)
// 		fmt.Fprintf(w, "unknown dashboard widget id")
// 		return
// 	}
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		fmt.Fprintf(w, "error deleting dashboard widget: %v", err.Error())
// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// }

func (d *Dashboard) handlerListDashboards(w http.ResponseWriter, _ *http.Request) {
	dash, err := d.queries.ListDashboards(d.ctx)
	if err != nil && err != sql.ErrNoRows && err != pgx.ErrNoRows {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error listing dashboards from db: %v", err.Error())
		return
	}

	if dash == nil {
		dash = []dashdb.Dashboard{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dash)
}

func (d *Dashboard) handlerListDashboardWidgets(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard url missing")
		return
	}

	dash, err := d.queries.ListDashboardWidgets(d.ctx, pgText(dashboardURL))
	if err != nil && err != sql.ErrNoRows {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error listing dashboards from db: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dash)
}

// func (d *Dashboard) handlerListDashboardWidgetPromptHistory(w http.ResponseWriter, r *http.Request) {
// 	vars := mux.Vars(r)

// 	dashboardID, err := strconv.Atoi(vars["dashboard_id"])
// 	if err != nil {
// 		w.WriteHeader(http.StatusBadRequest)
// 		fmt.Fprintf(w, "dashboard id missing or should be numeric")
// 		return
// 	}
// 	dashboardWidgetID, err := strconv.Atoi(vars["dashboard_widget_id"])
// 	if err != nil {
// 		w.WriteHeader(http.StatusBadRequest)
// 		fmt.Fprintf(w, "dashboard widget id missing or should be numeric")
// 		return
// 	}

// 	dash, err := d.queries.ListDashboardWidgetPromptHistory(d.ctx, dashdb.ListDashboardWidgetPromptHistoryParams{DashboardID: pgInt(dashboardID), DashboardWidgetID: pgInt(dashboardWidgetID)})
// 	if err != nil && err != sql.ErrNoRows {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		fmt.Fprintf(w, "error listing dashboards from db: %v", err.Error())
// 		return
// 	}

// 	w.WriteHeader(http.StatusOK)
// 	_ = json.NewEncoder(w).Encode(dash)
// }

func (d *Dashboard) handlerUpdateDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardWidgetID, err := strconv.Atoi(vars["wid"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard widget id missing or should be numeric")
		return
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error reading request POST payload")
		return
	}

	widget, err := d.queries.GetDashboardWidget(d.ctx, int64(dashboardWidgetID))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard widget: %v", err.Error())
		return
	}

	params := dashdb.UpdateDashboardWidgetParams{}
	if err := json.Unmarshal(bs, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid json")
		return
	}

	params.ID = int64(dashboardWidgetID)
	if params.Content == "" {
		params.Content = widget.Content
	}
	if !params.Description.Valid {
		params.Description = widget.Description
	}
	if params.Name == "" {
		params.Name = widget.Name
	}
	if params.Query == "" {
		params.Query = widget.Query
	}

	if err := d.queries.UpdateDashboardWidget(d.ctx, params); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error listing dashboards from db: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (d *Dashboard) handlerUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard url missing")
		return
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error reading request POST payload")
		return
	}

	dash, err := d.queries.GetDashboard(d.ctx, dashboardURL)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard: %v", err.Error())
		return
	}

	params := dashdb.UpdateDashboardParams{}
	if err := json.Unmarshal(bs, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid json")
		return
	}

	params.Url = dashboardURL
	if !params.Description.Valid {
		params.Description = dash.Description
	}
	if params.Name == "" {
		params.Name = dash.Name
	}

	if err := d.queries.UpdateDashboard(d.ctx, params); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error listing dashboards from db: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (d *Dashboard) handlerCreateDashboard(w http.ResponseWriter, r *http.Request) {
	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error reading request POST payload")
		return
	}
	params := dashdb.CreateDashboardParams{}
	if err := json.Unmarshal(bs, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid json: %v", err.Error())
		return
	}

	if params.Url == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "url cannot be empty")
		return
	}

	if params.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "name cannot be empty")
		return
	}

	dash, err := d.queries.CreateDashboard(d.ctx, params)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error creating dashboard: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dash)
}

func (d *Dashboard) handlerCreateDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardURL := vars["url"]
	if dashboardURL == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard url missing")
		return
	}

	type reqParams struct {
		Prompt string
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error reading request POST payload")
		return
	}
	params := reqParams{}
	if err := json.Unmarshal(bs, &params); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "invalid json: %v", err.Error())
		return
	}
	if params.Prompt == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "prompt cannot be empty")
		return
	}

	order, err := d.queries.GetDashboardWidgetNextWidgetOrder(d.ctx, pgText(dashboardURL))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "couldn't get next widget order: %v", err)
		return
	}

	createDashboardWidgetParams := dashdb.CreateDashboardWidgetParams{
		DashboardID: pgText(dashboardURL),
		WidgetOrder: pgInt(int(order)),
	}
	widget, err := d.queries.CreateDashboardWidget(d.ctx, createDashboardWidgetParams)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to create widget: %v", err)
		return
	}

	conv, err := d.ai.NewConversation(int(widget.ID))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to create conversation with AI: %v", err)
		return
	}
	d.conversations[int(widget.ID)] = conv

	resp, err := conv.Prompt(params.Prompt)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to prompt AI to create widget: %v", err)
		return
	}

	updateDashboardWidgetParams := dashdb.UpdateDashboardWidgetParams{
		ID:          int64(widget.ID),
		WidgetOrder: pgInt(int(order)),
		Name:        resp.Title,
		Description: pgtype.Text{String: resp.Description, Valid: true},
		Query:       resp.SQLQuery,
		Content:     resp.HTMLContent,
	}
	if err := d.queries.UpdateDashboardWidget(d.ctx, updateDashboardWidgetParams); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to update widget: %v", err)
		return
	}

	againWidget, err := d.queries.GetDashboardWidget(d.ctx, widget.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to get created widget: %v", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(againWidget)
}

func (d *Dashboard) handlerHealthcheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func pgInt(i int) pgtype.Int8 {
	return pgtype.Int8{Int64: int64(i), Valid: true}
}

func pgText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func (d *Dashboard) executeQuery(query string) ([]map[string]any, error) {
	rows, err := d.pool.Query(d.ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := rows.FieldDescriptions()
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = string(col.Name)
	}

	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columnNames {
			val := values[i]
			// Convert pgtype types to native Go types for JSON serialization
			switch v := val.(type) {
			case pgtype.Text:
				if v.Valid {
					row[col] = v.String
				} else {
					row[col] = nil
				}
			case pgtype.Int8:
				if v.Valid {
					row[col] = v.Int64
				} else {
					row[col] = nil
				}
			case pgtype.Float8:
				if v.Valid {
					row[col] = v.Float64
				} else {
					row[col] = nil
				}
			case pgtype.Bool:
				if v.Valid {
					row[col] = v.Bool
				} else {
					row[col] = nil
				}
			case pgtype.Timestamp:
				if v.Valid {
					row[col] = v.Time
				} else {
					row[col] = nil
				}
			case []byte:
				row[col] = string(v)
			default:
				row[col] = val
			}
		}
		results = append(results, row)
	}

	return results, rows.Err()
}
