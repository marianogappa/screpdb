package dashboard

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"time"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
	"github.com/marianogappa/screpdb/internal/winsandbox"
)

// The enrichment worker (issue #341) recovers the full recording of
// multiplayer games the user left before the end. It is fully automatic and
// backend-only: it keeps a durable queue of candidate games, resolves each
// game's Battle.net id from the user's own profile while the game is still in
// the profile's fast-rolling 25-entry window, then polls the gameinfo endpoint
// with backoff until a co-player's copy is materially longer than ours and
// stable, downloads it, and drops it beside the user's file as
// "<stem>-complete.rep" for the watcher to ingest like any other file.
//
// Every step is budgeted: bridge calls ride the #319 background bucket AND an
// own persisted daily cap, GCS HEADs are free, and every entry carries durable
// stop conditions (rolled off the profile, GCS retention passed, nothing
// better found once the game settled) so a game can never drain the budget
// forever.
const (
	enrichTick = time.Minute
	// enrichProfileMinInterval spaces the own-profile fetches that resolve
	// game ids. The profile lists 25 games, so one fetch covers a whole
	// session's worth of candidates.
	enrichProfileMinInterval = 30 * time.Minute
	// enrichFirstPollDelay lets the game run on after the user left before the
	// first gameinfo poll. Together with enrichConfirmDelay it sets how soon
	// after leaving a game its full recording lands: about half an hour, so it
	// arrives during the same play session.
	enrichFirstPollDelay = 15 * time.Minute
	// enrichConfirmDelay re-polls after seeing a materially longer copy, so a
	// still-running game is not downloaded mid-way: a size unchanged across
	// two polls means whoever stayed longest has uploaded.
	enrichConfirmDelay = 15 * time.Minute
	// enrichMaxPlausibleGame bounds how long a game can run in total. Almost
	// no game crosses 40 minutes and even marathon games stay under 2 hours,
	// so once the user's recorded portion plus the time since they left
	// exceeds this, the game is surely over (see enrichSettleDeadline).
	enrichMaxPlausibleGame = 2 * time.Hour
	// enrichMinResidual keeps a game "possibly still running" for at least
	// this long after the user left, even when their recording already spans
	// most of enrichMaxPlausibleGame: co-players still need time to finish
	// and upload.
	enrichMinResidual = 30 * time.Minute
	// enrichMaxAge stops chasing games older than GCS retention (17-46 days
	// measured; anything this old is gone).
	enrichMaxAge = 40 * 24 * time.Hour
	// enrichBetterRatio is the Content-Length ratio below which a co-player
	// copy is the same recording, merely compressed differently.
	enrichBetterRatio = 1.05
	// enrichDailyCallCap bounds the worker's own bridge spend. Unlike the
	// facade's pacing, this is a real ceiling, because this worker chases games
	// speculatively: it polls for a winner that may never be published, so
	// without a stop it would keep asking forever. A long session costs roughly
	// three calls per game (profile share, first poll, confirm), so this covers
	// a big session and then stands down.
	enrichDailyCallCap     = 40
	enrichMaxProfileMisses = 3
	// enrichMinHumans scopes the feature to multiplayer: in a two-human game
	// the user leaving ends the game, so there is nothing to recover.
	enrichMinHumans = 3
	// enrichUploadSlack is how long after the user left a fresh profile fetch
	// must be before "our md5 is not listed" counts as a miss.
	enrichUploadSlack = 5 * time.Minute
	// enrichSweepBackoff spaces retries after a full gateway sweep found no
	// profile that is provably the logged-in account's own.
	enrichSweepBackoff = 6 * time.Hour
)

const bnetGameSourceName = "AssumedBattleNet"

// enrichProfile is what the worker needs from one own-profile fetch.
type enrichProfile struct {
	Replays   []bnetfacade.ProfileReplay
	GameRefs  []bnetfacade.ProfileGameRef
	FetchedAt time.Time
	Cached    bool
}

