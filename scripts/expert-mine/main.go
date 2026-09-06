// expert-mine re-derives the expert golden-line timings in
// internal/patterns/markers/definitions.go from the aurora-ID-labelled
// progamer corpus. It is the committed, reproducible form of the procedure in
// internal/patterns/markers/MEASUREMENT.md — read that first.
//
// Pipeline:
//
//  1. label:   read <harvest>/replays.jsonl and the pro JSONs in <corpus>;
//     a player-game is pro iff auroraId != 0, the id is enrolled in
//     pros_merged.json and not in pro_exclusions.json, and the game
//     lasted >= 240s. Never trust the harvest's proName field.
//  2. stage:   copy the selected .rep files flat into <workdir>/staged
//     (skippable once the folder exists).
//  3. load:    read <workdir>/staged into the in-memory replay library with
//     the production loader — the same parse, filters and pattern
//     detection the dashboard runs.
//  4. join:    resolve each pro label to a (replay, player) of the loaded
//     corpus by toon, else opponent-toon elimination, else unique
//     race. Drop the rest; never guess.
//  5. measure: read payload.expert_actuals for every bo_% marker of a
//     resolved pro player (the same resolution path the Build Orders
//     tab scores) and emit per-milestone n/p10/p50/p90 + the in-band%
//     of the CURRENT definitions, plus proposed target/tolerance.
//     The fuzzy Zerg opener (bo_z_fuzzy) has no expert_actuals; its
//     per-label pool/hatch seconds are read from the production
//     stream with the same filters as LoadEarlyZergTimings — what the
//     dashboard renders those rows from.
//
// Outputs under <workdir>/out:
//
//	milestones.tsv  per (feature_key, milestone): n, percentiles, in-band%,
//	                current + proposed target/tolerance
//	actuals.tsv     raw per-game milestone seconds (for pooled / split probes)
//	fuzzy.tsv       raw per-game fuzzy-opener label + pool/hatch/overlord secs
//	phase2.tsv      non-BO constants: first-upgrade / first-tech p5 floors per
//	                matchup, muta-vs-turret completion gap percentiles
//	meta.json       DetectorVersion, corpus hash, join tallies
//
// Usage:
//
//	go run ./scripts/expert-mine \
//	  -harvest ~/Code/go/src/github.com/marianogappa/screpharvest/harvest \
//	  -corpus  ~/Code/go/src/github.com/marianogappa/scfingerprint/corpus \
//	  -workdir /tmp/expert-mine
//
// Re-run with -stage=false to re-measure the already-staged folder (e.g. after
// editing definitions.go, to recompute in-band% against the new bands). The
// corpus is re-read on every run; there is no scratch database any more.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/iofacade"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/library/load"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/scripts/procorpus"
)

// earlyZergWindowSeconds bounds the fuzzy-opener scan, mirroring the window
// the dashboard's LoadEarlyZergTimings uses.
const earlyZergWindowSeconds = 600

// corpus is the loaded replay library plus the player index the measurement
// steps address it by. A player id is library.PlayerID(replayID, ordinal),
// the same identity the dashboard's API exposes.
type corpus struct {
	snapshot *library.Snapshot
	byPlayer map[int64]playerRef
}

type playerRef struct {
	replay  *library.Replay
	ordinal uint8
}

