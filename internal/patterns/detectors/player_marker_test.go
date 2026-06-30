package detectors

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// recordingState is a test PredicateState that captures every observed fact.
// Stays Pending forever so the detector drives the full stream through
// enqueueDedup / flushDedupBefore without early-committing.
type recordingState struct {
	observed []cmdenrich.EnrichedCommand
}

func (s *recordingState) Observe(f cmdenrich.EnrichedCommand) {
	s.observed = append(s.observed, f)
}
func (s *recordingState) Decision(int) markers.TriState { return markers.Pending }
func (s *recordingState) Finalize() markers.TriState    { return markers.Rejected }

func findBOForTest(t *testing.T, name string) markers.Marker {
	t.Helper()
	for _, bo := range markers.Markers() {
		if bo.Name == name {
			return bo
		}
	}
	t.Fatalf("BO %q not registered", name)
	return markers.Marker{}
}

func TestMarkerDetector_9Pool_Positive(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "Z", "Zerg", 1)
	// 9 Pool BO: 5 drone morphs (4 starting + 5 = 9 supply), no Overlord
	// before Pool, Pool placed at second 73.
	for i := 0; i < 5; i++ {
		builder.WithCommand(1, 5+i*3, models.ActionTypeUnitMorph, models.GeneralUnitDrone)
	}
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool)
	builder.WithCommand(1, 125, models.ActionTypeUnitMorph, models.GeneralUnitZergling)
	replay, players := builder.Build()

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected 9 pool detection to save")
	}
	result := detector.GetResult()
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.PatternName != "Build Order: 9 Pool" {
		t.Fatalf("unexpected pattern name: %q", result.PatternName)
	}
}

func TestMarkerDetector_TenPlusScouts_FiresOnMoneyMapOnly(t *testing.T) {
	mk := findBOForTest(t, "10+ Scouts")

	// Positive: Money map + 10 Scouts produced.
	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(1200)
	for i := 0; i < 10; i++ {
		builder.WithCommand(1, 600+i*5, models.ActionTypeTrain, "Scout")
	}
	replay, players := builder.Build()
	replay.MapKind = "Money"

	detector := NewMarkerPlayerDetector(mk)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()
	if !detector.ShouldSave() {
		t.Fatalf("expected 10+ Scouts to fire on Money map with 10 Scouts")
	}

	// Negative: same stream, but on a Regular map — MapKind gate rejects.
	builder2 := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(1200)
	for i := 0; i < 10; i++ {
		builder2.WithCommand(1, 600+i*5, models.ActionTypeTrain, "Scout")
	}
	replay2, players2 := builder2.Build()
	replay2.MapKind = "Regular"

	detector2 := NewMarkerPlayerDetector(mk)
	detector2.SetReplayPlayerID(1)
	detector2.Initialize(replay2, players2)
	for _, cmd := range builder2.GetCommands() {
		detector2.ProcessCommand(cmd)
	}
	detector2.Finalize()
	if detector2.ShouldSave() {
		t.Fatalf("expected 10+ Scouts to be gated off on Regular map")
	}
}

func TestMarkerDetector_InitialBOStillFiresOnMoneyMap(t *testing.T) {
	// Money maps must not block detection — the per-player Build Orders
	// tab and per-player summary pills should still surface BOs. The
	// render layer (game-list & replay-summary "Featuring" strips) is
	// where Money-map suppression happens; the detector stays neutral.
	builder := NewTestReplayBuilder().WithPlayer(1, "Z", "Zerg", 1)
	for i := 0; i < 5; i++ {
		builder.WithCommand(1, 5+i*3, models.ActionTypeUnitMorph, models.GeneralUnitDrone)
	}
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool)
	replay, players := builder.Build()
	replay.MapKind = "Money"

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected 9 pool detection to fire on Money map (suppression is render-layer only)")
	}
}

func TestMarkerDetector_9Pool_NegativeWhenOverlordBeforePool(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "Z", "Zerg", 1)
	builder.WithCommand(1, 10, models.ActionTypeUnitMorph, models.GeneralUnitDrone)
	builder.WithCommand(1, 30, models.ActionTypeUnitMorph, models.GeneralUnitOverlord) // disqualifying
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool)
	replay, players := builder.Build()

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatalf("expected 9 pool to NOT match when overlord produced before pool")
	}
}