// enrichDeps isolates the worker's side effects so the state machine is
// testable with fakes.
type enrichDeps struct {
	now func() time.Time
	// logf carries the worker's console diagnostics; a no-op unless --debug.
	logf func(format string, args ...any)
	// enabled is the labs flag gate: the worker exists from startup but does
	// nothing until the user opts in.
	enabled    func() bool
	bridgeAddr func() (addr string, ok bool)
	// gatewayCandidates orders the gateways the user's own profile may live
	// on, most likely first. The bridge cannot be asked which gateway the
	// session is on, and games in the backlog may span gateways, so the
	// worker sweeps these until a profile proves it is the logged-in
	// account's own (its replays[] entries carry md5+url).
	gatewayCandidates func() []int64
	youKeys           func() map[string]struct{}
	snapshot          func() *library.Snapshot
	complete          func() bool
	profile           func(ctx context.Context, toon string, gateway int64) (enrichProfile, error)
	gameInfo          func(ctx context.Context, addr, gameID string) (*bnetfacade.GameInfo, error)
	headSize          func(ctx context.Context, gcsPath string) (int64, error)
	download          func(ctx context.Context, gcsPath string) ([]byte, error)
	place             func(destPath string, data []byte) error
	fileMD5           func(path string) (string, int64, error)
}

type enrichWorker struct {
	queue *persist.EnrichQueue
	deps  enrichDeps
	// resolvedGateway remembers, per toon, the gateway whose profile proved
	// to be the logged-in account's own; nextProfileAttempt rate-limits the
	// fetches and backs off after a fruitless full sweep.
	resolvedGateway    map[string]int64
	nextProfileAttempt map[string]time.Time
	// amnestied is set once the first active tick has cleared persisted poll
	// schedules: a restart retries promptly instead of honouring backoff
	// decisions a previous run made under conditions that may be gone.
	amnestied bool
}

func newEnrichWorker(queue *persist.EnrichQueue, deps enrichDeps) *enrichWorker {
	return &enrichWorker{
		queue:              queue,
		deps:               deps,
		resolvedGateway:    map[string]int64{},
		nextProfileAttempt: map[string]time.Time{},
	}
}

// profileIsOwnAccount reports whether a fetched profile belongs to the
// logged-in account: Battle.net includes md5 and url only on the requesting
// account's own replays[] entries.
func profileIsOwnAccount(p enrichProfile) bool {
	for _, rep := range p.Replays {
		if rep.MD5 != "" && rep.URL != "" {
			return true
		}
	}
	return false
}

