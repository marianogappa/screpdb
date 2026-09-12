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
	"github.com/marianogappa/screpdb/internal/library"
)

const (
	BnetDirName          = "bnet"
	bnetProfilesFileName = "profiles.v2.jsonl"
	// bnetProfilesFilePrefix matches every version of the profile store, so a
	// format bump can delete the stale ones. The game archive deliberately has
	// no such sweep: profiles refill on the 24h TTL, the archive cannot.
	bnetProfilesFilePrefix = "profiles."
	// bnetProfileBudgetEntries bounds the store by entry count — a distilled
	// record is small and predictable, so entries are the meaningful unit.
	// 20,000 is far past 90 days of distinct opponents at the bridge's 600
	// fetches/day cap.
	bnetProfileBudgetEntries = 20000
)

// BnetProfile is one distilled Battle.net profile lookup. The upstream payload
// is parsed once at fetch and discarded: everything the app reads is a field
// here. Adding a field later needs a code change and nothing else, because the
// 24h profile TTL refetches every profile anyone actually looks at.
type BnetProfile struct {
	Toon        string         `json:"t"`
	Gateway     int64          `json:"g"`
	Found       bool           `json:"f,omitempty"`
	FetchedAt   time.Time      `json:"w"`
	AuroraID    int64          `json:"a,omitempty"`
	BattleTag   string         `json:"b,omitempty"`
	CountryCode string         `json:"c,omitempty"`
	AvatarID    string         `json:"v,omitempty"`
	Toons       []BnetToon     `json:"o,omitempty"`
	Ladder      []BnetLadder   `json:"m,omitempty"`
	Lifetime    []BnetLifetime `json:"l,omitempty"`
}

// BnetToon is one toons[] row: a name the account plays under on one gateway.
type BnetToon struct {
	Toon          string `json:"t"`
	Gateway       int    `json:"g"`
	GamesLastWeek int    `json:"w,omitempty"`
}

// BnetLadder is one matchmaked_stats row, kept per season.
type BnetLadder struct {
	Season        int    `json:"s"`
	Toon          string `json:"t,omitempty"`
	Rating        int    `json:"r,omitempty"`
	HighestRating int    `json:"h,omitempty"`
	Wins          int    `json:"w,omitempty"`
	Losses        int    `json:"l,omitempty"`
	Disconnects   int    `json:"d,omitempty"`
	Bucket        int    `json:"k,omitempty"`
}

// BnetLifetime is one stats[] row, keyed by (season, gateway, toon). Season 0
// is the non-ladder bucket; 15+ are per-season ladder rows.
type BnetLifetime struct {
	Season  int                       `json:"s"`
	Gateway int                       `json:"g"`
	Toon    string                    `json:"t,omitempty"`
	Race    map[string]BnetRaceTotals `json:"r,omitempty"`
}

// BnetRaceTotals keeps the six real per-race lifetime metrics; the end-screen
// score counters (resources_*, units_*, structures_*, score_*) are noise and
// are dropped at distillation.
type BnetRaceTotals struct {
	Wins        int     `json:"w,omitempty"`
	Losses      int     `json:"l,omitempty"`
	Draws       int     `json:"d,omitempty"`
	Disconnects int     `json:"x,omitempty"`
	APMSum      float64 `json:"a,omitempty"`
	PlayTimeSec int64   `json:"p,omitempty"`
}

// BnetSeasonBuckets is matchmaked_current_season_buckets. Every cached profile
// carries the identical vector: it is the global MMR bracket boundary table,
// not per-player data, so it is kept once here instead of per record.
var BnetSeasonBuckets = [...]int{0, 1163, 1397, 1561, 1742, 2028, 2244, 9999}

type bnetKey struct {
	toon    string
	gateway int64
}

// BnetCache keeps every distilled profile in memory and mirrors them into one
// JSONL file at <root>/bnet/profiles.v2.jsonl, rewritten atomically on every
// change. Reads never touch disk; one mutex serialises everything.
type BnetCache struct {
	root    string
	mu      sync.Mutex
	entries map[bnetKey]BnetProfile
}

func NewBnetCache(root string) *BnetCache {
	return &BnetCache{root: root, entries: map[bnetKey]BnetProfile{}}
}

// BnetProfilesPath is where the profile store lives under root.
func BnetProfilesPath(root string) string {
	return filepath.Join(root, BnetDirName, bnetProfilesFileName)
}

// Load reads the store. Torn or malformed lines are skipped: this is a cache
// that refills itself from the next fetch. Stale-version profile files are
// deleted — a version bump needs no migration here.
func (c *BnetCache) Load() error {
	c.removeStaleVersions()
	raw, err := iofacade.ReadFile(BnetProfilesPath(c.root))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("persist: read %s: %w", BnetProfilesPath(c.root), err)
	}
	loaded := map[bnetKey]BnetProfile{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var p BnetProfile
		if json.Unmarshal([]byte(line), &p) != nil || p.Toon == "" {
			continue
		}
		loaded[bnetKey{p.Toon, p.Gateway}] = p
	}
	c.mu.Lock()
	c.entries = loaded
	c.mu.Unlock()
	return nil
}

func (c *BnetCache) removeStaleVersions() {
	dir := filepath.Join(c.root, BnetDirName)
	entries, err := iofacade.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if name == bnetProfilesFileName || !strings.HasPrefix(name, bnetProfilesFilePrefix) {
			continue
		}
		_ = iofacade.Remove(filepath.Join(dir, name))
	}
}

