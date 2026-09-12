package persist

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
)

// The game archive is append-only JSONL at <root>/bnet/games.v2.jsonl.
// Battle.net only returns an account's last ~25 games, so unlike the profile
// store this history CANNOT be refetched: there is no age-based pruning, no
// stale-version sweep, and a format bump must ship a converter.
const (
	bnetGamesFileName = "games.v2.jsonl"
	// bnetGamesCompactMinLines keeps compaction from churning a small file;
	// below this the duplicate lines cost nothing worth an extra rewrite.
	bnetGamesCompactMinLines = 1000
	// bnetGamesCompactDupRatio triggers compaction once re-observed games
	// exceed this share of the log's lines.
	bnetGamesCompactDupRatio = 0.2
)

// BnetGame is one game, merged from a profile payload's game_results[] and
// replays[] — the two arrays are complementary, joined on
// replays[].link == game_results[].game_id.
type BnetGame struct {
	GameID     string    `json:"i"`
	CreateTime time.Time `json:"c"`
	Gateway    int       `json:"g,omitempty"`
	// Ladder records that match_guid was present: matchmade games get no
	// lobby replay entry, so the guid itself carries nothing else worth 36
	// characters per game.
	Ladder    bool             `json:"q,omitempty"`
	MapName   string           `json:"m,omitempty"`
	TileSet   int              `json:"e,omitempty"`
	MapWidth  int              `json:"w,omitempty"`
	MapHeight int              `json:"h,omitempty"`
	GameName  string           `json:"n,omitempty"`
	Host      string           `json:"o,omitempty"`
	Type      int              `json:"t,omitempty"`
	SubType   int              `json:"s,omitempty"`
	Players   []BnetGamePlayer `json:"p,omitempty"`
	// Accounts lists the aurora ids whose profile fetches reported this game,
	// so per-account reads (play habits, recent games) stay possible after
	// the per-account files are gone.
	Accounts []int64 `json:"ac,omitempty"`

	// Own replay. Present only for the fetching account's own games, and only
	// while GCS still serves it (17-46 days). Cleared by retention.
	MD5 string `json:"d,omitempty"`
	URL string `json:"u,omitempty"`
}

// BnetGamePlayer is one raced slot of a game.
type BnetGamePlayer struct {
	Toon     string `json:"t"`
	Race     string `json:"r,omitempty"`
	Team     int    `json:"m,omitempty"`
	Computer bool   `json:"ai,omitempty"`
	Left     bool   `json:"l,omitempty"`
	// Result is 0 unknown, 1 win, 2 loss, 3 draw, 4 disconnect. Per the
	// provenance rule it is only ever set from the player's own profile.
	Result  int `json:"o,omitempty"`
	APM     int `json:"a,omitempty"`
	Seconds int `json:"x,omitempty"`
}

const (
	BnetResultUnknown    = 0
	BnetResultWin        = 1
	BnetResultLoss       = 2
	BnetResultDraw       = 3
	BnetResultDisconnect = 4
)

// BnetGameArchive holds every game in memory, keyed by game id, and mirrors
// changes into an append-only log. Records are immutable in spirit: merges
// only fill unknown fields, so the archive monotonically improves as more
// profiles are seen.
type BnetGameArchive struct {
	root  string
	mu    sync.Mutex
	games map[string]BnetGame
	// fileLines counts the log's lines, so the duplicate share (fileLines vs
	// len(games)) can trigger compaction without re-reading the file.
	fileLines int
}

func NewBnetGameArchive(root string) *BnetGameArchive {
	return &BnetGameArchive{root: root, games: map[string]BnetGame{}}
}

// BnetGamesPath is where the game archive lives under root.
func BnetGamesPath(root string) string {
	return filepath.Join(root, BnetDirName, bnetGamesFileName)
}

// Load reads the log; for a re-observed game id the later line wins, matching
// how Upsert appends the fully merged record. Torn or malformed lines are
// skipped, never fatal.
func (a *BnetGameArchive) Load() error {
	raw, err := iofacade.ReadFile(BnetGamesPath(a.root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("persist: read %s: %w", BnetGamesPath(a.root), err)
	}
	loaded := map[string]BnetGame{}
	lines := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines++
		var g BnetGame
		if json.Unmarshal([]byte(line), &g) != nil || g.GameID == "" {
			continue
		}
		loaded[g.GameID] = g
	}
	a.mu.Lock()
	a.games = loaded
	a.fileLines = lines
	a.mu.Unlock()
	return nil
}

func (a *BnetGameArchive) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.games)
}

// Upsert merges the games into the archive and appends the changed records to
// the log. Unchanged re-observations cost nothing.
func (a *BnetGameArchive) Upsert(games []BnetGame) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var b strings.Builder
	appended := 0
	for _, g := range games {
		if g.GameID == "" {
			continue
		}
		merged, changed := mergeBnetGame(a.games[g.GameID], g)
		if !changed {
			continue
		}
		line, err := json.Marshal(merged)
		if err != nil {
			return fmt.Errorf("persist: encode bnet game: %w", err)
		}
		a.games[g.GameID] = merged
		b.Write(line)
		b.WriteByte('\n')
		appended++
	}
	if appended == 0 {
		return nil
	}
	if err := iofacade.MkdirAll(filepath.Dir(BnetGamesPath(a.root)), 0o755); err != nil {
		return err
	}
	if err := iofacade.AppendFile(BnetGamesPath(a.root), []byte(b.String()), 0o644); err != nil {
		return err
	}
	a.fileLines += appended
	return a.compactIfNeededLocked()
}

