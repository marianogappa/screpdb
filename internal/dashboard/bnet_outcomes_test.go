package dashboard

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	dashboarddb "github.com/marianogappa/screpdb/internal/dashboard/db"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/models"
)

var outcomeGameStart = time.Date(2026, 10, 25, 18, 6, 39, 0, time.UTC)

func outcomeReplay() *library.Replay {
	r := librarytest.Replay(
		librarytest.WithDate(outcomeGameStart),
		librarytest.WithDuration(1059),
		librarytest.WithPlayer("chobo86"),
		librarytest.WithPlayer("cazadorXD"),
		librarytest.WithPlayer("Watcher", librarytest.Observer()),
	)
	r.Players[1].Flags |= library.PlayerLeft
	r.Players[1].LeaveSec = 996
	return r
}

func outcomeGame(id string, created time.Time, players ...persist.BnetGamePlayer) persist.BnetGame {
	return persist.BnetGame{GameID: id, CreateTime: created, Players: players}
}

func TestBnetGameForMatchesTheSamePeopleAtTheSameTime(t *testing.T) {
	r := outcomeReplay()
	game := outcomeGame("1", outcomeGameStart.Add(17*time.Minute),
		persist.BnetGamePlayer{Toon: "Chobo86", Result: persist.BnetResultWin},
		persist.BnetGamePlayer{Toon: "cazadorXD"})
	elsewhere := outcomeGame("2", outcomeGameStart.Add(-3*time.Hour), game.Players...)
	strangers := outcomeGame("3", game.CreateTime, persist.BnetGamePlayer{Toon: "chobo86"}, persist.BnetGamePlayer{Toon: "someone"})

	got, ok := bnetGameFor(r, indexBnetGames([]persist.BnetGame{game, elsewhere, strangers}))
	if !ok || got.GameID != "1" {
		t.Fatalf("got %+v %v", got, ok)
	}
	if results := bnetResultsOf(got); len(results) != 1 || results[0] != (library.BnetResult{Key: "chobo86", Outcome: models.OutcomeWon}) {
		t.Fatalf("results %+v", results)
	}
}

// A rematch of the same two players lands in the same window; the time each spent in the game tells which is which.
func TestBnetGameForTellsRematchesApartByTimePlayed(t *testing.T) {
	r := outcomeReplay()
	this := outcomeGame("this", outcomeGameStart.Add(18*time.Minute),
		persist.BnetGamePlayer{Toon: "chobo86", Seconds: 1117, Result: persist.BnetResultWin},
		persist.BnetGamePlayer{Toon: "cazadorXD", Seconds: 1051})
	next := outcomeGame("next", outcomeGameStart.Add(25*time.Minute),
		persist.BnetGamePlayer{Toon: "chobo86", Seconds: 300, Result: persist.BnetResultLoss},
		persist.BnetGamePlayer{Toon: "cazadorXD"})
	got, ok := bnetGameFor(r, indexBnetGames([]persist.BnetGame{this, next}))
	if !ok || got.GameID != "this" {
		t.Fatalf("got %+v %v", got, ok)
	}

	untimed := outcomeGame("untimed", next.CreateTime, persist.BnetGamePlayer{Toon: "chobo86"}, persist.BnetGamePlayer{Toon: "cazadorXD"})
	if got, ok := bnetGameFor(r, indexBnetGames([]persist.BnetGame{this, untimed})); ok {
		t.Fatalf("two candidates the times cannot separate matched %q", got.GameID)
	}
	if got, ok := bnetGameFor(r, indexBnetGames([]persist.BnetGame{next})); ok {
		t.Fatalf("a game whose times contradict the replay matched %q", got.GameID)
	}
}

