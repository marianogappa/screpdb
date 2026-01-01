package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/pkg/browser"
	"github.com/spf13/cobra"
)

var (
	dashboardPostgresConnString string
	openaiAPIKey                string
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Start LLM Dashboard",
	Long:  ``,
	RunE:  runDashboard,
}

func init() {
	dashboardCmd.Flags().StringVarP(&dashboardPostgresConnString, "postgres-connection-string", "p", "", "PostgreSQL connection string (e.g., 'host=localhost port=5432 user=postgres password=secret dbname=screpdb sslmode=disable')")
	dashboardCmd.Flags().StringVarP(&openaiAPIKey, "openai-api-key", "k", "", "An API KEY from OpenAI in order to prompt for widget creation")
}

func runDashboard(cmd *cobra.Command, args []string) error {
	// TODO: store is Postgres only
	store, err := storage.NewPostgresStorage(dashboardPostgresConnString)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
	}

	dash, err := dashboard.New(cmd.Context(), store, dashboardPostgresConnString, openaiAPIKey)
	if err != nil {
		return err
	}

	// Start backend server asynchronously
	log.Println("Starting backend server...")
	backendReady := dash.StartAsync()
	if err := <-backendReady; err != nil {
		return fmt.Errorf("backend server failed to start: %w", err)
	}

	// Start frontend dev server
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	frontendDir := filepath.Join(cwd, "internal", "dashboard", "frontend")
	log.Println("Starting frontend dev server...")
	frontendCmd := exec.Command("npm", "run", "dev")
	frontendCmd.Dir = frontendDir
	frontendCmd.Stdout = os.Stdout
	frontendCmd.Stderr = os.Stderr
	if err := frontendCmd.Start(); err != nil {
		return fmt.Errorf("failed to start frontend dev server: %w", err)
	}

	// Wait for frontend to be ready
	frontendURL := "http://localhost:3000"
	log.Println("Waiting for frontend dev server to be ready...")
	if err := waitForServerReady(frontendURL, 30, 200*time.Millisecond); err != nil {
		log.Printf("Warning: %v", err)
	}

	// Open browser
	log.Printf("Opening browser to %s...", frontendURL)
	if err := browser.OpenURL(frontendURL); err != nil {
		log.Printf("Warning: failed to open browser: %v", err)
	}

	// Wait for frontend process (this will block until Ctrl+C)
	return frontendCmd.Wait()
}

// waitForServerReady polls the given URL until the server responds with a successful status code,
// or until maxAttempts is reached. Returns nil if the server becomes ready, or an error if it times out.
func waitForServerReady(url string, maxAttempts int, pollInterval time.Duration) error {
	for i := range maxAttempts {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound {
				log.Printf("Server at %s is ready", url)
				return nil
			}
		}
		if i == maxAttempts-1 {
			return fmt.Errorf("server at %s may not be ready after %d attempts", url, maxAttempts)
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("server at %s failed to become ready", url)
}
