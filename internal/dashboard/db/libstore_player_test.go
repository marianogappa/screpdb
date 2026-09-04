package db

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/fpvec"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/load"
)

func TestLibStoreGetPlayerNameByKey(t *testing.T) {
	s := newTestLibStore(t,
		melee("a"),
		librarytest.Replay(
			librarytest.WithPlayer("flash", librarytest.Team(1)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
		librarytest.Replay(
			librarytest.WithPlayer("Ghost", librarytest.Team(1), librarytest.Observer()),
			librarytest.WithPlayer("Rush", librarytest.Team(2), librarytest.Type(library.PlayerTypeRescuePassive)),
			librarytest.WithPlayer("Human", librarytest.Team(3)),
		),
	)

	// MIN(name) over the matching slots, trimmed.
	if got, _ := s.GetPlayerNameByKey(context.Background(), "FLASH"); got != "Flash" {
		t.Fatalf("GetPlayerNameByKey(flash) = %q, want Flash", got)
	}
	if got, _ := s.GetPlayerNameByKey(context.Background(), "ghost"); got != "" {
		t.Fatalf("observers must not resolve a name, got %q", got)
	}
	if got, _ := s.GetPlayerNameByKey(context.Background(), "rush"); got != "" {
		t.Fatalf("non-human slots must not resolve a name, got %q", got)
	}
	if got, _ := s.GetPlayerNameByKey(context.Background(), "nobody"); got != "" {
		t.Fatalf("unknown key = %q, want empty", got)
	}
}

func TestLibStoreGetPlayerOverviewSummary(t *testing.T) {
	s := newTestLibStore(t,
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Winner(), librarytest.APM(200, 150)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.APM(0, 0)),
		),
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.APM(100, 50)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.APM(80, 60)),
		),
	)

	got, err := s.GetPlayerOverviewSummary(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	want := &PlayerOverviewSummaryRow{PlayerName: "Flash", GamesPlayed: 2, Wins: 1, AverageAPM: 150, AverageEAPM: 100}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("summary = %+v, want %+v", got, want)
	}

	// Zero APM games are excluded from the average, not counted as zero.
	bisu, err := s.GetPlayerOverviewSummary(context.Background(), "bisu")
	if err != nil {
		t.Fatal(err)
	}
	if bisu.GamesPlayed != 2 || bisu.AverageAPM != 80 || bisu.AverageEAPM != 60 {
		t.Fatalf("bisu = %+v", bisu)
	}

	empty, err := s.GetPlayerOverviewSummary(context.Background(), "nobody")
	if err != nil {
		t.Fatal(err)
	}
	if *empty != (PlayerOverviewSummaryRow{}) {
		t.Fatalf("unknown key = %+v, want zero row", empty)
	}
}

