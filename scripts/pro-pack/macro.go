package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/icza/screp/repparser"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/macro"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/propack"
	"github.com/marianogappa/screpdb/scripts/procorpus"
)

type macroOptions struct {
	harvestDir  string
	corpusDir   string
	outDir      string
	cachePath   string
	perPro      int
	workers     int
	minDuration int
	limit       int
}

// macroSample is one pro side's scorecard, cached per side so an interrupted run resumes.
type macroSample struct {
	Key    string             `json:"key"`
	Race   string             `json:"race,omitempty"`
	Skip   string             `json:"skip,omitempty"`
	Values map[string]float64 `json:"values,omitempty"`
}

type macroJob struct {
	matchID string
	path    string
	frames  uint32
	sides   []procorpus.ProSide
	players []procorpus.PlayerRow
}

var engineRaceNames = map[uint8]string{0: "zerg", 1: "terran", 2: "protoss"}

func sampleKey(s procorpus.ProSide) string { return s.MatchID + ":" + fmt.Sprint(s.AuroraID) }

// runMacroRanges runs the engine pass over the labelled pro sides of the harvest and writes the
// per-race percentiles of every macro scorecard metric to macro_ranges.json.
func runMacroRanges(o macroOptions) {
	registry, err := procorpus.LoadRegistry(o.corpusDir)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}
	sides, err := procorpus.LabelCorpus(o.harvestDir, registry, o.minDuration)
	if err != nil {
		log.Fatalf("label: %v", err)
	}
	picked := pickPerPro(sides, o.perPro)
	log.Printf("labelled %d pro sides, picked %d (at most %d per pro)", len(sides), len(picked), o.perPro)

	done := readMacroCache(o.cachePath)
	jobs := macroJobs(o.harvestDir, picked, done)
	if o.limit > 0 && len(jobs) > o.limit {
		jobs = jobs[:o.limit]
	}
	log.Printf("%d replays to run through the engine (%d sides cached)", len(jobs), len(done))

	if len(jobs) > 0 {
		data, err := openEngineData()
		if err != nil {
			log.Fatalf("game data: %v", err)
		}
		runMacroJobs(jobs, data, o, done)
	}

	ranges := buildMacroRanges(picked, done)
	data, err := json.MarshalIndent(ranges, "", " ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(o.outDir, "macro_ranges.json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	log.Printf("wrote %s: games per race %v", path, ranges.Games)
}

func openEngineData() (*engine.Data, error) {
	if dir := os.Getenv("OPENBW_DATA"); dir != "" {
		return engine.LoadData(dir)
	}
	return engine.OpenGameData()
}

// pickPerPro keeps each pro's most recent sides, so no prolific ladder grinder dominates a race.
func pickPerPro(sides []procorpus.ProSide, perPro int) []procorpus.ProSide {
	byPro := map[string][]procorpus.ProSide{}
	for _, s := range sides {
		byPro[s.ProName] = append(byPro[s.ProName], s)
	}
	var out []procorpus.ProSide
	for _, list := range byPro {
		sort.Slice(list, func(i, j int) bool { return list[i].Timestamp > list[j].Timestamp })
		out = append(out, list[:min(len(list), perPro)]...)
	}
	return out
}

func macroJobs(harvestDir string, sides []procorpus.ProSide, done map[string]macroSample) []macroJob {
	byMatch := map[string][]procorpus.ProSide{}
	for _, s := range sides {
		if _, ok := done[sampleKey(s)]; !ok {
			byMatch[s.MatchID] = append(byMatch[s.MatchID], s)
		}
	}
	var jobs []macroJob
	for matchID, matchSides := range byMatch {
		path := filepath.Join(harvestDir, "replays", matchID+".rep")
		replay, err := repparser.ParseFileConfig(path, repparser.Config{})
		if err != nil {
			for _, s := range matchSides {
				done[sampleKey(s)] = macroSample{Key: sampleKey(s), Skip: "unreadable"}
			}
			continue
		}
		var players []procorpus.PlayerRow
		for _, p := range replay.Header.Players {
			if p.Observer || p.Type.Name != "Human" {
				continue
			}
			players = append(players, procorpus.PlayerRow{ID: int64(p.ID), Name: p.Name, Race: p.Race.Name})
		}
		jobs = append(jobs, macroJob{matchID: matchID, path: path, frames: uint32(replay.Header.Frames), sides: matchSides, players: players})
	}
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].frames > jobs[j].frames })
	return jobs
}

func runMacroJobs(jobs []macroJob, data *engine.Data, o macroOptions, done map[string]macroSample) {
	f, err := os.OpenFile(o.cachePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Fatalf("cache: %v", err)
	}
	defer f.Close()
	var mu sync.Mutex
	record := func(samples []macroSample) {
		mu.Lock()
		defer mu.Unlock()
		for _, s := range samples {
			done[s.Key] = s
			line, _ := json.Marshal(s)
			f.Write(append(line, '\n'))
		}
	}
	queue := make(chan macroJob)
	var wg sync.WaitGroup
	var finished int
	start := time.Now()
	for range o.workers {
		wg.Go(func() {
			for job := range queue {
				samples := scoreJob(job, data)
				record(samples)
				mu.Lock()
				finished++
				if finished%25 == 0 || finished == len(jobs) {
					log.Printf("engine pass %d/%d (%s)", finished, len(jobs), time.Since(start).Round(time.Second))
				}
				mu.Unlock()
			}
		})
	}
	for _, job := range jobs {
		queue <- job
	}
	close(queue)
	wg.Wait()
}

