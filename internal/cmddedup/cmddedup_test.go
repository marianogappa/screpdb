package cmddedup

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func techCmd(player int64, second int, name string) *models.Command {
	n := name
	return &models.Command{
		PlayerID:             player,
		SecondsFromGameStart: second,
		ActionType:           "Tech",
		TechName:             &n,
	}
}

func upgradeCmd(player int64, second int, name string) *models.Command {
	n := name
	return &models.Command{
		PlayerID:             player,
		SecondsFromGameStart: second,
		ActionType:           "Upgrade",
		UpgradeName:          &n,
	}
}

func indices(commands, kept []*models.Command) []int {
	pos := map[*models.Command]int{}
	for i, c := range commands {
		pos[c] = i
	}
	out := make([]int, 0, len(kept))
	for _, c := range kept {
		out = append(out, pos[c])
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDedup_OneShotTech_KeepsLatest(t *testing.T) {
	cmds := []*models.Command{
		techCmd(1, 300, models.TechLurkerAspect),
		techCmd(1, 320, models.TechLurkerAspect),
	}
	got := Dedup(cmds)
	if len(got) != 1 {
		t.Fatalf("expected 1 kept, got %d", len(got))
	}
	if got[0] != cmds[1] {
		t.Fatalf("expected latest (idx 1) kept; got idx %v", indices(cmds, got))
	}
}

func TestDedup_OneShotUpgrade_KeepsLatest(t *testing.T) {
	// Singularity Charge — 105s research duration, MaxLevel=1.
	cmds := []*models.Command{
		upgradeCmd(1, 600, models.UpgradeSingularityChargeDragoonRange),
		upgradeCmd(1, 650, models.UpgradeSingularityChargeDragoonRange),
	}
	got := Dedup(cmds)
	if len(got) != 1 || got[0] != cmds[1] {
		t.Fatalf("expected only the latest kept; kept %v", indices(cmds, got))
	}
}

func TestDedup_Tiered_L1Spam_CollapsesToLatest(t *testing.T) {
	// Ground Weapons L1 duration is 167.58s. Two clicks 80s apart should
	// collapse to the second one.
	cmds := []*models.Command{
		upgradeCmd(1, 300, models.UpgradeProtossGroundWeapons),
		upgradeCmd(1, 380, models.UpgradeProtossGroundWeapons),
	}
	got := Dedup(cmds)
	if len(got) != 1 || got[0] != cmds[1] {
		t.Fatalf("expected only latest kept (forge-rebuilt scenario); kept %v", indices(cmds, got))
	}
}

func TestDedup_Tiered_LegitimateProgression_AllKept(t *testing.T) {
	// Three legitimate levels, each starting just after the previous duration.
	// L1 dur 167.58, L2 dur 180.18, L3 dur 192.78. Use generous gaps.
	cmds := []*models.Command{
		upgradeCmd(1, 300, models.UpgradeProtossGroundWeapons),
		upgradeCmd(1, 500, models.UpgradeProtossGroundWeapons), // > 300+167.58
		upgradeCmd(1, 700, models.UpgradeProtossGroundWeapons), // > 500+180.18
	}
	got := Dedup(cmds)
	if len(got) != 3 {
		t.Fatalf("expected all 3 levels kept; kept %v", indices(cmds, got))
	}
}

func TestDedup_Tiered_FourthOccurrenceDropped(t *testing.T) {
	// After L3 finishes, a 4th occurrence is over-cap and should be dropped.
	cmds := []*models.Command{
		upgradeCmd(1, 300, models.UpgradeProtossGroundWeapons),
		upgradeCmd(1, 500, models.UpgradeProtossGroundWeapons),
		upgradeCmd(1, 700, models.UpgradeProtossGroundWeapons),
		upgradeCmd(1, 1000, models.UpgradeProtossGroundWeapons), // post-L3
	}
	got := Dedup(cmds)
	if len(got) != 3 {
		t.Fatalf("expected 3 kept (L1/L2/L3), got %d", len(got))
	}
	idxs := indices(cmds, got)
	if !equalInts(idxs, []int{0, 1, 2}) {
		t.Fatalf("expected kept indices [0 1 2], got %v", idxs)
	}
}

func TestDedup_CrossPlayerIsolation(t *testing.T) {
	// Player 1 and Player 2 both research Stim Packs at the same second. Both
	// must be kept — the dedup is per-(player,name).
	cmds := []*models.Command{
		techCmd(1, 300, models.TechStimPacks),
		techCmd(2, 300, models.TechStimPacks),
	}
	got := Dedup(cmds)
	if len(got) != 2 {
		t.Fatalf("expected both players' Stim Packs kept; kept %d", len(got))
	}
}

func TestDedup_UnknownNamePassesThrough(t *testing.T) {
	// Two upgrades with a name that's not in models.LookupUpgrade — should not
	// be touched, even if duplicated.
	cmds := []*models.Command{
		upgradeCmd(1, 300, "Some Modded Upgrade Name"),
		upgradeCmd(1, 320, "Some Modded Upgrade Name"),
	}
	got := Dedup(cmds)
	if len(got) != 2 {
		t.Fatalf("unknown name should pass through; kept %d", len(got))
	}
}

func TestDedup_NonResearchCommandsUntouched(t *testing.T) {
	// Build/Train commands should never be considered.
	build := &models.Command{PlayerID: 1, SecondsFromGameStart: 100, ActionType: "Build"}
	train := &models.Command{PlayerID: 1, SecondsFromGameStart: 110, ActionType: "Train"}
	cmds := []*models.Command{build, train}
	got := Dedup(cmds)
	if len(got) != 2 {
		t.Fatalf("non-research commands must pass through; kept %d", len(got))
	}
}

func TestDedup_PreservesOriginalOrder(t *testing.T) {
	// Mixed command list: dedup must remove the right indices and keep the
	// rest in stable original order.
	other := &models.Command{PlayerID: 1, SecondsFromGameStart: 290, ActionType: "Build"}
	dup1 := techCmd(1, 300, models.TechLurkerAspect)
	between := &models.Command{PlayerID: 1, SecondsFromGameStart: 310, ActionType: "Build"}
	dup2 := techCmd(1, 320, models.TechLurkerAspect)
	after := &models.Command{PlayerID: 1, SecondsFromGameStart: 330, ActionType: "Train"}
	cmds := []*models.Command{other, dup1, between, dup2, after}

	got := Dedup(cmds)
	if len(got) != 4 {
		t.Fatalf("expected 4 kept (the earlier Lurker Aspect dropped); got %d", len(got))
	}
	want := []*models.Command{other, between, dup2, after}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("order mismatch at %d: got %v want %v", i, indices(cmds, got), indices(cmds, want))
		}
	}
}

func TestDedup_NilAndEmptyInputs(t *testing.T) {
	if got := Dedup(nil); got != nil {
		t.Fatalf("expected nil for nil input, got %v", got)
	}
	empty := []*models.Command{}
	if got := Dedup(empty); len(got) != 0 {
		t.Fatalf("expected empty for empty input, got len %d", len(got))
	}
}
