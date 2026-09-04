package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/load"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
)

func gameIDs(rows []WorkflowGameListRow) []int64 {
	out := make([]int64, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ReplayID)
	}
	return out
}

func listedIDs(t *testing.T, s *LibStore, query GamesQuery) []int64 {
	t.Helper()
	rows, err := s.ListGames(context.Background(), query, 100, 0)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	total, err := s.CountGames(context.Background(), query)
	if err != nil {
		t.Fatalf("CountGames: %v", err)
	}
	if total != int64(len(rows)) {
		t.Fatalf("CountGames = %d but ListGames returned %d rows", total, len(rows))
	}
	return gameIDs(rows)
}

func containsID(ids []int64, want int64) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestLibStoreListGamesRowShapeAndOrder(t *testing.T) {
	newest := librarytest.Replay(
		librarytest.WithDate(librarytest.BaseDate),
		librarytest.WithMap("Polypoid"),
		librarytest.WithMatchup("TvZ"),
		librarytest.WithDuration(1500),
		librarytest.WithFlags(library.FlagTeamStacking|library.FlagTeamInfoIncomplete),
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
	)
	oldest := librarytest.Replay(
		librarytest.WithDate(librarytest.BaseDate.AddDate(-1, 0, 0)),
		librarytest.WithPlayer("Flash"),
	)
	s := newTestLibStore(t, oldest, newest)

	rows, err := s.ListGames(context.Background(), GamesQuery{}, 100, 0)
	if err != nil {
		t.Fatalf("ListGames: %v", err)
	}
	if len(rows) != 2 || rows[0].ReplayID != newest.ID || rows[1].ReplayID != oldest.ID {
		t.Fatalf("games are not newest first: %v", gameIDs(rows))
	}
	got := rows[0]
	if got.MapName != "Polypoid" || got.MapKind != "Regular" || got.GameType != "Melee" {
		t.Errorf("map/kind/type = %q/%q/%q", got.MapName, got.MapKind, got.GameType)
	}
	if got.Matchup != "TvZ" || got.DurationSeconds != 1500 {
		t.Errorf("matchup/duration = %q/%d", got.Matchup, got.DurationSeconds)
	}
	if !got.TeamStacking || !got.TeamInfoIncomplete {
		t.Errorf("flags did not survive: %+v", got)
	}
	if got.ReplayDate != newest.Date.String() || got.FileName != newest.FileName() {
		t.Errorf("date/file = %q/%q", got.ReplayDate, got.FileName)
	}
}

func TestLibStoreListGamesPaginates(t *testing.T) {
	first := melee("first")
	second := melee("second")
	third := melee("third")
	s := newTestLibStore(t, third, second, first)

	all := listedIDs(t, s, GamesQuery{})
	if len(all) != 3 {
		t.Fatalf("want 3 games, got %v", all)
	}
	page, err := s.ListGames(context.Background(), GamesQuery{}, 2, 0)
	if err != nil || len(page) != 2 {
		t.Fatalf("first page = %v, %v", gameIDs(page), err)
	}
	if page[0].ReplayID != all[0] || page[1].ReplayID != all[1] {
		t.Fatalf("first page out of order: %v vs %v", gameIDs(page), all)
	}
	page, err = s.ListGames(context.Background(), GamesQuery{}, 2, 2)
	if err != nil || len(page) != 1 || page[0].ReplayID != all[2] {
		t.Fatalf("second page = %v, %v", gameIDs(page), err)
	}
	if page, err := s.ListGames(context.Background(), GamesQuery{}, 2, 99); err != nil || len(page) != 0 {
		t.Fatalf("past-the-end page = %v, %v", gameIDs(page), err)
	}
}

