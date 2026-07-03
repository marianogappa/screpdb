package worldstate

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func accessorEngine() (*Engine, *models.Player, *models.Player) {
	replay := &models.Replay{DurationSeconds: 1200, MapWidth: 128, MapHeight: 128}
	p1 := &models.Player{PlayerID: 1, SlotID: 1, Name: "P1", Race: "Protoss", Team: 1, Type: models.PlayerTypeHuman}
	p2 := &models.Player{PlayerID: 2, SlotID: 2, Name: "P2", Race: "Protoss", Team: 2, Type: models.PlayerTypeHuman}
	return NewEngine(replay, []*models.Player{p1, p2}, rushProxyTestMapContext()), p1, p2
}

func TestLastCommandSecond(t *testing.T) {
	engine, p1, _ := accessorEngine()
	if _, ok := engine.LastCommandSecond(1); ok {
		t.Fatal("expected no last command before any command")
	}
	engine.ProcessCommand(&models.Command{Player: p1, ActionType: "Move", SecondsFromGameStart: 30})
	engine.ProcessCommand(&models.Command{Player: p1, ActionType: "Move", SecondsFromGameStart: 90})
	engine.ProcessCommand(&models.Command{Player: p1, ActionType: "Move", SecondsFromGameStart: 45})
	sec, ok := engine.LastCommandSecond(1)
	if !ok {
		t.Fatal("expected last command to be recorded")
	}
	if sec != 90 {
		t.Fatalf("last command second=%d want 90 (max, not last-processed)", sec)
	}
	if _, ok := engine.LastCommandSecond(99); ok {
		t.Fatal("unknown player should not have a last command")
	}
}

func TestEnrichedStream_CopiesAllCommandsInOrder(t *testing.T) {
	engine, p1, p2 := accessorEngine()
	engine.ProcessCommand(&models.Command{Player: p1, ActionType: "Move", SecondsFromGameStart: 10})
	engine.ProcessCommand(&models.Command{Player: p2, ActionType: "Move", SecondsFromGameStart: 20})
	engine.ProcessCommand(&models.Command{Player: p1, ActionType: "Move", SecondsFromGameStart: 30})

	stream := engine.EnrichedStream()
	if len(stream) != 3 {
		t.Fatalf("stream len=%d want 3", len(stream))
	}
	if stream[0].Second != 10 || stream[2].Second != 30 {
		t.Fatalf("stream not in time order: %d..%d", stream[0].Second, stream[2].Second)
	}
	// Mutating the returned copy must not affect the engine's internal stream.
	stream[0].Second = 999
	again := engine.EnrichedStream()
	if again[0].Second != 10 {
		t.Fatal("EnrichedStream returned a shared slice; mutation leaked")
	}
}

func TestNaturalExpansionForPlayer(t *testing.T) {
	engine, p1, _ := accessorEngine()
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(14),
		Y:                    intPtr(10),
		SecondsFromGameStart: 60,
	})
	engine.Finalize()
	name, ok := engine.NaturalExpansionForPlayer(1)
	if !ok {
		t.Fatal("expected p1 to have a natural expansion assigned")
	}
	if name == "" {
		t.Fatal("natural expansion display name should be non-empty")
	}
	if _, ok := engine.NaturalExpansionForPlayer(200); ok {
		t.Fatal("unknown player should have no natural expansion")
	}
}

func TestFirstExpansionForPlayer(t *testing.T) {
	engine, p1, _ := accessorEngine()
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(14),
		Y:                    intPtr(10),
		SecondsFromGameStart: 60,
	})
	sec, where := engine.FirstExpansionForPlayer(1)
	if sec == nil || where == nil {
		t.Fatalf("expected an expansion for p1, got sec=%v where=%v", sec, where)
	}
	if *sec != 60 {
		t.Fatalf("expansion second=%d want 60", *sec)
	}
	if *where == "" {
		t.Fatal("expansion location text should be non-empty")
	}
	// A player that never expands returns nils.
	if s, w := engine.FirstExpansionForPlayer(2); s != nil || w != nil {
		t.Fatalf("p2 never expanded; got sec=%v where=%v", s, w)
	}
}

