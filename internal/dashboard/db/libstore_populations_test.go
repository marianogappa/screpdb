package db

import (
	"context"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
)

// daysAgoDate is a date the players-list day arithmetic reads as exactly
// `days` ago: UTC so the wall-clock-as-UTC reinterpretation is an identity,
// and an extra hour so truncation cannot round it into the previous day.
func daysAgoDate(days int) time.Time {
	return time.Now().UTC().Add(-time.Duration(days)*24*time.Hour - time.Hour)
}

func playersListNames(rows []WorkflowPlayersListRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.PlayerName)
	}
	return out
}

func TestLibStoreCountReplaysIgnoresTheGlobalFilter(t *testing.T) {
	s := newTestLibStore(t, melee("long"), melee("short", librarytest.WithDuration(30)))

	total, err := s.CountReplays(context.Background())
	if err != nil {
		t.Fatalf("CountReplays: %v", err)
	}
	if total != 2 {
		t.Fatalf("CountReplays = %d, want the unfiltered corpus total of 2", total)
	}
	games, err := s.CountGames(context.Background(), GamesQuery{})
	if err != nil {
		t.Fatalf("CountGames: %v", err)
	}
	if games != 1 {
		t.Fatalf("CountGames = %d, want the filtered total of 1", games)
	}
}

func TestLibStorePlayersListAggregate(t *testing.T) {
	s := newTestLibStore(t,
		librarytest.Replay(
			librarytest.WithDate(daysAgoDate(5)),
			librarytest.WithPlayer("Flash", librarytest.APM(300, 200)),
			librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg), librarytest.APM(0, 0)),
		),
		librarytest.Replay(
			librarytest.WithDate(daysAgoDate(50)),
			librarytest.WithPlayer("flash", librarytest.APM(100, 80)),
			librarytest.WithPlayer("jaedong", librarytest.Race(library.RaceZerg), librarytest.APM(400, 300)),
		),
	)

	rows, err := s.ListPlayers(context.Background(), PlayersQuery{SortColumn: "player_name", SortDir: "ASC"}, 100, 0)
	if err != nil {
		t.Fatalf("ListPlayers: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want two player keys", rows)
	}
	byKey := map[string]WorkflowPlayersListRow{}
	for _, row := range rows {
		byKey[row.PlayerKey] = row
	}
	flash := byKey["flash"]
	if flash.GamesPlayed != 2 {
		t.Errorf("flash games = %d, want 2 across both name casings", flash.GamesPlayed)
	}
	if flash.PlayerName != "Flash" {
		t.Errorf("flash display name = %q, want the smallest raw name", flash.PlayerName)
	}
	if flash.AverageAPM != 200 {
		t.Errorf("flash APM = %v, want 200", flash.AverageAPM)
	}
	if flash.Race != "Terran" {
		t.Errorf("flash race = %q", flash.Race)
	}
	if flash.LastPlayedDaysAgo != 5 {
		t.Errorf("flash last played = %d days ago, want 5", flash.LastPlayedDaysAgo)
	}
	jaedong := byKey["jaedong"]
	if jaedong.AverageAPM != 400 {
		t.Errorf("jaedong APM = %v, want the zero-APM game excluded from the average", jaedong.AverageAPM)
	}
	if jaedong.Race != "Zerg" {
		t.Errorf("jaedong race = %q", jaedong.Race)
	}
}

