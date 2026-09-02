package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetupRouter_PlayerHotkeySignatureReturnsJSON(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/players/nobody/hotkey-signature", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Cards       []any          `json:"cards"`
		GamesByRace map[string]int `json:"games_by_race"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v: %s", err, rec.Body.String())
	}
	if payload.Cards == nil {
		t.Fatal("cards must be an empty array, not null")
	}
}

func TestSetupRouter_GameHotkeysUnknownReplayIs404(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/games/999999/hotkeys", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSetupRouter_HotkeyMapValidatesParams(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	for _, path := range []string{
		"/api/custom/hotkeys/map",
		"/api/custom/hotkeys/map?replay_id=1",
		"/api/custom/hotkeys/map?replay_id=1&player_id=1&minute=999",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d: %s", path, rec.Code, rec.Body.String())
		}
	}
}
