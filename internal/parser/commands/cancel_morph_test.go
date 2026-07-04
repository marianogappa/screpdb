package commands

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func morph(pid int64, unit string, count int) *models.Command {
	u := unit
	return &models.Command{PlayerID: pid, ActionType: models.ActionTypeUnitMorph, UnitType: &u, MorphUnitCount: count}
}

func cancel(pid int64) *models.Command {
	return &models.Command{PlayerID: pid, ActionType: models.ActionTypeCancelMorph}
}

func build(pid int64, unit string) *models.Command {
	u := unit
	return &models.Command{PlayerID: pid, ActionType: models.ActionTypeBuild, UnitType: &u}
}

// droneCountBefore counts Drone-morph units surviving before the named building.
func droneCountBefore(cmds []*models.Command, pid int64, building string) int {
	n := 0
	for _, c := range cmds {
		if c.PlayerID != pid {
			continue
		}
		if c.ActionType == models.ActionTypeBuild && c.UnitType != nil && *c.UnitType == building {
			break
		}
		if c.ActionType == models.ActionTypeUnitMorph && c.UnitType != nil && *c.UnitType == models.UnitNameDrone {
			cnt := c.MorphUnitCount
			if cnt < 1 {
				cnt = 1
			}
			n += cnt
		}
	}
	return n
}

func TestDropCancelledMorphs_CancelBeforeOverlordDropsDrone(t *testing.T) {
	// pirro21's real opener: two drone morphs, one cancelled, then the pool —
	// 5 Pool, not 6 (no Overlord morphed yet, so the cancel is a Drone).
	cmds := []*models.Command{
		morph(1, models.UnitNameDrone, 1),
		morph(1, models.UnitNameDrone, 1),
		cancel(1),
		build(1, models.GeneralUnitSpawningPool),
	}
	out := DropCancelledMorphs(cmds)
	if got := droneCountBefore(out, 1, models.GeneralUnitSpawningPool); got != 1 {
		t.Fatalf("drones before pool = %d, want 1 (one drone cancelled)", got)
	}
}

func TestDropCancelledMorphs_CancelAfterOverlordIgnored(t *testing.T) {
	// Once an Overlord has morphed, a cancel is ambiguous (could be the Overlord)
	// and must not touch the drone count.
	cmds := []*models.Command{
		morph(1, models.UnitNameDrone, 1),
		morph(1, models.UnitNameOverlord, 1),
		morph(1, models.UnitNameDrone, 1),
		cancel(1),
		build(1, models.GeneralUnitSpawningPool),
	}
	out := DropCancelledMorphs(cmds)
	if got := droneCountBefore(out, 1, models.GeneralUnitSpawningPool); got != 2 {
		t.Fatalf("drones before pool = %d, want 2 (cancel after Overlord ignored)", got)
	}
}

func TestDropCancelledMorphs_MultiLarvaDecrementsNotDrops(t *testing.T) {
	// A multi-larva morph (3 drones in one command) with one cancel keeps two.
	cmds := []*models.Command{
		morph(1, models.UnitNameDrone, 3),
		cancel(1),
		build(1, models.GeneralUnitSpawningPool),
	}
	out := DropCancelledMorphs(cmds)
	if len(out) != 3 {
		t.Fatalf("command dropped when it should only decrement: len=%d, want 3", len(out))
	}
	if got := droneCountBefore(out, 1, models.GeneralUnitSpawningPool); got != 2 {
		t.Fatalf("drones before pool = %d, want 2 (one of three larvae cancelled)", got)
	}
}

func TestDropCancelledMorphs_CancelWithNoDroneIsNoop(t *testing.T) {
	cmds := []*models.Command{
		cancel(1),
		build(1, models.GeneralUnitSpawningPool),
	}
	if out := DropCancelledMorphs(cmds); len(out) != 2 {
		t.Fatalf("no-op cancel changed the stream: len=%d, want 2", len(out))
	}
}

func TestDropCancelledMorphs_PlayersIndependent(t *testing.T) {
	// Player 2's cancel must not decrement player 1's drones.
	cmds := []*models.Command{
		morph(1, models.UnitNameDrone, 1),
		morph(2, models.UnitNameDrone, 1),
		cancel(2),
		build(1, models.GeneralUnitSpawningPool),
		build(2, models.GeneralUnitSpawningPool),
	}
	out := DropCancelledMorphs(cmds)
	if got := droneCountBefore(out, 1, models.GeneralUnitSpawningPool); got != 1 {
		t.Fatalf("player 1 drones = %d, want 1 (unaffected by P2 cancel)", got)
	}
	if got := droneCountBefore(out, 2, models.GeneralUnitSpawningPool); got != 0 {
		t.Fatalf("player 2 drones = %d, want 0 (its drone cancelled)", got)
	}
}