func TestMarkerDetector_SkipsOtherRaces(t *testing.T) {
	// A Protoss player should not trigger the Zerg 9 pool detector.
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1)
	builder.WithCommand(1, 73, models.ActionTypeBuild, models.GeneralUnitSpawningPool) // nonsensical but proves race gating
	replay, players := builder.Build()

	bo := findBOForTest(t, "9 Pool")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if detector.ShouldSave() {
		t.Fatalf("Protoss player must not match a Zerg BO")
	}
}

// Dedup collapses same-subject, SAME-TILE Build facts within the 3s gap during
// the opening window (pre-BuildDedupMaxSecond). Only the latest observation lands.
func TestBuildDedup_SameTileWithinOpeningCollapses(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFactAt("Gateway", 70, 5, 5))
	d.enqueueDedup(buildFactAt("Gateway", 72, 5, 5)) // same tile, inside gap → replaces pending
	d.flushAllPending()

	if got := len(rec.observed); got != 1 {
		t.Fatalf("expected 1 observation (dedup), got %d", got)
	}
	if rec.observed[0].Second != 72 {
		t.Fatalf("expected latest-wins at second=72, got %d", rec.observed[0].Second)
	}
}

// Two same-subject Build facts within the 3s gap but at DIFFERENT tiles are
// genuinely distinct buildings (e.g. two Gateways going down back-to-back at
// separate spots). Dedup must NOT collapse them — both observations land.
func TestBuildDedup_DifferentTileDoesNotCollapse(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFactAt("Gateway", 70, 5, 5))
	d.enqueueDedup(buildFactAt("Gateway", 72, 9, 9)) // different tile, inside gap → distinct
	d.flushAllPending()

	if got := len(rec.observed); got != 2 {
		t.Fatalf("expected 2 observations (distinct tiles), got %d", got)
	}
	if rec.observed[0].Second != 70 || rec.observed[1].Second != 72 {
		t.Fatalf("expected both builds in order (70, 72), got %+v", rec.observed)
	}
}

// Past BuildDedupMaxSecond (240s), dedup is off: every same-subject Build fact
// reaches the predicate even when they arrive within the 3s gap.
func TestBuildDedup_PastCapDoesNotCollapse(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFact("Gateway", 300))
	d.enqueueDedup(buildFact("Gateway", 301)) // 1s apart but past cap → both land

	if got := len(rec.observed); got != 2 {
		t.Fatalf("expected 2 observations past cap, got %d", got)
	}
}

// A pending entry from before the cap must flush (as its own observation) the
// moment the first post-cap fact arrives, so nothing is silently dropped.
func TestBuildDedup_PendingFromBeforeCapFlushesAtCap(t *testing.T) {
	d, rec := newRecordingBuildDetector()
	d.enqueueDedup(buildFact("Gateway", 238)) // pending, pre-cap
	d.enqueueDedup(buildFact("Gateway", 240)) // at cap → flushes prior, observes self

	if got := len(rec.observed); got != 2 {
		t.Fatalf("expected 2 observations (pre-cap pending flushed + post-cap), got %d", got)
	}
	if rec.observed[0].Second != 238 || rec.observed[1].Second != 240 {
		t.Fatalf("unexpected order: %+v", rec.observed)
	}
}

// newGateTestMarker builds a never_researched-style marker with a
// per-(own_race, opp_race) gate of 7:00 for (P, Z) only, on top of a flat
// 10-minute MinReplaySeconds fallback. Used by the matchup-gate tests.
func newGateTestMarker() markers.Marker {
	return markers.Marker{
		Name:             "Test Never Researched",
		PatternName:      "Test Never Researched",
		FeatureKey:       "test_never_researched",
		Kind:             markers.KindMarker,
		Rule:             markers.Not(markers.TechExists()),
		RuleDeadline:     10 * 60 * 60,
		MinReplaySeconds: 10 * 60,
		MinReplaySecondsByMatchup: map[markers.Race]map[markers.Race]int{
			markers.RaceProtoss: {markers.RaceZerg: 7 * 60},
		},
	}
}