func (d *Dashboard) enrichDeps() enrichDeps {
	logf := func(string, ...any) {}
	if d.debug {
		logf = log.Printf
	}
	return enrichDeps{
		now:     time.Now,
		logf:    logf,
		enabled: func() bool { return d.featureFlagEnabled(d.ctx, featureFlagCompleteReplays) },
		bridgeAddr: func() (string, bool) {
			addr, _ := d.bnetAddr.Load().(string)
			ok := addr != "" && !d.bnetDisabled.Load() &&
				d.currentBnetState() == bnetfacade.BridgeConnected
			return addr, ok
		},
		gatewayCandidates: func() []int64 {
			// Most likely first: whatever the monitor learned, then the
			// gateway CSettings says the user last logged into, then the rest.
			candidates := []int64{d.bnetGateway.Load(), d.youGateway.Load()}
			known := make([]int, 0, len(bnetfacade.GatewayNames))
			for id := range bnetfacade.GatewayNames {
				known = append(known, id)
			}
			sort.Ints(known)
			for _, id := range known {
				candidates = append(candidates, int64(id))
			}
			out := make([]int64, 0, len(candidates))
			seen := map[int64]struct{}{}
			for _, gw := range candidates {
				if gw <= 0 {
					continue
				}
				if _, dup := seen[gw]; dup {
					continue
				}
				seen[gw] = struct{}{}
				out = append(out, gw)
			}
			return out
		},
		youKeys:  d.loadYouKeys,
		snapshot: func() *library.Snapshot { return d.library.lib.Snapshot() },
		complete: func() bool { return d.library.lib.Progress().Complete() },
		profile: func(ctx context.Context, toon string, gateway int64) (enrichProfile, error) {
			res, err := d.getOrFetchBnetProfile(ctx, toon, gateway, bnetfacade.PriorityBackground, enrichProfileMinInterval/2)
			if err != nil {
				return enrichProfile{}, err
			}
			// The archive holds every game this account's fetches ever
			// reported, own-replay md5/url included, so the worker sees a
			// wider window than the 25 games one payload carries.
			games, err := d.dbStore.ListBnetGamesByAccount(ctx, res.AuroraID)
			if err != nil {
				return enrichProfile{}, err
			}
			p := enrichProfile{FetchedAt: res.FetchedAt, Cached: res.Cached}
			for _, g := range games {
				ref := bnetfacade.ProfileGameRef{GameID: g.GameID, CreateTime: g.CreateTime.Unix()}
				p.GameRefs = append(p.GameRefs, ref)
				if g.MD5 == "" {
					continue
				}
				p.Replays = append(p.Replays, bnetfacade.ProfileReplay{
					MD5: g.MD5, URL: g.URL, Link: g.GameID, CreateTime: ref.CreateTime,
				})
			}
			return p, nil
		},
		gameInfo: func(ctx context.Context, addr, gameID string) (*bnetfacade.GameInfo, error) {
			return bnetfacade.FetchGameInfo(ctx, addr, gameID, bnetfacade.PriorityBackground)
		},
		headSize: bnetfacade.HeadReplay,
		download: func(ctx context.Context, gcsPath string) ([]byte, error) {
			return bnetfacade.DownloadReplay(ctx, gcsPath, bnetfacade.PriorityBackground)
		},
		place:   placeCompleteReplay,
		fileMD5: fileMD5AndSize,
	}
}

func (d *Dashboard) startBnetEnrich(ctx context.Context) {
	if d.library == nil {
		return
	}
	w := newEnrichWorker(persist.NewEnrichQueue(d.library.root), d.enrichDeps())
	go func() {
		defer crashreport.GuardNonFatal(nil)
		ticker := time.NewTicker(enrichTick)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.tick(ctx)
			}
		}
	}()
}

func (w *enrichWorker) tick(ctx context.Context) {
	if !w.deps.enabled() {
		return
	}
	now := w.deps.now()
	addr, connected := w.deps.bridgeAddr()
	if !connected {
		return
	}
	if err := w.queue.Prune(now); err != nil {
		w.deps.logf("[bnet-enrich] pruning queue: %v", err)
	}
	if !w.amnestied {
		w.amnestied = true
		w.forgiveStaleSchedules(now)
	}
	if w.deps.complete() {
		w.discover(now)
	}
	w.resolveGameIDs(ctx, now)
	w.poll(ctx, now, addr)
}

// discover enqueues eligible games the queue has never seen. Runs only on a
// complete corpus so a "-complete" sibling still loading cannot be missed.
func (w *enrichWorker) discover(now time.Time) {
	known, err := w.queue.KnownPaths()
	if err != nil {
		w.deps.logf("[bnet-enrich] reading queue: %v", err)
		return
	}
	snap := w.deps.snapshot()
	youKeys := w.deps.youKeys()
	for _, r := range snap.Replays {
		path := r.Path()
		if _, seen := known[path]; seen {
			continue
		}
		toon, ok := enrichEligible(r, youKeys, snap, now)
		if !ok {
			continue
		}
		sum, size, err := w.deps.fileMD5(path)
		if err != nil {
			w.deps.logf("[bnet-enrich] hashing %s: %v", path, err)
			continue
		}
		entry := persist.EnrichEntry{
			MD5:        sum,
			Path:       path,
			SizeBytes:  size,
			ReplayDate: r.Date,
			UserLeftAt: r.Date.Add(time.Duration(r.Duration) * time.Second),
			Toon:       toon,
			CreatedAt:  now,
			Status:     persist.EnrichNeedGameID,
		}
		if err := w.queue.Upsert(entry); err != nil {
			w.deps.logf("[bnet-enrich] enqueueing %s: %v", path, err)
			return
		}
		w.deps.logf("[bnet-enrich] queued %s: the recording ends before the game did", filepath.Base(path))
	}
}

