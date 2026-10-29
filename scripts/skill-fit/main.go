// Command skill-fit turns scripts/skill-mine's measurements into internal/skill/model.json: per race, the
// cut values of every Skill tab metric between neighbouring tiers, and the combined score's regression and
// cuts. Tiers come from each player's own MMR (under 1,400, 1,400 to 1,799, 1,800 and up) and from the
// progamer labels (Pro).
//
//	go run ./scripts/skill-fit -in /tmp/skill-mine.jsonl -out internal/skill/model.json
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"math"
	"os"
	"sort"

	"github.com/marianogappa/screpdb/internal/skill"
)

type sample struct {
	Match string `json:"match"`
	Skip  string `json:"skip"`
	Side  struct {
		Race string `json:"race"`
		MMR  int    `json:"mmr"`
		Pro  string `json:"pro"`
	} `json:"side"`
	Values  map[string]float64 `json:"values"`
	Engine  string             `json:"engine"`
	MapKind string             `json:"map_kind"`
	Humans  int                `json:"humans"`
}

func (s sample) tier() int {
	switch {
	case s.Side.Pro != "":
		return skill.TierPro
	case s.Side.MMR < 1400:
		return skill.TierBeginner
	case s.Side.MMR < 1800:
		return skill.TierIntermediate
	}
	return skill.TierAdvanced
}

// Every metric the Skill tab shows is also in the combined score: one that does not count toward the tier
// has no business on the page.
var chartMetrics = []string{skill.EAPM, skill.UsedRecalls, skill.CheckRecalls, skill.ArmyCadence, skill.UnspentPerIncome, skill.ProductionIdle}

var combinedFeatures = chartMetrics

// perRaceMetrics depend on what a race's production makes, so their values only compare within a race:
// idle production is idle buildings for Terran and Protoss but waiting larva for Zerg, and Protoss, whose
// gateway units cost the most, starts army units least often. Unspent resources follow the same production differences.
var perRaceMetrics = map[string]bool{skill.ArmyCadence: true, skill.UnspentPerIncome: true, skill.ProductionIdle: true}

// Each metric's share of the combined score's weight stays within these bounds: a regression left alone
// hands most of the weight to whichever mechanics metric correlates best, and the score turns into a
// mirror of that one number.
const (
	minWeightShare = 0.10
	maxWeightShare = 0.30
)

