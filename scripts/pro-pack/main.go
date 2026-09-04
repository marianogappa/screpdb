// pro-pack generates internal/propack/data (the embedded built-in progamer
// profiles) from the aurora-ID-labelled ladder corpus. It is the offline
// "update process": run it, review the diff, commit, release. The app itself
// never talks to cwal.gg or Liquipedia.
//
// Inputs:
//
//   - the screpharvest harvest (replays.jsonl + replays/*.rep),
//   - the scfingerprint corpus dir (pros_merged.json, pro_exclusions.json),
//   - the scfingerprint dataset dir (players/*.json + liquipedia.json), which
//     is the roster: a player is built in iff scfingerprint knows them.
//
// Steps:
//
//  1. label the harvest by aurora ID and stage every labelled match's replay
//     flat into a work folder (scripts/procorpus),
//  2. read that folder into a replay library and join the labelled sides to
//     its players,
//  3. rename every resolved player to its pack key ("pro:<id>") so the
//     dashboard's own per-player aggregators see one player per pro regardless
//     of which account they played on, then run those aggregators (APM,
//     cadence, viewport switch rate, hotkey signatures),
//  4. optionally fetch country + portrait from Liquipedia (MediaWiki API,
//     gzip, identifying User-Agent, one request every 2s per their terms),
//  5. write pros.json + photos/.
//
// Usage:
//
//	go run ./scripts/pro-pack \
//	  -harvest ~/Code/go/src/github.com/marianogappa/screpharvest/harvest \
//	  -corpus  ~/Code/go/src/github.com/marianogappa/scfingerprint/corpus \
//	  -dataset $(go list -m -f '{{.Dir}}' github.com/marianogappa/scfingerprint)/internal/dataset
//
// Nothing is written outside -out and -staged, and no database is involved.
package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/dashboard"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/propack"
	"github.com/marianogappa/screpdb/scripts/procorpus"
)

const userAgent = "screpdb-pro-pack/1.0 (https://github.com/marianogappa/screpdb)"

type datasetIdentity struct {
	ID         string `json:"id"`
	Confidence string `json:"confidence"`
	Aliases    []struct {
		Name    string `json:"name"`
		Primary bool   `json:"primary"`
	} `json:"aliases"`
}

type roster struct {
	id         string
	label      string
	confidence string
	liquipedia string
	auroraIDs  []int64
}

