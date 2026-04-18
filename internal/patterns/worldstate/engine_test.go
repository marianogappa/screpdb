package worldstate

import (
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestProcessCommand_EmitsRaceEventForProtossNonProtossBuilding(t *testing.T) {
	replay := &models.Replay{}
	protoss := &models.Player{PlayerID: 1, Name: "P", Race: "Protoss", Team: 1}
	engine := NewEngine(replay, []*models.Player{protoss}, nil)

	engine.ProcessCommand(&models.Command{
		Player:               protoss,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitHatchery),
		SecondsFromGameStart: 200,
	})

	entries := engine.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 event, got %d", len(entries))
	}
	if entries[0].Type != "became_zerg" {
		t.Fatalf("expected became_zerg event type, got %q", entries[0].Type)
	}
	if entries[0].Description != "P became Zerg" {
		t.Fatalf("unexpected race event description: %q", entries[0].Description)
	}
}

func TestProcessCommand_EmitsBothBecameTerranAndBecameZergOnceEach(t *testing.T) {
	replay := &models.Replay{}
	protoss := &models.Player{PlayerID: 1, Name: "P", Race: "Protoss", Team: 1}
	engine := NewEngine(replay, []*models.Player{protoss}, nil)

	engine.ProcessCommand(&models.Command{
		Player:               protoss,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitBarracks),
		SecondsFromGameStart: 180,
	})
	engine.ProcessCommand(&models.Command{
		Player:               protoss,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitSupplyDepot),
		SecondsFromGameStart: 181,
	})
	engine.ProcessCommand(&models.Command{
		Player:               protoss,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitHatchery),
		SecondsFromGameStart: 300,
	})
	engine.ProcessCommand(&models.Command{
		Player:               protoss,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitExtractor),
		SecondsFromGameStart: 301,
	})

	entries := engine.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 events (one per race), got %d", len(entries))
	}
	if entries[0].Type != "became_terran" || entries[0].Description != "P became Terran" {
		t.Fatalf("unexpected terran race switch event: %+v", entries[0])
	}
	if entries[1].Type != "became_zerg" || entries[1].Description != "P became Zerg" {
		t.Fatalf("unexpected zerg race switch event: %+v", entries[1])
	}
}

func TestProcessCommand_DoesNotEmitZerglingRushWithoutAttackInference(t *testing.T) {
	replay := &models.Replay{}
	zerg := &models.Player{PlayerID: 2, Name: "Z", Race: "Zerg", Team: 2}
	engine := NewEngine(replay, []*models.Player{zerg}, nil)

	engine.ProcessCommand(&models.Command{
		Player:               zerg,
		ActionType:           models.ActionTypeUnitMorph,
		UnitType:             stringPtr(models.GeneralUnitZergling),
		SecondsFromGameStart: 139,
	})

	events := engine.ReplayEvents()
	for _, event := range events {
		if event.EventType == "zergling_rush" {
			t.Fatalf("expected no zergling_rush without inferred target location")
		}
	}
}

func TestProcessCommand_ZerglingRushDelayedTargetInference(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 300, MapWidth: 128, MapHeight: 128}
	p1 := &models.Player{PlayerID: 1, SlotID: 1, Name: "Z", Race: "Zerg", Team: 1, Type: models.PlayerTypeHuman}
	p2 := &models.Player{PlayerID: 2, SlotID: 2, Name: "T", Race: "Terran", Team: 2, Type: models.PlayerTypeHuman}
	engine := NewEngine(replay, []*models.Player{p1, p2}, rushProxyTestMapContext())

	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeUnitMorph,
		UnitType:             stringPtr(models.GeneralUnitZergling),
		SecondsFromGameStart: 100,
	})
	engine.ProcessCommand(&models.Command{
		Player:               p2,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitSupplyDepot),
		X:                    intPtr(40),
		Y:                    intPtr(40),
		SecondsFromGameStart: 110,
	})
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           "Targeted Order",
		OrderName:            stringPtr("Attack Move"),
		X:                    intPtr(tilePixel(40)),
		Y:                    intPtr(tilePixel(40)),
		SecondsFromGameStart: 130,
	})

	events := engine.ReplayEvents()
	var zRush *ReplayEvent
	for i := range events {
		if events[i].EventType == "zergling_rush" {
			zRush = &events[i]
			break
		}
	}
	if zRush == nil {
		t.Fatalf("expected zergling_rush event")
	}
	if zRush.Second != 100 {
		t.Fatalf("expected zergling rush second 100, got %d", zRush.Second)
	}
	if zRush.LocationBaseOclock == nil || *zRush.LocationBaseOclock != 5 {
		t.Fatalf("expected inferred target at 5 o'clock base, got %+v", zRush.LocationBaseOclock)
	}
}

