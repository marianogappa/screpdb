// expert-mine re-derives the expert golden-line timings in
// internal/patterns/markers/definitions.go from the aurora-ID-labelled
// progamer corpus. It is the committed, reproducible form of the procedure in
// internal/patterns/markers/MEASUREMENT.md — read that first.
//
// Pipeline (each step skippable once its artifact exists):
//
//  1. label:   read <harvest>/replays.jsonl and the pro JSONs in <corpus>;
//     a player-game is pro iff auroraId != 0, the id is enrolled in
//     pros_merged.json and not in pro_exclusions.json, and the game
//     lasted >= 240s. Never trust the harvest's proName field.
//  2. stage:   copy the selected .rep files flat into <workdir>/staged.
//  3. ingest:  screpdb-ingest the staged files into <workdir>/pro_corpus.db.
//  4. join:    resolve each pro label to a (replay, player) row by toon,
//     else opponent-toon elimination, else unique race. Drop the
//     rest; never guess.
//  5. measure: read payload.expert_actuals for every bo_% marker row of a
//     resolved pro player (the same resolution path the Build Orders
//     tab scores) and emit per-milestone n/p10/p50/p90 + the in-band%
//     of the CURRENT definitions, plus proposed target/tolerance.
//     The fuzzy Zerg opener (bo_z_fuzzy) has no expert_actuals; its
//     per-label pool/hatch seconds are read from the commands table
//     with the same filters as ListEarlyZergMorphsForBOTimings — the
//     query the dashboard renders those rows from.
//
// Outputs under <workdir>/out:
//
//	milestones.tsv  per (feature_key, milestone): n, percentiles, in-band%,
//	                current + proposed target/tolerance
//	actuals.tsv     raw per-game milestone seconds (for pooled / split probes)
//	fuzzy.tsv       raw per-game fuzzy-opener label + pool/hatch/overlord secs
//	phase2.tsv      non-BO constants: first-upgrade / first-tech p5 floors per
//	                matchup, muta-vs-turret completion gap percentiles
//	meta.json       AlgorithmVersion, corpus hash, join tallies
//
// Usage:
//
//	go run ./scripts/expert-mine \
//	  -harvest ~/Code/go/src/github.com/marianogappa/screpharvest/harvest \
//	  -corpus  ~/Code/go/src/github.com/marianogappa/scfingerprint/corpus \
//	  -workdir /tmp/expert-mine
//
// Re-run with -stage=false -ingest=false to re-measure an existing scratch DB
// (e.g. after editing definitions.go, to recompute in-band% against the new
// bands).
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/marianogappa/screpdb/internal/appdata"
	"github.com/marianogappa/screpdb/internal/ingest"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/core"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	_ "modernc.org/sqlite"
)

type harvestRow struct {
	MatchID     string `json:"matchId"`
	Duration    int    `json:"duration"`
	AuroraID    int64  `json:"auroraId"`
	Toon        string `json:"toon"`
	Race        string `json:"race"`
	MMR         int    `json:"mmr"`
	OppAuroraID int64  `json:"oppAuroraId"`
	OppToon     string `json:"oppToon"`
	OppRace     string `json:"oppRace"`
	OppMMR      int    `json:"oppMmr"`
}

// proSide is one labelled progamer player-game before the DB join.
type proSide struct {
	matchID  string
	auroraID int64
	toon     string // may be empty (~32% of harvest rows)
	oppToon  string
	race     string // full race name ("Zerg")
	mmr      int
}

