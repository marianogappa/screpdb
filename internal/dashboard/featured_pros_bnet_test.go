package dashboard

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/propack"
)

func TestFeaturedProRefreshTargetsAskOncePerAccount(t *testing.T) {
	now := time.Now()
	stale := now.Add(-2 * time.Hour)
	toons := []propack.Toon{
		{Toon: "kimsabuho", Gateway: 30},
		{Toon: "DemonHunterho", Gateway: 30},
		{Toon: "VOOVIDVOODAW", Gateway: 45},
		{Toon: "NeverSeen", Gateway: 20},
		{Toon: "FreshMiss", Gateway: 10},
		{Toon: "OldMiss", Gateway: 11},
		{Toon: "Fresh", Gateway: 30},
	}
	accountToons := []persist.BnetToon{{Toon: "kimsabuho", Gateway: 30}, {Toon: "DemonHunterho", Gateway: 30}, {Toon: "VOOVIDVOODAW", Gateway: 45, GamesLastWeek: 3}}
	profiles := []persist.BnetProfile{
		{Toon: "kimsabuho", Gateway: 30, Found: true, AuroraID: 1, FetchedAt: stale.Add(-time.Hour), Toons: accountToons},
		{Toon: "DemonHunterho", Gateway: 30, Found: true, AuroraID: 1, FetchedAt: stale, Toons: accountToons},
		{Toon: "FreshMiss", Gateway: 10, FetchedAt: now.Add(-time.Hour)},
		{Toon: "OldMiss", Gateway: 11, FetchedAt: now.Add(-25 * time.Hour)},
		{Toon: "Fresh", Gateway: 30, Found: true, AuroraID: 2, FetchedAt: now.Add(-time.Minute)},
	}
	got := featuredProRefreshTargets(toons, profiles, now, featuredProVisitMaxAge)
	want := map[bnetToonGateway]bool{
		{"NeverSeen", 20}:     true,
		{"OldMiss", 11}:       true,
		{"DemonHunterho", 30}: true,
	}
	if len(got) != len(want) {
		t.Fatalf("targets %v, want %v", got, want)
	}
	for _, target := range got {
		if !want[target.bnetToonGateway] {
			t.Errorf("unexpected target %v", target)
		}
		if target.toon == "DemonHunterho" && fmt.Sprint(target.alternates) != "[{VOOVIDVOODAW 45} {kimsabuho 30}]" {
			t.Errorf("alternates %v, want the account's other toons, most played first", target.alternates)
		}
	}
}

func TestFeaturedProRefreshTargetsSurviveARenamedToon(t *testing.T) {
	now := time.Now()
	toons := []propack.Toon{{Toon: "llalllllIIIlllI", Gateway: 30}}
	profiles := []persist.BnetProfile{
		{Toon: "llalllllIIIlllI", Gateway: 30, FetchedAt: now.Add(-time.Minute)},
		{Toon: "llallllliiillli", Gateway: 30, Found: true, AuroraID: 7, FetchedAt: now.Add(-48 * time.Hour),
			Toons: []persist.BnetToon{{Toon: "llalllllIIIlllI", Gateway: 30}, {Toon: "RoyaL1123", Gateway: 30}}},
	}
	got := featuredProRefreshTargets(toons, profiles, now, featuredProVisitMaxAge)
	if len(got) != 1 || got[0].bnetToonGateway != (bnetToonGateway{"llallllliiillli", 30}) {
		t.Fatalf("targets %v, want the account's found row, not the lowercase twin of the miss", got)
	}
	if fmt.Sprint(got[0].alternates) != "[{RoyaL1123 30}]" {
		t.Fatalf("alternates %v, want the account's toon that has not just missed", got[0].alternates)
	}
}

func bnetGameAt(id string, at time.Time, toon string) persist.BnetGame {
	return persist.BnetGame{
		GameID: id, CreateTime: at, Gateway: 30, MapName: "Map",
		Players: []persist.BnetGamePlayer{{Toon: toon, Race: "zerg", Team: 1}, {Toon: "rival", Race: "terran", Team: 2}},
	}
}

