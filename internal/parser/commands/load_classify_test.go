package commands

import (
	"testing"

	"github.com/marianogappa/screpdb/internal/models"
)

func TestClassifyLoadsDropshipShuttleBunker(t *testing.T) {
	rightClick := "Right Click"
	dropship := models.GeneralUnitDropship
	shuttle := models.GeneralUnitShuttle
	bunker := models.GeneralUnitBunker

	cmds := []*models.Command{
		{PlayerID: 1, ActionType: rightClick, SecondsFromGameStart: 100, TargetUnitType: &dropship},
		{PlayerID: 1, ActionType: rightClick, SecondsFromGameStart: 110, TargetUnitType: &shuttle},
		{PlayerID: 1, ActionType: rightClick, SecondsFromGameStart: 120, TargetUnitType: &bunker},
	}

	ClassifyLoads(cmds)

	if cmds[0].ActionType != ActionTypeLoad {
		t.Errorf("Dropship right-click: want Load, got %q", cmds[0].ActionType)
	}
	if cmds[1].ActionType != ActionTypeLoad {
		t.Errorf("Shuttle right-click: want Load, got %q", cmds[1].ActionType)
	}
	if cmds[2].ActionType != ActionTypeLoadBunker {
		t.Errorf("Bunker right-click: want LoadBunker, got %q", cmds[2].ActionType)
	}
}

func TestClassifyLoadsOverlordGatedOnVentralSacs(t *testing.T) {
	rightClick := "Right Click"
	overlord := models.GeneralUnitOverlord
	upgrade := "Upgrade"
	ventral := models.UpgradeVentralSacsOverlordTransport

	cmds := []*models.Command{
		// Pre-research right-click on Overlord stays as Right Click.
		{PlayerID: 2, ActionType: rightClick, SecondsFromGameStart: 60, TargetUnitType: &overlord},
		// Ventral Sacs starts at 100s, duration ~101s → completes at 201s.
		{PlayerID: 2, ActionType: upgrade, SecondsFromGameStart: 100, UpgradeName: &ventral},
		// Right after completion: rewrites to Load.
		{PlayerID: 2, ActionType: rightClick, SecondsFromGameStart: 250, TargetUnitType: &overlord},
		// Different player without research: stays as Right Click.
		{PlayerID: 3, ActionType: rightClick, SecondsFromGameStart: 250, TargetUnitType: &overlord},
	}

	ClassifyLoads(cmds)

	if cmds[0].ActionType != rightClick {
		t.Errorf("Pre-research Overlord right-click: want Right Click, got %q", cmds[0].ActionType)
	}
	if cmds[2].ActionType != ActionTypeLoad {
		t.Errorf("Post-research Overlord right-click: want Load, got %q", cmds[2].ActionType)
	}
	if cmds[3].ActionType != rightClick {
		t.Errorf("Other player Overlord right-click without research: want Right Click, got %q", cmds[3].ActionType)
	}
}

func TestClassifyLoadsIgnoresNonTransport(t *testing.T) {
	rightClick := "Right Click"
	zealot := "Zealot" // arbitrary non-transport
	cmds := []*models.Command{
		{PlayerID: 1, ActionType: rightClick, SecondsFromGameStart: 50, TargetUnitType: &zealot},
		{PlayerID: 1, ActionType: rightClick, SecondsFromGameStart: 60}, // no target
	}
	ClassifyLoads(cmds)
	if cmds[0].ActionType != rightClick {
		t.Errorf("Zealot right-click: want Right Click, got %q", cmds[0].ActionType)
	}
	if cmds[1].ActionType != rightClick {
		t.Errorf("Right-click on ground: want Right Click, got %q", cmds[1].ActionType)
	}
}
