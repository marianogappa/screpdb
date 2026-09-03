package library

import (
	"sync"
	"sync/atomic"
	"time"
)

// Options tune the committer. Zero values take the defaults below.
type Options struct {
	// CoalesceRecords commits once this many added records are pending.
	CoalesceRecords int
	// CoalesceDelay commits pending mutations after this long without a flush.
	CoalesceDelay time.Duration
	// EventBuffer is the per-subscriber channel depth; slow subscribers drop events.
	EventBuffer int
	// MutationBuffer bounds in-flight mutations before Add blocks.
	MutationBuffer int
}

func (o Options) withDefaults() Options {
	if o.CoalesceRecords <= 0 {
		o.CoalesceRecords = 100
	}
	if o.CoalesceDelay <= 0 {
		o.CoalesceDelay = 250 * time.Millisecond
	}
	if o.EventBuffer <= 0 {
		o.EventBuffer = 64
	}
	if o.MutationBuffer <= 0 {
		o.MutationBuffer = 256
	}
	return o
}

type EventKind uint8

const (
	EventProgress EventKind = iota + 1
	EventCorpus
)

// Event is what subscribers receive: a progress update or a committed corpus
// change listing the replay ids that appeared and disappeared.
type Event struct {
	Kind       EventKind
	Progress   ProgressState
	Generation uint64
	Version    uint64
	Added      []int64
	Removed    []int64
}

type filterState struct {
	config  FilterConfig
	version uint64
}

// Library owns the corpus. All mutations go through one committer goroutine
// that builds successor snapshots; readers never block.
type Library struct {
	opts      Options
	snap      atomic.Pointer[Snapshot]
	progress  atomic.Pointer[ProgressState]
	filter    atomic.Pointer[filterState]
	filterSeq atomic.Uint64

	mutations chan mutation
	stop      chan struct{}
	done      chan struct{}
	closeOnce sync.Once

	subMu   sync.Mutex
	subs    map[uint64]chan Event
	nextSub uint64
	closed  bool
}

func New(opts Options) *Library {
	opts = opts.withDefaults()
	l := &Library{
		opts:      opts,
		mutations: make(chan mutation, opts.MutationBuffer),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		subs:      map[uint64]chan Event{},
	}
	initial := ProgressState{Phase: PhaseIdle}
	l.progress.Store(&initial)
	l.snap.Store(newSnapshot(0, 0, nil, initial))
	l.filter.Store(&filterState{config: DefaultFilterConfig(), version: 0})
	go l.run()
	return l
}

// Snapshot returns the current unfiltered corpus.
func (l *Library) Snapshot() *Snapshot { return l.snap.Load() }

// Progress returns the latest loader progress.
func (l *Library) Progress() ProgressState { return *l.progress.Load() }

// View returns the current snapshot seen through the current filter.
func (l *Library) View() *View {
	return l.Snapshot().view(*l.filter.Load(), l.Progress)
}

// SetFilter normalises and installs a new global filter. Views built for
// the previous filter stay valid for callers already holding them.
func (l *Library) SetFilter(config FilterConfig) error {
	normalized, err := config.Normalize()
	if err != nil {
		return err
	}
	if l.filter.Load().config.Equal(normalized) {
		return nil
	}
	l.filter.Store(&filterState{config: normalized, version: l.filterSeq.Add(1)})
	return nil
}

func (l *Library) Filter() FilterConfig { return l.filter.Load().config }

// Add hands records to the library. Records belong to the library from here
// on; their ID may be rewritten to resolve a checksum collision. Records for
// a generation that is neither live nor staged are discarded.
func (l *Library) Add(generation uint64, replays ...*Replay) {
	if len(replays) == 0 {
		return
	}
	l.send(mutation{kind: opAdd, gen: generation, replays: replays})
}

// Remove drops a file path; the record disappears when no path remains.
func (l *Library) Remove(generation uint64, path string) {
	l.send(mutation{kind: opRemove, gen: generation, path: path})
}

// Alias attaches a file to the record already holding checksum, without
// re-parsing. A checksum that is not in the corpus is ignored.
func (l *Library) Alias(generation uint64, file FileRef, checksum [32]byte) {
	l.send(mutation{kind: opAlias, gen: generation, file: file, checksum: checksum})
}

// Reset immediately replaces the corpus with an empty one for generation.
func (l *Library) Reset(generation uint64) {
	l.send(mutation{kind: opReset, gen: generation})
}

