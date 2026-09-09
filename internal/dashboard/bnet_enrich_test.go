package dashboard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marianogappa/screpdb/internal/bnetfacade"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/librarytest"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

const testGCSURL = "https://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"

type enrichHarness struct {
	worker   *enrichWorker
	queue    *persist.EnrichQueue
	now      time.Time
	lib      *library.Library
	profile  enrichProfile
	prfErr   error
	info     *bnetfacade.GameInfo
	infoErr  error
	sizes    map[string]int64
	placed   map[string][]byte
	download []byte
}

func newEnrichHarness(t *testing.T) *enrichHarness {
	t.Helper()
	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	root := filepath.Join(t.TempDir(), "appdata")
	// Another test in the package may have flipped iofacade to enforcing.
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatalf("AllowDir: %v", err)
	}
	h := &enrichHarness{
		queue:    persist.NewEnrichQueue(root),
		now:      time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC),
		lib:      lib,
		sizes:    map[string]int64{},
		placed:   map[string][]byte{},
		download: []byte("replay-bytes"),
	}
	h.worker = newEnrichWorker(h.queue, enrichDeps{
		now:               func() time.Time { return h.now },
		logf:              t.Logf,
		enabled:           func() bool { return true },
		bridgeAddr:        func() (string, bool) { return "127.0.0.1:1", true },
		gatewayCandidates: func() []int64 { return []int64{30} },
		youKeys:           func() map[string]struct{} { return map[string]struct{}{"me": {}} },
		snapshot:          func() *library.Snapshot { return lib.Snapshot() },
		complete:          func() bool { return true },
		profile: func(context.Context, string, int64) (enrichProfile, error) {
			return h.profile, h.prfErr
		},
		gameInfo: func(context.Context, string, string) (*bnetfacade.GameInfo, error) {
			return h.info, h.infoErr
		},
		headSize: func(_ context.Context, gcsPath string) (int64, error) {
			size, ok := h.sizes[gcsPath]
			if !ok {
				return 0, bnetfacade.ErrReplayGone
			}
			return size, nil
		},
		download: func(context.Context, string) ([]byte, error) { return h.download, nil },
		place: func(dest string, data []byte) error {
			h.placed[dest] = data
			return nil
		},
		fileMD5: func(string) (string, int64, error) { return "ownmd5", 100_000, nil },
	})
	return h
}

// eligibleReplay is a Battle.net multiplayer game the user (player "me") left
// without a determined winner, recorded 2 hours before the harness clock.
func (h *enrichHarness) eligibleReplay(t *testing.T, opts ...librarytest.Option) *library.Replay {
	t.Helper()
	base := []librarytest.Option{
		librarytest.WithDate(h.now.Add(-2 * time.Hour)),
		librarytest.WithDuration(600),
		librarytest.WithPath("/replays/bgh.rep", h.now.Add(-2*time.Hour)),
		librarytest.WithPlayer("me"),
		librarytest.WithPlayer("ally"),
		librarytest.WithPlayer("enemy1"),
		librarytest.WithPlayer("enemy2"),
	}
	r := librarytest.Replay(append(base, opts...)...)
	r.GameSource = library.Strings.Intern(bnetGameSourceName)
	for i := range r.Players {
		r.Players[i].Type = library.PlayerTypeHuman
	}
	h.lib.Add(0, r)
	h.lib.Flush()
	return r
}

func (h *enrichHarness) entry(t *testing.T) persist.EnrichEntry {
	t.Helper()
	entry, ok, err := h.queue.Get("ownmd5")
	if err != nil || !ok {
		t.Fatalf("entry: ok=%v err=%v", ok, err)
	}
	return entry
}

