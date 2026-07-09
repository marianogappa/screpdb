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
	}

	dbPath, err := appdata.ResolveDBPath(opts.SQLitePath)
	if err != nil {
		return fmt.Errorf("failed to resolve database path: %w", err)
	}
	log.Printf("Using SQLite database at %s", dbPath)

	dash, err := dashboard.New(ctx, dbPath, opts.Headless)
	if err != nil {
		return err
	}

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
