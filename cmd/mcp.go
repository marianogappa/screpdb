package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/marianogappa/screpdb/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	mcpPort        int
	mcpNoAutoStart bool
	mcpReplayDir   string
	mcpWaitForLoad bool
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for querying the replay corpus",
	Long: `Start a Model Context Protocol (MCP) server that answers questions about your
replays in natural language.

It holds no data of its own: it reads the JSON API of a running screpdb, which
owns the in-memory replay corpus. If the dashboard is already open it attaches
to that; otherwise it starts a headless server of its own and shuts it down on
exit.`,
	RunE: runMCP,
}

func init() {
	mcpCmd.Flags().IntVarP(&mcpPort, "port", "p", 8000, "First port to look for a running screpdb on")
	mcpCmd.Flags().BoolVar(&mcpNoAutoStart, "no-auto-start", false, "Fail instead of starting a headless screpdb when none is running")
	mcpCmd.Flags().StringVar(&mcpReplayDir, "replay-dir", "", "Replay folder for a server this command starts. Defaults to the saved folder.")
	mcpCmd.Flags().BoolVar(&mcpWaitForLoad, "wait-for-load", true, "Wait for a server this command starts to finish reading the replay folder before serving")
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx, stopSignals := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	client, stopServer, err := mcp.Connect(ctx, mcp.ConnectOptions{
		Port:          mcpPort,
		AutoStart:     !mcpNoAutoStart,
		ReplayDir:     mcpReplayDir,
		WaitForCorpus: mcpWaitForLoad,
	})
	if err != nil {
		return fmt.Errorf("failed to reach a screpdb server: %w", err)
	}
	defer stopServer()

	log.Printf("MCP server running against %s", client.Addr())
	return mcp.NewServer(client).Start(ctx)
}
