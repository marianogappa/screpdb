// Command skill-mine measures every candidate skill metric on ladder games whose players' MMR is known,
// so the Skill tab only shows metrics that follow MMR and places its tiers where the ladder puts them.
//
// It samples the screpharvest corpus evenly across MMR (a cap of games per 100-MMR bin), runs the fast
// pass (the library loader) and the engine pass on each game, and writes one JSON line per player:
// MMR, race, whether the side is a labelled pro, and the metric values. Interrupted runs resume.
//
//	go run ./scripts/skill-mine \
//	  -harvest ../screpharvest/harvest \
//	  -corpus  ../scfingerprint/corpus \
//	  -bwdata  "$HOME/Library/Application Support/screpdb/bwdata" \
//	  -out     /tmp/skill-mine.jsonl
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/icza/screp/rep/repcmd"
	"github.com/icza/screp/rep/repcore"
	"github.com/icza/screp/repparser"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/macro"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/compact"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/parser"
	"github.com/marianogappa/screpdb/internal/skill"
	"github.com/marianogappa/screpdb/scripts/procorpus"
)

type side struct {
	Toon    string `json:"toon"`
	Aurora  int64  `json:"aurora"`
	Race    string `json:"race"`
	MMR     int    `json:"mmr"`
	Pro     string `json:"pro,omitempty"`
	OppRace string `json:"opp_race"`
	OppMMR  int    `json:"opp_mmr"`
}

type sample struct {
	Match    string             `json:"match"`
	Duration int                `json:"duration"`
	Side     side               `json:"side"`
	Won      *bool              `json:"won,omitempty"`
	Engine   string             `json:"engine,omitempty"`
	Values   map[string]float64 `json:"values,omitempty"`
	Skip     string             `json:"skip,omitempty"`
	// MapKind and Humans describe unlabelled games (-dir), where the question is how a kind of game shifts
	// the metrics rather than how they follow MMR.
	MapKind string `json:"map_kind,omitempty"`
	Humans  int    `json:"humans,omitempty"`
}

type job struct {
	row   procorpus.HarvestRow
	path  string
	sides [2]side
}

func main() {
	harvestDirs := flag.String("harvest", "", "screpharvest harvest dirs, comma separated (e.g. the pro harvest and the tiers harvest)")
	corpusDir := flag.String("corpus", "", "scfingerprint corpus dir, to label pro sides")
	bwdata := flag.String("bwdata", "", "extracted StarCraft data dir for the engine pass")
	out := flag.String("out", filepath.Join(os.TempDir(), "skill-mine.jsonl"), "output JSONL (resumes)")
	perBin := flag.Int("per-bin", 160, "games per 100-MMR bin of the lower-MMR side")
	minDuration := flag.Int("min-duration", 480, "minimum game duration in seconds")
	workers := flag.Int("workers", max(1, runtime.NumCPU()-1), "parallel games")
	dir := flag.String("dir", "", "instead of the harvest, measure unlabelled replays under this folder (for map kinds and team games)")
	perKind := flag.Int("per-kind", 300, "with -dir: games per map kind (regular, money)")
	flag.Parse()
	if *dir != "" {
		data, err := engine.LoadData(*bwdata)
		if err != nil {
			log.Fatalf("game data: %v", err)
		}
		mineDir(*dir, *out, data, *perKind, *minDuration, *workers)
		return
	}

	pros := map[string]string{}
	if *corpusDir != "" {
		reg, err := procorpus.LoadRegistry(*corpusDir)
		if err != nil {
			log.Fatalf("registry: %v", err)
		}
		for _, dir := range strings.Split(*harvestDirs, ",") {
			sides, err := procorpus.LabelCorpus(dir, reg, *minDuration)
			if err != nil {
				log.Fatalf("label: %v", err)
			}
			for _, s := range sides {
				pros[fmt.Sprintf("%s:%d", s.MatchID, s.AuroraID)] = s.ProName
			}
		}
	}
	jobs := pick(strings.Split(*harvestDirs, ","), pros, *perBin, *minDuration, done(*out))
	log.Printf("%d games to run (%d pro sides labelled)", len(jobs), len(pros))

	data, err := engine.LoadData(*bwdata)
	if err != nil {
		log.Fatalf("game data: %v", err)
	}
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var mu sync.Mutex
	w := bufio.NewWriter(f)
	ch := make(chan job)
	var wg sync.WaitGroup
	finished := 0
	for range *workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				samples := run(j, data)
				mu.Lock()
				for _, s := range samples {
					b, _ := json.Marshal(s)
					w.Write(append(b, '\n'))
				}
				w.Flush()
				finished++
				if finished%50 == 0 {
					log.Printf("%d/%d games", finished, len(jobs))
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
}

func done(path string) map[string]bool {
	seen := map[string]bool{}
	f, err := os.Open(path)
	if err != nil {
		return seen
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var x sample
		if json.Unmarshal(s.Bytes(), &x) == nil {
			seen[x.Match] = true
		}
	}
	return seen
}

// pick samples 1v1 ladder games with both MMRs known, evenly across MMR: each game is binned by its
// lower MMR so a bin's games really are played at that level.
func pick(harvestDirs []string, pros map[string]string, perBin, minDuration int, skip map[string]bool) []job {
	var all []candidate
	seen := map[string]bool{}
	for _, harvestDir := range harvestDirs {
		all = append(all, readHarvest(harvestDir, minDuration, seen)...)
	}
	return pickEvenly(all, pros, perBin, skip)
}

type candidate struct {
	row  procorpus.HarvestRow
	file string
}

func readHarvest(harvestDir string, minDuration int, seen map[string]bool) []candidate {
	f, err := os.Open(filepath.Join(harvestDir, "replays.jsonl"))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var all []candidate
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var row struct {
			procorpus.HarvestRow
			File string `json:"file"`
		}
		if json.Unmarshal(s.Bytes(), &row) != nil || seen[row.MatchID] {
			continue
		}
		seen[row.MatchID] = true
		if row.MMR <= 0 || row.OppMMR <= 0 || row.Duration < minDuration || len(row.Race) != 1 || len(row.OppRace) != 1 {
			continue
		}
		all = append(all, candidate{row.HarvestRow, filepath.Join(harvestDir, row.File)})
	}
	return all
}

