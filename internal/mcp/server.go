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

	// Register the SQL query tool. This is the workhorse: it answers arbitrary
	// natural-language questions about any ingested game or player by running
	// read-only SQL. Call get_database_schema first to learn the real columns.
	sqlTool := mcp.NewTool("query_database",
		mcp.WithDescription("Run a read-only SQL query (SELECT/WITH/EXPLAIN/PRAGMA only) against the StarCraft: Remastered replay database and get the rows back. Tables: replays (one row per game: map, matchup, duration, engine), players (one row per player per replay: race, APM/eAPM, is_winner, start location), commands (the ordered action stream — builds, trains, morphs, tech, upgrades, micro), commands_low_value (high-volume noise: right-clicks, hotkeys, pings — usually excluded), replay_events (derived analysis: build-order openers, timing markers, and narrative game events like rushes/drops/proxies), player_aliases (maps battle.net tags to canonical player identities). Call get_database_schema for exact columns and get_starcraft_knowledge for domain terms before writing non-trivial queries."),
		mcp.WithString("sql",
			mcp.Required(),
			mcp.Description("A single read-only SQL statement (SELECT, WITH, EXPLAIN, or PRAGMA). Writes are rejected."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	mcpServer.AddTool(sqlTool, s.handleSQLQuery)

	// Register a schema information tool
	schemaTool := mcp.NewTool("get_database_schema",
		mcp.WithDescription("Return the live database schema (columns and types for every table, introspected from the database itself) plus curated notes on join patterns, common WHERE clauses, and the values found in key columns. Read this before writing a query."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	mcpServer.AddTool(schemaTool, s.handleGetSchema)

	// Register a StarCraft knowledge summary tool
	knowledgeTool := mcp.NewTool("get_starcraft_knowledge",
		mcp.WithDescription("Return domain knowledge about StarCraft: Remastered and how screpdb models it — game mechanics, build orders, meta terminology (rush, timing push, tech switch, natural), and how the derived replay_events (markers and game events) are computed. Read this to answer strategy questions correctly."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	mcpServer.AddTool(knowledgeTool, s.handleGetStarCraftKnowledge)

	// Curated discovery tools so an agent can orient itself without guessing
	// SQL against an empty result set.
	playersTool := mcp.NewTool("list_top_players",
		mcp.WithDescription("List the human players with the most games in the database (name, game count, and the races they've played). Use this to discover who is in the corpus before asking player-specific questions."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of players to return (default 25)."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(playersTool, s.handleListTopPlayers)

	eventsTool := mcp.NewTool("list_event_types",
		mcp.WithDescription("List the derived analysis available in replay_events: every (event_kind, event_type) pair with how many rows exist. Markers are per-replay summaries (build-order openers, timings); game_events are narrative moments (rushes, drops, proxies). Use this to learn what strategic patterns you can query."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(eventsTool, s.handleListEventTypes)

	return s
}

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	// Start the server using stdio transport
	return server.ServeStdio(s.mcpServer)
}

// readOnlyLeadingKeywords are the statement kinds the query tool accepts. The
// database holds an expensive, hand-curated corpus, so we never let a client
// (or the LLM driving it) mutate it — this tool is for questions, not edits.
var readOnlyLeadingKeywords = []string{"SELECT", "WITH", "EXPLAIN", "PRAGMA"}

// ensureReadOnly rejects anything that isn't a single read-only statement.
func ensureReadOnly(sql string) error {
	stmt := stripSQLComments(sql)

	// Disallow stacked statements (e.g. "SELECT 1; DROP TABLE x"). A single
	// trailing semicolon is fine.
	if trimmed := strings.TrimRight(strings.TrimSpace(stmt), ";"); strings.Contains(trimmed, ";") {
		return fmt.Errorf("only a single read-only statement is allowed; multiple statements are not permitted")
	}

	fields := strings.Fields(strings.TrimSpace(stmt))
	if len(fields) == 0 {
		return fmt.Errorf("empty query")
	}
	leading := strings.ToUpper(fields[0])
	for _, kw := range readOnlyLeadingKeywords {
		if leading == kw {
			return nil
		}
	}
	return fmt.Errorf("only read-only queries are allowed (must start with one of %s); got %q", strings.Join(readOnlyLeadingKeywords, ", "), leading)
}

// stripSQLComments removes -- line comments and /* */ block comments so the
// leading-keyword check can't be fooled by a comment prefix.
func stripSQLComments(sql string) string {
	var b strings.Builder
	for i := 0; i < len(sql); i++ {
		if i+1 < len(sql) && sql[i] == '-' && sql[i+1] == '-' {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(sql) && sql[i] == '/' && sql[i+1] == '*' {
			i += 2
			for i+1 < len(sql) && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			i++ // skip the '/' of '*/' (loop's i++ skips the '*')
			continue
		}
		b.WriteByte(sql[i])
	}
	return b.String()
}

// handleSQLQuery handles SQL query execution
func (s *Server) handleSQLQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid sql parameter: %v", err)), nil
	}

	if err := ensureReadOnly(query); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
	- Replays have up to 8 players (and up to 4 observers) and a sequential list of commands/actions (like Chess). Command timing is tracked in "frames" since game start and also with a timestamp (seconds_from_game_start).
	- The commands table has action-type-specific fields, so for a given row many fields are null.
	- commands vs commands_low_value: high-signal actions (Build, Train, morphs, Tech, Upgrade, targeted micro) live in commands; high-volume noise (Right Click, Hotkey, Minimap Ping, Vision, Alliance) is split into commands_low_value so it can be excluded from analysis. Same schema in both. Right-clicks/hotkeys are only stored if ingestion was configured to keep them, so don't assume they exist.
	- replay_events is the DERIVED analysis layer (not raw stream). event_kind = 'marker' rows are one-per-(replay, player, event_type) summaries screpdb computed — build-order openers are stored as event_type feature keys prefixed 'bo_' (e.g. bo_9_pool, bo_12_hatch, bo_gate_expand, bo_t_111); opener_unresolved / *_fuzzy / bo_*_other are catch-alls. Other markers are timings/behaviours (e.g. used_hotkey_groups, viewport_multitasking, never_upgraded). event_kind = 'game_event' rows are narrative moments (rushes, drops, proxies, nydus, mind control, scout, expansion). source_player_id/target_player_id join to players.id; location_base_type ('starting'|'natural'|'expansion') and location_base_oclock give map position; payload is optional JSON. To discover the actual event_type values, use the list_event_types tool or: SELECT event_kind, event_type, COUNT(*) FROM replay_events GROUP BY 1,2 ORDER BY 3 DESC.
	- player_aliases maps battle.net tags to canonical player identities. players.name is the raw in-replay name; join through player_aliases (battle_tag_normalized) when you need to group a person's games across smurfs/tags.

	- JOIN patterns:
		- players.replay_id = replays.id
		- commands.replay_id = replays.id (also commands_low_value.replay_id = replays.id)
		- commands.player_id = players.id
		- replay_events.replay_id = replays.id
		- replay_events.source_player_id = players.id (the acting player; target_player_id is the player acted upon, may be NULL)

	- Common WHERE clauses:
		- players.type = 'Human' (i.e. skip 'Computer' players)
		- players.is_observer = false (i.e. Observer players are not part of the game)
		- replays.matchup is a normalized string like 'TvZ' or 'PvP' for 1v1s (and e.g. 'PvPvTvZ' for larger games); replays.duration_seconds is already in seconds, frame_count is the raw frame length (≈ 23.81 frames/sec on fastest). For 1v1 analysis, filter to matchup values with a single 'v' (e.g. matchup LIKE '_v_').

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

// handleListTopPlayers lists the human players with the most games.
func (s *Server) handleListTopPlayers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := request.GetInt("limit", 25)
	if limit <= 0 || limit > 500 {
		limit = 25
	}
	query := fmt.Sprintf(`
		SELECT name,
		       COUNT(*) AS games,
		       GROUP_CONCAT(DISTINCT race) AS races
		FROM players
		WHERE type = 'Human' AND is_observer = 0
		GROUP BY name
		ORDER BY games DESC
		LIMIT %d`, limit)

	results, err := s.storage.Query(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query execution failed: %v", err)), nil
	}
	return mcp.NewToolResultText(s.formatQueryResults(results)), nil
}

// handleListEventTypes enumerates the derived replay_events available to query.
func (s *Server) handleListEventTypes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := `
		SELECT event_kind, event_type, COUNT(*) AS rows
		FROM replay_events
		GROUP BY event_kind, event_type
		ORDER BY event_kind, rows DESC`

	results, err := s.storage.Query(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Query execution failed: %v", err)), nil
	}
	return mcp.NewToolResultText(s.formatQueryResults(results)), nil
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