func TestFirstEventSecondForPlayer(t *testing.T) {
	engine, p1, _ := accessorEngine()
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(14),
		Y:                    intPtr(10),
		SecondsFromGameStart: 60,
	})
	if got := engine.FirstEventSecondForPlayer(1, "expansion"); got == nil || *got != 60 {
		t.Fatalf("expansion event second=%v want 60", got)
	}
	// Unknown / unsupported event types return nil rather than scanning.
	if got := engine.FirstEventSecondForPlayer(1, "not_a_real_event"); got != nil {
		t.Fatalf("unsupported event type should return nil, got %v", got)
	}
	// Supported type but no matching event -> nil.
	if got := engine.FirstEventSecondForPlayer(1, "nuke"); got != nil {
		t.Fatalf("no nuke event expected, got %v", got)
	}
	// Right event type but wrong player -> nil.
	if got := engine.FirstEventSecondForPlayer(2, "expansion"); got != nil {
		t.Fatalf("p2 has no expansion event, got %v", got)
	}
}

func TestDebugSnapshot(t *testing.T) {
	engine, p1, _ := accessorEngine()
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(14),
		Y:                    intPtr(10),
		SecondsFromGameStart: 60,
	})
	engine.Finalize()
	bases, startByPID, naturalByPID, _ := engine.DebugSnapshot()
	if len(bases) == 0 {
		t.Fatal("expected bases in snapshot")
	}
	for i, b := range bases {
		if b.Index != i {
			t.Fatalf("base %d has Index=%d", i, b.Index)
		}
	}
	if _, ok := startByPID[1]; !ok {
		t.Fatal("expected p1 start base recorded")
	}
	if _, ok := naturalByPID[1]; !ok {
		t.Fatal("expected p1 natural base recorded after expansion")
	}
	// Mutating returned maps must not affect the engine.
	startByPID[1] = 999
	bases2, startByPID2, _, _ := engine.DebugSnapshot()
	if startByPID2[1] == 999 {
		t.Fatal("DebugSnapshot leaked its internal map")
	}
	if len(bases2) != len(bases) {
		t.Fatal("snapshot base count should be stable")
	}
}

func TestReportAttackFilter_NoBasesIsEmpty(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 600}
	p1 := &models.Player{PlayerID: 1, Name: "P1", Race: "Protoss", Team: 1, Type: models.PlayerTypeHuman}
	engine := NewEngine(replay, []*models.Player{p1}, nil)
	engine.ProcessCommand(&models.Command{Player: p1, ActionType: "Move", SecondsFromGameStart: 10})
	rep := engine.ReportAttackFilter()
	if rep.Total != 0 || rep.Kept != 0 || rep.Dropped != 0 || len(rep.Decisions) != 0 {
		t.Fatalf("no-bases report should be empty, got %+v", rep)
	}
}

func TestReportAttackFilter_KeptPlusDroppedEqualsTotal(t *testing.T) {
	engine, p1, p2 := accessorEngine()
	// p2 sits in its base; p1 pushes into p2's main repeatedly to generate
	// attack candidates that the importance filter classifies.
	engine.ProcessCommand(&models.Command{Player: p2, ActionType: "Move", X: intPtr(tilePixel(40)), Y: intPtr(tilePixel(40)), SecondsFromGameStart: 5})
	id := byte(0x0a) // Attack Move family
	for i := 0; i < 8; i++ {
		engine.ProcessCommand(&models.Command{
			Player:               p1,
			ActionType:           "TargetedOrder",
			OrderName:            stringPtr("Attack Move"),
			OrderID:              &id,
			X:                    intPtr(tilePixel(40)),
			Y:                    intPtr(tilePixel(40)),
			SecondsFromGameStart: 300 + i*20,
		})
	}
	rep := engine.ReportAttackFilter()
	if rep.Kept+rep.Dropped != rep.Total {
		t.Fatalf("kept(%d)+dropped(%d) != total(%d)", rep.Kept, rep.Dropped, rep.Total)
	}
	if len(rep.Decisions) != rep.Total {
		t.Fatalf("decisions(%d) != total(%d)", len(rep.Decisions), rep.Total)
	}
	for _, d := range rep.Decisions {
		if d.Reason == "" {
			t.Fatal("every decision should carry a reason")
		}
	}
}
