// Package enginepass upgrades the in-memory library from command-inferred ("fast") data to engine data,
// one replay at a time, in the background. Digests are cached on disk, so a warm launch upgrades every
// replay in seconds and only new replays, or an EngineVersion bump, cost engine passes.
package enginepass

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/digestcache"
	"github.com/marianogappa/screpdb/internal/engine/macro"
	"github.com/marianogappa/screpdb/internal/gamedata"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/power"
)

const (
	pausedRecheck = 10 * time.Second
	libraryPoll   = 500 * time.Millisecond
)

type Options struct {
	Library   *library.Library
	CachePath string
	// SnapshotPath holds exact snapshots of the games someone opened; empty disables them.
	SnapshotPath string
	// Workers is the number of parallel engine passes; 0 leaves at least one core free.
	Workers int
	// OpenData loads the game data; nil means engine.OpenGameData, which finds the SC:R install.
	OpenData func() (*engine.Data, error)
	// Paused is checked before each engine pass. Cache hits are applied even while paused.
	Paused func() bool
	// LowPriority runs workers on low-priority OS threads; off in tests.
	LowPriority bool
	Logf        func(format string, args ...any)
	// Features names the game events of a trusted digest that the games list shows; nil leaves them out.
	Features func(checksum [32]byte, d *engine.Digest) []string
}

type State uint8

const (
	StateStarting State = iota
	// StateOff means there is no StarCraft: Remastered install: only the fast pass runs.
	StateOff
	StateRunning
	StateFailed
)

type Status struct {
	State  State
	Reason string
	Queued int
	Paused bool
}

