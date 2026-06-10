package dashboard

import (
	"database/sql"
	"math"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/dashboard/db"
)

// Supply discipline is an experimental skill proxy: how steadily a player keeps
// supply ahead in the early game. We take every supply-providing command
// (depot/pylon/overlord/base), seed each race's starting supply at t=0, and
// measure the gaps between additions up to the point supply is effectively
// maxed (or 80% of the game). Gaps are weighted toward the early game (linear
// decay to 0 at 15 min) because that is where a supply block hurts most — the
// validated phase/weighting result. The per-game weighted-mean gap is then
// matchup-normalized (gaps differ a lot by matchup) into a 0-100 score where
// higher = more disciplined.
//
// Caveats baked into the data: Overlord/Pylon counts are not de-duped (see
// internal/builddedup — only Terran depots are), and the metric cannot tell a
// real supply block from "supply maxed / not needed". Hence: experimental.

const (
	supplyWeightZeroSecond     = 15 * 60 // gap weight decays linearly to 0 here
	supplyCapProvided          = 200     // stop the window once supply is maxed
	supplyMinAdds              = 4       // need >= this many additions (incl. seed)
	supplyMinWindowSeconds     = 120
	supplyEarlyGapStartSeconds = 7 * 60 // "worst early gaps" are those starting before here
	supplyScorePerSD           = 16.0   // z-score -> 0-100 score slope
	supplyDefaultMinGames      = 4
	supplyMaxLimit             = 5000
)

// supplyProviderValue is the displayed supply each provider adds. Lair/Hive are
// morphs of an already-counted Hatchery, so they add nothing and are absent.
var supplyProviderValue = map[string]int64{
	"Supply Depot": 8, "Pylon": 8, "Overlord": 8,
	"Command Center": 10, "Nexus": 9, "Hatchery": 1,
}

// supplyRaceSeed is the supply each race starts with (starting base + initial
// Overlord) — these have no Build/morph command, so they seed the t=0 point.
var supplyRaceSeed = map[string]int64{"Terran": 10, "Protoss": 9, "Zerg": 9}

type supplyBaseline struct{ Mean, SD float64 }

// supplyBaselines calibrate the per-game score, keyed [ownRace][oppRace]. Derived
// from a reference corpus (weighted-mean early supply gap, seconds). Used only
// for the per-game score; the player-level leaderboard recomputes percentiles
// from the live corpus, so it stays accurate on any database.
var supplyBaselines = map[string]map[string]supplyBaseline{
	"Protoss": {"Protoss": {49.6, 11.6}, "Terran": {43.9, 10.8}, "Zerg": {46.9, 10.3}},
	"Terran":  {"Protoss": {54.5, 12.3}, "Terran": {56.2, 11.4}, "Zerg": {59.0, 12.7}},
	"Zerg":    {"Protoss": {46.2, 11.2}, "Terran": {54.5, 12.5}, "Zerg": {76.7, 20.4}},
}

var supplyGlobalBaseline = supplyBaseline{Mean: 52.0, SD: 14.0}

func supplyBaselineFor(ownRace, oppRace string) supplyBaseline {
	if byOpp, ok := supplyBaselines[ownRace]; ok {
		if b, ok := byOpp[oppRace]; ok {
			return b
		}
	}
	return supplyGlobalBaseline
}

func supplyOppRace(matchup, ownRace string) string {
	letter := map[string]string{"P": "Protoss", "T": "Terran", "Z": "Zerg"}
	m := strings.TrimSpace(matchup)
	if len(m) != 3 || m[1] != 'v' {
		return ownRace
	}
	a, b := string(m[0]), string(m[2])
	own := ""
	if ownRace != "" {
		own = string(ownRace[0])
	}
	if b == own {
		return letter[a]
	}
	return letter[b]
}

func supplyGapWeight(startSecond int64) float64 {
	w := 1.0 - float64(startSecond)/float64(supplyWeightZeroSecond)
	if w < 0 {
		return 0
	}
	return w
}

type supplyGap struct {
	StartSecond int64
	DurationSec int64
}

// supplyGameResult is the per-player-game outcome of the metric.
type supplyGameResult struct {
	Eligible       bool
	WeightedGapSec float64
	SupplyCount    int64
	Gaps           []supplyGap
}

