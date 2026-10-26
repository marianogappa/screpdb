package dashboard

import (
	"context"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/models"
)

// Battle.net settles the games nothing else can: every profile fetch, whatever it was for, records each
// player's own result for their last 25 games, and a recording that stopped before the game ended still
// has a game that did end. So the archive is read for undecided games whenever it grows, and asking
// Battle.net is never needed just for this, except for the user's own games (see resolveOwnOutcomes).
const (
	bnetOutcomesDebounce = 3 * time.Second
	// A game's create time on Battle.net falls between its start and a while after the recording ends,
	// measured from 28s after the start to 15 minutes after the user left.
	bnetOutcomeStartSlack = 2 * time.Minute
	bnetOutcomeEndSlack   = 30 * time.Minute
	// Battle.net counts a player's time in the game 5.5% longer than the replay's game seconds.
	bnetSecondsPerGameSecond = 1.055
	bnetSecondsTolerance     = 20
	// Battle.net lists a game on its players' profiles once it has ended; this is how long that takes.
	bnetOutcomeSettle = 2 * time.Minute
	// The user's profile holds their last 25 games, about a day of play at most.
	bnetOutcomeWindow = 24 * time.Hour
)

func (d *Dashboard) startBnetOutcomes(ctx context.Context) {
	if d.library == nil {
		return
	}
	_, events, cancel := d.library.lib.Subscribe()
	go func() {
		defer crashreport.GuardNonFatal(nil)
		defer cancel()
		var timer <-chan time.Time
		arm := func() {
			if timer == nil {
				timer = time.After(bnetOutcomesDebounce)
			}
		}
		arm()
		for {
			select {
			case <-ctx.Done():
				return
			case <-d.bnetOutcomesKick:
				arm()
			case ev, ok := <-events:
				if !ok {
					return
				}
				// Battle.net's own answers arrive as upgrades, so only new games are a reason to look again.
				if (ev.Kind == library.EventCorpus && len(ev.Added) > 0) || (ev.Kind == library.EventProgress && ev.Progress.Complete()) {
					arm()
				}
			case <-timer:
				timer = nil
				d.resolveBnetOutcomes(ctx)
			}
		}
	}()
}

// kickBnetOutcomes asks for the archive to be read again, after it learned something.
func (d *Dashboard) kickBnetOutcomes() {
	select {
	case d.bnetOutcomesKick <- struct{}{}:
	default:
	}
}

func (d *Dashboard) resolveBnetOutcomes(ctx context.Context) {
	games, err := d.dbStore.ListBnetGamesWithResults(ctx)
	if err != nil || len(games) == 0 {
		return
	}
	index := indexBnetGames(games)
	lib := d.library.lib
	for _, r := range lib.Snapshot().Replays {
		if !r.WinnerUndecided() && r.BnetResults() == nil {
			continue
		}
		g, ok := bnetGameFor(r, index)
		if !ok {
			continue
		}
		if results := bnetResultsOf(g); len(results) > 0 {
			lib.SetBnetResults(r.Checksum, results, "game "+g.GameID+": results from the players' own profiles")
		}
	}
}

func indexBnetGames(games []persist.BnetGame) map[string][]persist.BnetGame {
	index := map[string][]persist.BnetGame{}
	for _, g := range games {
		var keys []string
		for _, p := range g.Players {
			if !p.Computer {
				keys = append(keys, library.PlayerKey(p.Toon))
			}
		}
		key := toonSetKey(keys)
		index[key] = append(index[key], g)
	}
	return index
}

func toonSetKey(keys []string) string {
	slices.Sort(keys)
	return strings.Join(keys, "\x00")
}