func pickEvenly(all []candidate, pros map[string]string, perBin int, skip map[string]bool) []job {
	rand.New(rand.NewSource(1)).Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	bins := map[int]int{}
	var jobs []job
	for _, c := range all {
		bin := min(c.row.MMR, c.row.OppMMR) / 100
		if bins[bin] >= perBin {
			continue
		}
		bins[bin]++
		if skip[c.row.MatchID] {
			continue
		}
		if _, err := os.Stat(c.file); err != nil {
			continue
		}
		r := c.row
		jobs = append(jobs, job{row: r, path: c.file, sides: [2]side{
			{Toon: r.Toon, Aurora: r.AuroraID, Race: r.Race, MMR: r.MMR, OppRace: r.OppRace, OppMMR: r.OppMMR, Pro: pros[fmt.Sprintf("%s:%d", r.MatchID, r.AuroraID)]},
			{Toon: r.OppToon, Aurora: r.OppAuroraID, Race: r.OppRace, MMR: r.OppMMR, OppRace: r.Race, OppMMR: r.MMR, Pro: pros[fmt.Sprintf("%s:%d", r.MatchID, r.OppAuroraID)]},
		}})
	}
	keys := make([]int, 0, len(bins))
	for k := range bins {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		log.Printf("MMR %d-%d: %d games", k*100, k*100+99, bins[k])
	}
	return jobs
}