// computeSupplyGame folds one player's supply events (any order) into the
// weighted-gap metric, seeding the race's starting supply at t=0 and windowing
// at the supply cap / 80% of the game.
func computeSupplyGame(events []db.SupplyProviderEventRow, race string, durationSeconds int64) supplyGameResult {
	type add struct {
		sec int64
		val int64
	}
	adds := []add{{0, supplyRaceSeed[race]}}
	for _, e := range events {
		v := supplyProviderValue[e.UnitType]
		if v == 0 {
			continue
		}
		sec := e.Second
		if sec < 0 {
			sec = 0
		}
		adds = append(adds, add{sec, v})
	}
	sort.SliceStable(adds, func(i, j int) bool { return adds[i].sec < adds[j].sec })

	end := int64(0.8 * float64(durationSeconds))
	times := []int64{}
	cum := int64(0)
	for _, a := range adds {
		cum += a.val
		if a.sec > end {
			break
		}
		times = append(times, a.sec)
		if cum >= supplyCapProvided {
			break
		}
	}
	if len(times) < supplyMinAdds || times[len(times)-1]-times[0] < supplyMinWindowSeconds {
		return supplyGameResult{}
	}
	gaps := make([]supplyGap, 0, len(times)-1)
	num, den := 0.0, 0.0
	for i := 0; i+1 < len(times); i++ {
		start := times[i]
		dur := times[i+1] - times[i]
		gaps = append(gaps, supplyGap{StartSecond: start, DurationSec: dur})
		w := supplyGapWeight(start)
		num += w * float64(dur)
		den += w
	}
	wgap := 0.0
	if den > 0 {
		wgap = num / den
	}
	return supplyGameResult{Eligible: true, WeightedGapSec: wgap, SupplyCount: int64(len(times)), Gaps: gaps}
}

func supplyScore(weightedGap float64, ownRace, oppRace string) int64 {
	b := supplyBaselineFor(ownRace, oppRace)
	if b.SD <= 0 {
		return 50
	}
	z := (weightedGap - b.Mean) / b.SD
	score := math.Round(50 - supplyScorePerSD*z)
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int64(score)
}

func topSupplyEarlyGaps(gaps []supplyGap, n int) []workflowSupplyGap {
	early := make([]supplyGap, 0, len(gaps))
	for _, g := range gaps {
		if g.StartSecond < supplyEarlyGapStartSeconds {
			early = append(early, g)
		}
	}
	sort.Slice(early, func(i, j int) bool {
		if early[i].DurationSec == early[j].DurationSec {
			return early[i].StartSecond < early[j].StartSecond
		}
		return early[i].DurationSec > early[j].DurationSec
	})
	if len(early) > n {
		early = early[:n]
	}
	out := make([]workflowSupplyGap, 0, len(early))
	for _, g := range early {
		out = append(out, workflowSupplyGap{StartSecond: g.StartSecond, DurationSec: g.DurationSec})
	}
	return out
}

// populateSupplyDisciplineForGameDetail fills detail.SupplyDiscipline (one row
// per player) for a single game's report.
func (d *Dashboard) populateSupplyDisciplineForGameDetail(detail *workflowGameDetail) error {
	detail.SupplyDiscipline = []workflowSupplyDisciplinePlayer{}
	rows, err := d.dbStore.ListSupplyProviderEventsForReplay(d.ctx, detail.ReplayID)
	if err != nil {
		return err
	}
	byPlayer := map[int64][]db.SupplyProviderEventRow{}
	for _, r := range rows {
		byPlayer[r.PlayerID] = append(byPlayer[r.PlayerID], r)
	}
	for _, p := range detail.Players {
		res := computeSupplyGame(byPlayer[p.PlayerID], p.Race, detail.DurationSeconds)
		entry := workflowSupplyDisciplinePlayer{
			PlayerID:   p.PlayerID,
			PlayerKey:  p.PlayerKey,
			PlayerName: p.Name,
			Team:       p.Team,
			IsWinner:   p.IsWinner,
			WorstGaps:  []workflowSupplyGap{},
		}
		if !res.Eligible {
			entry.IneligibleReason = "too few supply additions"
			detail.SupplyDiscipline = append(detail.SupplyDiscipline, entry)
			continue
		}
		opp := supplyOppRaceForReplay(detail, p.Race)
		base := supplyBaselineFor(p.Race, opp)
		entry.Eligible = true
		entry.WeightedGapSec = math.Round(res.WeightedGapSec*10) / 10
		entry.TypicalGapSec = base.Mean
		entry.SupplyCount = res.SupplyCount
		entry.Score = supplyScore(res.WeightedGapSec, p.Race, opp)
		entry.WorstGaps = topSupplyEarlyGaps(res.Gaps, 5)
		detail.SupplyDiscipline = append(detail.SupplyDiscipline, entry)
	}
	return nil
}