func TestProcessCommand_BunkerRushRequiresMarineAndEnemyPolygon(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 300, MapWidth: 128, MapHeight: 128}
	p1 := &models.Player{PlayerID: 1, SlotID: 1, Name: "Terran", Race: "Terran", Team: 1, Type: models.PlayerTypeHuman}
	p2 := &models.Player{PlayerID: 2, SlotID: 2, Name: "Protoss", Race: "Protoss", Team: 2, Type: models.PlayerTypeHuman}
	engine := NewEngine(replay, []*models.Player{p1, p2}, rushProxyTestMapContext())

	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeTrain,
		UnitType:             stringPtr(models.GeneralUnitMarine),
		SecondsFromGameStart: 120,
	})
	engine.ProcessCommand(&models.Command{
		Player:               p2,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitPylon),
		X:                    intPtr(40),
		Y:                    intPtr(40),
		SecondsFromGameStart: 150,
	})
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitBunker),
		X:                    intPtr(40),
		Y:                    intPtr(40),
		SecondsFromGameStart: 180,
	})

	events := engine.ReplayEvents()
	found := false
	for _, event := range events {
		if event.EventType == "bunker_rush" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected bunker_rush event with marine prerequisite")
	}
}

func TestProcessCommand_ProxyGateRequiresTwoHumanAndMidPlacement(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 300, MapWidth: 128, MapHeight: 128}
	p1 := &models.Player{PlayerID: 1, SlotID: 1, Name: "P1", Race: "Protoss", Team: 1, Type: models.PlayerTypeHuman}
	p2 := &models.Player{PlayerID: 2, SlotID: 2, Name: "P2", Race: "Protoss", Team: 2, Type: models.PlayerTypeHuman}
	engine := NewEngine(replay, []*models.Player{p1, p2}, rushProxyTestMapContext())

	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitGateway),
		X:                    intPtr(24),
		Y:                    intPtr(24),
		SecondsFromGameStart: 200,
	})

	events := engine.ReplayEvents()
	found := false
	for _, event := range events {
		if event.EventType == "proxy_gate" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected proxy_gate event for valid midpoint placement")
	}
}

func TestProcessCommand_WorkerPressureBeforeFiveMinutesIsScout(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 400, MapWidth: 128, MapHeight: 128}
	p1 := &models.Player{PlayerID: 1, SlotID: 1, Name: "T1", Race: "Terran", Team: 1, Type: models.PlayerTypeHuman}
	p2 := &models.Player{PlayerID: 2, SlotID: 2, Name: "Z2", Race: "Zerg", Team: 2, Type: models.PlayerTypeHuman}
	engine := NewEngine(replay, []*models.Player{p1, p2}, rushProxyTestMapContext())

	engine.ProcessCommand(&models.Command{
		Player:               p2,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitSpawningPool),
		X:                    intPtr(40),
		Y:                    intPtr(40),
		SecondsFromGameStart: 50,
	})
	for i := 0; i < 5; i++ {
		engine.ProcessCommand(&models.Command{
			Player:               p1,
			ActionType:           "Targeted Order",
			OrderName:            stringPtr("Attack Move"),
			X:                    intPtr(tilePixel(40)),
			Y:                    intPtr(tilePixel(40)),
			SecondsFromGameStart: 120 + i,
		})
	}

	events := engine.ReplayEvents()
	for _, event := range events {
		if event.EventType != "scout" {
			continue
		}
		if len(event.AttackUnitTypes) != 1 || event.AttackUnitTypes[0] != models.GeneralUnitSCV {
			t.Fatalf("expected scout worker payload to be [SCV], got %v", event.AttackUnitTypes)
		}
		return
	}
	t.Fatalf("expected scout event after early worker pressure")
}