// Stage opens a hidden working set for generation so a folder change can be
// loaded while the previous corpus keeps serving; Promote swaps it in.
func (l *Library) Stage(generation uint64) {
	l.send(mutation{kind: opStage, gen: generation})
}

// Promote makes the staged generation live. A no-op when nothing matching is staged.
func (l *Library) Promote(generation uint64) {
	l.send(mutation{kind: opPromote, gen: generation})
}

// Flush commits every pending mutation and returns once the snapshot is visible.
func (l *Library) Flush() {
	done := make(chan struct{})
	if !l.send(mutation{kind: opFlush, done: done}) {
		return
	}
	select {
	case <-done:
	case <-l.done:
	}
}

// SetProgress publishes loader progress to subscribers and stamps future snapshots.
func (l *Library) SetProgress(p ProgressState) {
	l.progress.Store(&p)
	l.publish(Event{Kind: EventProgress, Progress: p, Generation: p.Generation, Version: p.Version})
}

// Subscribe returns the current progress, a channel of future events and a
// cancel function. Events are dropped rather than blocking the committer when
// the subscriber falls behind.
func (l *Library) Subscribe() (ProgressState, <-chan Event, func()) {
	ch := make(chan Event, l.opts.EventBuffer)
	l.subMu.Lock()
	if l.closed {
		l.subMu.Unlock()
		close(ch)
		return l.Progress(), ch, func() {}
	}
	id := l.nextSub
	l.nextSub++
	l.subs[id] = ch
	l.subMu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			l.subMu.Lock()
			defer l.subMu.Unlock()
			if _, ok := l.subs[id]; ok {
				delete(l.subs, id)
				close(ch)
			}
		})
	}
	return l.Progress(), ch, cancel
}

// Close stops the committer and closes every subscriber channel.
func (l *Library) Close() {
	l.closeOnce.Do(func() {
		close(l.stop)
		<-l.done
		l.subMu.Lock()
		l.closed = true
		for id, ch := range l.subs {
			delete(l.subs, id)
			close(ch)
		}
		l.subMu.Unlock()
	})
}