type Runner struct {
	opts Options
	lib  *library.Library

	mu        sync.Mutex
	queue     *queue
	running   map[[32]byte]bool
	status    Status
	data      *engine.Data
	dataHash  [32]byte
	cache     *digestcache.Cache
	snapCache *digestcache.Cache
	snapLocks map[[32]byte]*sync.Mutex

	wake     chan struct{}
	stop     chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func DefaultWorkers() int { return max(1, runtime.NumCPU()/2-1) }

func New(opts Options) *Runner {
	if opts.Workers <= 0 {
		opts.Workers = DefaultWorkers()
	}
	if opts.OpenData == nil {
		opts.OpenData = engine.OpenGameData
	}
	if opts.Paused == nil {
		opts.Paused = func() bool { return false }
	}
	if opts.Logf == nil {
		opts.Logf = log.Printf
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Runner{opts: opts, lib: opts.Library, queue: newQueue(), wake: make(chan struct{}, 1), stop: make(chan struct{}), ctx: ctx, cancel: cancel}
}

// Start prepares the game data and the cache, then upgrades the library until Close. Without a
// StarCraft: Remastered install it stops at once in StateOff.
func (r *Runner) Start() {
	r.wg.Go(func() {
		defer crashreport.GuardNonFatal(func() { r.setState(StateFailed, "the engine pass stopped after an internal error") })
		r.run()
	})
}

func (r *Runner) Close() {
	r.stopOnce.Do(func() {
		close(r.stop)
		r.cancel()
	})
	r.wg.Wait()
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cache != nil {
		r.cache.Close()
	}
	if r.snapCache != nil {
		r.snapCache.Close()
	}
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.status
	s.Queued = r.queue.len()
	s.Paused = s.State == StateRunning && r.opts.Paused()
	return s
}

// Digest reads a replay's full digest from the cache, for pages that need more than the summary.
func (r *Runner) Digest(checksum [32]byte) (*engine.Digest, bool) {
	r.mu.Lock()
	cache := r.cache
	r.mu.Unlock()
	if cache == nil {
		return nil, false
	}
	entry, ok, err := cache.Get(r.key(checksum))
	if err != nil || !ok || entry.Failed != "" {
		return nil, false
	}
	d, err := engine.Decode(entry.Digest)
	return d, err == nil
}

var ErrNotReady = errors.New("enginepass: the engine is not running")

// Snapshots gives the exact map at each requested second of a replay. Seconds already cached cost
// nothing; the rest cost one engine pass up to the last of them, run on the caller's goroutine because
// someone is waiting for it. Concurrent requests for one replay share that pass.
func (r *Runner) Snapshots(ctx context.Context, checksum [32]byte, path string, seconds []int) (map[int]engine.Snapshot, error) {
	r.mu.Lock()
	data, snapCache := r.data, r.snapCache
	if data == nil || snapCache == nil {
		r.mu.Unlock()
		return nil, ErrNotReady
	}
	lock, ok := r.snapLocks[checksum]
	if !ok {
		lock = &sync.Mutex{}
		r.snapLocks[checksum] = lock
	}
	r.mu.Unlock()
	lock.Lock()
	defer lock.Unlock()

	key := r.key(checksum)
	have := map[int]engine.Snapshot{}
	if entry, ok, err := snapCache.Get(key); err == nil && ok && entry.Failed == "" {
		if decoded, err := engine.DecodeSnapshots(entry.Digest); err == nil {
			have = decoded
		}
	}
	var missing []int
	for _, s := range seconds {
		if _, ok := have[s]; !ok {
			missing = append(missing, s)
		}
	}
	if len(missing) > 0 {
		b, err := iofacade.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if sha256.Sum256(b) != checksum {
			return nil, fmt.Errorf("enginepass: %s changed since it was loaded", path)
		}
		fresh, err := engine.Snapshots(ctx, b, data, missing)
		if err != nil {
			return nil, err
		}
		for s, snap := range fresh {
			have[s] = snap
		}
		if encoded, err := engine.EncodeSnapshots(have); err == nil {
			if err := snapCache.Put(key, digestcache.Entry{Digest: encoded}); err != nil {
				r.opts.Logf("Engine snapshots: %v", err)
			}
		}
	}
	out := make(map[int]engine.Snapshot, len(seconds))
	for _, s := range seconds {
		if snap, ok := have[s]; ok {
			out[s] = snap
		}
	}
	return out, nil
}

// Focus moves a replay to the front of the queue: the game the user has open upgrades next.
// Focus runs the replay the user has open now, on a worker of its own, instead of after the passes already
// running: a long team game on every worker would otherwise keep the open game waiting for minutes. It runs
// on battery too: pausing on battery is for the background queue, not for the game someone just opened.
func (r *Runner) Focus(replayID int64) {
	rec, ok := r.lib.Snapshot().ByID(replayID)
	if !ok {
		return
	}
	r.mu.Lock()
	j, queued := r.queue.byChecksum[rec.Checksum]
	if queued && r.status.State == StateRunning {
		r.queue.remove(rec.Checksum)
		r.mu.Unlock()
		r.wg.Go(func() { r.process(j) })
		return
	}
	r.queue.setFocus(rec.Checksum)
	r.mu.Unlock()
	r.signal()
}

// Pending reports whether the replay is waiting for, or in, an engine pass.
func (r *Runner) Pending(replayID int64) bool {
	rec, ok := r.lib.Snapshot().ByID(replayID)
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, queued := r.queue.byChecksum[rec.Checksum]
	return queued || r.running[rec.Checksum]
}

// Unavailable reports whether the engine pass settled the replay without data the dashboard can use:
// the engine could not run it, or it drifted from the real game.
func (r *Runner) Unavailable(replayID int64) bool {
	rec, ok := r.lib.Snapshot().ByID(replayID)
	if !ok {
		return false
	}
	if rec.EngineData != nil {
		return rec.EngineData.LowConfidence
	}
	r.mu.Lock()
	cache, running := r.cache, r.status.State == StateRunning
	r.mu.Unlock()
	if !running || cache == nil {
		return false
	}
	entry, ok, err := cache.Get(r.key(rec.Checksum))
	return err == nil && ok && entry.Failed != ""
}

func (r *Runner) setState(s State, reason string) {
	r.mu.Lock()
	r.status.State, r.status.Reason = s, reason
	r.mu.Unlock()
}

// Engine work starts only once the library has read the whole folder: the fast pass must load as fast
// as it does without the engine, and extracting game data or decoding digests would compete with it.
func (r *Runner) waitForLibrary() bool {
	for !r.lib.Progress().Complete() {
		select {
		case <-r.stop:
			return false
		case <-time.After(libraryPoll):
		}
	}
	return true
}

func (r *Runner) run() {
	if !r.waitForLibrary() {
		return
	}
	data, err := r.opts.OpenData()
	if errors.Is(err, gamedata.ErrNoInstall) {
		r.setState(StateOff, "no StarCraft: Remastered install found")
		r.opts.Logf("Engine pass off: no StarCraft: Remastered install found; using command-inferred data only")
		return
	}
	if err != nil {
		r.setState(StateFailed, err.Error())
		r.opts.Logf("Engine pass off: %v", err)
		return
	}
	var dataHash [32]byte
	if _, err := hex.Decode(dataHash[:], []byte(data.FilesHash())); err != nil {
		r.setState(StateFailed, "game data hash: "+err.Error())
		return
	}
	current := func(k digestcache.Key) bool {
		return k.EngineVersion == engine.EngineVersion && k.DataHash == dataHash
	}
	cache, err := digestcache.Open(r.opts.CachePath, current)
	if err != nil {
		r.setState(StateFailed, err.Error())
		r.opts.Logf("Engine pass off: %v", err)
		return
	}
	var snapCache *digestcache.Cache
	if r.opts.SnapshotPath != "" {
		if snapCache, err = digestcache.Open(r.opts.SnapshotPath, current); err != nil {
			r.opts.Logf("Engine snapshots off: %v", err)
		}
	}
	r.mu.Lock()
	r.data, r.dataHash, r.cache, r.snapCache = data, dataHash, cache, snapCache
	r.snapLocks = map[[32]byte]*sync.Mutex{}
	r.status.State = StateRunning
	r.mu.Unlock()

	_, events, cancel := r.lib.Subscribe()
	defer cancel()
	r.enqueue(r.lib.Snapshot().Replays)
	for range r.opts.Workers {
		r.wg.Go(r.work)
	}
	for {
		select {
		case <-r.stop:
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			r.onEvent(ev)
		}
	}
}

func (r *Runner) onEvent(ev library.Event) {
	if ev.Kind != library.EventCorpus {
		return
	}
	snap := r.lib.Snapshot()
	var added []*library.Replay
	for _, id := range ev.Added {
		if rec, ok := snap.ByID(id); ok {
			added = append(added, rec)
		}
	}
	r.mu.Lock()
	for _, id := range ev.Removed {
		for checksum, j := range r.queue.byChecksum {
			if j.id == id {
				r.queue.remove(checksum)
				break
			}
		}
	}
	for _, rec := range added {
		if ownPath, ok := library.CompleteBasePath(rec.Path()); ok {
			if own, ok := snap.ByPath(ownPath); ok {
				r.queue.remove(own.Checksum)
			}
		}
	}
	r.mu.Unlock()
	r.enqueue(added)
}

// enqueue applies cached digests at once and queues the rest for an engine pass.
func (r *Runner) enqueue(records []*library.Replay) {
	queued := 0
	for _, rec := range records {
		if rec.EngineData != nil && rec.EngineData.EngineVersion == engine.EngineVersion {
			continue
		}
		// The user's own copy of a game whose complete copy is in the library is hidden: its pass would be
		// work nobody sees.
		if rec.Flags.Has(library.FlagSuperseded) {
			continue
		}
		if r.applyCached(rec.Checksum) {
			continue
		}
		r.mu.Lock()
		r.queue.push(&job{checksum: rec.Checksum, id: rec.ID, date: rec.Date, path: rec.Path()})
		r.mu.Unlock()
		queued++
	}
	if queued > 0 {
		r.signal()
	}
}

func (r *Runner) key(checksum [32]byte) digestcache.Key {
	return digestcache.Key{Checksum: checksum, EngineVersion: engine.EngineVersion, DataHash: r.dataHash}
}

// applyCached reports whether the cache settled this replay: a digest to apply, or a failed pass not to retry.
func (r *Runner) applyCached(checksum [32]byte) bool {
	entry, ok, err := r.cache.Get(r.key(checksum))
	if err != nil || !ok {
		return false
	}
	if entry.Failed != "" {
		return true
	}
	d, err := engine.Decode(entry.Digest)
	if err != nil {
		return false
	}
	r.lib.SetEngineData(checksum, r.summarize(checksum, d))
	return true
}

func (r *Runner) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runner) work() {
	if r.opts.LowPriority {
		power.LowerThreadPriority()
	}
	for {
		j := r.next()
		if j == nil {
			return
		}
		r.process(j)
	}
}

// next blocks until a job is due and the runner is not paused, or returns nil once Close is called.
func (r *Runner) next() *job {
	for {
		if r.opts.Paused() {
			select {
			case <-r.stop:
				return nil
			case <-time.After(pausedRecheck):
				continue
			}
		}
		r.mu.Lock()
		j := r.queue.pop()
		more := r.queue.len() > 0
		r.mu.Unlock()
		if j != nil {
			if more {
				r.signal()
			}
			return j
		}
		select {
		case <-r.stop:
			return nil
		case <-r.wake:
		}
	}
}

func (r *Runner) process(j *job) {
	r.mu.Lock()
	if r.running == nil {
		r.running = map[[32]byte]bool{}
	}
	if r.running[j.checksum] {
		r.mu.Unlock()
		return
	}
	r.running[j.checksum] = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.running, j.checksum)
		r.mu.Unlock()
	}()
	if r.applyCached(j.checksum) {
		return
	}
	b, err := iofacade.ReadFile(j.path)
	if err != nil || sha256.Sum256(b) != j.checksum {
		return
	}
	d, err := r.pass(b)
	if r.ctx.Err() != nil {
		return
	}
	if err != nil {
		if putErr := r.cache.Put(r.key(j.checksum), digestcache.Entry{Failed: err.Error()}); putErr != nil {
			r.opts.Logf("Engine pass: %v", putErr)
		}
		return
	}
	encoded, err := engine.Encode(d)
	if err != nil {
		return
	}
	if err := r.cache.Put(r.key(j.checksum), digestcache.Entry{Digest: encoded}); err != nil {
		r.opts.Logf("Engine pass: %v", err)
	}
	r.lib.SetEngineData(j.checksum, r.summarize(j.checksum, d))
}

