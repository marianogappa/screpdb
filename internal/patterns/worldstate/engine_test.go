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
	if entries[0].Type != "race" {
		t.Fatalf("expected race event type, got %q", entries[0].Type)
	}
	if entries[0].Description != "P becomes Zerg" {
		t.Fatalf("unexpected race event description: %q", entries[0].Description)
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