func main() {
	log.SetFlags(log.LstdFlags)
	harvestDir := flag.String("harvest", "", "screpharvest harvest dir (holds replays.jsonl + replays/)")
	corpusDir := flag.String("corpus", "", "scfingerprint corpus dir (holds pros_merged.json + pro_exclusions.json)")
	workdir := flag.String("workdir", "", "scratch dir for staged reps and outputs")
	doStage := flag.Bool("stage", true, "copy pro .rep files into <workdir>/staged")
	minDuration := flag.Int("min-duration", 240, "minimum game duration in seconds")
	minN := flag.Int("min-n", 20, "sample floor below which no value should be baked")
	flag.Parse()
	if *harvestDir == "" || *corpusDir == "" || *workdir == "" {
		flag.Usage()
		os.Exit(2)
	}

	registry, err := procorpus.LoadRegistry(*corpusDir)
	if err != nil {
		log.Fatalf("registry: %v", err)
	}
	sides, err := procorpus.LabelCorpus(*harvestDir, registry, *minDuration)
	if err != nil {
		log.Fatalf("label: %v", err)
	}
	byMatch := procorpus.GroupByMatch(sides)
	log.Printf("labelled %d pro player-games across %d matches", len(sides), len(byMatch))

	stagedDir := filepath.Join(*workdir, "staged")
	outDir := filepath.Join(*workdir, "out")
	for _, d := range []string{stagedDir, outDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", d, err)
		}
	}

	if *doStage {
		staged, missing := procorpus.Stage(*harvestDir, stagedDir, byMatch)
		log.Printf("staged %d replay files (%d not on disk)", staged, missing)
	}

	c, closeCorpus, err := loadCorpus(stagedDir)
	if err != nil {
		log.Fatalf("load: %v", err)
	}
	defer closeCorpus()

	joined, tallies := procorpus.Join(c.replays1v1(), byMatch)
	log.Printf("joined %d/%d player-games (%s)", len(joined), len(sides), tallies)

	if err := measureMilestones(c, joined, outDir, *minN); err != nil {
		log.Fatalf("measure: %v", err)
	}
	if err := measureFuzzy(c, joined, outDir); err != nil {
		log.Fatalf("fuzzy: %v", err)
	}
	if err := measurePhase2(c, joined, outDir); err != nil {
		log.Fatalf("phase2: %v", err)
	}
	if err := writeMeta(outDir, sides, tallies); err != nil {
		log.Fatalf("meta: %v", err)
	}
	log.Printf("outputs in %s", outDir)
}

// loadCorpus reads the staged folder with the production loader, so what is
// measured here is exactly what the dashboard would show for the same files.
func loadCorpus(stagedDir string) (*corpus, func(), error) {
	// appdata.Dir registers the app-data root the map cache lives under; the
	// staged folder has to be permitted explicitly, as the dashboard's loader
	// manager does for the replay folder.
	if _, err := appdata.Dir(); err != nil {
		return nil, nil, fmt.Errorf("appdata: %w", err)
	}
	if err := iofacade.AllowDir(stagedDir); err != nil {
		return nil, nil, fmt.Errorf("allow %s: %w", stagedDir, err)
	}

	lib := library.New(library.Options{})
	// MaxReplays -1 reads the whole folder; the loader's default cap keeps
	// only the newest few hundred, which would silently shrink the corpus.
	loader := load.New(lib, load.Options{
		Folder:     stagedDir,
		Generation: 1,
		MaxReplays: -1,
		Log:        func(e load.LogEvent) { log.Printf("[load] %s", e.Message) },
	})
	start := time.Now()
	if err := loader.Run(context.Background()); err != nil {
		lib.Close()
		return nil, nil, err
	}
	snapshot := lib.Snapshot()
	log.Printf("loaded %d replays in %s", snapshot.Len(), time.Since(start).Round(time.Second))

	c := &corpus{snapshot: snapshot, byPlayer: map[int64]playerRef{}}
	for _, r := range snapshot.Replays {
		for i := range r.Players {
			c.byPlayer[r.PlayerID(uint8(i))] = playerRef{replay: r, ordinal: uint8(i)}
		}
	}
	return c, func() { lib.Close() }, nil
}

