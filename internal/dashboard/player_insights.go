package dashboard

import (
	"maps"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/skill"
)

// Insights are advice drawn from the viewer's own games against what progamers do. They are shown only to
// Advanced players: a progamer needs no advice, and a beginner's games have bigger problems. Every rule is
// strict, so an insight that shows is one the numbers say plainly.
//
// Pro references come from 2026 ladder games of 18 Korean progamer Terrans (549 games, cwal.gg), so every
// insight is about the viewer's Terran games for now.
type workflowPlayerInsight struct {
	Key      string                   `json:"key"`
	Games    int                      `json:"games"`
	Params   map[string]any           `json:"params"`
	Examples []workflowInsightExample `json:"examples"`
	// From is the date of the oldest game the insight reads.
	From string `json:"from"`
}

// workflowInsightExample is a recent game that shows an insight, opened on Tab.
type workflowInsightExample struct {
	ReplayID int64  `json:"replay_id"`
	Opponent string `json:"opponent"`
	Map      string `json:"map"`
	Won      bool   `json:"won"`
	Date     string `json:"date"`
	Tab      string `json:"tab,omitempty"`
}

const insightExamples = 3

const (
	proFloatSecondsPer10Min = 32
	floatMinGames           = 20
	floatMinGameSeconds     = 8 * 60
	floatMinSecondsPer10Min = 75
	floatTimesPro           = 3
	floatHeavySeconds       = 120
	floatSplitMinGames      = 8
	floatSplitMinGap        = 0.10

	openerMinMatchupGames = 30
	openerMinGames        = 12
	openerMinGap          = 0.25

	turretsMinGames      = 10
	turretsAbovePro      = 1
	turretLeadAbovePro   = 8
	proWorkersLostToMuta = 4.5
	workersSafeMargin    = 1
)

type insightGame struct {
	replay   *library.Replay
	oppName  string
	matchup  string
	won      bool
	opener   string
	engine   *library.EnginePlayer
	duration int
}

func (d *Dashboard) playerInsights(playerKey string, s *workflowPlayerSkillSummary) []workflowPlayerInsight {
	if d.library == nil || s == nil || s.Combined.Tier != skill.TierAdvanced {
		return nil
	}
	if name, err := d.playerNameForKey(playerKey); err != nil || !d.isYouPlayer(name) {
		return nil
	}
	games := insightGames(d.library.lib.View().PlayerGames(playerKey))
	var out []workflowPlayerInsight
	for _, rule := range []func([]insightGame) *workflowPlayerInsight{floatingInsight, openerInsight, earlyTurretsInsight} {
		if in := rule(games); in != nil {
			out = append(out, *in)
		}
	}
	return out
}

// insightGames are the player's decided 1v1 Terran games against a human, from the skill window.
func insightGames(refs []library.PlayerRef) []insightGame {
	var latest time.Time
	for _, ref := range refs {
		if ref.Replay.Date.After(latest) {
			latest = ref.Replay.Date
		}
	}
	var out []insightGame
	for _, ref := range refs {
		r, me := ref.Replay, ref.Player()
		if !r.Flags.Has(library.FlagIsOneOnOne) || me.IsObserver() || me.Race != library.RaceTerran || r.Date.Before(latest.Add(-skillWindow)) {
			continue
		}
		outcome := me.PlayerOutcome()
		if outcome != models.OutcomeWon && outcome != models.OutcomeLost {
			continue
		}
		opp := -1
		for i := range r.Players {
			if i != int(ref.Ordinal) && !r.Players[i].IsObserver() {
				opp = i
			}
		}
		if opp < 0 || !r.Players[opp].IsHuman() {
			continue
		}
		g := insightGame{
			replay:   r,
			oppName:  r.Players[opp].Name,
			matchup:  "Tv" + r.Players[opp].Race.String()[:1],
			won:      outcome == models.OutcomeWon,
			opener:   openerOf(r, opp),
			duration: int(r.Duration),
		}
		if r.EngineData != nil && !r.EngineData.LowConfidence {
			for i := range r.EngineData.Players {
				if r.EngineData.Players[i].Slot == me.Slot {
					g.engine = &r.EngineData.Players[i]
				}
			}
		}
		out = append(out, g)
	}
	return out
}

