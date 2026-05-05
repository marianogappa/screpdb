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

// genericCmd builds a non-Alliance, non-Leave command for inactivity-window
// math. action_type is intentionally vague; ComputeActivity counts every
// command regardless of type.
func genericCmd(sec int, issuer *models.Player) *models.Command {
	return &models.Command{
		ActionType:           "Right Click",
		SecondsFromGameStart: sec,
		Player:               issuer,
	}
}

// burst returns InactivityMinActions+1 commands at the given second to keep
// the player decisively above the "alive" threshold for the 60s window
// covering that second.
func burst(sec int, issuer *models.Player) []*models.Command {
	out := make([]*models.Command, 0, InactivityMinActions+1)
	for i := 0; i < InactivityMinActions+1; i++ {
		out = append(out, genericCmd(sec, issuer))
	}
	return out
}

// keepAliveSeries emits a sustained "alive" command stream for the given
// player from secStart to secEnd (inclusive), pacing >=20 commands per
// 60-second window.
func keepAliveSeries(secStart, secEnd int, issuer *models.Player) []*models.Command {
	out := []*models.Command{}
	for t := secStart; t <= secEnd; t++ {
		// 1 command per second is plenty (60/60 = 60 cmds/min ≥ 20).
		out = append(out, genericCmd(t, issuer))
	}
	return out
}

func emptyActivity() Activity {
	return Activity{
		StoppedSecByPID: map[byte]int{},
		LeaveSecByPID:   map[byte]int{},
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
	res := AnalyzeAlliances(players, nil, 600, emptyActivity())

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
	res := AnalyzeAlliances(players, cmds, 1200, emptyActivity())

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
	cmds := []*models.Command{allianceCmd(30, a, 1, 2)}
	res := AnalyzeAlliances(players, cmds, 600, emptyActivity())

	if res.AnyMutualResolved {
		t.Fatalf("one-way alliance should not be considered mutual")
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	expectTeams(t, last.Teams, [][]byte{{1}, {2}, {3}})
}

func TestAnalyzeAlliances_StackingBand_ExceedsThreshold(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}
	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2),
		allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4, 5),
		allianceCmd(10, d, 3, 4, 5),
		allianceCmd(10, e, 3, 4, 5),
		allianceCmd(400, e, 5),
	}
	res := AnalyzeAlliances(players, cmds, 900, emptyActivity())

	if !res.TeamStackingFlag {
		t.Fatalf("expected stacking flag after >5min of 2v3")
	}
	if res.StackingBandStartSec != 10 {
		t.Fatalf("expected stacking band to start at sec=10, got %d", res.StackingBandStartSec)
	}
}

func TestAnalyzeAlliances_TransientImbalance_BelowThreshold(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}
	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2, 3),
		allianceCmd(10, b, 1, 2, 3),
		allianceCmd(10, c, 1, 2, 3),
		allianceCmd(10, d, 4, 5),
		allianceCmd(10, e, 4, 5),
		allianceCmd(70, a, 1, 2),
		allianceCmd(70, b, 1, 2),
		allianceCmd(70, c, 3),
	}
	res := AnalyzeAlliances(players, cmds, 1200, emptyActivity())

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
	res := AnalyzeAlliances(players, cmds, 600, emptyActivity())

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
	res := AnalyzeAlliances(players, cmds, 600, emptyActivity())
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

// --- Effective-player / inactivity tests --------------------------------

// TestStacking_DeadPlayerIsExcluded — 5p, 2v3 forms at sec=10, but one of
// the three (player 5) goes silent after sec=200. With the inactivity-aware
// rule, the team containing the dead player shrinks to 2 → balanced 2v2 →
// no stacking flag.
func TestStacking_DeadPlayerIsExcluded(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}

	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2), allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4, 5), allianceCmd(10, d, 3, 4, 5), allianceCmd(10, e, 3, 4, 5),
	}
	// Active players keep playing throughout.
	for _, alive := range []*models.Player{a, b, c, d} {
		cmds = append(cmds, keepAliveSeries(0, 1800, alive)...)
	}
	// E plays through sec=200, then goes silent.
	cmds = append(cmds, keepAliveSeries(0, 200, e)...)

	activity := ComputeActivity(players, cmds, 1800)
	if _, ok := activity.StoppedSecByPID[5]; !ok {
		t.Fatalf("expected player 5 to be marked as stopped, got activity=%+v", activity)
	}

	res := AnalyzeAlliances(players, cmds, 1800, activity)
	if res.TeamStackingFlag {
		t.Fatalf("expected no stacking — dead player should be excluded from stacking check")
	}
}

