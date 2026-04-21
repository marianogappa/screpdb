package worldstate

import "testing"

func TestAttackRangeTracker_OpensAfterMinPressureInWindow(t *testing.T) {
	tr := newAttackRangeTracker()
	key := "1|2|0"
	for sec := 0; sec < attackPressureMinCount-1; sec++ {
		if tr.recordEnemyBaseCommand(key, sec, true, false) {
			t.Fatalf("should not open before %d commands at sec %d", attackPressureMinCount, sec)
		}
	}
	if !tr.recordEnemyBaseCommand(key, attackPressureMinCount-1, true, false) {
		t.Fatalf("expected open on %dth pressure command within window", attackPressureMinCount)
	}
	if tr.recordEnemyBaseCommand(key, attackPressureMinCount, true, false) {
		t.Fatal("should not emit second open while still sustained")
	}
}

func TestAttackRangeTracker_ClosesAfterIdle(t *testing.T) {
	tr := newAttackRangeTracker()
	key := "1|2|0"
	for sec := 0; sec < attackPressureMinCount; sec++ {
		tr.recordEnemyBaseCommand(key, sec, true, false)
	}
	if !tr.byKey[key].open {
		t.Fatal("expected open range")
	}
	last := attackPressureMinCount - 1
	tr.tickIdle(last + attackRangeEndIdleSec + 1)
	if tr.byKey[key].open {
		t.Fatal("expected closed after idle")
	}
	base := last + attackRangeEndIdleSec + 2
	for i := 0; i < attackPressureMinCount-1; i++ {
		if tr.recordEnemyBaseCommand(key, base+i, true, false) {
			t.Fatalf("should not reopen before %d pressure cmds at offset %d", attackPressureMinCount, i)
		}
	}
	if !tr.recordEnemyBaseCommand(key, base+attackPressureMinCount-1, true, false) {
		t.Fatal("expected reopen after min pressure commands post-idle")
	}
}
