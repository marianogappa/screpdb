package dashboard

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gorilla/mux"
	_ "modernc.org/sqlite"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/ingest"
	"github.com/marianogappa/screpdb/internal/netfacade"
)

//go:embed frontend/build
var embeddedFrontendBuild embed.FS

type Dashboard struct {
	ctx                 context.Context
	db                  *sql.DB
	dbStore             *dashboarddb.Store
	replayScopedMu      sync.RWMutex
	replayScopedDB      *sql.DB
	globalReplayFilter  globalReplayFilterConfig
	sqlitePath          string
	ingestMu            sync.Mutex
	ingestRunning       bool
	ingestStatus        string
	ingestError         string
	ingestInputDir      string
	ingestSessionID     int64
	ingestEvents        []ingest.LogEvent
	ingestSubscribers   map[chan ingestStreamMessage]struct{}
	sampleSetAutoLoaded bool
	pendingSampleIngest bool
}

func New(ctx context.Context, sqlitePath string) (*Dashboard, error) {
	if err := runMigrations(sqlitePath); err != nil {
		return nil, fmt.Errorf("failed to run migration routine: %w", err)
	}

	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := dashboarddb.EnableForeignKeys(db); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	dashboard := &Dashboard{
		ctx:          ctx,
		db:           db,
		ingestStatus: "idle",
		sqlitePath:   sqlitePath,
	}
	dashboard.dbStore = dashboarddb.NewStore(dashboard.db, dashboard.currentReplayScopedDB, dashboard.withFilteredConnection)
	if err := dashboard.initializeIngestSettings(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize ingest settings: %w", err)
	}
	if err := dashboard.refreshReplayScopedDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize replay scoped db: %w", err)
	}
	return dashboard, nil
}

// recoveryMiddleware turns a panic in any HTTP handler into a crash report and
// a 500, without tearing down the server. net/http already isolates handler
// panics from the process, but it does so silently — the GUI user has no
// console, so the report file (and the prefilled issue link logged with it) is
// their only way to surface the bug. Browser-open is deliberately suppressed
// here (unlike goroutine Guards): a repeatedly-panicking request must not spawn
// a new issue tab on every retry.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if rec == http.ErrAbortHandler {
				panic(rec) // net/http's own sentinel — let it handle it
			}
			crashreport.Handle(rec, debug.Stack(), false)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}

func (d *Dashboard) setupRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(recoveryMiddleware)
	swagger, err := apigen.GetSwagger()
	if err != nil {
		panic(fmt.Errorf("failed to load embedded OpenAPI spec: %w", err))
	}
	swagger.Servers = nil
	openapiRouter, err := gorillamux.NewRouter(swagger)
	if err != nil {
		panic(fmt.Errorf("failed to create OpenAPI validator router: %w", err))
	}
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/") || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			route, pathParams, findErr := openapiRouter.FindRoute(r)
			if findErr != nil {
				if errors.Is(findErr, routers.ErrMethodNotAllowed) {
					http.Error(w, findErr.Error(), http.StatusMethodNotAllowed)
					return
				}
				// Unknown API paths should continue to SPA fallback behavior.
				next.ServeHTTP(w, r)
				return
			}

			requestValidationInput := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
			}
			if validationErr := openapi3filter.ValidateRequest(r.Context(), requestValidationInput); validationErr != nil {
				status := http.StatusBadRequest
				if _, ok := validationErr.(*openapi3filter.SecurityRequirementsError); ok {
					status = http.StatusUnauthorized
				}
				http.Error(w, validationErr.Error(), status)
				return
			}

			next.ServeHTTP(w, r)
		})
	})
	strictHandler := apigen.NewStrictHandlerWithOptions(
		newOpenAPIStrictAdapter(d),
		nil,
		apigen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
				http.Error(w, err.Error(), http.StatusBadRequest)
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
				http.Error(w, err.Error(), dashboardservice.StatusCode(err))
			},
		},
	)
	// websocket endpoint remains a manual route to preserve Upgrade semantics
	r.HandleFunc("/api/custom/ingest/logs", d.handlerIngestLogs).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/game-assets/unit", d.handlerGameAssetUnit).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/game-assets/building", d.handlerGameAssetBuilding).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/game-assets/map", d.handlerGameAssetMap).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/debug/map-layout/{replayID}", d.handlerDebugMapLayout).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/markers/definitions", d.handlerMarkersDefinitions).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/sample-set/load", d.handlerLoadSampleSet).Methods(http.MethodPost)
	r.HandleFunc("/api/custom/update/status", d.handlerUpdateStatus).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/update/apply", d.handlerUpdateApply).Methods(http.MethodPost)
	apigen.HandlerFromMux(strictHandler, r)
	r.PathPrefix("/api/").Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
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
		Handler: r,
		Addr:    addr,
		// Aggregate dashboard queries (player summary outliers, per-race
		// segmented pills) can run for over a minute on large corpora —
		// SQLite is single-connection-serialized and Random players need
		// 3x the per-race work. 240s caps zombie connections while
		// letting honest queries complete. ReadHeaderTimeout stays short
		// to limit slow-loris.
		WriteTimeout:      240 * time.Second,
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		defer crashreport.Guard()
		log.Printf("Server starting on %s...", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case errChan <- err:
			default:
			}
		}
	}()

	// Wait for the server to be ready. A localhost TCP dial (via netfacade) is
	// sufficient: routes are registered before ListenAndServe, so once the
	// listener accepts, the dashboard is serving. This makes no outbound
	// network call (issue #135).
	go func() {
		defer crashreport.Guard()
		if err := netfacade.WaitForLocalListener(addr, 30, 100*time.Millisecond); err != nil {
			select {
			case errChan <- err:
			default:
			}
			return
		}
		log.Println("Backend server is ready")
		select {
		case errChan <- nil:
		default:
		}
	}()

	return errChan
}

