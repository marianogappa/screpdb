package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/icza/screp/repparser"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/propack"
)

const minGameSeconds = 420

type harvestRow struct {
	MatchID  string `json:"matchId"`
	File     string `json:"file"`
	AuroraID int64  `json:"auroraId"`
	Toon     string `json:"toon"`
	Race     string `json:"race"`
	OppAur   int64  `json:"oppAuroraId"`
	OppToon  string `json:"oppToon"`
	OppPro   string `json:"oppProName"`
	OppRace  string `json:"oppRace"`
}

type side struct {
	pro, toon, race, path string
}

type sideTraits struct {
	Pro  string `json:"pro"`
	Race string `json:"race"`
	gameTraits
}

func main() {
	harvests := flag.String("harvest", "", "comma-separated screpharvest output dirs")
	traitsOut := flag.String("traits", "", "write per-game traits as JSON lines here")
	out := flag.String("out", "internal/propack/data/hotkey_oddities.json", "oddities file to write")
	flag.Parse()

	sides := loadSides(strings.Split(*harvests, ","))
	fmt.Fprintln(os.Stderr, "pro sides:", len(sides))
	results := extractAll(sides)
	fmt.Fprintln(os.Stderr, "games kept:", len(results))
	if *traitsOut != "" {
		f, err := os.Create(*traitsOut)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		enc := json.NewEncoder(f)
		for _, r := range results {
			enc.Encode(r)
		}
		f.Close()
	}
	builtIn := builtInGames(results)
	comparisonPool = buildComparisonPool(builtIn)
	if err := writeOddities(*out, measuredOddities(inPoolGames(builtIn)), proRates(builtIn)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// builtInGames keeps the games of built-in players, so rate lines compare a player with the same pool the
// chart lines do and the app can call that pool "built-in players".
func builtInGames(results []sideTraits) []sideTraits {
	pack, err := propack.Load()
	if err != nil {
		return nil
	}
	var out []sideTraits
	for _, r := range results {
		if pro := pack.ByLabel(r.Pro); pro != nil && !excludedProIDs[pro.ID] {
			out = append(out, r)
		}
	}
	return out
}

// comparisonPool is every player and race all habit lines compare against, keyed "id|race name": a built-in
// player whose hotkey chart and ladder replays both hold at least poolMinGames games of the race. One pool
// for every line keeps the "of N players" counts on a card the same.
var comparisonPool map[string]bool

const poolMinGames = 20

func inPool(id, race string) bool { return comparisonPool[id+"|"+race] }

func buildComparisonPool(games []sideTraits) map[string]bool {
	pack, err := propack.Load()
	if err != nil {
		return nil
	}
	replays := map[string]int{}
	for _, g := range games {
		if pro := pack.ByLabel(g.Pro); pro != nil {
			replays[pro.ID+"|"+raceNames[g.Race]]++
		}
	}
	pool := map[string]bool{}
	for i := range pack.Pros {
		pro := &pack.Pros[i]
		if excludedProIDs[pro.ID] {
			continue
		}
		for _, sig := range pro.Hotkeys {
			key := pro.ID + "|" + sig.Race
			if sig.Games >= poolMinGames && replays[key] >= poolMinGames {
				pool[key] = true
			}
		}
	}
	return pool
}

func inPoolGames(games []sideTraits) []sideTraits {
	pack, err := propack.Load()
	if err != nil {
		return nil
	}
	var out []sideTraits
	for _, g := range games {
		if pro := pack.ByLabel(g.Pro); pro != nil && inPool(pro.ID, raceNames[g.Race]) {
			out = append(out, g)
		}
	}
	return out
}

func writeOddities(path string, byLabel map[string]map[string][]propack.HotkeyOddity, ratesByLabel map[string]map[string]hotkeystream.Rates) error {
	pack, err := propack.Load()
	if err != nil {
		return err
	}
	file := propack.OdditiesFile{Rates: map[string]map[string]hotkeystream.Rates{}, Source: "key habits from the pack's hotkey signatures; rates from the screpharvest ladder corpus (scripts/hotkey-oddities)", Pros: map[string]map[string][]propack.HotkeyOddity{}}
	for label, byRace := range byLabel {
		if pro := pack.ByLabel(label); pro != nil {
			file.Pros[pro.ID] = byRace
		}
	}
	for label, byRace := range ratesByLabel {
		if pro := pack.ByLabel(label); pro != nil && !excludedProIDs[pro.ID] {
			file.Rates[pro.ID] = byRace
		}
	}
	for id, byRace := range chartOddities(pack) {
		if file.Pros[id] == nil {
			file.Pros[id] = map[string][]propack.HotkeyOddity{}
		}
		for race, found := range byRace {
			file.Pros[id][race] = append(file.Pros[id][race], found...)
		}
	}
	for id, byRace := range standardLayoutOddities(pack) {
		if file.Pros[id] == nil {
			file.Pros[id] = map[string][]propack.HotkeyOddity{}
		}
		for race, found := range byRace {
			file.Pros[id][race] = append(file.Pros[id][race], found...)
		}
	}
	for id, byRace := range uncommonOddities(pack) {
		if file.Pros[id] == nil {
			file.Pros[id] = map[string][]propack.HotkeyOddity{}
		}
		for race, found := range byRace {
			file.Pros[id][race] = append(file.Pros[id][race], found...)
		}
	}
	for _, byRace := range file.Pros {
		for race, found := range byRace {
			byRace[race] = finalize(dropContradictions(dropRepeatedHall(dropRepeatedUnit(found))))
		}
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func loadSides(dirs []string) []side {
	pro := map[int64]string{}
	for _, dir := range dirs {
		scanLines(filepath.Join(dir, "identities.jsonl"), func(b []byte) {
			var id struct {
				Handles []struct {
					AuroraID int64  `json:"auroraId"`
					ProName  string `json:"proName"`
				} `json:"handles"`
			}
			json.Unmarshal(b, &id)
			for _, h := range id.Handles {
				if h.ProName != "" && h.AuroraID > 0 {
					pro[h.AuroraID] = h.ProName
				}
			}
		})
	}
	seen := map[string]bool{}
	var sides []side
	for _, dir := range dirs {
		scanLines(filepath.Join(dir, "replays.jsonl"), func(b []byte) {
			var r harvestRow
			json.Unmarshal(b, &r)
			path := filepath.Join(dir, r.File)
			add := func(p, toon, race string) {
				if p == "" || p == "Sai" || seen[r.MatchID+"|"+toon] {
					return
				}
				seen[r.MatchID+"|"+toon] = true
				sides = append(sides, side{p, toon, race, path})
			}
			if r.AuroraID > 0 {
				add(pro[r.AuroraID], r.Toon, r.Race)
			}
			if r.OppAur > 0 {
				p := r.OppPro
				if p == "" {
					p = pro[r.OppAur]
				}
				add(p, r.OppToon, r.OppRace)
			}
		})
	}
	return sides
}

func scanLines(path string, fn func([]byte)) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		fn(sc.Bytes())
	}
}

func extractAll(sides []side) []sideTraits {
	jobs := make(chan side)
	out := make(chan sideTraits)
	var wg sync.WaitGroup
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				if t, ok := extractSide(s); ok {
					out <- t
				}
			}
		}()
	}
	go func() {
		for _, s := range sides {
			jobs <- s
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()
	var all []sideTraits
	for t := range out {
		all = append(all, t)
	}
	return all
}

func extractSide(s side) (t sideTraits, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	if _, err := os.Stat(s.path); err != nil {
		return t, false
	}
	r, err := repparser.ParseFile(s.path)
	if err != nil || int(r.Header.Frames.Seconds()) < minGameSeconds {
		return t, false
	}
	for _, p := range r.Header.Players {
		if p != nil && strings.EqualFold(p.Name, s.toon) {
			events := hotkeystream.Extract(r)[p.ID]
			return sideTraits{Pro: s.pro, Race: s.race, gameTraits: extractTraits(events, r.Commands.Cmds, p.ID, int(r.Header.Frames.Seconds()))}, true
		}
	}
	return t, false
}
