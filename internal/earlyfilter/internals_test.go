package earlyfilter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
)

// --- sim.go: newBuiltHatchery / newStartingHatchery ---

func TestNewBuiltHatchery(t *testing.T) {
	step := secondsToFrame(larvaSpawnIntervalS)
	tests := []struct {
		name        string
		completion  int32
		wantNext    int32
		wantAvail   int
		wantCompFrm int32
	}{
		{"frame zero", 0, step, larvaPerHatchery, 0},
		{"mid game", 1000, 1000 + step, larvaPerHatchery, 1000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newBuiltHatchery(tc.completion)
			if h.completionFrame != tc.wantCompFrm {
				t.Errorf("completionFrame = %d, want %d", h.completionFrame, tc.wantCompFrm)
			}
			if h.nextSpawnFrame != tc.wantNext {
				t.Errorf("nextSpawnFrame = %d, want %d", h.nextSpawnFrame, tc.wantNext)
			}
			if h.available != tc.wantAvail {
				t.Errorf("available = %d, want %d", h.available, tc.wantAvail)
			}
		})
	}
}

// A built hatchery must not spawn larva before its completion frame, but must
// begin spawning once online.
func TestNewBuiltHatcheryAdvanceGating(t *testing.T) {
	completion := secondsToFrame(60)
	h := newBuiltHatchery(completion)
	h.available = 0

	h.advance(completion - 1)
	if h.available != 0 {
		t.Fatalf("hatchery spawned before completion: available=%d", h.available)
	}

	// One full interval past completion → exactly one larva.
	h.advance(completion + secondsToFrame(larvaSpawnIntervalS))
	if h.available != 1 {
		t.Fatalf("expected 1 larva one interval after completion, got %d", h.available)
	}
}

// --- sim.go: availableLarvaCount ---

func TestAvailableLarvaCount(t *testing.T) {
	tests := []struct {
		name       string
		hatcheries []hatcheryLarva
		want       int
	}{
		{"none", nil, 0},
		{"single starting", []hatcheryLarva{{available: 3}}, 3},
		{"multi", []hatcheryLarva{{available: 3}, {available: 2}, {available: 0}}, 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &playerSim{hatcheries: tc.hatcheries}
			if got := p.availableLarvaCount(); got != tc.want {
				t.Errorf("availableLarvaCount() = %d, want %d", got, tc.want)
			}
		})
	}
}

// --- sim.go: producedCount ---

func TestProducedCount(t *testing.T) {
	drone, _ := cmdenrich.EconOf(models.GeneralUnitDrone)   // 50m, larva-consuming
	zealot, _ := cmdenrich.EconOf(models.GeneralUnitZealot) // 100m, not larva
	freeEcon := cmdenrich.UnitEcon{Minerals: 0}

	tests := []struct {
		name     string
		minerals float64
		larva    int
		subject  string
		econ     cmdenrich.UnitEcon
		intended int
		want     int
	}{
		{"intended one returns one", 1000, 3, models.GeneralUnitDrone, drone, 1, 1},
		{"intended zero clamps to one", 1000, 3, models.GeneralUnitDrone, drone, 0, 1},
		{"non-morph capped by minerals", 250, 0, models.GeneralUnitZealot, zealot, 4, 2},
		{"morph capped by larva", 1000, 2, models.GeneralUnitDrone, drone, 5, 2},
		{"morph capped by minerals below larva", 150, 5, models.GeneralUnitDrone, drone, 5, 3},
		{"morph fully affordable", 1000, 5, models.GeneralUnitDrone, drone, 4, 4},
		{"zero-cost unit not capped by minerals", 0, 5, models.GeneralUnitDrone, freeEcon, 4, 4},
		{"exhausted larva clamps to one", 1000, 0, models.GeneralUnitDrone, drone, 5, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &playerSim{minerals: tc.minerals}
			if tc.larva > 0 || tc.subject == models.GeneralUnitDrone {
				p.hatcheries = []hatcheryLarva{{available: tc.larva}}
			}
			got := p.producedCount(tc.subject, tc.econ, tc.intended)
			if got != tc.want {
				t.Errorf("producedCount(%s, intended=%d, min=%v, larva=%d) = %d, want %d",
					tc.subject, tc.intended, tc.minerals, tc.larva, got, tc.want)
			}
		})
	}
}

