package models

import "testing"

func TestUpgradeCompletionSec(t *testing.T) {
	// Ventral Sacs duration is 100.8s — completes at startSec + 101 (rounded).
	sec, ok := UpgradeCompletionSec(100, UpgradeVentralSacsOverlordTransport)
	if !ok {
		t.Fatalf("expected ok for Ventral Sacs")
	}
	if sec != 201 {
		t.Errorf("Ventral Sacs at 100s: want 201 (100 + 101), got %d", sec)
	}

	// Unknown upgrade returns (startSec, false).
	sec, ok = UpgradeCompletionSec(50, "Made-Up Upgrade")
	if ok {
		t.Errorf("expected ok=false for unknown upgrade, got %d", sec)
	}
	if sec != 50 {
		t.Errorf("unknown upgrade: want startSec passthrough 50, got %d", sec)
	}
}
