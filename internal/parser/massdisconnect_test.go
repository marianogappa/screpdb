package parser

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

// Scenarios mirror the issue #358 evidence: replay 209 (7-cluster saver
// disconnect), replay 74 (2-cluster saver disconnect — guards against a
// count-based threshold), replay 206 (legitimate win with a genuine mid-game
// drop), and replay 114 (lobby abort whose drop cluster leaves condition 4
// unmet).

func mdPlayers(n int) []*models.Player {
	players := make([]*models.Player, 0, n)
	for i := 0; i < n; i++ {
		players = append(players, &models.Player{PlayerID: byte(i), Name: string(rune('A' + i)), Team: byte(i%4 + 1)})
	}
	return players
}

func mdLeave(p *models.Player, frame int32, sec int, reason string) *models.Command {
	return &models.Command{
		Player:               p,
		ActionType:           "Leave Game",
		Frame:                frame,
		SecondsFromGameStart: sec,
		LeaveReason:          &reason,
	}
}

func TestDetectMassDisconnectEnd_SevenPlayerCluster(t *testing.T) {
	players := mdPlayers(8)
	saver := byte(0)
	var cmds []*models.Command
	for _, p := range players[1:] {
		cmds = append(cmds, mdLeave(p, 4125, 173, "Dropped"))
	}
	md := DetectMassDisconnectEnd(players, cmds, &saver, 176)
	if md == nil || md.SaverPID != saver || md.ClusterSecond != 173 {
		t.Fatalf("expected saver-disconnect verdict at 173, got %+v", md)
	}
}

func TestDetectMassDisconnectEnd_TwoPlayerClusterAfterQuits(t *testing.T) {
	players := mdPlayers(8)
	saver := byte(0)
	cmds := []*models.Command{
		mdLeave(players[1], 1513, 63, "Quit"),
		mdLeave(players[2], 5094, 212, "Quit"),
		mdLeave(players[3], 20000, 833, "Quit"),
		mdLeave(players[4], 30000, 1250, "Quit"),
		mdLeave(players[5], 40000, 1666, "Quit"),
		mdLeave(players[6], 57000, 2381, "Dropped"),
		mdLeave(players[7], 57000, 2381, "Dropped"),
	}
	md := DetectMassDisconnectEnd(players, cmds, &saver, 2384)
	if md == nil || md.ClusterSecond != 2381 {
		t.Fatalf("a 2-drop cluster at the end is still a saver disconnect, got %+v", md)
	}
}

func TestDetectMassDisconnectEnd_GenuineMidGameDropIsNotFlagged(t *testing.T) {
	players := mdPlayers(6)
	saver := byte(0)
	cmds := []*models.Command{
		mdLeave(players[1], 1513, 63, "Quit"),
		mdLeave(players[2], 5094, 212, "Quit"),
		mdLeave(players[3], 35652, 1485, "Quit"),
		mdLeave(players[4], 39090, 1628, "Dropped"),
		mdLeave(players[5], 41646, 1735, "Quit"),
	}
	if md := DetectMassDisconnectEnd(players, cmds, &saver, 1740); md != nil {
		t.Fatalf("staggered quits with one mid-game drop is a legitimate win, got %+v", md)
	}
}

func TestDetectMassDisconnectEnd_LobbyAbortClusterFailsLeaveCount(t *testing.T) {
	players := mdPlayers(8)
	saver := byte(0)
	cmds := []*models.Command{
		mdLeave(players[1], 40, 1, "Dropped"),
		mdLeave(players[2], 40, 1, "Dropped"),
	}
	if md := DetectMassDisconnectEnd(players, cmds, &saver, 2); md != nil {
		t.Fatalf("a lobby abort where 5 players have no leave at all must not be flagged, got %+v", md)
	}
}

func TestDetectMassDisconnectEnd_SaverWithOwnLeaveIsNotFlagged(t *testing.T) {
	players := mdPlayers(4)
	saver := byte(0)
	cmds := []*models.Command{
		mdLeave(players[0], 4000, 166, "Quit"),
		mdLeave(players[1], 4125, 172, "Dropped"),
		mdLeave(players[2], 4125, 172, "Dropped"),
		mdLeave(players[3], 4125, 172, "Dropped"),
	}
	if md := DetectMassDisconnectEnd(players, cmds, &saver, 175); md != nil {
		t.Fatalf("a saver with a real leave command is not a saver disconnect, got %+v", md)
	}
}

func TestDetectMassDisconnectEnd_MidGameClusterIsNotFlagged(t *testing.T) {
	players := mdPlayers(4)
	saver := byte(0)
	cmds := []*models.Command{
		mdLeave(players[1], 4000, 166, "Dropped"),
		mdLeave(players[2], 4000, 166, "Dropped"),
		mdLeave(players[3], 9000, 375, "Quit"),
	}
	if md := DetectMassDisconnectEnd(players, cmds, &saver, 600); md != nil {
		t.Fatalf("a drop cluster minutes before the end is not a saver disconnect, got %+v", md)
	}
}

func TestDetectMassDisconnectEnd_UnknownSaver(t *testing.T) {
	players := mdPlayers(3)
	cmds := []*models.Command{
		mdLeave(players[1], 4125, 172, "Dropped"),
		mdLeave(players[2], 4125, 172, "Dropped"),
	}
	if md := DetectMassDisconnectEnd(players, cmds, nil, 175); md != nil {
		t.Fatalf("without a known saver there is no verdict, got %+v", md)
	}
}