// TestStacking_LeftPlayerIsExcluded — the surviving players form a 2v3
// alliance at sec=10 but player 5 leaves shortly after (sec=100, within
// the stacking threshold). After the leave, the topology is effectively
// 2v2 — the brief pre-leave 2v3 window is well below 300s, so no flag.
func TestStacking_LeftPlayerIsExcluded(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}

	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2), allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4, 5), allianceCmd(10, d, 3, 4, 5), allianceCmd(10, e, 3, 4, 5),
	}
	for _, alive := range []*models.Player{a, b, c, d, e} {
		cmds = append(cmds, keepAliveSeries(0, 100, alive)...)
	}
	cmds = append(cmds, leaveCmd(100, e))
	for _, alive := range []*models.Player{a, b, c, d} {
		cmds = append(cmds, keepAliveSeries(101, 1800, alive)...)
	}

	activity := ComputeActivity(players, cmds, 1800)
	if got := activity.LeaveSecByPID[5]; got != 100 {
		t.Fatalf("expected leaveSec[5]=100, got %d", got)
	}

	res := AnalyzeAlliances(players, cmds, 1800, activity)
	if res.TeamStackingFlag {
		t.Fatalf("expected no stacking — pre-leave 2v3 was only 90s; post-leave is 2v2")
	}
}

// TestStacking_LeaveAfterRealStackingDoesFire — the inverse: when the
// 2v3 actually persists past 300s before the leave, that's a real stacking
// band and the flag fires regardless of the late leave.
func TestStacking_LeaveAfterRealStackingDoesFire(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}

	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2), allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4, 5), allianceCmd(10, d, 3, 4, 5), allianceCmd(10, e, 3, 4, 5),
	}
	// All five active for >5 minutes after the alliance forms.
	for _, alive := range []*models.Player{a, b, c, d, e} {
		cmds = append(cmds, keepAliveSeries(0, 400, alive)...)
	}
	cmds = append(cmds, leaveCmd(400, e))
	for _, alive := range []*models.Player{a, b, c, d} {
		cmds = append(cmds, keepAliveSeries(401, 1800, alive)...)
	}

	activity := ComputeActivity(players, cmds, 1800)
	res := AnalyzeAlliances(players, cmds, 1800, activity)
	if !res.TeamStackingFlag {
		t.Fatalf("expected stacking — 2v3 lasted ~390s before the leave (>300s threshold)")
	}
}

// TestStacking_AllAliveStillFires — regression: 2v3 with all five active
// players throughout still flags stacking.
func TestStacking_AllAliveStillFires(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}

	cmds := []*models.Command{
		allianceCmd(10, a, 1, 2), allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4, 5), allianceCmd(10, d, 3, 4, 5), allianceCmd(10, e, 3, 4, 5),
	}
	for _, alive := range []*models.Player{a, b, c, d, e} {
		cmds = append(cmds, keepAliveSeries(0, 1800, alive)...)
	}

	activity := ComputeActivity(players, cmds, 1800)
	if len(activity.StoppedSecByPID) != 0 {
		t.Fatalf("expected no stop events for all-active game, got %v", activity.StoppedSecByPID)
	}

	res := AnalyzeAlliances(players, cmds, 1800, activity)
	if !res.TeamStackingFlag {
		t.Fatalf("expected stacking flag — all five active throughout, this is a real 2v3")
	}
}

// TestStacking_StoppedIsMonotonic — a player flickers (alive 0–200, dead
// 200–500, brief revival around 500–520, dead after). stoppedSec should be
// the LAST alive moment (~520), not 200. Encodes "no early collapse on
// short revivals".
func TestStacking_StoppedIsMonotonic(t *testing.T) {
	x := p(1, 1)
	cmds := []*models.Command{}
	cmds = append(cmds, keepAliveSeries(0, 200, x)...)
	// silence 200..500
	cmds = append(cmds, keepAliveSeries(500, 520, x)...)
	// silence after 520

	activity := ComputeActivity([]*models.Player{x}, cmds, 1800)
	stopped, ok := activity.StoppedSecByPID[1]
	if !ok {
		t.Fatalf("expected stoppedSec for player 1")
	}
	if stopped < 510 || stopped > 520 {
		t.Fatalf("expected stoppedSec near the late revival window (~510–520), got %d", stopped)
	}
}