// mergeBnetGame fills existing's unknown fields from incoming: a non-zero
// value beats a zero one, an existing non-zero value is never overwritten.
// Players merge by toon under the same rule, so a result learned from its own
// player's profile is never clobbered by another profile's unreliable row.
func mergeBnetGame(existing BnetGame, incoming BnetGame) (BnetGame, bool) {
	if existing.GameID == "" {
		return incoming, true
	}
	changed := false
	fillStr := func(dst *string, src string) {
		if *dst == "" && src != "" {
			*dst = src
			changed = true
		}
	}
	fillInt := func(dst *int, src int) {
		if *dst == 0 && src != 0 {
			*dst = src
			changed = true
		}
	}
	if existing.CreateTime.IsZero() && !incoming.CreateTime.IsZero() {
		existing.CreateTime = incoming.CreateTime
		changed = true
	}
	if !existing.Ladder && incoming.Ladder {
		existing.Ladder = true
		changed = true
	}
	fillInt(&existing.Gateway, incoming.Gateway)
	fillStr(&existing.MapName, incoming.MapName)
	fillInt(&existing.TileSet, incoming.TileSet)
	fillInt(&existing.MapWidth, incoming.MapWidth)
	fillInt(&existing.MapHeight, incoming.MapHeight)
	fillStr(&existing.GameName, incoming.GameName)
	fillStr(&existing.Host, incoming.Host)
	fillInt(&existing.Type, incoming.Type)
	fillInt(&existing.SubType, incoming.SubType)
	fillStr(&existing.MD5, incoming.MD5)
	fillStr(&existing.URL, incoming.URL)

	byToon := map[string]int{}
	for i, p := range existing.Players {
		byToon[strings.ToLower(strings.TrimSpace(p.Toon))] = i
	}
	for _, p := range incoming.Players {
		key := strings.ToLower(strings.TrimSpace(p.Toon))
		if key == "" {
			continue
		}
		i, ok := byToon[key]
		if !ok {
			byToon[key] = len(existing.Players)
			existing.Players = append(existing.Players, p)
			changed = true
			continue
		}
		dst := &existing.Players[i]
		fillStr(&dst.Race, p.Race)
		fillInt(&dst.Team, p.Team)
		fillInt(&dst.Result, p.Result)
		fillInt(&dst.APM, p.APM)
		fillInt(&dst.Seconds, p.Seconds)
		if !dst.Computer && p.Computer {
			dst.Computer = true
			changed = true
		}
		if !dst.Left && p.Left {
			dst.Left = true
			changed = true
		}
	}
	for _, id := range incoming.Accounts {
		found := false
		for _, have := range existing.Accounts {
			if have == id {
				found = true
				break
			}
		}
		if !found {
			existing.Accounts = append(existing.Accounts, id)
			changed = true
		}
	}
	return existing, changed
}

// TimesSince returns when an account played, newest first, back to since.
func (a *BnetGameArchive) TimesSince(auroraID int64, since time.Time) []time.Time {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []time.Time
	for _, g := range a.games {
		if !g.reportedBy(auroraID) || g.CreateTime.Before(since) {
			continue
		}
		out = append(out, g.CreateTime.UTC())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].After(out[j]) })
	return out
}

// GamesForAccount returns every game an account's profile fetches reported,
// newest first.
func (a *BnetGameArchive) GamesForAccount(auroraID int64) []BnetGame {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []BnetGame
	for _, g := range a.games {
		if g.reportedBy(auroraID) {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreateTime.After(out[j].CreateTime) })
	return out
}

func (g *BnetGame) reportedBy(auroraID int64) bool {
	if auroraID == 0 {
		return false
	}
	for _, id := range g.Accounts {
		if id == auroraID {
			return true
		}
	}
	return false
}

// ClearReplayRefsOlderThan drops the own-replay MD5/URL of games older than
// maxAge: GCS has already stopped serving them, so the bytes buy nothing.
func (a *BnetGameArchive) ClearReplayRefsOlderThan(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	a.mu.Lock()
	defer a.mu.Unlock()
	cleared := false
	for id, g := range a.games {
		if g.MD5 == "" && g.URL == "" {
			continue
		}
		if g.CreateTime.IsZero() || !g.CreateTime.Before(cutoff) {
			continue
		}
		g.MD5, g.URL = "", ""
		a.games[id] = g
		cleared = true
	}
	if !cleared {
		return nil
	}
	return a.compactLocked()
}

// CompactIfNeeded rewrites the log when re-observed games exceed the
// duplicate-share threshold. Last-write-wins is preserved because memory
// already holds the merged records.
func (a *BnetGameArchive) CompactIfNeeded() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.compactIfNeededLocked()
}

func (a *BnetGameArchive) compactIfNeededLocked() error {
	if a.fileLines < bnetGamesCompactMinLines {
		return nil
	}
	if float64(a.fileLines-len(a.games)) <= float64(a.fileLines)*bnetGamesCompactDupRatio {
		return nil
	}
	return a.compactLocked()
}

func (a *BnetGameArchive) compactLocked() error {
	ids := make([]string, 0, len(a.games))
	for id := range a.games {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var b strings.Builder
	for _, id := range ids {
		line, err := json.Marshal(a.games[id])
		if err != nil {
			return fmt.Errorf("persist: encode bnet game: %w", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	if err := writeFileAtomic(BnetGamesPath(a.root), []byte(b.String())); err != nil {
		return err
	}
	a.fileLines = len(a.games)
	return nil
}