// --- sim.go: consumeLarva ---

func TestConsumeLarva(t *testing.T) {
	step := secondsToFrame(larvaSpawnIntervalS)

	t.Run("takes from first available", func(t *testing.T) {
		p := &playerSim{hatcheries: []hatcheryLarva{{available: 0}, {available: 2}}}
		if !p.consumeLarva() {
			t.Fatal("expected consumeLarva to succeed")
		}
		if p.hatcheries[1].available != 1 {
			t.Fatalf("expected second hatchery larva decremented to 1, got %d", p.hatcheries[1].available)
		}
		if p.hatcheries[0].available != 0 {
			t.Fatalf("first hatchery should be untouched, got %d", p.hatcheries[0].available)
		}
	})

	t.Run("borrows soonest-to-spawn when none available", func(t *testing.T) {
		p := &playerSim{hatcheries: []hatcheryLarva{
			{available: 0, nextSpawnFrame: 500},
			{available: 0, nextSpawnFrame: 200}, // soonest
		}}
		if !p.consumeLarva() {
			t.Fatal("expected borrow to succeed")
		}
		// The soonest hatchery's next spawn is pushed out by one cycle.
		if p.hatcheries[1].nextSpawnFrame != 200+step {
			t.Fatalf("expected borrowed hatchery nextSpawnFrame=%d, got %d", 200+step, p.hatcheries[1].nextSpawnFrame)
		}
		if p.hatcheries[0].nextSpawnFrame != 500 {
			t.Fatalf("non-borrowed hatchery must be untouched, got %d", p.hatcheries[0].nextSpawnFrame)
		}
	})

	t.Run("no hatcheries fails", func(t *testing.T) {
		p := &playerSim{}
		if p.consumeLarva() {
			t.Fatal("expected consumeLarva to fail with no hatcheries")
		}
	})
}

// --- sim.go: schedulePending ordering ---

func TestSchedulePendingOrdering(t *testing.T) {
	p := &playerSim{}
	for _, f := range []int32{300, 100, 200, 50, 250} {
		p.schedulePending(pendingEvent{completionFrame: f})
	}
	var got []int32
	for _, ev := range p.pending {
		got = append(got, ev.completionFrame)
	}
	want := []int32{50, 100, 200, 250, 300}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pending not sorted: got %v, want %v", got, want)
		}
	}
}

// --- sim.go: isGasBuildingSubject ---

