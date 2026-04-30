package parser

import (
	"reflect"
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

// p builds a non-observer human player.
func p(pid byte, slot uint16) *models.Player {
	return &models.Player{PlayerID: pid, SlotID: slot, Type: "Human"}
}

// allianceCmd builds an Alliance command for issuer with the given slot IDs as
// the new alliance set.
func allianceCmd(sec int, issuer *models.Player, slotIDs ...byte) *models.Command {
	ids := make([]int64, 0, len(slotIDs))
	for _, s := range slotIDs {
		ids = append(ids, int64(s))
	}
	return &models.Command{
		ActionType:           "Alliance",
		SecondsFromGameStart: sec,
		Player:               issuer,
		AlliancePlayerIDs:    &ids,
	}
}

// expectTeams verifies the topology of the last snapshot is the given list of
// player_id sets (order-insensitive within and across teams).
func expectTeams(t *testing.T, got [][]byte, want [][]byte) {
	t.Helper()
	normalize := func(in [][]byte) [][]byte {
		out := make([][]byte, len(in))
		for i, t := range in {
			cp := append([]byte(nil), t...)
			out[i] = cp
		}
		return out
	}
	g := normalize(got)
	w := normalize(want)
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("topology mismatch:\n  got:  %v\n  want: %v", g, w)
	}
}

func TestAnalyzeAlliances_NoCommands_AllSolo(t *testing.T) {
	players := []*models.Player{p(1, 1), p(2, 2), p(3, 3)}
	res := AnalyzeAlliances(players, nil, 600)

	if res.AnyMutualResolved {
		t.Fatalf("expected no mutual alliances, got AnyMutualResolved=true")
	}
	if res.TeamStackingFlag {
		t.Fatalf("expected no stacking flag")
	}
	if len(res.Snapshots) != 1 {
		t.Fatalf("expected single (initial) snapshot, got %d", len(res.Snapshots))
	}
	expectTeams(t, res.Snapshots[0].Teams, [][]byte{{1}, {2}, {3}})
	for _, pid := range []byte{1, 2, 3} {
		if _, ok := res.ResolvedTeams[pid]; !ok {
			t.Fatalf("expected resolved team for pid %d", pid)
		}
	}
}

func TestAnalyzeAlliances_FullyMutual_2v2_NoStacking(t *testing.T) {
	a, b, c, d := p(1, 1), p(2, 2), p(3, 3), p(4, 4)
	players := []*models.Player{a, b, c, d}
	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2),
		allianceCmd(11, b, 1, 2),
		allianceCmd(12, c, 3, 4),
		allianceCmd(13, d, 3, 4),
	}
	res := AnalyzeAlliances(players, cmds, 1200)

	if !res.AnyMutualResolved {
		t.Fatalf("expected mutual alliance to resolve")
	}
	if res.TeamStackingFlag {
		t.Fatalf("expected no stacking for balanced 2v2")
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	expectTeams(t, last.Teams, [][]byte{{1, 2}, {3, 4}})
	if last.Stacking {
		t.Fatalf("balanced 2v2 should not be stacking")
	}
	// ResolvedTeams should reflect the 2v2 partition.
	if res.ResolvedTeams[1] != res.ResolvedTeams[2] {
		t.Fatalf("expected 1 and 2 on same team")
	}
	if res.ResolvedTeams[3] != res.ResolvedTeams[4] {
		t.Fatalf("expected 3 and 4 on same team")
	}
	if res.ResolvedTeams[1] == res.ResolvedTeams[3] {
		t.Fatalf("expected different teams for 1/2 vs 3/4")
	}
}

func TestAnalyzeAlliances_OneWayAlliance_NoMutual(t *testing.T) {
	a, b := p(1, 1), p(2, 2)
	players := []*models.Player{a, b, p(3, 3)}
	// a allies with b, but b never reciprocates.
	cmds := []*models.Command{allianceCmd(30, a, 1, 2)}
	res := AnalyzeAlliances(players, cmds, 600)

	if res.AnyMutualResolved {
		t.Fatalf("one-way alliance should not be considered mutual")
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	expectTeams(t, last.Teams, [][]byte{{1}, {2}, {3}})
}

func TestAnalyzeAlliances_StackingBand_ExceedsThreshold(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}
	// Phase 1: 2v3 (1+2 vs 3+4+5) starting at sec=10. Lasts > 5min then changes.
	// Sec values are seconds-from-game-start.
	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2),
		allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4, 5),
		allianceCmd(10, d, 3, 4, 5),
		allianceCmd(10, e, 3, 4, 5),
		// At sec=400 (>5min after stacking starts), one player drops out → 2v2v1.
		allianceCmd(400, e, 5),
	}
	res := AnalyzeAlliances(players, cmds, 900)

	if !res.TeamStackingFlag {
		t.Fatalf("expected stacking flag after >5min of 2v3")
	}
}