func openerOf(r *library.Replay, player int) string {
	for _, m := range r.Markers {
		name := library.Features.Name(m.Feature)
		if int(m.Player) == player && strings.HasPrefix(name, "bo_") && !catchAllOpener(name) {
			return name
		}
	}
	return ""
}

func catchAllOpener(key string) bool {
	return strings.HasSuffix(key, "_other") || strings.Contains(key, "fuzzy") || strings.Contains(key, "unresolved")
}

func floatingInsight(games []insightGame) *workflowPlayerInsight {
	var rates []float64
	type split struct{ heavyWins, heavy, restWins, rest int }
	byMatchup := map[string]*split{}
	for _, g := range games {
		if !longEngineGame(g) {
			continue
		}
		e := g.engine
		rates = append(rates, float64(e.FloatSeconds)/float64(e.Seconds)*600)
		s := byMatchup[g.matchup]
		if s == nil {
			s = &split{}
			byMatchup[g.matchup] = s
		}
		if e.FloatSeconds >= floatHeavySeconds {
			s.heavy++
			s.heavyWins += b2i(g.won)
		} else {
			s.rest++
			s.restWins += b2i(g.won)
		}
	}
	if len(rates) < floatMinGames {
		return nil
	}
	median := medianOf(rates)
	if median < floatMinSecondsPer10Min || median < floatTimesPro*proFloatSecondsPer10Min {
		return nil
	}
	in := &workflowPlayerInsight{Key: "floating", Games: len(rates), From: oldestOf(games, longEngineGame), Params: map[string]any{
		"seconds_per_10min":     int(math.Round(median)),
		"pro_seconds_per_10min": proFloatSecondsPer10Min,
	}}
	best, bestGap := "", floatSplitMinGap
	for _, mu := range slices.Sorted(maps.Keys(byMatchup)) {
		s := byMatchup[mu]
		if s.heavy < floatSplitMinGames || s.rest < floatSplitMinGames {
			continue
		}
		if gap := ratio(s.restWins, s.rest) - ratio(s.heavyWins, s.heavy); gap >= bestGap {
			best, bestGap = mu, gap
		}
	}
	in.Examples = examplesBy(games, "", func(g insightGame) (float64, bool) {
		e := g.engine
		return float64(0), e != nil && int(e.Seconds) >= floatMinGameSeconds && e.FloatSeconds >= floatHeavySeconds && !g.won
	})
	if best != "" {
		s := byMatchup[best]
		in.Params["matchup"] = best
		in.Params["win_floating"] = percent(s.heavyWins, s.heavy)
		in.Params["win_rest"] = percent(s.restWins, s.rest)
	}
	return in
}

func openerInsight(games []insightGame) *workflowPlayerInsight {
	type tally struct{ wins, games int }
	matchups := map[string]*tally{}
	openers := map[[2]string]*tally{}
	for _, g := range games {
		m := matchups[g.matchup]
		if m == nil {
			m = &tally{}
			matchups[g.matchup] = m
		}
		m.games++
		m.wins += b2i(g.won)
		if g.opener == "" {
			continue
		}
		k := [2]string{g.matchup, g.opener}
		o := openers[k]
		if o == nil {
			o = &tally{}
			openers[k] = o
		}
		o.games++
		o.wins += b2i(g.won)
	}
	keys := slices.Collect(maps.Keys(openers))
	sort.Slice(keys, func(i, j int) bool { return keys[i][0]+keys[i][1] < keys[j][0]+keys[j][1] })
	var in *workflowPlayerInsight
	bestGap := openerMinGap
	for _, k := range keys {
		o, m := openers[k], matchups[k[0]]
		rest := tally{m.wins - o.wins, m.games - o.games}
		if m.games < openerMinMatchupGames || o.games < openerMinGames || rest.games < openerMinGames {
			continue
		}
		gap := ratio(rest.wins, rest.games) - ratio(o.wins, o.games)
		if gap < bestGap {
			continue
		}
		bestGap = gap
		in = &workflowPlayerInsight{Key: "opener", Games: m.games, From: oldestOf(games, func(g insightGame) bool { return g.matchup == k[0] }), Params: map[string]any{
			"matchup":  k[0],
			"opener":   k[1],
			"win_vs":   percent(o.wins, o.games),
			"win_rest": percent(rest.wins, rest.games),
		}}
		matchup, opener := k[0], k[1]
		in.Examples = examplesBy(games, "build-orders", func(g insightGame) (float64, bool) {
			return 0, g.matchup == matchup && g.opener == opener && !g.won
		})
	}
	return in
}

