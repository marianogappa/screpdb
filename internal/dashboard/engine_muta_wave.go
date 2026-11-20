//go:build !js

package dashboard

import (
	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/moments"
)

func (d *Dashboard) applyEngineMutaWave(detail *workflowGameDetail) {
	if detail.MutaliskTimingSummary == nil {
		return
	}
	dg, _ := d.engineDigest(detail.ReplayID)
	if dg == nil {
		return
	}
	zerg, terran, ok := moments.OneVsOneTvZ(dg)
	if !ok {
		return
	}
	w := moments.DetectMutaWave(dg, zerg, terran)
	if w == nil {
		return
	}
	detail.MutaliskTimingSummary.Engine = mutaWavePayload(w)
}

func mutaWavePayload(w *moments.MutaWave) *workflowMutaWave {
	out := &workflowMutaWave{
		FirstHatchSecond:     frameSecond(w.FirstHatch),
		Arrived:              w.Arrival != engine.NoFrame,
		MutasAtArrival:       w.MutasAtArrival,
		TurretsUp:            w.TurretsUp,
		TurretsBuilding:      w.TurretsBuilding,
		TurretReadySeconds:   []int64{},
		Wave:                 mutaLedgerPayload(w.Wave),
		Game:                 mutaLedgerPayload(w.Game),
		Kills:                []workflowMutaKill{},
		ProTravelSeconds:     proMutaTravelSeconds,
		ProTurretsAtArrival:  [3]int{proTurretsAtArrivalP25, proTurretsAtArrival, proTurretsAtArrivalP75},
		ProTurretLeadSeconds: [3]int{proTurretLeadP25, proTurretLeadSeconds, proTurretLeadP75},
	}
	if out.Arrived {
		out.ArrivalSecond = frameSecond(w.Arrival)
	}
	for _, f := range w.TurretsReady {
		out.TurretReadySeconds = append(out.TurretReadySeconds, frameSecond(f))
	}
	for _, k := range w.Kills {
		name := ""
		if u := repcmd.UnitByID(k.Type); u != nil {
			name = u.Name
		}
		out.Kills = append(out.Kills, workflowMutaKill{Second: frameSecond(k.Frame), Unit: name, Muta: k.Muta})
	}
	return out
}

func mutaLedgerPayload(l moments.MutaLedger) workflowMutaLedger {
	return workflowMutaLedger{
		Workers: l.Workers, Turrets: l.Turrets, OtherBuildings: l.OtherBuildings, Units: l.Units,
		KilledValue: l.Killed.Value(), MutasLost: l.MutasLost, MutasLostToTurrets: l.MutasLostToTurrets, LostValue: l.Lost.Value(),
	}
}
