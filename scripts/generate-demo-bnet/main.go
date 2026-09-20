// Command generate-demo-bnet builds the Battle.net data the WASM demo ships.
//
// The demo has no bridge to call, so its Battle.net panels are only as good as
// what is baked into the binary. This writes that bake: the two v2 store files
// the app reads directly, rather than the pre-v2 per-payload tree the demo used
// to carry. That tree was ~6.8 MB of raw upstream payloads, every byte of which
// the browser downloaded so the app could distil it again on each page load,
// and it only worked at all by riding the legacy migration that exists for
// upgrades from old installs.
//
// Two sources, either or both:
//
//	-legacy DIR   the old per-payload tree, distilled on the way through.
//	-from ROOT    a real screpdb app-data directory, filtered by -toons.
//
// Data from -from is real, captured from someone's own bridge. Nothing here
// invents a profile: a demo that shows a made-up rating for a real account is
// worse than one that shows no rating at all.
//
//	go run ./scripts/generate-demo-bnet -legacy cmd/wasmdemo/bnetdata -out cmd/wasmdemo/bnetdata
//	go run ./scripts/generate-demo-bnet -from "$HOME/Library/Application Support/screpdb" -toons kimsabuho
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/persist"
)

type legacyEntry struct {
	Toon        string    `json:"toon"`
	Gateway     int64     `json:"gateway"`
	Found       bool      `json:"found"`
	AuroraID    int64     `json:"aurora_id"`
	BattleTag   string    `json:"battle_tag"`
	CountryCode string    `json:"country_code"`
	FetchedAt   time.Time `json:"fetched_at"`
	Payload     string    `json:"payload"`
}

func main() {
	legacyDir := flag.String("legacy", "", "pre-v2 per-payload tree to distil in")
	fromRoot := flag.String("from", "", "app-data root to copy real profiles out of")
	toons := flag.String("toons", "", "comma-separated toons to take from -from")
	outDir := flag.String("out", "cmd/wasmdemo/bnetdata", "destination for profiles.v2.jsonl and games.v2.jsonl")
	flag.Parse()

	if *legacyDir == "" && *fromRoot == "" {
		log.Fatal("nothing to do: pass -legacy, -from, or both")
	}

	profiles := map[string]persist.BnetProfile{}
	var games []persist.BnetGame

	if *legacyDir != "" {
		p, g, err := readLegacy(*legacyDir)
		if err != nil {
			log.Fatalf("reading %s: %v", *legacyDir, err)
		}
		mergeProfiles(profiles, p)
		games = append(games, g...)
		log.Printf("legacy tree: %d profiles, %d games", len(p), len(g))
	}

	if *fromRoot != "" {
		wanted := splitToons(*toons)
		if len(wanted) == 0 {
			log.Fatal("-from needs -toons: copying a whole personal cache into a public demo is not the intent")
		}
		p, g, err := readLiveStore(*fromRoot, wanted)
		if err != nil {
			log.Fatalf("reading %s: %v", *fromRoot, err)
		}
		mergeProfiles(profiles, p)
		games = append(games, g...)
		log.Printf("live store: %d profiles, %d games for %d toons", len(p), len(g), len(wanted))
	}

	if err := write(*outDir, profiles, games); err != nil {
		log.Fatalf("writing %s: %v", *outDir, err)
	}
	log.Printf("wrote %d profiles and %d games to %s", len(profiles), len(games), *outDir)
}

func readLegacy(dir string) ([]persist.BnetProfile, []persist.BnetGame, error) {
	var profiles []persist.BnetProfile
	var games []persist.BnetGame
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		var entry legacyEntry
		if json.Unmarshal(raw, &entry) != nil || strings.TrimSpace(entry.Toon) == "" {
			return nil
		}
		profile, entryGames := persist.DistillBnetProfile(entry.Toon, entry.Gateway, entry.FetchedAt, []byte(entry.Payload))
		if !profile.Found && entry.Found {
			profile.Found = true
			profile.AuroraID = entry.AuroraID
			profile.BattleTag = entry.BattleTag
			profile.CountryCode = entry.CountryCode
		}
		profiles = append(profiles, profile)
		games = append(games, entryGames...)
		return nil
	})
	return profiles, games, err
}

func readLiveStore(root string, wanted []string) ([]persist.BnetProfile, []persist.BnetGame, error) {
	if err := iofacade.AllowDir(root); err != nil {
		return nil, nil, err
	}
	cache := persist.NewBnetCache(root)
	if err := cache.Load(); err != nil {
		return nil, nil, err
	}
	archive := persist.NewBnetGameArchive(root)
	if err := archive.Load(); err != nil {
		return nil, nil, err
	}
	var profiles []persist.BnetProfile
	var games []persist.BnetGame
	seenAccount := map[int64]bool{}
	for _, p := range cache.ProfilesByToons(wanted) {
		if !p.Found {
			continue
		}
		profiles = append(profiles, p)
		if p.AuroraID != 0 && !seenAccount[p.AuroraID] {
			seenAccount[p.AuroraID] = true
			games = append(games, archive.GamesForAccount(p.AuroraID)...)
		}
	}
	return profiles, games, nil
}

// mergeProfiles keys on (toon, gateway) and keeps the freshest answer, so a
// real profile pulled with -from replaces the distilled legacy copy of it.
func mergeProfiles(into map[string]persist.BnetProfile, from []persist.BnetProfile) {
	for _, p := range from {
		key := fmt.Sprintf("%s|%d", library.PlayerKey(p.Toon), p.Gateway)
		if existing, ok := into[key]; ok && existing.FetchedAt.After(p.FetchedAt) {
			continue
		}
		into[key] = p
	}
}

func splitToons(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func write(dir string, profiles map[string]persist.BnetProfile, games []persist.BnetGame) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	keys := make([]string, 0, len(profiles))
	for k := range profiles {
		keys = append(keys, k)
	}
	// Sorted so a regeneration that changes nothing produces no diff.
	sort.Strings(keys)
	ordered := make([]persist.BnetProfile, 0, len(keys))
	for _, k := range keys {
		ordered = append(ordered, profiles[k])
	}
	if err := writeJSONL(filepath.Join(dir, "profiles.v2.jsonl"), ordered); err != nil {
		return err
	}
	sort.SliceStable(games, func(i, j int) bool {
		if !games[i].CreateTime.Equal(games[j].CreateTime) {
			return games[i].CreateTime.Before(games[j].CreateTime)
		}
		return games[i].GameID < games[j].GameID
	})
	return writeJSONL(filepath.Join(dir, "games.v2.jsonl"), games)
}

func writeJSONL[T any](path string, rows []T) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
