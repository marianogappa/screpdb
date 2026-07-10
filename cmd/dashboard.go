package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/dashboardrun"
	"github.com/marianogappa/screpdb/internal/netfacade"
	"github.com/marianogappa/screpdb/internal/selfupdate"
	"github.com/marianogappa/screpdb/internal/winsandbox"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var dashboardOpts dashboardrun.Options

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start the dashboard",
	Long:  ``,
	RunE:  runDashboard,
}

func init() {
	addDashboardFlags(dashboardCmd)
}

func addDashboardFlags(cmd *cobra.Command) {
	dashboardrun.RegisterFlags(cmd.Flags(), &dashboardOpts)
}

func defaultDashboardOptions() dashboardrun.Options {
	return dashboardOpts
}

func RunDashboardWithContext(ctx context.Context, opts dashboardrun.Options) error {
	// Remove the placeholder left by a previous self-update swap (issue #212).
	selfupdate.CleanupOldBinary()
	if selfupdate.IsRestart() {
		// Self-update relaunch: wait for the previous process to release the
		// listening port (Windows has no exec-in-place) before we bind.
		time.Sleep(1500 * time.Millisecond)
	} else {
		// Single-instance guard: if screpdb is already serving on the requested
		// port, reconnect to it (open the browser) rather than starting an
		// invisible second server or failing on a busy port. If the port is held
		// by a foreign process, fall through to the next free port in range.
		// Skipped during a self-update relaunch, where we deliberately want to
		// wait for and rebind the same port.
		resolvedPort, alreadyRunning, resolveErr := resolveDashboardPort(opts.Port)
		if resolveErr != nil {
			return resolveErr
		}
		if alreadyRunning {
			serverURL := fmt.Sprintf("http://localhost:%d", resolvedPort)
			log.Printf("screpdb is already running at %s; opening it instead of starting another instance.", serverURL)
			if !opts.Headless {
				if err := openDashboardBrowser(serverURL); err != nil {
					log.Printf("Warning: failed to open browser: %v", err)
				}
			}
			return nil
		}
		opts.Port = resolvedPort
	}

	// Own the shutdown lifetime so the dashboard "Quit" button can stop us
	// cleanly: cancelling this context unblocks the <-ctx.Done() below, which on
	// Windows lets the launcher observe a clean worker exit and drop its tray.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	dbPath, err := appdata.ResolveDBPath(opts.SQLitePath)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}
	log.Printf("Using SQLite database at %s", dbPath)

	dash, err := dashboard.New(ctx, dbPath, opts.Headless)
	if err != nil {
		return err
	}
	dash.SetShutdownFunc(cancel)

	// Start backend server asynchronously
	serverURL := fmt.Sprintf("http://localhost:%d", opts.Port)
	log.Printf("Starting dashboard server on %s...", serverURL)
	backendReady := dash.StartAsync(opts.Port)
	if err := <-backendReady; err != nil {
		return fmt.Errorf("dashboard server failed to start: %w", err)
	}

	// Now that the server is up and DB setup is complete, run any sample-set
	// ingest queued at startup (deferred to avoid racing DB initialization).
	dash.StartPendingSampleIngest()

	// Open browser, unless this is a self-update relaunch — the user's existing
	// tab is still pointed here and will reconnect, so a second tab is just noise.
	// Headless mode is API-only, so there's no UI to open.
	switch {
	case opts.Headless:
		log.Printf("Running in headless mode; dashboard UI disabled. API available at %s/api", serverURL)
	case selfupdate.IsRestart():
		log.Printf("Self-update relaunch complete; refresh the existing browser tab to load the new version.")
	default:
		log.Printf("Opening browser to %s...", serverURL)
		if err := openDashboardBrowser(serverURL); err != nil {
			log.Printf("Warning: failed to open browser: %v", err)
		}
		// A successful open call does not guarantee a window appeared (on Windows
		// ShellExecute reports success even when nothing launches), so always tell
		// the user where to go if it didn't.
		log.Printf("If no browser window opened, open %s manually.", serverURL)
	}

	<-ctx.Done()
	return nil
}

// dashboardPortScanRange bounds how many consecutive ports single-instance
// resolution will probe starting at the requested port before giving up.
const dashboardPortScanRange = 10

// resolveDashboardPort implements screpdb's single-instance policy. Starting at
// the preferred port it scans a small range and returns either a free port to
// bind (alreadyRunning=false) or the port of an existing screpdb to reconnect to
// (alreadyRunning=true). A port held by a foreign process is skipped. It errors
// only if the whole range is occupied by non-screpdb processes.
func resolveDashboardPort(preferred int) (port int, alreadyRunning bool, err error) {
	for p := preferred; p < preferred+dashboardPortScanRange; p++ {
		addr := fmt.Sprintf("localhost:%d", p)
		if netfacade.LocalPortAvailable(addr) {
			return p, false, nil
		}
		if netfacade.IsLocalScrepdb(addr, 2*time.Second) {
			return p, true, nil
		}
		log.Printf("Port %d is in use by another process; trying the next one.", p)
	}
	return 0, false, fmt.Errorf(
		"no free port found in range %d-%d, and no existing screpdb to reconnect to",
		preferred, preferred+dashboardPortScanRange-1)
}

// openDashboardBrowser opens the dashboard in the default browser. The
// Low-integrity Windows worker cannot do this itself — a ShellExecute from Low
// integrity silently fails to spawn a window (issue #237) — so it hands the URL
// to the Medium launcher via the broker. Every other build opens it directly.
func openDashboardBrowser(serverURL string) error {
	if winsandbox.IsWorker() {
		dir, err := appdata.Dir()
		if err != nil {
			return fmt.Errorf("resolve app-data dir: %w", err)
		}
		return winsandbox.BrokerOpenURL(dir, serverURL)
	}
	return browser.OpenURL(serverURL)
}

func runDashboard(cmd *cobra.Command, args []string) error {
	signalCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return RunDashboardWithContext(signalCtx, defaultDashboardOptions())
}