// An 8-player game from the user's library, whose Battle.net times run 1.2% to 5.7% longer than the
// replay's: too far apart for matching by players and times, so only the file's MD5 can name the game.
func lagStretchedGame(t *testing.T, path string) (*library.Replay, persist.BnetGame) {
	t.Helper()
	start := time.Date(2026, 10, 18, 16, 15, 31, 0, time.UTC)
	r := librarytest.Replay(
		librarytest.WithPath(path, start),
		librarytest.WithDate(start),
		librarytest.WithDuration(2591),
		librarytest.WithPlayer("chobo86"),
		librarytest.WithPlayer("0lipower90"),
		librarytest.WithPlayer("tk:kot0"),
		librarytest.WithPlayer("HUSKY_LOKI"),
	)
	r.Players[1].Flags |= library.PlayerLeft
	r.Players[1].LeaveSec = 470
	r.Players[2].Flags |= library.PlayerLeft
	r.Players[2].LeaveSec = 741
	game := outcomeGame("1734417998", start.Add(43*time.Minute),
		persist.BnetGamePlayer{Toon: "chobo86", Seconds: 2623, Result: persist.BnetResultLoss},
		persist.BnetGamePlayer{Toon: "0lipower90", Seconds: 497, Result: persist.BnetResultLoss},
		persist.BnetGamePlayer{Toon: "tk:kot0", Seconds: 770, Result: persist.BnetResultLoss},
		persist.BnetGamePlayer{Toon: "HUSKY_LOKI", Seconds: 2626, Result: persist.BnetResultWin})
	if _, ok := bnetGameFor(r, indexBnetGames([]persist.BnetGame{game})); ok {
		t.Fatal("the fixture no longer needs the MD5 to match")
	}
	return r, game
}

func outcomeDashboard(t *testing.T, r *library.Replay, games ...persist.BnetGame) (*Dashboard, *library.Library, string) {
	t.Helper()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	lib.Add(0, r)
	lib.Flush()
	root := t.TempDir()
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	store := dashboarddb.NewLibStore(lib, persist.NewBnetCache(root), nil)
	archive := persist.NewBnetGameArchive(root)
	store.SetGameArchive(archive)
	if err := archive.Upsert(games); err != nil {
		t.Fatal(err)
	}
	return &Dashboard{ctx: context.Background(), dbStore: store, library: &libraryRuntime{lib: lib, root: root}}, lib, root
}

func writeReplayFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := iofacade.AllowDir(filepath.Dir(path)); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	if err := iofacade.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestResolveBnetOutcomesMatchesTheOwnReplayByMD5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "205902,(8)Big Game Hunters.rep")
	sum := writeReplayFile(t, path, "own replay bytes")
	r, game := lagStretchedGame(t, path)
	game.MD5 = sum
	d, lib, _ := outcomeDashboard(t, r, game)

	d.resolveBnetOutcomes(context.Background())
	lib.Flush()
	got, _ := lib.Snapshot().ByChecksum(r.Checksum)
	if got.Players[3].TeamOutcome() != models.OutcomeWon || got.Players[0].TeamOutcome() != models.OutcomeLost {
		t.Fatalf("outcomes %v / %v", got.Players[3].TeamOutcome(), got.Players[0].TeamOutcome())
	}
}

// The archive no longer lists the MD5 of games played before it kept them, but the completion worker
// recorded each chased game's id against the user's file; a complete copy is matched through that file.
func TestResolveBnetOutcomesMatchesACompleteCopyThroughTheQueue(t *testing.T) {
	own := filepath.Join(t.TempDir(), "205902,(8)Big Game Hunters.rep")
	sum := writeReplayFile(t, own, "own replay bytes")
	r, game := lagStretchedGame(t, library.CompletePathFor(own))
	d, lib, root := outcomeDashboard(t, r, game)
	if err := persist.NewEnrichQueue(root).Upsert(persist.EnrichEntry{MD5: sum, Path: own, GameID: game.GameID, Status: persist.EnrichNotBetter}); err != nil {
		t.Fatal(err)
	}

	d.resolveBnetOutcomes(context.Background())
	lib.Flush()
	got, _ := lib.Snapshot().ByChecksum(r.Checksum)
	if got.Players[3].TeamOutcome() != models.OutcomeWon {
		t.Fatalf("outcome %v", got.Players[3].TeamOutcome())
	}
}

func TestResolveBnetOutcomesIgnoresAnotherFilesMD5(t *testing.T) {
	path := filepath.Join(t.TempDir(), "205902,(8)Big Game Hunters.rep")
	writeReplayFile(t, path, "own replay bytes")
	r, game := lagStretchedGame(t, path)
	game.MD5 = "0123456789abcdef0123456789abcdef"
	d, lib, _ := outcomeDashboard(t, r, game)

	d.resolveBnetOutcomes(context.Background())
	lib.Flush()
	if got, _ := lib.Snapshot().ByChecksum(r.Checksum); !got.WinnerUndecided() {
		t.Fatal("settled from a game recorded by a different file")
	}
}

