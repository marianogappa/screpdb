package worldstate

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func leaveCmd(p *models.Player, sec int, reason string) *models.Command {
	return &models.Command{
		Player:               p,
		ActionType:           "Leave Game",
		SecondsFromGameStart: sec,
		LeaveReason:          &reason,
	}
}

func TestEmitLeaveGameEvents_QuitVsDropTypes(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 600}
	a := &models.Player{PlayerID: 1, Name: "A", Race: "Terran", Team: 1}
	b := &models.Player{PlayerID: 2, Name: "B", Race: "Zerg", Team: 2}
	c := &models.Player{PlayerID: 3, Name: "C", Race: "Protoss", Team: 3}
	engine := NewEngine(replay, []*models.Player{a, b, c}, nil)
	engine.ProcessCommand(leaveCmd(a, 100, "Quit"))
	engine.ProcessCommand(leaveCmd(b, 300, "Dropped"))
	engine.Finalize()

	byType := map[string]int{}
	for _, ev := range engine.ReplayEvents() {
		byType[ev.EventType]++
	}
	if byType["leave_game"] != 1 || byType["player_dropped"] != 1 || byType["mass_disconnect"] != 0 {
		t.Fatalf("expected 1 leave_game + 1 player_dropped, got %v", byType)
	}
}

func TestEmitLeaveGameEvents_MassDisconnectCondensesCluster(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 176}
	players := []*models.Player{
		{PlayerID: 0, Name: "Saver", Race: "Terran", Team: 4},
		{PlayerID: 1, Name: "B", Race: "Zerg", Team: 3},
		{PlayerID: 2, Name: "C", Race: "Protoss", Team: 2},
		{PlayerID: 3, Name: "D", Race: "Terran", Team: 1},
	}
	engine := NewEngine(replay, players, nil)
	engine.ProcessCommand(leaveCmd(players[1], 100, "Quit"))
	for _, p := range players[2:] {
		engine.ProcessCommand(leaveCmd(p, 173, "Dropped"))
	}
	engine.SetMassDisconnectEnd(0, 173)
	engine.Finalize()

	var leaves, drops, mass int
	var massDescr string
	for _, ev := range engine.ReplayEvents() {
		switch ev.EventType {
		case "leave_game":
			leaves++
		case "player_dropped":
			drops++
		case "mass_disconnect":
			mass++
			if ev.SourceReplayPlayerID == nil || *ev.SourceReplayPlayerID != 0 {
				t.Fatalf("mass_disconnect must be attributed to the saver, got %+v", ev)
			}
		}
	}
	for _, entry := range engine.Entries() {
		if entry.Type == "mass_disconnect" {
			massDescr = entry.Description
		}
	}
	if leaves != 1 || drops != 0 || mass != 1 {
		t.Fatalf("expected the cluster condensed into one mass_disconnect (1 leave_game, 0 player_dropped), got leaves=%d drops=%d mass=%d", leaves, drops, mass)
	}
	if massDescr != "Saver lost connection — the game ended without a result" {
		t.Fatalf("unexpected mass_disconnect description: %q", massDescr)
	}
}

func TestEmitLeaveGameEvents_GenuineDropOutsideClusterSurvives(t *testing.T) {
	replay := &models.Replay{DurationSeconds: 288}
	players := []*models.Player{
		{PlayerID: 0, Name: "Saver", Race: "Terran", Team: 4},
		{PlayerID: 1, Name: "B", Race: "Zerg", Team: 3},
		{PlayerID: 2, Name: "C", Race: "Protoss", Team: 2},
		{PlayerID: 3, Name: "D", Race: "Terran", Team: 1},
	}
	engine := NewEngine(replay, players, nil)
	engine.ProcessCommand(leaveCmd(players[1], 164, "Dropped"))
	for _, p := range players[2:] {
		engine.ProcessCommand(leaveCmd(p, 285, "Dropped"))
	}
	engine.SetMassDisconnectEnd(0, 285)
	engine.Finalize()

	var drops, mass int
	for _, ev := range engine.ReplayEvents() {
		switch ev.EventType {
		case "player_dropped":
			drops++
		case "mass_disconnect":
			mass++
		}
	}
	if drops != 1 || mass != 1 {
		t.Fatalf("the genuine 164s drop must survive as player_dropped beside the condensed cluster, got drops=%d mass=%d", drops, mass)
	}
}
