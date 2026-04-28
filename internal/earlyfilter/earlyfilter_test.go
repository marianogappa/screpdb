package earlyfilter

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// secondToFrame converts an in-game second to the equivalent fastest-speed
// frame. Used by tests that author commands at human-readable times.
func secondFrame(sec float64) int32 {
	return int32(sec * 1000.0 / float64(fastestFrameMs))
}

func makeCmd(action, subject string, second int, player *models.Player) *models.Command {
	subj := subject
	return &models.Command{
		ActionType:           action,
		UnitType:             &subj,
		Frame:                secondFrame(float64(second)),
		SecondsFromGameStart: second,
		Player:               player,
	}
}

func protossPlayer() *models.Player {
	return &models.Player{PlayerID: 1, Race: "Protoss"}
}

func terranPlayer() *models.Player {
	return &models.Player{PlayerID: 2, Race: "Terran"}
}

// TestProbeSpamDroppedForResources sends 20 Probe trains in the first 10
// seconds. The starting player has 50m and gathers ~4.5m/s, so far fewer
// than 20 should fit in the supply/mineral budget.
func TestProbeSpamDroppedForResources(t *testing.T) {
	p := protossPlayer()
	var cmds []*models.Command
	for i := 0; i < 20; i++ {
		cmds = append(cmds, makeCmd("Train", models.GeneralUnitProbe, i/2, p))
	}
	res := Apply(&models.Replay{}, []*models.Player{p}, nil, cmds, Options{})
	kept := 0
	for _, c := range res.Commands {
		en, _ := cmdenrich.Classify(c)
		if en.Subject == models.GeneralUnitProbe {
			kept++
		}
	}
	// Hard upper bound: 9-supply cap means at most 5 Probes can be queued
	// before a Pylon completes, and we have no Pylon in this fixture. So
	// kept must be ≤ 5.
	if kept > 5 {
		t.Fatalf("expected ≤5 Probes kept, got %d", kept)
	}
	if kept == 0 {
		t.Fatalf("expected at least 1 Probe kept (initial 50m affords one)")
	}
}

// TestEvolutionChamberDroppedUnconditionally — Zerg Ev Chamber early should
// always be dropped regardless of resources.
func TestEvolutionChamberDroppedUnconditionally(t *testing.T) {
	p := &models.Player{PlayerID: 1, Race: "Zerg"}
	// Give the player time to gather plenty of minerals.
	cmds := []*models.Command{
		makeCmd("Build", models.GeneralUnitEvolutionChamber, 200, p),
	}
	res := Apply(&models.Replay{}, []*models.Player{p}, nil, cmds, Options{})
	if len(res.Commands) != 0 {
		t.Fatalf("expected Evolution Chamber dropped, got %d kept", len(res.Commands))
	}
}

// TestPastWindowPassThrough — commands beyond the 5-minute window are kept
// as-is, no matter what.
func TestPastWindowPassThrough(t *testing.T) {
	p := protossPlayer()
	cmds := []*models.Command{
		makeCmd("Build", models.GeneralUnitGateway, 600, p), // 10 minutes
	}
	res := Apply(&models.Replay{}, []*models.Player{p}, nil, cmds, Options{})
	if len(res.Commands) != 1 {
		t.Fatalf("expected past-window command kept, got %d", len(res.Commands))
	}
}

// TestZealotImpliesGateway — A Zealot kept at second 200 should force a
// Gateway re-admission via backtrack (and a Pylon re-admission too, since
// Gateway prereq is Pylon). Workers will be force-dropped to fund them.
func TestZealotImpliesGateway(t *testing.T) {
	p := protossPlayer()
	// Construct a stream where:
	// - a flood of Probe trains at seconds 0..120 spends all minerals
	// - a Pylon at second 60 (will get dropped: out of minerals)
	// - a Gateway at second 100 (will get dropped: missing Pylon prereq)
	// - a Zealot at second 200 (will be kept-by-default, but no Gateway)
	var cmds []*models.Command
	for i := 0; i < 30; i++ {
		cmds = append(cmds, makeCmd("Train", models.GeneralUnitProbe, i*4, p))
	}
	cmds = append(cmds,
		makeCmd("Build", models.GeneralUnitPylon, 60, p),
		makeCmd("Build", models.GeneralUnitGateway, 100, p),
		makeCmd("Train", models.GeneralUnitZealot, 200, p),
	)

	res := Apply(&models.Replay{}, []*models.Player{p}, nil, cmds, Options{})

	// Zealot must be kept (it's the consequent that drives backtrack).
	zealots, gateways, pylons := 0, 0, 0
	for _, c := range res.Commands {
		en, _ := cmdenrich.Classify(c)
		switch en.Subject {
		case models.GeneralUnitZealot:
			zealots++
		case models.GeneralUnitGateway:
			gateways++
		case models.GeneralUnitPylon:
			pylons++
		}
	}
	if zealots != 1 {
		t.Fatalf("expected Zealot kept (consequent), got %d", zealots)
	}
	if gateways != 1 {
		t.Fatalf("expected Gateway kept (forward) or re-admitted (backtrack), got %d", gateways)
	}
	if pylons != 1 {
		t.Fatalf("expected Pylon kept (forward) or re-admitted (backtrack), got %d", pylons)
	}
}

