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

	dash, err := dashboard.New(ctx, dbPath)
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
	if selfupdate.IsRestart() {
		log.Printf("Self-update relaunch complete; refresh the existing browser tab to load the new version.")
	} else {
		log.Printf("Opening browser to %s...", serverURL)
		if err := browser.OpenURL(serverURL); err != nil {
			log.Printf("Warning: failed to open browser: %v", err)
		}
	}

	<-ctx.Done()
	return nil
}

func runDashboard(cmd *cobra.Command, args []string) error {
	signalCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return RunDashboardWithContext(signalCtx, defaultDashboardOptions())
}
