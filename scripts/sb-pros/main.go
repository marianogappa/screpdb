// sb-pros refreshes internal/propack/data/shieldbattery.json, the confirmed
// ShieldBattery accounts of the built-in progamers, from a public TeamLiquid
// post, verifying every account against ShieldBattery's public user API.
//
//	go run ./scripts/sb-pros          # rewrite the committed file
//	go run ./scripts/sb-pros -check   # CI: fail when the committed file is stale
//
// Network failures only warn: the list must not block unrelated PRs because a
// third-party site is down.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	sourceURL    = "https://tl.net/forum/brood-war/646806-list-of-top-players-active-on-shieldbattery"
	batchInfoURL = "https://shieldbattery.net/api/1/users/batch-info"
	// batch-info rejects more than 20 ids per call; a few calls stay far
	// below its ~40 requests per minute limit.
	batchInfoMaxIDs = 20
	userAgent       = "Mozilla/5.0 (compatible; screpdb-sb-pros/1.0)"
	maxVanishing    = 0.20
)

// labelAliases covers TL labels that are neither a pack label nor a Liquipedia
// page name of the same player.
var labelAliases = map[string]string{
	"jd":   "jaedong",
	"no0b": "noob",
}

type account struct {
	ID   uint32 `json:"id"`
	Name string `json:"name"`
}

type output struct {
	Source string               `json:"source"`
	Pros   map[string][]account `json:"pros"`
}

var client = &http.Client{Timeout: 30 * time.Second}

type networkError struct{ err error }

func (e networkError) Error() string { return e.err.Error() }
func (e networkError) Unwrap() error { return e.err }

func main() {
	check := flag.Bool("check", false, "write nothing; exit nonzero when the committed file differs from a fresh scrape")
	out := flag.String("out", "internal/propack/data/shieldbattery.json", "file to write or check")
	packPath := flag.String("pack", "internal/propack/data/pros.json", "embedded pro pack to map TL labels onto")
	flag.Parse()

	if err := run(*check, *out, *packPath); err != nil {
		var netErr networkError
		if errors.As(err, &netErr) {
			fmt.Printf("::warning::sb-pros skipped, network failure: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "sb-pros: %v\n", err)
		os.Exit(1)
	}
}

func run(check bool, outPath, packPath string) error {
	page, err := get(sourceURL)
	if err != nil {
		return networkError{fmt.Errorf("fetching %s: %w", sourceURL, err)}
	}
	entries, err := parsePage(string(page))
	if err != nil {
		return err
	}

	committed, err := readCommitted(outPath)
	if err != nil {
		return err
	}
	if err := checkVanishing(committed, entries); err != nil {
		return err
	}

	names, err := verify(entries)
	if err != nil {
		if check {
			return networkError{fmt.Errorf("verifying accounts on ShieldBattery: %w", err)}
		}
		fmt.Printf("::warning::could not verify accounts on ShieldBattery, keeping the TL names: %v\n", err)
		names = nil
	}

	pack, err := loadPack(packPath)
	if err != nil {
		return err
	}
	result, unmatched := build(entries, pack, names)
	if len(unmatched) > 0 {
		fmt.Printf("TL labels with no built-in pro (not written): %s\n", strings.Join(unmatched, ", "))
	}

	data, err := encode(result)
	if err != nil {
		return err
	}
	if check {
		if committed == nil || !bytes.Equal(committed.raw, data) {
			return fmt.Errorf("%s is out of date with %s; run `go run ./scripts/sb-pros` and commit the result\n%s", outPath, sourceURL, describeDiff(committed, result))
		}
		fmt.Printf("%s is up to date (%d pros)\n", outPath, len(result.Pros))
		return nil
	}
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%d pros)\n", outPath, len(result.Pros))
	return nil
}

func get(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 8<<20))
}

func allAccounts(entries []tlEntry) []tlAccount {
	var accs []tlAccount
	for _, e := range entries {
		for _, a := range e.Accounts {
			accs = append(accs, a)
			accs = append(accs, a.Formerly...)
		}
	}
	return accs
}