func main() {
	in := flag.String("in", "", "skill-mine JSONL")
	out := flag.String("out", "internal/skill/model.json", "model file")
	margin := flag.Float64("confident-margin", 0.3, "distance from every cut, in tiers, for a confident combined tier")
	contextPath := flag.String("context", "", "skill-mine -dir output (unlabelled games of every kind), to adjust team games and money maps")
	flag.Parse()

	samples := read(*in)
	model := skill.Model{Players: len(samples), ConfidentMargin: *margin, Metrics: map[string]skill.MetricCuts{}}
	games := map[string]bool{}
	for _, s := range samples {
		games[s.Match] = true
	}
	model.Games = len(games)

	for _, key := range chartMetrics {
		var all []sample
		for _, s := range samples {
			if _, ok := s.Values[key]; ok {
				all = append(all, s)
			}
		}
		rho := spearman(all, key)
		if perRaceMetrics[key] {
			rho = 0
			for _, race := range []string{"T", "P", "Z"} {
				var own []sample
				for _, s := range all {
					if s.Side.Race == race {
						own = append(own, s)
					}
				}
				rho += spearman(own, key) / 3
			}
		}
		mc := skill.MetricCuts{HigherIsBetter: rho >= 0, PerRace: perRaceMetrics[key], Rho: round(rho, 2), Cuts: map[string][3]float64{}, Ends: map[string][2]float64{}}
		sign := 1.0
		if rho < 0 {
			sign = -1
		}
		for _, race := range []string{"T", "P", "Z", skill.AllRaces} {
			var scored []scoredTier
			for _, s := range all {
				if s.Side.Race == race || race == skill.AllRaces {
					scored = append(scored, scoredTier{sign * s.Values[key], s.tier()})
				}
			}
			if len(scored) < 300 {
				continue
			}
			c := equalErrorCuts(scored)
			mc.Cuts[race] = [3]float64{round(sign*c[0], 3), round(sign*c[1], 3), round(sign*c[2], 3)}
			lo, hi := ends(scored)
			if sign > 0 && lo > 0 {
				// A metric that counts something reaches down to zero, its natural floor.
				lo = 0
			}
			mc.Ends[race] = [2]float64{round(sign*lo, 3), round(sign*hi, 3)}
		}
		model.Metrics[key] = mc
	}

	model.Combined = skill.Combined{Features: combinedFeatures, Races: map[string]skill.CombinedForRace{}}
	for _, race := range []string{"T", "P", "Z"} {
		var rows []sample
		for _, s := range samples {
			if s.Side.Race != race {
				continue
			}
			ok := true
			for _, f := range combinedFeatures {
				if _, has := s.Values[f]; !has {
					ok = false
				}
			}
			if ok {
				rows = append(rows, s)
			}
		}
		cr := fitRace(rows, model.Metrics)
		var scored []scoredTier
		for _, s := range rows {
			scored = append(scored, scoredTier{predict(cr, s), s.tier()})
		}
		cr.Cuts = equalErrorCuts(scored)
		lo, hi := ends(scored)
		cr.Ends = [2]float64{round(lo, 4), round(hi, 4)}
		for i := range cr.Cuts {
			cr.Cuts[i] = round(cr.Cuts[i], 4)
		}
		model.Combined.Races[race] = cr
		log.Printf("%s: %d players, weights %v", race, len(rows), cr.Weights)
	}

	if *contextPath != "" {
		model.ContextAdjust = contextAdjust(&model, readContext(*contextPath), samples)
	}

	b, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, append(b, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s: %d players from %d games", *out, model.Players, model.Games)
}

var adjustedMetrics = []string{skill.ArmyCadence, skill.UnspentPerIncome, skill.ProductionIdle}

// adjustedTiers are the EAPM tiers the context tables are built from: money-map and team players are
// almost all below Advanced, so only there do the contexts have enough players to compare. EAPM is the
// mechanics level here because money and team play do not change it, and the combined score now holds the
// very macro metrics being adjusted.
var adjustedTiers = map[int]bool{skill.TierBeginner: true, skill.TierIntermediate: true}

const contextQuantiles = 21

func readContext(path string) []sample {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var out []sample
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var x sample
		if json.Unmarshal(s.Bytes(), &x) == nil && x.Engine == "high" && x.Values["apm"] >= skill.MinAPM && x.MapKind != "" {
			out = append(out, x)
		}
	}
	return out
}

// contextAdjust builds, per context and race, quantile tables that convert each macro metric to its 1v1
// ladder value at the same percentile, among players of the same mechanics level. Per race, because the
// same metric can live on different scales: idle production is idle buildings for Terran and Protoss but
// capped larva for Zerg.
func contextAdjust(model *skill.Model, rows, ladder []sample) map[string]map[string]skill.QuantileMap {
	matched := func(r sample) bool {
		res := model.Evaluate(r.Side.Race, skill.ContextOneVsOne, map[string]float64{skill.EAPM: r.Values[skill.EAPM]})
		p, ok := res.Metrics[skill.EAPM]
		return ok && adjustedTiers[p.Tier]
	}
	from, to := map[string][]sample{}, map[string][]sample{}
	for _, r := range rows {
		if !matched(r) {
			continue
		}
		switch {
		case r.MapKind == "money":
			from[skill.AdjustKey(skill.ContextMoney, r.Side.Race)] = append(from[skill.AdjustKey(skill.ContextMoney, r.Side.Race)], r)
		case r.Humans != 2:
			from[skill.AdjustKey(skill.ContextTeamRegular, r.Side.Race)] = append(from[skill.AdjustKey(skill.ContextTeamRegular, r.Side.Race)], r)
		}
	}
	for _, r := range ladder {
		if r.Side.Pro == "" && matched(r) {
			to[r.Side.Race] = append(to[r.Side.Race], r)
		}
	}
	out := map[string]map[string]skill.QuantileMap{}
	for _, ctx := range []string{skill.ContextTeamRegular, skill.ContextMoney} {
		for _, race := range []string{"T", "P", "Z"} {
			key := skill.AdjustKey(ctx, race)
			for _, metric := range adjustedMetrics {
				f, t := quantiles(from[key], metric), quantiles(to[race], metric)
				if f == nil || t == nil {
					log.Printf("%s %s: too few players to adjust", key, metric)
					continue
				}
				if out[key] == nil {
					out[key] = map[string]skill.QuantileMap{}
				}
				out[key][metric] = skill.QuantileMap{From: f, To: t}
				log.Printf("%s %s: median %.3f -> %.3f", key, metric, f[contextQuantiles/2], t[contextQuantiles/2])
			}
		}
	}
	return out
}