// replays1v1 returns the analysed 1v1 replays the labelled sides are joined
// against, with their human non-observer players.
func (c *corpus) replays1v1() []procorpus.Replay1v1 {
	out := make([]procorpus.Replay1v1, 0, c.snapshot.Len())
	for _, r := range c.snapshot.Replays {
		if library.Strings.Name(r.TeamFormat) != "1v1" {
			continue
		}
		replay := procorpus.Replay1v1{
			ID:       r.ID,
			FileName: r.FileName(),
			Matchup:  library.Strings.Name(r.Matchup),
		}
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() || !p.IsHuman() {
				continue
			}
			replay.Players = append(replay.Players, procorpus.PlayerRow{
				ID:   r.PlayerID(uint8(i)),
				Name: p.Name,
				Race: p.Race.String(),
			})
		}
		out = append(out, replay)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// markerRow is one marker of a resolved pro player.
type markerRow struct {
	jp      procorpus.JoinedPlayer
	feature string
	payload []byte
}

// loadMarkerRows collects the markers of every resolved pro player whose
// feature key has the given prefix (a bare prefix, not a SQL LIKE pattern).
func (c *corpus) loadMarkerRows(joined []procorpus.JoinedPlayer, prefix string) []markerRow {
	var out []markerRow
	for _, jp := range joined {
		ref, ok := c.byPlayer[jp.PlayerID]
		if !ok {
			continue
		}
		for i := range ref.replay.Markers {
			m := &ref.replay.Markers[i]
			if m.Player != ref.ordinal {
				continue
			}
			feature := library.Features.Name(m.Feature)
			if !strings.HasPrefix(feature, prefix) {
				continue
			}
			out = append(out, markerRow{jp: jp, feature: feature, payload: m.Payload})
		}
	}
	return out
}

func measureMilestones(c *corpus, joined []procorpus.JoinedPlayer, outDir string, minN int) error {
	rows := c.loadMarkerRows(joined, "bo_")

	type slot struct {
		feature string
		idx     int
	}
	secs := map[slot][]int{}
	actualsF, err := os.Create(filepath.Join(outDir, "actuals.tsv"))
	if err != nil {
		return err
	}
	defer actualsF.Close()
	aw := bufio.NewWriter(actualsF)
	fmt.Fprintln(aw, "feature_key\tidx\tkey\tsecond\tfile\tplayer\tmatchup")
	for _, r := range rows {
		m := markers.ByFeatureKey(r.feature)
		if m == nil || len(m.Expert) == 0 {
			continue
		}
		actuals := markers.DecodeExpertActuals(r.payload)
		for i, ev := range m.Expert {
			if i >= len(actuals) || !actuals[i].Found {
				continue
			}
			s := slot{feature: r.feature, idx: i}
			secs[s] = append(secs[s], actuals[i].Second)
			fmt.Fprintf(aw, "%s\t%d\t%s\t%d\t%s\t%s\t%s\n",
				r.feature, i, ev.Key, actuals[i].Second, r.jp.FileName, r.jp.Name, r.jp.Matchup)
		}
	}
	if err := aw.Flush(); err != nil {
		return err
	}

	milestonesF, err := os.Create(filepath.Join(outDir, "milestones.tsv"))
	if err != nil {
		return err
	}
	defer milestonesF.Close()
	mw := bufio.NewWriter(milestonesF)
	fmt.Fprintln(mw, "feature_key\tidx\tkey\tn\tp10\tp50\tp90\tcur_target\tcur_early\tcur_late\tin_band_pct\tproposed_target\tproposed_early\tproposed_late\tbakeable")
	for _, m := range markers.Markers() {
		for i, ev := range m.Expert {
			s := slot{feature: m.FeatureKey, idx: i}
			vals := secs[s]
			sort.Ints(vals)
			n := len(vals)
			var p10, p50, p90, inBand int
			if n > 0 {
				p10, p50, p90 = percentile(vals, 0.10), percentile(vals, 0.50), percentile(vals, 0.90)
				for _, v := range vals {
					d := v - ev.TargetSecond
					if d >= -ev.Tolerance.EarlySeconds && d <= ev.Tolerance.LateSeconds {
						inBand++
					}
				}
			}
			inBandPct := 0.0
			if n > 0 {
				inBandPct = 100 * float64(inBand) / float64(n)
			}
			propEarly, propLate := max(2, p50-p10), max(2, p90-p50)
			fmt.Fprintf(mw, "%s\t%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%.0f\t%d\t%d\t%d\t%v\n",
				m.FeatureKey, i, ev.Key, n, p10, p50, p90,
				ev.TargetSecond, ev.Tolerance.EarlySeconds, ev.Tolerance.LateSeconds,
				inBandPct, p50, propEarly, propLate, n >= minN)
		}
	}
	return mw.Flush()
}

// measureFuzzy reads each resolved pro player's bo_z_fuzzy label plus their
// first Spawning Pool / Hatchery / Overlord seconds from the production
// stream — the same source and filters as LoadEarlyZergTimings, which is what
// the dashboard renders for the simplified Zerg BO rows.
func measureFuzzy(c *corpus, joined []procorpus.JoinedPlayer, outDir string) error {
	rows := c.loadMarkerRows(joined, "bo_z_fuzzy")
	f, err := os.Create(filepath.Join(outDir, "fuzzy.tsv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "label\tpool_sec\thatch_sec\toverlord_sec\tfile\tplayer\tmatchup")
	for _, r := range rows {
		label, ok := markers.DecodePayloadLabel(r.payload)
		if !ok {
			continue
		}
		firsts := c.earlyZergFirsts(r.jp.PlayerID)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", label,
			tsvInt(firsts, "Spawning Pool"), tsvInt(firsts, "Hatchery"), tsvInt(firsts, "Overlord"),
			r.jp.FileName, r.jp.Name, r.jp.Matchup)
	}
	return w.Flush()
}

// earlyZergFirsts returns the first second this player built a Spawning Pool
// or Hatchery, and morphed an Overlord, inside the early-game window.
func (c *corpus) earlyZergFirsts(playerID int64) map[string]int {
	firsts := map[string]int{}
	ref, ok := c.byPlayer[playerID]
	if !ok {
		return firsts
	}
	prod := &ref.replay.Prod
	for i := 0; i < prod.Len(); i++ {
		if prod.Player[i] != ref.ordinal || int(prod.Sec[i]) >= earlyZergWindowSeconds {
			continue
		}
		name := prod.SubjectName(i)
		switch {
		case prod.Kind[i] == library.ProdBuild && (name == "Spawning Pool" || name == "Hatchery"):
		case prod.Kind[i] == library.ProdUnitMorph && name == "Overlord":
		default:
			continue
		}
		if cur, seen := firsts[name]; !seen || int(prod.Sec[i]) < cur {
			firsts[name] = int(prod.Sec[i])
		}
	}
	return firsts
}

func tsvInt(m map[string]int, k string) string {
	if v, ok := m[k]; ok {
		return fmt.Sprint(v)
	}
	return ""
}

// measurePhase2 re-derives the non-BO corpus constants: the never_upgraded /
// never_researched per-matchup p5 floors (first HP-upgrade / first tech-or-
// non-HP-upgrade command second) and the muta-vs-turret completion-gap
// percentiles (prerequisite-clamped, mirroring the dashboard's computation).
func measurePhase2(c *corpus, joined []procorpus.JoinedPlayer, outDir string) error {
	f, err := os.Create(filepath.Join(outDir, "phase2.tsv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	oppRace := map[int64]map[int64]string{} // replayID -> playerID -> opponent race
	firstHPUp := map[int64]int{}            // playerID -> second
	firstResearch := map[int64]int{}        // playerID -> second (tech or non-HP upgrade)
	for _, r := range c.snapshot.Replays {
		if library.Strings.Name(r.TeamFormat) != "1v1" {
			continue
		}
		type pr struct {
			id   int64
			race string
		}
		var ps []pr
		for i := range r.Players {
			p := &r.Players[i]
			if p.IsObserver() || !p.IsHuman() {
				continue
			}
			ps = append(ps, pr{id: r.PlayerID(uint8(i)), race: p.Race.String()})
		}
		if len(ps) == 2 {
			oppRace[r.ID] = map[int64]string{ps[0].id: ps[1].race, ps[1].id: ps[0].race}
		}
		collectFirstResearch(r, firstHPUp, firstResearch)
	}

	upSecs := map[string][]int{}
	techSecs := map[string][]int{}
	for _, jp := range joined {
		opp, ok := oppRace[jp.ReplayID][jp.PlayerID]
		if !ok {
			continue
		}
		key := jp.Race + "v" + opp
		if s, ok := firstHPUp[jp.PlayerID]; ok {
			upSecs[key] = append(upSecs[key], s)
		}
		if s, ok := firstResearch[jp.PlayerID]; ok {
			techSecs[key] = append(techSecs[key], s)
		}
	}
	fmt.Fprintln(w, "metric\tbucket\tn\tp5\tp50")
	for name, m := range map[string]map[string][]int{"first_hp_upgrade": upSecs, "first_research": techSecs} {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vals := m[k]
			sort.Ints(vals)
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n", name, k, len(vals), percentile(vals, 0.05), percentile(vals, 0.50))
		}
	}

	gaps := mutaTurretGaps(c)
	sort.Ints(gaps)
	fmt.Fprintf(w, "muta_turret_gap\tTvZ\t%d\t%d\t%d\n", len(gaps), percentile(gaps, 0.05), percentile(gaps, 0.50))
	fmt.Fprintf(w, "muta_turret_gap_p25_p75\tTvZ\t%d\t%d\t%d\n", len(gaps), percentile(gaps, 0.25), percentile(gaps, 0.75))
	return w.Flush()
}

// collectFirstResearch records, per player of one replay, the first HP-upgrade
// second and the first tech-or-non-HP-upgrade second.
func collectFirstResearch(r *library.Replay, firstHPUp, firstResearch map[int64]int) {
	prod := &r.Prod
	for i := 0; i < prod.Len(); i++ {
		kind := prod.Kind[i]
		if kind != library.ProdUpgrade && kind != library.ProdTech {
			continue
		}
		playerID := r.PlayerID(prod.Player[i])
		sec := int(prod.Sec[i])
		target := firstResearch
		if kind == library.ProdUpgrade && models.IsHPUpgrade(prod.SubjectName(i)) {
			target = firstHPUp
		}
		if cur, ok := target[playerID]; !ok || sec < cur {
			target[playerID] = sec
		}
	}
}

// mutaTurretGaps pairs the mutalisk_timing / turret_timing payloads per replay
// and computes turret_finish - muta_finish with the same prerequisite clamping
// as populateMutaliskTimingForGameDetail.
func mutaTurretGaps(c *corpus) []int {
	type side struct {
		spireCmd, firstMutaCmd, ebayCmd, firstTurretCmd int
		hasZ, hasT                                      bool
	}
	byReplay := map[int64]*side{}
	for _, r := range c.snapshot.Replays {
		for i := range r.Markers {
			m := &r.Markers[i]
			typ := library.Features.Name(m.Feature)
			if typ != "mutalisk_timing" && typ != "turret_timing" {
				continue
			}
			var raw map[string]float64
			if err := json.Unmarshal(m.Payload, &raw); err != nil {
				continue
			}
			s := byReplay[r.ID]
			if s == nil {
				s = &side{}
				byReplay[r.ID] = s
			}
			switch typ {
			case "mutalisk_timing":
				s.hasZ = true
				s.spireCmd = int(raw["spire_cmd"])
				s.firstMutaCmd = int(raw["first_muta_cmd"])
			case "turret_timing":
				s.hasT = true
				s.ebayCmd = int(raw["ebay_cmd"])
				s.firstTurretCmd = int(raw["first_turret_cmd"])
			}
		}
	}
	var gaps []int
	for _, s := range byReplay {
		if !s.hasZ || !s.hasT || s.firstMutaCmd <= 0 || s.firstTurretCmd <= 0 {
			continue
		}
		mutaStart := s.firstMutaCmd
		if spireFinish := s.spireCmd + int(models.BuildTimeSpire); s.spireCmd > 0 && mutaStart < spireFinish {
			mutaStart = spireFinish
		}
		mutaFinish := mutaStart + int(models.BuildTimeMutalisk)
		turretStart := s.firstTurretCmd
		if ebayFinish := s.ebayCmd + int(models.BuildTimeEngineeringBay); s.ebayCmd > 0 && turretStart < ebayFinish {
			turretStart = ebayFinish
		}
		turretFinish := turretStart + int(math.Round(models.BuildTimeMissileTurret))
		gaps = append(gaps, turretFinish-mutaFinish)
	}
	return gaps
}

func writeMeta(outDir string, sides []procorpus.ProSide, tallies procorpus.JoinTallies) error {
	keys := make([]string, 0, len(sides))
	for _, s := range sides {
		keys = append(keys, fmt.Sprintf("%s:%d", s.MatchID, s.AuroraID))
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintln(h, k)
	}
	meta := map[string]any{
		"detector_version": core.DetectorVersion,
		"corpus_hash":      fmt.Sprintf("%x", h.Sum(nil)),
		"pro_player_games": len(sides),
		"join":             tallies,
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "meta.json"), append(data, '\n'), 0o644)
}

// percentile returns the nearest-rank percentile of a sorted slice.
func percentile(sorted []int, q float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Round(q * float64(len(sorted)-1)))
	return sorted[idx]
}