func (r *Runner) summarize(checksum [32]byte, d *engine.Digest) *library.EngineData {
	money := false
	if rec, ok := r.lib.Snapshot().ByChecksum(checksum); ok {
		money = rec.MapKind == library.MapKindMoney
	}
	data := Summarize(d, money)
	if r.opts.Features != nil && !data.LowConfidence {
		data.Features = r.opts.Features(checksum, d)
	}
	return data
}

// pass turns a panic into a failed pass so one bad replay neither stops the worker nor is retried.
func (r *Runner) pass(replay []byte) (d *engine.Digest, err error) {
	defer func() {
		if p := recover(); p != nil {
			crashreport.ReportPanic(p, debug.Stack())
			err = fmt.Errorf("panic in the engine pass: %v", p)
		}
	}()
	return engine.Pass(r.ctx, replay, r.data)
}

// Summarize runs the digest detectors; it is what each launch does with a cached digest, so detector
// changes reach users without new engine passes. Money maps measure worker production to 8:00 and banked
// resources over the first 20 minutes: economies saturate early, and banking piles up late.
func Summarize(d *engine.Digest, moneyMap bool) *library.EngineData {
	data := &library.EngineData{
		EngineVersion: d.EngineVersion,
		LowConfidence: d.Confidence() == engine.ConfidenceLow,
		TrustedUntil:  uint32(max(d.TrustedUntil(), 0)),
	}
	workersUntil, banked := macro.WorkerWindowEnd, map[int8]float32{}
	if moneyMap {
		workersUntil = macro.MoneyMapWorkerWindowEnd
		for _, p := range macro.Compute(macro.FirstSeconds(d, macro.MoneyMapBankedWindowEnd)) {
			banked[p.Slot] = float32(p.FloatSeconds) / float32(max(1, p.Seconds))
		}
	}
	for _, p := range macro.Compute(d) {
		ep := enginePlayer(p)
		ep.BankedShare = float32(p.FloatSeconds) / float32(max(1, p.Seconds))
		if b, ok := banked[p.Slot]; ok {
			ep.BankedShare = b
		}
		ep.HallBusy, ep.LarvaPerHatch = float32(math.NaN()), float32(math.NaN())
		if w, ok := macro.WorkerProductionOf(d, p.Slot, workersUntil); ok {
			if w.Zerg {
				ep.LarvaPerHatch = float32(w.LarvaPerHatchery)
			} else {
				ep.HallBusy = float32(w.HallBusy)
			}
		}
		data.Players = append(data.Players, ep)
	}
	return data
}