func quantiles(rows []sample, metric string) []float64 {
	var xs []float64
	for _, r := range rows {
		if v, ok := r.Values[metric]; ok {
			xs = append(xs, v)
		}
	}
	if len(xs) < 30 {
		return nil
	}
	sort.Float64s(xs)
	out := make([]float64, contextQuantiles)
	for i := range out {
		out[i] = round(xs[i*(len(xs)-1)/(contextQuantiles-1)], 4)
	}
	return out
}

func read(path string) []sample {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var out []sample
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var x sample
		if json.Unmarshal(s.Bytes(), &x) != nil || x.Skip != "" || x.Values["apm"] < skill.MinAPM || x.Side.MMR <= 0 {
			continue
		}
		out = append(out, x)
	}
	return out
}

// ends are the 0.5th and 99.5th percentile scores: where the axis stops, past the outer cuts.
func ends(scored []scoredTier) (float64, float64) {
	xs := make([]float64, len(scored))
	for i, s := range scored {
		xs[i] = s.score
	}
	sort.Float64s(xs)
	return xs[len(xs)*5/1000], xs[len(xs)*995/1000]
}

type scoredTier struct {
	score float64
	tier  int
}

// equalErrorCuts puts each cut where as many of the lower tier score at or above it as of the upper tier
// score below it, so no boundary favours either side.
func equalErrorCuts(scored []scoredTier) [3]float64 {
	var cuts [3]float64
	for k := 0; k < 3; k++ {
		var lo, hi []float64
		for _, s := range scored {
			switch s.tier {
			case k:
				lo = append(lo, s.score)
			case k + 1:
				hi = append(hi, s.score)
			}
		}
		sort.Float64s(lo)
		sort.Float64s(hi)
		candidates := append(append([]float64(nil), lo...), hi...)
		sort.Float64s(candidates)
		best, gap := 0.0, math.Inf(1)
		for _, c := range candidates {
			above := float64(len(lo)-sort.SearchFloat64s(lo, c)) / float64(len(lo))
			below := float64(sort.SearchFloat64s(hi, c)) / float64(len(hi))
			if d := math.Abs(above - below); d < gap {
				best, gap = c, d
			}
		}
		cuts[k] = best
		if k > 0 && cuts[k] < cuts[k-1] {
			cuts[k] = cuts[k-1]
		}
	}
	return cuts
}

// fitRace is a ridge regression of the tier on clamped z-scores of the combined features, with each
// weight then given the sign its metric has on its own and a share between minWeightShare and
// maxWeightShare; a last fit rescales the bounded weights onto the tier.
func fitRace(rows []sample, metrics map[string]skill.MetricCuts) skill.CombinedForRace {
	n := len(combinedFeatures)
	cr := skill.CombinedForRace{Mean: make([]float64, n), SD: make([]float64, n), Weights: make([]float64, n)}
	for i, f := range combinedFeatures {
		var sum, sq float64
		for _, s := range rows {
			sum += s.Values[f]
		}
		cr.Mean[i] = sum / float64(len(rows))
		for _, s := range rows {
			d := s.Values[f] - cr.Mean[i]
			sq += d * d
		}
		cr.SD[i] = math.Sqrt(sq / float64(len(rows)))
		if cr.SD[i] == 0 {
			cr.SD[i] = 1
		}
	}
	k := n + 1
	a := make([][]float64, k)
	b := make([]float64, k)
	for i := range a {
		a[i] = make([]float64, k)
		if i > 0 {
			a[i][i] = 1
		}
	}
	for _, s := range rows {
		x := features(cr, s)
		y := float64(s.tier())
		for i := 0; i < k; i++ {
			b[i] += x[i] * y
			for j := 0; j < k; j++ {
				a[i][j] += x[i] * x[j]
			}
		}
	}
	w := solve(a, b)
	signs := make([]float64, n)
	for i, f := range combinedFeatures {
		signs[i] = -1
		if metrics[f].HigherIsBetter {
			signs[i] = 1
		}
	}
	shares := boundedShares(w[1:], signs)
	var sx, sy, sxx, sxy float64
	for _, s := range rows {
		x := features(cr, s)
		composite := 0.0
		for i := range shares {
			composite += signs[i] * shares[i] * x[i+1]
		}
		y := float64(s.tier())
		sx, sy, sxx, sxy = sx+composite, sy+y, sxx+composite*composite, sxy+composite*y
	}
	m := float64(len(rows))
	scale := (m*sxy - sx*sy) / (m*sxx - sx*sx)
	cr.Intercept = round((sy-scale*sx)/m, 5)
	for i := range cr.Weights {
		cr.Weights[i] = round(scale*signs[i]*shares[i], 5)
		cr.Mean[i] = round(cr.Mean[i], 4)
		cr.SD[i] = round(cr.SD[i], 4)
	}
	return cr
}

