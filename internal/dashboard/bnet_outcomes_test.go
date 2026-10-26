package dashboard

import (
	"context"
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