func TestLibStoreListPlayerRecentGames(t *testing.T) {
	replays := make([]*library.Replay, 0, 12)
	for i := 0; i < 12; i++ {
		replays = append(replays, melee("game"))
	}
	s := newTestLibStore(t, replays...)

	rows, err := s.ListPlayerRecentGames(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 10 {
		t.Fatalf("got %d rows, want the LIMIT 10", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].ReplayDate < rows[i].ReplayDate {
			t.Fatal("rows must be newest first")
		}
	}
}

func TestLibStoreListPlayerRecentGamesLabels(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithMap("Polypoid"),
		librarytest.WithGameType("Top vs Bottom"),
		librarytest.WithMatchup("TvP"),
		librarytest.WithPlayer("Zerg2", librarytest.Team(2), librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Winner()),
		librarytest.WithPlayer("Watcher", librarytest.Team(1), librarytest.Observer()),
		librarytest.WithPlayer("Bot", librarytest.Team(2), librarytest.Type(library.PlayerTypeRescuePassive)),
		librarytest.WithPlayer("Ally", librarytest.Team(1), librarytest.Winner()),
	)
	s := newTestLibStore(t, r)

	rows, err := s.ListPlayerRecentGames(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	row := rows[0]
	// Team ascending, then slot order; observers and non-humans excluded.
	if row.PlayersLabel != "Flash, Ally, Zerg2" {
		t.Fatalf("players label = %q", row.PlayersLabel)
	}
	if row.WinnersLabel != "Flash, Ally" {
		t.Fatalf("winners label = %q", row.WinnersLabel)
	}
	if row.MapName != "Polypoid" || row.GameType != "Top vs Bottom" || row.Matchup != "TvP" {
		t.Fatalf("row = %+v", row)
	}
	if row.DurationSeconds != 900 || row.MapKind != "Regular" || row.FileName == "" {
		t.Fatalf("row = %+v", row)
	}
}

func TestLibStoreListPlayerMatchups(t *testing.T) {
	oneOnOne := func(ownRace library.Race, oppRace library.Race, won bool) *library.Replay {
		selfOpts := []librarytest.PlayerOption{librarytest.Team(1), librarytest.Race(ownRace)}
		if won {
			selfOpts = append(selfOpts, librarytest.Winner())
		}
		r := librarytest.Replay(
			librarytest.WithPlayer("Flash", selfOpts...),
			librarytest.WithPlayer("Opp", librarytest.Team(2), librarytest.Race(oppRace)),
		)
		r.Flags |= library.FlagIsOneOnOne
		return r
	}
	team := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Ally", librarytest.Team(1), librarytest.Race(library.RaceZerg)),
		librarytest.WithPlayer("Opp1", librarytest.Team(2), librarytest.Race(library.RaceProtoss)),
		librarytest.WithPlayer("Opp2", librarytest.Team(2), librarytest.Race(library.RaceProtoss)),
	)
	s := newTestLibStore(t,
		oneOnOne(library.RaceTerran, library.RaceProtoss, true),
		oneOnOne(library.RaceTerran, library.RaceProtoss, false),
		oneOnOne(library.RaceTerran, library.RaceZerg, true),
		team,
	)

	rows, err := s.ListPlayerMatchups(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	want := []PlayerMatchupRow{
		{OwnRace: "Terran", OppRace: "Protoss", Games: 2, Wins: 1},
		{OwnRace: "Terran", OppRace: "Zerg", Games: 1, Wins: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("matchups = %+v, want %+v (team games must be excluded)", rows, want)
	}
	if rows, _ := s.ListPlayerMatchups(context.Background(), "nobody"); len(rows) != 0 {
		t.Fatalf("unknown key = %+v", rows)
	}
}

func TestLibStoreListRaceSections(t *testing.T) {
	s := newTestLibStore(t,
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran), librarytest.Winner()),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceZerg), librarytest.Winner()),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
		librarytest.Replay(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceProtoss), librarytest.Observer()),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
	)

	rows, err := s.ListRaceSections(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	want := []RaceSectionRow{
		{Race: "Terran", GameCount: 2, Wins: 1},
		{Race: "Zerg", GameCount: 1, Wins: 1},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("race sections = %+v, want %+v", rows, want)
	}
}

func TestLibStoreListRaceOrderRows(t *testing.T) {
	r := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.Race(library.RaceProtoss)),
		librarytest.WithProd(0, 300, library.ProdUpgrade, "Terran Infantry Weapons"),
		librarytest.WithProd(0, 120, library.ProdTech, "Stim Packs"),
		librarytest.WithProd(0, 60, library.ProdBuild, "Barracks"),
		librarytest.WithProd(1, 200, library.ProdTech, "Psionic Storm"),
	)
	observed := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Observer()),
		librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		librarytest.WithProd(0, 90, library.ProdTech, "Lockdown"),
	)
	s := newTestLibStore(t, r, observed)

	rows, err := s.ListRaceOrderRows(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (Build dropped, observer slot dropped): %+v", len(rows), rows)
	}
	if rows[0].ActionType != "Tech" || rows[0].TechName == nil || *rows[0].TechName != "Stim Packs" || rows[0].Second != 120 {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if rows[1].ActionType != "Upgrade" || rows[1].UpgradeName == nil || *rows[1].UpgradeName != "Terran Infantry Weapons" {
		t.Fatalf("row 1 = %+v", rows[1])
	}
	if rows[0].Race != "Terran" || rows[0].PlayerID != r.PlayerID(0) {
		t.Fatalf("row 0 identity = %+v", rows[0])
	}
	if rows, _ := s.ListRaceOrderRows(context.Background(), "nobody"); len(rows) != 0 {
		t.Fatalf("unknown key = %+v", rows)
	}
}