func (d *Dashboard) executeQuery(query string, replaysFilterSQL *string) ([]map[string]any, []string, error) {
	var (
		results []map[string]any
		columns []string
	)
	err := d.dbStore.WithFilteredConnection(replaysFilterSQL, func(db *sql.DB) error {
		rows, err := dashboarddb.QueryContextOnDB(d.ctx, db, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		scannedRows, scannedColumns, scanErr := dashboarddb.ScanDynamicRows(rows)
		if scanErr != nil {
			return scanErr
		}
		results = scannedRows
		columns = scannedColumns
		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	return results, columns, nil
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
	effectiveFilterSQL := composeReplayFilterSQL(d.currentGlobalReplayFilterSQL(), replaysFilterSQL)
	if normalizeSQL(nullableStringValue(replaysFilterSQL)) == "" {
		return d.withReplayScopedDB(fn)
	}
	db, err := openReplayScopedDB(d.sqlitePath, effectiveFilterSQL)
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}

func applyReplayFilterViews(db *sql.DB, filterSQL string) error {
	qualified := qualifyReplayFilterSQL(filterSQL)
	if hasUnqualifiedReplays(qualified) {
		return fmt.Errorf("replays_filter_sql must reference main.replays when used in a view")
	}
	return dashboarddb.ApplyReplayTempViews(db, qualified)
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
	tables := []string{
		"replays",
		"players",
		"commands",
		"commands_low_value",
		"replay_events",
	}
	for _, table := range tables {
		re := regexp.MustCompile(`(?i)\b(from|join)\s+(?:main\.)?(?:\"` + table + `\"|` + "`" + table + "`" + `|\[` + table + `\]|` + table + `)\b`)
		qualified = re.ReplaceAllString(qualified, "${1} main."+table)
	}
	return qualified
}

func hasUnqualifiedReplays(filterSQL string) bool {
	tables := []string{
		"replays",
		"players",
		"commands",
		"commands_low_value",
		"replay_events",
	}
	for _, table := range tables {
		re := regexp.MustCompile(`(?i)\b(from|join)\s+(?:\"` + table + `\"|` + "`" + table + "`" + `|\[` + table + `\]|` + table + `)\b`)
		if re.MatchString(filterSQL) {
			return true
		}
	}
	return false
}

func openReplayScopedDB(sqlitePath string, replaysFilterSQL *string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteDSN(sqlitePath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := dashboarddb.EnableForeignKeys(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	filterSQL := normalizeSQL(nullableStringValue(replaysFilterSQL))
	if filterSQL != "" {
		if err := applyReplayFilterViews(db, filterSQL); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	return db, nil
}

func (d *Dashboard) currentReplayScopedDB() *sql.DB {
	d.replayScopedMu.RLock()
	defer d.replayScopedMu.RUnlock()
	if d.replayScopedDB != nil {
		return d.replayScopedDB
	}
	return d.db
}

func (d *Dashboard) withReplayScopedDB(fn func(db *sql.DB) error) error {
	d.replayScopedMu.RLock()
	db := d.replayScopedDB
	d.replayScopedMu.RUnlock()
	if db == nil {
		db = d.db
	}
	return fn(db)
}

func (d *Dashboard) currentGlobalReplayFilterSQL() *string {
	d.replayScopedMu.RLock()
	defer d.replayScopedMu.RUnlock()
	if d.globalReplayFilter.CompiledReplaysFilterSQL == nil {
		return nil
	}
	value := *d.globalReplayFilter.CompiledReplaysFilterSQL
	return &value
}

func (d *Dashboard) refreshReplayScopedDB() error {
	config, err := d.getGlobalReplayFilterConfig(d.ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		config = defaultGlobalReplayFilterConfig()
		if _, updateErr := d.updateGlobalReplayFilterConfig(d.ctx, config); updateErr != nil {
			return updateErr
		}
	}

	db, err := openReplayScopedDB(d.sqlitePath, config.CompiledReplaysFilterSQL)
	if err != nil {
		return err
	}

	d.replayScopedMu.Lock()
	oldDB := d.replayScopedDB
	d.replayScopedDB = db
	d.globalReplayFilter = config
	d.replayScopedMu.Unlock()

	if oldDB != nil {
		_ = oldDB.Close()
	}
	return nil
}