func main() {
	log.SetFlags(log.LstdFlags)
	harvestDir := flag.String("harvest", "", "screpharvest harvest dir (holds replays.jsonl + replays/)")
	corpusDir := flag.String("corpus", "", "scfingerprint corpus dir (holds pros_merged.json + pro_exclusions.json)")
	workdir := flag.String("workdir", "", "scratch dir for staged reps, DB and outputs")
	doStage := flag.Bool("stage", true, "copy pro .rep files into <workdir>/staged")
	doIngest := flag.Bool("ingest", true, "ingest staged files into <workdir>/pro_corpus.db")
	minDuration := flag.Int("min-duration", 240, "minimum game duration in seconds")
	minN := flag.Int("min-n", 20, "sample floor below which no value should be baked")
	flag.Parse()
	if *harvestDir == "" || *corpusDir == "" || *workdir == "" {
		flag.Usage()
		os.Exit(2)
	}

	sides, err := labelCorpus(*harvestDir, *corpusDir, *minDuration)
	if err != nil {
		log.Fatalf("label: %v", err)
	}
	byMatch := map[string][]proSide{}
	for _, s := range sides {
		byMatch[s.matchID] = append(byMatch[s.matchID], s)
	}
	log.Printf("labelled %d pro player-games across %d matches", len(sides), len(byMatch))

	stagedDir := filepath.Join(*workdir, "staged")
	dbPath := filepath.Join(*workdir, "pro_corpus.db")
	outDir := filepath.Join(*workdir, "out")
	for _, d := range []string{stagedDir, outDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", d, err)
		}
	}

	if *doStage {
		staged, missing := stage(*harvestDir, stagedDir, byMatch)
		log.Printf("staged %d replay files (%d not on disk)", staged, missing)
	}

	if *doIngest {
		// appdata.Dir registers the app-data root the ingest pipeline's map
		// cache lives under; ingest.Run registers the staged dir itself.
		if _, err := appdata.Dir(); err != nil {
			log.Fatalf("appdata: %v", err)
		}
		start := time.Now()
		if err := ingest.Run(context.Background(), ingest.Config{
			InputDir:   stagedDir,
			SQLitePath: dbPath,
			UseColor:   true,
		}); err != nil {
			log.Fatalf("ingest: %v", err)
		}
		log.Printf("ingest done in %s", time.Since(start).Round(time.Second))
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	joined, tallies, err := join(db, byMatch)
	if err != nil {
		log.Fatalf("join: %v", err)
	}
	log.Printf("joined %d/%d player-games (%s)", len(joined), len(sides), tallies)

	if err := measureMilestones(db, joined, outDir, *minN); err != nil {
		log.Fatalf("measure: %v", err)
	}
	if err := measureFuzzy(db, joined, outDir); err != nil {
		log.Fatalf("fuzzy: %v", err)
	}
	if err := measurePhase2(db, joined, outDir); err != nil {
		log.Fatalf("phase2: %v", err)
	}
	if err := writeMeta(outDir, sides, tallies); err != nil {
		log.Fatalf("meta: %v", err)
	}
	log.Printf("outputs in %s", outDir)
}

var raceByLetter = map[string]string{"Z": "Zerg", "T": "Terran", "P": "Protoss"}

func labelCorpus(harvestDir, corpusDir string, minDuration int) ([]proSide, error) {
	allowed, err := loadProAuroraIDs(corpusDir)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(harvestDir, "replays.jsonl"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	seen := map[string]bool{} // matchID:auroraID
	var sides []proSide
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		var r harvestRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, fmt.Errorf("bad jsonl row: %w", err)
		}
		if r.Duration < minDuration {
			continue
		}
		// Both perspectives of a row can be a pro (pro-vs-pro games).
		for _, cand := range []proSide{
			{matchID: r.MatchID, auroraID: r.AuroraID, toon: r.Toon, oppToon: r.OppToon, race: raceByLetter[r.Race], mmr: r.MMR},
			{matchID: r.MatchID, auroraID: r.OppAuroraID, toon: r.OppToon, oppToon: r.Toon, race: raceByLetter[r.OppRace], mmr: r.OppMMR},
		} {
			if cand.auroraID == 0 || !allowed[cand.auroraID] || cand.race == "" {
				continue
			}
			key := cand.matchID + ":" + fmt.Sprint(cand.auroraID)
			if seen[key] {
				continue
			}
			seen[key] = true
			sides = append(sides, cand)
		}
	}
	return sides, sc.Err()
}

