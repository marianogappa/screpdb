package mcp

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/storage"
	"github.com/mark3labs/mcp-go/mcp"
)

func newTestStore(t *testing.T) *storage.SQLiteStorage {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := storage.NewSQLiteStorage(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Initialize(context.Background(), true, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return store
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil {
		t.Fatal("nil result")
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func callReq(sql string) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Name = "query_database"
	if sql != "" {
		req.Params.Arguments = map[string]any{"sql": sql}
	}
	return req
}

func TestNewServer_RegistersServer(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.storage == nil || s.mcpServer == nil {
		t.Fatal("server fields not populated")
	}
}

func TestHandleSQLQuery_Success(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)
	ctx := context.Background()

	res, err := s.handleSQLQuery(ctx, callReq("SELECT COUNT(*) AS n FROM replays"))
	if err != nil {
		t.Fatalf("handleSQLQuery: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", textOf(t, res))
	}
	out := textOf(t, res)
	if !strings.Contains(out, "Query Results:") || !strings.Contains(out, "Total rows:") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestHandleSQLQuery_MissingParam(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	res, err := s.handleSQLQuery(context.Background(), callReq(""))
	if err != nil {
		t.Fatalf("handleSQLQuery: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing sql param")
	}
	if !strings.Contains(textOf(t, res), "Invalid sql parameter") {
		t.Fatalf("unexpected message: %s", textOf(t, res))
	}
}

func TestHandleSQLQuery_BadSQL(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	res, err := s.handleSQLQuery(context.Background(), callReq("SELECT * FROM no_such_table"))
	if err != nil {
		t.Fatalf("handleSQLQuery: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for bad sql")
	}
	if !strings.Contains(textOf(t, res), "Query execution failed") {
		t.Fatalf("unexpected message: %s", textOf(t, res))
	}
}

func TestHandleSQLQuery_RejectsWrites(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	for _, sql := range []string{
		"DELETE FROM replays",
		"DROP TABLE players",
		"UPDATE players SET name = 'x'",
		"INSERT INTO replays (id) VALUES (1)",
		"SELECT 1; DROP TABLE players",
		"/* sneaky */ DROP TABLE players",
	} {
		res, err := s.handleSQLQuery(context.Background(), callReq(sql))
		if err != nil {
			t.Fatalf("handleSQLQuery(%q): %v", sql, err)
		}
		if !res.IsError {
			t.Fatalf("expected write %q to be rejected, got: %s", sql, textOf(t, res))
		}
	}

	// Sanity: read-only statements still pass the guard.
	for _, sql := range []string{
		"SELECT COUNT(*) FROM replays",
		"  with x as (select 1) select * from x",
		"EXPLAIN QUERY PLAN SELECT * FROM replays",
		"SELECT 1;",
	} {
		if err := ensureReadOnly(sql); err != nil {
			t.Fatalf("expected %q to be allowed: %v", sql, err)
		}
	}
}

func TestHandleGetSchema(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	res, err := s.handleGetSchema(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleGetSchema: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	out := textOf(t, res)
	if !strings.Contains(out, "JOIN patterns") || !strings.Contains(out, "replays") {
		t.Fatalf("schema observations missing: %q", out[:min(200, len(out))])
	}
	// The modern derived-analysis tables must be introspected too.
	if !strings.Contains(out, "replay_events") || !strings.Contains(out, "player_aliases") {
		t.Fatalf("schema missing modern tables (replay_events/player_aliases)")
	}
}

func TestHandleListTopPlayers(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	res, err := s.handleListTopPlayers(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleListTopPlayers: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
}

func TestHandleListEventTypes(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	res, err := s.handleListEventTypes(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleListEventTypes: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
}

func TestHandleGetStarCraftKnowledge(t *testing.T) {
	store := newTestStore(t)
	s := NewServer(store)

	res, err := s.handleGetStarCraftKnowledge(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleGetStarCraftKnowledge: %v", err)
	}
	if res.IsError {
		t.Fatal("unexpected error result")
	}
	if strings.TrimSpace(textOf(t, res)) == "" {
		t.Fatal("knowledge text is empty")
	}
}

func TestFormatQueryResults_Empty(t *testing.T) {
	s := &Server{}
	if got := s.formatQueryResults(nil); got != "No results found." {
		t.Fatalf("got %q", got)
	}
	if got := s.formatQueryResults([]map[string]any{}); got != "No results found." {
		t.Fatalf("got %q", got)
	}
}

func TestFormatQueryResults_TableShape(t *testing.T) {
	s := &Server{}
	rows := []map[string]any{
		{"name": "alice", "wins": int64(3)},
		{"name": "bob", "wins": int64(1)},
	}
	out := s.formatQueryResults(rows)
	if !strings.Contains(out, "Query Results:") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, " | ") {
		t.Fatalf("missing column separator: %q", out)
	}
	if !strings.Contains(out, "alice") || !strings.Contains(out, "bob") {
		t.Fatalf("missing data rows: %q", out)
	}
	if !strings.Contains(out, "Total rows: 2") {
		t.Fatalf("missing total: %q", out)
	}
	// Separator line uses dashes sized to the header cells.
	if !strings.Contains(out, "----") {
		t.Fatalf("missing dash separator: %q", out)
	}
}
