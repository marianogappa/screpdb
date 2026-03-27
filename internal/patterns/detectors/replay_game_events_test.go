package detectors

import (
	"strings"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
	"github.com/marianogappa/screpdb/internal/patterns/worldstate"
)

func TestGameEventsReplayDetector_IncludesRaceAndZerglingRushEvents(t *testing.T) {
	replay := &models.Replay{ID: 1}
	protoss := &models.Player{PlayerID: 1, Name: "P", Race: "Protoss", Team: 1}
	zerg := &models.Player{PlayerID: 2, Name: "Z", Race: "Zerg", Team: 2}
	players := []*models.Player{protoss, zerg}
	ws := worldstate.NewEngine(replay, players, nil)

	ws.ProcessCommand(&models.Command{
		Player:               protoss,
		ActionType:           models.ActionTypeBuild,
		UnitType:             stringPtr(models.GeneralUnitHatchery),
		SecondsFromGameStart: 120,
	})
	ws.ProcessCommand(&models.Command{
		Player:               zerg,
		ActionType:           models.ActionTypeUnitMorph,
		UnitType:             stringPtr(models.GeneralUnitZergling),
		SecondsFromGameStart: 130,
	})

	detector := NewGameEventsReplayDetector()
	detector.Initialize(replay, players)
	detector.SetWorldState(ws)
	detector.Finalize()

	result := detector.GetResult()
	if result == nil || result.ValueString == nil {
		t.Fatalf("expected game events detector result")
	}

	raw := *result.ValueString
	if !strings.Contains(raw, `"type":"race"`) || !strings.Contains(raw, "becomes Zerg") {
		t.Fatalf("expected race event in payload: %s", raw)
	}
	if !strings.Contains(raw, `"type":"rush"`) || !strings.Contains(raw, "Zergling rushes") {
		t.Fatalf("expected zergling rush event in payload: %s", raw)
	}
}

