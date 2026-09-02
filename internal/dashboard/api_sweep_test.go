package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestReadOnlyEndpointSweep exercises every read-only game- and player-scoped
// endpoint against the whole ingested test corpus. It asserts the endpoints
// answer 200 with valid JSON for real data of every shape the corpus holds
// (1v1s, a multi-team money map, all three races), which pins the many
// data-dependent branches a single-fixture test never reaches.
func TestReadOnlyEndpointSweep(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()

	getJSON := func(t *testing.T, path string) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", path, rec.Code, rec.Body.String())
		}
		if !json.Valid(rec.Body.Bytes()) {
			t.Fatalf("%s: invalid JSON", path)
		}
		return rec.Body.Bytes()
	}

	for _, replayID := range listTestGames(t, r) {
		id := itoa64(replayID)
		getJSON(t, "/api/games/"+id)
		getJSON(t, "/api/games/"+id+"/hotkeys")
		getJSON(t, "/api/custom/debug/map-layout/"+id)
	}

	var playersPayload struct {
		Items []struct {
			Name string `json:"player_key"`
		} `json:"items"`
	}
	if err := json.Unmarshal(getJSON(t, "/api/players?limit=50"), &playersPayload); err != nil {
		t.Fatalf("players json: %v", err)
	}
	if len(playersPayload.Items) == 0 {
		t.Fatal("no players in corpus")
	}
	playerPaths := []string{
		"",
		"/last-games",
		"/chat-summary",
		"/hotkey-signature",
		"/insights/apm-histogram",
		"/insights/unit-production-cadence",
		"/insight?type=apm",
		"/insight?type=unit-production-cadence",
		"/insight?type=viewport-switch-rate",
	}
	seen := 0
	for _, p := range playersPayload.Items {
		if p.Name == "" {
			continue
		}
		seen++
		if seen > 6 {
			break
		}
		base := "/api/players/" + url.PathEscape(normalizePlayerKey(p.Name))
		for _, suffix := range playerPaths {
			getJSON(t, base+suffix)
		}
	}
	if seen == 0 {
		t.Fatal("no named players to sweep")
	}

	for _, path := range []string{
		"/api/players/insights/viewport-multitasking",
		"/api/custom/markers/definitions",
		"/api/custom/update/status",
	} {
		getJSON(t, path)
	}
}

// TestGameAssetMapReturnsPNG renders the plain terrain map for a real replay
// through the shared game-assets cache path.
func TestGameAssetMapReturnsPNG(t *testing.T) {
	d := newTestDashboard(t)
	r := d.setupRouter()
	games := listTestGames(t, r)
	if len(games) == 0 {
		t.Fatal("no games")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/custom/game-assets/map?replay_id="+itoa64(games[0]), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("map asset: %d %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.Bytes(); len(body) < 24 || string(body[1:4]) != "PNG" {
		t.Fatalf("expected PNG bytes, got prefix %q", rec.Body.Bytes()[:8])
	}
}