func TestLibStoreListMatchupOrderRows(t *testing.T) {
	solo := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.Race(library.RaceProtoss)),
		librarytest.WithProd(0, 120, library.ProdTech, "Stim Packs"),
		librarytest.WithProd(0, 300, library.ProdUpgrade, "Terran Infantry Weapons"),
	)
	solo.Flags |= library.FlagIsOneOnOne
	team := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Ally", librarytest.Team(1)),
		librarytest.WithPlayer("Opp1", librarytest.Team(2)),
		librarytest.WithPlayer("Opp2", librarytest.Team(2)),
		librarytest.WithProd(0, 100, library.ProdTech, "Lockdown"),
	)
	s := newTestLibStore(t, solo, team)

	rows, err := s.ListMatchupOrderRows(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (team game excluded): %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.OwnRace != "Terran" || row.OppRace != "Protoss" || row.ReplayID != solo.ID {
			t.Fatalf("row = %+v", row)
		}
	}
	if rows[0].Second != 120 || rows[1].Second != 300 {
		t.Fatalf("rows must be second ascending: %+v", rows)
	}
}

func TestLibStoreListPlayerChatRows(t *testing.T) {
	older := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		librarytest.WithChat(0, 10, "old glhf"),
	)
	newer := librarytest.Replay(
		librarytest.WithDate(librarytest.BaseDate),
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		librarytest.WithChat(0, 5, "first"),
		librarytest.WithChat(0, 50, "last"),
		librarytest.WithChat(0, 60, "   "),
		librarytest.WithChat(1, 20, "not mine"),
	)
	s := newTestLibStore(t, older, newer)

	rows, err := s.ListPlayerChatRows(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	want := []PlayerChatRow{
		{ReplayID: newer.ID, Message: "last"},
		{ReplayID: newer.ID, Message: "first"},
		{ReplayID: older.ID, Message: "old glhf"},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("chat rows = %+v, want %+v", rows, want)
	}
	if rows, _ := s.ListPlayerChatRows(context.Background(), "nobody"); len(rows) != 0 {
		t.Fatalf("unknown key = %+v", rows)
	}
}

func TestLibStoreListPlayerFirstExpansionTimings(t *testing.T) {
	regular := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		librarytest.WithEvent("expansion", 400, 0, library.NoPlayer),
		librarytest.WithEvent("expansion", 250, 0, library.NoPlayer),
		librarytest.WithEvent("expansion", 100, 1, library.NoPlayer),
		librarytest.WithEvent("rush", 60, 0, library.NoPlayer),
	)
	money := librarytest.Replay(
		librarytest.WithMapKind(library.MapKindMoney),
		librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(library.RaceTerran)),
		librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		librarytest.WithEvent("expansion", 80, 0, library.NoPlayer),
	)
	ums := librarytest.Replay(
		librarytest.WithMapKind(library.MapKindUseMapSettings),
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithEvent("expansion", 30, 0, library.NoPlayer),
	)
	s := newTestLibStore(t, regular, money, ums)

	rows, err := s.ListPlayerFirstExpansionTimings(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	want := []PlayerFirstExpansionTimingRow{
		{Race: "Terran", MapKind: "Money", ReplayID: money.ID, FirstExpansionSecond: 80},
		{Race: "Terran", MapKind: "Regular", ReplayID: regular.ID, FirstExpansionSecond: 250},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("timings = %+v, want %+v", rows, want)
	}
	if rows, _ := s.ListPlayerFirstExpansionTimings(context.Background(), "nobody"); len(rows) != 0 {
		t.Fatalf("unknown key = %+v", rows)
	}
}

