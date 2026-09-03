package library

import (
	"sort"
	"sync"
)

// PlayerRef points at one player slot of one replay.
type PlayerRef struct {
	Replay  *Replay
	Ordinal uint8
}

func (p PlayerRef) Player() *Player { return &p.Replay.Players[p.Ordinal] }
func (p PlayerRef) ID() int64       { return p.Replay.PlayerID(p.Ordinal) }

type Phase string

const (
	PhaseIdle     Phase = "idle"
	PhaseScanning Phase = "scanning"
	PhaseRecent   Phase = "recent"
	PhaseBackfill Phase = "backfill"
	PhaseReady    Phase = "ready"
	PhaseWatching Phase = "watching"
	PhaseFailed   Phase = "failed"
)

// ProgressState is the loader's public counter set, published to
// subscribers and stamped on every snapshot.
type ProgressState struct {
	Generation uint64 `json:"generation"`
	Version    uint64 `json:"version"`
	Folder     string `json:"folder"`
	Phase      Phase  `json:"phase"`
	Total      int    `json:"total"`
	Loaded     int    `json:"loaded"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
}

// Complete reports whether the whole folder has been loaded.
func (p ProgressState) Complete() bool {
	return p.Phase == PhaseReady || p.Phase == PhaseWatching
}

// Snapshot is an immutable corpus state. Replays are ordered by date
// descending then id descending; indexes are rebuilt on every commit.
type Snapshot struct {
	Generation uint64
	Version    uint64
	Replays    []*Replay
	Progress   ProgressState

	byID       map[int64]*Replay
	byChecksum map[[32]byte]*Replay
	byPath     map[string]*Replay
	players    map[string][]PlayerRef

	viewsMu sync.Mutex
	views   map[uint64]*View
}

func newSnapshot(generation, version uint64, records []*Replay, progress ProgressState) *Snapshot {
	replays := make([]*Replay, len(records))
	copy(replays, records)
	sort.Slice(replays, func(i, j int) bool {
		if !replays[i].Date.Equal(replays[j].Date) {
			return replays[i].Date.After(replays[j].Date)
		}
		return replays[i].ID > replays[j].ID
	})

	s := &Snapshot{
		Generation: generation,
		Version:    version,
		Replays:    replays,
		Progress:   progress,
		byID:       make(map[int64]*Replay, len(replays)),
		byChecksum: make(map[[32]byte]*Replay, len(replays)),
		byPath:     make(map[string]*Replay, len(replays)),
		players:    make(map[string][]PlayerRef, len(replays)*2),
		views:      map[uint64]*View{},
	}
	for _, r := range replays {
		s.byID[r.ID] = r
		s.byChecksum[r.Checksum] = r
		for _, f := range r.Paths {
			s.byPath[f.Path] = r
		}
		for i := range r.Players {
			key := r.Players[i].Key
			s.players[key] = append(s.players[key], PlayerRef{Replay: r, Ordinal: uint8(i)})
		}
	}
	return s
}

func (s *Snapshot) Len() int { return len(s.Replays) }

func (s *Snapshot) ByID(id int64) (*Replay, bool) {
	r, ok := s.byID[id]
	return r, ok
}

func (s *Snapshot) ByChecksum(sum [32]byte) (*Replay, bool) {
	r, ok := s.byChecksum[sum]
	return r, ok
}

func (s *Snapshot) ByPath(path string) (*Replay, bool) {
	r, ok := s.byPath[path]
	return r, ok
}

// PlayerGames returns every slot a player key occupies, newest game first.
// The slice is shared with the snapshot and must not be modified.
func (s *Snapshot) PlayerGames(key string) []PlayerRef { return s.players[key] }

// PlayerKeys returns every player key in the corpus, sorted.
func (s *Snapshot) PlayerKeys() []string {
	keys := make([]string, 0, len(s.players))
	for k := range s.players {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *Snapshot) view(f filterState, progress func() ProgressState) *View {
	s.viewsMu.Lock()
	defer s.viewsMu.Unlock()
	if v, ok := s.views[f.version]; ok {
		return v
	}
	v := newView(s, f.config, f.version, progress)
	s.views[f.version] = v
	return v
}
