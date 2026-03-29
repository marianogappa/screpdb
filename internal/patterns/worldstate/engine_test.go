package worldstate

import (
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

func TestProcessCommand_EmitsZerglingRushBeforeTwoTwenty(t *testing.T) {
	replay := &models.Replay{}
	zerg := &models.Player{PlayerID: 2, Name: "Z", Race: "Zerg", Team: 2}
	engine := NewEngine(replay, []*models.Player{zerg}, nil)

	engine.ProcessCommand(&models.Command{
		Player:               zerg,
		ActionType:           models.ActionTypeUnitMorph,
		UnitType:             stringPtr(models.GeneralUnitZergling),
		SecondsFromGameStart: 139,
	})

	entries := engine.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 event, got %d", len(entries))
	}
	if entries[0].Type != "rush" {
		t.Fatalf("expected rush event type, got %q", entries[0].Type)
	}
	if entries[0].Description != "Z Zergling rushes" {
		t.Fatalf("unexpected rush event description: %q", entries[0].Description)
	}
}

func stringPtr(value string) *string {
	return &value
}
