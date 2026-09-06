package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mark3labs/mcp-go/mcp"
)

// fakeDashboard is a stand-in for a running screpdb: it answers the handful of
// API paths the tools reach, and records what was asked for.
type fakeDashboard struct {
	*httptest.Server
	requests []string
	status   int
	body     string
}

func newFakeDashboard(t *testing.T) *fakeDashboard {
	t.Helper()
	f := &fakeDashboard{status: http.StatusOK}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r.URL.String())
		if r.URL.Path == "/api/health" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"app":"screpdb","total_replays":42,"version":"test",
				"library":{"phase":"ready","loaded":42,"total":42,"complete":true,"replay_dir":"/replays"}}`)
			return
		}
		w.WriteHeader(f.status)
		if f.body != "" {
			fmt.Fprint(w, f.body)
			return
		}
		fmt.Fprint(w, `{"rows":[{"name":"flash"}]}`)
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeDashboard) addr() string { return strings.TrimPrefix(f.URL, "http://") }

func (f *fakeDashboard) server(t *testing.T) *Server {
	t.Helper()
	return NewServer(NewClient(f.addr()))
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

func queryReq(path string) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Name = "query_replay_api"
	if path != "" {
		req.Params.Arguments = map[string]any{"path": path}
	}
	return req
}

func TestResolveAcceptsExposedPaths(t *testing.T) {
	for _, path := range []string{
		"/api/games",
		"/api/games?limit=20&matchup=TvZ",
		"/api/games/17",
		"/api/games/17/hotkeys",
		"/api/players",
		"/api/players?sort_by=games&sort_dir=desc",
		"/api/players/flash",
		"/api/players/flash/last-games",
		"/api/players/flash/chat-summary",
		"/api/players/flash/insight?type=build_orders",
		"/api/players/flash/hotkey-signature",
		"/api/players/flash/insights/apm-histogram",
		"/api/players/flash/insights/unit-production-cadence?filter=all",
		"/api/players/insights/apm-histogram",
		"/api/players/insights/unit-production-cadence?min_games=5",
		"/api/players/insights/viewport-multitasking",
		"/api/custom/markers/definitions",
		"/api/health",
		"api/games",
	} {
		if _, err := resolve(path); err != nil {
			t.Errorf("resolve(%q) = %v, want it allowed", path, err)
		}
	}
}

func TestResolveRejectsEverythingElse(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"", "path is required"},
		{"http://evil.example/api/games", "pass a path, not a URL"},
		{"/api/games/17/see", "not an exposed endpoint"},
		{"/api/custom/ingest", "not an exposed endpoint"},
		{"/api/custom/update/apply", "not an exposed endpoint"},
		{"/api/custom/bnet/profile", "not an exposed endpoint"},
		{"/api/custom/sample-set/load", "not an exposed endpoint"},
		{"/api/player-colors", "not an exposed endpoint"},
		{"/api/custom/pros/1/photo", "not an exposed endpoint"},
		{"/api/games?sql=DROP", "does not accept sql"},
		{"/api/health?verbose=1", "takes no query parameters"},
	}
	for _, tt := range tests {
		_, err := resolve(tt.path)
		if err == nil {
			t.Errorf("resolve(%q) was allowed; want rejected", tt.path)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("resolve(%q) = %q, want it to mention %q", tt.path, err, tt.want)
		}
	}
}

// TestExposedSurfaceMatchesTheSpec keeps the allowlist honest against the
// source-of-truth OpenAPI document: every exposed path must exist there as a
// GET with exactly the query parameters listed here, and the mutating,
// UI-only and game-launching operations must stay out.
func TestExposedSurfaceMatchesTheSpec(t *testing.T) {
	specPath := filepath.Join("..", "..", "api", "openapi", "dashboard.v1.yaml")
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("load %s: %v", specPath, err)
	}

	inSpec := map[string]*openapi3.PathItem{}
	for template, item := range spec.Paths.Map() {
		inSpec[template] = item
	}

	for _, e := range exposed {
		item, ok := inSpec[e.template]
		if !ok {
			t.Errorf("%s is exposed but the spec has no such path", e.template)
			continue
		}
		if item.Get == nil {
			t.Errorf("%s is exposed but the spec has no GET for it", e.template)
			continue
		}
		var wantQuery []string
		for _, ref := range append(append(openapi3.Parameters{}, item.Parameters...), item.Get.Parameters...) {
			if ref.Value != nil && ref.Value.In == openapi3.ParameterInQuery {
				wantQuery = append(wantQuery, ref.Value.Name)
			}
		}
		sort.Strings(wantQuery)
		if strings.Join(wantQuery, ",") != strings.Join(e.queryParams, ",") {
			t.Errorf("%s query params = %v, spec says %v", e.template, e.queryParams, wantQuery)
		}
	}

	exposedPaths := map[string]bool{}
	for _, e := range exposed {
		exposedPaths[e.template] = true
	}
	for _, mustNotBeExposed := range []string{
		"/api/games/{replayID}/see",
		"/api/custom/sample-set/load",
		"/api/custom/update/apply",
		"/api/custom/bnet/profile",
		"/api/custom/pros/{proID}/photo",
		"/api/custom/global-replay-filter",
		"/api/custom/library/settings",
		"/api/custom/debug/map-layout/{replayID}",
	} {
		if _, ok := inSpec[mustNotBeExposed]; !ok {
			t.Errorf("the spec no longer has %s; this guard is checking nothing", mustNotBeExposed)
		}
		if exposedPaths[mustNotBeExposed] {
			t.Errorf("%s must never be reachable from MCP", mustNotBeExposed)
		}
	}
}

func TestHandleQuerySuccess(t *testing.T) {
	fake := newFakeDashboard(t)
	res, err := fake.server(t).handleQuery(context.Background(), queryReq("/api/games?limit=2"))
	if err != nil {
		t.Fatalf("handleQuery: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error result: %s", textOf(t, res))
	}
	out := textOf(t, res)
	if !strings.Contains(out, "GET /api/games?limit=2") || !strings.Contains(out, "flash") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestHandleQueryMissingParam(t *testing.T) {
	fake := newFakeDashboard(t)
	res, err := fake.server(t).handleQuery(context.Background(), queryReq(""))
	if err != nil {
		t.Fatalf("handleQuery: %v", err)
	}
	if !res.IsError || !strings.Contains(textOf(t, res), "Invalid path parameter") {
		t.Fatalf("unexpected result: %s", textOf(t, res))
	}
}

func TestHandleQueryRejectsUnexposedPath(t *testing.T) {
	fake := newFakeDashboard(t)
	res, err := fake.server(t).handleQuery(context.Background(), queryReq("/api/games/1/see"))
	if err != nil {
		t.Fatalf("handleQuery: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected /api/games/1/see to be refused")
	}
	if len(fake.requests) != 0 {
		t.Fatalf("a refused path still reached the server: %v", fake.requests)
	}
}

func TestHandleQuerySurfacesServerErrors(t *testing.T) {
	fake := newFakeDashboard(t)
	fake.status = http.StatusNotFound
	fake.body = `{"error":"no such game"}`
	res, err := fake.server(t).handleQuery(context.Background(), queryReq("/api/games/999"))
	if err != nil {
		t.Fatalf("handleQuery: %v", err)
	}
	if !res.IsError || !strings.Contains(textOf(t, res), "404") {
		t.Fatalf("unexpected result: %s", textOf(t, res))
	}
}

func TestHandleGetSchema(t *testing.T) {
	fake := newFakeDashboard(t)
	res, err := fake.server(t).handleGetSchema(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleGetSchema: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	out := textOf(t, res)
	for _, want := range []string{
		"42 replays from /replays",
		"GET /api/games",
		"GET /api/players/{playerKey}/insight",
		"query: duration, featuring, limit, map, map_kind, matchup, offset, player",
		"There is no raw command stream",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("schema is missing %q", want)
		}
	}
	if strings.Contains(out, "/see") || strings.Contains(out, "SQL") && !strings.Contains(out, "There is no SQL") {
		t.Errorf("schema advertises something it should not:\n%s", out)
	}
}

func TestHandleListTopPlayers(t *testing.T) {
	fake := newFakeDashboard(t)
	s := fake.server(t)
	if _, err := s.handleListTopPlayers(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("handleListTopPlayers: %v", err)
	}
	var req mcp.CallToolRequest
	req.Params.Arguments = map[string]any{"limit": float64(5)}
	if _, err := s.handleListTopPlayers(context.Background(), req); err != nil {
		t.Fatalf("handleListTopPlayers(limit): %v", err)
	}
	want := []string{
		"/api/players?limit=25&sort_by=games&sort_dir=desc",
		"/api/players?limit=5&sort_by=games&sort_dir=desc",
	}
	if len(fake.requests) != 2 || fake.requests[0] != want[0] || fake.requests[1] != want[1] {
		t.Fatalf("requests = %v, want %v", fake.requests, want)
	}
}

func TestHandleListMarkerDefinitions(t *testing.T) {
	fake := newFakeDashboard(t)
	if _, err := fake.server(t).handleListMarkerDefinitions(context.Background(), mcp.CallToolRequest{}); err != nil {
		t.Fatalf("handleListMarkerDefinitions: %v", err)
	}
	if len(fake.requests) != 1 || fake.requests[0] != "/api/custom/markers/definitions" {
		t.Fatalf("requests = %v", fake.requests)
	}
}

func TestHandleGetStarCraftKnowledge(t *testing.T) {
	fake := newFakeDashboard(t)
	res, err := fake.server(t).handleGetStarCraftKnowledge(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleGetStarCraftKnowledge: %v", err)
	}
	out := textOf(t, res)
	if strings.TrimSpace(out) == "" {
		t.Fatal("knowledge text is empty")
	}
	// The knowledge must describe the library model, not the retired schema.
	for _, gone := range []string{"replay_events table", "commands_low_value", "players.hotkey_stream", "JOIN"} {
		if strings.Contains(out, gone) {
			t.Errorf("knowledge still describes the SQL schema: %q", gone)
		}
	}
}

func TestClientHealth(t *testing.T) {
	fake := newFakeDashboard(t)
	health, err := NewClient(fake.addr()).Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.App != "screpdb" || health.TotalReplays != 42 || !health.Library.Complete {
		t.Fatalf("health = %+v", health)
	}
}

func TestClientRefusesRemoteHosts(t *testing.T) {
	_, err := NewClient("example.com:80").Get(context.Background(), "/api/games")
	if err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("err = %v, want a loopback refusal", err)
	}
}

func TestFormatJSONIndentsAndPassesThroughNonJSON(t *testing.T) {
	out := formatJSON("/api/games", []byte(`{"a":1}`))
	if !strings.Contains(out, "GET /api/games") || !strings.Contains(out, "\n  \"a\": 1") {
		t.Fatalf("unexpected output: %q", out)
	}
	if got := formatJSON("/api/games", []byte("not json")); got != "not json" {
		t.Fatalf("non-JSON body = %q", got)
	}
}

func TestFormatJSONTruncatesOversizedResponses(t *testing.T) {
	big := make([]string, 0, 40000)
	for i := range cap(big) {
		big = append(big, fmt.Sprintf("row-%d-padding-padding-padding", i))
	}
	body, err := json.Marshal(big)
	if err != nil {
		t.Fatal(err)
	}
	out := formatJSON("/api/games", body)
	if !strings.Contains(out, "truncated at") {
		t.Fatal("an oversized response was not truncated")
	}
	if len(out) > maxToolResultBytes+1024 {
		t.Fatalf("truncated output is still %d bytes", len(out))
	}
}

func TestConnectAttachesToARunningServer(t *testing.T) {
	fake := newFakeDashboard(t)
	port := portOf(t, fake.addr())
	client, stop, err := Connect(context.Background(), ConnectOptions{Port: port})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer stop()
	if client.Addr() != fmt.Sprintf("localhost:%d", port) {
		t.Fatalf("attached to %q, want the fake dashboard's port", client.Addr())
	}
}

func TestConnectWithoutAutoStartExplainsHowToStartOne(t *testing.T) {
	// A port range nothing is listening on.
	_, _, err := Connect(context.Background(), ConnectOptions{Port: 59123, AutoStart: false})
	if err == nil {
		t.Fatal("expected Connect to fail with no server running")
	}
	for _, want := range []string{"no screpdb server found", "screpdb dashboard --headless"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func portOf(t *testing.T, addr string) int {
	t.Helper()
	var port int
	if _, err := fmt.Sscanf(addr[strings.LastIndex(addr, ":")+1:], "%d", &port); err != nil {
		t.Fatalf("parse port from %q: %v", addr, err)
	}
	return port
}

// The tests below stand a helper process in for the real binary so the spawn,
// wait and shutdown path is exercised end to end. The helper is this test
// binary re-invoked with fakeServerEnv set; go test runs it as an ordinary
// test, which TestFakeScrepdbServer turns into a tiny /api/health server.
const (
	fakeServerEnv  = "SCREPDB_MCP_TEST_FAKE_SERVER"
	fakeServerPort = "SCREPDB_MCP_TEST_FAKE_PORT"
)

// TestFakeScrepdbServer is the helper process, not a test of its own. Modes:
// "ready" serves a complete corpus, "loading" serves one that never finishes,
// "crash" exits immediately.
func TestFakeScrepdbServer(t *testing.T) {
	mode := os.Getenv(fakeServerEnv)
	if mode == "" {
		t.Skip("helper process; only runs when re-invoked by another test")
	}
	if mode == "crash" {
		os.Exit(3)
	}
	complete := mode == "ready"
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"app":"screpdb","total_replays":7,"version":"test",
			"library":{"phase":"recent","loaded":3,"total":7,"complete":%t,"replay_dir":"/replays"}}`, complete)
	})
	srv := &http.Server{Addr: "localhost:" + os.Getenv(fakeServerPort), Handler: mux}
	go func() { _ = srv.ListenAndServe() }()
	// Outlive the parent's wait; the parent kills this process when it is done.
	time.Sleep(2 * time.Minute)
}

