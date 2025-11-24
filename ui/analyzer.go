package ui

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/marianogappa/screpdb/internal/models"
)

var templateStr = `Players:

{{- range . -}}
<b>{{.Name}}</b>
First attacking unit: {{.FirstAttackingUnitName}} created at {{.FirstAttackingUnitTime}}
First upgrade name: {{.FirstUpgradeName}} created at {{.FirstUpgradeTime}}
First expansion at: {{.FirstExpansionTime}}

{{end -}}`

func analyze(r *models.ReplayData) string {
	return mustExecuteTemplate(templateStr, analyzePlayers(r))
}

func mustExecuteTemplate(tpl string, data any) string {
	var buf bytes.Buffer
	template.Must(template.New("").Parse(tpl)).Execute(&buf, data)
	return buf.String()
}

type PlayerSection struct {
	Name                   string
	FirstAttackingUnitName string
	FirstAttackingUnitTime string
	FirstUpgradeName       string
	FirstUpgradeTime       string
	FirstExpansionTime     string
}

func timeToStr(t time.Time, startTime time.Time) string {
	return fmt.Sprintf("%v", t.Sub(startTime))
}

func analyzePlayers(r *models.ReplayData) []PlayerSection {
	pss := make([]PlayerSection, 0, len(r.Players))
	for _, p := range r.Players {
		pss = append(pss, analyzePlayer(r, p))
	}
	return pss
}

func analyzePlayer(r *models.ReplayData, player *models.Player) PlayerSection {
	ps := PlayerSection{Name: player.Name}
	for _, c := range r.Commands {
		if c.PlayerID != int64(player.PlayerID) {
			continue
		}
		if ps.FirstAttackingUnitName != "" && ps.FirstUpgradeName != "" && ps.FirstExpansionTime != "" {
			break
		}
		if ps.FirstAttackingUnitName == "" && c.IsAttackingUnitBuild() {
			ps.FirstAttackingUnitName = c.UnitBuildName()
			ps.FirstAttackingUnitTime = timeToStr(c.RunAt, r.Replay.ReplayDate)
		}
		if ps.FirstUpgradeName == "" && c.IsUpgrade() {
			ps.FirstUpgradeName = c.GetUpgradeName()
			ps.FirstUpgradeTime = timeToStr(c.RunAt, r.Replay.ReplayDate)
		}
		if ps.FirstExpansionTime == "" && c.IsBaseBuild() {
			ps.FirstExpansionTime = timeToStr(c.RunAt, r.Replay.ReplayDate)
		}
	}
	return ps
}
