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
	"github.com/marianogappa/screpdb/internal/dashboard/dashdb"
	"github.com/marianogappa/screpdb/internal/storage"
)

type Dashboard struct {
	ctx           context.Context
	conn          *pgx.Conn
	queries       *dashdb.Queries
	conversations map[int]*Conversation
	ai            *AI
}

func New(ctx context.Context, store storage.Storage, postgresConnectionString string, openAIAPIKey string) (*Dashboard, error) {
	if err := runMigrations(postgresConnectionString); err != nil {
		return nil, fmt.Errorf("failed to run migration routine: %w", err)
	}

	conn, err := pgx.Connect(ctx, postgresConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := dashdb.New(conn)

	ai, err := NewAI(ctx, openAIAPIKey, store)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	return &Dashboard{ctx: ctx, conn: conn, queries: queries, ai: ai, conversations: map[int]*Conversation{}}, nil
}

func (d *Dashboard) Run() error {
	r := mux.NewRouter()
	r.Use(mux.CORSMethodMiddleware(r))
	r.HandleFunc("/api/dashboard", d.handlerListDashboards).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}", d.handlerGetDashboard).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}", d.handlerDeleteDashboard).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard", d.handlerCreateDashboard).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}", d.handlerUpdateDashboard).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}/widget", d.handlerListDashboardWidgets).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}/widget/{wid}", d.handlerDeleteDashboardWidget).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}/widget", d.handlerCreateDashboardWidget).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{id}/widget/{wid}", d.handlerUpdateDashboardWidget).Methods(http.MethodPost, http.MethodOptions)

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

	dashboardID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard id missing or should be numeric")
		return
	}

	dash, err := d.queries.GetDashboard(d.ctx, int64(dashboardID))
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard id")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error getting dashboard id from db: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dash)
}

func (d *Dashboard) handlerDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard id missing or should be numeric")
		return
	}

	err = d.queries.DeleteDashboardWidgetPromptHistoriesOfDashboard(d.ctx, pgInt(dashboardID))
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard id")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error deleting dashboard: %v", err.Error())
		return
	}

	err = d.queries.DeleteDashboardWidgetsOfDashboard(d.ctx, pgInt(dashboardID))
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard id")
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error deleting dashboard: %v", err.Error())
		return
	}

	err = d.queries.DeleteDashboard(d.ctx, int64(dashboardID))
	if err == sql.ErrNoRows || err == pgx.ErrNoRows {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard id")
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

	err = d.queries.DeleteDashboardWidgetPromptHistory(d.ctx, pgInt(dashboardWidgetID))
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
	if err != nil && err != sql.ErrNoRows {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error listing dashboards from db: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dash)
}

func (d *Dashboard) handlerListDashboardWidgets(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard id missing or should be numeric")
		return
	}

	dash, err := d.queries.ListDashboardWidgets(d.ctx, pgInt(dashboardID))
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

	dashboardWidgetID, err := strconv.Atoi(vars["id"])
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

	dashboardID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard id missing or should be numeric")
		return
	}

	bs, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "error reading request POST payload")
		return
	}

	dash, err := d.queries.GetDashboard(d.ctx, int64(dashboardID))
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

	params.ID = int64(dashboardID)
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

	if params.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "name cannot be empty")
		return
	}

	dash, err := d.queries.CreateDashboard(d.ctx, params)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "unknown dashboard widget: %v", err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dash)
}

func (d *Dashboard) handlerCreateDashboardWidget(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	dashboardID, err := strconv.Atoi(vars["id"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "dashboard id missing or should be numeric")
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

	order, err := d.queries.GetDashboardWidgetNextWidgetOrder(d.ctx, pgInt(dashboardID))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "couldn't get next widget order: %v", err)
		return
	}

	createDashboardWidgetParams := dashdb.CreateDashboardWidgetParams{
		DashboardID: pgInt(dashboardID),
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