// enrichEligible reports whether a game's full recording is worth recovering,
// and which of the user's toons played it. Tier 0 of the issue: a determined
// winner that includes the user means the recording reached the end.
func enrichEligible(r *library.Replay, youKeys map[string]struct{}, snap *library.Snapshot, now time.Time) (toon string, ok bool) {
	if len(youKeys) == 0 || r == nil {
		return "", false
	}
	if library.Strings.Name(r.GameSource) != bnetGameSourceName {
		return "", false
	}
	if r.Flags.Has(library.FlagIsCompleteCopy) || r.Flags.Has(library.FlagSuperseded) {
		return "", false
	}
	if now.Sub(r.Date) > enrichMaxAge {
		return "", false
	}
	humans := 0
	winnerDetermined := false
	var you *library.Player
	youWinner := false
	for i := range r.Players {
		p := &r.Players[i]
		if p.IsWinner() {
			winnerDetermined = true
		}
		if !p.IsObserver() && p.Type == library.PlayerTypeHuman {
			humans++
		}
		if _, isYou := youKeys[p.Key]; isYou {
			if you == nil {
				you = p
			}
			if p.IsWinner() {
				youWinner = true
			}
		}
	}
	if humans < enrichMinHumans || you == nil {
		return "", false
	}
	if winnerDetermined && youWinner {
		return "", false
	}
	if _, exists := snap.ByPath(library.CompletePathFor(r.Path())); exists {
		return "", false
	}
	return you.Name, true
}

// resolveGameIDs matches queued files against the user's own profile replays[]
// to harvest each game's id — the one fast clock in the flow: entries roll off
// after 25 newer games, so this happens as soon as the bridge allows.
func (w *enrichWorker) resolveGameIDs(ctx context.Context, now time.Time) {
	live, err := w.queue.Live()
	if err != nil {
		w.deps.logf("[bnet-enrich] reading queue: %v", err)
		return
	}
	byToon := map[string][]persist.EnrichEntry{}
	for _, entry := range live {
		if entry.Status != persist.EnrichNeedGameID {
			continue
		}
		// A toon that never qualifies (logged out for weeks, a name CSettings
		// no longer matches) must not keep its games in the sweep forever.
		if now.Sub(entry.ReplayDate) > enrichMaxAge {
			entry.Status = persist.EnrichExpired
			entry.Reason = "GCS retention window passed before the game id could be harvested"
			w.upsert(entry)
			continue
		}
		byToon[entry.Toon] = append(byToon[entry.Toon], entry)
	}
	for toon, entries := range byToon {
		if now.Before(w.nextProfileAttempt[toon]) {
			continue
		}
		profile, ok := w.ownProfile(ctx, now, toon)
		if !ok {
			continue
		}
		repByMD5 := map[string]bnetfacade.ProfileReplay{}
		for _, rep := range profile.Replays {
			if rep.MD5 != "" {
				repByMD5[rep.MD5] = rep
			}
		}
		for _, entry := range entries {
			link := ""
			if rep, found := repByMD5[entry.MD5]; found {
				link = rep.Link
				// Some replays[] entries arrive with an empty link. The same
				// payload's game_results still carries the game id: recover it
				// by create time, which the two sections agree on to seconds.
				if link == "" {
					link = nearestGameRef(profile.GameRefs, rep.CreateTime)
				}
			}
			if link != "" {
				entry.GameID = link
				entry.Status = persist.EnrichWaiting
				entry.NextPollAt = entry.UserLeftAt.Add(enrichFirstPollDelay)
				if entry.NextPollAt.Before(now) {
					entry.NextPollAt = now
				}
				w.deps.logf("[bnet-enrich] resolved %s to game %s", filepath.Base(entry.Path), link)
			} else {
				// A profile older than the game proves nothing: the upload
				// may simply postdate the cache.
				if profile.FetchedAt.Before(entry.UserLeftAt.Add(enrichUploadSlack)) {
					continue
				}
				entry.ProfileMisses++
				if entry.ProfileMisses < enrichMaxProfileMisses {
					w.upsert(entry)
					continue
				}
				entry.Status = persist.EnrichUnavailable
				entry.Reason = "rolled off the profile's replay list before the game id could be harvested"
			}
			w.upsert(entry)
		}
	}
}