// TestPhotonCannonImpliesForge — kept Photon Cannon proves Forge existed.
// If forward pass drops the Forge for resources, backtrack must re-admit
// it. The user's bug report: Photon Cannon was dropping with reason
// "missing_prereq:Forge" — the engine never permits that, so the policy
// is wrong; the Cannon itself is evidence.
func TestPhotonCannonImpliesForge(t *testing.T) {
	p := protossPlayer()
	// Force minerals to be tight: spam probes early. Then issue Pylon,
	// Forge, Photon Cannon at increasingly-late seconds. The forward
	// pass may drop Forge for resources; backtrack must re-admit.
	var cmds []*models.Command
	for i := 0; i < 6; i++ {
		cmds = append(cmds, makeCmd("Train", models.GeneralUnitProbe, i*3, p))
	}
	cmds = append(cmds,
		makeCmd("Build", models.GeneralUnitPylon, 30, p),
		makeCmd("Build", models.GeneralUnitForge, 90, p),
		makeCmd("Build", models.GeneralUnitPhotonCannon, 130, p),
	)
	res := Apply(&models.Replay{}, []*models.Player{p}, nil, cmds, Options{})

	cannons, forges, pylons := 0, 0, 0
	for _, c := range res.Commands {
		en, _ := cmdenrich.Classify(c)
		switch en.Subject {
		case models.GeneralUnitPhotonCannon:
			cannons++
		case models.GeneralUnitForge:
			forges++
		case models.GeneralUnitPylon:
			pylons++
		}
	}
	if cannons != 1 {
		t.Fatalf("Photon Cannon must never be dropped for prereq reasons; kept=%d", cannons)
	}
	if forges != 1 {
		t.Fatalf("Forge required (consequent of kept Cannon); kept=%d", forges)
	}
	if pylons != 1 {
		t.Fatalf("Pylon required (transitive prereq of Cannon); kept=%d", pylons)
	}
}

// TestGasGatherSubtractsFromMineralIncome — when a Harvest1 order targets
// a geyser the player owns a built Refinery on, 3 workers leave the
// mineral line for ~43s. Mineral count at end should be lower than the
// no-gas baseline by roughly that period of 3-worker income.
func TestGasGatherSubtractsFromMineralIncome(t *testing.T) {
	p := terranPlayer()
	bx, by := 9, 3 // tile coords; centre = (9*32+64, 3*32+32) = (352, 128)
	geysers := &models.ReplayMapContext{
		Geysers: []models.MapResourcePosition{{X: 352, Y: 128}},
	}

	// Build Refinery at second 60 — enough mineral income to afford the
	// 100m cost (Terran starts 50m, gathers ~4.3m/s → 50+260=310m by s=60).
	mkRef := func() *models.Command {
		c := makeCmd("Build", models.GeneralUnitRefinery, 60, p)
		x, y := bx, by
		c.X, c.Y = &x, &y
		return c
	}
	// Harvest1 at pixel near geyser, after Refinery completes (60+25=85s).
	mkGather := func(sec int) *models.Command {
		c := &models.Command{
			ActionType:           "Targeted Order",
			Frame:                secondFrame(float64(sec)),
			SecondsFromGameStart: sec,
			Player:               p,
		}
		on := models.UnitOrderHarvest1
		c.OrderName = &on
		x, y := 350, 130
		c.X, c.Y = &x, &y
		return c
	}

	tmp := t.TempDir()
	withGas := []*models.Command{mkRef(), mkGather(90)}
	resGas := Apply(&models.Replay{FileChecksum: "gas"}, []*models.Player{p}, geysers, withGas, Options{MaxSecond: 180, DebugDir: tmp})

	withoutGas := []*models.Command{mkRef()}
	resNoGas := Apply(&models.Replay{FileChecksum: "nogas"}, []*models.Player{p}, geysers, withoutGas, Options{MaxSecond: 180, DebugDir: tmp})

	// Both traces will have ticks at fixed cadence; pull final tick mineral
	// counts and verify gas case is materially lower.
	gasFinalMin := finalTickMinerals(t, resGas)
	noGasFinalMin := finalTickMinerals(t, resNoGas)

	// 3 workers * 65/min * (43/60) = ~140 minerals difference at minimum.
	// Use a conservative threshold (50) to allow for sim quantisation.
	if noGasFinalMin-gasFinalMin < 50 {
		t.Fatalf("gas case minerals (%d) should be at least 50 less than no-gas (%d)", gasFinalMin, noGasFinalMin)
	}
}

