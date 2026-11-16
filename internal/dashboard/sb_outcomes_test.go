package dashboard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/sbfacade"
)

const sbTestGameID = "01a1da1c-8486-7801-827d-b56d61fe89e8"

var sbGameStart = time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)

type sbHarness struct {
	w       *sbOutcomes
	replays []*library.Replay
	set     map[[32]byte][]library.BnetResult
	answers []func() (*sbfacade.Game, error)
	asked   int
	now     time.Time
}

// newSBHarness renames the second player: the record names accounts, never names.
func newSBHarness(t *testing.T) *sbHarness {
	r := librarytest.Replay(
		librarytest.WithDate(sbGameStart),
		librarytest.WithDuration(600),
		librarytest.WithPlayer("RenamedSince"),
		librarytest.WithPlayer("foe"),
		librarytest.WithPlayer("Watcher", librarytest.Observer()),
	)
	r.ShieldBattery = &models.ShieldBatteryGame{GameID: sbTestGameID, UserIDs: map[byte]uint32{0: 17185, 1: 18606, 2: 99}}
	root := t.TempDir()
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatal(err)
	}
	h := &sbHarness{replays: []*library.Replay{r}, set: map[[32]byte][]library.BnetResult{}, now: sbGameStart.Add(time.Hour)}
	h.w = &sbOutcomes{
		snapshot: func() *library.Snapshot { return &library.Snapshot{Replays: h.replays} },
		setResults: func(checksum [32]byte, results []library.BnetResult, _ string) {
			h.set[checksum] = results
		},
		store: persist.NewSBGames(root),
		fetch: func(context.Context, string) (*sbfacade.Game, error) {
			h.asked++
			answer := h.answers[0]
			h.answers = h.answers[1:]
			return answer()
		},
		now:      func() time.Time { return h.now },
		attempts: map[string]*sbOutcomeAttempt{},
	}
	return h
}

func settled() (*sbfacade.Game, error) {
	return &sbfacade.Game{ID: sbTestGameID, Results: map[uint32]sbfacade.Result{17185: sbfacade.ResultWin, 18606: sbfacade.ResultLoss, 99: sbfacade.ResultWin}}, nil
}

func pending() (*sbfacade.Game, error) { return &sbfacade.Game{ID: sbTestGameID}, nil }

func TestSBOutcomesSettleAnUndecidedGameByAccount(t *testing.T) {
	h := newSBHarness(t)
	h.answers = append(h.answers, settled)
	if next := h.w.resolve(context.Background()); !next.IsZero() {
		t.Fatalf("nothing should be left to ask, next %v", next)
	}
	want := []library.BnetResult{{Key: "foe", Outcome: models.OutcomeLost}, {Key: "renamedsince", Outcome: models.OutcomeWon}}
	if got := h.set[h.replays[0].Checksum]; len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("results %+v", got)
	}

	// A settled game is answered from the cache from then on.
	h.set = map[[32]byte][]library.BnetResult{}
	h.w.resolve(context.Background())
	if h.asked != 1 || len(h.set[h.replays[0].Checksum]) != 2 {
		t.Fatalf("asked %d times, results %+v", h.asked, h.set)
	}
}

func TestSBOutcomesLeaveDecidedAndOtherGamesAlone(t *testing.T) {
	h := newSBHarness(t)
	h.replays[0].Players[0].Flags |= library.PlayerResultWon | library.TeamResultWon
	bnet := librarytest.Replay(librarytest.WithPlayer("a"), librarytest.WithPlayer("b"))
	h.replays = append(h.replays, bnet)
	h.w.resolve(context.Background())
	if h.asked != 0 || len(h.set) != 0 {
		t.Fatalf("asked %d, set %+v", h.asked, h.set)
	}
}

func TestSBOutcomesWaitForTheGameToEndThenRetry(t *testing.T) {
	h := newSBHarness(t)
	h.now = sbGameStart.Add(5 * time.Minute)
	ends := sbGameStart.Add(10*time.Minute + sbOutcomeSettle)
	if next := h.w.resolve(context.Background()); !next.Equal(ends) || h.asked != 0 {
		t.Fatalf("next %v asked %d", next, h.asked)
	}

	h.now = ends
	h.answers = append(h.answers, pending)
	if next := h.w.resolve(context.Background()); !next.Equal(ends.Add(sbOutcomeRetries[0])) {
		t.Fatalf("retry at %v", next)
	}
	if next := h.w.resolve(context.Background()); h.asked != 1 || next.IsZero() {
		t.Fatalf("asked again before the retry was due: %d", h.asked)
	}

	for i := range sbOutcomeRetries {
		h.now = h.w.attempts[sbTestGameID].next
		h.answers = append(h.answers, pending)
		next := h.w.resolve(context.Background())
		if i == len(sbOutcomeRetries)-1 && !next.IsZero() {
			t.Fatalf("still retrying after %d tries", h.asked)
		}
	}
	h.now = h.now.Add(24 * time.Hour)
	h.w.resolve(context.Background())
	if h.asked != len(sbOutcomeRetries)+1 || len(h.set) != 0 {
		t.Fatalf("asked %d, set %+v", h.asked, h.set)
	}
}

func TestSBOutcomesStopTheRoundWhenRateLimited(t *testing.T) {
	h := newSBHarness(t)
	second := librarytest.Replay(librarytest.WithDate(sbGameStart), librarytest.WithPlayer("x"), librarytest.WithPlayer("y"))
	second.ShieldBattery = &models.ShieldBatteryGame{GameID: "01a1da1c-0000-0000-0000-000000000000", UserIDs: map[byte]uint32{0: 1, 1: 2}}
	h.replays = append(h.replays, second)
	h.answers = append(h.answers, func() (*sbfacade.Game, error) { return nil, sbfacade.ErrRateLimited })
	if next := h.w.resolve(context.Background()); !next.Equal(h.now.Add(sbOutcomeRetries[0])) || h.asked != 1 {
		t.Fatalf("next %v asked %d", next, h.asked)
	}
}

func TestSBOutcomesGiveUpOnUnknownGamesAndRetryErrors(t *testing.T) {
	h := newSBHarness(t)
	h.answers = append(h.answers, func() (*sbfacade.Game, error) { return nil, sbfacade.ErrNotFound })
	if next := h.w.resolve(context.Background()); !next.IsZero() {
		t.Fatalf("unknown game rescheduled at %v", next)
	}

	h = newSBHarness(t)
	h.answers = append(h.answers, func() (*sbfacade.Game, error) { return nil, errors.New("connection reset") })
	if next := h.w.resolve(context.Background()); !next.Equal(h.now.Add(sbOutcomeRetries[0])) {
		t.Fatalf("error retry at %v", next)
	}
}

func TestSBOutcomesCacheCanceledGames(t *testing.T) {
	h := newSBHarness(t)
	h.answers = append(h.answers, func() (*sbfacade.Game, error) { return &sbfacade.Game{ID: sbTestGameID, Canceled: true}, nil })
	h.w.resolve(context.Background())
	h.w.resolve(context.Background())
	if h.asked != 1 || len(h.set) != 0 {
		t.Fatalf("asked %d, set %+v", h.asked, h.set)
	}
}