// nearestGameRef returns the game id whose create time is closest to
// createTime, within a tolerance that separates any account's consecutive
// games (measured skew between replays[] and game_results[] stamps of one
// game is seconds; one account's games are minutes apart).
func nearestGameRef(refs []bnetfacade.ProfileGameRef, createTime int64) string {
	const toleranceSeconds = 120
	best, bestDiff := "", int64(toleranceSeconds+1)
	for _, ref := range refs {
		diff := ref.CreateTime - createTime
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best, bestDiff = ref.GameID, diff
		}
	}
	return best
}

// ownProfile returns a profile that provably belongs to the logged-in account
// for toon, walking the candidate gateways until one qualifies. A queued game
// may have been played on any gateway, and the bridge cannot say which one the
// session is on — but only the logged-in account's own profile carries md5+url
// in replays[], so qualification is decided from the payload, never guessed.
// The qualifying gateway is remembered; a fruitless full sweep backs off.
func (w *enrichWorker) ownProfile(ctx context.Context, now time.Time, toon string) (enrichProfile, bool) {
	w.nextProfileAttempt[toon] = now.Add(enrichProfileMinInterval)
	if gw, known := w.resolvedGateway[toon]; known {
		profile, err := w.fetchProfile(ctx, now, toon, gw)
		if err != nil {
			return enrichProfile{}, false
		}
		if profileIsOwnAccount(profile) {
			return profile, true
		}
		// Logged out or switched accounts since: forget and re-sweep later.
		delete(w.resolvedGateway, toon)
		return enrichProfile{}, false
	}
	for _, gw := range w.deps.gatewayCandidates() {
		if !w.allowCall(now) {
			// Out of budget mid-sweep: retry from the top next interval.
			return enrichProfile{}, false
		}
		profile, err := w.fetchProfile(ctx, now, toon, gw)
		if err != nil {
			// Bridge trouble is not gateway-specific; stop the sweep.
			return enrichProfile{}, false
		}
		if profileIsOwnAccount(profile) {
			w.resolvedGateway[toon] = gw
			w.deps.logf("[bnet-enrich] %q is the logged-in account on gateway %d", toon, gw)
			return profile, true
		}
	}
	w.nextProfileAttempt[toon] = now.Add(enrichSweepBackoff)
	w.deps.logf("[bnet-enrich] no gateway returned %q as the logged-in account; retrying in %s", toon, enrichSweepBackoff)
	return enrichProfile{}, false
}

func (w *enrichWorker) fetchProfile(ctx context.Context, now time.Time, toon string, gateway int64) (enrichProfile, error) {
	if !w.allowCall(now) {
		return enrichProfile{}, fmt.Errorf("bnet-enrich: daily call cap reached")
	}
	profile, err := w.deps.profile(ctx, toon, gateway)
	if err != nil {
		w.deps.logf("[bnet-enrich] fetching profile for %q on gateway %d: %v", toon, gateway, err)
		return enrichProfile{}, err
	}
	if !profile.Cached {
		w.countCall(now)
	}
	return profile, nil
}

// poll advances every due waiting entry by one gameinfo round trip, newest
// game first: when the daily cap cuts a backlog short, the games the user just
// played — the ones they are most likely looking at — go first.
func (w *enrichWorker) poll(ctx context.Context, now time.Time, addr string) {
	live, err := w.queue.Live()
	if err != nil {
		w.deps.logf("[bnet-enrich] reading queue: %v", err)
		return
	}
	sort.Slice(live, func(i, j int) bool { return live[i].ReplayDate.After(live[j].ReplayDate) })
	for _, entry := range live {
		if entry.Status != persist.EnrichWaiting || now.Before(entry.NextPollAt) {
			continue
		}
		if now.Sub(entry.ReplayDate) > enrichMaxAge {
			entry.Status = persist.EnrichExpired
			entry.Reason = "GCS retention window passed"
			w.upsert(entry)
			continue
		}
		if !w.allowCall(now) {
			return
		}
		w.countCall(now)
		w.pollOne(ctx, now, addr, entry)
	}
}

