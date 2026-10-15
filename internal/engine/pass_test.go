package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/marianogappa/openbw-go/bwgame"
)

var testData = sync.OnceValues(func() (*Data, error) { return LoadData(os.Getenv("OPENBW_DATA")) })

func passReplay(t *testing.T, name string) *Digest {
	t.Helper()
	if os.Getenv("OPENBW_DATA") == "" {
		t.Skip("OPENBW_DATA is not set; the engine needs the StarCraft game data files")
	}
	data, err := testData()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join("..", "testdata", "replays", name))
	if err != nil {
		t.Fatal(err)
	}
	d, err := Pass(b, data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// SomaJyJ.rep: a 1v1 ZvT on SC:R that the engine plays in sync to the end.
func TestPassOneVersusOne(t *testing.T) {
	d := passReplay(t, "SomaJyJ.rep")
	if d.EngineVersion != EngineVersion || d.LastFrame < d.EndFrame-2 {
		t.Fatalf("version %d, last frame %d of %d", d.EngineVersion, d.LastFrame, d.EndFrame)
	}
	if d.Confidence() != ConfidenceHigh {
		t.Fatalf("confidence low: %+v", d.Drift)
	}
	if len(d.Players) != 2 {
		t.Fatalf("players %+v", d.Players)
	}
	if want := len(d.Players) * int(d.LastFrame/EconomyEveryFrames); len(d.Economy) < want-len(d.Players) {
		t.Fatalf("economy samples %d, want about %d", len(d.Economy), want)
	}
	assertZergHatcheryMorph(t, d)
	assertProductionsLinkToCommands(t, d)
	assertEconomyIsPlausible(t, d)

	b, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, d) {
		t.Fatal("digest changed in an encode and decode round trip")
	}
	t.Logf("digest %d bytes, %d productions, %d deaths, %d economy samples", len(b), len(d.Productions), len(d.Deaths), len(d.Economy))
}

func assertZergHatcheryMorph(t *testing.T, d *Digest) {
	t.Helper()
	for _, p := range d.Productions {
		if p.Type == uint16(bwgame.UnitTypesZergHatchery) && p.From == uint16(bwgame.UnitTypesZergDrone) && p.Complete != NoFrame {
			if p.Complete-p.Start < 1000 {
				t.Fatalf("hatchery built in %d frames", p.Complete-p.Start)
			}
			return
		}
	}
	t.Fatal("no completed Drone to Hatchery morph")
}

func assertProductionsLinkToCommands(t *testing.T, d *Digest) {
	t.Helper()
	linked := 0
	for _, p := range d.Productions {
		if p.Cmd != NoCmd {
			linked++
		}
	}
	if linked*10 < len(d.Productions)*9 {
		t.Fatalf("%d of %d productions link to a command", linked, len(d.Productions))
	}
}

func assertEconomyIsPlausible(t *testing.T, d *Digest) {
	t.Helper()
	for _, s := range d.Economy {
		if s.Minerals < 0 || s.SupplyUsed > s.SupplyMax+16 || s.SupplyMax > 400 || s.Workers > 200 {
			t.Fatalf("implausible sample %+v", s)
		}
	}
	last := d.Economy[len(d.Economy)-1]
	if last.GatheredMinerals < 5000 {
		t.Fatalf("gathered only %d minerals by the end", last.GatheredMinerals)
	}
}

// bgh.rep: an 8-player Big Game Hunters game; team games desync far more often than 1v1 in openbw-go.
func TestPassTeamGameReportsDrift(t *testing.T) {
	if testing.Short() {
		t.Skip("an 8-player game takes about a minute")
	}
	d := passReplay(t, "bgh.rep")
	if len(d.Players) != 8 {
		t.Fatalf("players %+v", d.Players)
	}
	if d.Drift.Selects == 0 || d.Drift.LinkedCommands == 0 {
		t.Fatalf("drift not measured: %+v", d.Drift)
	}
	if d.Confidence() == ConfidenceLow && d.TrustedUntil() > d.LastFrame {
		t.Fatalf("low confidence without a trusted-until frame: %+v", d.Drift)
	}
	t.Logf("drift %+v confidence %d trusted until %d", d.Drift, d.Confidence(), d.TrustedUntil())
}

func TestPassRejectsGarbage(t *testing.T) {
	if os.Getenv("OPENBW_DATA") == "" {
		t.Skip("OPENBW_DATA is not set")
	}
	data, err := testData()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Pass([]byte("not a replay"), data); err == nil {
		t.Fatal("Pass accepted garbage")
	}
}
