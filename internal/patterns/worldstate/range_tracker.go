package worldstate

import "fmt"

// Rolling-window attack pressure: require several opening-class commands inside
// attackPressureWindowSec before a range opens; keep it open on sustain (including right-click)
// until attackRangeEndIdleSec elapses without sustain.
const (
	attackPressureWindowSec = 60
	attackPressureMinCount  = 10
	attackRangeEndIdleSec   = 90
)

type attackPressureRange struct {
	open           bool
	lastSustainSec int
	// pendingOpeningTimes records seconds of opening-pressure commands while the range is closed.
	pendingOpeningTimes []int
}

type attackRangeTracker struct {
	byKey map[string]*attackPressureRange
}

func newAttackRangeTracker() *attackRangeTracker {
	return &attackRangeTracker{byKey: make(map[string]*attackPressureRange)}
}

func attackRangeMapKey(attackerPID, defenderOwnerPID byte, eventBaseIdx int) string {
	return fmt.Sprintf("%d|%d|%d", attackerPID, defenderOwnerPID, eventBaseIdx)
}

func (t *attackRangeTracker) getOrCreate(key string) *attackPressureRange {
	r := t.byKey[key]
	if r == nil {
		r = &attackPressureRange{lastSustainSec: -1e9}
		t.byKey[key] = r
	}
	return r
}

// tickIdle closes attack ranges that have seen no sustain for attackRangeEndIdleSec.
func (t *attackRangeTracker) tickIdle(sec int) {
	for _, r := range t.byKey {
		if !r.open {
			continue
		}
		if sec-r.lastSustainSec > attackRangeEndIdleSec {
			r.open = false
			r.pendingOpeningTimes = r.pendingOpeningTimes[:0]
		}
	}
}

// recordEnemyBaseCommand updates opening detection and sustain. It returns true exactly when a new
// range opens (caller should emit attack or scout). When the range is already open, sustain
// extends lastSustainSec when sustainAfterOpen or openingPressure is true.
func (t *attackRangeTracker) recordEnemyBaseCommand(key string, sec int, openingPressure, sustainAfterOpen bool) (opened bool) {
	r := t.getOrCreate(key)

	if r.open {
		if openingPressure || sustainAfterOpen {
			r.lastSustainSec = sec
		}
		return false
	}

	if openingPressure {
		r.pendingOpeningTimes = append(r.pendingOpeningTimes, sec)
		r.trimPending(sec)
		if len(r.pendingOpeningTimes) >= attackPressureMinCount &&
			sec-r.pendingOpeningTimes[0] <= attackPressureWindowSec {
			r.open = true
			r.lastSustainSec = sec
			r.pendingOpeningTimes = r.pendingOpeningTimes[:0]
			return true
		}
	}
	return false
}

func (r *attackPressureRange) trimPending(sec int) {
	cutoff := sec - attackPressureWindowSec
	i := 0
	for i < len(r.pendingOpeningTimes) && r.pendingOpeningTimes[i] < cutoff {
		i++
	}
	if i > 0 {
		r.pendingOpeningTimes = r.pendingOpeningTimes[i:]
	}
}