func undecidedOwnReplay(h *enrichHarness, end time.Time, me ...librarytest.PlayerOption) *library.Replay {
	r := librarytest.Replay(
		librarytest.WithDate(end.Add(-600*time.Second)),
		librarytest.WithDuration(600),
		librarytest.WithPlayer("me", me...),
		librarytest.WithPlayer("foe"),
	)
	r.GameSource = library.Strings.Intern(bnetGameSourceName)
	h.lib.Add(0, r)
	h.lib.Flush()
	return r
}

func TestUndecidedOwnGameFetchesTheProfileOnceAtOutcomePriority(t *testing.T) {
	h := newEnrichHarness(t)
	h.worker.deps.enabled = func() bool { return false }
	var requests []profileRequest
	h.worker.deps.profile = func(_ context.Context, _ string, _ int64, req profileRequest) (enrichProfile, error) {
		requests = append(requests, req)
		return enrichProfile{Replays: []bnetfacade.ProfileReplay{{MD5: "m", URL: "u"}}, FetchedAt: h.now}, nil
	}
	end := h.now.Add(-10 * time.Minute)
	undecidedOwnReplay(h, end)

	h.worker.tick(context.Background())
	if len(requests) != 1 || requests[0].prio != bnetfacade.PriorityOutcome {
		t.Fatalf("requests %+v", requests)
	}
	if want := h.now.Sub(end.Add(bnetOutcomeSettle)); requests[0].maxAge != want {
		t.Fatalf("a profile older than the game's end would be served from cache: maxAge %v, want %v", requests[0].maxAge, want)
	}

	h.now = h.now.Add(time.Hour)
	h.worker.tick(context.Background())
	if len(requests) != 1 {
		t.Fatalf("asked again for a game the profile already answered for: %+v", requests)
	}

	undecidedOwnReplay(h, h.now.Add(-5*time.Minute))
	h.worker.tick(context.Background())
	if len(requests) != 2 {
		t.Fatalf("a newer undecided game did not ask: %+v", requests)
	}
}

func TestOutcomeFetchSkipsDecidedFreshAndOldGames(t *testing.T) {
	h := newEnrichHarness(t)
	h.worker.deps.enabled = func() bool { return false }
	h.worker.deps.profile = func(context.Context, string, int64, profileRequest) (enrichProfile, error) {
		t.Fatal("profile fetched")
		return enrichProfile{}, nil
	}
	undecidedOwnReplay(h, h.now.Add(-10*time.Minute), librarytest.Winner())
	undecidedOwnReplay(h, h.now)
	undecidedOwnReplay(h, h.now.Add(-bnetOutcomeWindow-time.Hour))
	h.worker.tick(context.Background())
}

func TestResolveBnetOutcomesSettlesUndecidedGamesFromTheArchive(t *testing.T) {
	r := outcomeReplay()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	lib.Add(0, r)
	lib.Flush()
	root := t.TempDir()
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	store := dashboarddb.NewLibStore(lib, persist.NewBnetCache(root), nil)
	archive := persist.NewBnetGameArchive(root)
	store.SetGameArchive(archive)
	d := &Dashboard{ctx: context.Background(), dbStore: store, library: &libraryRuntime{lib: lib}}
	if err := archive.Upsert([]persist.BnetGame{outcomeGame("1", outcomeGameStart.Add(17*time.Minute),
		persist.BnetGamePlayer{Toon: "chobo86", Result: persist.BnetResultWin},
		persist.BnetGamePlayer{Toon: "cazadorXD"})}); err != nil {
		t.Fatal(err)
	}

	d.resolveBnetOutcomes(context.Background())
	lib.Flush()
	got, _ := lib.Snapshot().ByChecksum(r.Checksum)
	if got.Players[0].TeamOutcome() != models.OutcomeWon || got.Players[1].TeamOutcome() != models.OutcomeLost {
		t.Fatalf("outcomes %v / %v", got.Players[0].TeamOutcome(), got.Players[1].TeamOutcome())
	}
	if got.BnetOutcomeSource != "game 1: results from the players' own profiles" {
		t.Fatalf("source %q", got.BnetOutcomeSource)
	}
}
