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
	if len(first) != len(featuredVisibleIDs) {
		t.Fatalf("first page featured %d, visible set has %d", len(first), len(featuredVisibleIDs))
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
	var needle string
	for i := range pack.Pros {
		if featuredVisibleIDs[pack.Pros[i].ID] {
			needle = pack.Pros[i].Label
			break
		}
	}
	if needle == "" {
		t.Skip("no visible pro in pack")
	}
	var filtered []workflowFeaturedPlayerItem
	if err := json.Unmarshal(get("/api/players?limit=5&name=" + url.QueryEscape(needle))["featured_players"], &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered) == 0 {
		t.Fatalf("name filter %q returned 0 featured rows", needle)
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

	for _, suffix := range []string{"/last-games", "/chat-summary"} {
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

// The Players page Skill tab draws the built-in progamers who carry Skill as stars beside the library's
// medians.
func TestPlayersSkillCarriesTheProgamers(t *testing.T) {
	pack := loadPackForTest(t)
	d := newTestDashboard(t)
	r := d.setupRouter()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/custom/players/skill", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload workflowPlayersSkill
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	withSkill := 0
	for _, pro := range pack.Pros {
		if pro.Skill != nil {
			withSkill++
		}
	}
	if withSkill > 0 && len(payload.Featured) == 0 {
		t.Fatalf("%d pros carry Skill but none are drawn", withSkill)
	}
	for _, f := range payload.Featured {
		if f.PlayerKey == "" || f.Games == 0 || f.Combined.Position <= 0 {
			t.Errorf("featured %+v", f)
		}
	}
	if payload.Model == nil || payload.MinGames != minSkillGames {
		t.Fatalf("model %v min games %d", payload.Model, payload.MinGames)
	}
}

// A progamer running the app must never be shown their own built-in profile:
// once one of the user's accounts is a known toon of the pro, the pro drops
// out of every featured surface.
func TestFeaturedProExcludedWhenUserIsThePro(t *testing.T) {
	pack := loadPackForTest(t)
	var pro *propack.Pro
	for i := range pack.Pros {
		if len(pack.Pros[i].Toons) > 0 && featuredVisibleIDs[pack.Pros[i].ID] {
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
	visibleCount := 0
	for _, p := range pack.Pros {
		if featuredVisibleIDs[p.ID] {
			visibleCount++
		}
	}
	if len(d.featuredPlayersList("")) != visibleCount-1 {
		t.Fatalf("expected exactly one exclusion, got %d of %d listed", len(d.featuredPlayersList("")), visibleCount)
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

// The Skill overlay draws a capped, race-balanced slice of the pack, not all of it: an unbounded overlay
// buries the local players it is meant to give a reference for.
func TestFeaturedOverlayIsCappedAndRaceBalanced(t *testing.T) {
	loadPackForTest(t)
	d := newTestDashboard(t)
	pros := d.featuredOverlayPros(func(p *propack.Pro) bool { return p.APM != nil })
	if len(pros) == 0 || len(pros) > featuredOverlayLimit {
		t.Fatalf("%d overlay pros, want 1 to %d", len(pros), featuredOverlayLimit)
	}
	perRace := map[string]int{}
	for _, pro := range pros {
		perRace[pro.MainRace]++
	}
	for _, race := range []string{"Zerg", "Terran", "Protoss"} {
		if perRace[race] == 0 || perRace[race] > featuredOverlayPerRace {
			t.Errorf("%d %s pros, want 1 to %d", perRace[race], race, featuredOverlayPerRace)
		}
	}
	all := d.featuredPros()
	for _, pro := range all {
		if d.featuredPro(pro.Key()) == nil {
			t.Fatalf("pro %s dropped off its own page", pro.ID)
		}
	}
}
