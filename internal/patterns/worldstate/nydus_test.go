package worldstate

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

// enemyStartTile / homeStartTile are tile coordinates of the two mains in
// rushProxyTestMapContext. BuildNydusExit is a build order, so its raw command
// coords are in tiles (cmdenrich converts to pixels).
const (
	enemyStartTile = 40 // start-b, player 2's main
	homeStartTile  = 8  // start-a, player 1's main
)

func nydusEngine() (*Engine, *models.Player, *models.Player) {
	replay := &models.Replay{DurationSeconds: 1200, MapWidth: 128, MapHeight: 128}
	z := &models.Player{PlayerID: 1, SlotID: 1, Name: "Z", Race: "Zerg", Team: 1, Type: models.PlayerTypeHuman}
	t := &models.Player{PlayerID: 2, SlotID: 2, Name: "T", Race: "Terran", Team: 2, Type: models.PlayerTypeHuman}
	return NewEngine(replay, []*models.Player{z, t}, rushProxyTestMapContext()), z, t
}

func buildNydusExit(engine *Engine, p *models.Player, tile, sec int) {
	engine.ProcessCommand(&models.Command{
		Player:               p,
		ActionType:           "TargetedOrder",
		OrderName:            stringPtr(models.UnitOrderBuildNydusExit),
		X:                    intPtr(tile),
		Y:                    intPtr(tile),
		SecondsFromGameStart: sec,
	})
}

func nydusEvents(engine *Engine) []ReplayEvent {
	out := []ReplayEvent{}
	for _, ev := range engine.ReplayEvents() {
		if ev.EventType == "nydus_attack" {
			out = append(out, ev)
		}
	}
	return out
}

// Realistic path: a forward exit followed by the attacker operating units at
// the enemy main (post-placement activity) confirms an army insertion. This is
// the primary detector — EnterNydusCanal is absent from real replays.
func TestNydus_ForwardExitWithFollowupActivity(t *testing.T) {
	engine, z, _ := nydusEngine()
	buildNydusExit(engine, z, enemyStartTile, 400)
	for i := 0; i < 6; i++ {
		engine.ProcessCommand(&models.Command{
			Player:               z,
			ActionType:           "Move",
			X:                    intPtr(tilePixel(enemyStartTile)),
			Y:                    intPtr(tilePixel(enemyStartTile)),
			SecondsFromGameStart: 405 + i*3,
		})
	}

	events := nydusEvents(engine)
	if len(events) != 1 {
		t.Fatalf("expected 1 nydus_attack event, got %d", len(events))
	}
	ev := events[0]
	if ev.Second != 400 {
		t.Fatalf("expected event at exit placement second 400, got %d", ev.Second)
	}
	if ev.SourceReplayPlayerID == nil || *ev.SourceReplayPlayerID != 1 {
		t.Fatalf("expected source player 1, got %v", ev.SourceReplayPlayerID)
	}
	if ev.TargetReplayPlayerID == nil || *ev.TargetReplayPlayerID != 2 {
		t.Fatalf("expected target player 2, got %v", ev.TargetReplayPlayerID)
	}
	if ev.LocationBaseOclock == nil || *ev.LocationBaseOclock != 5 {
		t.Fatalf("expected exit at enemy main (5 o'clock), got %v", ev.LocationBaseOclock)
	}
}

// When explicit EnterNydusCanal orders ARE present, a burst corroborates on its
// own (the rare-but-supported teleport tier).
func TestNydus_ForwardExitWithTeleportBurst(t *testing.T) {
	engine, z, _ := nydusEngine()
	buildNydusExit(engine, z, enemyStartTile, 400)
	for i := 0; i < 3; i++ {
		engine.ProcessCommand(&models.Command{
			Player:               z,
			ActionType:           "TargetedOrder",
			OrderName:            stringPtr(models.UnitOrderEnterNydusCanal),
			X:                    intPtr(tilePixel(homeStartTile)),
			Y:                    intPtr(tilePixel(homeStartTile)),
			SecondsFromGameStart: 402 + i,
		})
	}

	if events := nydusEvents(engine); len(events) != 1 {
		t.Fatalf("expected 1 nydus_attack event from teleport burst, got %d", len(events))
	}
}

// A lone forward exit with no follow-up (vision / map-control canal) must NOT
// fire — mirrors the drops pass's homecoming suppression.
func TestNydus_UnconfirmedForwardExitDoesNotFire(t *testing.T) {
	engine, z, _ := nydusEngine()
	buildNydusExit(engine, z, enemyStartTile, 400)

	if events := nydusEvents(engine); len(events) != 0 {
		t.Fatalf("expected no nydus_attack for an unconfirmed forward exit, got %d", len(events))
	}
}

// An exit at the player's own main is defensive / mobility, never offensive.
func TestNydus_HomeExitIsNotOffensive(t *testing.T) {
	engine, z, _ := nydusEngine()
	buildNydusExit(engine, z, homeStartTile, 400)
	for i := 0; i < 6; i++ {
		engine.ProcessCommand(&models.Command{
			Player:               z,
			ActionType:           "Move",
			X:                    intPtr(tilePixel(homeStartTile)),
			Y:                    intPtr(tilePixel(homeStartTile)),
			SecondsFromGameStart: 405 + i*3,
		})
	}

	if events := nydusEvents(engine); len(events) != 0 {
		t.Fatalf("expected no nydus_attack for a home exit, got %d", len(events))
	}
}
