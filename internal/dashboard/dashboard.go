package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"

	"github.com/marianogappa/screpdb/internal/dashboard/variables"
	"github.com/marianogappa/screpdb/internal/storage"
)

type Dashboard struct {
	ctx           context.Context
	db            *sql.DB
	conversations map[int]*Conversation
	ai            *AI
}

func New(ctx context.Context, store storage.Storage, sqlitePath string, openAIAPIKey string) (*Dashboard, error) {
	if err := runMigrations(sqlitePath); err != nil {
		return nil, fmt.Errorf("failed to run migration routine: %w", err)
	}

	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	ai, err := NewAI(ctx, openAIAPIKey, store, db, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	return &Dashboard{ctx: ctx, db: db, ai: ai, conversations: map[int]*Conversation{}}, nil
}

func (d *Dashboard) setupRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(mux.CORSMethodMiddleware(r))
	r.HandleFunc("/api/dashboard", d.handlerListDashboards).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerGetDashboard).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerDeleteDashboard).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard", d.handlerCreateDashboard).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerUpdateDashboard).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget", d.handlerListDashboardWidgets).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget/{wid}", d.handlerDeleteDashboardWidget).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget", d.handlerCreateDashboardWidget).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget/{wid}", d.handlerUpdateDashboardWidget).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/query", d.handlerExecuteQuery).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/query/variables", d.handlerGetQueryVariables).Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/api/health", d.handlerHealthcheck).Methods(http.MethodGet, http.MethodOptions)
	http.Handle("/", r)
	return r
}

func (d *Dashboard) Run() error {
	r := d.setupRouter()

	srv := &http.Server{
		Handler: r,
		Addr:    "localhost:8000",
		// WriteTimeout: 60 * time.Second,
		// ReadTimeout:  60 * time.Second,
	}

	log.Println("Server listening on localhost:8000...")
	return srv.ListenAndServe()
}

// StartAsync starts the server in a goroutine and returns a channel that will receive an error if the server fails to start,
// or nil when the server is ready. The server will be accessible at localhost:8000.
func (d *Dashboard) StartAsync() <-chan error {
	errChan := make(chan error, 1)
	r := d.setupRouter()

	srv := &http.Server{
		Handler:      r,
		Addr:         "localhost:8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Server starting on localhost:8000...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for server to be ready
	go func() {
		for i := 0; i < 30; i++ {
			resp, err := http.Get("http://localhost:8000/api/health")
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Println("Backend server is ready")
					errChan <- nil
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		errChan <- fmt.Errorf("server failed to start within 3 seconds")
	}()

	return errChan
}

// widgetConfigToBytes converts WidgetConfig to []byte for database storage
func widgetConfigToBytes(config WidgetConfig) ([]byte, error) {
	return json.Marshal(config)
}

// bytesToWidgetConfig converts []byte from database to WidgetConfig
func bytesToWidgetConfig(data []byte) (WidgetConfig, error) {
	var config WidgetConfig
	if len(data) == 0 {
		return config, nil
	}
	err := json.Unmarshal(data, &config)
	return config, err
}

func (d *Dashboard) executeQuery(query string, usedVariables map[string]variables.Variable) ([]map[string]any, []string, error) {
	args := buildNamedArgs(usedVariables)
	rows, err := d.db.QueryContext(d.ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			switch v := val.(type) {
			case []byte:
				row[col] = string(v)
			case nil:
				row[col] = nil
			default:
				row[col] = v
			}
		}
		results = append(results, row)
	}

	return results, columns, rows.Err()
}

func buildNamedArgs(usedVariables map[string]variables.Variable) []any {
	if len(usedVariables) == 0 {
		return nil
	}
	args := make([]any, 0, len(usedVariables))
	for varName, variable := range usedVariables {
		args = append(args, sql.Named(varName, variable.UsedValue))
	}
	return args
}

func sqliteDSN(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "file:screp.db?_pragma=foreign_keys(1)"
	}
	if path == ":memory:" || strings.HasPrefix(path, "file:") {
		if strings.Contains(path, "_pragma=foreign_keys(1)") {
			return path
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return path + sep + "_pragma=foreign_keys(1)"
	}
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)", path)
}