func TestAnalyzeAlliances_TransientImbalance_BelowThreshold(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}
	// 3v2 forms briefly (60s) then resolves to a balanced 2v2v1.
	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2, 3),
		allianceCmd(10, b, 1, 2, 3),
		allianceCmd(10, c, 1, 2, 3),
		allianceCmd(10, d, 4, 5),
		allianceCmd(10, e, 4, 5),
		// Resolve at sec=70 by removing C from the trio.
		allianceCmd(70, a, 1, 2),
		allianceCmd(70, b, 1, 2),
		allianceCmd(70, c, 3),
	}
	res := AnalyzeAlliances(players, cmds, 1200)

	if res.TeamStackingFlag {
		t.Fatalf("expected no stacking flag for sub-threshold 3v2 band")
	}
}

func TestAnalyzeAlliances_ComputerExcluded(t *testing.T) {
	human1 := p(1, 1)
	human2 := p(2, 2)
	computer := &models.Player{PlayerID: 255, SlotID: 3, Type: "Computer"}
	players := []*models.Player{human1, human2, computer}

	cmds := []*models.Command{
		allianceCmd(10, human1, 1, 2),
		allianceCmd(10, human2, 1, 2),
	}
	res := AnalyzeAlliances(players, cmds, 600)

	for _, snap := range res.Snapshots {
		for _, team := range snap.Teams {
			for _, pid := range team {
				if pid == 255 {
					t.Fatalf("computer pid=255 leaked into topology: %v", snap.Teams)
				}
			}
		}
	}
}

func TestAnalyzeAlliances_ObserverExcluded(t *testing.T) {
	a, b := p(1, 1), p(2, 2)
	obs := &models.Player{PlayerID: 3, SlotID: 3, Type: "Human", IsObserver: true}
	players := []*models.Player{a, b, obs}
	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2),
		allianceCmd(10, b, 1, 2),
	}
	res := AnalyzeAlliances(players, cmds, 600)
	for _, snap := range res.Snapshots {
		for _, team := range snap.Teams {
			for _, pid := range team {
				if pid == 3 {
					t.Fatalf("observer pid=3 leaked into topology")
				}
			}
		}
	}
}

// leaveCmd builds a Leave Game command for the given player at sec.
func leaveCmd(sec int, issuer *models.Player) *models.Command {
	return &models.Command{
		ActionType:           "Leave Game",
		SecondsFromGameStart: sec,
		Player:               issuer,
	}
}

func TestDeriveWinnersFromLeaves_LargestRemainingTeamWins(t *testing.T) {
	// 2v2: A,B (team 1) vs C,D (team 2). A and B leave → team 2 wins.
	a := &models.Player{PlayerID: 1, SlotID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, SlotID: 2, Type: "Human", Team: 1}
	c := &models.Player{PlayerID: 3, SlotID: 3, Type: "Human", Team: 2}
	d := &models.Player{PlayerID: 4, SlotID: 4, Type: "Human", Team: 2}
	players := []*models.Player{a, b, c, d}
	cmds := []*models.Command{
		leaveCmd(600, a),
		leaveCmd(700, b),
	}
	DeriveWinnersFromLeaves(players, cmds, nil)
	if a.IsWinner || b.IsWinner {
		t.Fatalf("team 1 leavers should not be winners")
	}
	if !c.IsWinner || !d.IsWinner {
		t.Fatalf("team 2 should win (largest remaining)")
	}
}

func TestDeriveWinnersFromLeaves_NoLeaves_NoWinner(t *testing.T) {
	a := &models.Player{PlayerID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, Type: "Human", Team: 2}
	DeriveWinnersFromLeaves([]*models.Player{a, b}, nil, nil)
	if a.IsWinner || b.IsWinner {
		t.Fatalf("no winners should be set when no leave commands recorded")
	}
}

