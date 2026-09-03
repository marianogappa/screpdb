package library

import (
	"sort"
	"sync"
)

// CorpusStamp tells API consumers which corpus state a response was built from.
type CorpusStamp struct {
	Generation uint64 `json:"generation"`
	Version    uint64 `json:"version"`
	Loaded     int    `json:"loaded"`
	Total      int    `json:"total"`
	Complete   bool   `json:"complete"`
}

// View is a snapshot seen through the global filter. It is immutable, cached
// per (snapshot, filter version), and carries memoised aggregates that live
// exactly as long as the snapshot does.
type View struct {
	snap          *Snapshot
	filter        FilterConfig
	filterVersion uint64
	progress      func() ProgressState
	replays       []*Replay

	keysOnce sync.Once
	keys     []string

	memoMu sync.Mutex
	memos  map[string]*memoEntry
}

type memoEntry struct {
	once sync.Once
	val  any
}

func newView(snap *Snapshot, filter FilterConfig, filterVersion uint64, progress func() ProgressState) *View {
	v := &View{
		snap:          snap,
		filter:        filter,
		filterVersion: filterVersion,
		progress:      progress,
		memos:         map[string]*memoEntry{},
	}
	v.replays = make([]*Replay, 0, len(snap.Replays))
	for _, r := range snap.Replays {
		if filter.Matches(r) {
			v.replays = append(v.replays, r)
		}
	}
	return v
}

func (v *View) Snapshot() *Snapshot  { return v.snap }
func (v *View) Filter() FilterConfig { return v.filter }

// Replays returns the filtered corpus in snapshot order (newest first).
// The slice is shared and must not be modified.
func (v *View) Replays() []*Replay { return v.replays }

func (v *View) Len() int { return len(v.replays) }

func (v *View) Contains(id int64) bool {
	_, ok := v.ByID(id)
	return ok
}

// ByID returns the replay only when it passes the filter.
func (v *View) ByID(id int64) (*Replay, bool) {
	r, ok := v.snap.byID[id]
	if !ok || !v.filter.Matches(r) {
		return nil, false
	}
	return r, true
}

// PlayerGames returns the filtered slots for a player key, newest first.
func (v *View) PlayerGames(key string) []PlayerRef {
	all := v.snap.players[key]
	out := make([]PlayerRef, 0, len(all))
	for _, ref := range all {
		if v.filter.Matches(ref.Replay) {
			out = append(out, ref)
		}
	}
	return out
}

// PlayerKeys returns the sorted keys of players with at least one filtered game.
func (v *View) PlayerKeys() []string {
	v.keysOnce.Do(func() {
		seen := make(map[string]struct{}, len(v.snap.players))
		for _, r := range v.replays {
			for i := range r.Players {
				seen[r.Players[i].Key] = struct{}{}
			}
		}
		v.keys = make([]string, 0, len(seen))
		for k := range seen {
			v.keys = append(v.keys, k)
		}
		sort.Strings(v.keys)
	})
	return v.keys
}

// Corpus stamps the view: Loaded counts committed records regardless of the
// filter; Total and Complete come from the live loader progress.
func (v *View) Corpus() CorpusStamp {
	p := v.progress()
	loaded := v.snap.Len()
	total := p.Total
	if total < loaded {
		total = loaded
	}
	return CorpusStamp{
		Generation: v.snap.Generation,
		Version:    v.snap.Version,
		Loaded:     loaded,
		Total:      total,
		Complete:   p.Complete(),
	}
}

// Memo builds a value once per key for the lifetime of the view. Concurrent
// callers for the same key block until the first build finishes.
func (v *View) Memo(key string, build func() any) any {
	v.memoMu.Lock()
	e, ok := v.memos[key]
	if !ok {
		e = &memoEntry{}
		v.memos[key] = e
	}
	v.memoMu.Unlock()
	e.once.Do(func() { e.val = build() })
	return e.val
}