func verify(entries []tlEntry) (map[uint32]string, error) {
	var ids []uint32
	seen := map[uint32]bool{}
	for _, a := range allAccounts(entries) {
		if !seen[a.ID] {
			seen[a.ID] = true
			ids = append(ids, a.ID)
		}
	}
	names := make(map[uint32]string, len(ids))
	for start := 0; start < len(ids); start += batchInfoMaxIDs {
		q := url.Values{}
		for _, id := range ids[start:min(start+batchInfoMaxIDs, len(ids))] {
			q.Add("u", strconv.FormatUint(uint64(id), 10))
		}
		body, err := get(batchInfoURL + "?" + q.Encode())
		if err != nil {
			return nil, err
		}
		var resp struct {
			UserInfos []struct {
				ID   uint32 `json:"id"`
				Name string `json:"name"`
			} `json:"userInfos"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("decoding batch-info: %w", err)
		}
		for _, u := range resp.UserInfos {
			if u.Name != "" {
				names[u.ID] = u.Name
			}
		}
	}
	for _, id := range ids {
		if _, ok := names[id]; !ok {
			fmt.Printf("::warning::ShieldBattery has no user %d, keeping the TL name\n", id)
		}
	}
	return names, nil
}

type packPro struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Liquipedia string `json:"liquipedia"`
}

func loadPack(path string) ([]packPro, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading pro pack: %w", err)
	}
	var pack struct {
		Pros []packPro `json:"pros"`
	}
	if err := json.Unmarshal(raw, &pack); err != nil {
		return nil, fmt.Errorf("parsing pro pack: %w", err)
	}
	return pack.Pros, nil
}

func liquipediaName(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	name := u.Path[strings.LastIndex(u.Path, "/")+1:]
	if i := strings.Index(name, "_("); i >= 0 {
		name = name[:i]
	}
	return strings.ReplaceAll(name, "_", " ")
}

func proIndex(pack []packPro) map[string]string {
	byName := map[string]string{}
	for _, p := range pack {
		if lp := strings.ToLower(liquipediaName(p.Liquipedia)); lp != "" {
			if _, taken := byName[lp]; !taken {
				byName[lp] = p.ID
			}
		}
	}
	for _, p := range pack {
		byName[strings.ToLower(strings.TrimSpace(p.Label))] = p.ID
	}
	for alias, label := range labelAliases {
		if id, ok := byName[label]; ok {
			byName[alias] = id
		}
	}
	return byName
}

func build(entries []tlEntry, pack []packPro, names map[uint32]string) (output, []string) {
	byName := proIndex(pack)
	name := func(a tlAccount) string {
		if n, ok := names[a.ID]; ok {
			return n
		}
		return a.Name
	}
	result := output{Source: sourceURL, Pros: map[string][]account{}}
	var unmatched []string
	for _, e := range entries {
		id, ok := byName[strings.ToLower(e.Label)]
		if !ok {
			unmatched = append(unmatched, e.Label)
			continue
		}
		for _, a := range e.Accounts {
			result.Pros[id] = append(result.Pros[id], account{ID: a.ID, Name: name(a)})
			for _, f := range a.Formerly {
				result.Pros[id] = append(result.Pros[id], account{ID: f.ID, Name: name(f)})
			}
		}
	}
	for _, accs := range result.Pros {
		sort.Slice(accs, func(i, j int) bool { return accs[i].ID < accs[j].ID })
	}
	sort.Strings(unmatched)
	return result, unmatched
}

func encode(o output) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(o); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type committedFile struct {
	raw    []byte
	parsed output
}

func readCommitted(path string) (*committedFile, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := &committedFile{raw: raw}
	if err := json.Unmarshal(raw, &c.parsed); err != nil {
		return nil, fmt.Errorf("parsing committed %s: %w", path, err)
	}
	return c, nil
}

func checkVanishing(committed *committedFile, entries []tlEntry) error {
	if committed == nil {
		return nil
	}
	fresh := map[uint32]bool{}
	for _, a := range allAccounts(entries) {
		fresh[a.ID] = true
	}
	var total, gone int
	for _, accs := range committed.parsed.Pros {
		for _, a := range accs {
			total++
			if !fresh[a.ID] {
				gone++
			}
		}
	}
	if total > 0 && float64(gone)/float64(total) > maxVanishing {
		return formatChanged("%d of %d committed accounts vanished at once (more than %.0f%%)", gone, total, maxVanishing*100)
	}
	return nil
}

func describeDiff(committed *committedFile, fresh output) string {
	if committed == nil {
		return "the committed file is missing"
	}
	var lines []string
	ids := map[string]bool{}
	for id := range committed.parsed.Pros {
		ids[id] = true
	}
	for id := range fresh.Pros {
		ids[id] = true
	}
	sorted := make([]string, 0, len(ids))
	for id := range ids {
		sorted = append(sorted, id)
	}
	sort.Strings(sorted)
	for _, id := range sorted {
		was, _ := json.Marshal(committed.parsed.Pros[id])
		now, _ := json.Marshal(fresh.Pros[id])
		if !bytes.Equal(was, now) {
			lines = append(lines, fmt.Sprintf("  %s: committed %s, scraped %s", id, was, now))
		}
	}
	if len(lines) == 0 {
		return "  (formatting differs only)"
	}
	return strings.Join(lines, "\n")
}
