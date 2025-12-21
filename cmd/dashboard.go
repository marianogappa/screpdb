package cmd

import (
	"fmt"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/storage"
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
	store, err := storage.NewPostgresStorage(postgresConnString)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL storage: %w", err)
	}

	dash, err := dashboard.New(cmd.Context(), store, dashboardPostgresConnString, openaiAPIKey)
	if err != nil {
		return err
	}
	return dash.Run()
}
