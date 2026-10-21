package macro

import (
	"math"
	"testing"

	"github.com/marianogappa/screpdb/internal/engine"
)

func TestDeadBasesStopCountingForWorkerProduction(t *testing.T) {
	noDrift := engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame}

	terranGame := &engine.Digest{
		LastFrame: f(700),
		Players:   []engine.Player{{Slot: 0, Race: terran}},
		Initial:   []engine.InitialUnit{{Unit: 1, Type: cc, Owner: 0}},
		Productions: []engine.Production{
			{Owner: 0, Type: cc, Unit: 2, Producer: 9, Start: f(0), Complete: f(120), Cancel: engine.NoFrame},
			{Owner: 0, Type: scv, Unit: 3, Producer: 1, Start: f(120), Complete: f(600), Cancel: engine.NoFrame},
		},
		Deaths: []engine.Death{{Unit: 2, Frame: f(240), Owner: 0, Type: cc}},
		Drift:  noDrift,
	}
	w, ok := WorkerProductionOf(terranGame, 0, WorkerWindowEnd)
	if !ok || math.Abs(w.HallBusy-0.8) > 1e-9 {
		t.Fatalf("hall busy %v ok %v, want 0.8 (480 s busy of 480 + 120 s alive)", w.HallBusy, ok)
	}

	zergGame := &engine.Digest{
		LastFrame: f(700),
		Players:   []engine.Player{{Slot: 1, Race: zerg}},
		Initial:   []engine.InitialUnit{{Unit: 1, Type: hatch, Owner: 1}},
		Productions: []engine.Production{
			{Owner: 1, Type: hatch, Unit: 2, Producer: 9, Start: f(0), Complete: f(120), Cancel: engine.NoFrame},
		},
		Deaths: []engine.Death{{Unit: 2, Frame: f(240), Owner: 1, Type: hatch}},
		Economy: steadyEconomy(1, 700, func(_ int, s *engine.EconomySample) {
			s.Larva = 2
		}),
		Drift: noDrift,
	}
	w, ok = WorkerProductionOf(zergGame, 1, WorkerWindowEnd)
	if !ok || math.Abs(w.LarvaPerHatchery-1.75) > 1e-9 {
		t.Fatalf("larva per hatchery %v ok %v, want 1.75 (1 per hatchery for 120 s, then 2 for 360 s)", w.LarvaPerHatchery, ok)
	}
}
