package dashboard

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/gorilla/mux"

	"github.com/marianogappa/scfingerprint"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/dashboard/apigen"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	dashboardservice "github.com/marianogappa/screpdb/internal/dashboard/service"
	"github.com/marianogappa/screpdb/internal/netfacade"
)

//go:embed frontend/build
var embeddedFrontendBuild embed.FS

type Dashboard struct {
	ctx                 context.Context
	dbStore             dashboarddb.Reader
	library             *libraryRuntime
	libraryHub          *libraryHub
	sampleSetAutoLoaded bool
	headless            bool
	shutdown            func()
	fpDatasetOnce       sync.Once
	fpDataset           *scfingerprint.Dataset
	fpDatasetErr        error
	fpMatchCacheMu      sync.RWMutex
	fpMatchCacheVersion uint64
	fpMatchCache        map[string]*workflowFingerprintMatch
	bnetState           atomic.Value // stores bnetStatus
	bnetAddr            atomic.Value // stores string
	bnetDisabled        atomic.Bool
	bnetGateway         atomic.Int64 // active SC:R gateway (e.g. 20 = Europe, 30 = Korea)
	// bnetBackfillActive counts in-flight profile backfills, so the country-code
	// endpoint can tell a polling page whether more flags are still on the way.
	bnetBackfillActive atomic.Int64
	// youKeys holds the set of replay names that are the user, derived from
	// CSettings.json. Kept in memory rather than persisted: it is a pure
	// function of that file, so a stored copy could only go stale.
	youKeys atomic.Value // stores map[string]struct{}
	// featuredExcl caches which built-in progamer profiles are the user (see
	// excludedProIDs); refreshed after featuredExclusionTTL.
	featuredExclMu sync.Mutex
	featuredExcl   map[string]bool
	featuredExclAt time.Time
}

// SetShutdownFunc registers the callback the /api/custom/quit endpoint invokes to
// stop the process (typically the root context's cancel func). If unset, the
// quit handler falls back to os.Exit(0).
func (d *Dashboard) SetShutdownFunc(fn func()) {
	d.shutdown = fn
}

// Options configures a dashboard server.
type Options struct {
	// Root is the app-data directory: settings.json, the Battle.net caches and
	// the extracted example replays live here.
	Root string
	// ReplayDir overrides the saved replay folder, for headless callers.
	ReplayDir string
	// LegacyDBPath is the pre-library database whose settings and Battle.net
	// caches are carried over once, on the first run after upgrading.
	LegacyDBPath string
	Headless     bool
}