func TestIsGasBuildingSubject(t *testing.T) {
	tests := []struct {
		subject string
		want    bool
	}{
		{models.GeneralUnitRefinery, true},
		{models.GeneralUnitExtractor, true},
		{models.GeneralUnitAssimilator, true},
		{models.GeneralUnitGateway, false},
		{models.GeneralUnitPylon, false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isGasBuildingSubject(tc.subject); got != tc.want {
			t.Errorf("isGasBuildingSubject(%q) = %v, want %v", tc.subject, got, tc.want)
		}
	}
}

// --- backtrack.go: findLatestKeptWorkerBefore ---

func TestFindLatestKeptWorkerBefore(t *testing.T) {
	p := terranPlayer()
	other := protossPlayer()
	commands := []*models.Command{
		makeCmd("Train", models.GeneralUnitSCV, 10, p),       // 0 kept
		makeCmd("Train", models.GeneralUnitSCV, 20, p),       // 1 kept  (latest before 100)
		makeCmd("Train", models.GeneralUnitSCV, 30, p),       // 2 dropped — not eligible
		makeCmd("Train", models.GeneralUnitMarine, 25, p),    // 3 kept, not a worker
		makeCmd("Train", models.GeneralUnitProbe, 15, other), // 4 wrong player
		makeCmd("Train", models.GeneralUnitSCV, 200, p),      // 5 after frame
	}
	verdicts := map[int]Verdict{
		0: VerdictKept, 1: VerdictKept, 2: VerdictDropped,
		3: VerdictKept, 4: VerdictKept, 5: VerdictKept,
	}
	pid := int64(p.PlayerID)
	frame := secondFrame(100)

	t.Run("picks latest kept worker of player before frame", func(t *testing.T) {
		got := findLatestKeptWorkerBefore(commands, verdicts, pid, frame, map[int]bool{})
		if got != 1 {
			t.Fatalf("expected index 1, got %d", got)
		}
	})

	t.Run("skips already forceDropped", func(t *testing.T) {
		got := findLatestKeptWorkerBefore(commands, verdicts, pid, frame, map[int]bool{1: true})
		if got != 0 {
			t.Fatalf("expected index 0 after skipping forceDropped 1, got %d", got)
		}
	})

	t.Run("none eligible returns -1", func(t *testing.T) {
		got := findLatestKeptWorkerBefore(commands, verdicts, pid, secondFrame(5), map[int]bool{})
		if got != -1 {
			t.Fatalf("expected -1 when no worker before frame, got %d", got)
		}
	})
}

// --- backtrack.go: findLatestBuildBefore ---

func TestFindLatestBuildBefore(t *testing.T) {
	p := protossPlayer()
	other := terranPlayer()
	commands := []*models.Command{
		makeCmd("Build", models.GeneralUnitGateway, 40, p),     // 0 kept
		makeCmd("Build", models.GeneralUnitGateway, 60, p),     // 1 dropped (preferred target)
		makeCmd("Build", models.GeneralUnitGateway, 200, p),    // 2 after frame
		makeCmd("Build", models.GeneralUnitGateway, 50, other), // 3 wrong player
		makeCmd("Train", models.GeneralUnitZealot, 55, p),      // 4 not a building
	}
	verdicts := map[int]Verdict{0: VerdictKept, 1: VerdictDropped, 2: VerdictDropped, 3: VerdictDropped, 4: VerdictKept}
	pid := int64(p.PlayerID)
	frame := secondFrame(100)

	t.Run("prefers dropped build before frame", func(t *testing.T) {
		got := findLatestBuildBefore(commands, verdicts, pid, models.GeneralUnitGateway, frame, map[int]bool{})
		if got != 1 {
			t.Fatalf("expected dropped build index 1, got %d", got)
		}
	})

	t.Run("falls back to latest any build when the dropped one is already mustKeep", func(t *testing.T) {
		// idx 1 is dropped but mustKeep, so it is not a preferred (dropped)
		// target; bestAny still selects the latest matching build before frame,
		// which remains idx 1.
		got := findLatestBuildBefore(commands, verdicts, pid, models.GeneralUnitGateway, frame, map[int]bool{1: true})
		if got != 1 {
			t.Fatalf("expected fallback to latest build index 1, got %d", got)
		}
	})

	t.Run("no match returns -1", func(t *testing.T) {
		got := findLatestBuildBefore(commands, verdicts, pid, models.GeneralUnitForge, frame, map[int]bool{})
		if got != -1 {
			t.Fatalf("expected -1 for absent subject, got %d", got)
		}
	})

	t.Run("falls back to a kept build when no eligible dropped build exists", func(t *testing.T) {
		// Only a kept build of the subject exists before frame → bestDropped
		// stays -1 and bestAny (the kept build) is returned.
		cmds := []*models.Command{makeCmd("Build", models.GeneralUnitForge, 40, p)}
		vs := map[int]Verdict{0: VerdictKept}
		got := findLatestBuildBefore(cmds, vs, pid, models.GeneralUnitForge, frame, map[int]bool{})
		if got != 0 {
			t.Fatalf("expected kept build index 0, got %d", got)
		}
	})
}

// --- backtrack.go: resolveViolations recursive prereq chain + worker drop ---

// A Photon Cannon violation missing Forge must re-admit both Forge and its
// transitive prereq Pylon (chain), and when the re-admission overdraws it must
// also force-drop the latest kept worker before the earliest re-admitted build.
func TestResolveViolationsChainAndWorkerDrop(t *testing.T) {
	p := protossPlayer()
	commands := []*models.Command{
		makeCmd("Train", models.GeneralUnitProbe, 10, p),         // 0 kept worker (drop candidate)
		makeCmd("Build", models.GeneralUnitPylon, 30, p),         // 1 dropped
		makeCmd("Build", models.GeneralUnitForge, 60, p),         // 2 dropped
		makeCmd("Build", models.GeneralUnitPhotonCannon, 130, p), // 3 kept (consequent)
	}
	verdicts := map[int]Verdict{
		0: VerdictKept, 1: VerdictDropped, 2: VerdictDropped, 3: VerdictKept,
	}
	// Make both re-admissions unaffordable so a worker drop is triggered.
	mineralsAfter := map[int]int{1: 0, 2: 0, 3: 500}

	violations := findViolations(commands, verdicts)
	if len(violations) == 0 {
		t.Fatal("expected at least one violation for Cannon missing Forge")
	}

	mustKeep := map[int]bool{}
	forceDrop := map[int]bool{}
	progress := resolveViolations(violations, commands, verdicts, mineralsAfter, mustKeep, forceDrop)
	if !progress {
		t.Fatal("expected resolveViolations to report progress")
	}
	if !mustKeep[1] {
		t.Error("expected Pylon (idx 1) re-admitted via prereq chain")
	}
	if !mustKeep[2] {
		t.Error("expected Forge (idx 2) re-admitted")
	}
	if !forceDrop[0] {
		t.Error("expected the kept Probe (idx 0) force-dropped to fund unaffordable re-admission")
	}
}

// When the re-admission is affordable, resolveViolations must re-admit the
// prereq but must NOT force-drop a worker (guards the income-collapse spiral).
func TestResolveViolationsAffordableNoWorkerDrop(t *testing.T) {
	p := protossPlayer()
	commands := []*models.Command{
		makeCmd("Train", models.GeneralUnitProbe, 10, p),   // 0 kept worker
		makeCmd("Build", models.GeneralUnitPylon, 30, p),   // 1 dropped
		makeCmd("Build", models.GeneralUnitGateway, 60, p), // 2 dropped
		makeCmd("Train", models.GeneralUnitZealot, 130, p), // 3 kept consequent
	}
	verdicts := map[int]Verdict{0: VerdictKept, 1: VerdictDropped, 2: VerdictDropped, 3: VerdictKept}
	// Balances comfortably cover both builds → affordable.
	mineralsAfter := map[int]int{1: 500, 2: 500, 3: 500}

	violations := findViolations(commands, verdicts)
	mustKeep := map[int]bool{}
	forceDrop := map[int]bool{}
	resolveViolations(violations, commands, verdicts, mineralsAfter, mustKeep, forceDrop)

	if !mustKeep[2] {
		t.Error("expected Gateway re-admitted")
	}
	if !mustKeep[1] {
		t.Error("expected Pylon (Gateway prereq) re-admitted")
	}
	if forceDrop[0] {
		t.Error("affordable re-admission must not force-drop a worker")
	}
}

// --- trace.go: summaryFor ---

func TestSummaryFor(t *testing.T) {
	p := protossPlayer()
	other := terranPlayer()
	commands := []*models.Command{
		makeCmd("Build", models.GeneralUnitPylon, 10, p),        // 0 kept
		makeCmd("Build", models.GeneralUnitGateway, 20, p),      // 1 readmitted
		makeCmd("Train", models.GeneralUnitProbe, 30, p),        // 2 dropped
		makeCmd("Build", models.GeneralUnitForge, 40, p),        // 3 dropped_by_tags
		makeCmd("Train", models.GeneralUnitProbe, 50, p),        // 4 dropped_by_backtrack
		makeCmd("Train", models.GeneralUnitZealot, 60, p),       // 5 unclassified verdict ("")
		makeCmd("Build", models.GeneralUnitBarracks, 70, other), // 6 other player, ignored
	}
	verdicts := map[int]Verdict{
		0: VerdictKept, 1: VerdictReadmitted, 2: VerdictDropped,
		3: VerdictDroppedByTags, 4: VerdictDroppedByBacktrack, 5: "",
		6: VerdictKept,
	}
	got := summaryFor(commands, verdicts, int64(p.PlayerID))

	want := PlayerStats{
		Total:                   6, // excludes the other-player command
		Kept:                    3, // kept + readmitted + "" default
		Dropped:                 3, // dropped + dropped_by_tags + dropped_by_backtrack
		Readmitted:              1,
		WorkerDropsForBacktrack: 1,
	}
	if got != want {
		t.Fatalf("summaryFor = %+v, want %+v", got, want)
	}
}

// --- trace.go: writeTrace round-trip ---

func TestWriteTrace(t *testing.T) {
	dir := t.TempDir()
	tr := &Trace{
		Replay:     "r.rep",
		MaxSecond:  240,
		Iterations: 2,
		Players: []PlayerTrace{{
			PlayerID: 1, Race: "Protoss",
			Summary: PlayerStats{Total: 5, Kept: 4, Dropped: 1},
		}},
	}

	t.Run("writes named by checksum and round-trips", func(t *testing.T) {
		if err := writeTrace(dir, "abc123", tr); err != nil {
			t.Fatalf("writeTrace error: %v", err)
		}
		path := filepath.Join(dir, "abc123.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading trace: %v", err)
		}
		var got Trace
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Iterations != 2 || len(got.Players) != 1 || got.Players[0].Summary.Kept != 4 {
			t.Fatalf("round-tripped trace mismatch: %+v", got)
		}
	})

	t.Run("empty checksum falls back to unknown.json", func(t *testing.T) {
		if err := writeTrace(dir, "", tr); err != nil {
			t.Fatalf("writeTrace error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "unknown.json")); err != nil {
			t.Fatalf("expected unknown.json to exist: %v", err)
		}
	})

	t.Run("empty dir is a no-op", func(t *testing.T) {
		if err := writeTrace("", "abc", tr); err != nil {
			t.Fatalf("expected nil for empty dir, got %v", err)
		}
	})

	t.Run("nil trace is a no-op", func(t *testing.T) {
		if err := writeTrace(dir, "abc", nil); err != nil {
			t.Fatalf("expected nil for nil trace, got %v", err)
		}
	})
}

// --- earlyfilter.go: buildStats ---

func TestBuildStats(t *testing.T) {
	p := protossPlayer()
	other := terranPlayer()
	commands := []*models.Command{
		makeCmd("Build", models.GeneralUnitPylon, 10, p),        // 0 kept
		makeCmd("Build", models.GeneralUnitGateway, 20, p),      // 1 readmitted
		makeCmd("Train", models.GeneralUnitProbe, 30, p),        // 2 dropped
		makeCmd("Build", models.GeneralUnitForge, 40, p),        // 3 dropped_by_tags
		makeCmd("Train", models.GeneralUnitProbe, 50, p),        // 4 dropped_by_backtrack (forceDrop)
		makeCmd("Build", models.GeneralUnitBarracks, 60, other), // 5 other player kept
		nil, // 6 nil command skipped
	}
	verdicts := map[int]Verdict{
		0: VerdictKept, 1: VerdictReadmitted, 2: VerdictDropped,
		3: VerdictDroppedByTags, 4: VerdictDroppedByBacktrack, 5: VerdictKept,
	}
	forceDrop := map[int]bool{4: true}

	stats := buildStats(commands, verdicts, forceDrop)

	pStats := stats.PerPlayer[int64(p.PlayerID)]
	want := PlayerStats{
		Total: 5, Kept: 2, Dropped: 3, Readmitted: 1, WorkerDropsForBacktrack: 1,
	}
	if pStats != want {
		t.Fatalf("player stats = %+v, want %+v", pStats, want)
	}
	if o := stats.PerPlayer[int64(other.PlayerID)]; o.Total != 1 || o.Kept != 1 {
		t.Fatalf("other player stats = %+v, want Total=1 Kept=1", o)
	}
}

// dropped_by_backtrack without a forceDrop entry must count as Dropped but NOT
// as a worker drop (guards the stat from double-counting).
func TestBuildStatsBacktrackWithoutForceDrop(t *testing.T) {
	p := protossPlayer()
	commands := []*models.Command{makeCmd("Train", models.GeneralUnitProbe, 10, p)}
	verdicts := map[int]Verdict{0: VerdictDroppedByBacktrack}

	stats := buildStats(commands, verdicts, map[int]bool{})
	s := stats.PerPlayer[int64(p.PlayerID)]
	if s.Dropped != 1 || s.WorkerDropsForBacktrack != 0 {
		t.Fatalf("stats = %+v, want Dropped=1 WorkerDropsForBacktrack=0", s)
	}
}

// --- earlyfilter.go: runForward force-drop + pass-through branches ---

// A command with a nil Player is kept verbatim; a command past MaxSecond is
// kept with a minerals snapshot; forceDrop yields dropped_by_backtrack.
func TestRunForwardBranches(t *testing.T) {
	p := terranPlayer()
	commands := []*models.Command{
		{ActionType: "Train", Frame: 0, Player: nil},         // 0 nil player → kept
		makeCmd("Train", models.GeneralUnitSCV, 10, p),       // 1 forceDrop target
		makeCmd("Build", models.GeneralUnitBarracks, 400, p), // 2 past MaxSecond → kept
		makeCmd("Attack", models.GeneralUnitMarine, 20, p),   // 3 non-filtered kind → kept
	}
	sims := initSims(map[int64]string{int64(p.PlayerID): "Terran"}, nil)
	forceDrop := map[int]bool{1: true}

	verdicts, reasons, _ := runForward(commands, sims, map[int]bool{}, forceDrop, Options{MaxSecond: 240})

	if verdicts[0] != VerdictKept {
		t.Errorf("nil-player command: got %q, want kept", verdicts[0])
	}
	if verdicts[1] != VerdictDroppedByBacktrack || reasons[1] != "freed_minerals_for_backtrack" {
		t.Errorf("forceDrop command: got verdict=%q reason=%q", verdicts[1], reasons[1])
	}
	if verdicts[2] != VerdictKept {
		t.Errorf("past-window command: got %q, want kept", verdicts[2])
	}
	if verdicts[3] != VerdictKept {
		t.Errorf("non-filtered kind: got %q, want kept", verdicts[3])
	}
}

// A command whose player has no sim (not in raceByPlayer) is kept.
func TestRunForwardUnknownPlayerKept(t *testing.T) {
	p := terranPlayer()
	commands := []*models.Command{makeCmd("Train", models.GeneralUnitSCV, 10, p)}
	sims := initSims(map[int64]string{}, nil) // no sim for p
	verdicts, _, _ := runForward(commands, sims, map[int]bool{}, map[int]bool{}, Options{MaxSecond: 240})
	if verdicts[0] != VerdictKept {
		t.Fatalf("unknown-player command: got %q, want kept", verdicts[0])
	}
}

// mustKeep re-admits a command that the sim would otherwise drop for resources,
// recording the readmit reason and applying its effects.
func TestRunForwardMustKeepReadmit(t *testing.T) {
	p := protossPlayer()
	commands := []*models.Command{makeCmd("Build", models.GeneralUnitGateway, 30, p)}
	sims := initSims(map[int64]string{int64(p.PlayerID): "Protoss"}, nil)
	verdicts, reasons, _ := runForward(commands, sims, map[int]bool{0: true}, map[int]bool{}, Options{MaxSecond: 240})
	if verdicts[0] != VerdictReadmitted || reasons[0] != "tech_tree_readmit" {
		t.Fatalf("mustKeep command: got verdict=%q reason=%q, want readmitted", verdicts[0], reasons[0])
	}
}