// TestLibStorePlayersListRaceShareBoundary pins the strict 0.67 comparison:
// exactly two thirds and a bit is still Random.
func TestLibStorePlayersListRaceShareBoundary(t *testing.T) {
	build := func(protossGames int) *LibStore {
		replays := []*library.Replay{}
		for i := 0; i < 100; i++ {
			race := library.RaceTerran
			if i < protossGames {
				race = library.RaceProtoss
			}
			replays = append(replays, librarytest.Replay(librarytest.WithPlayer("Bisu", librarytest.Race(race))))
		}
		return newTestLibStore(t, replays...)
	}
	for _, tc := range []struct {
		protossGames int
		want         string
	}{
		{67, "Random"},
		{68, "Protoss"},
	} {
		rows, err := build(tc.protossGames).ListPlayers(context.Background(), PlayersQuery{}, 10, 0)
		if err != nil {
			t.Fatalf("ListPlayers: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("rows = %+v", rows)
		}
		if rows[0].Race != tc.want {
			t.Errorf("%d/100 protoss games = %q, want %q", tc.protossGames, rows[0].Race, tc.want)
		}
	}
}

func TestLibStorePlayersListFiltersAndTotalAgree(t *testing.T) {
	replays := []*library.Replay{}
	for i := 0; i < 5; i++ {
		replays = append(replays, librarytest.Replay(
			librarytest.WithDate(daysAgoDate(10)),
			librarytest.WithPlayer("Flash"),
		))
	}
	replays = append(replays,
		librarytest.Replay(librarytest.WithDate(daysAgoDate(60)), librarytest.WithPlayer("Bisu")),
		librarytest.Replay(librarytest.WithDate(daysAgoDate(400)), librarytest.WithPlayer("Stork")),
	)
	s := newTestLibStore(t, replays...)
	ctx := context.Background()

	assertAgrees := func(name string, query PlayersQuery, wantNames []string) {
		t.Helper()
		total, err := s.CountPlayers(ctx, query)
		if err != nil {
			t.Fatalf("%s: CountPlayers: %v", name, err)
		}
		rows, err := s.ListPlayers(ctx, query, 100, 0)
		if err != nil {
			t.Fatalf("%s: ListPlayers: %v", name, err)
		}
		if total != int64(len(rows)) {
			t.Fatalf("%s: CountPlayers = %d but ListPlayers returned %d", name, total, len(rows))
		}
		if len(rows) != len(wantNames) {
			t.Fatalf("%s: rows = %v, want %v", name, playersListNames(rows), wantNames)
		}
		got := map[string]bool{}
		for _, row := range rows {
			got[row.PlayerName] = true
		}
		for _, want := range wantNames {
			if !got[want] {
				t.Fatalf("%s: rows = %v, want %v", name, playersListNames(rows), wantNames)
			}
		}
	}

	assertAgrees("unfiltered", PlayersQuery{}, []string{"Flash", "Bisu", "Stork"})
	assertAgrees("five plus", PlayersQuery{OnlyFivePlus: true}, []string{"Flash"})
	assertAgrees("name contains", PlayersQuery{NameFilter: "las"}, []string{"Flash"})
	assertAgrees("name contains is case-insensitive", PlayersQuery{NameFilter: "FLASH"}, []string{"Flash"})
	assertAgrees("name miss", PlayersQuery{NameFilter: "nobody"}, nil)
	assertAgrees("last month", PlayersQuery{LastPlayed: []string{"1m"}}, []string{"Flash"})
	assertAgrees("last three months", PlayersQuery{LastPlayed: []string{"3m"}}, []string{"Flash", "Bisu"})
	assertAgrees("buckets OR", PlayersQuery{LastPlayed: []string{"1m", "3m"}}, []string{"Flash", "Bisu"})
	assertAgrees("bucket alias", PlayersQuery{LastPlayed: []string{"30d"}}, []string{"Flash"})
	assertAgrees("unknown bucket is a no-op", PlayersQuery{LastPlayed: []string{"nonsense"}}, []string{"Flash", "Bisu", "Stork"})
	assertAgrees("filters AND together", PlayersQuery{OnlyFivePlus: true, LastPlayed: []string{"3m"}}, []string{"Flash"})

	count1m, count3m, err := s.CountPlayersLastPlayedBuckets(ctx, PlayersQuery{})
	if err != nil {
		t.Fatalf("CountPlayersLastPlayedBuckets: %v", err)
	}
	if count1m != 1 || count3m != 2 {
		t.Fatalf("bucket counts = %d/%d, want 1/2", count1m, count3m)
	}
	// The bucket counts are computed over the same filtered rows as the page,
	// so an active bucket filter narrows them too.
	count1m, count3m, err = s.CountPlayersLastPlayedBuckets(ctx, PlayersQuery{LastPlayed: []string{"1m"}})
	if err != nil {
		t.Fatalf("CountPlayersLastPlayedBuckets: %v", err)
	}
	if count1m != 1 || count3m != 1 {
		t.Fatalf("bucket counts under a 1m filter = %d/%d, want 1/1", count1m, count3m)
	}
}

func TestLibStoreListPlayersSortsAndPages(t *testing.T) {
	s := newTestLibStore(t,
		librarytest.Replay(
			librarytest.WithDate(daysAgoDate(2)),
			librarytest.WithPlayer("Bisu", librarytest.Race(library.RaceProtoss), librarytest.APM(300, 200)),
		),
		librarytest.Replay(
			librarytest.WithDate(daysAgoDate(2)),
			librarytest.WithPlayer("Bisu", librarytest.Race(library.RaceProtoss), librarytest.APM(300, 200)),
		),
		librarytest.Replay(
			librarytest.WithDate(daysAgoDate(9)),
			librarytest.WithPlayer("Flash", librarytest.APM(100, 90)),
		),
		librarytest.Replay(
			librarytest.WithDate(daysAgoDate(40)),
			librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg), librarytest.APM(200, 150)),
		),
	)
	ctx := context.Background()

	cases := []struct {
		column string
		dir    string
		want   []string
	}{
		{"games_played", "DESC", []string{"Bisu", "Flash", "Jaedong"}},
		{"games_played", "ASC", []string{"Flash", "Jaedong", "Bisu"}},
		{"player_name", "ASC", []string{"Bisu", "Flash", "Jaedong"}},
		{"player_name", "DESC", []string{"Jaedong", "Flash", "Bisu"}},
		{"average_apm", "ASC", []string{"Flash", "Jaedong", "Bisu"}},
		{"average_apm", "DESC", []string{"Bisu", "Jaedong", "Flash"}},
		{"race", "ASC", []string{"Bisu", "Flash", "Jaedong"}},
		{"race", "DESC", []string{"Jaedong", "Flash", "Bisu"}},
		{"last_played_days_ago", "ASC", []string{"Bisu", "Flash", "Jaedong"}},
		{"last_played_days_ago", "DESC", []string{"Jaedong", "Flash", "Bisu"}},
	}
	for _, tc := range cases {
		rows, err := s.ListPlayers(ctx, PlayersQuery{SortColumn: tc.column, SortDir: tc.dir}, 100, 0)
		if err != nil {
			t.Fatalf("ListPlayers(%s %s): %v", tc.column, tc.dir, err)
		}
		got := playersListNames(rows)
		if len(got) != len(tc.want) {
			t.Fatalf("ListPlayers(%s %s) = %v, want %v", tc.column, tc.dir, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("ListPlayers(%s %s) = %v, want %v", tc.column, tc.dir, got, tc.want)
			}
		}
	}

	page, err := s.ListPlayers(ctx, PlayersQuery{SortColumn: "player_name", SortDir: "ASC"}, 2, 1)
	if err != nil {
		t.Fatalf("ListPlayers page: %v", err)
	}
	if got := playersListNames(page); len(got) != 2 || got[0] != "Flash" || got[1] != "Jaedong" {
		t.Fatalf("page = %v, want [Flash Jaedong]", got)
	}
	if page, err := s.ListPlayers(ctx, PlayersQuery{}, 2, 99); err != nil || page == nil || len(page) != 0 {
		t.Fatalf("past-the-end page = %v, %v", page, err)
	}
}