func main() {
	harvestDir := flag.String("harvest", "", "screpharvest harvest dir")
	corpusDir := flag.String("corpus", "", "scfingerprint corpus dir (pros_merged.json, pro_exclusions.json)")
	datasetDir := flag.String("dataset", "", "scfingerprint internal/dataset dir (players/*.json, liquipedia.json)")
	stagedDir := flag.String("staged", filepath.Join(os.TempDir(), "pro-pack", "staged"), "work folder the labelled replays are staged into")
	outDir := flag.String("out", "internal/propack/data", "output dir")
	photos := flag.Bool("photos", true, "fetch country + portrait from Liquipedia")
	minGames := flag.Int("min-games", 10, "drop pros with fewer sampled games")
	minDuration := flag.Int("min-duration", 240, "minimum game duration in seconds")
	flag.Parse()
	if *harvestDir == "" || *corpusDir == "" || *datasetDir == "" {
		flag.Usage()
		os.Exit(2)
	}

	registry, err := procorpus.LoadRegistry(*corpusDir)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}
	rosterByName, err := loadRoster(*datasetDir, registry)
	if err != nil {
		log.Fatalf("roster: %v", err)
	}
	log.Printf("roster: %d built-in identities", len(rosterByName))

	sides, err := procorpus.LabelCorpus(*harvestDir, registry, *minDuration)
	if err != nil {
		log.Fatalf("label: %v", err)
	}
	// A previous run may already have renamed the rows to their pack keys;
	// letting the join recognise those keeps the step idempotent.
	for i := range sides {
		if r, ok := rosterByName[sides[i].ProName]; ok {
			sides[i].AltNames = []string{propack.Key(r.id)}
		}
	}
	byMatch := procorpus.GroupByMatch(sides)

	if err := os.MkdirAll(*stagedDir, 0o755); err != nil {
		log.Fatalf("staged dir: %v", err)
	}
	staged, missing := procorpus.Stage(*harvestDir, *stagedDir, byMatch)
	log.Printf("staged %d replays in %s (%d missing from the harvest)", staged, *stagedDir, missing)

	ctx := context.Background()
	start := time.Now()
	corpus, err := dashboard.LoadProCorpus(ctx, *stagedDir, nil)
	if err != nil {
		log.Fatalf("read staged replays: %v", err)
	}
	defer corpus.Close()
	log.Printf("analysed %d replays in %s", corpus.Len(), time.Since(start).Round(time.Second))

	joined, tallies := procorpus.Join(replays1v1(corpus), byMatch)
	log.Printf("joined %d/%d player-games (%s)", len(joined), len(sides), tallies)

	renamed := corpus.RenamePlayers(packKeyByPlayerID(joined, rosterByName))
	log.Printf("renamed %d players to pack keys", renamed)

	keys := make([]string, 0, len(rosterByName))
	for _, r := range rosterByName {
		keys = append(keys, propack.Key(r.id))
	}
	metrics, err := corpus.Metrics(ctx, keys)
	if err != nil {
		log.Fatalf("metrics: %v", err)
	}
	log.Printf("computed metrics for %d pros", len(metrics))

	toonsByName := recentToons(sides)

	pack := propack.Pack{
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		AlgorithmVersion: core.AlgorithmVersion,
		Source:           "public ladder replays labelled by aurora ID (scfingerprint corpus); photos and countries from Liquipedia",
	}
	photosDir := filepath.Join(*outDir, "photos")
	if err := os.MkdirAll(photosDir, 0o755); err != nil {
		log.Fatalf("mkdir: %v", err)
	}
	var lp *liquipediaClient
	if *photos {
		lp = newLiquipediaClient()
	}
	names := make([]string, 0, len(rosterByName))
	for name := range rosterByName {
		names = append(names, name)
	}
	sort.Strings(names)
	referencedPhotos := map[string]bool{}
	for _, name := range names {
		r := rosterByName[name]
		m, ok := metrics[propack.Key(r.id)]
		if !ok || m.GamesSampled < *minGames {
			log.Printf("skip %s: %d sampled games", r.id, m.GamesSampled)
			continue
		}
		pro := propack.Pro{
			ID:                 r.id,
			Label:              r.label,
			Liquipedia:         r.liquipedia,
			Confidence:         r.confidence,
			AuroraIDs:          r.auroraIDs,
			Toons:              toonsByName[name],
			Races:              m.Races,
			MainRace:           mainRace(m.Races),
			GamesSampled:       m.GamesSampled,
			APM:                m.APM,
			Cadence:            m.Cadence,
			ViewportSwitchRate: m.ViewportSwitchRate,
			Hotkeys:            m.Hotkeys,
		}
		if lp != nil && r.liquipedia != "" {
			info, err := lp.playerInfo(r.liquipedia)
			if err != nil {
				log.Printf("liquipedia %s: %v", r.id, err)
			} else {
				pro.Country = info.country
				if info.imageURL != "" {
					fileName, err := lp.downloadPhoto(info.imageURL, photosDir, r.id)
					if err != nil {
						log.Printf("photo %s: %v", r.id, err)
					} else {
						pro.Photo = fileName
						pro.PhotoLicense = info.license
						referencedPhotos[fileName] = true
					}
				}
			}
		}
		pack.Pros = append(pack.Pros, pro)
	}
	if *photos {
		pruneUnreferencedPhotos(photosDir, referencedPhotos)
	}
	carryOverRanks(filepath.Join(*outDir, "pros.json"), pack.Pros)
	sort.Slice(pack.Pros, func(i, j int) bool { return pack.Pros[i].ID < pack.Pros[j].ID })
	data, err := json.MarshalIndent(pack, "", " ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(*outDir, "pros.json"), append(data, '\n'), 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("wrote %d pros to %s (%d KB)", len(pack.Pros), *outDir, len(data)/1024)
}