func TestLibStoreFingerprintCoverageAndVectors(t *testing.T) {
	solo := func(vector []float64, race library.Race, opts ...librarytest.Option) *library.Replay {
		base := []librarytest.Option{
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Race(race)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2), librarytest.Race(library.RaceProtoss)),
			librarytest.WithFingerprint(0, vector),
		}
		r := librarytest.Replay(append(base, opts...)...)
		r.Flags |= library.FlagIsOneOnOne
		return r
	}
	oldest := solo([]float64{1, 2}, library.RaceTerran, librarytest.WithDate(librarytest.BaseDate.Add(-time.Hour)))
	newest := solo([]float64{3, 4}, library.RaceZerg, librarytest.WithDate(librarytest.BaseDate))
	moneyMap := solo([]float64{9, 9}, library.RaceTerran, librarytest.WithMapKind(library.MapKindMoney))
	teamGame := librarytest.Replay(
		librarytest.WithPlayer("Flash", librarytest.Team(1)),
		librarytest.WithPlayer("Ally", librarytest.Team(1)),
		librarytest.WithPlayer("Opp1", librarytest.Team(2)),
		librarytest.WithPlayer("Opp2", librarytest.Team(2)),
		librarytest.WithFingerprint(0, []float64{7, 7}),
	)
	noVector := melee("plain")
	s := newTestLibStore(t, oldest, newest, moneyMap, teamGame, noVector)

	// Coverage ignores the 1v1 and map-kind gates the vector list applies.
	coverage, err := s.GetPlayerFingerprintCoverage(context.Background(), "flash", 1)
	if err != nil {
		t.Fatal(err)
	}
	if coverage != 4 {
		t.Fatalf("coverage = %d, want 4", coverage)
	}
	if coverage, _ := s.GetPlayerFingerprintCoverage(context.Background(), "flash", 99); coverage != 0 {
		t.Fatalf("other feature version = %d, want 0", coverage)
	}

	rows, err := s.ListPlayerFingerprintVectors(context.Background(), "flash", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d vectors, want 2 (money map and team game gated out): %+v", len(rows), rows)
	}
	if rows[0].Race != "Terran" || rows[1].Race != "Zerg" {
		t.Fatalf("vectors must be replay-date ascending: %+v", rows)
	}
	decoded, err := fpvec.Decode(rows[0].Vector)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, []float64{1, 2}) {
		t.Fatalf("decoded vector = %v", decoded)
	}
	if rows, _ := s.ListPlayerFingerprintVectors(context.Background(), "flash", 99); len(rows) != 0 {
		t.Fatalf("other feature version = %+v", rows)
	}
}

