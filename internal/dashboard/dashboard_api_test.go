package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

func TestDashboardAPI_WorkflowPlayerChatSummary(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/players/soma/chat-summary", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("workflow player chat summary status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		PlayerKey      string `json:"player_key"`
		SummaryVersion string `json:"summary_version"`
		ChatSummary    struct {
			TotalMessages int64 `json:"total_messages"`
		} `json:"chat_summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("workflow player chat summary json: %v", err)
	}
	if resp.PlayerKey != "soma" {
		t.Fatalf("expected player key soma, got %q", resp.PlayerKey)
	}
	if resp.SummaryVersion == "" {
		t.Fatalf("expected summary version")
	}
	if resp.ChatSummary.TotalMessages < 0 {
		t.Fatalf("expected non-negative total messages, got %d", resp.ChatSummary.TotalMessages)
	}
}

func TestDashboardAPI_LibrarySettingsUpdateAndGet(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	replayDir := testCorpusDir(t)

	body := []byte(fmt.Sprintf(`{"input_dir":%q}`, replayDir))
	rec := performDashboardRequest(router, http.MethodPut, "/api/custom/ingest/settings", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("update library settings status %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/custom/ingest/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get library settings status %d: %s", rec.Code, rec.Body.String())
	}
	var resp librarySettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("get library settings json: %v", err)
	}
	if resp.ReplayDir != replayDir {
		t.Fatalf("expected replay dir %q, got %q", replayDir, resp.ReplayDir)
	}
	if resp.IsSampleSet {
		t.Fatal("a corpus folder is not the example replays")
	}
}

func TestDashboardAPI_LibrarySettingsRejectsFolderWithoutReplays(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	body := []byte(fmt.Sprintf(`{"input_dir":%q}`, t.TempDir()))
	rec := performDashboardRequest(router, http.MethodPut, "/api/custom/ingest/settings", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "does not contain any .rep files") {
		t.Fatalf("expected missing replay files error, got %q", rec.Body.String())
	}
}

func TestDashboardAPI_HealthReportsTheLibraryState(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		TotalReplays int64 `json:"total_replays"`
		Library      struct {
			Status    string `json:"status"`
			Loaded    int    `json:"loaded"`
			Total     int    `json:"total"`
			Complete  bool   `json:"complete"`
			ReplayDir string `json:"replay_dir"`
		} `json:"library"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("health json: %v", err)
	}
	if resp.TotalReplays != 4 {
		t.Fatalf("total replays = %d, want the 4 committed ones", resp.TotalReplays)
	}
	if resp.Library.Status != libraryStatusWatching || !resp.Library.Complete {
		t.Fatalf("library = %+v, want a finished load", resp.Library)
	}
	if resp.Library.Loaded != 4 || resp.Library.Total != 4 || resp.Library.ReplayDir != dash.ReplayDir() {
		t.Fatalf("library = %+v", resp.Library)
	}
}

func TestDashboardAPI_StaleReplaysAreGone(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	rec := performDashboardRequest(router, http.MethodGet, "/api/custom/replays/stale-count", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stale count status %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Count int64 `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("stale count json: %v", err)
	}
	if resp.Count != 0 {
		t.Fatalf("nothing can be stale any more, got %d", resp.Count)
	}
}

// newTestDashboard builds a dashboard over the committed replay corpus. The
// corpus is copied into a temp folder per test so a test that changes the
// folder, or drops a replay into it, cannot disturb the checked-in files.
func newTestDashboard(t *testing.T) *Dashboard {
	t.Helper()
	t.Cleanup(iofacade.Reset)
	ctx := context.Background()

	dash, err := New(ctx, Options{Root: t.TempDir(), ReplayDir: testCorpusDir(t), Headless: false})
	if err != nil {
		t.Fatalf("New dashboard: %v", err)
	}
	t.Cleanup(dash.Close)
	if err := dash.StartLibrary(); err != nil {
		t.Fatalf("StartLibrary: %v", err)
	}
	waitForTestCorpus(t, dash)
	return dash
}

// newTestDashboardWithReplays builds a dashboard over a hand-made corpus, for
// the cases the committed replays cannot express (a player with enough games
// of one race, say). It has no replay folder, so anything that reads one is
// unavailable.
func newTestDashboardWithReplays(t *testing.T, replays ...*library.Replay) *Dashboard {
	t.Helper()
	lib := library.New(library.Options{CoalesceRecords: 1, CoalesceDelay: time.Millisecond})
	t.Cleanup(lib.Close)
	lib.Reset(1)
	lib.Add(1, replays...)
	lib.Flush()

	root := t.TempDir()
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	settings, err := dashboarddb.NewFileSettings(root, lib)
	if err != nil {
		t.Fatalf("NewFileSettings: %v", err)
	}
	dash := &Dashboard{ctx: context.Background(), libraryHub: newLibraryHub()}
	dash.dbStore = dashboarddb.NewLibStore(lib, persist.NewBnetCache(root), settings)
	return dash
}

// testCorpusDir copies the committed replays into a folder of their own. The
// copy is absolute, which the scan requires, and writable, which the folder
// change and watcher tests need.
func testCorpusDir(t *testing.T) string {
	t.Helper()
	source, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	dir := t.TempDir()
	copied := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".rep" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("write %s: %v", entry.Name(), err)
		}
		copied++
	}
	if copied == 0 {
		t.Skip("no replay files in the committed corpus")
	}
	return dir
}

func waitForTestCorpus(t *testing.T, dash *Dashboard) {
	t.Helper()
	deadline := time.After(60 * time.Second)
	for {
		if dash.libraryHub.Progress().Complete() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("the replay library never finished loading (phase %q)", dash.libraryHub.Progress().Phase)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func resolveReplayDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	baseDir := filepath.Dir(thisFile)
	candidates := []string{
		filepath.Join(baseDir, "..", "testdata", "replays"),
		filepath.Join(baseDir, "..", "..", "testutils", "replays"),
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			// Register the testdata replays folder as a permitted iofacade root:
			// other tests in this package may have flipped the facade into
			// enforcing mode by registering their own roots (global state).
			_ = iofacade.AllowDir(dir)
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}

func containsString(list []string, value string) bool {
	return slices.Contains(list, value)
}

func performDashboardRequest(router http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader([]byte{})
	} else {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