// loadRoster reads the scfingerprint dataset identities and pairs each with
// its registry aurora IDs by name (dataset IDs are the lower-cased registry
// names). Identities unknown to the registry cannot be labelled and are skipped.
func loadRoster(datasetDir string, reg *procorpus.Registry) (map[string]roster, error) {
	var links map[string]string
	if err := procorpus.ReadJSON(filepath.Join(datasetDir, "liquipedia.json"), &links); err != nil {
		return nil, err
	}
	registryNameByLower := map[string]string{}
	for name := range reg.AurorasByName {
		registryNameByLower[strings.ToLower(name)] = name
	}
	entries, err := os.ReadDir(filepath.Join(datasetDir, "players"))
	if err != nil {
		return nil, err
	}
	out := map[string]roster{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		var identity datasetIdentity
		if err := procorpus.ReadJSON(filepath.Join(datasetDir, "players", entry.Name()), &identity); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		id := strings.ToLower(strings.TrimSpace(identity.ID))
		registryName, ok := registryNameByLower[id]
		if !ok {
			log.Printf("roster: %s is not in pros_merged.json, skipping", id)
			continue
		}
		label := registryName
		for _, alias := range identity.Aliases {
			if alias.Primary && strings.TrimSpace(alias.Name) != "" {
				label = strings.TrimSpace(alias.Name)
			}
		}
		out[registryName] = roster{
			id:         id,
			label:      label,
			confidence: identity.Confidence,
			liquipedia: links[id],
			auroraIDs:  reg.AurorasByName[registryName],
		}
	}
	return out, nil
}

// renameProRows points every resolved pro player row at its pack key. Pros
// unknown to the roster (registry-only names) are left alone.
func replays1v1(corpus *dashboard.ProCorpus) []procorpus.Replay1v1 {
	analysed := corpus.Replays1v1()
	out := make([]procorpus.Replay1v1, 0, len(analysed))
	for _, replay := range analysed {
		players := make([]procorpus.PlayerRow, 0, len(replay.Players))
		for _, player := range replay.Players {
			players = append(players, procorpus.PlayerRow{ID: player.PlayerID, Name: player.Name, Race: player.Race})
		}
		out = append(out, procorpus.Replay1v1{
			ID:       replay.ReplayID,
			FileName: replay.FileName,
			Matchup:  replay.Matchup,
			Players:  players,
		})
	}
	return out
}

func packKeyByPlayerID(joined []procorpus.JoinedPlayer, rosterByName map[string]roster) map[int64]string {
	out := make(map[int64]string, len(joined))
	for _, jp := range joined {
		if r, ok := rosterByName[jp.Side.ProName]; ok {
			out[jp.PlayerID] = propack.Key(r.id)
		}
	}
	return out
}

// recentToons lists each pro's known (toon, gateway) accounts, most recently
// seen first, capped so the app spends at most a handful of bridge requests
// on a profile page.
func recentToons(sides []procorpus.ProSide) map[string][]propack.Toon {
	const maxToons = 6
	type seen struct {
		toon propack.Toon
		last int64
	}
	byName := map[string]map[string]*seen{}
	for _, s := range sides {
		toon := strings.TrimSpace(s.Toon)
		if toon == "" || s.Gateway == 0 {
			continue
		}
		key := fmt.Sprintf("%s/%d", strings.ToLower(toon), s.Gateway)
		if byName[s.ProName] == nil {
			byName[s.ProName] = map[string]*seen{}
		}
		entry, ok := byName[s.ProName][key]
		if !ok {
			entry = &seen{toon: propack.Toon{Toon: toon, Gateway: s.Gateway}}
			byName[s.ProName][key] = entry
		}
		if s.Timestamp > entry.last {
			entry.last = s.Timestamp
		}
	}
	out := map[string][]propack.Toon{}
	for name, entries := range byName {
		list := make([]*seen, 0, len(entries))
		for _, e := range entries {
			list = append(list, e)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].last > list[j].last })
		for i, e := range list {
			if i >= maxToons {
				break
			}
			out[name] = append(out[name], e.toon)
		}
	}
	return out
}

func mainRace(races map[string]int) string {
	best, bestGames := "", -1
	for race, games := range races {
		if games > bestGames || (games == bestGames && race < best) {
			best, bestGames = race, games
		}
	}
	return best
}

