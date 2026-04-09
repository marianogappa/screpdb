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

	"github.com/gorilla/mux"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/storage"
)

const dashboardTestDB = "file:dashboard_test?mode=memory&cache=shared"

func TestDashboardAPI_ListAndGet(t *testing.T) {
	dash := newTestDashboard(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/custom/dashboard", nil)
	dash.handlerListDashboards(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list dashboards status %d: %s", rec.Code, rec.Body.String())
	}

	var listResp []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("list dashboards json: %v", err)
	}
	if len(listResp) == 0 || listResp[0].URL != "default" {
		t.Fatalf("expected default dashboard in list")
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/custom/dashboard/default", nil)
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerGetDashboard(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get dashboard status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDashboardAPI_WidgetFlow(t *testing.T) {
	dash := newTestDashboard(t)

	// Create widget without prompt
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/custom/dashboard/default/widget", bytes.NewReader([]byte(`{"Prompt": ""}`)))
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerCreateDashboardWidget(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create widget status %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("create widget json: %v", err)
	}
	if createResp.ID == 0 {
		t.Fatalf("expected widget id")
	}

	// Update widget query
	updateBody := map[string]any{
		"name":        "Replay Count",
		"description": "Count of replays",
		"config": map[string]any{
			"type": "table",
		},
		"query": "SELECT COUNT(*) AS c FROM replays",
	}
	updateJSON, _ := json.Marshal(updateBody)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/custom/dashboard/default/widget", bytes.NewReader(updateJSON))
	req = mux.SetURLVars(req, map[string]string{"url": "default", "wid": fmt.Sprintf("%d", createResp.ID)})
	dash.handlerUpdateDashboardWidget(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update widget status %d: %s", rec.Code, rec.Body.String())
	}

	// Get dashboard — should return widget metadata without results
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/custom/dashboard/default", nil)
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerGetDashboard(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get dashboard status %d: %s", rec.Code, rec.Body.String())
	}

	var getResp struct {
		Widgets []struct {
			Query string `json:"query"`
		} `json:"widgets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("get dashboard json: %v", err)
	}
	if len(getResp.Widgets) != 1 {
		t.Fatalf("expected 1 widget, got %d", len(getResp.Widgets))
	}
	if getResp.Widgets[0].Query != "SELECT COUNT(*) AS c FROM replays" {
		t.Fatalf("expected widget query, got %q", getResp.Widgets[0].Query)
	}

	// Execute widget query via /api/custom/query
	queryBody := []byte(`{"query": "SELECT COUNT(*) AS c FROM replays", "variable_values": {}}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(queryBody))
	dash.handlerExecuteQuery(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute query status %d: %s", rec.Code, rec.Body.String())
	}

	var queryResp struct {
		Results []map[string]any `json:"results"`
		Columns []string         `json:"columns"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queryResp); err != nil {
		t.Fatalf("execute query json: %v", err)
	}
	if len(queryResp.Results) != 1 {
		t.Fatalf("expected 1 result row, got %d", len(queryResp.Results))
	}
	if !containsString(queryResp.Columns, "c") {
		t.Fatalf("expected column c in results")
	}
}

func TestDashboardAPI_ExecuteQuery(t *testing.T) {
	dash := newTestDashboard(t)

	body := []byte(`{"query": "SELECT COUNT(*) AS c FROM replays", "variable_values": {}}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(body))
	dash.handlerExecuteQuery(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute query status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Results []map[string]any `json:"results"`
		Columns []string         `json:"columns"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("execute query json: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result row, got %d", len(resp.Results))
	}
	if !containsString(resp.Columns, "c") {
		t.Fatalf("expected column c in results")
	}
}

func TestDashboardAPI_ReplayFilter(t *testing.T) {
	dash := newTestDashboard(t)

	filterSQL := "SELECT * FROM replays WHERE file_name = 'SomaTyson.rep'"

	updateBody := map[string]any{
		"replays_filter_sql": filterSQL,
	}
	updateJSON, _ := json.Marshal(updateBody)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/custom/dashboard/default", bytes.NewReader(updateJSON))
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerUpdateDashboard(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update dashboard status %d: %s", rec.Code, rec.Body.String())
	}

	// Create widget without prompt
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/custom/dashboard/default/widget", bytes.NewReader([]byte(`{"Prompt": ""}`)))
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerCreateDashboardWidget(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create widget status %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("create widget json: %v", err)
	}
	if createResp.ID == 0 {
		t.Fatalf("expected widget id")
	}

	updateWidgetBody := map[string]any{
		"name":        "Replay Count",
		"description": "Count of replays",
		"config": map[string]any{
			"type": "table",
		},
		"query": "SELECT COUNT(*) AS c FROM replays",
	}
	updateWidgetJSON, _ := json.Marshal(updateWidgetBody)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/custom/dashboard/default/widget", bytes.NewReader(updateWidgetJSON))
	req = mux.SetURLVars(req, map[string]string{"url": "default", "wid": fmt.Sprintf("%d", createResp.ID)})
	dash.handlerUpdateDashboardWidget(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update widget status %d: %s", rec.Code, rec.Body.String())
	}

	// Execute widget query via /api/custom/query with dashboard_url for replay filter
	queryBody := []byte(`{"query": "SELECT COUNT(*) AS c FROM replays", "variable_values": {}, "dashboard_url": "default"}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(queryBody))
	dash.handlerExecuteQuery(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute query status %d: %s", rec.Code, rec.Body.String())
	}

	var queryResp struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &queryResp); err != nil {
		t.Fatalf("execute query json: %v", err)
	}
	if len(queryResp.Results) != 1 {
		t.Fatalf("expected 1 result row, got %d", len(queryResp.Results))
	}
	countVal, ok := queryResp.Results[0]["c"]
	if !ok {
		t.Fatalf("expected count column c")
	}
	var count int64
	switch v := countVal.(type) {
	case float64:
		count = int64(v)
	case int64:
		count = v
	case int:
		count = int64(v)
	default:
		t.Fatalf("unexpected count type: %T", countVal)
	}
	if count != 1 {
		t.Fatalf("expected filtered replay count 1, got %d", count)
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

func TestValidateReplayFilterSQLStoresQualified(t *testing.T) {
	dash := newTestDashboard(t)
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

	qualified, err := dash.validateReplayFilterSQL(&input)
	if err != nil {
		t.Fatalf("validateReplayFilterSQL: %v", err)
	}
	if !strings.Contains(qualified, "FROM main.replays r") {
		t.Fatalf("expected qualified SQL, got: %s", qualified)
	}
	if !strings.Contains(qualified, "FROM main.players p") {
		t.Fatalf("expected qualified SQL, got: %s", qualified)
	}
}

func TestDashboardAPI_WorkflowPlayerChatSummary(t *testing.T) {
	dash := newTestDashboard(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/players/soma/chat-summary", nil)
	req = mux.SetURLVars(req, map[string]string{"playerKey": "soma"})
	dash.handlerPlayerChatSummary(rec, req)
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
	replayDir, err := resolveReplayDir()
	if err != nil {
		t.Fatalf("resolveReplayDir: %v", err)
	}

	body := []byte(fmt.Sprintf(`{"input_dir":%q}`, replayDir))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/custom/ingest/settings", bytes.NewReader(body))
	dash.handlerUpdateIngestSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update ingest settings status %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/custom/ingest/settings", nil)
	dash.handlerGetIngestSettings(rec, req)
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
	emptyDir := t.TempDir()

	body := []byte(fmt.Sprintf(`{"input_dir":%q}`, emptyDir))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/custom/ingest/settings", bytes.NewReader(body))
	dash.handlerUpdateIngestSettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "does not contain any .rep files") {
		t.Fatalf("expected missing replay files error, got %q", rec.Body.String())
	}
}

func TestDashboardAPI_IngestUsesStoredInputDir(t *testing.T) {
	dash := newTestDashboard(t)
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

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/custom/ingest", bytes.NewReader([]byte(`{}`)))
	dash.handlerIngest(rec, req)
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

	dash, err := New(ctx, store, dashboardTestDB, "", "", "")
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
			return dir, nil
		}
	}
	return "", os.ErrNotExist
}

func containsString(list []string, value string) bool {
	return slices.Contains(list, value)
}
