package detectors

import (
	"encoding/json"
	"testing"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/markers"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

// stubEvaluator is a test CustomEvaluator: it records every observed fact and
// returns a caller-configured verdict at Finalize. It also captures the
// CustomEvalContext so tests can assert the detector wired replay/worldstate
// through correctly.
type stubEvaluator struct {
	observed  []cmdenrich.EnrichedCommand
	result    markers.CustomResult
	gotCtx    markers.CustomEvalContext
	finalized bool
}

func (e *stubEvaluator) Observe(f cmdenrich.EnrichedCommand) {
	e.observed = append(e.observed, f)
}

func (e *stubEvaluator) Finalize(ctx markers.CustomEvalContext) markers.CustomResult {
	e.finalized = true
	e.gotCtx = ctx
	return e.result
}

// newCustomMarker builds a KindMarker whose Custom factory always returns the
// same stub instance, so the test can inspect it after driving the detector.
func newCustomMarker(stub *stubEvaluator, deadline int) markers.Marker {
	return markers.Marker{
		Name:         "Test Custom",
		PatternName:  "Test Custom",
		FeatureKey:   "test_custom",
		Kind:         markers.KindMarker,
		Custom:       func() markers.CustomEvaluator { return stub },
		RuleDeadline: deadline,
	}
}

// Name returns the marker's PatternName verbatim.
func TestMarkerDetector_Name(t *testing.T) {
	d := NewMarkerPlayerDetector(markers.Marker{PatternName: "Build Order: 9 Pool"})
	if got := d.Name(); got != "Build Order: 9 Pool" {
		t.Fatalf("Name() = %q, want %q", got, "Build Order: 9 Pool")
	}
}

// A Custom marker that finalizes at end-of-replay: every classified command is
// funnelled into the evaluator's Observe, and Finalize's CustomResult drives
// matched / DetectedAtSecond / Payload on the persisted result.
func TestMarkerDetector_Custom_FinalizeAtEndOfReplay_Matched(t *testing.T) {
	payload := json.RawMessage(`{"count":3}`)
	stub := &stubEvaluator{result: markers.CustomResult{Matched: true, DetectedAtSecond: 222, Payload: payload}}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1).
		WithDurationSeconds(600).
		WithCommand(1, 100, models.ActionTypeTrain, "Wraith").
		WithCommand(1, 150, models.ActionTypeTrain, "Wraith")
	replay, players := builder.Build()

	ws := worldstate.NewEngine(replay, players, &models.ReplayMapContext{})
	ws.Finalize()

	d := NewMarkerPlayerDetector(newCustomMarker(stub, 10*60*60))
	d.SetReplayPlayerID(1)
	d.SetWorldState(ws)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	// No command trips the deadline, so end-of-replay Finalize does the work.
	d.Finalize()

	if !stub.finalized {
		t.Fatalf("expected evaluator to be finalized")
	}
	if len(stub.observed) != 2 {
		t.Fatalf("expected 2 observed facts fed to evaluator, got %d", len(stub.observed))
	}
	if stub.gotCtx.ReplayPlayerID != 1 || stub.gotCtx.Replay != replay || stub.gotCtx.WorldState != ws {
		t.Fatalf("evaluator context not wired: %+v", stub.gotCtx)
	}
	if !d.ShouldSave() {
		t.Fatalf("expected matched Custom marker to save")
	}
	result := d.GetResult()
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.PatternName != "Test Custom" {
		t.Fatalf("PatternName = %q", result.PatternName)
	}
	if result.DetectedAtSecond != 222 {
		t.Fatalf("DetectedAtSecond = %d, want 222", result.DetectedAtSecond)
	}
	if string(result.Payload) != string(payload) {
		t.Fatalf("Payload = %s, want %s", result.Payload, payload)
	}
}

// A Custom marker whose evaluator returns Matched:false must not save.
func TestMarkerDetector_Custom_Unmatched_DoesNotSave(t *testing.T) {
	stub := &stubEvaluator{result: markers.CustomResult{Matched: false}}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1).
		WithDurationSeconds(600).
		WithCommand(1, 100, models.ActionTypeTrain, "Wraith")
	replay, players := builder.Build()

	d := NewMarkerPlayerDetector(newCustomMarker(stub, 10*60*60))
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()

	if d.ShouldSave() {
		t.Fatalf("expected unmatched Custom marker to NOT save")
	}
	if d.GetResult() != nil {
		t.Fatalf("expected nil result for unmatched marker")
	}
}