func TestMergeFeaturedBnetDetailShowsTheLeadAccountsLatestList(t *testing.T) {
	base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	profiles := []persist.BnetProfile{
		{Toon: "oldmain", Gateway: 30, Found: true, AuroraID: 1, BattleTag: "Old#1", FetchedAt: base,
			Toons: []persist.BnetToon{{Toon: "oldmain", Gateway: 30, GamesLastWeek: 9}}},
		{Toon: "newmain", Gateway: 30, Found: true, AuroraID: 2, BattleTag: "New#2", FetchedAt: base.Add(-time.Hour),
			Toons: []persist.BnetToon{{Toon: "newmain", Gateway: 30}, {Toon: "newalt", Gateway: 30, GamesLastWeek: 1}}},
	}
	games := map[int64][]persist.BnetGame{
		1: {bnetGameAt("a1", base.Add(-24*time.Hour), "oldmain")},
		2: {bnetGameAt("b1", base.Add(2*time.Hour), "newmain"), bnetGameAt("b0", base.Add(-48*time.Hour), "newalt")},
	}
	for i := 0; i < 30; i++ {
		games[2] = append(games[2], bnetGameAt(fmt.Sprintf("old%d", i), base.Add(-time.Duration(70+i)*24*time.Hour), "newmain"))
	}
	detail := mergeFeaturedBnetDetail(profiles, games)
	if detail == nil || detail.BattleTag != "New#2" {
		t.Fatalf("lead account: %+v", detail)
	}
	if len(detail.RecentGames) != bnetProfileGamesListed {
		t.Fatalf("recent games %d, want the %d a profile answer lists", len(detail.RecentGames), bnetProfileGamesListed)
	}
	if detail.RecentGames[0].GameID != "b1" || detail.RecentGames[1].GameID != "b0" {
		t.Fatalf("recent games start %s,%s, want the lead account's newest first", detail.RecentGames[0].GameID, detail.RecentGames[1].GameID)
	}
	for _, g := range detail.RecentGames {
		if g.GameID == "a1" {
			t.Fatal("another account's games leaked into the lead's list")
		}
	}
	var aliases []string
	for _, toon := range detail.Toons {
		aliases = append(aliases, toon.Toon)
	}
	if strings.Join(aliases, ",") != "newmain,oldmain,newalt" {
		t.Fatalf("aliases %v, want every account's toons by recency", aliases)
	}
}

func TestMergeFeaturedBnetDetailWithoutAnAccount(t *testing.T) {
	if detail := mergeFeaturedBnetDetail([]persist.BnetProfile{{Toon: "x", Gateway: 30}}, nil); detail != nil {
		t.Fatalf("a miss should not make a section: %+v", detail)
	}
}

func TestRefreshFeaturedProBnetArchivesNewGamesOncePerWindow(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()
	played := time.Now().Add(-10 * time.Minute).Unix()

	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.URL.Path)
		mu.Unlock()
		fmt.Fprintf(w, `{"aurora_id": 5, "toons": [{"toon": "ProMain", "gateway_id": 30}, {"toon": "ProAlt", "gateway_id": 30}],
			"game_results": [{"game_id": "fresh-game", "create_time": "%d", "gateway_id": 30, "attributes": {"mapName": "Map"},
			"players": [{"toon": "ProMain", "result": "win", "attributes": {"type": "player", "race": "zerg", "team": "1"}},
			{"toon": "Rival", "attributes": {"type": "player", "race": "terran", "team": "2"}}]}]}`, played)
	}))
	defer srv.Close()

	pro := &propack.Pro{ID: "testpro", Toons: []propack.Toon{{Toon: "ProMain", Gateway: 30}, {Toon: "ProAlt", Gateway: 30}}}
	seed := persist.BnetProfile{Toon: "ProMain", Gateway: 30, Found: true, AuroraID: 5, FetchedAt: time.Now().Add(-3 * time.Hour),
		Toons: []persist.BnetToon{{Toon: "ProMain", Gateway: 30}, {Toon: "ProAlt", Gateway: 30}}}
	if err := d.dbStore.UpsertBnetProfile(ctx, seed); err != nil {
		t.Fatal(err)
	}

	if d.refreshFeaturedProBnet(pro, d.featuredProBnetProfiles(ctx, pro), bnetfacade.PriorityUser, featuredProVisitMaxAge) {
		t.Fatal("no bridge, yet a refresh was reported")
	}
	d.bnetAddr.Store(srv.Listener.Addr().String())
	if !d.refreshFeaturedProBnet(pro, d.featuredProBnetProfiles(ctx, pro), bnetfacade.PriorityUser, featuredProVisitMaxAge) {
		t.Fatal("stale account was not refreshed")
	}
	waitFor(t, func() bool {
		d.featuredRefresh.mu.Lock()
		defer d.featuredRefresh.mu.Unlock()
		return !d.featuredRefresh.inflight[pro.ID]
	})
	mu.Lock()
	requests := len(paths)
	mu.Unlock()
	if requests != 1 {
		t.Fatalf("bridge requests %d, want one for the single account", requests)
	}
	games, err := d.dbStore.ListBnetGamesByAccount(ctx, 5)
	if err != nil || len(games) != 1 || games[0].GameID != "fresh-game" {
		t.Fatalf("archive %v (%v), want the new game", games, err)
	}
	if d.refreshFeaturedProBnet(pro, d.featuredProBnetProfiles(ctx, pro), bnetfacade.PriorityUser, featuredProVisitMaxAge) {
		t.Fatal("a just refreshed account was asked again")
	}
}

