package dashboard

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"

	"github.com/marianogappa/screpdb/internal/dashboard/variables"
	"github.com/marianogappa/screpdb/internal/storage"
)

//go:embed frontend/build
var embeddedFrontendBuild embed.FS

type Dashboard struct {
	ctx           context.Context
	db            *sql.DB
	conversations map[int]*Conversation
	ai            *AI
	sqlitePath    string
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

	return &Dashboard{ctx: ctx, db: db, ai: ai, conversations: map[int]*Conversation{}, sqlitePath: sqlitePath}, nil
}

func (d *Dashboard) setupRouter() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/api/dashboard", d.handlerListDashboards).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerGetDashboard).Methods(http.MethodGet, http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerDeleteDashboard).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard", d.handlerCreateDashboard).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}", d.handlerUpdateDashboard).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget", d.handlerListDashboardWidgets).Methods(http.MethodGet, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget/{wid}", d.handlerDeleteDashboardWidget).Methods(http.MethodDelete, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget", d.handlerCreateDashboardWidget).Methods(http.MethodPut, http.MethodOptions)
	r.HandleFunc("/api/dashboard/{url}/widget/{wid}", d.handlerUpdateDashboardWidget).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/query", d.handlerExecuteQuery).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/query/variables", d.handlerGetQueryVariables).Methods(http.MethodPost, http.MethodOptions)
	r.HandleFunc("/api/ingest", d.handlerIngest).Methods(http.MethodPost, http.MethodOptions)

	r.HandleFunc("/api/health", d.handlerHealthcheck).Methods(http.MethodGet, http.MethodOptions)
	r.PathPrefix("/").Handler(d.spaHandler())
	return r
}

func (d *Dashboard) spaHandler() http.Handler {
	buildFS, err := fs.Sub(embeddedFrontendBuild, "frontend/build")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded frontend build is unavailable", http.StatusInternalServerError)
		})
	}
	indexHTML, err := fs.ReadFile(buildFS, "index.html")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "embedded frontend index.html is unavailable", http.StatusInternalServerError)
		})
	}

	fileServer := http.FileServer(http.FS(buildFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(buildFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexHTML)
	})
}

func (d *Dashboard) Run(port int) error {
	r := d.setupRouter()
	addr := fmt.Sprintf("localhost:%d", port)

	srv := &http.Server{
		Handler: r,
		Addr:    addr,
		// WriteTimeout: 60 * time.Second,
		// ReadTimeout:  60 * time.Second,
	}

	log.Printf("Server listening on %s...", addr)
	return srv.ListenAndServe()
}

// StartAsync starts the server in a goroutine and returns a channel that will receive an error if the server fails to start,
// or nil when the server is ready. The server will be accessible at localhost:<port>.
func (d *Dashboard) StartAsync(port int) <-chan error {
	errChan := make(chan error, 2)
	r := d.setupRouter()
	addr := fmt.Sprintf("localhost:%d", port)

	srv := &http.Server{
		Handler:      r,
		Addr:         addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Printf("Server starting on %s...", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Wait for server to be ready
	go func() {
		for i := 0; i < 30; i++ {
			resp, err := http.Get(fmt.Sprintf("http://%s/api/health", addr))
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					log.Println("Backend server is ready")
					select {
					case errChan <- nil:
					default:
					}
					return
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		select {
		case errChan <- fmt.Errorf("server failed to start within 3 seconds"):
		default:
		}
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

func (d *Dashboard) executeQuery(query string, usedVariables map[string]variables.Variable, replaysFilterSQL *string) ([]map[string]any, []string, error) {
	args := buildNamedArgs(usedVariables)
	var results []map[string]any
	var columns []string

	err := d.withFilteredConnection(replaysFilterSQL, func(db *sql.DB) error {
		rows, err := db.QueryContext(d.ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()

		columns, err = rows.Columns()
		if err != nil {
			return err
		}

		for rows.Next() {
			values := make([]any, len(columns))
			valuePtrs := make([]any, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return err
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

		return rows.Err()
	})

	if err != nil {
		return nil, nil, err
	}
	return results, columns, nil
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

func (d *Dashboard) withFilteredConnection(replaysFilterSQL *string, fn func(db *sql.DB) error) error {
	db, err := sql.Open("sqlite", sqliteDSN(d.sqlitePath))
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if replaysFilterSQL != nil {
		filterSQL := normalizeSQL(*replaysFilterSQL)
		if filterSQL != "" {
			if err := applyReplayFilterViews(db, filterSQL); err != nil {
				return err
			}
		}
	}

	return fn(db)
}

func applyReplayFilterViews(db *sql.DB, filterSQL string) error {
	qualified := qualifyReplayFilterSQL(filterSQL)
	if hasUnqualifiedReplays(qualified) {
		return fmt.Errorf("replays_filter_sql must reference main.replays when used in a view")
	}
	if _, err := db.Exec(`CREATE TEMP VIEW replays AS ` + qualified); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW players AS SELECT * FROM main.players WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE TEMP VIEW commands AS SELECT * FROM main.commands WHERE replay_id IN (SELECT id FROM replays)`); err != nil {
		return err
	}
	return nil
}

func normalizeSQL(value string) string {
	trimmed := strings.TrimSpace(value)
	for strings.HasSuffix(trimmed, ";") {
		trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	}
	return trimmed
}

func normalizeSQLWhitespace(value string) string {
	trimmed := normalizeSQL(value)
	re := regexp.MustCompile(`\s+`)
	return re.ReplaceAllString(trimmed, " ")
}

func qualifyReplayFilterSQL(filterSQL string) string {
	normalized := normalizeSQLWhitespace(filterSQL)
	qualified := normalized
	tables := []string{"replays", "players", "commands"}
	for _, table := range tables {
		re := regexp.MustCompile(`(?i)\b(from|join)\s+(?:main\.)?(?:\"` + table + `\"|` + "`" + table + "`" + `|\[` + table + `\]|` + table + `)\b`)
		qualified = re.ReplaceAllString(qualified, "${1} main."+table)
	}
	return qualified
}

func hasUnqualifiedReplays(filterSQL string) bool {
	tables := []string{"replays", "players", "commands"}
	for _, table := range tables {
		re := regexp.MustCompile(`(?i)\b(from|join)\s+(?:\"` + table + `\"|` + "`" + table + "`" + `|\[` + table + `\]|` + table + `)\b`)
		if re.MatchString(filterSQL) {
			return true
		}
	}
	return false
}
