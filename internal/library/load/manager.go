package load

import (
	"context"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/watch"
)

// ManagerOptions configure the whole load-and-watch lifecycle. The zero value
// is usable once Folder is set.
type ManagerOptions struct {
	Folder string
	Loader Options
	Watch  watch.Options
	Log    func(LogEvent)
}

// Manager owns the folder the library mirrors: it loads it, watches it, and
// swaps to a different folder without ever blanking the running dashboard.
type Manager struct {
	lib  *library.Library
	opts ManagerOptions

	mu         sync.Mutex
	folder     string
	generation uint64
	loader     *Loader
	cancel     context.CancelFunc
	watcher    *watch.Watcher
	done       chan struct{}
	closed     bool
}

func NewManager(lib *library.Library, opts ManagerOptions) *Manager {
	return &Manager{lib: lib, opts: opts, folder: opts.Folder}
}

// Start loads the configured folder in the background and begins watching it.
// It returns as soon as the load is under way so the server can start serving.
func (m *Manager) Start(ctx context.Context) error {
	return m.restart(ctx, m.Folder(), false)
}

// SetFolder points the library at a different folder. The current corpus keeps
// serving until the newest replays of the new folder have loaded.
func (m *Manager) SetFolder(ctx context.Context, folder string) error {
	if err := fileops.ValidateReplayDir(folder); err != nil {
		return err
	}
	if err := iofacade.AllowDir(folder); err != nil {
		return err
	}
	return m.restart(ctx, folder, true)
}

// Rescan re-reads the folder now instead of waiting for the next check.
func (m *Manager) Rescan() {
	m.mu.Lock()
	watcher := m.watcher
	m.mu.Unlock()
	if watcher != nil {
		watcher.Reconcile()
	}
}

func (m *Manager) Folder() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.folder
}

func (m *Manager) Generation() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.generation
}

// Close stops the current load and watcher and waits for them to finish.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	cancel, done := m.cancel, m.done
	m.cancel, m.done, m.watcher = nil, nil, nil
	m.mu.Unlock()
	stop(cancel, done)
}

func (m *Manager) restart(ctx context.Context, folder string, staged bool) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	previousCancel, previousDone := m.cancel, m.done
	m.mu.Unlock()
	stop(previousCancel, previousDone)

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		cancel()
		close(done)
		return nil
	}
	m.generation++
	generation := m.generation
	m.folder = folder
	loaderOpts := m.opts.Loader
	loaderOpts.Folder = folder
	loaderOpts.Generation = generation
	loaderOpts.Staged = staged
	if loaderOpts.Log == nil {
		loaderOpts.Log = m.opts.Log
	}
	loader := New(m.lib, loaderOpts)
	m.loader = loader
	m.cancel = cancel
	m.done = done
	m.mu.Unlock()

	go func() {
		defer close(done)
		if err := loader.Run(runCtx); err != nil || runCtx.Err() != nil {
			return
		}
		m.watch(runCtx, folder, generation, loader)
	}()
	return nil
}

func (m *Manager) watch(ctx context.Context, folder string, generation uint64, loader *Loader) {
	watchOpts := m.opts.Watch
	watchOpts.Folder = folder
	if watchOpts.Log == nil && m.opts.Log != nil {
		log := m.opts.Log
		watchOpts.Log = func(message string) { log(LogEvent{Level: LogLevelWarn, Message: message}) }
	}
	watcher := watch.New(watchOpts, m, func(change watch.Change) {
		m.apply(ctx, generation, loader, change)
	})

	m.mu.Lock()
	if m.closed || m.generation != generation {
		m.mu.Unlock()
		return
	}
	m.watcher = watcher
	m.mu.Unlock()

	loader.SetPhase(library.PhaseWatching)
	_ = watcher.Run(ctx)
}

func (m *Manager) apply(ctx context.Context, generation uint64, loader *Loader, change watch.Change) {
	for _, path := range change.Removed {
		m.lib.Remove(generation, path)
	}
	if len(change.Removed) > 0 {
		m.lib.Flush()
		loader.SetPhase(library.PhaseWatching)
	}
	if len(change.Changed) > 0 {
		loader.Process(ctx, change.Changed)
		loader.SetPhase(library.PhaseWatching)
	}
}

// Known implements watch.Corpus.
func (m *Manager) Known(path string, size int64, modTime time.Time) bool {
	return knownFile(m.lib.Snapshot(), path, size, modTime)
}

// Paths implements watch.Corpus.
func (m *Manager) Paths() []string {
	snap := m.lib.Snapshot()
	paths := make([]string, 0, snap.Len())
	for _, record := range snap.Replays {
		for _, ref := range record.Paths {
			paths = append(paths, ref.Path)
		}
	}
	return paths
}

func knownFile(snap *library.Snapshot, path string, size int64, modTime time.Time) bool {
	record, ok := snap.ByPath(path)
	if !ok {
		return false
	}
	for _, ref := range record.Paths {
		if ref.Path == path {
			return ref.Size == size && ref.ModTime.Equal(modTime)
		}
	}
	return false
}

func stop(cancel context.CancelFunc, done chan struct{}) {
	if cancel == nil {
		return
	}
	cancel()
	if done != nil {
		<-done
	}
}