// withFakeServer points execCommand at the helper process in the given mode.
func withFakeServer(t *testing.T, mode string) {
	t.Helper()
	previous := execCommand
	execCommand = func(_ string, args ...string) *exec.Cmd {
		port := ""
		for i, a := range args {
			if a == "-p" && i+1 < len(args) {
				port = args[i+1]
			}
		}
		cmd := exec.Command(os.Args[0], "-test.run=TestFakeScrepdbServer", "-test.timeout=5m")
		cmd.Env = append(os.Environ(), fakeServerEnv+"="+mode, fakeServerPort+"="+port)
		return cmd
	}
	t.Cleanup(func() { execCommand = previous })
}

func TestConnectAutoStartsAServerAndStopsIt(t *testing.T) {
	withFakeServer(t, "ready")
	client, stop, err := Connect(context.Background(), ConnectOptions{
		Port: 59200, AutoStart: true, ReplayDir: "/replays", WaitForCorpus: true,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.TotalReplays != 7 {
		t.Fatalf("health = %+v, want the started server's corpus", health)
	}
	// stop must return: it kills the child and reaps it exactly once.
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("stop did not return; the child was not reaped")
	}
	if _, err := client.Health(context.Background()); err == nil {
		t.Fatal("the started server is still answering after stop")
	}
}

func TestConnectFailsWhenTheStartedServerDies(t *testing.T) {
	withFakeServer(t, "crash")
	start := time.Now()
	_, _, err := Connect(context.Background(), ConnectOptions{Port: 59220, AutoStart: true})
	if err == nil {
		t.Fatal("expected Connect to fail when the child exits")
	}
	if !strings.Contains(err.Error(), "exited first") {
		t.Errorf("error %q should say the child exited first", err)
	}
	// It must notice the exit rather than burning the whole listen budget.
	if elapsed := time.Since(start); elapsed > listenTimeout/2 {
		t.Errorf("took %s to notice a child that exited immediately", elapsed)
	}
}

func TestConnectCanSkipWaitingForTheCorpus(t *testing.T) {
	withFakeServer(t, "loading")
	client, stop, err := Connect(context.Background(), ConnectOptions{
		Port: 59240, AutoStart: true, WaitForCorpus: false,
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer stop()
	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if health.Library.Complete {
		t.Fatal("the fake server should still be loading")
	}
}
