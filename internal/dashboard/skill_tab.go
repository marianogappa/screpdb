package dashboard

import (
	"math"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/skill"
)

// Skill status of a game. The model was fitted on 1v1 ladder games of 8:00 or more on regular maps; team
// games and money maps are still rated, with caveats the tab shows, but short and custom-rules games are not.
const (
	skillStatusOK       = "ok"
	skillStatusTooShort = "too_short"
	skillStatusCustom   = "custom_rules"

	skillCaveatTeamGame = "team_game"
	skillCaveatMoneyMap = "money_map"
)

// workflowPlayerSkill places one player's game on the four skill tiers. Metrics hold every measured metric;
// the engine ones (unspent resources against income, idle production) only exist in engine mode.
type workflowPlayerSkill struct {
	Combined  *skill.Placement           `json:"combined,omitempty"`
	Confident bool                       `json:"confident"`
	Metrics   map[string]skill.Placement `json:"metrics"`
}

func (d *Dashboard) populateSkill(detail *workflowGameDetail) {
	model, err := skill.Load()
	if err != nil {
		crashreport.Handle(err, nil, false)
		return
	}
	if d.library == nil {
		return
	}
	rec, ok := d.library.lib.Snapshot().ByID(detail.ReplayID)
	if !ok {
		return
	}
	g := rateGame(model, rec)
	detail.SkillStatus, detail.SkillCaveats = g.status, g.caveats
	if g.status == skillStatusOK {
		detail.SkillModel = skillModelSummary(model)
	}
	for ordinal, r := range g.players {
		id := library.PlayerID(rec.ID, uint8(ordinal))
		for i := range detail.Players {
			if detail.Players[i].PlayerID == id {
				detail.Players[i].Skill = &workflowPlayerSkill{Combined: r.result.Combined, Confident: r.result.Confident, Metrics: r.result.Metrics}
			}
		}
	}
}

type ratedGame struct {
	status  string
	caveats []string
	players map[int]ratedPlayer
}

type ratedPlayer struct {
	values map[string]float64
	result skill.Result
}

// rateGame rates every human of a game with trusted engine data; status is empty when the game has no
// Skill tab at all.
func rateGame(model *skill.Model, rec *library.Replay) ratedGame {
	var humans []int
	for i := range rec.Players {
		if p := &rec.Players[i]; p.IsHuman() && !p.IsObserver() {
			humans = append(humans, i)
		}
	}
	// Skill is an engine feature: without trusted engine data the game simply has no Skill tab.
	if rec.EngineData == nil || rec.EngineData.LowConfidence || len(humans) < 2 {
		return ratedGame{}
	}
	switch {
	case rec.MapKind == library.MapKindUseMapSettings:
		return ratedGame{status: skillStatusCustom}
	case int(rec.Duration) < skill.MinGameSeconds:
		return ratedGame{status: skillStatusTooShort}
	}
	g := ratedGame{status: skillStatusOK, caveats: []string{}, players: map[int]ratedPlayer{}}
	if len(humans) != 2 {
		g.caveats = append(g.caveats, skillCaveatTeamGame)
	}
	if rec.MapKind == library.MapKindMoney {
		g.caveats = append(g.caveats, skillCaveatMoneyMap)
	}
	engine := map[uint8]library.EnginePlayer{}
	for _, p := range rec.EngineData.Players {
		engine[p.Slot] = p
	}
	for _, ordinal := range humans {
		p := &rec.Players[ordinal]
		if p.APM < skill.MinAPM {
			continue
		}
		race := skillRace(p.Race)
		if race == "" {
			continue
		}
		values := skillValues(p)
		if e, ok := engine[p.Slot]; ok {
			addFinite(values, skill.UnspentPerIncome, e.UnspentPerIncome)
			addFinite(values, skill.ProductionIdle, e.ProductionIdleShare)
		}
		g.players[ordinal] = ratedPlayer{values: values, result: model.Evaluate(race, skillContext(rec, len(humans)), values)}
	}
	return g
}

// skillValues are the fast-pass metrics, measured as scripts/skill-mine measured them.
func skillValues(p *library.Player) map[string]float64 {
	values := map[string]float64{skill.EAPM: float64(p.EAPM)}
	if p.Flags.Has(library.PlayerHasSkillCommands) {
		values[skill.UsedRecalls] = float64(p.UsedRecalls)
		values[skill.CheckRecalls] = float64(p.CheckRecalls)
		values[skill.ArmyCadence] = float64(p.ArmyCadence)
	}
	return values
}

func addFinite(values map[string]float64, key string, v float32) {
	if f := float64(v); !math.IsNaN(f) && !math.IsInf(f, 0) {
		values[key] = f
	}
}

func skillRace(r library.Race) string {
	switch r {
	case library.RaceTerran:
		return "T"
	case library.RaceProtoss:
		return "P"
	case library.RaceZerg:
		return "Z"
	}
	return ""
}

func skillContext(rec *library.Replay, humans int) string {
	switch {
	case rec.MapKind == library.MapKindMoney:
		return skill.ContextMoney
	case humans != 2:
		return skill.ContextTeamRegular
	}
	return skill.ContextOneVsOne
}
