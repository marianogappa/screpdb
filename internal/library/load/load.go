// Package load builds the in-memory replay library from a folder of replay
// files. It scans newest first, parses on every core, compacts each replay and
// commits batches as they complete, so the dashboard has a usable corpus about
// a second after startup while the rest streams in behind it.
package load

import (
	"context"
	"errors"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/library"
)

const (
	defaultRecentCount = 100
	defaultPublishRate = 100 * time.Millisecond
)

type Options struct {
	Folder     string
	Generation uint64
	// Workers defaults to one less than GOMAXPROCS so HTTP keeps a core.
	Workers int
	// RecentCount is how many of the newest replays load before the rest.
	RecentCount int
	// Staged loads into a hidden working set and promotes it once the recent
	// phase commits, so a folder change never blanks the running dashboard.
	Staged bool
	// PublishRate throttles progress events while a phase runs.
	PublishRate time.Duration
	Log         func(LogEvent)
}

func (o Options) withDefaults() Options {
	if o.Workers <= 0 {
		o.Workers = max(1, runtime.GOMAXPROCS(0)-1)
	}
	if o.RecentCount <= 0 {
		o.RecentCount = defaultRecentCount
	}
	if o.PublishRate <= 0 {
		o.PublishRate = defaultPublishRate
	}
	return o
}

type Loader struct {
	lib  *library.Library
	opts Options

	seen  sync.Map
	phase atomic.Value

	aliasMu sync.Mutex
	aliases map[[32]byte][]library.FileRef

	total   atomic.Int64
	loaded  atomic.Int64
	failed  atomic.Int64
	skipped atomic.Int64
}

func New(lib *library.Library, opts Options) *Loader {
	l := &Loader{lib: lib, opts: opts.withDefaults()}
	l.phase.Store(library.PhaseIdle)
	return l
}

// Run loads the whole folder. It returns once every file has been handled or
// ctx is cancelled; a cancelled run leaves whatever it committed in place.
func (l *Loader) Run(ctx context.Context) error {
	l.setPhase(library.PhaseScanning)
	l.publish()

	files, err := fileops.ScanReplayFolder(l.opts.Folder)
	if err != nil {
		l.setPhase(library.PhaseFailed)
		l.publish()
		l.logf(LogLevelError, "Could not read %s: %v", l.opts.Folder, err)
		return err
	}
	l.total.Store(int64(len(files)))
	l.logf(LogLevelInfo, "Found %d replays in %s", len(files), l.opts.Folder)

	// A staged load fills a hidden working set so the folder currently on
	// screen keeps serving; a load into a generation that is not live yet
	// starts that generation empty. A rescan of the live generation adds to
	// what is already there.
	switch {
	case l.opts.Staged:
		l.lib.Stage(l.opts.Generation)
	case l.lib.Snapshot().Generation != l.opts.Generation:
		l.lib.Reset(l.opts.Generation)
	}

	stopPublishing := l.publishPeriodically(ctx)
	defer stopPublishing()

	recent := min(l.opts.RecentCount, len(files))
	l.setPhase(library.PhaseRecent)
	l.runPhase(ctx, files[:recent])
	l.lib.Flush()
	l.flushAliases(l.opts.Generation)
	if l.opts.Staged {
		l.lib.Promote(l.opts.Generation)
		l.lib.Flush()
	}
	l.publish()
	if ctx.Err() != nil {
		return ctx.Err()
	}

	l.setPhase(library.PhaseBackfill)
	l.runPhase(ctx, files[recent:])
	l.lib.Flush()
	l.flushAliases(l.opts.Generation)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// The parse of each replay allocates megabytes that die immediately after
	// compaction; hand that back rather than sitting on a load-time peak.
	debug.FreeOSMemory()
	l.setPhase(library.PhaseReady)
	l.publish()
	l.logf(LogLevelSuccess, "Loaded %d replays (%d skipped, %d failed)", l.loaded.Load(), l.skipped.Load(), l.failed.Load())
	return nil
}

// Process handles an explicit set of files in the current generation, for the
// watcher. It reuses the same dedup, panic isolation and counters as a full run.
func (l *Loader) Process(ctx context.Context, files []fileops.FileInfo) {
	if len(files) == 0 {
		return
	}
	l.total.Add(int64(len(files)))
	l.runPhase(ctx, files)
	l.lib.Flush()
	l.flushAliases(l.opts.Generation)
	l.publish()
}

func (l *Loader) runPhase(ctx context.Context, files []fileops.FileInfo) {
	if len(files) == 0 {
		return
	}
	jobs := make(chan fileops.FileInfo)
	var wg sync.WaitGroup
	for range l.opts.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				l.handle(ctx, file)
			}
		}()
	}
	for _, file := range files {
		select {
		case jobs <- file:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}

func (l *Loader) handle(ctx context.Context, file fileops.FileInfo) {
	if ctx.Err() != nil {
		return
	}
	var result outcome
	err := guarded(func() error {
		var err error
		result, err = l.process(ctx, l.opts.Generation, file)
		return err
	})
	switch {
	case err == nil:
		l.loaded.Add(1)
		if result != outcomeLoaded {
			l.skipped.Add(1)
		}
	case ctx.Err() != nil:
		return
	case errors.Is(err, errUnsupportedUMS):
		l.loaded.Add(1)
		l.skipped.Add(1)
	default:
		l.failed.Add(1)
		l.logf(LogLevelWarn, "Could not read %s: %v", file.Name, err)
	}
}

func (l *Loader) publishPeriodically(ctx context.Context) func() {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(l.opts.PublishRate)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				l.publish()
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

func (l *Loader) publish() { l.lib.SetProgress(l.Progress()) }

// Progress is the loader's current counters. Loaded counts files handled
// without error, of which Skipped produced no new record (duplicate content,
// already in the corpus, or use map settings).
func (l *Loader) Progress() library.ProgressState {
	return library.ProgressState{
		Generation: l.opts.Generation,
		Version:    l.lib.Snapshot().Version,
		Folder:     l.opts.Folder,
		Phase:      l.currentPhase(),
		Total:      int(l.total.Load()),
		Loaded:     int(l.loaded.Load()),
		Failed:     int(l.failed.Load()),
		Skipped:    int(l.skipped.Load()),
	}
}

// SetPhase moves the loader to a terminal phase once a run is over, so the
// watcher can report that it is live.
func (l *Loader) SetPhase(phase library.Phase) {
	l.setPhase(phase)
	l.publish()
}

func (l *Loader) setPhase(phase library.Phase) { l.phase.Store(phase) }

func (l *Loader) currentPhase() library.Phase {
	phase, _ := l.phase.Load().(library.Phase)
	return phase
}