func TestLibStorePlayerPopulationsSkipObserversAndNonHumans(t *testing.T) {
	s := newTestLibStore(t, librarytest.Replay(
		librarytest.WithPlayer("Flash"),
		librarytest.WithPlayer("Artosis", librarytest.Observer()),
		librarytest.WithPlayer("Rescue", librarytest.Type(library.PlayerTypeRescuePassive)),
	))
	ctx := context.Background()

	rows, err := s.ListPlayers(ctx, PlayersQuery{}, 100, 0)
	if err != nil {
		t.Fatalf("ListPlayers: %v", err)
	}
	if len(rows) != 1 || rows[0].PlayerKey != "flash" {
		t.Fatalf("rows = %+v, want only the human non-observer", rows)
	}
	colors, err := s.ListTopPlayerColorRows(ctx)
	if err != nil {
		t.Fatalf("ListTopPlayerColorRows: %v", err)
	}
	// The colour rows count every non-observer slot, non-human ones included.
	if len(colors) != 2 {
		t.Fatalf("colour rows = %+v, want the two non-observer slots", colors)
	}
	for _, row := range colors {
		if row.PlayerKey == "artosis" {
			t.Fatalf("an observer reached the colour rows: %+v", colors)
		}
	}
}

func TestLibStoreListTopPlayerColorRowsRanksAndCaps(t *testing.T) {
	replays := []*library.Replay{
		librarytest.Replay(librarytest.WithPlayer("Flash")),
		librarytest.Replay(librarytest.WithPlayer("Flash")),
	}
	for i := 0; i < 20; i++ {
		replays = append(replays, librarytest.Replay(librarytest.WithPlayer(string(rune('a'+i))+"player")))
	}
	s := newTestLibStore(t, replays...)

	rows, err := s.ListTopPlayerColorRows(context.Background())
	if err != nil {
		t.Fatalf("ListTopPlayerColorRows: %v", err)
	}
	if len(rows) != 15 {
		t.Fatalf("rows = %d, want the 15-row cap", len(rows))
	}
	if rows[0].PlayerKey != "flash" || rows[0].Games != 2 {
		t.Fatalf("top row = %+v, want flash with 2 games", rows[0])
	}
	if rows[1].PlayerKey != "aplayer" {
		t.Fatalf("second row = %+v, want the key tie-break", rows[1])
	}
}