// runGateTest builds a no-Tech 1v1 replay with the given duration / team-format
// and reports whether the detector saved the marker.
func runGateTest(t *testing.T, mk markers.Marker, ownRace, oppRace string, teamFormat string, durationSeconds int) bool {
	t.Helper()
	builder := NewTestReplayBuilder().
		WithPlayer(1, "own", ownRace, 1).
		WithPlayer(2, "opp", oppRace, 2).
		WithDurationSeconds(durationSeconds)
	replay, players := builder.Build()
	replay.TeamFormat = teamFormat

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()
	return d.ShouldSave()
}

// 1v1 PvZ at 5:00. Per-matchup gate (P vs Z) is 7:00, so the marker is
// suppressed — even though the flat 10:00 fallback would also have suppressed
// it. (Sanity case.)
func TestMarkerDetector_MatchupGate_OneVOne_BelowMatchupThreshold(t *testing.T) {
	if runGateTest(t, newGateTestMarker(), "Protoss", "Zerg", "1v1", 5*60) {
		t.Fatalf("expected suppression at 5:00 in 1v1 PvZ (below P-vs-Z 7:00 floor)")
	}
}

// 1v1 PvZ at 8:00. Per-matchup gate is 7:00 → fire. Flat fallback would have
// suppressed at 8:00 (< 10:00); this proves the matchup gate REPLACES the
// flat floor for 1v1, not stacks above it.
func TestMarkerDetector_MatchupGate_OneVOne_AboveMatchupBelowFlat(t *testing.T) {
	if !runGateTest(t, newGateTestMarker(), "Protoss", "Zerg", "1v1", 8*60) {
		t.Fatalf("expected fire at 8:00 in 1v1 PvZ (above P-vs-Z 7:00 floor, despite < 10:00 flat fallback)")
	}
}

// 1v1 ZvP at 8:00. No entry for (Z, P) in the map → fall back to flat 10:00
// MinReplaySeconds → suppress.
func TestMarkerDetector_MatchupGate_OneVOne_MissingBucketFallsBack(t *testing.T) {
	if runGateTest(t, newGateTestMarker(), "Zerg", "Protoss", "1v1", 8*60) {
		t.Fatalf("expected suppression at 8:00 in 1v1 ZvP (no per-matchup entry → flat 10:00 fallback)")
	}
}

// 2v2 PvZ stand-in at 8:00: TeamFormat != "1v1" → matchup map is ignored even
// though (P, Z) is populated. Flat 10:00 fallback applies → suppress.
func TestMarkerDetector_MatchupGate_NonOneVOne_UsesFlatFallback(t *testing.T) {
	if runGateTest(t, newGateTestMarker(), "Protoss", "Zerg", "2v2", 8*60) {
		t.Fatalf("expected suppression at 8:00 in 2v2 (non-1v1 → flat 10:00 fallback even with matchup map populated)")
	}
}