// A command past the Custom marker's RuleDeadline finalizes the evaluator
// mid-stream (processCustom → finalizeCustomAtDeadline) and marks the detector
// finished; later commands are then ignored.
func TestMarkerDetector_Custom_DeadlineTrip_FinalizesMidStream(t *testing.T) {
	stub := &stubEvaluator{result: markers.CustomResult{Matched: true, DetectedAtSecond: 50}}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1).
		WithDurationSeconds(600).
		WithCommand(1, 30, models.ActionTypeTrain, "Wraith").  // observed (before deadline)
		WithCommand(1, 120, models.ActionTypeTrain, "Wraith"). // now > deadline (100) → finalize
		WithCommand(1, 130, models.ActionTypeTrain, "Wraith")  // finished → ignored
	replay, players := builder.Build()

	d := NewMarkerPlayerDetector(newCustomMarker(stub, 100))
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}

	if !d.IsFinished() {
		t.Fatalf("expected detector finished after deadline trip")
	}
	if !stub.finalized {
		t.Fatalf("expected evaluator finalized at deadline")
	}
	// Only the first (sub-deadline) command should have been observed; the
	// deadline-tripping command and the trailing one must not be.
	if len(stub.observed) != 1 || stub.observed[0].Second != 30 {
		t.Fatalf("expected exactly the second=30 fact observed, got %+v", stub.observed)
	}
	if !d.ShouldSave() {
		t.Fatalf("expected matched marker to save")
	}
	if got := d.GetResult().DetectedAtSecond; got != 50 {
		t.Fatalf("DetectedAtSecond = %d, want 50", got)
	}
}

// A misconfigured marker (neither Rule nor Custom set) finalizes as a no-op on
// the first processed command and never saves.
func TestMarkerDetector_Misconfigured_NoRuleNoCustom(t *testing.T) {
	builder := NewTestReplayBuilder().
		WithPlayer(1, "T", "Terran", 1).
		WithDurationSeconds(600).
		WithCommand(1, 30, models.ActionTypeTrain, "Wraith")
	replay, players := builder.Build()

	d := NewMarkerPlayerDetector(markers.Marker{
		Name:         "Broken",
		PatternName:  "Broken",
		Kind:         markers.KindMarker,
		RuleDeadline: 10 * 60 * 60,
	})
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	if !d.IsFinished() {
		t.Fatalf("expected misconfigured marker to finish on first command")
	}
	d.Finalize()
	if d.ShouldSave() {
		t.Fatalf("misconfigured marker must never save")
	}
}

// The Matchup gate rejects a rule marker whose command stream would otherwise
// match, when the replay's matchup is not admitted.
func TestMarkerDetector_MatchupGate_RejectsAndFlipSecond(t *testing.T) {
	mk := markers.Marker{
		Name:         "Test Gateway",
		PatternName:  "Test Gateway",
		FeatureKey:   "test_gateway",
		Kind:         markers.KindMarker,
		Matchup:      []string{"PvP"},
		Rule:         markers.FirstBuildExists(models.GeneralUnitGateway),
		RuleDeadline: 10 * 60,
	}

	// Positive PvP: admitted matchup → saves.
	posBuilder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithMatchup("PvP").
		WithDurationSeconds(600).
		WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay, players := posBuilder.Build()
	replay.TeamFormat = "1v1"

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range posBuilder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()
	if !d.ShouldSave() {
		t.Fatalf("expected match on admitted PvP matchup")
	}

	// Negative PvT: matchup gate rejects on first command.
	negBuilder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithMatchup("PvT").
		WithDurationSeconds(600).
		WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay2, players2 := negBuilder.Build()
	replay2.TeamFormat = "1v1"

	d2 := NewMarkerPlayerDetector(mk)
	d2.SetReplayPlayerID(1)
	d2.Initialize(replay2, players2)
	for _, cmd := range negBuilder.GetCommands() {
		d2.ProcessCommand(cmd)
	}
	d2.Finalize()
	if d2.ShouldSave() {
		t.Fatalf("expected matchup gate to reject PvT for a PvP-only marker")
	}
	if !d2.IsFinished() {
		t.Fatalf("expected rejected detector to be finished")
	}
}