func TestEnrichEligibility(t *testing.T) {
	h := newEnrichHarness(t)
	now := h.now
	you := map[string]struct{}{"me": {}}

	eligible := h.eligibleReplay(t)
	snap := h.lib.Snapshot()
	if toon, ok := enrichEligible(eligible, you, snap, now); !ok || toon != "me" {
		t.Fatalf("eligible game rejected: %q %v", toon, ok)
	}
	if _, ok := enrichEligible(eligible, nil, snap, now); ok {
		t.Fatal("eligible without you-keys")
	}

	ineligible := map[string]*library.Replay{
		"not battle.net": func() *library.Replay {
			r := librarytest.Replay(librarytest.WithPlayer("me"), librarytest.WithPlayer("b"), librarytest.WithPlayer("c"))
			r.GameSource = library.Strings.Intern("PreSCR")
			for i := range r.Players {
				r.Players[i].Type = library.PlayerTypeHuman
			}
			return r
		}(),
		"two humans": func() *library.Replay {
			r := librarytest.Replay(librarytest.WithPlayer("me"), librarytest.WithPlayer("b"))
			r.GameSource = library.Strings.Intern(bnetGameSourceName)
			for i := range r.Players {
				r.Players[i].Type = library.PlayerTypeHuman
			}
			return r
		}(),
		"user among determined winners": func() *library.Replay {
			r := librarytest.Replay(librarytest.WithPlayer("me"), librarytest.WithPlayer("b"), librarytest.WithPlayer("c"))
			r.GameSource = library.Strings.Intern(bnetGameSourceName)
			for i := range r.Players {
				r.Players[i].Type = library.PlayerTypeHuman
			}
			r.Players[0].Flags |= library.PlayerWinner
			return r
		}(),
		"user not in game": func() *library.Replay {
			r := librarytest.Replay(librarytest.WithPlayer("a"), librarytest.WithPlayer("b"), librarytest.WithPlayer("c"))
			r.GameSource = library.Strings.Intern(bnetGameSourceName)
			for i := range r.Players {
				r.Players[i].Type = library.PlayerTypeHuman
			}
			return r
		}(),
		"too old": func() *library.Replay {
			r := librarytest.Replay(
				librarytest.WithDate(now.Add(-enrichMaxAge-time.Hour)),
				librarytest.WithPlayer("me"), librarytest.WithPlayer("b"), librarytest.WithPlayer("c"))
			r.GameSource = library.Strings.Intern(bnetGameSourceName)
			for i := range r.Players {
				r.Players[i].Type = library.PlayerTypeHuman
			}
			return r
		}(),
	}
	for name, r := range ineligible {
		if _, ok := enrichEligible(r, you, snap, now); ok {
			t.Errorf("%s: unexpectedly eligible", name)
		}
	}

	// A user who lost while the winner was still determined is eligible: the
	// recording may have been truncated and named the wrong winner.
	lost := h.eligibleReplay(t, librarytest.WithPath("/replays/lost.rep", now))
	lost.Players[1].Flags |= library.PlayerWinner
	if _, ok := enrichEligible(lost, you, snap, now); !ok {
		t.Error("determined winner without the user should stay eligible")
	}
}

func TestEnrichDiscoverAndResolve(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.info = &bnetfacade.GameInfo{}
	h.profile = enrichProfile{
		Replays:   []bnetfacade.ProfileReplay{{MD5: "ownmd5", Link: "MM-GAME-1", URL: "https://x"}},
		FetchedAt: h.now,
	}
	h.worker.tick(context.Background())

	entry := h.entry(t)
	if entry.Status != persist.EnrichWaiting || entry.GameID != "MM-GAME-1" {
		t.Fatalf("entry after resolve: %+v", entry)
	}
	// The user left 90 minutes ago, past the first-poll delay, so the first
	// (empty) poll already happened in the same tick: one call for the profile
	// fetch, one for the gameinfo poll.
	if entry.PollCount != 1 {
		t.Fatalf("PollCount: %d", entry.PollCount)
	}
	calls, _ := h.queue.Calls(h.now)
	if calls != 2 {
		t.Fatalf("calls: %d", calls)
	}
}