// boundedShares turns regression weights into shares of the total that respect each metric's own sign and
// the share bounds: a weight of the wrong sign (a side effect of correlated metrics) starts from zero.
func boundedShares(w, signs []float64) []float64 {
	shares := make([]float64, len(w))
	total := 0.0
	for i := range w {
		shares[i] = max(0, w[i]*signs[i])
		total += shares[i]
	}
	for i := range shares {
		shares[i] /= max(total, 1e-9)
	}
	for range 100 {
		sum := 0.0
		for i := range shares {
			shares[i] = min(maxWeightShare, max(minWeightShare, shares[i]))
			sum += shares[i]
		}
		for i := range shares {
			shares[i] /= sum
		}
	}
	return shares
}

func features(cr skill.CombinedForRace, s sample) []float64 {
	x := []float64{1}
	for i, f := range combinedFeatures {
		x = append(x, max(-3, min(3, (s.Values[f]-cr.Mean[i])/cr.SD[i])))
	}
	return x
}

func predict(cr skill.CombinedForRace, s sample) float64 {
	x := features(cr, s)
	score := cr.Intercept
	for i, w := range cr.Weights {
		score += w * x[i+1]
	}
	return score
}

func solve(a [][]float64, b []float64) []float64 {
	n := len(b)
	for c := 0; c < n; c++ {
		p := c
		for r := c + 1; r < n; r++ {
			if math.Abs(a[r][c]) > math.Abs(a[p][c]) {
				p = r
			}
		}
		a[c], a[p] = a[p], a[c]
		b[c], b[p] = b[p], b[c]
		for r := 0; r < n; r++ {
			if r == c || a[c][c] == 0 {
				continue
			}
			f := a[r][c] / a[c][c]
			for k := c; k < n; k++ {
				a[r][k] -= f * a[c][k]
			}
			b[r] -= f * b[c]
		}
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = b[i] / a[i][i]
	}
	return out
}

func spearman(rows []sample, key string) float64 {
	xs := make([]float64, len(rows))
	ys := make([]float64, len(rows))
	for i, s := range rows {
		xs[i], ys[i] = s.Values[key], float64(s.Side.MMR)
	}
	return pearson(ranks(xs), ranks(ys))
}

func ranks(xs []float64) []float64 {
	idx := make([]int, len(xs))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(a, b int) bool { return xs[idx[a]] < xs[idx[b]] })
	r := make([]float64, len(xs))
	for i := 0; i < len(idx); {
		j := i
		for j+1 < len(idx) && xs[idx[j+1]] == xs[idx[i]] {
			j++
		}
		for k := i; k <= j; k++ {
			r[idx[k]] = float64(i+j) / 2
		}
		i = j + 1
	}
	return r
}

func pearson(a, b []float64) float64 {
	n := float64(len(a))
	var ma, mb float64
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma, mb = ma/n, mb/n
	var cov, va, vb float64
	for i := range a {
		cov += (a[i] - ma) * (b[i] - mb)
		va += (a[i] - ma) * (a[i] - ma)
		vb += (b[i] - mb) * (b[i] - mb)
	}
	if va == 0 || vb == 0 {
		return 0
	}
	return cov / math.Sqrt(va*vb)
}

func round(x float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(x*p) / p
}
