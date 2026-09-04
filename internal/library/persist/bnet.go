package persist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
)

const BnetProfilesDirName = "bnet_profiles"

// BnetProfile is one cached Battle.net profile lookup. Payload is the raw
// upstream JSON and is only read from disk on demand.
type BnetProfile struct {
	Toon        string    `json:"toon"`
	Gateway     int64     `json:"gateway"`
	Found       bool      `json:"found"`
	AuroraID    int64     `json:"aurora_id"`
	BattleTag   string    `json:"battle_tag"`
	CountryCode string    `json:"country_code"`
	FetchedAt   time.Time `json:"fetched_at"`
	Payload     string    `json:"payload"`
}

type bnetHeader struct {
	Toon        string    `json:"toon"`
	Gateway     int64     `json:"gateway"`
	Found       bool      `json:"found"`
	AuroraID    int64     `json:"aurora_id"`
	BattleTag   string    `json:"battle_tag"`
	CountryCode string    `json:"country_code"`
	FetchedAt   time.Time `json:"fetched_at"`
}

type bnetKey struct {
	toon    string
	gateway int64
}

// BnetCache keeps profile headers in memory and one JSON file per entry at
// <root>/bnet_profiles/<gateway>/<hex(sha256(toon))[:16]>.json. One mutex
// serialises every write.
type BnetCache struct {
	root    string
	mu      sync.Mutex
	entries map[bnetKey]bnetHeader
}

func NewBnetCache(root string) *BnetCache {
	return &BnetCache{root: root, entries: map[bnetKey]bnetHeader{}}
}

func (c *BnetCache) Dir() string { return filepath.Join(c.root, BnetProfilesDirName) }

// BnetProfilePath is where the entry for (toon, gateway) lives under root.
func BnetProfilePath(root, toon string, gateway int64) string {
	sum := sha256.Sum256([]byte(toon))
	return filepath.Join(root, BnetProfilesDirName, strconv.FormatInt(gateway, 10), hex.EncodeToString(sum[:8])+".json")
}

// Load reads every entry's header. Unreadable or malformed files are skipped;
// a missing directory is an empty cache.
func (c *BnetCache) Load() error {
	dir := c.Dir()
	if _, err := iofacade.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("persist: stat %s: %w", dir, err)
	}
	loaded := map[bnetKey]bnetHeader{}
	err := iofacade.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, readErr := iofacade.ReadFile(path)
		if readErr != nil {
			return nil
		}
		var h bnetHeader
		if json.Unmarshal(raw, &h) != nil || h.Toon == "" {
			return nil
		}
		loaded[bnetKey{h.Toon, h.Gateway}] = h
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist: walk %s: %w", dir, err)
	}
	c.mu.Lock()
	c.entries = loaded
	c.mu.Unlock()
	return nil
}

func (c *BnetCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}

// Get returns the cached profile with its payload, or nil when none is cached.
func (c *BnetCache) Get(toon string, gateway int64) (*BnetProfile, error) {
	c.mu.Lock()
	h, ok := c.entries[bnetKey{toon, gateway}]
	c.mu.Unlock()
	if !ok {
		return nil, nil
	}
	return c.readProfile(h)
}

func (c *BnetCache) readProfile(h bnetHeader) (*BnetProfile, error) {
	path := BnetProfilePath(c.root, h.Toon, h.Gateway)
	raw, err := iofacade.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("persist: read %s: %w", path, err)
	}
	var p BnetProfile
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("persist: decode %s: %w", path, err)
	}
	return &p, nil
}

// Upsert writes the entry's file atomically and updates the in-memory header.
func (c *BnetCache) Upsert(p BnetProfile) error {
	if strings.TrimSpace(p.Toon) == "" {
		return errors.New("persist: bnet profile toon is required")
	}
	p.FetchedAt = p.FetchedAt.UTC()
	raw, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("persist: encode bnet profile: %w", err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := writeFileAtomic(BnetProfilePath(c.root, p.Toon, p.Gateway), raw); err != nil {
		return err
	}
	c.entries[bnetKey{p.Toon, p.Gateway}] = bnetHeader{
		Toon: p.Toon, Gateway: p.Gateway, Found: p.Found, AuroraID: p.AuroraID,
		BattleTag: p.BattleTag, CountryCode: p.CountryCode, FetchedAt: p.FetchedAt,
	}
	return nil
}

// CountryCodesByToons maps each player key (lowercased, trimmed toon) to the
// country code of its found profile; the most recently fetched entry wins.
func (c *BnetCache) CountryCodesByToons(toons []string) map[string]string {
	wanted := playerKeySet(toons)
	out := make(map[string]string, len(wanted))
	newest := map[string]time.Time{}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, h := range c.entries {
		key := library.PlayerKey(h.Toon)
		if _, ok := wanted[key]; !ok || !h.Found || h.CountryCode == "" {
			continue
		}
		if seen, ok := newest[key]; ok && !h.FetchedAt.After(seen) {
			continue
		}
		newest[key] = h.FetchedAt
		out[key] = h.CountryCode
	}
	return out
}

// PayloadsByToons returns every found profile whose toon matches one of the
// player keys, payload included. Read-only: it never triggers a fetch.
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
	for key, header := range c.entries {
		if !header.Found || header.AuroraID == 0 {
			continue
		}
		if _, ok := wanted[library.PlayerKey(key.toon)]; !ok {
			continue
		}
		if _, ok := seen[header.AuroraID]; ok {
			continue
		}
		seen[header.AuroraID] = struct{}{}
		out = append(out, header.AuroraID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (c *BnetCache) PayloadsByToons(toons []string) ([]BnetProfile, error) {
	wanted := playerKeySet(toons)
	c.mu.Lock()
	headers := make([]bnetHeader, 0, len(wanted))
	for _, h := range c.entries {
		if _, ok := wanted[library.PlayerKey(h.Toon)]; ok && h.Found {
			headers = append(headers, h)
		}
	}
	c.mu.Unlock()

	out := make([]BnetProfile, 0, len(headers))
	for _, h := range headers {
		p, err := c.readProfile(h)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

// PruneOlderThan deletes entries fetched more than d ago and returns how many.
func (c *BnetCache) PruneOlderThan(d time.Duration) (int, error) {
	cutoff := time.Now().Add(-d)
	c.mu.Lock()
	defer c.mu.Unlock()
	pruned := 0
	for key, h := range c.entries {
		if !h.FetchedAt.Before(cutoff) {
			continue
		}
		path := BnetProfilePath(c.root, h.Toon, h.Gateway)
		if err := iofacade.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return pruned, fmt.Errorf("persist: remove %s: %w", path, err)
		}
		delete(c.entries, key)
		pruned++
	}
	return pruned, nil
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