func loadProAuroraIDs(corpusDir string) (map[int64]bool, error) {
	var merged map[string][]int64
	if err := readJSON(filepath.Join(corpusDir, "pros_merged.json"), &merged); err != nil {
		return nil, err
	}
	var exclusions map[string]json.RawMessage
	if err := readJSON(filepath.Join(corpusDir, "pro_exclusions.json"), &exclusions); err != nil {
		return nil, err
	}
	excluded := map[int64]bool{}
	for name, raw := range exclusions {
		if name == "_comment" {
			continue
		}
		var byID map[string]string
		if err := json.Unmarshal(raw, &byID); err != nil {
			return nil, fmt.Errorf("pro_exclusions[%s]: %w", name, err)
		}
		for idStr := range byID {
			var id int64
			fmt.Sscan(idStr, &id)
			excluded[id] = true
		}
	}
	allowed := map[int64]bool{}
	for _, ids := range merged {
		for _, id := range ids {
			if !excluded[id] {
				allowed[id] = true
			}
		}
	}
	return allowed, nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func stage(harvestDir, stagedDir string, byMatch map[string][]proSide) (staged, missing int) {
	for matchID := range byMatch {
		src := filepath.Join(harvestDir, "replays", matchID+".rep")
		dst := filepath.Join(stagedDir, matchID+".rep")
		if _, err := os.Stat(dst); err == nil {
			staged++
			continue
		}
		if err := copyFile(src, dst); err != nil {
			missing++
			continue
		}
		staged++
	}
	return staged, missing
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// joinedPlayer is one resolved (replay, player) pro row.
type joinedPlayer struct {
	replayID int64
	playerID int64
	fileName string
	matchup  string
	name     string
	race     string
}

type joinTallies struct {
	ByToon, ByOppElim, ByRace, Unresolved, NotIngested int
}

func (t joinTallies) String() string {
	return fmt.Sprintf("by-toon=%d by-opp-elim=%d by-race=%d unresolved=%d not-ingested=%d",
		t.ByToon, t.ByOppElim, t.ByRace, t.Unresolved, t.NotIngested)
}

type playerRow struct {
	id   int64
	name string
	race string
}

func join(db *sql.DB, byMatch map[string][]proSide) ([]joinedPlayer, joinTallies, error) {
	type replayRow struct {
		id       int64
		fileName string
		matchup  string
		players  []playerRow
	}
	rows, err := db.Query(`
		SELECT r.id, r.file_name, r.matchup, p.id, p.name, p.race
		FROM replays r
		JOIN players p ON p.replay_id = r.id
		WHERE r.team_format = '1v1' AND p.is_observer = 0 AND p.type = 'Human'
		ORDER BY r.id, p.id`)
	if err != nil {
		return nil, joinTallies{}, err
	}
	defer rows.Close()
	byFile := map[string]*replayRow{}
	for rows.Next() {
		var rid, pid int64
		var fileName, matchup, pname, prace string
		if err := rows.Scan(&rid, &fileName, &matchup, &pid, &pname, &prace); err != nil {
			return nil, joinTallies{}, err
		}
		rr, ok := byFile[fileName]
		if !ok {
			rr = &replayRow{id: rid, fileName: fileName, matchup: matchup}
			byFile[fileName] = rr
		}
		rr.players = append(rr.players, playerRow{id: pid, name: pname, race: prace})
	}
	if err := rows.Err(); err != nil {
		return nil, joinTallies{}, err
	}

	var out []joinedPlayer
	var t joinTallies
	for matchID, sides := range byMatch {
		rr, ok := byFile[matchID+".rep"]
		if !ok {
			t.NotIngested += len(sides)
			continue
		}
		for _, s := range sides {
			p, method := resolvePlayer(rr.players, s)
			switch method {
			case "toon":
				t.ByToon++
			case "opp":
				t.ByOppElim++
			case "race":
				t.ByRace++
			default:
				t.Unresolved++
				continue
			}
			out = append(out, joinedPlayer{
				replayID: rr.id, playerID: p.id, fileName: rr.fileName,
				matchup: rr.matchup, name: p.name, race: p.race,
			})
		}
	}
	return out, t, nil
}

func resolvePlayer(players []playerRow, s proSide) (playerRow, string) {
	var zero playerRow
	if s.toon != "" {
		for _, p := range players {
			if p.name == s.toon {
				return p, "toon"
			}
		}
	}
	if s.oppToon != "" && len(players) == 2 {
		if players[0].name == s.oppToon {
			return players[1], "opp"
		}
		if players[1].name == s.oppToon {
			return players[0], "opp"
		}
	}
	var match *playerRow
	count := 0
	for i := range players {
		if players[i].race == s.race {
			match = &players[i]
			count++
		}
	}
	if count == 1 {
		return *match, "race"
	}
	return zero, ""
}

// markerRow is one bo_% marker event of a resolved pro player.
type markerRow struct {
	jp      joinedPlayer
	feature string
	payload []byte
}

func loadMarkerRows(db *sql.DB, joined []joinedPlayer, like string) ([]markerRow, error) {
	byPlayer := map[int64]joinedPlayer{}
	for _, jp := range joined {
		byPlayer[jp.playerID] = jp
	}
	rows, err := db.Query(`
		SELECT e.source_player_id, e.event_type, e.payload
		FROM replay_events e
		WHERE e.event_kind = 'marker' AND e.event_type LIKE ? ESCAPE '\'`, like)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []markerRow
	for rows.Next() {
		var pid sql.NullInt64
		var feature string
		var payload sql.NullString
		if err := rows.Scan(&pid, &feature, &payload); err != nil {
			return nil, err
		}
		jp, ok := byPlayer[pid.Int64]
		if !ok {
			continue
		}
		out = append(out, markerRow{jp: jp, feature: feature, payload: []byte(payload.String)})
	}
	return out, rows.Err()
}

func measureMilestones(db *sql.DB, joined []joinedPlayer, outDir string, minN int) error {
	rows, err := loadMarkerRows(db, joined, `bo\_%`)
	if err != nil {
		return err
	}

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
				r.feature, i, ev.Key, actuals[i].Second, r.jp.fileName, r.jp.name, r.jp.matchup)
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
// first Spawning Pool / Hatchery / Overlord seconds from the commands table —
// the same source and filters as ListEarlyZergMorphsForBOTimings, which is
// what the dashboard renders for the simplified Zerg BO rows.
func measureFuzzy(db *sql.DB, joined []joinedPlayer, outDir string) error {
	rows, err := loadMarkerRows(db, joined, `bo\_z\_fuzzy`)
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(outDir, "fuzzy.tsv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "label\tpool_sec\thatch_sec\toverlord_sec\tfile\tplayer\tmatchup")
	stmt, err := db.Prepare(`
		SELECT c.action_type, c.unit_type, MIN(c.seconds_from_game_start)
		FROM commands c
		WHERE c.player_id = ? AND c.seconds_from_game_start < 600
		  AND ((c.action_type = 'Build' AND c.unit_type IN ('Spawning Pool', 'Hatchery'))
		    OR (c.action_type = 'Unit Morph' AND c.unit_type = 'Overlord'))
		GROUP BY c.action_type, c.unit_type`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		label, ok := markers.DecodePayloadLabel(r.payload)
		if !ok {
			continue
		}
		firsts := map[string]int{}
		frows, err := stmt.Query(r.jp.playerID)
		if err != nil {
			return err
		}
		for frows.Next() {
			var action, unit string
			var sec int
			if err := frows.Scan(&action, &unit, &sec); err != nil {
				frows.Close()
				return err
			}
			firsts[unit] = sec
		}
		if err := frows.Err(); err != nil {
			frows.Close()
			return err
		}
		frows.Close()
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", label,
			tsvInt(firsts, "Spawning Pool"), tsvInt(firsts, "Hatchery"), tsvInt(firsts, "Overlord"),
			r.jp.fileName, r.jp.name, r.jp.matchup)
	}
	return w.Flush()
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
func measurePhase2(db *sql.DB, joined []joinedPlayer, outDir string) error {
	f, err := os.Create(filepath.Join(outDir, "phase2.tsv"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)

	oppRace := map[int64]map[int64]string{} // replayID -> playerID -> opponent race
	prows, err := db.Query(`
		SELECT p.replay_id, p.id, p.race FROM players p
		JOIN replays r ON r.id = p.replay_id
		WHERE r.team_format = '1v1' AND p.is_observer = 0 AND p.type = 'Human'`)
	if err != nil {
		return err
	}
	type pr struct {
		id   int64
		race string
	}
	replayPlayers := map[int64][]pr{}
	for prows.Next() {
		var rid, pid int64
		var race string
		if err := prows.Scan(&rid, &pid, &race); err != nil {
			prows.Close()
			return err
		}
		replayPlayers[rid] = append(replayPlayers[rid], pr{pid, race})
	}
	if err := prows.Err(); err != nil {
		prows.Close()
		return err
	}
	prows.Close()
	for rid, ps := range replayPlayers {
		if len(ps) != 2 {
			continue
		}
		oppRace[rid] = map[int64]string{ps[0].id: ps[1].race, ps[1].id: ps[0].race}
	}

	firstHPUp := map[int64]int{}     // playerID -> second
	firstResearch := map[int64]int{} // playerID -> second (tech or non-HP upgrade)
	crows, err := db.Query(`
		SELECT c.player_id, c.action_type, c.upgrade_name, MIN(c.seconds_from_game_start)
		FROM commands c
		WHERE c.action_type IN ('Upgrade', 'Tech')
		GROUP BY c.player_id, c.action_type, c.upgrade_name`)
	if err != nil {
		return err
	}
	for crows.Next() {
		var pid int64
		var action string
		var upgrade sql.NullString
		var sec int
		if err := crows.Scan(&pid, &action, &upgrade, &sec); err != nil {
			crows.Close()
			return err
		}
		isHP := action == "Upgrade" && upgrade.Valid && models.IsHPUpgrade(upgrade.String)
		target := firstResearch
		if isHP {
			target = firstHPUp
		}
		if cur, ok := target[pid]; !ok || sec < cur {
			target[pid] = sec
		}
	}
	if err := crows.Err(); err != nil {
		crows.Close()
		return err
	}
	crows.Close()

	upSecs := map[string][]int{}
	techSecs := map[string][]int{}
	for _, jp := range joined {
		opp, ok := oppRace[jp.replayID][jp.playerID]
		if !ok {
			continue
		}
		key := jp.race + "v" + opp
		if s, ok := firstHPUp[jp.playerID]; ok {
			upSecs[key] = append(upSecs[key], s)
		}
		if s, ok := firstResearch[jp.playerID]; ok {
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

	gaps, err := mutaTurretGaps(db)
	if err != nil {
		return err
	}
	sort.Ints(gaps)
	fmt.Fprintf(w, "muta_turret_gap\tTvZ\t%d\t%d\t%d\n", len(gaps), percentile(gaps, 0.05), percentile(gaps, 0.50))
	fmt.Fprintf(w, "muta_turret_gap_p25_p75\tTvZ\t%d\t%d\t%d\n", len(gaps), percentile(gaps, 0.25), percentile(gaps, 0.75))
	return w.Flush()
}

// mutaTurretGaps pairs the mutalisk_timing / turret_timing payloads per replay
// and computes turret_finish - muta_finish with the same prerequisite clamping
// as populateMutaliskTimingForGameDetail.
func mutaTurretGaps(db *sql.DB) ([]int, error) {
	rows, err := db.Query(`
		SELECT e.replay_id, e.event_type, e.payload
		FROM replay_events e
		WHERE e.event_kind = 'marker' AND e.event_type IN ('mutalisk_timing', 'turret_timing')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type side struct {
		spireCmd, firstMutaCmd, ebayCmd, firstTurretCmd int
		hasZ, hasT                                      bool
	}
	byReplay := map[int64]*side{}
	for rows.Next() {
		var rid int64
		var typ string
		var payload sql.NullString
		if err := rows.Scan(&rid, &typ, &payload); err != nil {
			return nil, err
		}
		var raw map[string]float64
		if err := json.Unmarshal([]byte(payload.String), &raw); err != nil {
			continue
		}
		s := byReplay[rid]
		if s == nil {
			s = &side{}
			byReplay[rid] = s
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
	if err := rows.Err(); err != nil {
		return nil, err
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
	return gaps, nil
}

func writeMeta(outDir string, sides []proSide, tallies joinTallies) error {
	keys := make([]string, 0, len(sides))
	for _, s := range sides {
		keys = append(keys, fmt.Sprintf("%s:%d", s.matchID, s.auroraID))
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintln(h, k)
	}
	meta := map[string]any{
		"algorithm_version": core.AlgorithmVersion,
		"corpus_hash":       fmt.Sprintf("%x", h.Sum(nil)),
		"pro_player_games":  len(sides),
		"join":              tallies,
		"generated_at":      time.Now().UTC().Format(time.RFC3339),
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