func TestLibStoreCountPlayerBnetGames(t *testing.T) {
	bnet := func(opts ...librarytest.Option) *library.Replay {
		r := librarytest.Replay(opts...)
		r.GameSource = library.Strings.Intern("AssumedBattleNet")
		return r
	}
	s := newTestLibStore(t,
		bnet(
			librarytest.WithPlayer("Flash", librarytest.Team(1)),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
		bnet(
			librarytest.WithPlayer("Flash", librarytest.Team(1), librarytest.Observer()),
			librarytest.WithPlayer("Bisu", librarytest.Team(2)),
		),
		melee("local"),
	)

	got, err := s.CountPlayerBnetGames(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("bnet games = %d, want 1 (observer slot excluded, local game excluded)", got)
	}
	if got, _ := s.CountPlayerBnetGames(context.Background(), "nobody"); got != 0 {
		t.Fatalf("unknown key = %d", got)
	}
}

func TestLibStorePlayerReadsHonourTheGlobalFilter(t *testing.T) {
	long := melee("long")
	short := melee("short", librarytest.WithDuration(30))
	s := newTestLibStore(t, long, short)

	summary, err := s.GetPlayerOverviewSummary(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if summary.GamesPlayed != 1 {
		t.Fatalf("games = %d, want 1 (short game filtered out)", summary.GamesPlayed)
	}
	rows, err := s.ListPlayerRecentGames(context.Background(), "flash")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ReplayID != long.ID {
		t.Fatalf("recent games = %+v", rows)
	}
}

// TestLibStoreRealCorpusReadsAreSelfConsistent runs every ported read against
// the real 4-replay corpus loaded through the folder loader, so the fixture
// tests are backed by at least one pass over replays nobody hand-shaped.
func TestLibStoreRealCorpusReadsAreSelfConsistent(t *testing.T) {
	// The scanner resolves paths against the folder, so a relative folder
	// makes it reject every file under a dot-prefixed ancestor directory.
	folder, err := filepath.Abs(filepath.Join("..", "..", "testdata", "replays"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(folder); err != nil {
		t.Skipf("no replay corpus: %v", err)
	}
	lib := library.New(library.Options{CoalesceRecords: 1, CoalesceDelay: time.Millisecond})
	t.Cleanup(lib.Close)
	loader := load.New(lib, load.Options{Folder: folder, Generation: 1, Workers: 2})
	if err := loader.Run(context.Background()); err != nil {
		t.Fatalf("load corpus: %v", err)
	}
	lib.Flush()
	s := NewLibStore(lib, nil, nil)
	if snap := lib.Snapshot(); snap.Len() == 0 {
		t.Fatalf("loader committed nothing: progress = %+v", loader.Progress())
	}
	view := lib.View()
	if view.Len() == 0 {
		t.Skip("the whole corpus is filtered out")
	}

	ctx := context.Background()
	keys := view.PlayerKeys()
	if len(keys) == 0 {
		t.Fatal("filtered corpus has no player keys")
	}
	recentTotal, orderTotal := 0, 0
	for _, key := range keys {
		refs := s.playerGames(key)
		if len(refs) == 0 {
			t.Fatalf("%q: PlayerKeys listed a key with no games", key)
		}

		summary, err := s.GetPlayerOverviewSummary(ctx, key)
		if err != nil {
			t.Fatalf("%q: overview: %v", key, err)
		}
		if summary.GamesPlayed > int64(len(refs)) || summary.Wins > summary.GamesPlayed {
			t.Fatalf("%q: overview = %+v over %d slots", key, summary, len(refs))
		}

		name, err := s.GetPlayerNameByKey(ctx, key)
		if err != nil {
			t.Fatalf("%q: name: %v", key, err)
		}
		if summary.GamesPlayed > 0 && normalizeKey(name) != key {
			t.Fatalf("%q: resolved name %q", key, name)
		}

		recent, err := s.ListPlayerRecentGames(ctx, key)
		if err != nil {
			t.Fatalf("%q: recent games: %v", key, err)
		}
		if len(recent) > playerRecentGamesLimit || int64(len(recent)) > summary.GamesPlayed {
			t.Fatalf("%q: %d recent games for %d played", key, len(recent), summary.GamesPlayed)
		}
		recentTotal += len(recent)
		for i, row := range recent {
			if _, ok := view.ByID(row.ReplayID); !ok {
				t.Fatalf("%q: recent game %d is not in the view", key, row.ReplayID)
			}
			if i > 0 && recent[i-1].ReplayDate < row.ReplayDate {
				t.Fatalf("%q: recent games are not newest first", key)
			}
		}

		sections, err := s.ListRaceSections(ctx, key)
		if err != nil {
			t.Fatalf("%q: race sections: %v", key, err)
		}
		total := int64(0)
		for i, row := range sections {
			total += row.GameCount
			if i > 0 && sections[i-1].GameCount < row.GameCount {
				t.Fatalf("%q: race sections are not game-count descending", key)
			}
		}
		if total != summary.GamesPlayed {
			t.Fatalf("%q: race sections total %d, overview says %d", key, total, summary.GamesPlayed)
		}

		matchups, err := s.ListPlayerMatchups(ctx, key)
		if err != nil {
			t.Fatalf("%q: matchups: %v", key, err)
		}
		for i, row := range matchups {
			if row.Wins > row.Games || row.Games <= 0 {
				t.Fatalf("%q: matchup %+v", key, row)
			}
			if i > 0 && matchups[i-1].Games < row.Games {
				t.Fatalf("%q: matchups are not game-count descending", key)
			}
		}

		raceOrders, err := s.ListRaceOrderRows(ctx, key)
		if err != nil {
			t.Fatalf("%q: race orders: %v", key, err)
		}
		orderTotal += len(raceOrders)
		for i, row := range raceOrders {
			assertOrderRow(t, key, view, row.PlayerID, row.ActionType, row.TechName, row.UpgradeName)
			if i > 0 && raceOrders[i-1].PlayerID == row.PlayerID && raceOrders[i-1].Second > row.Second {
				t.Fatalf("%q: race orders are not second ascending within a game", key)
			}
		}

		matchupOrders, err := s.ListMatchupOrderRows(ctx, key)
		if err != nil {
			t.Fatalf("%q: matchup orders: %v", key, err)
		}
		for i, row := range matchupOrders {
			assertOrderRow(t, key, view, row.PlayerID, row.ActionType, row.TechName, row.UpgradeName)
			replayID, _ := library.SplitPlayerID(row.PlayerID)
			if replayID != row.ReplayID {
				t.Fatalf("%q: matchup order row %+v has a mismatched replay id", key, row)
			}
			if i > 0 && matchupOrders[i-1].PlayerID == row.PlayerID && matchupOrders[i-1].Second > row.Second {
				t.Fatalf("%q: matchup orders are not second ascending within a game", key)
			}
		}

		chat, err := s.ListPlayerChatRows(ctx, key)
		if err != nil {
			t.Fatalf("%q: chat: %v", key, err)
		}
		for _, row := range chat {
			if _, ok := view.ByID(row.ReplayID); !ok {
				t.Fatalf("%q: chat row points at replay %d outside the view", key, row.ReplayID)
			}
			if strings.TrimSpace(row.Message) == "" {
				t.Fatalf("%q: chat row has an empty message", key)
			}
		}

		expansions, err := s.ListPlayerFirstExpansionTimings(ctx, key)
		if err != nil {
			t.Fatalf("%q: expansions: %v", key, err)
		}
		for _, row := range expansions {
			r, ok := view.ByID(row.ReplayID)
			if !ok {
				t.Fatalf("%q: expansion row points at replay %d outside the view", key, row.ReplayID)
			}
			if row.MapKind == library.MapKindUseMapSettings.String() {
				t.Fatalf("%q: UseMapSettings replay must not report an expansion", key)
			}
			if row.FirstExpansionSecond < 0 || row.FirstExpansionSecond > int64(r.Duration) {
				t.Fatalf("%q: expansion second %d outside a %ds game", key, row.FirstExpansionSecond, r.Duration)
			}
		}

		coverage, err := s.GetPlayerFingerprintCoverage(ctx, key, 1)
		if err != nil {
			t.Fatalf("%q: coverage: %v", key, err)
		}
		if coverage < 0 || coverage > int64(len(refs)) {
			t.Fatalf("%q: coverage %d over %d slots", key, coverage, len(refs))
		}
		vectors, err := s.ListPlayerFingerprintVectors(ctx, key, 1)
		if err != nil {
			t.Fatalf("%q: vectors: %v", key, err)
		}
		for _, row := range vectors {
			if _, err := fpvec.Decode(row.Vector); err != nil {
				t.Fatalf("%q: vector does not decode: %v", key, err)
			}
		}

		if _, err := s.CountPlayerBnetGames(ctx, key); err != nil {
			t.Fatalf("%q: bnet games: %v", key, err)
		}

		streams, err := s.ListPlayerHotkeyStreamsByKey(ctx, key)
		if err != nil {
			t.Fatalf("%q: hotkey streams: %v", key, err)
		}
		if len(streams) > hotkeySignatureMaxGames {
			t.Fatalf("%q: %d hotkey streams exceeds the cap", key, len(streams))
		}
		for _, row := range streams {
			if len(row.HotkeyStream) == 0 || row.DurationSeconds < hotkeySignatureMinDuration {
				t.Fatalf("%q: hotkey stream row %+v", key, row)
			}
		}
	}

	if recentTotal == 0 || orderTotal == 0 {
		t.Fatalf("the corpus produced %d recent games and %d order rows; the assertions above are vacuous", recentTotal, orderTotal)
	}

	replayIDs := make([]int64, 0, view.Len())
	for _, r := range view.Replays() {
		replayIDs = append(replayIDs, r.ID)

		hotkeyRows, err := s.ListReplayPlayerHotkeyStreams(ctx, r.ID)
		if err != nil {
			t.Fatalf("replay %d: hotkey streams: %v", r.ID, err)
		}
		for i, row := range hotkeyRows {
			one, err := s.GetReplayPlayerHotkeyStream(ctx, r.ID, row.PlayerID)
			if err != nil {
				t.Fatalf("replay %d: player %d: %v", r.ID, row.PlayerID, err)
			}
			if one.Name != row.Name || one.Race != row.Race {
				t.Fatalf("replay %d: player %d disagrees across reads", r.ID, row.PlayerID)
			}
			if i > 0 && hotkeyRows[i-1].Team > row.Team {
				t.Fatalf("replay %d: hotkey rows are not team ascending", r.ID)
			}
		}
	}

	sessionRows, err := s.ListReplaysByIDs(ctx, replayIDs)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessionRows) != len(replayIDs) {
		t.Fatalf("ListReplaysByIDs returned %d of %d view replays", len(sessionRows), len(replayIDs))
	}
	for i := 1; i < len(sessionRows); i++ {
		if sessionRows[i-1].ReplayDate < sessionRows[i].ReplayDate {
			t.Fatal("ListReplaysByIDs is not replay-date descending")
		}
	}

	apmRows, err := s.ListPlayerAPMByReplayIDs(ctx, replayIDs)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range apmRows {
		if _, ok := view.ByID(row.ReplayID); !ok {
			t.Fatalf("apm row points at replay %d outside the view", row.ReplayID)
		}
	}

	candidates, err := s.ListRecentAutosaveGamesForPlayers(ctx, keys, 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) == 0 {
		t.Fatal("every corpus player key should yield session candidates")
	}
	for i, row := range candidates {
		if row.FilePath == "" || row.PlayerName == "" {
			t.Fatalf("session candidate = %+v", row)
		}
		if i > 0 && candidates[i-1].ReplayDate < row.ReplayDate {
			t.Fatal("session candidates are not newest first")
		}
	}
}

func assertOrderRow(t *testing.T, key string, view *library.View, playerID int64, actionType string, techName, upgradeName *string) {
	t.Helper()
	replayID, ordinal := library.SplitPlayerID(playerID)
	r, ok := view.ByID(replayID)
	if !ok {
		t.Fatalf("%q: order row points at replay %d outside the view", key, replayID)
	}
	if int(ordinal) >= len(r.Players) {
		t.Fatalf("%q: order row ordinal %d out of range", key, ordinal)
	}
	switch actionType {
	case orderActionTech:
		if techName == nil || *techName == "" || upgradeName != nil {
			t.Fatalf("%q: Tech row must carry only a tech name", key)
		}
	case orderActionUpgrade:
		if upgradeName == nil || *upgradeName == "" || techName != nil {
			t.Fatalf("%q: Upgrade row must carry only an upgrade name", key)
		}
	default:
		t.Fatalf("%q: unexpected action type %q", key, actionType)
	}
}
