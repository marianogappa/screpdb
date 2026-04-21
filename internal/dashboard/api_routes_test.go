package dashboard

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestSetupRouter_JSONEndpoints verifies the embedded router wires custom + main API paths
// to handlers that return JSON (not the SPA fallback).
func TestSetupRouter_JSONEndpoints(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	tests := []struct {
		name   string
		method string
		path   string
		body   []byte
	}{
		{"health", http.MethodGet, "/api/health", nil},
		{"custom list dashboards", http.MethodGet, "/api/custom/dashboard", nil},
		{"custom get default dashboard", http.MethodGet, "/api/custom/dashboard/default", nil},
		{"games list", http.MethodGet, "/api/games", nil},
		{"players list", http.MethodGet, "/api/players", nil},
		{"player colors", http.MethodGet, "/api/player-colors", nil},
		{"players apm histogram", http.MethodGet, "/api/players/insights/apm-histogram", nil},
		{"players delay histogram", http.MethodGet, "/api/players/insights/first-unit-delay", nil},
		{"players cadence", http.MethodGet, "/api/players/insights/unit-production-cadence", nil},
		{"players viewport", http.MethodGet, "/api/players/insights/viewport-multitasking", nil},
		{"global replay filter get", http.MethodGet, "/api/custom/global-replay-filter", nil},
		{"global replay filter options", http.MethodGet, "/api/custom/global-replay-filter/options", nil},
		{"ingest settings get", http.MethodGet, "/api/custom/ingest/settings", nil},
		{"aliases list", http.MethodGet, "/api/custom/aliases", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reqBody io.Reader
			if len(tt.body) > 0 {
				reqBody = bytes.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, reqBody)
			if len(tt.body) > 0 {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
			}
			respBytes := rec.Body.Bytes()
			if len(respBytes) == 0 {
				t.Fatalf("empty body")
			}
			if respBytes[0] != '{' && respBytes[0] != '[' {
				t.Fatalf("expected JSON object or array, got first byte %q body prefix %q", respBytes[0], truncateForLog(respBytes, 120))
			}
			if !json.Valid(respBytes) {
				t.Fatalf("invalid JSON: %s", truncateForLog(respBytes, 200))
			}
		})
	}
}

func TestSetupRouter_CustomQueryExecute(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	body := []byte(`{"query": "SELECT 1 AS n", "variable_values": {}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/custom/query", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

func TestSetupRouter_PlayerChatSummaryThroughRouter(t *testing.T) {
	d := newTestDashboard(t)
	var playerKey string
	err := d.dbStore.DefaultQueryRow(`SELECT lower(trim(name)) FROM players WHERE is_observer = 0 LIMIT 1`).Scan(&playerKey)
	if err != nil {
		t.Skip("no players in test DB")
	}
	r := d.setupRouter()
	path := "/api/players/" + playerKey + "/chat-summary"
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

func TestSetupRouter_GameDetailThroughRouter(t *testing.T) {
	d := newTestDashboard(t)
	var replayID int64
	if err := d.dbStore.DefaultQueryRow(`SELECT id FROM replays ORDER BY id LIMIT 1`).Scan(&replayID); err != nil {
		t.Skip("no replays in test DB")
	}
	r := d.setupRouter()
	path := "/api/games/" + strconv.FormatInt(replayID, 10)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("invalid JSON: %s", rec.Body.String())
	}
}

func TestSetupRouter_GameAssetIconReturnsPNG(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/custom/game-assets/unit?name=marine", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.Bytes()
	if len(body) < 24 || string(body[1:4]) != "PNG" {
		t.Fatalf("expected PNG bytes, got len=%d prefix=%q", len(body), truncateForLog(body, 16))
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "image/png") {
		t.Fatalf("expected image/png content-type, got %q", ct)
	}
}

func TestSetupRouter_GameAssetMapReturnsPNG(t *testing.T) {
	d := newTestDashboard(t)
	var replayID int64
	if err := d.dbStore.DefaultQueryRow(`SELECT id FROM replays WHERE trim(coalesce(file_path,'')) != '' ORDER BY id LIMIT 1`).Scan(&replayID); err != nil {
		t.Skip("no replay with file_path in test DB")
	}
	r := d.setupRouter()
	path := "/api/custom/game-assets/map?replay_id=" + strconv.FormatInt(replayID, 10)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.Bytes()
	if len(body) < 24 || string(body[1:4]) != "PNG" {
		t.Fatalf("expected PNG bytes, got len=%d prefix=%q", len(body), truncateForLog(body, 16))
	}
}

// TestSetupRouter_LegacyWorkflowPrefixIsSPA proves old /api/workflow/* URLs are no longer API routes.
func TestSetupRouter_LegacyWorkflowPrefixIsSPA(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/workflow/games", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected SPA text/html for legacy /api/workflow path, got content-type %q", ct)
	}
}

func TestSetupRouter_LegacyCustomPathsWithoutPrefixAreSPA(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	for _, path := range []string{"/api/dashboard", "/api/query"} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d", rec.Code)
			}
			ct := rec.Header().Get("Content-Type")
			if !strings.Contains(ct, "text/html") {
				t.Fatalf("expected SPA for %s, got content-type %q", path, ct)
			}
		})
	}
}

func TestSetupRouter_StrictInputValidation(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/custom/query", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestSetupRouter_StatusWrappedServiceError(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/games/999999999999", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}

func truncateForLog(b []byte, max int) string {
	s := string(b)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
