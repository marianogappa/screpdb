package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var (
	dashboardSQLitePath string
	openaiAPIKey        string
	dashboardPort       int
)

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
	cmd.Flags().StringVarP(&dashboardSQLitePath, "sqlite-path", "s", "screp.db", "SQLite database file path")
	cmd.Flags().StringVarP(&openaiAPIKey, "openai-api-key", "k", "", "An API KEY from OpenAI in order to prompt for widget creation")
	cmd.Flags().IntVarP(&dashboardPort, "port", "p", 8000, "Dashboard server port")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	store, err := storage.NewSQLiteStorage(dashboardSQLitePath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite storage: %w", err)
	}

	dash, err := dashboard.New(cmd.Context(), store, dashboardSQLitePath, openaiAPIKey)
	if err != nil {
		return err
	}

	// Start backend server asynchronously
	serverURL := fmt.Sprintf("http://localhost:%d", dashboardPort)
	log.Printf("Starting dashboard server on %s...", serverURL)
	backendReady := dash.StartAsync(dashboardPort)
	if err := <-backendReady; err != nil {
		return fmt.Errorf("dashboard server failed to start: %w", err)
	}

	// Open browser
	log.Printf("Opening browser to %s...", serverURL)
	if err := browser.OpenURL(serverURL); err != nil {
		log.Printf("Warning: failed to open browser: %v", err)
	}

	// Keep process running while the server is active.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	return nil
}