// runGateTestWithLastCommand builds a no-Tech replay where the target player's
// last command is at lastCmdSec, the replay runs to durationSec, and the
// detector receives a worldstate that tracks per-player last command times.
// Used to lock in the per-player time-in-game gate: a player who left or
// stopped playing before the gate threshold must NOT trigger "never X"
// markers even if the replay kept running long enough overall.
func runGateTestWithLastCommand(t *testing.T, mk markers.Marker, teamFormat string, durationSec, lastCmdSec int) bool {
	t.Helper()
	builder := NewTestReplayBuilder().
		WithPlayer(1, "own", "Protoss", 1).
		WithPlayer(2, "opp", "Zerg", 2).
		WithPlayer(3, "ally", "Protoss", 1).
		WithPlayer(4, "ally2", "Zerg", 2).
		WithDurationSeconds(durationSec).
		WithCommand(1, lastCmdSec, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay, players := builder.Build()
	replay.TeamFormat = teamFormat

	ws := worldstate.NewEngine(replay, players, &models.ReplayMapContext{})
	for _, cmd := range builder.GetCommands() {
		ws.ProcessCommand(cmd)
	}
	ws.Finalize()

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.SetWorldState(ws)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()
	return d.ShouldSave()
}

// Non-1v1 (2v2), replay 20:00, target player's last command at 5:00. Flat
// 10:00 floor applies (TeamFormat != "1v1"). Pre-fix, replay duration alone
// gated this in → marker wrongly fired for a player who quit at 5:00 in a
// 20:00 game. Post-fix, the player's last-command second is the effective
// time-in-game and the marker is suppressed.
func TestMarkerDetector_NonOneVOne_PlayerLeftBeforeFlatGate_Suppressed(t *testing.T) {
	if runGateTestWithLastCommand(t, newGateTestMarker(), "2v2", 20*60, 5*60) {
		t.Fatalf("expected suppression: 2v2 player whose last command was at 5:00 must not get a never_X marker on a 20:00 replay (10:00 floor)")
	}
}

// Same 2v2 / 20:00 game, but the target player kept issuing commands past
// the 10:00 floor (last command at 12:00). Marker should fire normally —
// the per-player gate is satisfied.
func TestMarkerDetector_NonOneVOne_PlayerStayedPastFlatGate_Fires(t *testing.T) {
	if !runGateTestWithLastCommand(t, newGateTestMarker(), "2v2", 20*60, 12*60) {
		t.Fatalf("expected fire: 2v2 player active until 12:00 must get the never_X marker on a 20:00 replay (10:00 floor)")
	}
}

// A matchup entry below matchupGateMinSeconds is lifted to that floor so
// successful rushes stay suppressed even when progamer p5 is very early
// (e.g. ZvZ first-Upgrade ≈ 2:05 would otherwise fire on a 2:30 4-pool win).
func TestMarkerDetector_MatchupGate_HardFloorLiftsLowMatchupValue(t *testing.T) {
	mk := newGateTestMarker()
	// Override the (P, Z) entry with a value well below the hard floor.
	mk.MinReplaySecondsByMatchup = map[markers.Race]map[markers.Race]int{
		markers.RaceProtoss: {markers.RaceZerg: 100}, // 1:40, below 4:00
	}
	// Game at 3:00 — above matchup value (100) but below hard floor (240) → suppress.
	if runGateTest(t, mk, "Protoss", "Zerg", "1v1", 3*60) {
		t.Fatalf("expected suppression at 3:00 with matchup value 1:40 (lifted to 4:00 hard floor)")
	}
	// Game at 5:00 — above hard floor → fire.
	if !runGateTest(t, mk, "Protoss", "Zerg", "1v1", 5*60) {
		t.Fatalf("expected fire at 5:00 with matchup value 1:40 (above lifted 4:00 floor)")
	}
}

// runBunkerRushGate drives the real Bunker Rush marker over an all-in
// bunker-topology stream and reports whether it saves, given whether the
// worldstate carries an offensive bunker_rush event for the player. This
// locks the spatial gate (issue #164): topology alone must not save — a
// defensive sim-city Bunker (no event) is rejected, an offensive one fires.
func runBunkerRushGate(t *testing.T, withEvent bool) bool {
	t.Helper()
	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1).
		WithPlayer(2, "Z", "Zerg", 2).
		WithDurationSeconds(600).
		WithCommand(1, 56, models.ActionTypeBuild, models.GeneralUnitBarracks).
		WithCommand(1, 127, models.ActionTypeBuild, models.GeneralUnitBunker).
		WithCommand(1, 160, models.ActionTypeBuild, models.GeneralUnitBunker)
	replay, players := builder.Build()

	ws := worldstate.NewEngine(replay, players, &models.ReplayMapContext{})
	for _, cmd := range builder.GetCommands() {
		ws.ProcessCommand(cmd)
	}
	if withEvent {
		src := byte(1)
		ws.AppendReplayEvents([]worldstate.ReplayEvent{
			{EventType: "bunker_rush", Second: 130, SourceReplayPlayerID: &src},
		})
	}
	ws.Finalize()

	d := NewMarkerPlayerDetector(findBOForTest(t, "Bunker Rush"))
	d.SetReplayPlayerID(1)
	d.SetWorldState(ws)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()
	return d.ShouldSave()
}

// Offensive bunker rush: topology + a bunker_rush worldstate event → saves.
func TestMarkerDetector_BunkerRush_FiresWithOffensiveEvent(t *testing.T) {
	if !runBunkerRushGate(t, true) {
		t.Fatalf("expected Bunker Rush to save when an offensive bunker_rush event is present")
	}
}

