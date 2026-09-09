package persist

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

const (
	EnrichDirName = "bnet_enrich"
	enrichFile    = "queue.json"

	// enrichRetainTerminal keeps terminal entries around as durable stop
	// conditions: without them a restart would re-enqueue a game the queue
	// already gave up on and re-spend bridge budget on it.
	enrichRetainTerminal = 90 * 24 * time.Hour
)

// Enrichment statuses. need_game_id and waiting are live; the rest are
// terminal and never leave the file until pruned by age.
const (
	EnrichNeedGameID  = "need_game_id"
	EnrichWaiting     = "waiting"
	EnrichDone        = "done"
	EnrichNotBetter   = "not_better"
	EnrichUnavailable = "unavailable"
	EnrichExpired     = "expired"
)

// EnrichEntry is one multiplayer game whose full recording may be recoverable
// from a co-player's uploaded copy (issue #341), keyed by the local file's md5.
type EnrichEntry struct {
	MD5        string    `json:"md5"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ReplayDate time.Time `json:"replay_date"`
	// UserLeftAt is when the user's recording ends (game start + duration):
	// the real game can only have ended after it.
	UserLeftAt    time.Time `json:"user_left_at"`
	Toon          string    `json:"toon"`
	GameID        string    `json:"game_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	NextPollAt    time.Time `json:"next_poll_at"`
	PollCount     int       `json:"poll_count,omitempty"`
	PollErrors    int       `json:"poll_errors,omitempty"`
	ProfileMisses int       `json:"profile_misses,omitempty"`
	LastBestSize  int64     `json:"last_best_size,omitempty"`
	Status        string    `json:"status"`
	Reason        string    `json:"reason,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Terminal reports whether the entry reached a durable stop condition.
func (e EnrichEntry) Terminal() bool {
	switch e.Status {
	case EnrichDone, EnrichNotBetter, EnrichUnavailable, EnrichExpired:
		return true
	}
	return false
}

type enrichFileShape struct {
	Entries map[string]EnrichEntry `json:"entries"`
	// CallsDay and CallsToday implement the worker's self-cap on bridge
	// calls, persisted so restarts don't reset it.
	CallsDay   string `json:"calls_day,omitempty"`
	CallsToday int    `json:"calls_today,omitempty"`
}

// EnrichQueue stores the enrichment queue at <root>/bnet_enrich/queue.json.
// Entries load on first use; one mutex serialises every read and write.
type EnrichQueue struct {
	root   string
	mu     sync.Mutex
	loaded *enrichFileShape
}

func NewEnrichQueue(root string) *EnrichQueue {
	return &EnrichQueue{root: root}
}

func (q *EnrichQueue) path() string {
	return filepath.Join(q.root, EnrichDirName, enrichFile)
}

// Get returns the entry for md5, if any.
func (q *EnrichQueue) Get(md5 string) (EnrichEntry, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return EnrichEntry{}, false, err
	}
	entry, ok := file.Entries[md5]
	return entry, ok, nil
}

// Upsert stores an entry under its md5, stamping UpdatedAt.
func (q *EnrichQueue) Upsert(entry EnrichEntry) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return err
	}
	entry.UpdatedAt = time.Now().UTC()
	file.Entries[entry.MD5] = entry
	return q.writeLocked(file)
}

// Live returns the non-terminal entries, in no particular order.
func (q *EnrichQueue) Live() ([]EnrichEntry, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return nil, err
	}
	out := make([]EnrichEntry, 0, len(file.Entries))
	for _, entry := range file.Entries {
		if !entry.Terminal() {
			out = append(out, entry)
		}
	}
	return out, nil
}

// Calls returns how many bridge calls the worker spent today (UTC).
func (q *EnrichQueue) Calls(now time.Time) (int, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return 0, err
	}
	if file.CallsDay != now.UTC().Format("2006-01-02") {
		return 0, nil
	}
	return file.CallsToday, nil
}

// CountCall records one bridge call against the daily self-cap. The counter
// resets on the first call of each UTC day.
func (q *EnrichQueue) CountCall(now time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return err
	}
	day := now.UTC().Format("2006-01-02")
	if file.CallsDay != day {
		file.CallsDay = day
		file.CallsToday = 0
	}
	file.CallsToday++
	return q.writeLocked(file)
}

// KnownPaths returns the local file paths of every entry, live or terminal,
// so discovery can skip files without hashing them again.
func (q *EnrichQueue) KnownPaths() (map[string]struct{}, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(file.Entries))
	for _, entry := range file.Entries {
		out[entry.Path] = struct{}{}
	}
	return out, nil
}

// Prune drops terminal entries older than the retention window.
func (q *EnrichQueue) Prune(now time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	file, err := q.fileLocked()
	if err != nil {
		return err
	}
	changed := false
	for md5, entry := range file.Entries {
		if entry.Terminal() && now.Sub(entry.UpdatedAt) > enrichRetainTerminal {
			delete(file.Entries, md5)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return q.writeLocked(file)
}

func (q *EnrichQueue) fileLocked() (*enrichFileShape, error) {
	if q.loaded != nil {
		return q.loaded, nil
	}
	file := &enrichFileShape{Entries: map[string]EnrichEntry{}}
	raw, err := iofacade.ReadFile(q.path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return nil, err
	default:
		// A truncated or hand-edited file becomes an empty queue: candidates
		// re-enqueue from the corpus on the next sweep.
		var parsed enrichFileShape
		if err := json.Unmarshal(raw, &parsed); err == nil && parsed.Entries != nil {
			file = &parsed
		}
	}
	q.loaded = file
	return file, nil
}

func (q *EnrichQueue) writeLocked(file *enrichFileShape) error {
	path := q.path()
	if err := iofacade.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(file)
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := iofacade.WriteFile(temp, raw, 0o644); err != nil {
		return err
	}
	return iofacade.Rename(temp, path)
}