func run(j job, data *engine.Data) []sample {
	fail := func(why string) []sample { return []sample{{Match: j.row.MatchID, Skip: why}} }
	b, err := os.ReadFile(j.path)
	if err != nil {
		return fail(err.Error())
	}
	replay := parser.CreateReplayFromFileInfo(j.path, filepath.Base(j.path), int64(len(b)), "")
	parsed, err := parser.ParseReplayWithOptions(j.path, replay, parser.Options{})
	if err != nil {
		return fail("parse: " + err.Error())
	}
	rec, err := compact.FromReplayData(parsed, compact.FileMeta{Path: j.path, Size: int64(len(b))})
	if err != nil {
		return fail("compact: " + err.Error())
	}
	var humans []*library.Player
	for i := range rec.Players {
		p := &rec.Players[i]
		if !p.IsObserver() && p.IsHuman() {
			humans = append(humans, p)
		}
	}
	if len(humans) != 2 {
		return fail("not a 1v1")
	}
	players := resolve(humans, j.sides)
	if players[0] == nil || players[1] == nil {
		return fail("sides not resolved")
	}
	var scorecards, early map[uint8]macro.Player
	var at8 map[int8]engine.EconomySample
	var windowed map[int8]map[string]float64
	engineState := ""
	d, err := engine.Pass(context.Background(), b, data)
	switch {
	case err != nil:
		engineState = "error"
	case d.Confidence() != engine.ConfidenceHigh:
		engineState = "low"
	default:
		engineState = "high"
		scorecards = map[uint8]macro.Player{}
		for _, m := range macro.Compute(d) {
			scorecards[uint8(m.Slot)] = m
		}
		early = map[uint8]macro.Player{}
		for _, m := range macro.Compute(firstSeconds(d, earlySeconds)) {
			early[uint8(m.Slot)] = m
		}
		at8 = economyAt(d, earlySeconds)
		windowed = windowedUnspent(d)
	}
	actions := actionStats(b)
	minutes := float64(j.row.Duration) / 60
	var out []sample
	for i, p := range players {
		values := fastValues(p, minutes)
		for k, v := range actions[p.ReplayPlayerID] {
			values[k] = v
		}
		if d != nil && engineState == "high" {
			for k, v := range macroWindowValues(d, int8(p.Slot), macro.WorkerWindowEnd) {
				values[k] = v
			}
		}
		if m, ok := scorecards[p.Slot]; ok {
			for k, v := range engineValues(m) {
				values[k] = v
			}
			for k, v := range candidateValues(d, int8(p.Slot), m.Seconds, p.Race == library.RaceZerg) {
				values[k] = v
			}
			for k, v := range armyUseValues(d, int8(p.Slot)) {
				values[k] = v
			}
			for k, v := range windowed[int8(p.Slot)] {
				values[k] = v
			}
		}
		if m, ok := early[p.Slot]; ok {
			for k, v := range engineValues(m) {
				values["e8_"+k] = v
			}
		}
		if e, ok := at8[int8(p.Slot)]; ok {
			values["e8_workers"] = float64(e.Workers)
			values["e8_supply"] = float64(e.SupplyUsed) / 2
			values["e8_gathered"] = float64(e.GatheredMinerals + e.GatheredGas)
		}
		s := sample{Match: j.row.MatchID, Duration: j.row.Duration, Side: j.sides[i], Engine: engineState, Values: values}
		switch p.PlayerOutcome() {
		case models.OutcomeWon:
			won := true
			s.Won = &won
		case models.OutcomeLost:
			won := false
			s.Won = &won
		}
		out = append(out, s)
	}
	return out
}

// resolve matches the harvest's two sides to the replay's two players: by name, else by race.
func resolve(humans []*library.Player, sides [2]side) [2]*library.Player {
	var out [2]*library.Player
	for i, s := range sides {
		for _, p := range humans {
			if s.Toon != "" && p.Name == s.Toon {
				out[i] = p
			}
		}
	}
	for i := range out {
		if out[i] == nil && out[1-i] != nil {
			for _, p := range humans {
				if p != out[1-i] {
					out[i] = p
				}
			}
		}
	}
	if out[0] == nil && out[1] == nil && sides[0].Race != sides[1].Race {
		for i, s := range sides {
			for _, p := range humans {
				if strings.EqualFold(p.Race.String()[:1], s.Race) {
					out[i] = p
				}
			}
		}
	}
	return out
}

func fastValues(p *library.Player, minutes float64) map[string]float64 {
	v := map[string]float64{
		"apm":  float64(p.APM),
		"eapm": float64(p.EAPM),
	}
	v["viewport_switches_per_min"] = float64(p.Viewport)
	if c := p.Cadence; c != nil {
		v["cadence_rate_per_min"] = c.RatePerMinute
		v["cadence_cv_gap"] = c.CVGap
		v["cadence_burstiness"] = c.Burstiness
		v["cadence_idle20"] = c.Idle20Ratio
		v["cadence_score"] = c.Score
	}
	if events, err := hotkeystream.Decode(p.HotkeyStream); err == nil && minutes > 0 {
		groups := map[byte]bool{}
		recalls, recallsLate, assigns := 0, 0, 0
		for _, e := range events {
			switch e.Type {
			case hotkeystream.TypeSelect:
				recalls++
				if e.Sec >= skill.MeasuredFromSecond {
					recallsLate++
				}
			case hotkeystream.TypeAssignUnits, hotkeystream.TypeAssignBuilding, hotkeystream.TypeAdd:
				assigns++
				groups[e.Group] = true
			}
		}
		v["hotkey_recalls_per_min"] = float64(recallsLate) / (minutes - skill.MeasuredFromSecond/60)
		v["hotkey_assigns_per_min"] = float64(assigns) / minutes
		v["hotkey_groups"] = float64(len(groups))
	}
	return v
}

