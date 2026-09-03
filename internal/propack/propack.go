// Package propack embeds the precomputed profiles of progamers and other
// popular, high-level players so the dashboard can show them next to the
// user's own players without shipping their replays.
//
// Nothing here is computed at runtime: the pack is generated offline by
// scripts/pro-pack from the aurora-ID-labelled ladder corpus (see
// internal/patterns/markers/MEASUREMENT.md for the labelling procedure) with
// the very same aggregators the dashboard runs on local players, and committed
// as data/pros.json plus the Liquipedia portraits under data/photos. The app
// never queries cwal.gg or Liquipedia; it only reads this embedded snapshot.
package propack

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync"

	"github.com/marianogappa/screpdb/internal/hotkeystream"
)

//go:embed data
var dataFS embed.FS

// KeyPrefix marks a player key as a built-in profile. Pro keys live in their
// own namespace ("pro:bisu") so no local replay name can collide with them.
const KeyPrefix = "pro:"

// Key returns the player key of a pro ID.
func Key(id string) string { return KeyPrefix + strings.ToLower(strings.TrimSpace(id)) }

// IDFromKey returns the pro ID a player key names, or false when the key is a
// regular local player.
func IDFromKey(key string) (string, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if !strings.HasPrefix(key, KeyPrefix) {
		return "", false
	}
	id := strings.TrimPrefix(key, KeyPrefix)
	return id, id != ""
}

// Metric is one skill-proxy aggregate baked from a pro's sampled games.
type Metric struct {
	Value float64 `json:"value"`
	Games int     `json:"games"`
}

// Cadence mirrors the dashboard's unit-production-cadence player aggregate.
type Cadence struct {
	Score      float64 `json:"score"`
	RatePerMin float64 `json:"rate_per_min"`
	CVGap      float64 `json:"cv_gap"`
	Burstiness float64 `json:"burstiness"`
	Idle20     float64 `json:"idle20_ratio"`
	Games      int     `json:"games"`
}

// Toon is one Battle.net account the pro is known to play on. The dashboard
// uses these to fetch their live profile through the SC:R bridge when it is
// connected; nothing else in the app depends on them.
type Toon struct {
	Toon    string `json:"toon"`
	Gateway int    `json:"gateway"`
}

// Pro is one built-in profile.
type Pro struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Liquipedia string  `json:"liquipedia,omitempty"`
	Country    string  `json:"country,omitempty"`
	Confidence string  `json:"confidence,omitempty"`
	AuroraIDs  []int64 `json:"aurora_ids,omitempty"`
	Toons      []Toon  `json:"toons,omitempty"`
	// Photo is the file name under data/photos, empty when Liquipedia had no
	// portrait for the player.
	Photo string `json:"photo,omitempty"`
	// Rank orders the featured strip by popularity (1 = first). It is curated
	// once from recent ASL results and carried over by the generator; zero
	// means unranked, which sorts after every ranked pro.
	Rank         int            `json:"rank,omitempty"`
	PhotoLicense string         `json:"photo_license,omitempty"`
	MainRace     string         `json:"main_race,omitempty"`
	Races        map[string]int `json:"races,omitempty"`
	GamesSampled int            `json:"games_sampled"`

	APM                *Metric                   `json:"apm,omitempty"`
	Cadence            *Cadence                  `json:"cadence,omitempty"`
	ViewportSwitchRate *Metric                   `json:"viewport_switch_rate,omitempty"`
	Hotkeys            []*hotkeystream.Signature `json:"hotkeys,omitempty"`
}

// Key returns the pro's player key.
func (p *Pro) Key() string { return Key(p.ID) }

// Pack is the embedded snapshot.
type Pack struct {
	GeneratedAt      string `json:"generated_at"`
	AlgorithmVersion int    `json:"algorithm_version"`
	Source           string `json:"source,omitempty"`
	Pros             []Pro  `json:"pros"`

	byID    map[string]*Pro
	byLabel map[string]*Pro
}

var (
	loadOnce sync.Once
	loaded   *Pack
	loadErr  error
)

// Load returns the embedded pack, parsed once.
func Load() (*Pack, error) {
	loadOnce.Do(func() {
		loaded, loadErr = parse()
	})
	return loaded, loadErr
}

func parse() (*Pack, error) {
	raw, err := dataFS.ReadFile("data/pros.json")
	if err != nil {
		return nil, fmt.Errorf("propack: reading embedded pack: %w", err)
	}
	var pack Pack
	if err := json.Unmarshal(raw, &pack); err != nil {
		return nil, fmt.Errorf("propack: parsing embedded pack: %w", err)
	}
	pack.byID = make(map[string]*Pro, len(pack.Pros))
	pack.byLabel = make(map[string]*Pro, len(pack.Pros))
	for i := range pack.Pros {
		p := &pack.Pros[i]
		p.ID = strings.ToLower(strings.TrimSpace(p.ID))
		if p.ID == "" {
			return nil, fmt.Errorf("propack: entry %d has no id", i)
		}
		if _, dup := pack.byID[p.ID]; dup {
			return nil, fmt.Errorf("propack: duplicate id %q", p.ID)
		}
		pack.byID[p.ID] = p
		pack.byLabel[strings.ToLower(strings.TrimSpace(p.Label))] = p
	}
	return &pack, nil
}

// ByID returns the pro with this ID, or nil.
func (p *Pack) ByID(id string) *Pro {
	if p == nil {
		return nil
	}
	return p.byID[strings.ToLower(strings.TrimSpace(id))]
}

// ByLabel returns the pro with this display label (case-insensitive), or nil.
// Labels are what the scfingerprint dataset reports on a fingerprint match.
func (p *Pack) ByLabel(label string) *Pro {
	if p == nil {
		return nil
	}
	return p.byLabel[strings.ToLower(strings.TrimSpace(label))]
}

// Photo returns the embedded portrait bytes and their MIME type.
func Photo(pro *Pro) ([]byte, string, bool) {
	if pro == nil || pro.Photo == "" || strings.ContainsAny(pro.Photo, `/\`) {
		return nil, "", false
	}
	data, err := fs.ReadFile(dataFS, path.Join("data/photos", pro.Photo))
	if err != nil || len(data) == 0 {
		return nil, "", false
	}
	mime := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(pro.Photo), ".png") {
		mime = "image/png"
	}
	return data, mime, true
}
