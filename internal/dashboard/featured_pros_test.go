package dashboard

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/marianogappa/screpdb/internal/propack"
)

func loadPackForTest(t *testing.T) *propack.Pack {
	t.Helper()
	pack, err := propack.Load()
	if err != nil {
		t.Fatalf("propack.Load: %v", err)
	}
	if len(pack.Pros) == 0 {
		t.Skip("embedded pro pack is empty")
	}
	return pack
}

func TestFeaturedProsAppearOnPlayersListFirstPageOnly(t *testing.T) {
	pack := loadPackForTest(t)
	d := newTestDashboard(t)
	r := d.setupRouter()

	get := func(path string) map[string]json.RawMessage {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		var out map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		return out
	}
	var first []workflowFeaturedPlayerItem
	if err := json.Unmarshal(get("/api/players?limit=5")["featured_players"], &first); err != nil {
		t.Fatal(err)
	}
	if len(first) != len(pack.Pros) {
		t.Fatalf("first page featured %d, pack has %d", len(first), len(pack.Pros))
	}
	for _, item := range first {
		if _, ok := propack.IDFromKey(item.PlayerKey); !ok {
			t.Errorf("featured key %q lacks the pro prefix", item.PlayerKey)
		}
		if item.PlayerName == "" || item.GamesSampled == 0 {
			t.Errorf("featured item %+v incomplete", item)
		}
	}
	var second []workflowFeaturedPlayerItem
	if err := json.Unmarshal(get("/api/players?limit=5&offset=5")["featured_players"], &second); err != nil {
		t.Fatal(err)
	}
	if len(second) != 0 {
		t.Fatalf("second page carried %d featured rows", len(second))
	}
	needle := pack.Pros[0].Label
	var filtered []workflowFeaturedPlayerItem
	if err := json.Unmarshal(get("/api/players?limit=5&name=" + url.QueryEscape(needle))["featured_players"], &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered) == 0 || len(filtered) == len(pack.Pros) && len(pack.Pros) > 1 {
		t.Fatalf("name filter %q returned %d featured rows", needle, len(filtered))
	}
}

func TestFeaturedProEndpoints(t *testing.T) {
	pack := loadPackForTest(t)
	d := newTestDashboard(t)
	r := d.setupRouter()
	pro := pack.Pros[0]
	base := "/api/players/" + url.PathEscape(pro.Key())

	getJSON := func(path string, want int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Fatalf("%s: status %d (want %d): %s", path, rec.Code, want, rec.Body.String())
		}
		return rec.Body.Bytes()
	}

	var overview workflowPlayerOverview
	if err := json.Unmarshal(getJSON(base, http.StatusOK), &overview); err != nil {
		t.Fatal(err)
	}
	if overview.Featured == nil || overview.Featured.ID != pro.ID || overview.PlayerName != pro.Label {
		t.Fatalf("overview not featured: %+v", overview.Featured)
	}
	if overview.GamesPlayed != int64(pro.GamesSampled) {
		t.Fatalf("games_played %d, want sampled %d", overview.GamesPlayed, pro.GamesSampled)
	}

	var sig hotkeySignaturePayload
	if err := json.Unmarshal(getJSON(base+"/hotkey-signature", http.StatusOK), &sig); err != nil {
		t.Fatal(err)
	}
	if len(sig.Cards) != len(pro.Hotkeys) {
		t.Fatalf("hotkey cards %d, want %d", len(sig.Cards), len(pro.Hotkeys))
	}

	for _, insightType := range []string{"apm", "unit-production-cadence", "viewport-switch-rate"} {
		var insight workflowPlayerAsyncInsight
		if err := json.Unmarshal(getJSON(base+"/insight?type="+insightType, http.StatusOK), &insight); err != nil {
			t.Fatal(err)
		}
		if insight.PlayerKey != pro.Key() || insight.PlayerName != pro.Label {
			t.Errorf("%s: identity %q/%q", insightType, insight.PlayerKey, insight.PlayerName)
		}
	}

	for _, suffix := range []string{"/last-games", "/chat-summary", "/insights/apm-histogram", "/insights/unit-production-cadence"} {
		getJSON(base+suffix, http.StatusNotFound)
	}
	getJSON("/api/players/pro:definitely-not-a-pro", http.StatusNotFound)

	if pro.Photo != "" {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/custom/pros/"+pro.ID+"/photo", nil))
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Fatalf("photo: %d", rec.Code)
		}
	}
}

func TestFeaturedPointsRideTheDistributions(t *testing.T) {
	loadPackForTest(t)
	d := newTestDashboard(t)
	r := d.setupRouter()
	for _, path := range []string{
		"/api/players/insights/apm-histogram",
		"/api/players/insights/unit-production-cadence",
		"/api/players/insights/viewport-multitasking",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d", path, rec.Code)
		}
		var payload struct {
			Featured []workflowFeaturedPoint `json:"featured_players"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Featured) == 0 {
			t.Errorf("%s: no featured points", path)
		}
	}
}

// A progamer running the app must never be shown their own built-in profile:
// once one of the user's accounts is a known toon of the pro, the pro drops
// out of every featured surface.
func TestFeaturedProExcludedWhenUserIsThePro(t *testing.T) {
	pack := loadPackForTest(t)
	var pro *propack.Pro
	for i := range pack.Pros {
		if len(pack.Pros[i].Toons) > 0 {
			pro = &pack.Pros[i]
			break
		}
	}
	if pro == nil {
		t.Skip("no pro with known toons in the pack")
	}
	d := newTestDashboard(t)
	d.youKeys.Store(youKeySetFromBattleTags([]string{pro.Toons[0].Toon + "#1234"}))

	if got := d.featuredPro(pro.Key()); got != nil {
		t.Fatalf("pro %s still featured while the user is them", pro.ID)
	}
	for _, item := range d.featuredPlayersList("") {
		if item.PlayerKey == pro.Key() {
			t.Fatalf("pro %s still listed", pro.ID)
		}
	}
	if len(d.featuredPlayersList("")) != len(pack.Pros)-1 {
		t.Fatalf("expected exactly one exclusion, got %d of %d listed", len(d.featuredPlayersList("")), len(pack.Pros))
	}
}

func TestFeaturedProsOrderedByCuratedRank(t *testing.T) {
	d := newTestDashboard(t)
	pros := d.featuredPros()
	if len(pros) < 2 {
		t.Skip("pack has fewer than two pros")
	}
	lastRank := 0
	for _, pro := range pros {
		if pro.Rank <= 0 {
			break
		}
		if pro.Rank < lastRank {
			t.Fatalf("%s (rank %d) listed after rank %d", pro.ID, pro.Rank, lastRank)
		}
		lastRank = pro.Rank
	}
	if pros[0].Rank != 1 {
		t.Fatalf("first featured pro should carry rank 1, got %s rank %d", pros[0].ID, pros[0].Rank)
	}
}
