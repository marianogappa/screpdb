package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/storage"
)

const aiTestDB = "file:ai_integration_test?mode=memory&cache=shared"

func newTestDashboardWithAI(t *testing.T, vendor, apiKey, model string) *Dashboard {
	t.Helper()
	ctx := context.Background()

	store, err := storage.NewSQLiteStorage(aiTestDB)
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

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

	dash, err := New(ctx, store, aiTestDB, vendor, apiKey, model)
	if err != nil {
		t.Fatalf("New dashboard: %v", err)
	}
	t.Cleanup(func() { _ = dash.db.Close() })
	return dash
}

func TestGeminiIntegration_CreateWidgetWithPrompt(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini integration test")
	}

	dash := newTestDashboardWithAI(t, AIVendorGemini, apiKey, "")

	body, _ := json.Marshal(map[string]string{"Prompt": "Show me the total number of replays as a gauge"})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/dashboard/default/widget", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerCreateDashboardWidget(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("create widget with Gemini prompt: status %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID          int64        `json:"id"`
		Name        string       `json:"name"`
		Description *string      `json:"description"`
		Config      WidgetConfig `json:"config"`
		Query       string       `json:"query"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.ID == 0 {
		t.Fatal("expected non-zero widget ID")
	}
	if resp.Name == "" {
		t.Fatal("expected non-empty widget name")
	}
	if resp.Query == "" {
		t.Fatal("expected non-empty SQL query")
	}
	if resp.Config.Type == "" {
		t.Fatal("expected non-empty widget config type")
	}

	t.Logf("Gemini created widget: id=%d name=%q type=%q query=%q", resp.ID, resp.Name, resp.Config.Type, resp.Query)

	// Verify the query actually runs
	rec = httptest.NewRecorder()
	queryBody, _ := json.Marshal(map[string]any{"query": resp.Query, "variable_values": map[string]any{}})
	req = httptest.NewRequest(http.MethodPost, "/api/query", bytes.NewReader(queryBody))
	dash.handlerExecuteQuery(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("execute Gemini-generated query: status %d: %s", rec.Code, rec.Body.String())
	}
	t.Logf("Gemini-generated query executed successfully")
}

func TestGeminiIntegration_ConversationPromptDirect(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini integration test")
	}

	ctx := context.Background()
	store, err := storage.NewSQLiteStorage("file:ai_direct_test?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("NewSQLiteStorage: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

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

	dash, err := New(ctx, store, "file:ai_direct_test?mode=memory&cache=shared", AIVendorGemini, apiKey, "")
	if err != nil {
		t.Fatalf("New dashboard: %v", err)
	}
	t.Cleanup(func() { _ = dash.db.Close() })

	// Create a widget first (needed for conversation history)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/dashboard/default/widget", bytes.NewReader([]byte(`{"Prompt": ""}`)))
	req = mux.SetURLVars(req, map[string]string{"url": "default"})
	dash.handlerCreateDashboardWidget(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create empty widget: status %d: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}

	conv, err := dash.ai.NewConversation(createResp.ID)
	if err != nil {
		t.Fatalf("NewConversation: %v", err)
	}

	sr, err := conv.Prompt("Show me how many replays are in the database. Use a gauge widget.")
	if err != nil {
		t.Fatalf("Prompt failed: %v", err)
	}

	if sr.Title == "" {
		t.Fatal("expected non-empty title")
	}
	if sr.SQLQuery == "" {
		t.Fatal("expected non-empty SQL query")
	}
	if sr.Config.Type == "" {
		t.Fatal("expected non-empty config type")
	}

	t.Logf("Gemini direct prompt result: title=%q type=%q query=%q", sr.Title, sr.Config.Type, sr.SQLQuery)

	// Verify the SQL actually works
	rows, err := store.Query(ctx, sr.SQLQuery)
	if err != nil {
		t.Fatalf("query returned by Gemini failed: %v\nquery: %s", err, sr.SQLQuery)
	}
	t.Logf("Query returned %d rows", len(rows))
	for i, row := range rows {
		if i >= 3 {
			t.Logf("  ... (%d more rows)", len(rows)-3)
			break
		}
		t.Logf("  row %d: %v", i, row)
	}
}

func TestGeminiIntegration_ToolConversion(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("GEMINI_API_KEY not set, skipping Gemini integration test")
	}

	ctx := context.Background()
	_, err := newGeminiLLM(apiKey, "")
	if err != nil {
		t.Fatalf("failed to create Gemini LLM: %v", err)
	}
	t.Logf("Gemini LLM created successfully with model %s", GeminiDefaultModel)

	_ = ctx
}

func newGeminiLLM(apiKey, model string) (*AI, error) {
	if model == "" {
		model = GeminiDefaultModel
	}
	a := &AI{ctx: context.Background()}
	opt := WithGemini(apiKey, model)
	if err := opt(a); err != nil {
		return nil, fmt.Errorf("WithGemini: %w", err)
	}
	return a, nil
}