func clampU16s(values []int) []uint16 {
	out := make([]uint16, len(values))
	for i, v := range values {
		out[i] = uint16(min(max(v, 0), math.MaxUint16))
	}
	return out
}

func enginePlayer(p macro.Player) library.EnginePlayer {
	return library.EnginePlayer{
		Slot:                 uint8(p.Slot),
		Seconds:              uint16(min(p.Seconds, math.MaxUint16)),
		SupplyBlockedSeconds: uint16(min(p.SupplyBlockedSeconds, math.MaxUint16)),
		SupplyBlocks:         uint16(min(p.SupplyBlocks, math.MaxUint16)),
		BlocksPer100Supply:   float32(p.BlocksPer100Supply),
		SpareSupply:          float32(p.SpareSupply),
		SpendingQuotient:     float32(p.SpendingQuotient),
		IncomePerMinute:      float32(p.IncomePerMinute),
		AverageUnspent:       float32(p.AverageUnspent),
		WorkerIdleShare:      float32(p.WorkerIdleShare),
		ProductionIdleShare:  float32(p.ProductionIdleShare),
		WorkersBy6Min:        uint16(min(p.WorkersBy6Min, math.MaxUint16)),
		SecondsTo40Workers:   float32(p.SecondsTo40Workers),
		FloatSeconds:         uint16(min(p.FloatSeconds, math.MaxUint16)),
		Income:               clampU16s(p.Income),
		Workers:              clampU16s(p.Workers),
	}
}
