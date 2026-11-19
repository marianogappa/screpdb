//go:build !js

package dashboard

import (
	"path/filepath"
	"sync"

	"github.com/marianogappa/screpdb/internal/enginepass"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/power"
)

const (
	engineDigestCacheFile   = "engine_digests.bin"
	engineSnapshotCacheFile = "engine_snapshots.bin"
)

// enginePassHolder guards the runner: shutdown clears it while requests read it.
type enginePassHolder struct {
	mu     sync.Mutex
	runner *enginepass.Runner
	// starCraftRunning is power.StarCraftRunning outside tests.
	starCraftRunning func() bool
}

func (h *enginePassHolder) get() *enginepass.Runner {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.runner
}

// startEnginePass upgrades the library to engine data in the background.
// Without a StarCraft: Remastered install it only logs: the fast pass alone keeps the dashboard working.
func (d *Dashboard) startEnginePass() {
	if !d.enginePassEnabled || d.library == nil {
		return
	}
	d.enginePass.mu.Lock()
	defer d.enginePass.mu.Unlock()
	if d.enginePass.runner != nil {
		return
	}
	runner := enginepass.New(enginepass.Options{
		Library:      d.library.lib,
		CachePath:    filepath.Join(d.library.root, engineDigestCacheFile),
		SnapshotPath: filepath.Join(d.library.root, engineSnapshotCacheFile),
		Workers:      d.enginePassWorkers,
		LowPriority:  true,
		Features:     d.engineFeatures,
	})
	d.enginePass.runner = runner
	d.libraryHub.setEngineState(func() string { return enginePassState(runner) })
	runner.Start()
}

// enginePassWorkers keeps the engine pass off the cores StarCraft: Remastered needs while someone plays,
// and, when the user opts in, takes the whole machine while the game is closed.
func (d *Dashboard) enginePassWorkers() int {
	if !d.featureFlagEnabled(d.ctx, featureFlagEnginePassAllCores) {
		return enginepass.DefaultWorkers()
	}
	running := d.enginePass.starCraftRunning
	if running == nil {
		running = power.StarCraftRunning
	}
	if running() {
		return enginepass.DefaultWorkers()
	}
	return enginepass.AllWorkers()
}

func (d *Dashboard) focusEnginePass(replayID int64) {
	if runner := d.enginePass.get(); runner != nil {
		runner.Focus(replayID)
	}
}

// enginePending reports whether the game is waiting for, or in, its engine pass.
func (d *Dashboard) enginePending(replayID int64) bool {
	runner := d.enginePass.get()
	return runner != nil && runner.Pending(replayID)
}

// engineUpgrading reports whether a game with engine data is being redone on a newer engine version.
func (d *Dashboard) engineUpgrading(replayID int64) bool {
	runner := d.enginePass.get()
	return runner != nil && runner.Upgrading(replayID)
}

// completeFallback is the user's own recording to show in place of a game's complete copy (issue #341) when
// the engine could not run the complete copy but may run the own one: a shorter recording can stop before
// the point where the longer one drifted. replayID may be either copy, as the page refreshes by the copy it
// shows.
func (d *Dashboard) completeFallback(replayID int64) (int64, bool) {
	runner := d.enginePass.get()
	if runner == nil || d.library == nil {
		return 0, false
	}
	snap := d.library.lib.Snapshot()
	rec, ok := snap.ByID(replayID)
	if !ok {
		return 0, false
	}
	complete, own := rec, rec
	if ownPath, isComplete := library.CompleteBasePath(rec.Path()); isComplete {
		if own, ok = snap.ByPath(ownPath); !ok {
			return 0, false
		}
	} else if complete, ok = snap.ByPath(library.CompletePathFor(rec.Path())); !ok || complete == rec {
		return 0, false
	}
	if !runner.Unavailable(complete.ID) || runner.Unavailable(own.ID) {
		return 0, false
	}
	return own.ID, true
}

func (d *Dashboard) engineUnavailable(replayID int64) bool {
	runner := d.enginePass.get()
	return runner != nil && runner.Unavailable(replayID)
}

func (d *Dashboard) closeEnginePass() {
	d.enginePass.mu.Lock()
	runner := d.enginePass.runner
	d.enginePass.runner = nil
	d.enginePass.mu.Unlock()
	if runner != nil {
		runner.Close()
	}
}

func enginePassState(r *enginepass.Runner) string {
	s := r.Status()
	switch {
	case s.State == enginepass.StateOff:
		return engineStateOff
	case s.State == enginepass.StateFailed:
		return engineStateFailed
	case s.State == enginepass.StateStarting:
		return engineStateStarting
	default:
		return engineStateRunning
	}
}