// bnetGameFor finds the Battle.net game a replay recorded: the same people, at about that time. Players
// rematching each other make two such games, which the players' time in the game tells apart; when it
// cannot, the replay is left alone rather than given another game's result.
func bnetGameFor(r *library.Replay, index map[string][]persist.BnetGame) (persist.BnetGame, bool) {
	var keys []string
	for i := range r.Players {
		if p := &r.Players[i]; !p.IsObserver() && p.IsHuman() {
			keys = append(keys, p.Key)
		}
	}
	if len(keys) < 2 {
		return persist.BnetGame{}, false
	}
	start := r.Date
	end := start.Add(time.Duration(r.Duration) * time.Second)
	var matches []persist.BnetGame
	for _, g := range index[toonSetKey(keys)] {
		if g.CreateTime.Before(start.Add(-bnetOutcomeStartSlack)) || g.CreateTime.After(end.Add(bnetOutcomeEndSlack)) {
			continue
		}
		if agree, compared := bnetTimesAgree(r, g); agree || !compared {
			matches = append(matches, g)
		}
	}
	if len(matches) != 1 {
		return persist.BnetGame{}, false
	}
	return matches[0], true
}

// bnetTimesAgree reports whether every player time Battle.net knows fits the replay: those who left in
// it left then, and the rest stayed at least as long as it ran. compared is false when it knows none.
func bnetTimesAgree(r *library.Replay, g persist.BnetGame) (agree, compared bool) {
	for _, bp := range g.Players {
		if bp.Seconds <= 0 {
			continue
		}
		p := playerByKey(r, library.PlayerKey(bp.Toon))
		if p == nil {
			return false, true
		}
		played := float64(bp.Seconds) / bnetSecondsPerGameSecond
		if p.Flags.Has(library.PlayerLeft) {
			if math.Abs(played-float64(p.LeaveSec)) > bnetSecondsTolerance {
				return false, true
			}
		} else if played < float64(r.Duration)-bnetSecondsTolerance {
			return false, true
		}
		compared = true
	}
	return compared, compared
}

func playerByKey(r *library.Replay, key string) *library.Player {
	for i := range r.Players {
		if r.Players[i].Key == key {
			return &r.Players[i]
		}
	}
	return nil
}

func bnetResultsOf(g persist.BnetGame) []library.BnetResult {
	var results []library.BnetResult
	for _, p := range g.Players {
		outcome := models.OutcomeUnknown
		switch p.Result {
		case persist.BnetResultWin:
			outcome = models.OutcomeWon
		case persist.BnetResultLoss:
			outcome = models.OutcomeLost
		case persist.BnetResultDisconnect:
			outcome = models.OutcomeDisconnected
		}
		if outcome != models.OutcomeUnknown {
			results = append(results, library.BnetResult{Key: library.PlayerKey(p.Toon), Outcome: outcome})
		}
	}
	slices.SortFunc(results, func(a, b library.BnetResult) int { return strings.Compare(a.Key, b.Key) })
	return results
}

// resolveOwnOutcomes asks for the user's own profile, ahead of background sweeps, once one of their games
// ends with no winner the recording or the engine could name. One fetch answers for every game in it, and
// it also serves the game-id step of the same tick, so a game costs at most one request between the two.
func (w *enrichWorker) resolveOwnOutcomes(ctx context.Context, now time.Time) {
	youKeys := w.deps.youKeys()
	if len(youKeys) == 0 {
		return
	}
	readyAt := map[string]time.Time{}
	for _, r := range w.deps.snapshot().Replays {
		if library.Strings.Name(r.GameSource) != bnetGameSourceName || !r.WinnerUndecided() {
			continue
		}
		ready := r.Date.Add(time.Duration(r.Duration)*time.Second + bnetOutcomeSettle)
		if now.Before(ready) || now.Sub(ready) > bnetOutcomeWindow {
			continue
		}
		for i := range r.Players {
			p := &r.Players[i]
			if _, isYou := youKeys[p.Key]; isYou && !p.IsObserver() {
				readyAt[p.Name] = later(readyAt[p.Name], ready)
				break
			}
		}
	}
	for toon, ready := range readyAt {
		if !w.outcomeAttempt[toon].Before(ready) {
			continue
		}
		if _, known := w.resolvedGateway[toon]; !known && now.Before(w.nextProfileAttempt[toon]) {
			continue
		}
		w.outcomeAttempt[toon] = now
		req := profileRequest{prio: bnetfacade.PriorityOutcome, maxAge: max(now.Sub(ready), time.Minute)}
		if profile, ok := w.ownProfile(ctx, now, toon, req); ok {
			w.prefetched[toon] = profile
		}
	}
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