func engineValues(m macro.Player) map[string]float64 {
	values := map[string]float64{
		"supply_blocked_share":  float64(m.SupplyBlockedSeconds) / math.Max(1, float64(m.Seconds)),
		"blocks_per_100_supply": m.BlocksPer100Supply,
		"spare_supply":          m.SpareSupply,
		"spending_quotient":     m.SpendingQuotient,
		"income_per_minute":     m.IncomePerMinute,
		"average_unspent":       m.AverageUnspent,
		"worker_idle_share":     m.WorkerIdleShare,
		"production_idle_share": m.ProductionIdleShare,
		"seconds_to_40_workers": m.SecondsTo40Workers,
		"float_share":           float64(m.FloatSeconds) / math.Max(1, float64(m.Seconds)),
	}
	values[skill.UnspentPerIncome] = m.UnspentPerIncome
	if m.Seconds >= 360 {
		values["workers_by_6min"] = float64(m.WorkersBy6Min)
	}
	for k, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			delete(values, k)
		}
	}
	return values
}

func armyUseValues(d *engine.Digest, slot int8) map[string]float64 {
	values := map[string]float64{}
	if idle := macro.IdleArmyShare(d, slot); !math.IsNaN(idle) {
		values[skill.IdleArmy] = idle
	}
	return values
}

// earlySeconds is the window before a game's outcome dominates what each player can do: a player who is
// losing at 15:00 has less income because they are losing, not because they macro worse.
const earlySeconds = 480

// firstSeconds is the digest cut at a game second, so the scorecard measures that opening window only.
func firstSeconds(d *engine.Digest, seconds int) *engine.Digest {
	cut := *d
	cut.LastFrame = min(d.LastFrame, int32(seconds*24-1))
	return &cut
}

func economyAt(d *engine.Digest, seconds int) map[int8]engine.EconomySample {
	at := map[int8]engine.EconomySample{}
	for _, e := range d.Economy {
		if int(e.Frame)/24 <= seconds {
			at[e.Owner] = e
		}
	}
	return at
}

// actionStats measures each player's actions from the command stream: the opening minutes, before a game's
// outcome changes what a player is doing, and the mix of what the actions are.
func actionStats(b []byte) map[uint8]map[string]float64 {
	r, err := repparser.ParseConfig(b, repparser.Config{Commands: true})
	if err != nil || r.Commands == nil {
		return nil
	}
	r.Compute()
	type counts struct{ all, effective, all6, effective6, hotkeys float64 }
	by := map[uint8]*counts{}
	window := repcore.Frame(6 * 60 * 1000 / 42)
	for _, c := range r.Commands.Cmds {
		base := c.BaseCmd()
		n := by[base.PlayerID]
		if n == nil {
			n = &counts{}
			by[base.PlayerID] = n
		}
		effective := base.IneffKind.Effective()
		n.all++
		if effective {
			n.effective++
		}
		if base.Frame < window {
			n.all6++
			if effective {
				n.effective6++
			}
		}
		if base.Type.ID == repcmd.TypeIDHotkey {
			n.hotkeys++
		}
	}
	out := map[uint8]map[string]float64{}
	for id, n := range by {
		if n.all == 0 {
			continue
		}
		out[id] = map[string]float64{
			"apm_first6":   n.all6 / 6,
			"eapm_first6":  n.effective6 / 6,
			"spam_share":   1 - n.effective/n.all,
			"hotkey_share": n.hotkeys / n.all,
		}
	}
	for id, c := range skill.CommandsOf(r.Commands.Cmds, int32(r.Header.Frames)) {
		if v, ok := out[id]; ok {
			v[skill.UsedRecalls], v[skill.CheckRecalls], v[skill.ArmyCadence], v[skill.MicroActions] = c.UsedRecalls, c.CheckRecalls, c.ArmyCadence, c.MicroActions
		}
	}
	return out
}

// macroWindowValues measures worker production between 2:00 and 10:00, when every player is building
// an economy: how busy Terran and Protoss town halls were making workers, and how much larva Zerg left
// unused per hatchery.
func macroWindowValues(d *engine.Digest, slot int8, until int) map[string]float64 {
	w, ok := macro.WorkerProductionOf(d, slot, until)
	if !ok {
		return nil
	}
	if w.Zerg {
		return map[string]float64{"larva_per_hatch": w.LarvaPerHatchery}
	}
	return map[string]float64{"hall_worker_busy": w.HallBusy}
}