// supplyOppRaceForReplay derives the opponent race for a 1v1 game from the
// game's matchup-like info. For team games / unknowns it falls back to the
// player's own race (mirror baseline).
func supplyOppRaceForReplay(detail *workflowGameDetail, ownRace string) string {
	if len(detail.Players) == 2 {
		for _, other := range detail.Players {
			if other.Race != "" && other.PlayerKey != "" && other.Race != ownRace {
				return other.Race
			}
		}
		for _, other := range detail.Players {
			if other.Race != "" {
				return other.Race
			}
		}
	}
	return ownRace
}

// --- player-level corpus aggregation ---

type supplyReplayMetric struct {
	ReplayID   int64
	PlayerKey  string
	PlayerName string
	OwnRace    string
	OppRace    string
	Score      int64
	GapSec     float64
}

func (d *Dashboard) querySupplyReplayMetrics(onlyPlayerKey string) ([]supplyReplayMetric, error) {
	rows, err := d.dbStore.ListSupplyProviderEvents(d.ctx, onlyPlayerKey)
	if err != nil {
		return nil, err
	}
	type key struct {
		replay int64
		pid    int64
	}
	grouped := map[key][]db.SupplyProviderEventRow{}
	order := []key{}
	for _, r := range rows {
		k := key{r.ReplayID, r.PlayerID}
		if _, ok := grouped[k]; !ok {
			order = append(order, k)
		}
		grouped[k] = append(grouped[k], r)
	}
	out := []supplyReplayMetric{}
	for _, k := range order {
		evs := grouped[k]
		head := evs[0]
		res := computeSupplyGame(evs, head.Race, head.DurationSeconds)
		if !res.Eligible {
			continue
		}
		opp := supplyOppRace(head.Matchup, head.Race)
		out = append(out, supplyReplayMetric{
			ReplayID:   k.replay,
			PlayerKey:  head.PlayerKey,
			PlayerName: head.PlayerName,
			OwnRace:    head.Race,
			OppRace:    opp,
			Score:      supplyScore(res.WeightedGapSec, head.Race, opp),
			GapSec:     res.WeightedGapSec,
		})
	}
	return out, nil
}

func (d *Dashboard) buildWorkflowPlayerSupplyDisciplineLeaderboard(minGames int64, limit int64) (workflowPlayerSupplyDisciplineLeaderboard, error) {
	result := workflowPlayerSupplyDisciplineLeaderboard{
		SummaryVersion: workflowSummaryVersion,
		MinGames:       minGames,
		Bins:           []workflowPlayerSupplyHistogramBin{},
		Players:        []workflowPlayerSupplyPoint{},
	}
	if minGames <= 0 {
		minGames = supplyDefaultMinGames
		result.MinGames = minGames
	}
	if limit < 0 {
		limit = 0
	}
	if limit > supplyMaxLimit {
		limit = supplyMaxLimit
	}
	metrics, err := d.querySupplyReplayMetrics("")
	if err != nil {
		return result, err
	}
	type agg struct {
		name     string
		games    int64
		sumScore float64
		sumGap   float64
	}
	byPlayer := map[string]*agg{}
	for _, m := range metrics {
		entry, ok := byPlayer[m.PlayerKey]
		if !ok {
			entry = &agg{name: m.PlayerName}
			byPlayer[m.PlayerKey] = entry
		}
		entry.games++
		entry.sumScore += float64(m.Score)
		entry.sumGap += m.GapSec
		if strings.TrimSpace(entry.name) == "" {
			entry.name = m.PlayerName
		}
	}
	for playerKey, entry := range byPlayer {
		if entry.games < minGames {
			continue
		}
		denom := float64(entry.games)
		result.Players = append(result.Players, workflowPlayerSupplyPoint{
			PlayerKey:         playerKey,
			PlayerName:        entry.name,
			GamesUsed:         entry.games,
			Score:             entry.sumScore / denom,
			AvgWeightedGapSec: entry.sumGap / denom,
		})
	}
	sort.Slice(result.Players, func(i, j int) bool {
		if result.Players[i].Score == result.Players[j].Score {
			if result.Players[i].GamesUsed == result.Players[j].GamesUsed {
				return result.Players[i].PlayerName < result.Players[j].PlayerName
			}
			return result.Players[i].GamesUsed > result.Players[j].GamesUsed
		}
		return result.Players[i].Score > result.Players[j].Score
	})
	if limit > 0 && int64(len(result.Players)) > limit {
		result.Players = result.Players[:limit]
	}
	result.PlayersIncluded = int64(len(result.Players))
	if len(result.Players) == 0 {
		return result, nil
	}
	values := make([]float64, 0, len(result.Players))
	for _, p := range result.Players {
		values = append(values, p.Score)
	}
	sort.Float64s(values)
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	result.MeanScore = mean
	varSum := 0.0
	for _, v := range values {
		varSum += (v - mean) * (v - mean)
	}
	result.StddevScore = math.Sqrt(varSum / float64(len(values)))
	result.Bins = supplyHistogramBins(values)
	return result, nil
}

