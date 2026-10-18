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
	Values map[string]float64 `json:"values"`
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

var chartMetrics = []string{skill.EAPM, skill.HotkeyRecalls, skill.ScreenSwitches, skill.SpendingQuotient, skill.HallBusy, skill.LarvaPerHatch, skill.BankedShare}

var combinedFeatures = []string{skill.EAPM, skill.HotkeyRecalls, skill.ScreenSwitches}

func main() {
	in := flag.String("in", "", "skill-mine JSONL")
	out := flag.String("out", "internal/skill/model.json", "model file")
	margin := flag.Float64("confident-margin", 0.3, "distance from every cut, in tiers, for a confident combined tier")
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
		mc := skill.MetricCuts{HigherIsBetter: rho >= 0, Rho: round(rho, 2), Cuts: map[string][3]float64{}}
		sign := 1.0
		if rho < 0 {
			sign = -1
		}
		for _, race := range []string{"T", "P", "Z"} {
			var scored []scoredTier
			for _, s := range all {
				if s.Side.Race == race {
					scored = append(scored, scoredTier{sign * s.Values[key], s.tier()})
				}
			}
			if len(scored) < 300 {
				continue
			}
			c := equalErrorCuts(scored)
			mc.Cuts[race] = [3]float64{round(sign*c[0], 3), round(sign*c[1], 3), round(sign*c[2], 3)}
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
		cr := fitRace(rows)
		var scored []scoredTier
		for _, s := range rows {
			scored = append(scored, scoredTier{predict(cr, s), s.tier()})
		}
		cr.Cuts = equalErrorCuts(scored)
		for i := range cr.Cuts {
			cr.Cuts[i] = round(cr.Cuts[i], 4)
		}
		model.Combined.Races[race] = cr
		log.Printf("%s: %d players, weights %v", race, len(rows), cr.Weights)
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

// fitRace is a ridge regression of the tier on clamped z-scores of the combined features.
func fitRace(rows []sample) skill.CombinedForRace {
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
	cr.Intercept = round(w[0], 5)
	for i := range cr.Weights {
		cr.Weights[i] = round(w[i+1], 5)
		cr.Mean[i] = round(cr.Mean[i], 4)
		cr.SD[i] = round(cr.SD[i], 4)
	}
	return cr
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