func waitFor(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRefreshFeaturedProBnetAsksAnAlternateWhenAToonIsGone(t *testing.T) {
	d := newTestDashboard(t)
	ctx := context.Background()
	var mu sync.Mutex
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		asked = append(asked, r.URL.Path)
		mu.Unlock()
		if strings.Contains(r.URL.Path, "/Renamed/") {
			fmt.Fprint(w, `{"aurora_id": 0}`)
			return
		}
		fmt.Fprint(w, `{"aurora_id": 9, "toons": [{"toon": "Current", "gateway_id": 30}]}`)
	}))
	defer srv.Close()
	d.bnetAddr.Store(srv.Listener.Addr().String())

	pro := &propack.Pro{ID: "renamed", Toons: []propack.Toon{{Toon: "Renamed", Gateway: 30}}}
	seed := persist.BnetProfile{Toon: "Renamed", Gateway: 30, Found: true, AuroraID: 9, FetchedAt: time.Now().Add(-48 * time.Hour),
		Toons: []persist.BnetToon{{Toon: "Renamed", Gateway: 30}, {Toon: "Current", Gateway: 30}}}
	if err := d.dbStore.UpsertBnetProfile(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if !d.refreshFeaturedProBnet(pro, d.featuredProBnetProfiles(ctx, pro), bnetfacade.PriorityUser, featuredProVisitMaxAge) {
		t.Fatal("stale account was not refreshed")
	}
	waitFor(t, func() bool {
		d.featuredRefresh.mu.Lock()
		defer d.featuredRefresh.mu.Unlock()
		return !d.featuredRefresh.inflight[pro.ID]
	})
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 2 || !strings.Contains(asked[1], "/Current/") {
		t.Fatalf("asked %v, want the renamed toon then its account's current one", asked)
	}
	detail := mergeFeaturedBnetDetail(d.featuredProBnetProfiles(ctx, pro), nil)
	if detail == nil || detail.Toon != "Current" {
		t.Fatalf("section %+v, want the account found through its current toon", detail)
	}
}

func TestMergeFeaturedBnetDetailKeepsOnlyTheLatestAnswersGames(t *testing.T) {
	base := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	profiles := []persist.BnetProfile{{Toon: "hero", Gateway: 30, Found: true, AuroraID: 3, FetchedAt: base.Add(72 * time.Hour), ListedFrom: base.Add(-time.Hour),
		Toons: []persist.BnetToon{{Toon: "hero", Gateway: 30}}}}
	games := map[int64][]persist.BnetGame{3: {
		bnetGameAt("sept", base.Add(-60*24*time.Hour), "hero"),
		bnetGameAt("nov-a", base.Add(-time.Hour), "hero"),
		bnetGameAt("nov-b", base, "hero"),
	}}
	detail := mergeFeaturedBnetDetail(profiles, games)
	var ids []string
	for _, g := range detail.RecentGames {
		ids = append(ids, g.GameID)
	}
	if strings.Join(ids, ",") != "nov-b,nov-a" {
		t.Fatalf("recent games %v, want only what the latest answer listed", ids)
	}

	profiles[0].ListedFrom = profiles[0].FetchedAt
	detail = mergeFeaturedBnetDetail(profiles, games)
	if len(detail.RecentGames) != 0 || detail.LastPlayedAt != base.Format(time.RFC3339) {
		t.Fatalf("an answer listing nothing: %d games, last played %q", len(detail.RecentGames), detail.LastPlayedAt)
	}
}