// TestActivity_NeverActiveStopsAtZero — a player who never reaches the
// "alive" threshold is treated as stopped from sec 0.
func TestActivity_NeverActiveStopsAtZero(t *testing.T) {
	x := p(1, 1)
	// Ten commands across 600 seconds — far below 20-per-60s threshold.
	cmds := []*models.Command{
		genericCmd(0, x), genericCmd(60, x), genericCmd(120, x), genericCmd(180, x),
		genericCmd(240, x), genericCmd(300, x), genericCmd(360, x), genericCmd(420, x),
		genericCmd(480, x), genericCmd(540, x),
	}
	activity := ComputeActivity([]*models.Player{x}, cmds, 1200)
	if got, ok := activity.StoppedSecByPID[1]; !ok || got != 0 {
		t.Fatalf("expected stoppedSec[1]=0, got (%d, ok=%v)", got, ok)
	}
}

// TestActivity_NoStopEventInLastMinute — last alive window is at gameDur-30s
// → no stop event, the player was active right up to game end.
func TestActivity_NoStopEventInLastMinute(t *testing.T) {
	x := p(1, 1)
	durationSec := 600
	// Active until durationSec - 30; nothing after.
	cmds := keepAliveSeries(0, durationSec-30, x)
	activity := ComputeActivity([]*models.Player{x}, cmds, durationSec)
	if _, ok := activity.StoppedSecByPID[1]; ok {
		t.Fatalf("did not expect stop event when last alive moment is in the end-grace window")
	}
}

// TestActivity_StopSuppressedByLeave — when a player has both a leave and
// would otherwise have a stop event, the stop event is suppressed (the
// leave already covers "gone").
func TestActivity_StopSuppressedByLeave(t *testing.T) {
	x := p(1, 1)
	cmds := []*models.Command{}
	cmds = append(cmds, keepAliveSeries(0, 200, x)...)
	cmds = append(cmds, leaveCmd(210, x))
	// After leaving, sometimes commands keep coming briefly — silent here.

	activity := ComputeActivity([]*models.Player{x}, cmds, 1800)
	if _, ok := activity.StoppedSecByPID[1]; ok {
		t.Fatalf("did not expect stop event when player has Leave Game")
	}
	if got := activity.LeaveSecByPID[1]; got != 210 {
		t.Fatalf("expected leaveSec[1]=210, got %d", got)
	}
}

// TestEffectiveTeams_DroppedPlayersBecomeBalanced — direct test of the
// effective-teams transformation: a 2v3 where the third player on the big
// team has stopped collapses to 2v2.
func TestEffectiveTeams_DroppedPlayersBecomeBalanced(t *testing.T) {
	teams := [][]byte{{1, 2}, {3, 4, 5}}
	activity := Activity{
		StoppedSecByPID: map[byte]int{5: 200},
		LeaveSecByPID:   map[byte]int{},
	}
	got := effectiveTeamsAt(teams, 1000, activity)
	want := [][]byte{{1, 2}, {3, 4}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("effectiveTeamsAt mismatch:\n  got:  %v\n  want: %v", got, want)
	}
	if isStacking(got) {
		t.Fatalf("expected balanced 2v2 to not be stacking")
	}
}

// TestLateAllianceTransitions — alliance topology changes after sec=600
// surface in LateAllianceTransitions.
func TestLateAllianceTransitions(t *testing.T) {
	a, b, c, d, e := p(1, 1), p(2, 2), p(3, 3), p(4, 4), p(5, 5)
	players := []*models.Player{a, b, c, d, e}
	cmds := []*models.Command{
		// Early 2v2v1 at sec=10.
		allianceCmd(10, a, 1, 2), allianceCmd(10, b, 1, 2),
		allianceCmd(10, c, 3, 4), allianceCmd(10, d, 3, 4),
		// Late re-alliance at sec=900: e joins {a,b}.
		allianceCmd(900, a, 1, 2, 5), allianceCmd(900, b, 1, 2, 5), allianceCmd(900, e, 1, 2, 5),
	}
	res := AnalyzeAlliances(players, cmds, 1800, emptyActivity())
	if len(res.LateAllianceTransitions) != 1 {
		t.Fatalf("expected 1 late alliance transition, got %d: %+v", len(res.LateAllianceTransitions), res.LateAllianceTransitions)
	}
	if res.LateAllianceTransitions[0].Sec != 900 {
		t.Fatalf("expected late transition at sec=900, got %d", res.LateAllianceTransitions[0].Sec)
	}
}

// --- Existing winner-derivation tests (signature unchanged) -------------

func TestDeriveWinnersFromLeaves_LargestRemainingTeamWins(t *testing.T) {
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
		{"3v1v1v1", [][]byte{{1, 2, 3}, {4}, {5}, {6}}, false},
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