func pruneUnreferencedPhotos(photosDir string, referenced map[string]bool) {
	entries, err := os.ReadDir(photosDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || referenced[entry.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(photosDir, entry.Name())); err == nil {
			log.Printf("pruned stale photo %s", entry.Name())
		}
	}
}

// Liquipedia: MediaWiki API on liquipedia.net/starcraft. Their API terms ask
// for gzip, an identifying User-Agent and at most one request every 2 seconds.

type liquipediaClient struct {
	http    *http.Client
	lastReq time.Time
}

type liquipediaInfo struct {
	country  string
	imageURL string
	// license is the image's LicenseShortName from extmetadata when Liquipedia
	// exposes one; most player photos carry none, which is exactly the
	// per-image rights question issue #324 raises.
	license string
}

func newLiquipediaClient() *liquipediaClient {
	return &liquipediaClient{http: &http.Client{Timeout: 60 * time.Second}}
}

func (c *liquipediaClient) get(rawURL string) ([]byte, error) {
	if wait := 2*time.Second - time.Since(c.lastReq); wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Encoding", "gzip")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d for %s", resp.StatusCode, rawURL)
	}
	var body io.Reader = resp.Body
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		body = gz
	}
	return io.ReadAll(body)
}

var (
	infoboxImageRe   = regexp.MustCompile(`(?m)^\|\s*image\s*=\s*([^\n|]+)`)
	infoboxCountryRe = regexp.MustCompile(`(?m)^\|\s*country\s*=\s*([^\n|]+)`)
)

