package dashboard

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/sbfacade"
)

// ShieldBattery keeps every game's result, reconciled from all its players' reports, and serves it to
// anyone by the game id each replay carries. So unlike Battle.net, any undecided ShieldBattery game can be
// settled, at any age, with one request, and is only ever asked about while it is undecided.
const (
	sbOutcomesDebounce = 3 * time.Second
	// The server reconciles results once every player has reported, shortly after the game ends.
	sbOutcomeSettle = time.Minute
)

// sbOutcomeRetries spaces out asking again about a game the server has not settled yet. Past the last one
// the game is left alone until the app restarts.
var sbOutcomeRetries = []time.Duration{2 * time.Minute, 10 * time.Minute, time.Hour, 6 * time.Hour}

type sbOutcomeAttempt struct {
	tries int
	next  time.Time
}

type sbOutcomes struct {
	snapshot   func() *library.Snapshot
	setResults func(checksum [32]byte, results []library.BnetResult, source string)
	store      *persist.SBGames
	fetch      func(ctx context.Context, gameID string) (*sbfacade.Game, error)
	now        func() time.Time
	attempts   map[string]*sbOutcomeAttempt
}

func (d *Dashboard) startSBOutcomes(ctx context.Context) {
	if d.library == nil {
		return
	}
	lib := d.library.lib
	w := &sbOutcomes{
		snapshot:   lib.Snapshot,
		setResults: lib.SetBnetResults,
		store:      persist.NewSBGames(d.library.root),
		fetch:      sbfacade.FetchGame,
		now:        time.Now,
		attempts:   map[string]*sbOutcomeAttempt{},
	}
	_, events, cancel := lib.Subscribe()
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer cancel()
		timer := time.NewTimer(sbOutcomesDebounce)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-events:
				if !ok {
					return
				}
				if (ev.Kind == library.EventCorpus && len(ev.Added) > 0) || (ev.Kind == library.EventProgress && ev.Progress.Complete()) {
					timer.Reset(sbOutcomesDebounce)
				}
			case <-timer.C:
				if next := w.resolve(ctx); !next.IsZero() {
					timer.Reset(max(next.Sub(w.now()), sbOutcomesDebounce))
				}
			}
		}
	}()
}

// resolve settles every undecided ShieldBattery game it can and returns when a game not settled yet is
// next worth asking about, or the zero time if none is.
func (w *sbOutcomes) resolve(ctx context.Context) time.Time {
	var next time.Time
	later := func(t time.Time) {
		if next.IsZero() || t.Before(next) {
			next = t
		}
	}
	for _, r := range w.snapshot().Replays {
		if r.ShieldBattery == nil || (!r.WinnerUndecided() && r.BnetResults() == nil) {
			continue
		}
		gameID := r.ShieldBattery.GameID
		game, known, err := w.store.Get(gameID)
		if err != nil {
			log.Printf("shieldbattery outcomes: reading cache: %v", err)
			return next
		}
		if !known {
			if ctx.Err() != nil {
				return next
			}
			due := w.dueAt(r)
			if due.IsZero() {
				continue
			}
			if w.now().Before(due) {
				later(due)
				continue
			}
			var stop bool
			game, known, stop = w.ask(ctx, gameID)
			if stop {
				later(w.now().Add(sbOutcomeRetries[0]))
				return next
			}
			if !known {
				if due := w.dueAt(r); !due.IsZero() {
					later(due)
				}
				continue
			}
		}
		if results := sbResultsOf(r, game); len(results) > 0 {
			w.setResults(r.Checksum, results, "game "+gameID+": results from ShieldBattery's record of the game")
		}
	}
	return next
}

// dueAt is when a game not settled yet is next worth asking about, or the zero time once it has been
// asked about enough.
func (w *sbOutcomes) dueAt(r *library.Replay) time.Time {
	if a := w.attempts[r.ShieldBattery.GameID]; a != nil {
		return a.next
	}
	return r.Date.Add(time.Duration(r.Duration)*time.Second + sbOutcomeSettle)
}

// ask fetches a game's record, stores it once settled, and reports stop when nothing more should be asked
// this round.
func (w *sbOutcomes) ask(ctx context.Context, gameID string) (game persist.SBGame, settled, stop bool) {
	fetched, err := w.fetch(ctx, gameID)
	switch {
	case errors.Is(err, sbfacade.ErrRateLimited) || ctx.Err() != nil:
		return game, false, true
	case errors.Is(err, sbfacade.ErrNotFound) || errors.Is(err, sbfacade.ErrBadGameID):
		w.giveUp(gameID)
		return game, false, false
	case err != nil:
		log.Printf("shieldbattery outcomes: game %s: %v", gameID, err)
		w.retry(gameID)
		return game, false, false
	case fetched.Results == nil && !fetched.Canceled:
		w.retry(gameID)
		return game, false, false
	}
	game = persist.SBGame{Canceled: fetched.Canceled, FetchedAt: w.now().UTC()}
	if fetched.Results != nil {
		game.Results = make(map[uint32]string, len(fetched.Results))
		for userID, result := range fetched.Results {
			game.Results[userID] = string(result)
		}
	}
	if err := w.store.Put(gameID, game); err != nil {
		log.Printf("shieldbattery outcomes: caching game %s: %v", gameID, err)
	}
	delete(w.attempts, gameID)
	return game, true, false
}

func (w *sbOutcomes) retry(gameID string) {
	a := w.attempts[gameID]
	if a == nil {
		a = &sbOutcomeAttempt{}
		w.attempts[gameID] = a
	}
	if a.tries >= len(sbOutcomeRetries) {
		w.giveUp(gameID)
		return
	}
	a.next = w.now().Add(sbOutcomeRetries[a.tries])
	a.tries++
}

func (w *sbOutcomes) giveUp(gameID string) {
	w.attempts[gameID] = &sbOutcomeAttempt{tries: len(sbOutcomeRetries)}
}

// sbResultsOf names each player's recorded result by their place in the replay. Players are matched by
// account id, not name: ShieldBattery accounts can be renamed after a game.
func sbResultsOf(r *library.Replay, game persist.SBGame) []library.BnetResult {
	var results []library.BnetResult
	for i := range r.Players {
		p := &r.Players[i]
		userID, ok := r.ShieldBattery.UserIDs[p.ReplayPlayerID]
		if !ok || p.IsObserver() {
			continue
		}
		outcome := models.OutcomeUnknown
		switch sbfacade.Result(game.Results[userID]) {
		case sbfacade.ResultWin:
			outcome = models.OutcomeWon
		case sbfacade.ResultLoss:
			outcome = models.OutcomeLost
		}
		if outcome != models.OutcomeUnknown {
			results = append(results, library.BnetResult{Key: p.Key, Outcome: outcome})
		}
	}
	slices.SortFunc(results, func(a, b library.BnetResult) int { return strings.Compare(a.Key, b.Key) })
	return results
}