func earlyTurretsInsight(games []insightGame) *workflowPlayerInsight {
	var turrets, leads, workers []float64
	for _, g := range games {
		if !mutasArrived(g) {
			continue
		}
		e := g.engine
		turrets = append(turrets, float64(e.TurretsAtMutaArrival))
		workers = append(workers, float64(e.WorkersLostToMutas))
		if e.TurretLeadOnMutas != library.NoTurretLead {
			leads = append(leads, float64(e.TurretLeadOnMutas))
		}
	}
	if len(turrets) < turretsMinGames || len(leads) < turretsMinGames {
		return nil
	}
	t, l, w := medianOf(turrets), medianOf(leads), medianOf(workers)
	if t < proTurretsAtArrival+turretsAbovePro || l < proTurretLeadSeconds+turretLeadAbovePro || w > proWorkersLostToMuta+workersSafeMargin {
		return nil
	}
	examples := examplesBy(games, "mutalisk-timing", func(g insightGame) (float64, bool) {
		e := g.engine
		if g.matchup != "TvZ" || e == nil || !e.MutaArrivalKnown || e.TurretLeadOnMutas == library.NoTurretLead {
			return 0, false
		}
		return float64(e.TurretLeadOnMutas), e.TurretLeadOnMutas >= proTurretLeadP75/2+proTurretLeadSeconds/2 && e.TurretsAtMutaArrival > proTurretsAtArrival
	})
	return &workflowPlayerInsight{Key: "early_turrets", Games: len(turrets), From: oldestOf(games, mutasArrived), Examples: examples, Params: map[string]any{
		"turrets":          roundHalf(t),
		"pro_turrets":      proTurretsAtArrival,
		"lead_seconds":     int(math.Round(l)),
		"pro_lead_seconds": proTurretLeadSeconds,
		"workers_lost":     roundHalf(w),
		"pro_workers_lost": proWorkersLostToMuta,
	}}
}

func medianOf(xs []float64) float64 {
	s := slices.Clone(xs)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func roundHalf(v float64) float64 { return math.Round(v*2) / 2 }

func ratio(a, b int) float64 { return float64(a) / float64(max(1, b)) }

func percent(a, b int) int { return int(math.Round(ratio(a, b) * 100)) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// examplesBy is the most recent of games that show an insight, the strongest first among equally recent ones.
func examplesBy(games []insightGame, tab string, show func(insightGame) (float64, bool)) []workflowInsightExample {
	type scored struct {
		g     insightGame
		score float64
	}
	var picked []scored
	for _, g := range games {
		if score, ok := show(g); ok {
			picked = append(picked, scored{g, score})
		}
	}
	sort.SliceStable(picked, func(i, j int) bool {
		a, b := picked[i].g.replay.Date, picked[j].g.replay.Date
		if !a.Equal(b) {
			return a.After(b)
		}
		return picked[i].score > picked[j].score
	})
	out := []workflowInsightExample{}
	for _, p := range picked[:min(len(picked), insightExamples)] {
		r := p.g.replay
		out = append(out, workflowInsightExample{
			ReplayID: r.ID, Opponent: p.g.oppName, Map: library.Strings.Name(r.Map), Won: p.g.won,
			Date: r.Date.UTC().Format(time.RFC3339), Tab: tab,
		})
	}
	return out
}

func longEngineGame(g insightGame) bool {
	return g.engine != nil && int(g.engine.Seconds) >= floatMinGameSeconds
}

func mutasArrived(g insightGame) bool {
	return g.matchup == "TvZ" && g.engine != nil && g.engine.MutaArrivalKnown
}

func oldestOf(games []insightGame, counted func(insightGame) bool) string {
	var oldest time.Time
	for _, g := range games {
		if counted(g) && (oldest.IsZero() || g.replay.Date.Before(oldest)) {
			oldest = g.replay.Date
		}
	}
	return oldest.UTC().Format(time.RFC3339)
}