// mineDir measures every human player of unlabelled games under dir, up to perKind games per map kind.
func mineDir(dir, out string, data *engine.Data, perKind, minDuration, workers int) {
	var paths []string
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".rep") {
			paths = append(paths, p)
		}
		return nil
	})
	rand.New(rand.NewSource(1)).Shuffle(len(paths), func(i, j int) { paths[i], paths[j] = paths[j], paths[i] })
	log.Printf("%d replays under %s", len(paths), dir)
	f, err := os.Create(out)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	var mu sync.Mutex
	taken := map[string]int{}
	ch := make(chan string)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range ch {
				b, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				replay := parser.CreateReplayFromFileInfo(p, filepath.Base(p), int64(len(b)), "")
				parsed, err := parser.ParseReplayWithOptions(p, replay, parser.Options{})
				if err != nil {
					continue
				}
				rec, err := compact.FromReplayData(parsed, compact.FileMeta{Path: p, Size: int64(len(b))})
				if err != nil || int(rec.Duration) < minDuration {
					continue
				}
				kind := ""
				switch rec.MapKind {
				case library.MapKindRegular:
					kind = "regular"
				case library.MapKindMoney:
					kind = "money"
				default:
					continue
				}
				var humans []*library.Player
				for i := range rec.Players {
					if pl := &rec.Players[i]; pl.IsHuman() && !pl.IsObserver() {
						humans = append(humans, pl)
					}
				}
				if len(humans) < 2 {
					continue
				}
				mu.Lock()
				if taken[kind] >= perKind {
					mu.Unlock()
					continue
				}
				taken[kind]++
				mu.Unlock()
				samples := measureUnlabelled(b, rec, humans, data)
				mu.Lock()
				for _, s := range samples {
					s.Match, s.MapKind, s.Humans = filepath.Base(p), kind, len(humans)
					line, _ := json.Marshal(s)
					w.Write(append(line, '\n'))
				}
				w.Flush()
				if (taken["money"]+taken["regular"])%50 == 0 {
					log.Printf("taken %v", taken)
				}
				mu.Unlock()
			}
		}()
	}
	for _, p := range paths {
		mu.Lock()
		full := taken["money"] >= perKind && taken["regular"] >= perKind
		mu.Unlock()
		if full {
			break
		}
		ch <- p
	}
	close(ch)
	wg.Wait()
	log.Printf("done: %v", taken)
}

func measureUnlabelled(b []byte, rec *library.Replay, humans []*library.Player, data *engine.Data) []sample {
	money := rec.MapKind == library.MapKindMoney
	banked := map[uint8]float64{}
	engineState := ""
	scorecards := map[uint8]macro.Player{}
	var windowed map[int8]map[string]float64
	var d *engine.Digest
	if dg, err := engine.Pass(context.Background(), b, data); err != nil {
		engineState = "error"
	} else if dg.Confidence() != engine.ConfidenceHigh {
		engineState = "low"
	} else {
		engineState, d = "high", dg
		for _, m := range macro.Compute(dg) {
			scorecards[uint8(m.Slot)] = m
		}
		windowed = windowedUnspent(dg)
		if money {
			for _, m := range macro.Compute(macro.FirstSeconds(dg, macro.MoneyMapBankedWindowEnd)) {
				banked[uint8(m.Slot)] = float64(m.FloatSeconds) / math.Max(1, float64(m.Seconds))
			}
		}
	}
	actions := actionStats(b)
	minutes := float64(rec.Duration) / 60
	var out []sample
	for _, p := range humans {
		values := fastValues(p, minutes)
		for k, v := range actions[p.ReplayPlayerID] {
			values[k] = v
		}
		if m, ok := scorecards[p.Slot]; ok {
			for k, v := range engineValues(m) {
				values[k] = v
			}
			for k, v := range candidateValues(d, int8(p.Slot), m.Seconds, p.Race == library.RaceZerg) {
				values[k] = v
			}
			for k, v := range armyUseValues(d, int8(p.Slot)) {
				values[k] = v
			}
			for k, v := range windowed[int8(p.Slot)] {
				values[k] = v
			}
			until := macro.WorkerWindowEnd
			if money {
				until = macro.MoneyMapWorkerWindowEnd
				if b, ok := banked[p.Slot]; ok {
					values["float_share"] = b
				}
			}
			for k, v := range macroWindowValues(d, int8(p.Slot), until) {
				values[k] = v
			}
		}
		out = append(out, sample{Duration: int(rec.Duration), Side: side{Toon: p.Name, Race: p.Race.String()[:1]}, Engine: engineState, Values: values})
	}
	return out
}