func finalTickMinerals(t *testing.T, r Result) int {
	t.Helper()
	if r.Trace == nil || len(r.Trace.Players) == 0 {
		t.Fatalf("expected trace with player data; got %+v", r.Trace)
	}
	ticks := r.Trace.Players[0].Ticks
	if len(ticks) == 0 {
		t.Fatalf("expected at least one tick")
	}
	return ticks[len(ticks)-1].Minerals
}

// TestLarvaCapsZergMorphs — Hatchery starts with 3 larva and regenerates
// 1 per 14.4s (cap 3). A flood of Drone Unit Morph commands must be
// rejected after the available larva is exhausted, even if minerals
// are infinite.
func TestLarvaCapsZergMorphs(t *testing.T) {
	p := &models.Player{PlayerID: 1, Race: "Zerg"}
	// 30 morph attempts in the first 60s. Hatchery has 3 larva at start;
	// regen rate ~1 / 14.4s → only ~3 + 4 = 7 should be admissible.
	var cmds []*models.Command
	for i := 0; i < 30; i++ {
		cmds = append(cmds, makeCmd("Unit Morph", models.GeneralUnitDrone, 1+i*2, p))
	}
	res := Apply(&models.Replay{}, []*models.Player{p}, nil, cmds, Options{MaxSecond: 120})
	kept := 0
	for _, c := range res.Commands {
		en, _ := cmdenrich.Classify(c)
		if en.Subject == models.GeneralUnitDrone {
			kept++
		}
	}
	// 60s window, 3 starting + floor(60/14.4) = 4 spawns = 7 max.
	// Allow ±1 for quantisation.
	if kept > 8 {
		t.Fatalf("expected ≤8 Drone morphs kept (larva-bounded); got %d", kept)
	}
	if kept < 3 {
		t.Fatalf("expected ≥3 Drone morphs kept (starting larva); got %d", kept)
	}
}

// TestEconLookupContract — every Subject in the table reads back identically.
func TestEconLookupContract(t *testing.T) {
	for _, subj := range []string{
		models.GeneralUnitPylon,
		models.GeneralUnitGateway,
		models.GeneralUnitProbe,
		models.GeneralUnitZealot,
		models.GeneralUnitSupplyDepot,
		models.GeneralUnitBarracks,
		models.GeneralUnitSCV,
		models.GeneralUnitMarine,
		models.GeneralUnitOverlord,
		models.GeneralUnitSpawningPool,
		models.GeneralUnitDrone,
		models.GeneralUnitZergling,
	} {
		if _, ok := cmdenrich.EconOf(subj); !ok {
			t.Errorf("expected EconOf(%q) to be present", subj)
		}
	}
}

// TestProducerOfContract — every produced unit has a producer.
func TestProducerOfContract(t *testing.T) {
	cases := map[string]string{
		models.GeneralUnitZealot:   models.GeneralUnitGateway,
		models.GeneralUnitMarine:   models.GeneralUnitBarracks,
		models.GeneralUnitZergling: models.GeneralUnitSpawningPool,
	}
	for unit, wantProducer := range cases {
		got, ok := cmdenrich.ProducerOf(unit)
		if !ok || got != wantProducer {
			t.Errorf("ProducerOf(%q) = (%q, %v); want (%q, true)", unit, got, ok, wantProducer)
		}
	}
}