func TestDeriveWinnersFromLeaves_RepSaverVirtualLeave(t *testing.T) {
	// 1v1: A (team 1) vs B (team 2). B is the rep saver — no recorded leave.
	// A leaves explicitly. Without rep saver: only A leaves, team 1 size = 0,
	// team 2 size = 1, unique max → team 2 wins. With rep saver added, both
	// teams hit zero (1-vs-1), tie-break: last leaver = B (rep saver) → team 2.
	a := &models.Player{PlayerID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, Type: "Human", Team: 2}
	players := []*models.Player{a, b}
	cmds := []*models.Command{leaveCmd(800, a)}
	saverPID := byte(2)
	DeriveWinnersFromLeaves(players, cmds, &saverPID)
	if a.IsWinner {
		t.Fatalf("team 1 (left) should not win")
	}
	if !b.IsWinner {
		t.Fatalf("team 2 (rep saver) should win via virtual-leave tie-break")
	}
}

func TestDeriveWinnersFromLeaves_TieNoTotalLeave_NoWinner(t *testing.T) {
	// 2v2: only one player from each team leaves. teamSizes = {1:1, 2:1}.
	// Tie at maxSize=1, len(leavers) != nonObsCount → no winner.
	a := &models.Player{PlayerID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, Type: "Human", Team: 1}
	c := &models.Player{PlayerID: 3, Type: "Human", Team: 2}
	d := &models.Player{PlayerID: 4, Type: "Human", Team: 2}
	players := []*models.Player{a, b, c, d}
	cmds := []*models.Command{leaveCmd(500, a), leaveCmd(600, c)}
	DeriveWinnersFromLeaves(players, cmds, nil)
	for _, p := range players {
		if p.IsWinner {
			t.Fatalf("no winner expected with sizes tied and not all left")
		}
	}
}

func TestDeriveWinnersFromLeaves_AllLeftLastLeaverTeamWins(t *testing.T) {
	// 2v2 where everyone on team 1 leaves first, then everyone on team 2.
	// Sizes after leaves both 0; len(leaverPIDs) == nonObsCount → tie-break.
	// Last leaver is on team 2 → team 2 wins.
	a := &models.Player{PlayerID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, Type: "Human", Team: 1}
	c := &models.Player{PlayerID: 3, Type: "Human", Team: 2}
	d := &models.Player{PlayerID: 4, Type: "Human", Team: 2}
	players := []*models.Player{a, b, c, d}
	cmds := []*models.Command{
		leaveCmd(100, a), leaveCmd(200, b),
		leaveCmd(300, c), leaveCmd(400, d),
	}
	DeriveWinnersFromLeaves(players, cmds, nil)
	if !c.IsWinner || !d.IsWinner {
		t.Fatalf("expected team 2 to win via last-leaver tie-break")
	}
}

func TestDeriveWinnersFromLeaves_AllSameTeam_NoWinner(t *testing.T) {
	// Every player on team 1 (the post-screp-fail case before our derivation
	// kicks in). The algorithm bails because len(teamSizes) < 2.
	a := &models.Player{PlayerID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, Type: "Human", Team: 1}
	players := []*models.Player{a, b}
	cmds := []*models.Command{leaveCmd(100, a)}
	DeriveWinnersFromLeaves(players, cmds, nil)
	if a.IsWinner || b.IsWinner {
		t.Fatalf("no winners with single-team game")
	}
}

func TestDeriveWinnersFromLeaves_AllComputerTeam_NoWinner(t *testing.T) {
	// Team 1 is all human, team 2 is all computer. Algorithm bails because
	// computers never leave.
	a := &models.Player{PlayerID: 1, Type: "Human", Team: 1}
	b := &models.Player{PlayerID: 2, Type: "Computer", Team: 2}
	players := []*models.Player{a, b}
	cmds := []*models.Command{leaveCmd(100, a)}
	DeriveWinnersFromLeaves(players, cmds, nil)
	if a.IsWinner || b.IsWinner {
		t.Fatalf("expected no winner when one team is all computers")
	}
}

func TestIsStacking(t *testing.T) {
	cases := []struct {
		name  string
		teams [][]byte
		want  bool
	}{
		{"all_solo", [][]byte{{1}, {2}, {3}}, false},
		{"2v2", [][]byte{{1, 2}, {3, 4}}, false},
		{"2v2v1", [][]byte{{1, 2}, {3, 4}, {5}}, false},
		{"3v2", [][]byte{{1, 2, 3}, {4, 5}}, true},
		{"3v2v2", [][]byte{{1, 2, 3}, {4, 5}, {6, 7}}, true},
		{"3v2v2v1", [][]byte{{1, 2, 3}, {4, 5}, {6, 7}, {8}}, true},
		{"3v3", [][]byte{{1, 2, 3}, {4, 5, 6}}, false},
		{"3v1v1v1", [][]byte{{1, 2, 3}, {4}, {5}, {6}}, false}, // only one non-solo team
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := isStacking(c.teams)
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
