package worldstate

import (
	"testing"

	"github.com/icza/screp/rep/repcmd"
)

func TestAttackOpeningPressure_RightClickNeverOpens(t *testing.T) {
	if attackOpeningPressure("Right Click", nil, "") {
		t.Fatal("right click must not open attack range")
	}
	if !attackSustainAfterOpen("Right Click", nil, "") {
		t.Fatal("right click should sustain after open")
	}
}

func TestAttackOpeningPressure_AttackMoveOpens(t *testing.T) {
	id := byte(repcmd.OrderIDAttackMove)
	if !attackOpeningPressure("Targeted Order", &id, "Attack Move") {
		t.Fatal("attack move should count as opening pressure")
	}
	if classifyTargetedAttackOrder(&id, "Attack Move") != orderAttackStrong {
		t.Fatalf("expected strong class, got %v", classifyTargetedAttackOrder(&id, "Attack Move"))
	}
}

func TestAttackOpeningPressure_DirectAttackStrong(t *testing.T) {
	id := byte(repcmd.OrderIDAttack1)
	if classifyTargetedAttackOrder(&id, "Attack1") != orderAttackStrong {
		t.Fatalf("expected strong, got %v", classifyTargetedAttackOrder(&id, "Attack1"))
	}
	if !attackOpeningPressure("Targeted Order", &id, "Attack1") {
		t.Fatal("direct attack should open")
	}
}

func TestAttackOpeningPressure_SiegingStrong(t *testing.T) {
	id := byte(0x62) // Sieging
	if classifyTargetedAttackOrder(&id, "Sieging") != orderAttackStrong {
		t.Fatalf("expected strong for siege, got %v", classifyTargetedAttackOrder(&id, "Sieging"))
	}
}

func TestEnemyBasePressureForZergRush_MatchesOpeningExceptSustainOnly(t *testing.T) {
	id := byte(repcmd.OrderIDAttackMove)
	if !enemyBasePressureForZergRush("Targeted Order", &id, "") {
		t.Fatal("attack move counts for zerg rush pressure")
	}
	if enemyBasePressureForZergRush("Right Click", nil, "") {
		t.Fatal("RC should not count for zerg rush pressure")
	}
}