func TestLibStoreGamesFilterAxes(t *testing.T) {
	flashTvZ := librarytest.Replay(
		librarytest.WithMap("Fighting Spirit"),
		librarytest.WithMatchup("TvZ"),
		librarytest.WithDuration(300),
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
	)
	bisuPvP := librarytest.Replay(
		librarytest.WithMap("Polypoid"),
		librarytest.WithMatchup("PvP"),
		librarytest.WithDuration(1800),
		librarytest.WithMapKind(library.MapKindMoney),
		librarytest.WithPlayer("Bisu", librarytest.Race(library.RaceProtoss)),
		librarytest.WithPlayer("Stork", librarytest.Race(library.RaceProtoss)),
	)
	s := newTestLibStore(t, flashTvZ, bisuPvP)

	cases := []struct {
		name  string
		query GamesQuery
		want  []int64
	}{
		{"no filter", GamesQuery{}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"player", GamesQuery{PlayerKeys: []string{"flash"}}, []int64{flashTvZ.ID}},
		{"player or", GamesQuery{PlayerKeys: []string{"flash", "bisu"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"player miss", GamesQuery{PlayerKeys: []string{"nobody"}}, nil},
		{"map", GamesQuery{MapNames: []string{"polypoid"}}, []int64{bisuPvP.ID}},
		{"map or", GamesQuery{MapNames: []string{"polypoid", "fighting spirit"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"duration under", GamesQuery{DurationBuckets: []string{"under_10m"}}, []int64{flashTvZ.ID}},
		{"duration over", GamesQuery{DurationBuckets: []string{"10m_plus"}}, []int64{bisuPvP.ID}},
		{"duration or", GamesQuery{DurationBuckets: []string{"under_10m", "10m_plus"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"duration unknown key is a no-op", GamesQuery{DurationBuckets: []string{"nonsense"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"matchup", GamesQuery{MatchupKeys: []string{"tvz"}}, []int64{flashTvZ.ID}},
		{"matchup or", GamesQuery{MatchupKeys: []string{"tvz", "pvp"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"matchup invalid key is a no-op", GamesQuery{MatchupKeys: []string{"tvtvt"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{"map kind money", GamesQuery{MapKindKeys: []string{"money"}}, []int64{bisuPvP.ID}},
		{"map kind regular", GamesQuery{MapKindKeys: []string{"regular"}}, []int64{flashTvZ.ID}},
		{"map kind both is a no-op", GamesQuery{MapKindKeys: []string{"money", "regular"}}, []int64{flashTvZ.ID, bisuPvP.ID}},
		{
			"axes AND together",
			GamesQuery{PlayerKeys: []string{"flash", "bisu"}, DurationBuckets: []string{"10m_plus"}},
			[]int64{bisuPvP.ID},
		},
		{
			"AND across axes can empty the result",
			GamesQuery{PlayerKeys: []string{"flash"}, MapNames: []string{"polypoid"}},
			nil,
		},
		{
			"three axes",
			GamesQuery{PlayerKeys: []string{"bisu"}, MatchupKeys: []string{"pvp"}, MapKindKeys: []string{"money"}},
			[]int64{bisuPvP.ID},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := listedIDs(t, s, tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for _, want := range tc.want {
				if !containsID(got, want) {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestLibStoreGamesFilterIgnoresObserverSlots(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("Artosis", librarytest.Observer()),
	)
	s := newTestLibStore(t, r)

	if got := listedIDs(t, s, GamesQuery{PlayerKeys: []string{"artosis"}}); len(got) != 0 {
		t.Fatalf("observer slot matched the player axis: %v", got)
	}
	if got := listedIDs(t, s, GamesQuery{PlayerKeys: []string{"flash"}}); len(got) != 1 {
		t.Fatalf("player axis missed a real slot: %v", got)
	}
}

// featureFixture is one replay carrying exactly the evidence one UI featuring
// key needs.
type featureFixture struct {
	key   string
	build func() *library.Replay
}

func libFeatureFixtures() []featureFixture {
	withPlayers := func(opts ...librarytest.Option) *library.Replay {
		base := []librarytest.Option{
			librarytest.WithPlayer("Flash"),
			librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
		}
		return librarytest.Replay(append(base, opts...)...)
	}
	event := func(eventType string) func() *library.Replay {
		return func() *library.Replay { return withPlayers(librarytest.WithEvent(eventType, 200, 0, 1)) }
	}
	marker := func(featureKey string) func() *library.Replay {
		return func() *library.Replay { return withPlayers(librarytest.WithMarker(featureKey, 0, 300, "")) }
	}
	return []featureFixture{
		{"cannon_rush", event("cannon_rush")},
		{"bunker_rush", event("bunker_rush")},
		{"zergling_rush", event("zergling_rush")},
		{"proxy_gate", event("proxy_gate")},
		{"proxy_rax", event("proxy_rax")},
		{"proxy_factory", event("proxy_factory")},
		{"proxy_starport", event("proxy_starport")},
		{"manner_pylon", event("manner_pylon")},
		{"drop", event("drop")},
		{"cliff_drop", marker("cliff_drop")},
		{"mind_control", marker("became_terran")},
		{"nukes", marker("threw_nukes")},
		{"recalls", marker("made_recalls")},
		{"carriers", marker("carriers")},
		{"battlecruisers", marker("battlecruisers")},
		{"team_stacking", func() *library.Replay {
			return withPlayers(librarytest.WithFlags(library.FlagTeamStacking))
		}},
		{"bo_4_pool", marker("bo_4_pool")},
		{"bo_z_fuzzy::~10 hatch", func() *library.Replay {
			return withPlayers(librarytest.WithFuzzyLabel(1, 240, "~10 Hatch"))
		}},
	}
}

// TestLibStoreFeaturingCountAgreesWithTheFilter is the invariant the shared
// shape exists for: a chip's count and what clicking it returns are the same
// question asked twice.
func TestLibStoreFeaturingCountAgreesWithTheFilter(t *testing.T) {
	fixtures := libFeatureFixtures()
	replays := make([]*library.Replay, 0, len(fixtures)+1)
	byKey := map[string]int64{}
	for _, fixture := range fixtures {
		r := fixture.build()
		replays = append(replays, r)
		byKey[fixture.key] = r.ID
	}
	// A game with no featuring evidence at all must never be counted or listed.
	plain := librarytest.Replay(librarytest.WithPlayer("Flash"), librarytest.WithPlayer("Bisu"))
	replays = append(replays, plain)
	s := newTestLibStore(t, replays...)

	keys := make([]string, 0, len(fixtures))
	for _, fixture := range fixtures {
		keys = append(keys, fixture.key)
	}
	counts, err := s.CountWorkflowFeaturingGames(context.Background(), keys)
	if err != nil {
		t.Fatalf("CountWorkflowFeaturingGames: %v", err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.key, func(t *testing.T) {
			ids := listedIDs(t, s, GamesQuery{Featuring: []string{fixture.key}})
			if counts[fixture.key] != int64(len(ids)) {
				t.Fatalf("count = %d, filter returned %d games", counts[fixture.key], len(ids))
			}
			if len(ids) != 1 || ids[0] != byKey[fixture.key] {
				t.Fatalf("filter returned %v, want the %s fixture (%d)", ids, fixture.key, byKey[fixture.key])
			}
			if containsID(ids, plain.ID) {
				t.Fatalf("the featureless game matched %s", fixture.key)
			}
		})
	}
	if counts["cannon_rush"] == 0 {
		t.Fatal("counts are all zero over a corpus that carries every feature")
	}
	if _, ok := counts["not_a_feature"]; ok {
		t.Fatal("an unrecognised key must not appear in the counts")
	}
	if got := listedIDs(t, s, GamesQuery{Featuring: []string{"not_a_feature"}}); len(got) != len(replays) {
		t.Fatalf("an unrecognised featuring key must be a no-op, got %v", got)
	}
}

func TestLibStoreFeaturingIsOrWithinTheAxis(t *testing.T) {
	nukes := librarytest.Replay(librarytest.WithPlayer("Flash"), librarytest.WithMarker("threw_nukes", 0, 600, ""))
	cannons := librarytest.Replay(librarytest.WithPlayer("Bisu"), librarytest.WithEvent("cannon_rush", 200, 0, library.NoPlayer))
	neither := librarytest.Replay(librarytest.WithPlayer("Stork"))
	s := newTestLibStore(t, nukes, cannons, neither)

	got := listedIDs(t, s, GamesQuery{Featuring: []string{"nukes", "cannon_rush"}})
	if len(got) != 2 || !containsID(got, nukes.ID) || !containsID(got, cannons.ID) {
		t.Fatalf("featuring OR = %v, want the nuke and cannon games", got)
	}
}

func TestLibStoreFuzzyLabelIsPerValue(t *testing.T) {
	tenHatch := librarytest.Replay(librarytest.WithPlayer("Jaedong"), librarytest.WithFuzzyLabel(0, 240, "~10 Hatch"))
	nineOverpool := librarytest.Replay(librarytest.WithPlayer("Soulkey"), librarytest.WithFuzzyLabel(0, 200, "~9 Overpool"))
	s := newTestLibStore(t, tenHatch, nineOverpool)

	if got := listedIDs(t, s, GamesQuery{Featuring: []string{"bo_z_fuzzy::~10 hatch"}}); len(got) != 1 || got[0] != tenHatch.ID {
		t.Fatalf("per-value filter = %v, want only the ~10 Hatch game", got)
	}
	if got := listedIDs(t, s, GamesQuery{Featuring: []string{"bo_z_fuzzy"}}); len(got) != 2 {
		t.Fatalf("the plain marker key must match both games, got %v", got)
	}
	labels, err := s.ListDistinctMarkerLabels(context.Background(), "bo_z_fuzzy")
	if err != nil {
		t.Fatalf("ListDistinctMarkerLabels: %v", err)
	}
	if len(labels) != 2 || labels[0] != "~9 Overpool" || labels[1] != "~10 Hatch" {
		t.Fatalf("labels = %v, want supply-rung order", labels)
	}
}

func TestLibStoreCountWorkflowDurationBucketsKeepsFive(t *testing.T) {
	s := newTestLibStore(t,
		melee("a", librarytest.WithDuration(300)),
		melee("b", librarytest.WithDuration(599)),
		melee("c", librarytest.WithDuration(600)),
		melee("d", librarytest.WithDuration(1200)),
		melee("e", librarytest.WithDuration(1800)),
		melee("f", librarytest.WithDuration(2700)),
		melee("g", librarytest.WithDuration(5000)),
	)
	under10m, m1020, m2030, m3045, m45Plus, err := s.CountWorkflowDurationBuckets(context.Background())
	if err != nil {
		t.Fatalf("CountWorkflowDurationBuckets: %v", err)
	}
	if under10m != 2 || m1020 != 1 || m2030 != 1 || m3045 != 1 || m45Plus != 2 {
		t.Fatalf("buckets = %d/%d/%d/%d/%d", under10m, m1020, m2030, m3045, m45Plus)
	}
}

func TestLibStoreMatchupAndMapKindCounts(t *testing.T) {
	s := newTestLibStore(t,
		melee("a", librarytest.WithMatchup("TvZ")),
		melee("b", librarytest.WithMatchup("TvZ")),
		melee("c", librarytest.WithMatchup("PvP"), librarytest.WithMapKind(library.MapKindMoney)),
		melee("d"),
	)
	matchups, err := s.CountWorkflowMatchupGames(context.Background())
	if err != nil {
		t.Fatalf("CountWorkflowMatchupGames: %v", err)
	}
	if matchups["tvz"] != 2 || matchups["pvp"] != 1 {
		t.Fatalf("matchups = %v", matchups)
	}
	if _, ok := matchups[""]; ok {
		t.Fatal("the empty matchup must not be a bucket")
	}
	mapKinds, err := s.CountWorkflowMapKindGames(context.Background())
	if err != nil {
		t.Fatalf("CountWorkflowMapKindGames: %v", err)
	}
	if mapKinds["regular"] != 3 || mapKinds["money"] != 1 {
		t.Fatalf("map kinds = %v", mapKinds)
	}
}

// TestLibStoreFilterOptionCountsIgnoreTheGamesListFilter pins the deliberate
// design: the chips describe the corpus, not the current result set, so a zero
// count means a dead end rather than "narrow further".
func TestLibStoreFilterOptionCountsIgnoreTheGamesListFilter(t *testing.T) {
	s := newTestLibStore(t,
		melee("a", librarytest.WithMatchup("TvZ")),
		melee("b", librarytest.WithMatchup("PvP")),
	)
	filtered := listedIDs(t, s, GamesQuery{MatchupKeys: []string{"tvz"}})
	if len(filtered) != 1 {
		t.Fatalf("filter did not narrow: %v", filtered)
	}
	matchups, err := s.CountWorkflowMatchupGames(context.Background())
	if err != nil {
		t.Fatalf("CountWorkflowMatchupGames: %v", err)
	}
	if matchups["pvp"] != 1 {
		t.Fatalf("PvP count changed under an active TvZ filter: %v", matchups)
	}
}

func TestLibStoreListWorkflowFilterPlayersNeedsFiveGames(t *testing.T) {
	replays := []*library.Replay{}
	for i := 0; i < 5; i++ {
		replays = append(replays, librarytest.Replay(
			librarytest.WithPlayer("Flash"),
			librarytest.WithPlayer("bisu"),
		))
	}
	replays = append(replays, librarytest.Replay(
		librarytest.WithPlayer("BISU"),
		librarytest.WithPlayer("Stork"),
		librarytest.WithPlayer("Artosis", librarytest.Observer()),
	))
	s := newTestLibStore(t, replays...)

	rows, err := s.ListWorkflowFilterPlayers(context.Background())
	if err != nil {
		t.Fatalf("ListWorkflowFilterPlayers: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want the two players with 5+ games", rows)
	}
	if rows[0].Games != 6 || rows[0].Key != "bisu" {
		t.Fatalf("first row = %+v, want bisu with 6 games", rows[0])
	}
	if rows[0].Label != "BISU" {
		t.Errorf("label = %q, want the lexicographically smallest raw name", rows[0].Label)
	}
	if rows[1].Key != "flash" || rows[1].Games != 5 {
		t.Fatalf("second row = %+v", rows[1])
	}
	for _, row := range rows {
		if row.Key == "artosis" || row.Key == "stork" {
			t.Fatalf("observer or sub-five player leaked in: %+v", rows)
		}
	}
}

func TestLibStoreListWorkflowFilterMapsRanksAndCaps(t *testing.T) {
	replays := []*library.Replay{
		melee("a", librarytest.WithMap("Polypoid")),
		melee("b", librarytest.WithMap("POLYPOID")),
		melee("c", librarytest.WithMap("Fighting Spirit")),
	}
	for i := 0; i < 20; i++ {
		replays = append(replays, melee("filler", librarytest.WithMap(string(rune('A'+i))+" Map")))
	}
	s := newTestLibStore(t, replays...)

	rows, err := s.ListWorkflowFilterMaps(context.Background())
	if err != nil {
		t.Fatalf("ListWorkflowFilterMaps: %v", err)
	}
	if len(rows) != 15 {
		t.Fatalf("rows = %d, want the 15-map cap", len(rows))
	}
	if rows[0].Label != "POLYPOID" || rows[0].Games != 2 {
		t.Fatalf("top row = %+v, want the case-collapsed Polypoid group with 2 games", rows[0])
	}
}

func TestLibStoreListReplayPlayersOrderAndShape(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Jaedong", librarytest.Team(2), librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Winner()),
		librarytest.WithPlayer("Artosis", librarytest.Team(3), librarytest.Observer()),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListReplayPlayers(context.Background(), []int64{r.ID})
	if err != nil {
		t.Fatalf("ListReplayPlayers: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want two non-observer players", rows)
	}
	if rows[0].Name != "Flash" || rows[0].Team != 1 || !rows[0].IsWinner {
		t.Fatalf("first row = %+v, want the team-1 winner", rows[0])
	}
	if rows[1].Name != "Jaedong" || rows[1].Race != "Zerg" {
		t.Fatalf("second row = %+v", rows[1])
	}
	if rows[0].PlayerID != library.PlayerID(r.ID, 1) || rows[1].PlayerID != library.PlayerID(r.ID, 0) {
		t.Fatalf("player ids = %d/%d", rows[0].PlayerID, rows[1].PlayerID)
	}
	if rows, err := s.ListReplayPlayers(context.Background(), nil); err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("no ids must give an empty non-nil slice, got %v, %v", rows, err)
	}
}

func TestLibStoreFeaturingRowReads(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
		librarytest.WithMarker("threw_nukes", 0, 700, ""),
		librarytest.WithFuzzyLabel(0, 240, "~10 Hatch"),
		librarytest.WithEvent("cannon_rush", 180, 0, library.NoPlayer),
		librarytest.WithEvent("manner_pylon", 190, 0, library.NoPlayer),
		librarytest.WithEvent("player_dropped", 800, 0, library.NoPlayer),
	)
	s := newTestLibStore(t, r)
	ctx := context.Background()

	patternRows, err := s.ListFeaturingPlayerPatternRows(ctx, []int64{r.ID})
	if err != nil {
		t.Fatalf("ListFeaturingPlayerPatternRows: %v", err)
	}
	byName := map[string]WorkflowPlayerPatternRow{}
	for _, row := range patternRows {
		byName[row.PatternName] = row
	}
	nuke, ok := byName["threw_nukes"]
	if !ok {
		t.Fatalf("rows = %+v, want threw_nukes", patternRows)
	}
	if nuke.ValueBool == nil || !*nuke.ValueBool || nuke.DetectedSecond != 700 {
		t.Errorf("nuke row = %+v", nuke)
	}
	fuzzy, ok := byName["bo_z_fuzzy"]
	if !ok || fuzzy.ValueString == nil || *fuzzy.ValueString != `{"label":"~10 Hatch"}` {
		t.Errorf("fuzzy row = %+v", fuzzy)
	}

	eventRows, err := s.ListFeaturingReplayEventRows(ctx, []int64{r.ID})
	if err != nil {
		t.Fatalf("ListFeaturingReplayEventRows: %v", err)
	}
	if len(eventRows) != 1 || eventRows[0].EventType != "cannon_rush" {
		t.Fatalf("event rows = %+v, want only the allowlisted cannon_rush", eventRows)
	}

	if rows, err := s.ListFeaturingPlayerPatternRows(ctx, nil); err != nil || len(rows) != 0 || rows == nil {
		t.Fatalf("no ids must give an empty non-nil slice, got %v, %v", rows, err)
	}
	if rows, err := s.ListFeaturingReplayEventRows(ctx, nil); err != nil || len(rows) != 0 || rows == nil {
		t.Fatalf("no ids must give an empty non-nil slice, got %v, %v", rows, err)
	}
}

func TestLibStoreCurrentPlayerReads(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.APM(310, 240), librarytest.Winner()),
		librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
		librarytest.WithMarker("threw_nukes", 0, 500, ""),
		librarytest.WithMarker("carriers", 1, 400, `{"count":7}`),
		librarytest.WithEvent("player_dropped", 900, 1, library.NoPlayer),
	)
	s := newTestLibStore(t, r)
	ctx := context.Background()

	rows, err := s.ListCurrentPlayersForReplayIDs(ctx, "flash", []int64{r.ID})
	if err != nil {
		t.Fatalf("ListCurrentPlayersForReplayIDs: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want one Flash slot", rows)
	}
	if rows[0].APM != 310 || rows[0].EAPM != 240 || !rows[0].IsWinner || rows[0].Race != "Terran" {
		t.Fatalf("row = %+v", rows[0])
	}
	flashID := library.PlayerID(r.ID, 0)
	jaedongID := library.PlayerID(r.ID, 1)
	if rows[0].PlayerID != flashID {
		t.Fatalf("player id = %d, want %d", rows[0].PlayerID, flashID)
	}

	patterns, err := s.ListPatternValuesForPlayerIDs(ctx, []int64{flashID, jaedongID})
	if err != nil {
		t.Fatalf("ListPatternValuesForPlayerIDs: %v", err)
	}
	if len(patterns) != 2 {
		t.Fatalf("patterns = %+v, want one per player", patterns)
	}
	if patterns[0].PlayerID != flashID || patterns[0].PatternName != "threw_nukes" {
		t.Fatalf("first pattern = %+v, want player-id order", patterns[0])
	}
	if patterns[0].PatternValue != "true" || patterns[0].Payload != "" {
		t.Errorf("a payload-less marker must read as true with no payload: %+v", patterns[0])
	}
	if patterns[1].PatternValue != `{"count":7}` || patterns[1].Payload != `{"count":7}` {
		t.Errorf("payload row = %+v", patterns[1])
	}

	dropped, err := s.ListDroppedPlayerIDs(ctx, []int64{flashID, jaedongID})
	if err != nil {
		t.Fatalf("ListDroppedPlayerIDs: %v", err)
	}
	if dropped[flashID] || !dropped[jaedongID] {
		t.Fatalf("dropped = %v, want only Jaedong", dropped)
	}
	if got, err := s.ListDroppedPlayerIDs(ctx, nil); err != nil || len(got) != 0 || got == nil {
		t.Fatalf("no ids must give an empty non-nil map, got %v, %v", got, err)
	}
}

func TestLibStoreGamesListReadsHonourTheGlobalFilter(t *testing.T) {
	long := melee("long", librarytest.WithMatchup("TvZ"))
	short := melee("short", librarytest.WithDuration(30), librarytest.WithMatchup("PvP"))
	s := newTestLibStore(t, long, short)

	if got := listedIDs(t, s, GamesQuery{}); len(got) != 1 || got[0] != long.ID {
		t.Fatalf("games list = %v, want only the long game", got)
	}
	matchups, err := s.CountWorkflowMatchupGames(context.Background())
	if err != nil {
		t.Fatalf("CountWorkflowMatchupGames: %v", err)
	}
	if _, ok := matchups["pvp"]; ok {
		t.Fatalf("a filtered-out game reached the filter-option counts: %v", matchups)
	}
}

// TestLibStoreFilterOptionsOverTheRealCorpus mirrors the contract of
// internal/dashboard/filter_options_coverage_test.go: over the ingested
// testdata replays every filter axis comes back populated with counts that
// describe the corpus.
func TestLibStoreFilterOptionsOverTheRealCorpus(t *testing.T) {
	corpus, err := filepath.Abs(filepath.Join("..", "..", "testdata", "replays"))
	if err != nil {
		t.Fatalf("resolve corpus: %v", err)
	}
	if _, err := os.Stat(corpus); err != nil {
		t.Skipf("no replay corpus: %v", err)
	}
	t.Cleanup(iofacade.Reset)
	if err := iofacade.AllowDir(corpus); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	lib := library.New(library.Options{CoalesceRecords: 1, CoalesceDelay: time.Millisecond})
	t.Cleanup(lib.Close)
	loader := load.New(lib, load.Options{Folder: corpus, Generation: 1, Workers: 2, PublishRate: time.Millisecond})
	if err := loader.Run(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	lib.Flush()
	if lib.Snapshot().Len() == 0 {
		t.Fatal("the corpus loaded to nothing")
	}
	s := NewLibStore(lib, nil, nil)
	ctx := context.Background()

	games, err := s.CountGames(ctx, GamesQuery{})
	if err != nil {
		t.Fatalf("CountGames: %v", err)
	}
	if games == 0 {
		t.Fatalf("no games survived the global filter over %d loaded replays", lib.Snapshot().Len())
	}

	players, err := s.ListWorkflowFilterPlayers(ctx)
	if err != nil {
		t.Fatalf("ListWorkflowFilterPlayers: %v", err)
	}
	maps, err := s.ListWorkflowFilterMaps(ctx)
	if err != nil {
		t.Fatalf("ListWorkflowFilterMaps: %v", err)
	}
	if len(maps) == 0 {
		t.Fatalf("maps axis empty over %d games", games)
	}
	mapGames := int64(0)
	for _, row := range maps {
		mapGames += row.Games
	}
	if mapGames == 0 {
		t.Fatal("map counts all zero over a non-empty corpus")
	}

	under10m, m1020, m2030, m3045, m45Plus, err := s.CountWorkflowDurationBuckets(ctx)
	if err != nil {
		t.Fatalf("CountWorkflowDurationBuckets: %v", err)
	}
	if under10m+m1020+m2030+m3045+m45Plus != games {
		t.Fatalf("duration buckets sum to %d, want %d", under10m+m1020+m2030+m3045+m45Plus, games)
	}

	matchups, err := s.CountWorkflowMatchupGames(ctx)
	if err != nil {
		t.Fatalf("CountWorkflowMatchupGames: %v", err)
	}
	matchupGames := int64(0)
	for _, count := range matchups {
		matchupGames += count
	}
	if matchupGames == 0 {
		t.Fatal("matchup counts all zero over a non-empty corpus")
	}

	mapKinds, err := s.CountWorkflowMapKindGames(ctx)
	if err != nil {
		t.Fatalf("CountWorkflowMapKindGames: %v", err)
	}
	mapKindGames := int64(0)
	for _, count := range mapKinds {
		mapKindGames += count
	}
	if mapKindGames != games {
		t.Fatalf("map-kind counts sum to %d, want %d", mapKindGames, games)
	}

	featureKeys := []string{}
	for _, m := range markers.Markers() {
		featureKeys = append(featureKeys, m.FeatureKey)
	}
	featureKeys = append(featureKeys,
		"cannon_rush", "bunker_rush", "zergling_rush", "proxy_gate", "proxy_rax",
		"proxy_factory", "proxy_starport", "manner_pylon", "drop", "mind_control",
		"nukes", "recalls", "carriers", "battlecruisers", "team_stacking",
	)
	labels, err := s.ListDistinctMarkerLabels(ctx, "bo_z_fuzzy")
	if err != nil {
		t.Fatalf("ListDistinctMarkerLabels: %v", err)
	}
	for _, label := range labels {
		featureKeys = append(featureKeys, PerValueFeatureKey("bo_z_fuzzy", label))
	}
	counts, err := s.CountWorkflowFeaturingGames(ctx, featureKeys)
	if err != nil {
		t.Fatalf("CountWorkflowFeaturingGames: %v", err)
	}
	featuringGames := int64(0)
	for _, count := range counts {
		featuringGames += count
	}
	if len(counts) == 0 || featuringGames == 0 {
		t.Fatalf("featuring axis empty or uncounted: %d options, %d games", len(counts), featuringGames)
	}

	// Every non-zero chip must be clickable: its count is exactly what the
	// filter returns, over the real corpus and not just fixtures.
	for key, count := range counts {
		if count == 0 {
			continue
		}
		ids := listedIDs(t, s, GamesQuery{Featuring: []string{key}})
		if int64(len(ids)) != count {
			t.Fatalf("%s: count = %d but the filter returned %d games", key, count, len(ids))
		}
	}

	t.Logf("corpus: %d games, %d players, %d maps, %d featuring chips", games, len(players), len(maps), len(counts))
}