func supplyHistogramBins(sortedValues []float64) []workflowPlayerSupplyHistogramBin {
	binCount := int(math.Round(math.Sqrt(float64(len(sortedValues)))))
	if binCount < 8 {
		binCount = 8
	}
	if binCount > 24 {
		binCount = 24
	}
	minV, maxV := sortedValues[0], sortedValues[len(sortedValues)-1]
	if maxV <= minV {
		return []workflowPlayerSupplyHistogramBin{{X0: minV, X1: minV + 1, Count: int64(len(sortedValues))}}
	}
	width := (maxV - minV) / float64(binCount)
	bins := make([]workflowPlayerSupplyHistogramBin, binCount)
	for i := 0; i < binCount; i++ {
		start := minV + float64(i)*width
		end := minV + float64(i+1)*width
		if i == binCount-1 {
			end = maxV
		}
		bins[i] = workflowPlayerSupplyHistogramBin{X0: start, X1: end}
	}
	for _, v := range sortedValues {
		idx := int(math.Floor((v - minV) / width))
		if idx < 0 {
			idx = 0
		}
		if idx >= binCount {
			idx = binCount - 1
		}
		bins[idx].Count++
	}
	return bins
}

func (d *Dashboard) buildWorkflowPlayerSupplyDisciplineInsight(playerKey string) (workflowPlayerSupplyDisciplineInsight, error) {
	result := workflowPlayerSupplyDisciplineInsight{
		SummaryVersion: workflowSummaryVersion,
		PlayerKey:      playerKey,
		ByMatchup:      []workflowSupplyMatchupBreakdown{},
		WorstGames:     []workflowSupplyWorstGame{},
	}
	playerName, err := d.dbStore.GetPlayerNameByKey(d.ctx, playerKey)
	if err != nil {
		return result, err
	}
	result.PlayerName = playerName
	if result.PlayerName == "" {
		return result, sql.ErrNoRows
	}
	metrics, err := d.querySupplyReplayMetrics(playerKey)
	if err != nil {
		return result, err
	}
	if len(metrics) == 0 {
		return result, nil
	}
	sumScore, sumGap := 0.0, 0.0
	byOpp := map[string]*struct {
		n        int64
		sumScore float64
		sumGap   float64
	}{}
	for _, m := range metrics {
		sumScore += float64(m.Score)
		sumGap += m.GapSec
		e := byOpp[m.OppRace]
		if e == nil {
			e = &struct {
				n        int64
				sumScore float64
				sumGap   float64
			}{}
			byOpp[m.OppRace] = e
		}
		e.n++
		e.sumScore += float64(m.Score)
		e.sumGap += m.GapSec
	}
	result.GamesUsed = int64(len(metrics))
	result.Score = sumScore / float64(len(metrics))
	result.AvgWeightedGapSec = math.Round(sumGap/float64(len(metrics))*10) / 10
	for opp, e := range byOpp {
		result.ByMatchup = append(result.ByMatchup, workflowSupplyMatchupBreakdown{
			OppRace:           opp,
			GamesUsed:         e.n,
			Score:             e.sumScore / float64(e.n),
			AvgWeightedGapSec: math.Round(e.sumGap/float64(e.n)*10) / 10,
			TypicalGapSec:     supplyBaselineFor(metrics[0].OwnRace, opp).Mean,
		})
	}
	sort.Slice(result.ByMatchup, func(i, j int) bool { return result.ByMatchup[i].OppRace < result.ByMatchup[j].OppRace })
	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Score < metrics[j].Score })
	for i := 0; i < len(metrics) && i < 8; i++ {
		m := metrics[i]
		result.WorstGames = append(result.WorstGames, workflowSupplyWorstGame{
			ReplayID:       m.ReplayID,
			OppRace:        m.OppRace,
			Score:          m.Score,
			WeightedGapSec: math.Round(m.GapSec*10) / 10,
		})
	}
	return result, nil
}