func TestLibStoreListPlayerApmAggregates(t *testing.T) {
	s := newTestLibStore(t,
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.APM(300, 200)),
			librarytest.WithPlayer("Silent", librarytest.APM(0, 0)),
		),
		librarytest.Replay(librarytest.WithPlayer("Flash", librarytest.APM(100, 80))),
	)
	ctx := context.Background()

	rows, err := s.ListPlayerApmAggregates(ctx, 1)
	if err != nil {
		t.Fatalf("ListPlayerApmAggregates: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want the zero-APM player dropped", rows)
	}
	if rows[0].PlayerKey != "flash" || rows[0].GamesPlayed != 2 || rows[0].AverageAPM != 200 {
		t.Fatalf("row = %+v", rows[0])
	}
	if rows[0].PlayerName != "Flash" {
		t.Errorf("name = %q", rows[0].PlayerName)
	}
	rows, err = s.ListPlayerApmAggregates(ctx, 3)
	if err != nil || len(rows) != 0 {
		t.Fatalf("minGames = 3 gave %+v, %v", rows, err)
	}
}

func TestLibStoreListHotkeyGamesRateByPlayer(t *testing.T) {
	s := newTestLibStore(t,
		librarytest.Replay(
			librarytest.WithPlayer("Flash"),
			librarytest.WithPlayer("Jaedong", librarytest.Race(library.RaceZerg)),
			librarytest.WithHotkeyStream(0, []byte{1, 2, 3}),
		),
		librarytest.Replay(librarytest.WithPlayer("Flash")),
	)

	rates, err := s.ListHotkeyGamesRateByPlayer(context.Background())
	if err != nil {
		t.Fatalf("ListHotkeyGamesRateByPlayer: %v", err)
	}
	if rates["flash"] != 50 {
		t.Errorf("flash rate = %v, want 50 (one of two games)", rates["flash"])
	}
	if rates["jaedong"] != 0 {
		t.Errorf("jaedong rate = %v, want 0", rates["jaedong"])
	}
}

func TestLibStorePlayersListDaysAgoTruncates(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		lastPlayed time.Time
		want       int64
	}{
		{now, 0},
		{now.Add(-23 * time.Hour), 0},
		{now.Add(-25 * time.Hour), 1},
		{now.Add(-30 * 24 * time.Hour), 30},
		{time.Time{}, 0},
	}
	for _, tc := range cases {
		if got := playersListDaysAgo(tc.lastPlayed, now); got != tc.want {
			t.Errorf("playersListDaysAgo(%v) = %d, want %d", tc.lastPlayed, got, tc.want)
		}
	}
}
