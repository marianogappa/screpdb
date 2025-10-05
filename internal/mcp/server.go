package mcp

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed starcraft_knowledge.txt
var starcraftKnowledge string

// Server implements the MCP server using mcp-go library
type Server struct {
	storage   storage.Storage
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server
func NewServer(storage storage.Storage) *Server {
	// Create MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		"screpdb-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s := &Server{
		storage:   storage,
		mcpServer: mcpServer,
	}

	// Register the SQL query tool
	sqlTool := mcp.NewTool("query_database",
		mcp.WithDescription("Execute SQL queries against the StarCraft replay database. The database contains tables: replays (metadata), players (player info), actions (game events), units (unit data), buildings (building data). Use this tool to analyze replay statistics, player performance, unit usage, and game patterns."),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("SQL query to execute against the StarCraft replay database"),
		),
	)

	mcpServer.AddTool(sqlTool, s.handleSQLQuery)

	// Register a schema information tool
	schemaTool := mcp.NewTool("get_database_schema",
		mcp.WithDescription("Detailed information about the StarCraft replay database schema including table structures, relationships obtained by querying the database itself."),
	)

	mcpServer.AddTool(schemaTool, s.handleGetSchema)

	// Register a StarCraft knowledge summary tool
	knowledgeTool := mcp.NewTool("get_starcraft_knowledge",
		mcp.WithDescription("Summary of StarCraft knowledge useful for knowing how to answer questions and make reports."),
	)

	mcpServer.AddTool(knowledgeTool, s.handleGetStarCraftKnowledge)

	return s
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	// Start the server using stdio transport
	return server.ServeStdio(s.mcpServer)
}

// handleSQLQuery handles SQL query execution
func (s *Server) handleSQLQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid sql parameter: %v", err)), nil
	}

	// Execute the query
	results, err := s.storage.Query(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query execution failed: %v", err)), nil
	}

	// Format results
	resultText := s.formatQueryResults(results)

	return mcp.NewToolResultText(resultText), nil
}

// handleGetSchema handles schema information requests
func (s *Server) handleGetSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema, err := s.storage.GetDatabaseSchema(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get database schema: %v", err)), nil
	}

	// Add some observations about the dataset
	observations := `
	- Replays have up to 8 players (and up to 4 observers) and a sequential list of commands/actions (like Chess). Command timing is tracked in "frames" since game start and also with a timestamp.
	- The commands table has action-type-specific fields, so for a given row many fields are null.

	- JOIN patterns:
		- players.replay_id = replays.id
		- commands.replay_id = replays.id
		- commands.player_id = players.id

	- Common WHERE clauses:
		- players.type = 'Human' (i.e. skip 'Computer' players)
		- players.is_observer = false (i.e. Observer players are not part of the game)
		- commands.is_effective = true (e.g. an ineffective Train command didn't result in a unit being trained)

	action_types:
		- Build
		- Land
		- RightClick
		- TargetedOrder
		- Train
		- BuildInterceptorOrScarab
		- MinimapPing
		- CancelTrain
		- UnitMorph
		- Tech
		- Upgrade
		- GameSpeed
		- Hotkey
		- Chat
		- Vision
		- Alliance
		- LeaveGame
		- Stop
		- CarrierStop
		- ReaverStop
		- ReturnCargo
		- UnloadAll
		- HoldPosition
		- Burrow
		- Unburrow
		- Siege
		- Unsiege
		- Cloack
		- Decloack
		- Cheat

	unit_types:

		- Supply Depot
		- Forge
		- Hydralisk Den
		- Siege Tank (Tank Mode)
		- Barracks
		- Reaver
		- Engineering Bay
		- ComSat
		- Valkyrie
		- Corsair
		- Creep Colony
		- Extractor
		- Covert Ops
		- Gateway
		- Ultralisk
		- Academy
		- Nuclear Missile
		- Defiler Mound
		- Guardian
		- Spore Colony
		- Templar Archives
		- Arbiter
		- Hive
		- Firebat
		- Zealot
		- Arbiter Tribunal
		- Cybernetics Core
		- Wraith
		- Overlord
		- Evolution Chamber
		- Stargate
		- Physics Lab
		- Spawning Pool
		- Science Facility
		- Fleet Beacon
		- Goliath
		- Probe
		- Missile Turret
		- Sunken Colony
		- Robotics Support Bay
		- Vulture
		- Nuclear Silo
		- Medic
		- Observatory
		- Queen
		- High Templar
		- Starport
		- Ghost
		- Spire
		- Armory
		- Factory
		- Nexus
		- Marine
		- Bunker
		- Battlecruiser
		- Shield Battery
		- Robotics Facility
		- Mutalisk
		- Carrier
		- Hydralisk
		- Shuttle
		- Scourge
		- Observer
		- Greater Spire
		- Devourer
		- Scout
		- Drone
		- Machine Shop
		- Lair
		- Refinery
		- Dark Templar
		- SCV
		- Nydus Canal
		- Queens Nest
		- Dropship
		- Hatchery
		- Ultralisk Cavern
		- Assimilator
		- Science Vessel
		- Dragoon
		- Photon Cannon
		- Lurker
		- Defiler
		- Pylon
		- Control Tower
		- Zergling
		- Citadel of Adun
		- Command Center
	`

	return mcp.NewToolResultText(schema + observations), nil
}

// handleGetStarCraftKnowledge handles StarCraft knowledge summary requests
func (s *Server) handleGetStarCraftKnowledge(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText(starcraftKnowledge), nil
}

// formatQueryResults formats query results for display
func (s *Server) formatQueryResults(results []map[string]any) string {
	if len(results) == 0 {
		return "No results found."
	}

	// Get column names from first row
	var columns []string
	for col := range results[0] {
		columns = append(columns, col)
	}

	// Create table format
	var output strings.Builder
	output.WriteString("Query Results:\n\n")

	// Header
	for i, col := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		output.WriteString(col)
	}
	output.WriteString("\n")

	// Separator
	for i, col := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		for j := 0; j < len(col); j++ {
			output.WriteString("-")
		}
	}
	output.WriteString("\n")

	// Data rows
	for _, row := range results {
		for i, col := range columns {
			if i > 0 {
				output.WriteString(" | ")
			}
			value := fmt.Sprintf("%v", row[col])
			output.WriteString(value)
		}
		output.WriteString("\n")
	}

	output.WriteString(fmt.Sprintf("\nTotal rows: %d", len(results)))

	return output.String()
}