func scoreJob(job macroJob, data *engine.Data) []macroSample {
	skipAll := func(reason string) []macroSample {
		out := make([]macroSample, len(job.sides))
		for i, s := range job.sides {
			out[i] = macroSample{Key: sampleKey(s), Skip: reason}
		}
		return out
	}
	if len(job.players) != 2 {
		return skipAll("not 1v1")
	}
	raw, err := os.ReadFile(job.path)
	if err != nil {
		return skipAll("unreadable")
	}
	digest, err := passGuarded(raw, data)
	if err != nil {
		return skipAll("engine: " + err.Error())
	}
	if digest.Confidence() != engine.ConfidenceHigh {
		return skipAll("low confidence")
	}
	scored := macro.Compute(digest)
	out := make([]macroSample, 0, len(job.sides))
	for _, s := range job.sides {
		sample := macroSample{Key: sampleKey(s), Skip: "unresolved"}
		if p, method := procorpus.ResolvePlayer(job.players, s); method != "" {
			sample.Skip = "too short"
			for _, dp := range digest.Players {
				if int64(dp.PlayerID) != p.ID {
					continue
				}
				for _, m := range scored {
					if m.Slot == dp.Slot {
						sample = macroSample{Key: sampleKey(s), Race: engineRaceNames[m.Race], Values: macroValues(m)}
					}
				}
			}
		}
		out = append(out, sample)
	}
	return out
}

func passGuarded(raw []byte, data *engine.Data) (d *engine.Digest, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return engine.Pass(context.Background(), raw, data)
}

// macroValues names each metric as the game detail's macro block does, and drops a metric the game
// gives no basis for, the same way the dashboard shows it as missing.
func macroValues(m macro.Player) map[string]float64 {
	values := map[string]float64{
		"supply_blocked_seconds": float64(m.SupplyBlockedSeconds),
		"supply_blocks":          float64(m.SupplyBlocks),
		"blocks_per_100_supply":  m.BlocksPer100Supply,
		"spare_supply":           m.SpareSupply,
		"spending_quotient":      m.SpendingQuotient,
		"income_per_minute":      m.IncomePerMinute,
		"average_unspent":        m.AverageUnspent,
		"worker_idle_share":      m.WorkerIdleShare,
		"production_idle_share":  m.ProductionIdleShare,
		"seconds_to_40_workers":  m.SecondsTo40Workers,
		"float_seconds":          float64(m.FloatSeconds),
	}
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

func readMacroCache(path string) map[string]macroSample {
	done := map[string]macroSample{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return done
	}
	if err != nil {
		log.Fatalf("cache: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<16), 1<<20)
	for sc.Scan() {
		var s macroSample
		if json.Unmarshal(sc.Bytes(), &s) == nil && s.Key != "" {
			done[s.Key] = s
		}
	}
	return done
}

func buildMacroRanges(picked []procorpus.ProSide, done map[string]macroSample) propack.MacroRanges {
	valuesByRace := map[string]map[string][]float64{}
	games := map[string]int{}
	skips := map[string]int{}
	for _, s := range picked {
		sample, ok := done[sampleKey(s)]
		if !ok {
			continue
		}
		if sample.Skip != "" {
			skips[strings.SplitN(sample.Skip, ":", 2)[0]]++
			continue
		}
		games[sample.Race]++
		if valuesByRace[sample.Race] == nil {
			valuesByRace[sample.Race] = map[string][]float64{}
		}
		for k, v := range sample.Values {
			valuesByRace[sample.Race][k] = append(valuesByRace[sample.Race][k], v)
		}
	}
	log.Printf("skipped sides: %v", skips)
	out := propack.MacroRanges{
		GeneratedAt:     time.Now().UTC().Format(time.RFC3339),
		EngineVersion:   engine.EngineVersion,
		DetectorVersion: core.DetectorVersion,
		Source:          "engine passes over public 1v1 ladder replays of labelled pros (scfingerprint corpus), high confidence only",
		Games:           games,
		Races:           map[string]map[string]propack.MacroRange{},
	}
	for race, metrics := range valuesByRace {
		out.Races[race] = map[string]propack.MacroRange{}
		for k, values := range metrics {
			slices.Sort(values)
			out.Races[race][k] = propack.MacroRange{
				P10: percentile(values, 0.10), P25: percentile(values, 0.25), P50: percentile(values, 0.50),
				P75: percentile(values, 0.75), P90: percentile(values, 0.90), Games: len(values),
			}
		}
	}
	return out
}

func percentile(sorted []float64, q float64) float64 {
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := min(lo+1, len(sorted)-1)
	v := sorted[lo] + (sorted[hi]-sorted[lo])*(pos-float64(lo))
	return math.Round(v*1000) / 1000
}

func defaultMacroWorkers() int { return max(1, runtime.NumCPU()/2) }