// playerInfo reads the page's wikitext for the infobox image and country, then
// resolves the image to a 160px thumbnail URL.
func (c *liquipediaClient) playerInfo(pageURL string) (liquipediaInfo, error) {
	title := strings.TrimPrefix(pageURL, "https://liquipedia.net/starcraft/")
	title, _ = url.PathUnescape(title)
	if title == "" || title == pageURL {
		return liquipediaInfo{}, fmt.Errorf("not a liquipedia starcraft URL: %s", pageURL)
	}
	q := url.Values{}
	q.Set("action", "query")
	q.Set("titles", title)
	q.Set("prop", "revisions")
	q.Set("rvprop", "content")
	q.Set("rvslots", "main")
	q.Set("redirects", "1")
	q.Set("format", "json")
	q.Set("formatversion", "2")
	body, err := c.get("https://liquipedia.net/starcraft/api.php?" + q.Encode())
	if err != nil {
		return liquipediaInfo{}, err
	}
	var parsed struct {
		Query struct {
			Pages []struct {
				Missing   bool `json:"missing"`
				Revisions []struct {
					Slots struct {
						Main struct {
							Content string `json:"content"`
						} `json:"main"`
					} `json:"slots"`
				} `json:"revisions"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return liquipediaInfo{}, err
	}
	if len(parsed.Query.Pages) == 0 || parsed.Query.Pages[0].Missing || len(parsed.Query.Pages[0].Revisions) == 0 {
		return liquipediaInfo{}, fmt.Errorf("page %q missing", title)
	}
	text := parsed.Query.Pages[0].Revisions[0].Slots.Main.Content
	info := liquipediaInfo{}
	if m := infoboxCountryRe.FindStringSubmatch(text); m != nil {
		info.country = countryAlpha2(strings.TrimSpace(m[1]))
	}
	if m := infoboxImageRe.FindStringSubmatch(text); m != nil && strings.TrimSpace(m[1]) != "" {
		imageURL, license, err := c.thumbURL(strings.TrimSpace(m[1]))
		if err != nil {
			// Country is still good; only the portrait is missing.
			log.Printf("liquipedia %s: %v", title, err)
			return info, nil
		}
		info.imageURL = imageURL
		info.license = license
	}
	return info, nil
}

func (c *liquipediaClient) thumbURL(fileName string) (string, string, error) {
	q := url.Values{}
	q.Set("action", "query")
	q.Set("titles", "File:"+fileName)
	q.Set("prop", "imageinfo")
	q.Set("iiprop", "url|mime|extmetadata")
	q.Set("iiurlwidth", "160")
	q.Set("format", "json")
	q.Set("formatversion", "2")
	body, err := c.get("https://liquipedia.net/starcraft/api.php?" + q.Encode())
	if err != nil {
		return "", "", err
	}
	var parsed struct {
		Query struct {
			Pages []struct {
				ImageInfo []struct {
					ThumbURL    string `json:"thumburl"`
					URL         string `json:"url"`
					Mime        string `json:"mime"`
					ExtMetadata map[string]struct {
						Value string `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", err
	}
	if len(parsed.Query.Pages) == 0 || len(parsed.Query.Pages[0].ImageInfo) == 0 {
		return "", "", fmt.Errorf("no imageinfo for %q", fileName)
	}
	ii := parsed.Query.Pages[0].ImageInfo[0]
	if ii.Mime != "image/jpeg" && ii.Mime != "image/png" {
		return "", "", fmt.Errorf("unsupported mime %q for %q", ii.Mime, fileName)
	}
	license := ii.ExtMetadata["LicenseShortName"].Value
	if ii.ThumbURL != "" {
		return ii.ThumbURL, license, nil
	}
	return ii.URL, license, nil
}

func (c *liquipediaClient) downloadPhoto(imageURL, photosDir, id string) (string, error) {
	data, err := c.get(imageURL)
	if err != nil {
		return "", err
	}
	ext := ".jpg"
	if len(data) >= 8 && string(data[1:4]) == "PNG" {
		ext = ".png"
	} else if len(data) < 3 || data[0] != 0xFF || data[1] != 0xD8 {
		return "", fmt.Errorf("not a JPEG or PNG (%d bytes)", len(data))
	}
	fileName := id + ext
	if err := os.WriteFile(filepath.Join(photosDir, fileName), data, 0o644); err != nil {
		return "", err
	}
	return fileName, nil
}

var countryCodes = map[string]string{
	"south korea": "KR", "korea": "KR", "republic of korea": "KR", "united states": "US", "usa": "US",
	"china": "CN", "poland": "PL", "germany": "DE", "russia": "RU", "canada": "CA", "brazil": "BR",
	"france": "FR", "united kingdom": "GB", "sweden": "SE", "netherlands": "NL", "spain": "ES",
	"italy": "IT", "australia": "AU", "japan": "JP", "taiwan": "TW", "ukraine": "UA", "finland": "FI",
	"norway": "NO", "denmark": "DK", "belgium": "BE", "austria": "AT", "switzerland": "CH",
	"mexico": "MX", "argentina": "AR", "peru": "PE", "chile": "CL", "vietnam": "VN", "bulgaria": "BG",
	"romania": "RO", "hungary": "HU", "czech republic": "CZ", "czechia": "CZ", "israel": "IL",
	"turkey": "TR", "portugal": "PT", "ireland": "IE", "greece": "GR", "hong kong": "HK",
	"belarus": "BY", "kazakhstan": "KZ", "lithuania": "LT", "latvia": "LV", "estonia": "EE",
	"croatia": "HR", "serbia": "RS", "slovakia": "SK", "slovenia": "SI", "philippines": "PH",
	"indonesia": "ID", "malaysia": "MY", "singapore": "SG", "thailand": "TH", "india": "IN",
	"new zealand": "NZ", "south africa": "ZA", "colombia": "CO", "venezuela": "VE",
}

func countryAlpha2(name string) string {
	return countryCodes[strings.ToLower(strings.TrimSpace(name))]
}

// carryOverRanks copies the curated popularity rank of each pro from the
// previously committed pack, so regenerating metrics never reshuffles the
// featured strip. Ranks were set once by hand from recent ASL results.
func carryOverRanks(previousPath string, pros []propack.Pro) {
	var previous propack.Pack
	if err := procorpus.ReadJSON(previousPath, &previous); err != nil {
		log.Printf("no previous pack at %s, ranks start empty: %v", previousPath, err)
		return
	}
	ranks := map[string]int{}
	for _, p := range previous.Pros {
		if p.Rank > 0 {
			ranks[p.ID] = p.Rank
		}
	}
	for i := range pros {
		pros[i].Rank = ranks[pros[i].ID]
	}
}