func New(ctx context.Context, opts Options) (*Dashboard, error) {
	dashboard := &Dashboard{
		ctx:        ctx,
		libraryHub: newLibraryHub(),
		headless:   opts.Headless,
	}

	runtime, err := newLibraryRuntime(ctx, libraryRuntimeOptions{
		Root:         opts.Root,
		ReplayDir:    opts.ReplayDir,
		LegacyDBPath: opts.LegacyDBPath,
		Hub:          dashboard.libraryHub,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open the replay library: %w", err)
	}
	dashboard.library = runtime
	dashboard.dbStore = runtime.store
	dashboard.sampleSetAutoLoaded = runtime.sampleSetAutoLoaded
	dashboard.refreshYouKeysBestEffort(ctx)
	return dashboard, nil
}

// ReplayDir is the folder the corpus mirrors.
func (d *Dashboard) ReplayDir() string {
	if d.library == nil {
		return ""
	}
	return d.library.Folder()
}

// StartLibrary begins loading the replay folder and watching it for changes.
// Call it once the HTTP server is accepting, so the first page load already
// has the newest games while the rest stream in behind it.
func (d *Dashboard) StartLibrary() error { return d.library.Start(d.ctx) }

// Close releases the library and stops the loader and watcher.
func (d *Dashboard) Close() {
	if d.library != nil {
		d.library.Close()
	}
}

// invalidateFingerprintCache drops the per-player fingerprint matches when the
// corpus or the filter behind them has moved on.
func (d *Dashboard) invalidateFingerprintCache() {
	d.fpMatchCacheMu.Lock()
	d.fpMatchCache = nil
	d.fpMatchCacheVersion = 0
	d.fpMatchCacheMu.Unlock()
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

// newMethodlessPathMatcher builds a router that matches only the spec paths
// that have no operations (operational endpoints whose operations are excluded
// from code generation). OpenAPI path templates like "/x/{id}" are already
// gorilla-mux compatible, so they register directly.
func newMethodlessPathMatcher(swagger *openapi3.T) *mux.Router {
	m := mux.NewRouter()
	for path, item := range swagger.Paths.Map() {
		if len(item.Operations()) == 0 {
			m.Path(path)
		}
	}
	return m
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
	// Operational/asset endpoints are documented in the spec but their operations
	// are excluded from code generation, so they appear in the embedded spec as
	// method-less path items. Those must defer to their hand-written handlers
	// rather than be rejected by the validator; build a matcher of just those
	// paths so we can still return a clean 405 for genuine wrong-method calls on
	// fully-documented routes.
	methodlessPaths := newMethodlessPathMatcher(swagger)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/api/") || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			route, pathParams, findErr := openapiRouter.FindRoute(r)
			if findErr != nil {
				if errors.Is(findErr, routers.ErrMethodNotAllowed) && !methodlessPaths.Match(r, &mux.RouteMatch{}) {
					// A fully-documented route called with an undocumented method.
					http.Error(w, findErr.Error(), http.StatusMethodNotAllowed)
					return
				}
				// Unknown paths, and operational endpoints served by hand-written
				// handlers (method-less path items), defer to the handler chain
				// (manual routes, then SPA/404 fallback).
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
	r.HandleFunc("/api/custom/library/events", d.handlerLibraryEvents).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/library/rescan", d.handlerLibraryRescan).Methods(http.MethodPost)
	r.HandleFunc("/api/custom/game-assets/unit", d.handlerGameAssetUnit).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/game-assets/building", d.handlerGameAssetBuilding).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/game-assets/map", d.handlerGameAssetMap).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/hotkeys/map", d.handlerHotkeyMap).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/debug/map-layout/{replayID}", d.handlerDebugMapLayout).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/markers/definitions", d.handlerMarkersDefinitions).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/sample-set/load", d.handlerLoadSampleSet).Methods(http.MethodPost)
	r.HandleFunc("/api/custom/update/status", d.handlerUpdateStatus).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/update/apply", d.handlerUpdateApply).Methods(http.MethodPost)
	r.HandleFunc("/api/custom/bnet/status", d.handlerBnetStatus).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/bnet/toggle", d.handlerBnetToggle).Methods(http.MethodPost)
	r.HandleFunc("/api/custom/bnet/profile", d.handlerBnetProfile).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/bnet/country-codes", d.handlerBnetCountryCodes).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/pros/{proID}/photo", d.handlerProPhoto).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/feature-flags", d.handlerFeatureFlags).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/feature-flags", d.handlerSetFeatureFlag).Methods(http.MethodPut)
	r.HandleFunc("/api/custom/gaming-session", d.handlerGamingSession).Methods(http.MethodGet)
	r.HandleFunc("/api/custom/quit", d.handlerQuit).Methods(http.MethodPost)
	apigen.HandlerFromMux(strictHandler, r)
	r.PathPrefix("/api/").Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	if d.headless {
		// API-only mode: don't serve the embedded SPA. Non-API paths get a
		// small JSON pointer to the API surface instead of the dashboard.
		r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"dashboard UI is disabled in headless mode; use the /api endpoints (see api/openapi/dashboard.v1.yaml)"}`))
		})
	} else {
		r.PathPrefix("/").Handler(d.spaHandler())
	}
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
		d.startBnetMonitor(d.ctx)
		select {
		case errChan <- nil:
		default:
		}
	}()

	return errChan
}
