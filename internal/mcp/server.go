// Package mcp exposes the screpdb replay corpus to an MCP client. It owns no
// data of its own: every answer is read from the JSON API of the process that
// loaded the replays into memory (issue #381), which is either an already
// running screpdb or a headless one this server started.
package mcp

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

//go:embed starcraft_knowledge.txt
var starcraftKnowledge string

// maxToolResultBytes caps one tool result. Game detail is the only response
// that regularly approaches it; the model is told when it was cut.
const maxToolResultBytes = 400 << 10

type Server struct {
	client    *Client
	mcpServer *server.MCPServer
}

func NewServer(client *Client) *Server {
	mcpServer := server.NewMCPServer(
		"screpdb-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(true),
	)
	s := &Server{client: client, mcpServer: mcpServer}

	// The workhorse: every question about the corpus is a GET against one of the
	// exposed endpoints, filtered and aggregated by the model.
	queryTool := mcp.NewTool("query_replay_api",
		mcp.WithDescription("Read the screpdb replay corpus over its read-only JSON API and get the response back. Pass an API path with an optional query string, e.g. /api/games?limit=20&matchup=TvZ, /api/games/{replayID}, /api/players?sort_by=games&sort_dir=desc, or /api/players/{playerKey}/insight?type=build_orders. Only a curated read-only subset of the dashboard API is reachable; call get_api_schema first for the exact paths and their parameters, and get_starcraft_knowledge for what the data means."),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("An API path, optionally with a query string. Must start with /api/."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(queryTool, s.handleQuery)

	schemaTool := mcp.NewTool("get_api_schema",
		mcp.WithDescription("Return every API path query_replay_api can reach, with its path and query parameters, plus notes on how the corpus is modelled and which questions each endpoint answers. Read this before calling query_replay_api."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(schemaTool, s.handleGetSchema)

	knowledgeTool := mcp.NewTool("get_starcraft_knowledge",
		mcp.WithDescription("Return domain knowledge about StarCraft: Remastered and how screpdb models it — game mechanics, build orders, meta terminology (rush, timing push, tech switch, natural), and how the derived markers and game events are computed. Read this to answer strategy questions correctly."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(knowledgeTool, s.handleGetStarCraftKnowledge)

	// Curated discovery tools, so an agent can orient itself without guessing at a
	// corpus it has not seen.
	playersTool := mcp.NewTool("list_top_players",
		mcp.WithDescription("List the players with the most games in the corpus. Use this to discover who is in it before asking player-specific questions; the playerKey each row carries is what the /api/players/{playerKey}/... endpoints take."),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of players to return (default 25, max 500)."),
		),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(playersTool, s.handleListTopPlayers)

	markersTool := mcp.NewTool("list_marker_definitions",
		mcp.WithDescription("List the derived analysis vocabulary: every marker (per-player, per-game summaries such as build-order openers and timing verdicts) and game event (narrative moments such as rushes, drops and proxies) screpdb computes, with its label and feature key. Use this to learn which strategic patterns are queryable."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	mcpServer.AddTool(markersTool, s.handleListMarkerDefinitions)

	return s
}

func (s *Server) Start(ctx context.Context) error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	raw, err := request.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid path parameter: %v", err)), nil
	}
	path, err := resolve(raw)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return s.get(ctx, path)
}

func (s *Server) handleListTopPlayers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	limit := request.GetInt("limit", 25)
	if limit <= 0 || limit > 500 {
		limit = 25
	}
	return s.get(ctx, fmt.Sprintf("/api/players?limit=%d&sort_by=games&sort_dir=desc", limit))
}

func (s *Server) handleListMarkerDefinitions(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.get(ctx, "/api/custom/markers/definitions")
}

func (s *Server) handleGetStarCraftKnowledge(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultText(starcraftKnowledge), nil
}

func (s *Server) handleGetSchema(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var b strings.Builder
	if health, err := s.client.Health(ctx); err == nil {
		fmt.Fprintf(&b, "Corpus: %d replays from %s (screpdb %s, load phase %q, %d/%d read).\n\n",
			health.TotalReplays, health.Library.ReplayDir, health.Version,
			health.Library.Phase, health.Library.Loaded, health.Library.Total)
	}
	b.WriteString(renderSchema())
	b.WriteString(schemaNotes)
	return mcp.NewToolResultText(b.String()), nil
}

func (s *Server) get(ctx context.Context, path string) (*mcp.CallToolResult, error) {
	body, err := s.client.Get(ctx, path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(formatJSON(path, body)), nil
}

// formatJSON indents for readability and truncates an oversized response rather
// than flooding the model's context with it.
func formatJSON(path string, body []byte) string {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err != nil {
		// Not JSON (an error page, say): hand back what the server sent.
		return string(body)
	}
	out := pretty.String()
	header := "GET " + path + "\n\n"
	if len(out) <= maxToolResultBytes {
		return header + out
	}
	// Cut on a rune boundary so the tail is not a broken code point.
	cut := maxToolResultBytes
	for cut > 0 && !utf8.RuneStart(out[cut]) {
		cut--
	}
	return fmt.Sprintf("%s%s\n\n…truncated at %d of %d bytes. Narrow the request (a smaller limit, or a single game or player) to see the rest.",
		header, out[:cut], cut, len(out))
}

// schemaNotes is the curated half of the schema tool: what the endpoints are
// for, and what this API cannot answer.
const schemaNotes = `
How the corpus is shaped

  - A game (replay) holds its players, its production stream (builds, trains,
    morphs, tech, upgrades and composition-relevant spell casts), its derived
    markers and game events, its chat, and its map layout. Every join you might
    want inside a game is already done: /api/games/{replayID} returns the whole
    game report in one response.
  - A player is identified corpus-wide by playerKey, the lower-cased name.
    /api/players lists them with their game counts; the per-player endpoints
    take that key.
  - Markers are one-per-(game, player) verdicts screpdb computed: build-order
    openers (feature keys prefixed bo_, e.g. bo_9_pool, bo_12_hatch,
    bo_gate_expand, bo_t_111), timing milestones, and behaviour flags such as
    never_used_hotkeys or viewport_multitasking. opener_unresolved, *_fuzzy and
    bo_*_other are catch-alls, not real openers.
  - Game events are narrative moments: rushes, drops, proxies, nydus, mind
    control, scouting and expansions, each with a second and a map location.
  - Use list_marker_definitions for the live vocabulary of both.

Choosing an endpoint

  - "Which games match X?"            /api/games with its filters
  - "What happened in this game?"     /api/games/{replayID}
  - "How did they use hotkeys?"       /api/games/{replayID}/hotkeys,
                                      /api/players/{playerKey}/hotkey-signature
  - "Who is in the corpus?"           /api/players (or list_top_players)
  - "How does this player play?"      /api/players/{playerKey},
                                      /api/players/{playerKey}/insight?type=…
  - "Their recent games?"             /api/players/{playerKey}/last-games
  - "What do they say in game?"       /api/players/{playerKey}/chat-summary
  - "How do they compare?"            /api/players/insights/apm-histogram,
                                      /api/players/insights/unit-production-cadence,
                                      /api/players/insights/viewport-multitasking

What this API cannot answer

  - There is no raw command stream. The production stream keeps what was built,
    trained, morphed, researched and upgraded, and when — but not individual
    right-clicks, move orders, unit tags or screen coordinates. Questions about
    micro at that resolution have no answer here.
  - There is no SQL. Filter with the query parameters each endpoint documents,
    then group and aggregate the JSON yourself.
  - The corpus is whatever replay folder the running screpdb read, capped at
    its newest games. It is not all of StarCraft.
`
