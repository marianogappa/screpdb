package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/dashboardrun"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var dashboardOpts dashboardrun.Options

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start LLM Dashboard",
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
	o := dashboardOpts
	o.NormalizeAfterParse()
	return o
}

func RunDashboardWithContext(ctx context.Context, opts dashboardrun.Options) error {
	store, err := storage.NewSQLiteStorage(opts.SQLitePath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite storage: %w", err)
	}

	dash, err := dashboard.New(ctx, store, opts.SQLitePath, opts.AIVendor, opts.AIAPIKey, opts.AIModel)
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

	// Open browser
	log.Printf("Opening browser to %s...", serverURL)
	if err := browser.OpenURL(serverURL); err != nil {
		log.Printf("Warning: failed to open browser: %v", err)
	}

	<-ctx.Done()
	return nil
}

func runDashboard(cmd *cobra.Command, args []string) error {
	signalCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return RunDashboardWithContext(signalCtx, defaultDashboardOptions())
}