func (c *BnetCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Get returns the cached profile, or nil when none is cached.
func (c *BnetCache) Get(toon string, gateway int64) *BnetProfile {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.entries[bnetKey{toon, gateway}]
	if !ok {
		return nil
	}
	return &p
}

// Upsert stores the entry, enforces the entry budget and rewrites the file.
func (c *BnetCache) Upsert(p BnetProfile) error {
	return c.UpsertBatch([]BnetProfile{p})
}

// UpsertBatch stores several entries under one file rewrite.
func (c *BnetCache) UpsertBatch(ps []BnetProfile) error {
	if len(ps) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range ps {
		if strings.TrimSpace(p.Toon) == "" {
			return errors.New("persist: bnet profile toon is required")
		}
		p.FetchedAt = p.FetchedAt.UTC()
		c.entries[bnetKey{p.Toon, p.Gateway}] = p
	}
	c.enforceBudgetLocked()
	return c.saveLocked()
}

// enforceBudgetLocked evicts the oldest entries by FetchedAt beyond the entry
// budget. The record size is bounded, so entries are the unit that matters.
func (c *BnetCache) enforceBudgetLocked() {
	over := len(c.entries) - bnetProfileBudgetEntries
	if over <= 0 {
		return
	}
	type aged struct {
		key       bnetKey
		fetchedAt time.Time
	}
	all := make([]aged, 0, len(c.entries))
	for key, p := range c.entries {
		all = append(all, aged{key, p.FetchedAt})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].fetchedAt.Before(all[j].fetchedAt) })
	for _, a := range all[:over] {
		delete(c.entries, a.key)
	}
}

func (c *BnetCache) saveLocked() error {
	keys := make([]bnetKey, 0, len(c.entries))
	for key := range c.entries {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].gateway != keys[j].gateway {
			return keys[i].gateway < keys[j].gateway
		}
		return keys[i].toon < keys[j].toon
	})
	var b strings.Builder
	for _, key := range keys {
		line, err := json.Marshal(c.entries[key])
		if err != nil {
			return fmt.Errorf("persist: encode bnet profile: %w", err)
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return writeFileAtomic(BnetProfilesPath(c.root), []byte(b.String()))
}

// CountryCodesByToons maps each player key (lowercased, trimmed toon) to the
// country code of its found profile; the most recently fetched entry wins.
func (c *BnetCache) CountryCodesByToons(toons []string) map[string]string {
	wanted := playerKeySet(toons)
	out := make(map[string]string, len(wanted))
	newest := map[string]time.Time{}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.entries {
		key := library.PlayerKey(p.Toon)
		if _, ok := wanted[key]; !ok || !p.Found || p.CountryCode == "" {
			continue
		}
		if seen, ok := newest[key]; ok && !p.FetchedAt.After(seen) {
			continue
		}
		newest[key] = p.FetchedAt
		out[key] = p.CountryCode
	}
	return out
}

// FetchedAtByToons maps each player key to the most recent FetchedAt across
// all gateways, considering only found entries.
func (c *BnetCache) FetchedAtByToons(toons []string) map[string]time.Time {
	wanted := playerKeySet(toons)
	out := make(map[string]time.Time, len(wanted))
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.entries {
		key := library.PlayerKey(p.Toon)
		if _, ok := wanted[key]; !ok || !p.Found {
			continue
		}
		if seen, ok := out[key]; ok && !p.FetchedAt.After(seen) {
			continue
		}
		out[key] = p.FetchedAt
	}
	return out
}

// AuroraIDsByToons returns the distinct aurora ids of the found profiles whose
// toon is one of the given normalised player keys.
func (c *BnetCache) AuroraIDsByToons(toons []string) []int64 {
	wanted := playerKeySet(toons)
	if len(wanted) == 0 {
		return []int64{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := map[int64]struct{}{}
	out := []int64{}
	for key, p := range c.entries {
		if !p.Found || p.AuroraID == 0 {
			continue
		}
		if _, ok := wanted[library.PlayerKey(key.toon)]; !ok {
			continue
		}
		if _, ok := seen[p.AuroraID]; ok {
			continue
		}
		seen[p.AuroraID] = struct{}{}
		out = append(out, p.AuroraID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ProfilesByToons returns every found profile whose toon matches one of the
// player keys. Read-only: it never triggers a fetch.
func (c *BnetCache) ProfilesByToons(toons []string) []BnetProfile {
	wanted := playerKeySet(toons)
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]BnetProfile, 0, len(wanted))
	for _, p := range c.entries {
		if _, ok := wanted[library.PlayerKey(p.Toon)]; ok && p.Found {
			out = append(out, p)
		}
	}
	return out
}

// FoundProfilesForToon returns every cached profile entry where Found is true
// for a given toon (case-insensitive), across all gateways.
func (c *BnetCache) FoundProfilesForToon(toon string) []BnetProfile {
	key := library.PlayerKey(toon)
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []BnetProfile
	for k, p := range c.entries {
		if !p.Found || library.PlayerKey(k.toon) != key {
			continue
		}
		out = append(out, p)
	}
	return out
}

// PruneOlderThan deletes entries fetched more than d ago and returns how many.
func (c *BnetCache) PruneOlderThan(d time.Duration) (int, error) {
	cutoff := time.Now().Add(-d)
	c.mu.Lock()
	defer c.mu.Unlock()
	pruned := 0
	for key, p := range c.entries {
		if !p.FetchedAt.Before(cutoff) {
			continue
		}
		delete(c.entries, key)
		pruned++
	}
	if pruned == 0 {
		return 0, nil
	}
	return pruned, c.saveLocked()
}

func playerKeySet(toons []string) map[string]struct{} {
	set := make(map[string]struct{}, len(toons))
	for _, t := range toons {
		if key := library.PlayerKey(t); key != "" {
			set[key] = struct{}{}
		}
	}
	return set
}