func (w *enrichWorker) pollOne(ctx context.Context, now time.Time, addr string, entry persist.EnrichEntry) {
	settled := !now.Before(enrichSettleDeadline(entry))
	info, err := w.deps.gameInfo(ctx, addr, entry.GameID)
	if err != nil {
		w.deps.logf("[bnet-enrich] gameinfo for %s: %v", filepath.Base(entry.Path), err)
		w.scheduleErrorRetry(&entry, now, "game info repeatedly unavailable")
		w.upsert(entry)
		return
	}
	bestSize, bestPath := int64(0), ""
	for _, rep := range info.Replays {
		if rep.MD5 == entry.MD5 || rep.URL == "" {
			continue
		}
		gcsPath, err := bnetfacade.GCSReplayPath(rep.URL)
		if err != nil {
			continue
		}
		size, err := w.deps.headSize(ctx, gcsPath)
		if err != nil {
			continue
		}
		if size > bestSize {
			bestSize, bestPath = size, gcsPath
		}
	}

	threshold := int64(float64(entry.SizeBytes) * enrichBetterRatio)
	switch {
	case bestSize == 0 || bestSize < threshold:
		// A completed cycle: errors count consecutive failures, so the
		// tolerance starts over.
		entry.PollErrors = 0
		if settled {
			entry.Status = persist.EnrichNotBetter
			entry.Reason = "no co-player copy materially longer than the user's"
		} else {
			entry.LastBestSize = bestSize
			w.scheduleNext(&entry, now)
		}
	case bestSize == entry.LastBestSize || settled:
		w.fetchAndPlace(ctx, &entry, bestPath, bestSize)
	default:
		// Materially longer but still possibly growing: confirm it is stable
		// before downloading, so a still-running game is not captured mid-way.
		entry.PollErrors = 0
		entry.LastBestSize = bestSize
		entry.PollCount++
		entry.NextPollAt = now.Add(enrichConfirmDelay)
	}
	w.upsert(entry)
}

func (w *enrichWorker) fetchAndPlace(ctx context.Context, entry *persist.EnrichEntry, gcsPath string, size int64) {
	dest := library.CompletePathFor(entry.Path)
	if _, err := iofacade.Stat(dest); err == nil {
		entry.Status = persist.EnrichDone
		entry.Reason = "complete copy already on disk"
		return
	}
	data, err := w.deps.download(ctx, gcsPath)
	if err != nil {
		w.deps.logf("[bnet-enrich] downloading %s: %v", gcsPath, err)
		w.scheduleErrorRetry(entry, w.deps.now(), "download repeatedly failed")
		return
	}
	if err := w.deps.place(dest, data); err != nil {
		w.deps.logf("[bnet-enrich] placing %s: %v", dest, err)
		w.scheduleErrorRetry(entry, w.deps.now(), "placing the downloaded file repeatedly failed")
		return
	}
	entry.Status = persist.EnrichDone
	entry.Reason = fmt.Sprintf("downloaded %d bytes (own copy %d)", size, entry.SizeBytes)
	w.deps.logf("[bnet-enrich] recovered full game beside %s (%d bytes vs %d)", filepath.Base(entry.Path), size, entry.SizeBytes)
}

// enrichSettleDeadline is when the game is surely over. The user's own
// recording already covers part of the game, so the remaining players can only
// have kept playing for the difference up to enrichMaxPlausibleGame, plus
// their upload; before this instant a longer copy needs the two-poll stability
// confirmation, after it the first poll decides.
func enrichSettleDeadline(entry persist.EnrichEntry) time.Time {
	residual := enrichMaxPlausibleGame - entry.UserLeftAt.Sub(entry.ReplayDate)
	if residual < enrichMinResidual {
		residual = enrichMinResidual
	}
	return entry.UserLeftAt.Add(residual + enrichUploadSlack)
}

