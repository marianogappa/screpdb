// Package watch keeps the replay library in step with the folder on disk. A
// periodic reconciliation walk is the source of truth; native filesystem
// notifications, where the platform supports them, only lower the latency.
package watch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/fileops"
	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	defaultReconcileInterval       = 10 * time.Second
	defaultNativeReconcileInterval = 60 * time.Second
	defaultSettle                  = 1500 * time.Millisecond
	defaultDebounce                = 500 * time.Millisecond
)

// Corpus is what the watcher compares the folder against.
type Corpus interface {
	// Known reports whether this exact file is already in the corpus.
	Known(path string, size int64, modTime time.Time) bool
	// Paths lists every file the corpus currently holds.
	Paths() []string
}

// Change is one reconciliation result: files to load and paths that are gone.
type Change struct {
	Changed []fileops.FileInfo
	Removed []string
}

func (c Change) empty() bool { return len(c.Changed) == 0 && len(c.Removed) == 0 }

type Options struct {
	Folder string
	// ReconcileInterval is the walk cadence without native notifications.
	ReconcileInterval time.Duration
	// NativeReconcileInterval is the slower safety net when notifications work,
	// covering events the platform dropped.
	NativeReconcileInterval time.Duration
	// Settle is how long a file's size and modification time must hold still
	// before it is considered finished being written.
	Settle   time.Duration
	Debounce time.Duration
	Log      func(string)
	// NewNativeWatcher is overridable for tests; it defaults to the facade's.
	NewNativeWatcher func() (iofacade.DirWatcher, error)
}

func (o Options) withDefaults() Options {
	if o.ReconcileInterval <= 0 {
		o.ReconcileInterval = defaultReconcileInterval
	}
	if o.NativeReconcileInterval <= 0 {
		o.NativeReconcileInterval = defaultNativeReconcileInterval
	}
	if o.Settle <= 0 {
		o.Settle = defaultSettle
	}
	if o.Debounce <= 0 {
		o.Debounce = defaultDebounce
	}
	if o.NewNativeWatcher == nil {
		o.NewNativeWatcher = iofacade.NewDirWatcher
	}
	return o
}

type stamp struct {
	size    int64
	modTime time.Time
}

type pending struct {
	stamp
	since time.Time
}

type Watcher struct {
	opts     Options
	corpus   Corpus
	onChange func(Change)

	native  iofacade.DirWatcher
	watched map[string]bool
	pending map[string]pending
	// emitted remembers what was already handed over, so a file the loader
	// cannot parse is not retried on every single walk.
	emitted map[string]stamp
	now     func() time.Time
}

func New(opts Options, corpus Corpus, onChange func(Change)) *Watcher {
	return &Watcher{
		opts:     opts.withDefaults(),
		corpus:   corpus,
		onChange: onChange,
		watched:  map[string]bool{},
		pending:  map[string]pending{},
		emitted:  map[string]stamp{},
		now:      time.Now,
	}
}

// Run watches until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	interval := w.opts.ReconcileInterval
	if native, err := w.opts.NewNativeWatcher(); err == nil {
		w.native = native
		interval = w.opts.NativeReconcileInterval
		defer native.Close()
	} else if !errors.Is(err, iofacade.ErrWatchUnsupported) {
		w.logf("Watching the replay folder for changes fell back to periodic checks: %v", err)
	}

	w.watchDir(w.opts.Folder)
	w.Reconcile()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	settle := time.NewTicker(w.opts.Settle)
	defer settle.Stop()

	var debounce <-chan time.Time
	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	events, errs := w.nativeChannels()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.Reconcile()
		case <-settle.C:
			if len(w.pending) > 0 {
				w.Reconcile()
			}
		case <-debounce:
			debounce = nil
			w.Reconcile()
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if !w.interesting(event) {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(w.opts.Debounce)
			} else {
				timer.Stop()
				timer.Reset(w.opts.Debounce)
			}
			debounce = timer.C
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			w.logf("Watching the replay folder reported an error: %v", err)
		}
	}
}

func (w *Watcher) nativeChannels() (<-chan iofacade.FSEvent, <-chan error) {
	if w.native == nil {
		return nil, nil
	}
	return w.native.Events(), w.native.Errors()
}

// interesting filters the notification stream down to replay files and to
// directory changes, which may mean a new subfolder to watch.
func (w *Watcher) interesting(event iofacade.FSEvent) bool {
	name := strings.ToLower(filepath.Base(event.Path))
	if strings.HasSuffix(name, ".rep") {
		return !fileops.IgnoredReplayPath(w.opts.Folder, event.Path)
	}
	return filepath.Ext(name) == ""
}

// Reconcile walks the folder and reports what changed. It is exported so a
// manual rescan and the tests can drive one pass directly.
func (w *Watcher) Reconcile() {
	files, err := fileops.ScanReplayFolder(w.opts.Folder)
	if err != nil {
		w.logf("Could not read the replay folder: %v", err)
		return
	}

	present := make(map[string]struct{}, len(files))
	var change Change
	now := w.now()
	for _, file := range files {
		present[file.Path] = struct{}{}
		w.watchDir(filepath.Dir(file.Path))
		current := stamp{size: file.Size, modTime: file.ModTime}
		if w.corpus.Known(file.Path, file.Size, file.ModTime) {
			delete(w.pending, file.Path)
			delete(w.emitted, file.Path)
			continue
		}
		if previous, ok := w.emitted[file.Path]; ok && previous == current {
			continue
		}
		waiting, ok := w.pending[file.Path]
		if !ok || waiting.stamp != current {
			w.pending[file.Path] = pending{stamp: current, since: now}
			continue
		}
		if now.Sub(waiting.since) < w.opts.Settle {
			continue
		}
		delete(w.pending, file.Path)
		w.emitted[file.Path] = current
		change.Changed = append(change.Changed, file)
	}

	for _, path := range w.corpus.Paths() {
		if _, ok := present[path]; ok {
			continue
		}
		if !w.underFolder(path) {
			continue
		}
		change.Removed = append(change.Removed, path)
	}
	for path := range w.pending {
		if _, ok := present[path]; !ok {
			delete(w.pending, path)
		}
	}
	for path := range w.emitted {
		if _, ok := present[path]; !ok {
			delete(w.emitted, path)
		}
	}

	if !change.empty() && w.onChange != nil {
		w.onChange(change)
	}
}

func (w *Watcher) underFolder(path string) bool {
	rel, err := filepath.Rel(w.opts.Folder, path)
	return err == nil && !strings.HasPrefix(rel, "..")
}

func (w *Watcher) watchDir(dir string) {
	if w.native == nil || w.watched[dir] {
		return
	}
	w.watched[dir] = true
	if err := w.native.Add(dir); err != nil {
		w.logf("Could not watch %s: %v", dir, err)
	}
}

func (w *Watcher) logf(format string, args ...any) {
	if w.opts.Log == nil {
		return
	}
	w.opts.Log(fmt.Sprintf(format, args...))
}
