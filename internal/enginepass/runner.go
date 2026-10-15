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

const pausedRecheck = 10 * time.Second

type Options struct {
	Library   *library.Library
	CachePath string
	// Workers is the number of parallel engine passes; 0 leaves at least one core free.
	Workers int
	// OpenData loads the game data; nil means engine.OpenGameData, which finds the SC:R install.
	OpenData func() (*engine.Data, error)
	// Paused is checked before each engine pass. Cache hits are applied even while paused.
	Paused func() bool
	// LowPriority runs workers on low-priority OS threads; off in tests.
	LowPriority bool
	Logf        func(format string, args ...any)
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

	mu       sync.Mutex
	queue    *queue
	status   Status
	data     *engine.Data
	dataHash [32]byte
	cache    *digestcache.Cache

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
}

func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.status
	s.Queued = r.queue.len()
	s.Paused = s.State == StateRunning && r.opts.Paused()
	return s
}

// Focus moves a replay to the front of the queue: the game the user has open upgrades next.
func (r *Runner) Focus(replayID int64) {
	rec, ok := r.lib.Snapshot().ByID(replayID)
	if !ok {
		return
	}
	r.mu.Lock()
	r.queue.setFocus(rec.Checksum)
	r.mu.Unlock()
	r.signal()
}

func (r *Runner) setState(s State, reason string) {
	r.mu.Lock()
	r.status.State, r.status.Reason = s, reason
	r.mu.Unlock()
}

func (r *Runner) run() {
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
	cache, err := digestcache.Open(r.opts.CachePath, func(k digestcache.Key) bool {
		return k.EngineVersion == engine.EngineVersion && k.DataHash == dataHash
	})
	if err != nil {
		r.setState(StateFailed, err.Error())
		r.opts.Logf("Engine pass off: %v", err)
		return
	}
	r.mu.Lock()
	r.data, r.dataHash, r.cache = data, dataHash, cache
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
	r.lib.SetEngineData(checksum, Summarize(d))
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
	r.lib.SetEngineData(j.checksum, Summarize(d))
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
// changes reach users without new engine passes.
func Summarize(d *engine.Digest) *library.EngineData {
	data := &library.EngineData{
		EngineVersion: d.EngineVersion,
		LowConfidence: d.Confidence() == engine.ConfidenceLow,
		TrustedUntil:  uint32(max(d.TrustedUntil(), 0)),
	}
	for _, p := range macro.Compute(d) {
		data.Players = append(data.Players, enginePlayer(p))
	}
	return data
}

func enginePlayer(p macro.Player) library.EnginePlayer {
	income := make([]uint16, len(p.Income))
	for i, v := range p.Income {
		income[i] = uint16(min(max(v, 0), math.MaxUint16))
	}
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
		Income:               income,
	}
}