// forgiveStaleSchedules makes every settled waiting entry due now, once per
// process start. Persisted NextPollAt values encode a previous run's backoff —
// often a transient bridge error hours ago — and honouring them across a
// restart keeps long-over games waiting for no live reason; one poll resolves
// a settled game for good. Unsettled games keep their schedules: their waits
// implement the stability confirmation, which a restart must not shortcut.
// The daily call cap still bounds the resulting burst.
func (w *enrichWorker) forgiveStaleSchedules(now time.Time) {
	live, err := w.queue.Live()
	if err != nil {
		w.deps.logf("[bnet-enrich] reading queue: %v", err)
		return
	}
	for _, entry := range live {
		if entry.Status != persist.EnrichWaiting || !entry.NextPollAt.After(now) {
			continue
		}
		if now.Before(enrichSettleDeadline(entry)) {
			continue
		}
		entry.NextPollAt = now
		w.upsert(entry)
	}
}

// enrichErrorRetryDelays retries a failed bridge call or download quickly at
// first — the bridge is known to return transient timeouts and "Internal
// error" bodies — then backs off. Only errors this persistent expire an entry;
// PollErrors counts consecutive failures and any success resets it, so a
// flaky minute never costs a game its enrichment.
var enrichErrorRetryDelays = []time.Duration{
	2 * time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 3 * time.Hour,
}

func (w *enrichWorker) scheduleErrorRetry(entry *persist.EnrichEntry, now time.Time, expiredReason string) {
	entry.PollErrors++
	if entry.PollErrors > len(enrichErrorRetryDelays) {
		entry.Status = persist.EnrichExpired
		entry.Reason = expiredReason
		return
	}
	entry.NextPollAt = now.Add(enrichErrorRetryDelays[entry.PollErrors-1])
}

// scheduleNext backs the next poll off exponentially: 30m, 1h, 2h, 4h, 8h,
// then 16h flat. With the settle window at 48h this bounds a game to a handful
// of bridge calls.
func (w *enrichWorker) scheduleNext(entry *persist.EnrichEntry, now time.Time) {
	shift := entry.PollCount
	if shift > 5 {
		shift = 5
	}
	entry.PollCount++
	entry.NextPollAt = now.Add(enrichFirstPollDelay << shift)
}

func (w *enrichWorker) allowCall(now time.Time) bool {
	calls, err := w.queue.Calls(now)
	if err != nil {
		w.deps.logf("[bnet-enrich] reading call counter: %v", err)
		return false
	}
	return calls < enrichDailyCallCap
}

func (w *enrichWorker) countCall(now time.Time) {
	if err := w.queue.CountCall(now); err != nil {
		w.deps.logf("[bnet-enrich] recording call: %v", err)
	}
}

func (w *enrichWorker) upsert(entry persist.EnrichEntry) {
	if err := w.queue.Upsert(entry); err != nil {
		w.deps.logf("[bnet-enrich] persisting %s: %v", entry.MD5, err)
	}
}

func fileMD5AndSize(path string) (string, int64, error) {
	data, err := iofacade.ReadFile(path)
	if err != nil {
		return "", 0, err
	}
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:]), int64(len(data)), nil
}

// placeCompleteReplay writes a downloaded complete copy beside the user's own
// file. The Low-integrity Windows worker cannot write to the replays folder,
// so it stages the bytes under app-data and asks the Medium launcher to copy
// them over (issue #237).
func placeCompleteReplay(dest string, data []byte) error {
	if !winsandbox.IsWorker() {
		return iofacade.WriteFile(dest, data, 0o644)
	}
	appDir, err := appdata.Dir()
	if err != nil {
		return err
	}
	temp := filepath.Join(appDir, "complete_download.tmp")
	if err := iofacade.WriteFile(temp, data, 0o644); err != nil {
		return err
	}
	defer func() { _ = iofacade.Remove(temp) }()
	_, err = winsandbox.BrokerPlaceReplay(appDir, temp, filepath.Dir(dest), filepath.Base(dest))
	return err
}
