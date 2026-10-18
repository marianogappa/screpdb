package dashboard

import (
	"math"

	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/hotkeystream"
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
// the engine ones (spending quotient, worker production, banked resources) only exist in engine mode.
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
	var humans []int
	for i := range rec.Players {
		if p := &rec.Players[i]; p.IsHuman() && !p.IsObserver() {
			humans = append(humans, i)
		}
	}
	switch {
	case len(humans) < 2:
		return
	case rec.MapKind == library.MapKindUseMapSettings:
		detail.SkillStatus = skillStatusCustom
		return
	case int(rec.Duration) < skill.MinGameSeconds:
		detail.SkillStatus = skillStatusTooShort
		return
	}
	detail.SkillStatus = skillStatusOK
	detail.SkillCaveats = []string{}
	if len(humans) != 2 {
		detail.SkillCaveats = append(detail.SkillCaveats, skillCaveatTeamGame)
	}
	if rec.MapKind == library.MapKindMoney {
		detail.SkillCaveats = append(detail.SkillCaveats, skillCaveatMoneyMap)
	}
	engine := map[uint8]library.EnginePlayer{}
	if rec.EngineData != nil && !rec.EngineData.LowConfidence {
		for _, p := range rec.EngineData.Players {
			engine[p.Slot] = p
		}
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
		values := skillValues(p, float64(rec.Duration)/60)
		if e, ok := engine[p.Slot]; ok {
			addFinite(values, skill.SpendingQuotient, e.SpendingQuotient)
			addFinite(values, skill.HallBusy, e.HallBusy)
			addFinite(values, skill.LarvaPerHatch, e.LarvaPerHatch)
			if e.Seconds > 0 {
				values[skill.BankedShare] = float64(e.FloatSeconds) / float64(e.Seconds)
			}
		}
		res := model.Evaluate(race, values)
		id := library.PlayerID(rec.ID, uint8(ordinal))
		for i := range detail.Players {
			if detail.Players[i].PlayerID == id {
				detail.Players[i].Skill = &workflowPlayerSkill{Combined: res.Combined, Confident: res.Confident, Metrics: res.Metrics}
			}
		}
	}
}

// skillValues are the fast-pass metrics, measured as scripts/skill-mine measured them.
func skillValues(p *library.Player, minutes float64) map[string]float64 {
	values := map[string]float64{skill.EAPM: float64(p.EAPM)}
	if p.Viewport > 0 {
		values[skill.ScreenSwitches] = float64(p.Viewport)
	}
	if events, err := hotkeystream.Decode(p.HotkeyStream); err == nil && len(events) > 0 && minutes > 0 {
		recalls := 0
		for _, e := range events {
			if e.Type == hotkeystream.TypeSelect {
				recalls++
			}
		}
		values[skill.HotkeyRecalls] = float64(recalls) / minutes
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