func (l *Library) publish(ev Event) {
	l.subMu.Lock()
	defer l.subMu.Unlock()
	for _, ch := range l.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (l *Library) send(m mutation) bool {
	select {
	case l.mutations <- m:
		return true
	case <-l.done:
		return false
	}
}

type opKind uint8

const (
	opAdd opKind = iota + 1
	opRemove
	opAlias
	opReset
	opStage
	opPromote
	opFlush
)

type mutation struct {
	kind     opKind
	gen      uint64
	replays  []*Replay
	path     string
	file     FileRef
	checksum [32]byte
	done     chan struct{}
}

func (l *Library) run() {
	defer close(l.done)

	live := newWorkingSet(0)
	var staging *workingSet
	var version uint64
	pending := 0
	var timer *time.Timer
	var timerC <-chan time.Time

	disarm := func() {
		if timer != nil {
			timer.Stop()
			timer = nil
			timerC = nil
		}
	}
	commit := func() {
		disarm()
		pending = 0
		if !live.dirty {
			return
		}
		version++
		snap := live.build(version, l.Progress())
		l.snap.Store(snap)
		ev := Event{Kind: EventCorpus, Generation: snap.Generation, Version: version}
		ev.Added, ev.Removed = live.drainChanges()
		l.publish(ev)
	}

	for {
		select {
		case m := <-l.mutations:
			switch m.kind {
			case opFlush:
				commit()
				close(m.done)
			case opReset:
				live = newWorkingSet(m.gen)
				live.dirty = true
				staging = nil
				commit()
			case opStage:
				staging = newWorkingSet(m.gen)
			case opPromote:
				if staging != nil && staging.gen == m.gen {
					live = staging
					live.dirty = true
					staging = nil
					commit()
				}
			default:
				target := live
				if m.gen != live.gen {
					if staging == nil || staging.gen != m.gen {
						continue
					}
					target = staging
				}
				target.apply(m)
				if target != live {
					continue
				}
				pending += len(m.replays)
				if m.kind != opAdd {
					pending++
				}
				if pending >= l.opts.CoalesceRecords {
					commit()
				} else if timer == nil {
					timer = time.NewTimer(l.opts.CoalesceDelay)
					timerC = timer.C
				}
			}
		case <-timerC:
			commit()
		case <-l.stop:
			commit()
			return
		}
	}
}

// workingSet is the committer's private, mutable copy of one generation.
type workingSet struct {
	gen        uint64
	byChecksum map[[32]byte]*Replay
	byID       map[int64]*Replay
	byPath     map[string]*Replay
	added      map[int64]struct{}
	removed    map[int64]struct{}
	dirty      bool
}

func newWorkingSet(gen uint64) *workingSet {
	return &workingSet{
		gen:        gen,
		byChecksum: map[[32]byte]*Replay{},
		byID:       map[int64]*Replay{},
		byPath:     map[string]*Replay{},
		added:      map[int64]struct{}{},
		removed:    map[int64]struct{}{},
	}
}

func (w *workingSet) apply(m mutation) {
	switch m.kind {
	case opAdd:
		for _, r := range m.replays {
			w.add(r)
		}
	case opRemove:
		w.detachPath(m.path)
	case opAlias:
		if existing, ok := w.byChecksum[m.checksum]; ok {
			w.attachPaths(existing, []FileRef{m.file})
		}
	}
}

func (w *workingSet) add(r *Replay) {
	if r == nil || len(r.Paths) == 0 {
		return
	}
	if existing, ok := w.byChecksum[r.Checksum]; ok {
		w.attachPaths(existing, r.Paths)
		return
	}
	id := r.ID
	if id <= 0 || id > MaxReplayID {
		id = ReplayIDFromChecksum(r.Checksum)
	}
	for {
		other, taken := w.byID[id]
		if !taken || other.Checksum == r.Checksum {
			break
		}
		id = NextReplayID(id)
	}
	r.ID = id
	for _, f := range r.Paths {
		w.detachPath(f.Path)
	}
	SortPaths(r.Paths)
	setAutosaveFlag(r)
	w.index(r)
	w.added[id] = struct{}{}
	delete(w.removed, id)
	w.dirty = true
}

func (w *workingSet) attachPaths(existing *Replay, files []FileRef) {
	merged := make([]FileRef, 0, len(existing.Paths)+len(files))
	merged = append(merged, existing.Paths...)
	changed := false
	for _, f := range files {
		if owner, ok := w.byPath[f.Path]; ok && owner != existing {
			w.detachPath(f.Path)
		}
		replaced := false
		for i := range merged {
			if merged[i].Path == f.Path {
				if merged[i] != f {
					merged[i] = f
					changed = true
				}
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, f)
			changed = true
		}
	}
	if !changed {
		return
	}
	SortPaths(merged)
	updated := *existing
	updated.Paths = merged
	setAutosaveFlag(&updated)
	w.unindex(existing)
	w.index(&updated)
	w.dirty = true
}

func (w *workingSet) detachPath(path string) {
	owner, ok := w.byPath[path]
	if !ok {
		return
	}
	w.unindex(owner)
	if len(owner.Paths) <= 1 {
		if _, addedNow := w.added[owner.ID]; addedNow {
			delete(w.added, owner.ID)
		} else {
			w.removed[owner.ID] = struct{}{}
		}
		w.dirty = true
		return
	}
	updated := *owner
	updated.Paths = make([]FileRef, 0, len(owner.Paths)-1)
	for _, f := range owner.Paths {
		if f.Path != path {
			updated.Paths = append(updated.Paths, f)
		}
	}
	setAutosaveFlag(&updated)
	w.index(&updated)
	w.dirty = true
}

func (w *workingSet) index(r *Replay) {
	w.byChecksum[r.Checksum] = r
	w.byID[r.ID] = r
	for _, f := range r.Paths {
		w.byPath[f.Path] = r
	}
}

func (w *workingSet) unindex(r *Replay) {
	delete(w.byChecksum, r.Checksum)
	delete(w.byID, r.ID)
	for _, f := range r.Paths {
		if w.byPath[f.Path] == r {
			delete(w.byPath, f.Path)
		}
	}
}

func (w *workingSet) build(version uint64, progress ProgressState) *Snapshot {
	records := make([]*Replay, 0, len(w.byChecksum))
	for _, r := range w.byChecksum {
		records = append(records, r)
	}
	w.dirty = false
	return newSnapshot(w.gen, version, records, progress)
}

func (w *workingSet) drainChanges() (added, removed []int64) {
	for id := range w.added {
		added = append(added, id)
	}
	for id := range w.removed {
		removed = append(removed, id)
	}
	w.added = map[int64]struct{}{}
	w.removed = map[int64]struct{}{}
	return added, removed
}

func setAutosaveFlag(r *Replay) {
	r.Flags &^= FlagIsAutosave
	for _, f := range r.Paths {
		if IsAutosavePath(f.Path) {
			r.Flags |= FlagIsAutosave
			return
		}
	}
}