// Defensive sim-city bunker: same all-in topology but NO bunker_rush event
// (the bunker was placed at the player's own base) → suppressed (issue #164).
func TestMarkerDetector_BunkerRush_SuppressedWithoutEvent(t *testing.T) {
	if runBunkerRushGate(t, false) {
		t.Fatalf("expected Bunker Rush to be suppressed when no offensive bunker_rush event fired")
	}
}

func newRecordingBuildDetector() (*MarkerPlayerDetector, *recordingState) {
	rec := &recordingState{}
	d := &MarkerPlayerDetector{
		state:   rec,
		pending: map[string]cmdenrich.EnrichedCommand{},
	}
	return d, rec
}

// buildFact makes a same-tile (1,1) building fact. Dedup keys on subject + tile,
// so same-subject buildFacts within the gap collapse; use buildFactAt to vary
// the tile.
func buildFact(subject string, second int) cmdenrich.EnrichedCommand {
	return buildFactAt(subject, second, 1, 1)
}

func buildFactAt(subject string, second, x, y int) cmdenrich.EnrichedCommand {
	return cmdenrich.EnrichedCommand{
		Kind:    cmdenrich.KindMakeBuilding,
		Subject: subject,
		Second:  second,
		X:       intPtr(x),
		Y:       intPtr(y),
	}
}

func TestMarkerDetector_2Gate_Positive(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1).WithMatchup("PvP")
	builder.WithCommand(1, 70, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway)
	builder.WithCommand(1, 130, models.ActionTypeTrain, "Zealot")
	replay, players := builder.Build()

	bo := findBOForTest(t, "2 Gate")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	if !detector.ShouldSave() {
		t.Fatalf("expected 2 gate detection to save")
	}
}

// Ensures the persisted payload carries Expert milestone seconds in
// position-aligned order with bo.Expert. The dashboard now reads this
// payload directly instead of re-resolving on every page load.
func TestMarkerDetector_2Gate_PayloadHasExpertActuals(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1).WithMatchup("PvP")
	// Anti-spam double-click on the 1st Gateway: two Build commands within
	// BuildDedupGapSeconds at the SAME tile (5,5). The detector should dedup →
	// keep the later one, then a real 2nd Gateway at a DIFFERENT tile, then a
	// Zealot. Payload should reflect the post-dedup timings, not the raw stream.
	builder.WithCommandAt(1, 70, models.ActionTypeBuild, models.GeneralUnitGateway, 5, 5) // dropped by dedup
	builder.WithCommandAt(1, 71, models.ActionTypeBuild, models.GeneralUnitGateway, 5, 5) // same tile → wins dedup window → 1st Gateway @ 71
	builder.WithCommandAt(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway, 9, 9) // different tile → 2nd Gateway @ 86
	builder.WithCommand(1, 130, models.ActionTypeTrain, "Zealot")                         // First Zealot @ 130
	replay, players := builder.Build()

	bo := findBOForTest(t, "2 Gate")
	detector := NewMarkerPlayerDetector(bo)
	detector.SetReplayPlayerID(1)
	detector.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		detector.ProcessCommand(cmd)
	}
	detector.Finalize()

	result := detector.GetResult()
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if len(result.Payload) == 0 {
		t.Fatalf("expected payload with expert_actuals, got empty")
	}
	actuals := markers.DecodeExpertActuals(result.Payload)
	if len(actuals) != len(bo.Expert) {
		t.Fatalf("expected %d expert actuals (one per bo.Expert), got %d", len(bo.Expert), len(actuals))
	}
	// bo.Expert order: Pylon, 1st Gateway, 2nd Gateway, First Zealot.
	// No Pylon command emitted in this test → actuals[0].Found==false.
	if actuals[0].Found {
		t.Fatalf("Pylon actual expected not-found (no Pylon emitted): %+v", actuals[0])
	}
	if !actuals[1].Found || actuals[1].Second != 71 {
		t.Fatalf("1st Gateway actual wrong (expected found@71): %+v", actuals[1])
	}
	if !actuals[2].Found || actuals[2].Second != 86 {
		t.Fatalf("2nd Gateway actual wrong (expected found@86): %+v", actuals[2])
	}
	if !actuals[3].Found || actuals[3].Second != 130 {
		t.Fatalf("First Zealot actual wrong (expected found@130): %+v", actuals[3])
	}
}
