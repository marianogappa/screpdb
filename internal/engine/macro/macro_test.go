package macro

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/marianogappa/screpdb/internal/engine"
)

var testData = sync.OnceValues(func() (*engine.Data, error) { return engine.LoadData(os.Getenv("OPENBW_DATA")) })

func digestOf(t *testing.T, name string) *engine.Digest {
	t.Helper()
	if os.Getenv("OPENBW_DATA") == "" {
		t.Skip("OPENBW_DATA is not set; the engine needs the StarCraft game data files")
	}
	data, err := testData()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "replays", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := engine.Pass(context.Background(), b, data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestRealGamesLog(t *testing.T) {
	for _, name := range []string{"SomaJyJ.rep", "SomaTyson.rep", "Somavssnow.rep"} {
		for _, p := range Compute(digestOf(t, name)) {
			t.Logf("%s slot %d race %d secs %d blocked %ds blocks %d per100 %.2f spare %.1f SQ %.0f income %.0f unspent %.0f workerIdle %.2f prodIdle %.2f income %v",
				name, p.Slot, p.Race, p.Seconds, p.SupplyBlockedSeconds, p.SupplyBlocks, p.BlocksPer100Supply, p.SpareSupply, p.SpendingQuotient, p.IncomePerMinute, p.AverageUnspent, p.WorkerIdleShare, p.ProductionIdleShare, p.Income)
		}
	}
}

const (
	terran = 1
	zerg   = 0
	cc     = 106
	rax    = 111
	scv    = 7
	marine = 0
	hatch  = 131
)

// steadyEconomy is one sample per second: 300 minerals banked, 10 minerals gathered per second, 20 of 40
// supply used, 20 workers; edit returns per-second changes.
func steadyEconomy(slot int8, seconds int, edit func(i int, s *engine.EconomySample)) []engine.EconomySample {
	var out []engine.EconomySample
	for i := range seconds {
		s := engine.EconomySample{Frame: int32((i + 1) * framesPerSecond), Owner: slot, Minerals: 300, GatheredMinerals: int32(10 * i), SupplyUsed: 40, SupplyMax: 80, Workers: 20}
		if edit != nil {
			edit(i, &s)
		}
		out = append(out, s)
	}
	return out
}

func f(second int) int32 { return int32(second * framesPerSecond) }

func TestTerranMacro(t *testing.T) {
	d := &engine.Digest{
		LastFrame: f(600),
		Players:   []engine.Player{{Slot: 0, Race: terran}},
		Initial:   []engine.InitialUnit{{Unit: 1, Type: cc, Owner: 0}},
		Productions: []engine.Production{
			{Owner: 0, Type: rax, Unit: 2, Producer: 9, Start: f(0), Complete: f(100), Cancel: engine.NoFrame},
			{Owner: 0, Type: scv, Unit: 3, Producer: 1, Start: f(0), Complete: f(300), Cancel: engine.NoFrame},
			{Owner: 0, Type: marine, Unit: 4, Producer: 2, Start: f(100), Complete: f(400), Cancel: engine.NoFrame},
		},
		Economy: steadyEconomy(0, 600, func(i int, s *engine.EconomySample) {
			if i >= 450 && i < 460 {
				s.SupplyUsed = s.SupplyMax
			}
		}),
		Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame},
	}
	got := Compute(d)
	if len(got) != 1 {
		t.Fatalf("players %+v", got)
	}
	p := got[0]
	if p.SupplyBlockedSeconds != 10 || p.SupplyBlocks != 1 {
		t.Fatalf("blocked %ds in %d blocks, want 10s in 1", p.SupplyBlockedSeconds, p.SupplyBlocks)
	}
	if want := 1 / (40.0 / 100); math.Abs(p.BlocksPer100Supply-want) > 1e-9 {
		t.Fatalf("blocks per 100 supply %v, want %v", p.BlocksPer100Supply, want)
	}
	if math.Abs(p.WorkerIdleShare-0.5) > 0.01 {
		t.Fatalf("worker idle %v, want 0.5 (the CC trained for 300 of 600 s)", p.WorkerIdleShare)
	}
	if math.Abs(p.ProductionIdleShare-0.4) > 0.01 {
		t.Fatalf("production idle %v, want 0.4 (the Barracks idled 200 of its 500 s)", p.ProductionIdleShare)
	}
	if math.Abs(p.IncomePerMinute-600) > 2 || math.Abs(p.AverageUnspent-300) > 1e-9 {
		t.Fatalf("income %v unspent %v", p.IncomePerMinute, p.AverageUnspent)
	}
	if want := 35*(0.00137*p.IncomePerMinute-math.Log(300)) + 240; math.Abs(p.SpendingQuotient-want) > 1e-9 {
		t.Fatalf("SQ %v, want %v", p.SpendingQuotient, want)
	}
	if len(p.Income) != 9 || p.Income[0] != 600 {
		t.Fatalf("income curve %v", p.Income)
	}
}

func TestZergLarvaCap(t *testing.T) {
	d := &engine.Digest{
		LastFrame: f(400),
		Players:   []engine.Player{{Slot: 1, Race: zerg}},
		Initial:   []engine.InitialUnit{{Unit: 1, Type: hatch, Owner: 1}},
		Economy: steadyEconomy(1, 400, func(i int, s *engine.EconomySample) {
			s.Larva = 1
			if i < 100 {
				s.Larva = 3
			}
		}),
		Drift: engine.Drift{DesyncFrame: engine.NoFrame, ErrorFrame: engine.NoFrame},
	}
	p := Compute(d)[0]
	if math.Abs(p.ProductionIdleShare-0.25) > 0.01 || !math.IsNaN(p.WorkerIdleShare) {
		t.Fatalf("larva capped %v worker idle %v, want 0.25 and NaN", p.ProductionIdleShare, p.WorkerIdleShare)
	}
}

func TestShortOrUntrustedGamesAreSkipped(t *testing.T) {
	d := &engine.Digest{
		LastFrame: f(900),
		Players:   []engine.Player{{Slot: 0, Race: terran}, {Slot: 1, Race: terran}},
		Economy:   append(steadyEconomy(0, 900, nil), steadyEconomy(1, 900, nil)...),
		Outcomes:  []engine.Outcome{{Frame: f(100), Owner: 1, Kind: engine.OutcomeLeft}},
		Drift:     engine.Drift{DesyncFrame: f(500), ErrorFrame: engine.NoFrame},
	}
	got := Compute(d)
	if len(got) != 1 || got[0].Slot != 0 || got[0].Seconds > 500 {
		t.Fatalf("want only slot 0, cut at the desync: %+v", got)
	}
}