// The MapKind gate rejects a rule marker whose stream would otherwise match,
// when the replay's map kind is not in the allowed set.
func TestMarkerDetector_MapKindGate_Rejects(t *testing.T) {
	mk := markers.Marker{
		Name:         "Test Money Gateway",
		PatternName:  "Test Money Gateway",
		FeatureKey:   "test_money_gateway",
		Kind:         markers.KindMarker,
		MapKind:      []string{"Money"},
		Rule:         markers.FirstBuildExists(models.GeneralUnitGateway),
		RuleDeadline: 10 * 60,
	}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(600).
		WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay, players := builder.Build()
	replay.MapKind = "Regular"

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()
	if d.ShouldSave() {
		t.Fatalf("expected MapKind gate to reject a Money-only marker on a Regular map")
	}

	// Same stream on a Money map → fires.
	builder2 := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(600).
		WithCommand(1, 86, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay2, players2 := builder2.Build()
	replay2.MapKind = "Money"

	d2 := NewMarkerPlayerDetector(mk)
	d2.SetReplayPlayerID(1)
	d2.Initialize(replay2, players2)
	for _, cmd := range builder2.GetCommands() {
		d2.ProcessCommand(cmd)
	}
	d2.Finalize()
	if !d2.ShouldSave() {
		t.Fatalf("expected fire on admitted Money map")
	}
}

// Trailing commands after a rule has committed Matched during streaming must be
// ignored (IsFinished early-return path). The Custom marker's RuleDeadline is
// large so the rule commits mid-stream, not at the deadline. We use a rule
// marker with Expert to keep it alive past first match, then confirm a trailing
// command doesn't panic on the nil dedup map or change the verdict.
func TestMarkerDetector_TrailingCommandsAfterCommitIgnored(t *testing.T) {
	mk := markers.Marker{
		Name:         "Test Zealot",
		PatternName:  "Test Zealot",
		FeatureKey:   "test_zealot",
		Kind:         markers.KindMarker,
		Rule:         markers.FirstProduceExists("Zealot"),
		RuleDeadline: 200,
	}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(600).
		WithCommand(1, 100, models.ActionTypeTrain, "Zealot"). // flips Matched
		WithCommand(1, 250, models.ActionTypeTrain, "Zealot")  // now > deadline → finalize on already-matched
	replay, players := builder.Build()

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()

	if !d.ShouldSave() {
		t.Fatalf("expected Zealot rule to save")
	}
	// DetectedAtSecond must remain the original flip second (100), not the
	// deadline-finalize second.
	if got := d.GetResult().DetectedAtSecond; got != 100 {
		t.Fatalf("DetectedAtSecond = %d, want 100 (original flip, not deadline)", got)
	}
}

// A rule marker with two Rule-based modifiers: one whose predicate holds and one
// that doesn't. matchedModifiers must return only the holding one, and the
// payload must carry it. Exercises the Rule-modifier branch + ordering.
func TestMarkerDetector_RuleModifiers_OnlyMatchingHold(t *testing.T) {
	mk := markers.Marker{
		Name:        "Test Gate Mods",
		PatternName: "Test Gate Mods",
		FeatureKey:  "test_gate_mods",
		Kind:        markers.KindMarker,
		Rule:        markers.FirstBuildExists(models.GeneralUnitGateway),
		Modifiers: []markers.Modifier{
			{Name: "has-zealot", Rule: markers.FirstProduceExists("Zealot")},
			{Name: "has-dragoon", Rule: markers.FirstProduceExists("Dragoon")},
		},
		RuleDeadline: 300,
	}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(600).
		WithCommand(1, 80, models.ActionTypeBuild, models.GeneralUnitGateway).
		WithCommand(1, 150, models.ActionTypeTrain, "Zealot") // only has-zealot holds
	replay, players := builder.Build()

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()

	got := d.matchedModifiers()
	if len(got) != 1 || got[0] != "has-zealot" {
		t.Fatalf("matchedModifiers = %v, want [has-zealot]", got)
	}

	result := d.GetResult()
	if result == nil || len(result.Payload) == 0 {
		t.Fatalf("expected payload carrying modifiers")
	}
	var decoded struct {
		Modifiers []string `json:"modifiers"`
	}
	if err := json.Unmarshal(result.Payload, &decoded); err != nil {
		t.Fatalf("payload decode: %v", err)
	}
	if len(decoded.Modifiers) != 1 || decoded.Modifiers[0] != "has-zealot" {
		t.Fatalf("payload modifiers = %v, want [has-zealot]", decoded.Modifiers)
	}
}

// A WorldstateEvent modifier holds iff the worldstate produced that event for
// the player. Exercises the WorldstateEvent branch of matchedModifiers and the
// hasWorldstateModifier deferral in GetResult (result nil until ws finalized).
func TestMarkerDetector_WorldstateModifier_HoldsWithEvent(t *testing.T) {
	mk := markers.Marker{
		Name:        "Test Gate WSMod",
		PatternName: "Test Gate WSMod",
		FeatureKey:  "test_gate_wsmod",
		Kind:        markers.KindMarker,
		Rule:        markers.FirstBuildExists(models.GeneralUnitGateway),
		Modifiers: []markers.Modifier{
			{Name: "proxy", WorldstateEvent: "proxy_gate"},
		},
		RuleDeadline: 300,
	}

	build := func(withEvent bool) *MarkerPlayerDetector {
		builder := NewTestReplayBuilder().
			WithPlayer(1, "P", "Protoss", 1).
			WithDurationSeconds(600).
			WithCommand(1, 80, models.ActionTypeBuild, models.GeneralUnitGateway)
		replay, players := builder.Build()

		ws := worldstate.NewEngine(replay, players, &models.ReplayMapContext{})
		for _, cmd := range builder.GetCommands() {
			ws.ProcessCommand(cmd)
		}
		if withEvent {
			src := byte(1)
			ws.AppendReplayEvents([]worldstate.ReplayEvent{
				{EventType: "proxy_gate", Second: 60, SourceReplayPlayerID: &src},
			})
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
		return d
	}

	withEvent := build(true)
	if got := withEvent.matchedModifiers(); len(got) != 1 || got[0] != "proxy" {
		t.Fatalf("with event: matchedModifiers = %v, want [proxy]", got)
	}

	withoutEvent := build(false)
	if got := withoutEvent.matchedModifiers(); len(got) != 0 {
		t.Fatalf("without event: matchedModifiers = %v, want empty", got)
	}
}

// A worldstate-backed modifier must not emit a result while the worldstate is
// unfinalized — GetResult defers (returns nil) so a premature Finalize isn't
// triggered.
func TestMarkerDetector_WorldstateModifier_DefersUntilFinalized(t *testing.T) {
	mk := markers.Marker{
		Name:        "Test Gate Defer",
		PatternName: "Test Gate Defer",
		FeatureKey:  "test_gate_defer",
		Kind:        markers.KindMarker,
		Rule:        markers.FirstBuildExists(models.GeneralUnitGateway),
		Modifiers: []markers.Modifier{
			{Name: "proxy", WorldstateEvent: "proxy_gate"},
		},
		RuleDeadline: 300,
	}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(600).
		WithCommand(1, 80, models.ActionTypeBuild, models.GeneralUnitGateway)
	replay, players := builder.Build()

	ws := worldstate.NewEngine(replay, players, &models.ReplayMapContext{})
	for _, cmd := range builder.GetCommands() {
		ws.ProcessCommand(cmd)
	}
	// Deliberately NOT finalized.

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.SetWorldState(ws)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()

	if d.GetResult() != nil {
		t.Fatalf("expected nil result while worldstate is unfinalized (deferral)")
	}
}

// resolveMinReplaySeconds falls back to the flat MinReplaySeconds when the
// player's own record is missing from the 1v1 roster (own == nil). We drive the
// detector for replayPlayerID 9, which has no matching player, so the matchup
// map is bypassed and the flat gate applies.
func TestMarkerDetector_ResolveMinReplaySeconds_MissingOwnPlayerFallsBack(t *testing.T) {
	mk := newGateTestMarker() // flat 10:00, matchup (P,Z)=7:00

	builder := NewTestReplayBuilder().
		WithPlayer(1, "own", "Protoss", 1).
		WithPlayer(2, "opp", "Zerg", 2).
		WithDurationSeconds(8 * 60) // above matchup 7:00 but below flat 10:00
	replay, players := builder.Build()
	replay.TeamFormat = "1v1"

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(9) // no such player → own == nil
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()

	// own == nil → flat 10:00 gate → 8:00 replay is below it → suppressed.
	if d.ShouldSave() {
		t.Fatalf("expected flat-gate fallback (own player missing) to suppress at 8:00")
	}
}

// resolveMinReplaySeconds falls back to the flat gate when there is no opponent
// in the 1v1 roster (opp == nil). We build a "1v1" that actually has only one
// player, so getOpponentInOneVOne returns nil.
func TestMarkerDetector_ResolveMinReplaySeconds_MissingOpponentFallsBack(t *testing.T) {
	mk := newGateTestMarker()

	builder := NewTestReplayBuilder().
		WithPlayer(1, "own", "Protoss", 1).
		WithDurationSeconds(8 * 60)
	replay, players := builder.Build()
	replay.TeamFormat = "1v1"

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
	}
	d.Finalize()

	// opp == nil → flat 10:00 gate → 8:00 below it → suppressed.
	if d.ShouldSave() {
		t.Fatalf("expected flat-gate fallback (opponent missing) to suppress at 8:00")
	}
}

// resolveMinReplaySeconds falls back to the flat gate when the player's own
// race has no bucket in MinReplaySecondsByMatchup. Own race Terran has no
// entry, so the flat 10:00 applies.
func TestMarkerDetector_ResolveMinReplaySeconds_MissingRaceBucketFallsBack(t *testing.T) {
	mk := newGateTestMarker() // only (Protoss, Zerg) populated

	if runGateTest(t, mk, "Terran", "Zerg", "1v1", 8*60) {
		t.Fatalf("expected flat-gate fallback (no Terran bucket) to suppress at 8:00")
	}
}

// A Not(...) rule that rejects mid-stream: the moment the disallowed produce
// appears, Decision flips to Rejected during streaming (not at the deadline),
// committing the detector as unmatched via checkRuleDecision's Rejected branch.
func TestMarkerDetector_RuleRejectsMidStream(t *testing.T) {
	mk := markers.Marker{
		Name:         "Test No Zealot",
		PatternName:  "Test No Zealot",
		FeatureKey:   "test_no_zealot",
		Kind:         markers.KindMarker,
		Rule:         markers.Not(markers.FirstProduceExists("Zealot")),
		RuleDeadline: 10 * 60,
	}

	builder := NewTestReplayBuilder().
		WithPlayer(1, "P", "Protoss", 1).
		WithDurationSeconds(600).
		WithCommand(1, 100, models.ActionTypeTrain, "Zealot"). // Not-child matches → rule Rejected here
		WithCommand(1, 200, models.ActionTypeTrain, "Zealot")  // detector finished → ignored
	replay, players := builder.Build()

	d := NewMarkerPlayerDetector(mk)
	d.SetReplayPlayerID(1)
	d.Initialize(replay, players)
	processed := 0
	for _, cmd := range builder.GetCommands() {
		d.ProcessCommand(cmd)
		processed++
		if processed == 1 && !d.IsFinished() {
			t.Fatalf("expected rule to commit Rejected on the first Zealot (mid-stream)")
		}
	}
	d.Finalize()
	if d.ShouldSave() {
		t.Fatalf("expected Not(Zealot) rule to be rejected once a Zealot was produced")
	}
}

// findPlayer's not-found branch: adding a command for a player id that was never
// registered leaves the command's Player nil. touch boolPtr too so the helper is
// exercised by a real assertion rather than left dead.
func TestTestUtils_FindPlayerNotFoundAndBoolPtr(t *testing.T) {
	builder := NewTestReplayBuilder().WithPlayer(1, "P", "Protoss", 1)
	// player id 7 is not registered → findPlayer returns nil → command.Player nil.
	builder.WithCommand(7, 50, models.ActionTypeBuild, models.GeneralUnitGateway)
	cmds := builder.GetCommands()
	if cmds[0].Player != nil {
		t.Fatalf("expected nil Player for unregistered player id, got %+v", cmds[0].Player)
	}

	b := boolPtr(true)
	if b == nil || *b != true {
		t.Fatalf("boolPtr(true) = %v, want pointer to true", b)
	}
}
