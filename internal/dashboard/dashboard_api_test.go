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

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/storage"
)

const dashboardTestDB = "file:dashboard_test?mode=memory&cache=shared"

func TestDashboardAPI_AliasCRUDAndImport(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()

	upsertBody := []byte(`{"canonical_alias":"ManualAlias","battle_tag":"ManualTag","source":"manual"}`)
	rec := performDashboardRequest(router, http.MethodPut, "/api/custom/aliases/entry", upsertBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("upsert alias status %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/custom/aliases", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list aliases status %d: %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Aliases []struct {
			ID             int64  `json:"id"`
			CanonicalAlias string `json:"canonical_alias"`
			BattleTagRaw   string `json:"battle_tag_raw"`
			Source         string `json:"source"`
		} `json:"aliases"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("list aliases json: %v", err)
	}
	var createdID int64
	for _, row := range listResp.Aliases {
		if row.CanonicalAlias == "ManualAlias" && row.BattleTagRaw == "ManualTag" && row.Source == "manual" {
			createdID = row.ID
			break
		}
	}
	if createdID == 0 {
		t.Fatalf("expected to find inserted manual alias")
	}

	rec = performDashboardRequest(router, http.MethodDelete, fmt.Sprintf("/api/custom/aliases/%d", createdID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete alias status %d: %s", rec.Code, rec.Body.String())
	}

	importBody := []byte(`{"aliases":{"ImportedAlias":[{"battle_tag":"ImportedTag","aurora_id":42}]}}`)
	rec = performDashboardRequest(router, http.MethodPut, "/api/custom/aliases", importBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("import aliases status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestQualifyReplayFilterSQLMultiline(t *testing.T) {
	input := `SELECT *
FROM replays r
WHERE EXISTS (
  SELECT 1
  FROM players p
  WHERE p.replay_id = r.id
    AND p.type = 'Human'
  GROUP BY p.replay_id
  HAVING COUNT(*) = 2
)`

	qualified := qualifyReplayFilterSQL(input)
	if !strings.Contains(qualified, "FROM main.replays r") {
		t.Fatalf("expected main.replays qualification, got: %s", qualified)
	}
	if strings.Contains(qualified, "FROM replays r") {
		t.Fatalf("expected unqualified replays to be rewritten, got: %s", qualified)
	}
	if !strings.Contains(qualified, "FROM main.players p") {
		t.Fatalf("expected main.players qualification, got: %s", qualified)
	}
}

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

func TestDashboardAPI_IngestSettingsUpdateAndGet(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	replayDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}

	body := []byte(fmt.Sprintf(`{"input_dir":%q}`, replayDir))
	rec := performDashboardRequest(router, http.MethodPut, "/api/custom/ingest/settings", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("update ingest settings status %d: %s", rec.Code, rec.Body.String())
	}

	rec = performDashboardRequest(router, http.MethodGet, "/api/custom/ingest/settings", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get ingest settings status %d: %s", rec.Code, rec.Body.String())
	}

	var resp ingestSettingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("get ingest settings json: %v", err)
	}
	if resp.InputDir != replayDir {
		t.Fatalf("expected input dir %q, got %q", replayDir, resp.InputDir)
	}
}

func TestDashboardAPI_IngestSettingsRejectsFolderWithoutReplays(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	emptyDir := t.TempDir()

	body := []byte(fmt.Sprintf(`{"input_dir":%q}`, emptyDir))
	rec := performDashboardRequest(router, http.MethodPut, "/api/custom/ingest/settings", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "does not contain any .rep files") {
		t.Fatalf("expected missing replay files error, got %q", rec.Body.String())
	}
}

func TestDashboardAPI_IngestUsesStoredInputDir(t *testing.T) {
	dash := newTestDashboard(t)
	router := dash.setupRouter()
	replayDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	if err := dash.setIngestInputDir(context.Background(), replayDir); err != nil {
		t.Fatalf("setIngestInputDir: %v", err)
	}

	dash.ingestMu.Lock()
	dash.ingestRunning = true
	dash.ingestMu.Unlock()

	rec := performDashboardRequest(router, http.MethodPost, "/api/custom/ingest", []byte(`{}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("ingest status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Started    bool   `json:"started"`
		InProgress bool   `json:"in_progress"`
		InputDir   string `json:"input_dir"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("ingest json: %v", err)
	}
	if resp.Started {
		t.Fatalf("expected ingest not to start while already running")
	}
	if !resp.InProgress {
		t.Fatalf("expected ingest response to indicate in progress")
	}
	if resp.InputDir != replayDir {
		t.Fatalf("expected stored input dir %q, got %q", replayDir, resp.InputDir)
	}
}

func newTestDashboard(t *testing.T) *Dashboard {
	t.Helper()
	ctx := context.Background()

	store, err := storage.NewSQLiteStorage(dashboardTestDB)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})

	if err := store.Initialize(ctx, true, true); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	replayDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}
	files, err := fileops.GetReplayFiles(replayDir)
	if err != nil {
		t.Fatalf("GetReplayFiles: %v", err)
	}
	if err := ingestFiles(ctx, store, files); err != nil {
		t.Fatalf("ingestFiles: %v", err)
	}

	dash, err := New(ctx, dashboardTestDB)
	if err != nil {
		t.Fatalf("New dashboard: %v", err)
	}
	t.Cleanup(func() {
		_ = dash.db.Close()
		if dash.replayScopedDB != nil {
			_ = dash.replayScopedDB.Close()
		}
	})
	return dash
}

func ingestFiles(ctx context.Context, store *storage.SQLiteStorage, files []fileops.FileInfo) error {
	dataChan, errChan := store.StartIngestion(ctx, storage.IngestionHooks{})
	for i := range files {
		fi := files[i]
		replay := parser.CreateReplayFromFileInfo(fi.Path, fi.Name, fi.Size, fi.Checksum)
		data, err := parser.ParseReplay(fi.Path, replay)
		if err != nil {
			return err
		}
		dataChan <- data
	}
	close(dataChan)
	return <-errChan
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