func TestEnrichProfileMissesRollOff(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.profile = enrichProfile{
		Replays:   []bnetfacade.ProfileReplay{{MD5: "someoneelse", URL: "https://x", Link: "MM-OTHER"}},
		FetchedAt: h.now,
	}
	for i := 0; i < enrichMaxProfileMisses; i++ {
		h.worker.tick(context.Background())
		h.now = h.now.Add(enrichProfileMinInterval + time.Minute)
	}
	entry := h.entry(t)
	if entry.Status != persist.EnrichUnavailable {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestEnrichProfileOlderThanGameDoesNotCountMiss(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.profile = enrichProfile{
		Replays:   []bnetfacade.ProfileReplay{{MD5: "someoneelse", URL: "https://x", Link: "MM-OTHER"}},
		FetchedAt: h.now.Add(-3 * time.Hour), Cached: true,
	}
	h.worker.tick(context.Background())
	entry := h.entry(t)
	if entry.Status != persist.EnrichNeedGameID || entry.ProfileMisses != 0 {
		t.Fatalf("entry: %+v", entry)
	}
	calls, _ := h.queue.Calls(h.now)
	if calls != 0 {
		t.Fatalf("cached profile fetch spent a call: %d", calls)
	}
}

// resolveEntry drives the queue to one waiting entry with a resolved game id,
// past its first (empty) gameinfo poll.
func (h *enrichHarness) resolveEntry(t *testing.T) persist.EnrichEntry {
	t.Helper()
	h.eligibleReplay(t)
	h.info = &bnetfacade.GameInfo{}
	h.profile = enrichProfile{
		Replays:   []bnetfacade.ProfileReplay{{MD5: "ownmd5", Link: "MM-GAME-1", URL: "https://x"}},
		FetchedAt: h.now,
	}
	h.worker.tick(context.Background())
	entry := h.entry(t)
	if entry.Status != persist.EnrichWaiting {
		t.Fatalf("entry after resolve: %+v", entry)
	}
	return entry
}

func TestEnrichDownloadsStableLongerCopy(t *testing.T) {
	h := newEnrichHarness(t)
	// Left half an hour ago: well before the settle deadline, so the download
	// must wait for the two-poll stability confirmation.
	entry := persist.EnrichEntry{
		MD5: "ownmd5", Path: "/replays/bgh.rep", SizeBytes: 100_000,
		ReplayDate: h.now.Add(-40 * time.Minute), UserLeftAt: h.now.Add(-30 * time.Minute),
		Toon: "me", GameID: "MM-GAME-1", Status: persist.EnrichWaiting, NextPollAt: h.now,
	}
	if err := h.queue.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	h.info = &bnetfacade.GameInfo{Replays: []bnetfacade.GameInfoReplay{
		{MD5: "ownmd5", URL: "https://storage.googleapis.com/starcraft-user-uploads-prod/S1-replays/1/2/ownmd5.replay"},
		{MD5: "other", URL: testGCSURL},
	}}
	h.sizes["/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"] = 250_000

	// First sighting of the longer copy must confirm stability, not download.
	h.now = entry.NextPollAt.Add(time.Minute)
	h.worker.tick(context.Background())
	entry = h.entry(t)
	if entry.Status != persist.EnrichWaiting || entry.LastBestSize != 250_000 {
		t.Fatalf("after first poll: %+v", entry)
	}
	if len(h.placed) != 0 {
		t.Fatal("downloaded before the size was stable")
	}

	// Second poll sees the same size: download and place.
	h.now = entry.NextPollAt.Add(time.Minute)
	h.worker.tick(context.Background())
	entry = h.entry(t)
	if entry.Status != persist.EnrichDone {
		t.Fatalf("after second poll: %+v", entry)
	}
	if string(h.placed["/replays/bgh-complete.rep"]) != "replay-bytes" {
		t.Fatalf("placed: %v", h.placed)
	}
}

func TestEnrichNotBetterAfterSettleWindow(t *testing.T) {
	h := newEnrichHarness(t)
	h.resolveEntry(t)
	h.info = &bnetfacade.GameInfo{Replays: []bnetfacade.GameInfoReplay{
		{MD5: "other", URL: testGCSURL},
	}}
	h.sizes["/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"] = 101_000

	h.now = enrichSettleDeadline(h.entry(t)).Add(time.Hour)
	h.worker.tick(context.Background())
	entry := h.entry(t)
	if entry.Status != persist.EnrichNotBetter {
		t.Fatalf("entry: %+v", entry)
	}
	if len(h.placed) != 0 {
		t.Fatal("downloaded a copy that is not materially longer")
	}
}

func TestEnrichExpiresOnRepeatedGameInfoErrors(t *testing.T) {
	h := newEnrichHarness(t)
	entry := h.resolveEntry(t)
	h.infoErr = errors.New("bridge hiccup")
	for i := 0; i <= len(enrichErrorRetryDelays); i++ {
		h.now = entry.NextPollAt.Add(time.Minute)
		h.worker.tick(context.Background())
		entry = h.entry(t)
	}
	if entry.Status != persist.EnrichExpired {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestEnrichDailyCapStopsBridgeCalls(t *testing.T) {
	h := newEnrichHarness(t)
	for i := 0; i < enrichDailyCallCap; i++ {
		if err := h.queue.CountCall(h.now); err != nil {
			t.Fatal(err)
		}
	}
	h.eligibleReplay(t)
	h.profile = enrichProfile{
		Replays:   []bnetfacade.ProfileReplay{{MD5: "ownmd5", Link: "MM-GAME-1", URL: "https://x"}},
		FetchedAt: h.now,
	}
	h.worker.tick(context.Background())
	entry := h.entry(t)
	if entry.Status != persist.EnrichNeedGameID {
		t.Fatalf("bridge call went out over the daily cap: %+v", entry)
	}
}

func TestEnrichPollsNewestFirstUnderTheCap(t *testing.T) {
	h := newEnrichHarness(t)
	older := persist.EnrichEntry{
		MD5: "older", Path: "/replays/older.rep", SizeBytes: 100_000,
		ReplayDate: h.now.Add(-30 * time.Hour), UserLeftAt: h.now.Add(-29 * time.Hour),
		Toon: "me", GameID: "MM-OLD", Status: persist.EnrichWaiting, NextPollAt: h.now,
	}
	newer := persist.EnrichEntry{
		MD5: "newer", Path: "/replays/newer.rep", SizeBytes: 100_000,
		ReplayDate: h.now.Add(-2 * time.Hour), UserLeftAt: h.now.Add(-90 * time.Minute),
		Toon: "me", GameID: "MM-NEW", Status: persist.EnrichWaiting, NextPollAt: h.now,
	}
	for _, entry := range []persist.EnrichEntry{older, newer} {
		if err := h.queue.Upsert(entry); err != nil {
			t.Fatal(err)
		}
	}
	// One call left in the daily budget: only the newest game may spend it.
	for i := 0; i < enrichDailyCallCap-1; i++ {
		if err := h.queue.CountCall(h.now); err != nil {
			t.Fatal(err)
		}
	}
	h.info = &bnetfacade.GameInfo{}
	h.worker.tick(context.Background())

	newerEntry, _, _ := h.queue.Get("newer")
	olderEntry, _, _ := h.queue.Get("older")
	if newerEntry.PollCount != 1 {
		t.Fatalf("newest game not polled: %+v", newerEntry)
	}
	if olderEntry.PollCount != 0 {
		t.Fatalf("older game polled ahead of the newest: %+v", olderEntry)
	}
}

func TestEnrichFetchAndPlaceEdges(t *testing.T) {
	t.Run("download failures expire the entry", func(t *testing.T) {
		h := newEnrichHarness(t)
		entry := h.resolveEntry(t)
		h.info = &bnetfacade.GameInfo{Replays: []bnetfacade.GameInfoReplay{{MD5: "other", URL: testGCSURL}}}
		h.sizes["/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"] = 250_000
		downloadErr := errors.New("gcs hiccup")
		h.worker.deps.download = func(context.Context, string) ([]byte, error) { return nil, downloadErr }
		h.now = enrichSettleDeadline(h.entry(t)).Add(time.Hour)
		for i := 0; i <= len(enrichErrorRetryDelays); i++ {
			h.worker.tick(context.Background())
			entry = h.entry(t)
			h.now = entry.NextPollAt.Add(time.Minute)
		}
		if entry.Status != persist.EnrichExpired {
			t.Fatalf("entry: %+v", entry)
		}
	})

	t.Run("existing complete file short-circuits to done", func(t *testing.T) {
		h := newEnrichHarness(t)
		dir := t.TempDir()
		if err := iofacade.AllowDir(dir); err != nil {
			t.Fatal(err)
		}
		ownPath := filepath.Join(dir, "bgh.rep")
		entry := persist.EnrichEntry{
			MD5: "ownmd5", Path: ownPath, SizeBytes: 100_000,
			ReplayDate: h.now.Add(-2 * time.Hour), UserLeftAt: h.now.Add(-90 * time.Minute),
			Toon: "me", GameID: "MM-1", Status: persist.EnrichWaiting, NextPollAt: h.now,
		}
		if err := h.queue.Upsert(entry); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "bgh-complete.rep"), []byte("already"), 0o644); err != nil {
			t.Fatal(err)
		}
		h.info = &bnetfacade.GameInfo{Replays: []bnetfacade.GameInfoReplay{{MD5: "other", URL: testGCSURL}}}
		h.sizes["/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"] = 250_000
		h.now = enrichSettleDeadline(h.entry(t)).Add(time.Hour)
		h.worker.tick(context.Background())
		got, _, _ := h.queue.Get("ownmd5")
		if got.Status != persist.EnrichDone || len(h.placed) != 0 {
			t.Fatalf("entry: %+v placed: %v", got, h.placed)
		}
	})
}

func TestFileMD5AndSize(t *testing.T) {
	dir := t.TempDir()
	if err := iofacade.AllowDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "x.rep")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum, size, err := fileMD5AndSize(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum != "5d41402abc4b2a76b9719d911017c592" || size != 5 {
		t.Fatalf("got %q %d", sum, size)
	}
	if _, _, err := fileMD5AndSize(filepath.Join(dir, "missing.rep")); err == nil {
		t.Fatal("expected error for a missing file")
	}
}

func TestPlaceCompleteReplayWritesFile(t *testing.T) {
	dir := t.TempDir()
	if err := iofacade.AllowDir(dir); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "bgh-complete.rep")
	if err := placeCompleteReplay(dest, []byte("bytes")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "bytes" {
		t.Fatalf("read back: %q %v", data, err)
	}
}

func TestEnrichDepsWiring(t *testing.T) {
	d := newTestDashboardWithReplays(t)
	// Without a library runtime the worker never starts.
	d.startBnetEnrich(context.Background())

	lib := library.New(library.Options{})
	t.Cleanup(lib.Close)
	root := t.TempDir()
	if err := iofacade.AllowDir(root); err != nil {
		t.Fatal(err)
	}
	d.library = &libraryRuntime{root: root, lib: lib}
	ctx, cancel := context.WithCancel(context.Background())
	d.startBnetEnrich(ctx)
	cancel()

	deps := d.enrichDeps()
	if deps.now().IsZero() {
		t.Fatal("now")
	}
	if deps.enabled() {
		t.Fatal("enrichment enabled without the labs flag")
	}
	deps.logf("must be callable when --debug is off")
	if _, ok := deps.bridgeAddr(); ok {
		t.Fatal("bridge reported connected with no address")
	}
	if gws := deps.gatewayCandidates(); len(gws) != len(bnetfacade.GatewayNames) {
		t.Fatalf("gateway candidates: %v", gws)
	}
	if deps.youKeys() != nil {
		t.Fatal("you keys without CSettings")
	}
	if deps.snapshot() == nil || deps.complete() {
		t.Fatal("snapshot/progress wiring")
	}
	if _, err := deps.profile(context.Background(), "me", 30); err == nil {
		t.Fatal("profile fetch should fail with no bridge")
	}
	if _, err := deps.gameInfo(context.Background(), "127.0.0.1:1", "MM-1"); err == nil {
		t.Fatal("gameinfo should fail against a closed port")
	}
	if _, err := deps.headSize(context.Background(), "/outside"); err == nil {
		t.Fatal("head should refuse a path outside the GCS prefix")
	}
	if _, err := deps.download(context.Background(), "/outside"); err == nil {
		t.Fatal("download should refuse a path outside the GCS prefix")
	}
}

func TestEnrichGatewaySweepFindsOwnAccount(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.info = &bnetfacade.GameInfo{}
	// The user last played on gateway 11, but candidates lead with 10 and 30:
	// only the qualifying gateway's profile carries md5+url.
	var fetched []int64
	h.worker.deps.gatewayCandidates = func() []int64 { return []int64{10, 30, 11} }
	h.worker.deps.profile = func(_ context.Context, _ string, gateway int64) (enrichProfile, error) {
		fetched = append(fetched, gateway)
		if gateway != 11 {
			return enrichProfile{FetchedAt: h.now}, nil
		}
		return enrichProfile{
			Replays:   []bnetfacade.ProfileReplay{{MD5: "ownmd5", Link: "MM-GAME-1", URL: "https://x"}},
			FetchedAt: h.now,
		}, nil
	}
	h.worker.tick(context.Background())

	entry := h.entry(t)
	if entry.Status != persist.EnrichWaiting || entry.GameID != "MM-GAME-1" {
		t.Fatalf("entry: %+v", entry)
	}
	if len(fetched) != 3 || fetched[2] != 11 {
		t.Fatalf("sweep order: %v", fetched)
	}
	if h.worker.resolvedGateway["me"] != 11 {
		t.Fatalf("resolved gateway: %v", h.worker.resolvedGateway)
	}

	// The next resolve goes straight to the remembered gateway.
	fetched = nil
	h.eligibleReplay(t, librarytest.WithPath("/replays/bgh2.rep", h.now))
	h.worker.deps.fileMD5 = func(string) (string, int64, error) { return "ownmd5b", 100_000, nil }
	h.now = h.now.Add(enrichProfileMinInterval + time.Minute)
	h.worker.tick(context.Background())
	if len(fetched) != 1 || fetched[0] != 11 {
		t.Fatalf("post-resolve fetches: %v", fetched)
	}
}

func TestEnrichGatewaySweepBacksOffWhenNothingQualifies(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.worker.deps.gatewayCandidates = func() []int64 { return []int64{10, 11} }
	calls := 0
	h.worker.deps.profile = func(context.Context, string, int64) (enrichProfile, error) {
		calls++
		return enrichProfile{FetchedAt: h.now}, nil
	}
	h.worker.tick(context.Background())
	if calls != 2 {
		t.Fatalf("sweep calls: %d", calls)
	}
	// Within the backoff no further profile fetches happen.
	h.now = h.now.Add(enrichSweepBackoff - time.Hour)
	h.worker.tick(context.Background())
	if calls != 2 {
		t.Fatalf("fetched during backoff: %d", calls)
	}
	entry := h.entry(t)
	if entry.Status != persist.EnrichNeedGameID {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestEnrichDisabledFlagIsInert(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.worker.deps.enabled = func() bool { return false }
	h.worker.deps.profile = func(context.Context, string, int64) (enrichProfile, error) {
		t.Fatal("profile fetched while the flag is off")
		return enrichProfile{}, nil
	}
	h.worker.tick(context.Background())
	if _, ok, _ := h.queue.Get("ownmd5"); ok {
		t.Fatal("game queued while the flag is off")
	}
}

func TestEnrichSettledGameDownloadsOnFirstPoll(t *testing.T) {
	h := newEnrichHarness(t)
	// Played yesterday: well past the settle window, so the first sighting of
	// a longer copy downloads without a confirmation round trip.
	entry := persist.EnrichEntry{
		MD5: "ownmd5", Path: "/replays/bgh.rep", SizeBytes: 100_000,
		ReplayDate: h.now.Add(-30 * time.Hour), UserLeftAt: h.now.Add(-29 * time.Hour),
		Toon: "me", GameID: "MM-1", Status: persist.EnrichWaiting, NextPollAt: h.now,
	}
	if err := h.queue.Upsert(entry); err != nil {
		t.Fatal(err)
	}
	h.info = &bnetfacade.GameInfo{Replays: []bnetfacade.GameInfoReplay{{MD5: "other", URL: testGCSURL}}}
	h.sizes["/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"] = 250_000
	h.worker.tick(context.Background())
	got := h.entry(t)
	if got.Status != persist.EnrichDone {
		t.Fatalf("entry: %+v", got)
	}
	if string(h.placed["/replays/bgh-complete.rep"]) != "replay-bytes" {
		t.Fatalf("placed: %v", h.placed)
	}
}

func TestEnrichRestartForgivesStaleSettledSchedules(t *testing.T) {
	h := newEnrichHarness(t)
	settled := persist.EnrichEntry{
		MD5: "ownmd5", Path: "/replays/bgh.rep", SizeBytes: 100_000,
		ReplayDate: h.now.Add(-30 * time.Hour), UserLeftAt: h.now.Add(-29 * time.Hour),
		Toon: "me", GameID: "MM-OLD", Status: persist.EnrichWaiting,
		NextPollAt: h.now.Add(10 * time.Hour), PollErrors: 1,
	}
	fresh := persist.EnrichEntry{
		MD5: "freshmd5", Path: "/replays/fresh.rep", SizeBytes: 100_000,
		ReplayDate: h.now.Add(-40 * time.Minute), UserLeftAt: h.now.Add(-30 * time.Minute),
		Toon: "me", GameID: "MM-NEW", Status: persist.EnrichWaiting,
		NextPollAt: h.now.Add(10 * time.Minute), LastBestSize: 250_000,
	}
	for _, entry := range []persist.EnrichEntry{settled, fresh} {
		if err := h.queue.Upsert(entry); err != nil {
			t.Fatal(err)
		}
	}
	h.info = &bnetfacade.GameInfo{Replays: []bnetfacade.GameInfoReplay{{MD5: "other", URL: testGCSURL}}}
	h.sizes["/starcraft-user-uploads-prod/S1-replays/1/2/other.replay"] = 250_000
	h.worker.tick(context.Background())

	// The settled game's hours-out schedule is forgiven and resolves now.
	got := h.entry(t)
	if got.Status != persist.EnrichDone {
		t.Fatalf("settled entry: %+v", got)
	}
	// The fresh game keeps its confirmation wait: pulling it forward would
	// fake the stability check.
	freshGot, _, _ := h.queue.Get("freshmd5")
	if freshGot.Status != persist.EnrichWaiting || !freshGot.NextPollAt.Equal(fresh.NextPollAt) {
		t.Fatalf("fresh entry: %+v", freshGot)
	}
}

func TestEnrichResolvesEmptyLinkViaGameResults(t *testing.T) {
	h := newEnrichHarness(t)
	h.eligibleReplay(t)
	h.info = &bnetfacade.GameInfo{}
	// The replays[] entry matches by md5 but carries no link (observed live);
	// the payload's game_results names the game 8 seconds off the upload stamp.
	h.profile = enrichProfile{
		Replays: []bnetfacade.ProfileReplay{
			{MD5: "ownmd5", URL: "https://x", Link: "", CreateTime: 1_788_724_490},
		},
		GameRefs: []bnetfacade.ProfileGameRef{
			{GameID: "1715169733", CreateTime: 1_788_724_482},
			{GameID: "1715160000", CreateTime: 1_788_723_000},
		},
		FetchedAt: h.now,
	}
	h.worker.tick(context.Background())
	entry := h.entry(t)
	if entry.Status != persist.EnrichWaiting || entry.GameID != "1715169733" {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestNearestGameRefTolerance(t *testing.T) {
	refs := []bnetfacade.ProfileGameRef{{GameID: "far", CreateTime: 1000}}
	if got := nearestGameRef(refs, 1500); got != "" {
		t.Fatalf("matched outside tolerance: %q", got)
	}
	if got := nearestGameRef(refs, 1100); got != "far" {
		t.Fatalf("missed inside tolerance: %q", got)
	}
	if got := nearestGameRef(nil, 1000); got != "" {
		t.Fatalf("matched with no refs: %q", got)
	}
}
