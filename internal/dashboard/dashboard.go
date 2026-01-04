package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/marianogappa/screpdb/internal/dashboard/variables"
	"github.com/marianogappa/screpdb/internal/storage"
)

type Dashboard struct {
	ctx           context.Context
	db            *sql.DB
	pgxPool       *pgxpool.Pool
	conversations map[int]*Conversation
	ai            *AI
}

func New(ctx context.Context, store storage.Storage, postgresConnectionString string, openAIAPIKey string) (*Dashboard, error) {
	if err := runMigrations(postgresConnectionString); err != nil {
		return nil, fmt.Errorf("failed to run migration routine: %w", err)
	}

	db, err := sql.Open("postgres", postgresConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Create pgx pool for native pgx queries with NamedArgs support
	pgxPool, err := pgxpool.New(ctx, postgresConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgx pool: %w", err)
	}

	if err := pgxPool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping pgx pool: %w", err)
	}

	ai, err := NewAI(ctx, openAIAPIKey, store, db, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create AI client: %w", err)
	}

	return &Dashboard{ctx: ctx, db: db, pgxPool: pgxPool, ai: ai, conversations: map[int]*Conversation{}}, nil
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

func (d *Dashboard) executeQuery(query string, usedVariables map[string]variables.Variable) ([]map[string]any, error) {
	// Build pgx.NamedArgs from variables
	// pgx.NamedArgs automatically converts @placeholder to $1, $2, etc.
	namedArgs := pgx.NamedArgs{}
	for varName, variable := range usedVariables {
		namedArgs[varName] = variable.UsedValue
	}

	// Use pgx's native Query method with NamedArgs
	// The query should already have @variable_name placeholders - no rewriting needed!
	rows, err := d.pgxPool.Query(d.ctx, query, namedArgs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get field descriptions from pgx rows
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			// Convert to native Go types for JSON serialization
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

	return results, rows.Err()
}