func rushProxyTestMapContext() *models.ReplayMapContext {
	return &models.ReplayMapContext{
		StartLocations: []models.MapStartLocation{
			{X: tilePixel(8), Y: tilePixel(8), SlotID: 1},
			{X: tilePixel(40), Y: tilePixel(40), SlotID: 2},
		},
		Layout: &models.MapContextLayout{
			Bases: []models.MapContextBase{
				{
					Name:   "start-a",
					Kind:   "start",
					Clock:  11,
					Center: models.MapResourcePosition{X: tilePixel(8), Y: tilePixel(8)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(6), Y: tilePixel(6)},
						{X: tilePixel(10), Y: tilePixel(6)},
						{X: tilePixel(10), Y: tilePixel(10)},
						{X: tilePixel(6), Y: tilePixel(10)},
					},
					NaturalExpansion: "nat-a",
				},
				{
					Name:   "nat-a",
					Kind:   "expa",
					Clock:  9,
					Center: models.MapResourcePosition{X: tilePixel(14), Y: tilePixel(10)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(12), Y: tilePixel(8)},
						{X: tilePixel(16), Y: tilePixel(8)},
						{X: tilePixel(16), Y: tilePixel(12)},
						{X: tilePixel(12), Y: tilePixel(12)},
					},
				},
				{
					Name:   "center",
					Kind:   "expa",
					Clock:  12,
					Center: models.MapResourcePosition{X: tilePixel(24), Y: tilePixel(24)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(22), Y: tilePixel(22)},
						{X: tilePixel(26), Y: tilePixel(22)},
						{X: tilePixel(26), Y: tilePixel(26)},
						{X: tilePixel(22), Y: tilePixel(26)},
					},
				},
				{
					Name:   "nat-b",
					Kind:   "expa",
					Clock:  7,
					Center: models.MapResourcePosition{X: tilePixel(34), Y: tilePixel(38)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(32), Y: tilePixel(36)},
						{X: tilePixel(36), Y: tilePixel(36)},
						{X: tilePixel(36), Y: tilePixel(40)},
						{X: tilePixel(32), Y: tilePixel(40)},
					},
				},
				{
					Name:   "start-b",
					Kind:   "start",
					Clock:  5,
					Center: models.MapResourcePosition{X: tilePixel(40), Y: tilePixel(40)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(38), Y: tilePixel(38)},
						{X: tilePixel(42), Y: tilePixel(38)},
						{X: tilePixel(42), Y: tilePixel(42)},
						{X: tilePixel(38), Y: tilePixel(42)},
					},
					NaturalExpansion: "nat-b",
				},
			},
		},
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestProcessCommand_ExpansionAndTakeoverAnnotateNaturalExpansions(t *testing.T) {
	replay := &models.Replay{MapWidth: 128, MapHeight: 128}
	p1 := &models.Player{PlayerID: 1, SlotID: 1, Name: "P1", Race: "Protoss", Team: 1}
	p2 := &models.Player{PlayerID: 2, SlotID: 2, Name: "P2", Race: "Protoss", Team: 2}
	ctx := &models.ReplayMapContext{
		StartLocations: []models.MapStartLocation{
			{X: tilePixel(8), Y: tilePixel(8), SlotID: 1},
			{X: tilePixel(40), Y: tilePixel(40), SlotID: 2},
		},
		Layout: &models.MapContextLayout{
			Bases: []models.MapContextBase{
				{
					Name:   "start-a",
					Kind:   "start",
					Clock:  11,
					Center: models.MapResourcePosition{X: tilePixel(8), Y: tilePixel(8)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(6), Y: tilePixel(6)},
						{X: tilePixel(10), Y: tilePixel(6)},
						{X: tilePixel(10), Y: tilePixel(10)},
						{X: tilePixel(6), Y: tilePixel(10)},
					},
					NaturalExpansion: "nat-a",
				},
				{
					Name:   "nat-a",
					Kind:   "expa",
					Clock:  9,
					Center: models.MapResourcePosition{X: tilePixel(14), Y: tilePixel(10)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(12), Y: tilePixel(8)},
						{X: tilePixel(16), Y: tilePixel(8)},
						{X: tilePixel(16), Y: tilePixel(12)},
						{X: tilePixel(12), Y: tilePixel(12)},
					},
				},
				{
					Name:   "start-b",
					Kind:   "start",
					Clock:  5,
					Center: models.MapResourcePosition{X: tilePixel(40), Y: tilePixel(40)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(38), Y: tilePixel(38)},
						{X: tilePixel(42), Y: tilePixel(38)},
						{X: tilePixel(42), Y: tilePixel(42)},
						{X: tilePixel(38), Y: tilePixel(42)},
					},
					NaturalExpansion: "nat-b",
				},
				{
					Name:   "nat-b",
					Kind:   "expa",
					Clock:  7,
					Center: models.MapResourcePosition{X: tilePixel(34), Y: tilePixel(38)},
					Polygon: []models.MapPolygonPoint{
						{X: tilePixel(32), Y: tilePixel(36)},
						{X: tilePixel(36), Y: tilePixel(36)},
						{X: tilePixel(36), Y: tilePixel(40)},
						{X: tilePixel(32), Y: tilePixel(40)},
					},
				},
			},
		},
	}
	engine := NewEngine(replay, []*models.Player{p1, p2}, ctx)

	engine.ProcessCommand(&models.Command{
		Player:               p2,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(34),
		Y:                    intPtr(38),
		SecondsFromGameStart: 50,
	})
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(14),
		Y:                    intPtr(10),
		SecondsFromGameStart: 60,
	})
	engine.ProcessCommand(&models.Command{
		Player:               p1,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitNexus),
		X:                    intPtr(34),
		Y:                    intPtr(38),
		SecondsFromGameStart: 120,
	})

	entries := engine.Entries()
	joined := ""
	for _, entry := range entries {
		joined += entry.Description + "\n"
	}
	if !strings.Contains(joined, "P1 expands to an expa near 9 (their natural expansion)") {
		t.Fatalf("expected own natural expansion annotation, got entries: %s", joined)
	}
	if !strings.Contains(joined, "P1 takes over an expa near 7 (natural expansion of at 5)") {
		t.Fatalf("expected other-player natural takeover annotation, got entries: %s", joined)
	}
	if strings.Contains(joined, "(not a natural expansion)") {
		t.Fatalf("did not expect non-natural annotation: %s", joined)
	}
}

func tilePixel(tile int) int {
	return tile*32 + 16
}

func intPtr(value int) *int {
	return &value
}
