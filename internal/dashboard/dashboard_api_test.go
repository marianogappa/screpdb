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
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard", nil)
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
	req = httptest.NewRequest(http.MethodGet, "/api/dashboard/default", nil)
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
	req := httptest.NewRequest(http.MethodPut, "/api/dashboard/default/widget", bytes.NewReader([]byte(`{"Prompt": ""}`)))
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
	req = httptest.NewRequest(http.MethodPost, "/api/dashboard/default/widget", bytes.NewReader(updateJSON))
	req = mux.SetURLVars(req, map[string]string{"url": "default", "wid": fmt.Sprintf("%d", createResp.ID)})
	dash.handlerUpdateDashboardWidget(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update widget status %d: %s", rec.Code, rec.Body.String())
	}

	// Get dashboard with results
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/dashboard/default", nil)
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerGetDashboard(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get dashboard status %d: %s", rec.Code, rec.Body.String())
	}

	var getResp struct {
		Widgets []struct {
			Results []map[string]any `json:"results"`
			Columns []string         `json:"columns"`
		} `json:"widgets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("get dashboard json: %v", err)
	}
	if len(getResp.Widgets) != 1 {
		t.Fatalf("expected 1 widget, got %d", len(getResp.Widgets))
	}
	if len(getResp.Widgets[0].Results) != 1 {
		t.Fatalf("expected 1 result row, got %d", len(getResp.Widgets[0].Results))
	}
	if !containsString(getResp.Widgets[0].Columns, "c") {
		t.Fatalf("expected column c in results")
	}
}

func TestDashboardAPI_ExecuteQuery(t *testing.T) {
	dash := newTestDashboard(t)

	body := []byte(`{"query": "SELECT COUNT(*) AS c FROM replays", "variable_values": {}}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/query", bytes.NewReader(body))
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

	dash, err := New(ctx, store, dashboardTestDB, "")
	if err != nil {
		t.Fatalf("New dashboard: %v", err)
	}
	t.Cleanup(func() {
		_ = dash.db.Close()
	})
	return dash
}

func ingestFiles(ctx context.Context, store *storage.SQLiteStorage, files []fileops.FileInfo) error {
	dataChan, errChan := store.StartIngestion(ctx)
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
