package dashboard

import (
	"sort"
	"time"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/skill"
)

// minSkillGames is how many rated games a player needs before their page shows a Skill average: one game
// is a guess, a handful starts to say something about the player.
const minSkillGames = 5

// workflowPlayerSkillSummary averages a player's rated games: positions on the tier axis, so a Zerg's
// numbers and a Terran's average on one scale.
type workflowPlayerSkillSummary struct {
	Games    int                        `json:"games"`
	Combined skill.Placement            `json:"combined"`
	Metrics  map[string]skill.Placement `json:"metrics"`
	Pills    []workflowSkillPill        `json:"pills"`
	Model    *workflowSkillModel        `json:"model"`
	// Timeline is every rated game, oldest first, for the evolution chart.
	Timeline []workflowSkillPoint `json:"timeline"`
}

type workflowSkillPoint struct {
	ReplayID int64   `json:"replay_id"`
	Date     string  `json:"date"`
	Position float64 `json:"position"`
}

// workflowSkillPill is a habit seen in most of a player's rated games.
type workflowSkillPill struct {
	Key   string `json:"key"`
	Games int    `json:"games"`
	Of    int    `json:"of"`
}

// skillHabits are the habits that set players apart in the ladder games: each held in a fifth or more of one
// tier's games and almost none of another's. A habit becomes a pill when most of a player's games show it.
var skillHabits = []struct {
	key  string
	seen func(v map[string]float64) (bool, bool)
}{
	{"no_hotkeys", func(v map[string]float64) (bool, bool) {
		used, ok1 := v[skill.UsedRecalls]
		checks, ok2 := v[skill.CheckRecalls]
		return used+checks < 1, ok1 && ok2
	}},
	{"never_checks_hotkeys", func(v map[string]float64) (bool, bool) {
		used, ok1 := v[skill.UsedRecalls]
		checks, ok2 := v[skill.CheckRecalls]
		return used >= 3 && checks < 1, ok1 && ok2
	}},
	{"hotkey_cycler", func(v map[string]float64) (bool, bool) {
		checks, ok := v[skill.CheckRecalls]
		return checks >= 100, ok
	}},
	{"nonstop_production", func(v map[string]float64) (bool, bool) {
		cadence, ok := v[skill.ArmyCadence]
		return cadence >= 0.55, ok
	}},
	{"banks_resources", func(v map[string]float64) (bool, bool) {
		unspent, ok := v[skill.UnspentPerIncome]
		return unspent >= 1, ok
	}},
}

const habitShare = 0.6

func (d *Dashboard) playerSkill(playerKey string) *workflowPlayerSkillSummary {
	return d.playerSkillAs(playerKey, "")
}

// playerSkillAs is playerSkill over the games a player played as race only, or every game when race is empty.
func (d *Dashboard) playerSkillAs(playerKey, race string) *workflowPlayerSkillSummary {
	if d.library == nil {
		return nil
	}
	model, err := skill.Load()
	if err != nil {
		crashreport.Handle(err, nil, false)
		return nil
	}
	var rated []ratedPlayer
	var timeline []workflowSkillPoint
	for _, ref := range d.library.lib.View().PlayerGames(playerKey) {
		if ref.Player().IsObserver() || race != "" && ref.Player().Race.String() != race {
			continue
		}
		if r, ok := rateGame(model, ref.Replay).players[int(ref.Ordinal)]; ok && r.result.Combined != nil {
			rated = append(rated, r)
			timeline = append(timeline, workflowSkillPoint{ReplayID: ref.Replay.ID, Date: ref.Replay.Date.UTC().Format(time.RFC3339), Position: r.result.Combined.Position})
		}
	}
	out := summarizeSkill(rated)
	if out == nil {
		return nil
	}
	sort.SliceStable(timeline, func(i, j int) bool { return timeline[i].Date < timeline[j].Date })
	out.Model, out.Timeline = skillModelSummary(model), timeline
	return out
}

func summarizeSkill(rated []ratedPlayer) *workflowPlayerSkillSummary {
	if len(rated) < minSkillGames {
		return nil
	}
	out := &workflowPlayerSkillSummary{Games: len(rated), Metrics: map[string]skill.Placement{}, Pills: []workflowSkillPill{}}
	var combined []skill.Placement
	byMetric := map[string][]skill.Placement{}
	for _, r := range rated {
		combined = append(combined, *r.result.Combined)
		for k, p := range r.result.Metrics {
			byMetric[k] = append(byMetric[k], p)
		}
	}
	out.Combined = gamesPlacement(combined)
	for k, ps := range byMetric {
		out.Metrics[k] = gamesPlacement(ps)
	}
	for _, h := range skillHabits {
		held, measured := 0, 0
		for _, r := range rated {
			if yes, ok := h.seen(r.values); ok {
				measured++
				if yes {
					held++
				}
			}
		}
		if measured >= minSkillGames && float64(held) >= habitShare*float64(measured) {
			out.Pills = append(out.Pills, workflowSkillPill{Key: h.key, Games: held, Of: measured})
		}
	}
	return out
}

// gamesPlacement is the median of a player's game placements, with the tier its position falls in. Per-game
// positions stop at the axis ends, so a mean would pull strong and weak players toward the middle tiers.
func gamesPlacement(ps []skill.Placement) skill.Placement {
	m := medianPlacement(ps)
	return skill.Placement{Value: m.Value, Position: m.Position, Tier: min(skill.TierPro, int(m.Position))}
}

// workflowSkillModel is what the Skill explainer shows of the model: how many ladder games it was fitted
// on, how closely each metric follows MMR, and each metric's share of the estimate per race.
type workflowSkillModel struct {
	Games   int                   `json:"games"`
	Players int                   `json:"players"`
	Metrics []workflowSkillMetric `json:"metrics"`
}

type workflowSkillMetric struct {
	Key            string             `json:"key"`
	Rho            float64            `json:"rho"`
	HigherIsBetter bool               `json:"higher_is_better"`
	Share          map[string]float64 `json:"share"`
}

func skillModelSummary(m *skill.Model) *workflowSkillModel {
	out := &workflowSkillModel{Games: m.Games, Players: m.Players}
	totals := map[string]float64{}
	for race, cr := range m.Combined.Races {
		for _, w := range cr.Weights {
			totals[race] += max(w, -w)
		}
	}
	for i, key := range m.Combined.Features {
		mc := m.Metrics[key]
		metric := workflowSkillMetric{Key: key, Rho: mc.Rho, HigherIsBetter: mc.HigherIsBetter, Share: map[string]float64{}}
		for race, cr := range m.Combined.Races {
			if totals[race] > 0 && i < len(cr.Weights) {
				metric.Share[race] = max(cr.Weights[i], -cr.Weights[i]) / totals[race]
			}
		}
		out.Metrics = append(out.Metrics, metric)
	}
	return out
}
