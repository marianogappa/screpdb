package worldstate

import (
	"fmt"
	"strings"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// CandidateAttack is a pre-taste-filter detection of an attack-class event
// produced by the rolling pressure tracker plus drop/recall/nuke point
// detection. Type is one of: "attack" | "scout" | "drop" | "recall" | "nuke".
//
// Attacker / Defender are raw replay byte PlayerIDs. Defender is neutralPID
// (255) when the polygon was unowned at detection time (rare for
// drop/recall/nuke; possible for attack-pressure that opens against an
// abandoned base).
//
// CarriedUnits holds the dropped unit-type names (from UnloadAll source
// units) so the events_compose layer can route generic "drop" to
// "reaver_drop" / "dt_drop" subtypes per screpdb's existing event_type set.
type CandidateAttack struct {
	Type         string
	Frame        int32
	Second       int
	Attacker     byte
	Defender     byte
	PolyID       int
	X            int
	Y            int
	CarriedUnits []string
}

const (
	// scoutCutoffSec: pressure ranges that open this early are scouts,
	// not attacks. Mirrors screpdb's rushWindowSec/attack-classification
	// threshold for "early game = scout, mid game = attack".
	scoutCutoffSec = 5 * 60
)

// BuildAttacks walks the enriched stream once, driving the rolling
// attack-pressure tracker plus drop/recall/nuke point detection. Owners
// are looked up against the supplied ownership timelines (per polygon).
func BuildAttacks(stream []cmdenrich.EnrichedCommand, polys []PolygonGeom, ownership []PolyOwnership) []CandidateAttack {
	tracker := newAttackRangeTracker()
	out := []CandidateAttack{}
	lastTickSec := -1

	timelineByPoly := make(map[int][]OwnEvent, len(ownership))
	for _, t := range ownership {
		timelineByPoly[t.PolyID] = t.Events
	}
	ownerAtSec := func(polyID int, sec int) byte {
		evs := timelineByPoly[polyID]
		owner := neutralPID
		for _, e := range evs {
			if e.Sec > sec {
				break
			}
			owner = e.Owner
		}
		return owner
	}

	combatStart := firstCombatUnitSec(stream)
	type scoutPair struct {
		Attacker byte
		Poly     int
	}
	scoutedPair := map[scoutPair]bool{}
	emitScout := func(attacker byte, polyID int, sec int, frame int32, x, y int, defender byte) {
		key := scoutPair{Attacker: attacker, Poly: polyID}
		if scoutedPair[key] {
			return
		}
		scoutedPair[key] = true
		out = append(out, CandidateAttack{
			Type: "scout", Frame: frame, Second: sec,
			Attacker: attacker, Defender: defender,
			PolyID: polyID, X: x, Y: y,
		})
	}

	for _, ec := range stream {
		sec := ec.Second
		if sec != lastTickSec {
			tracker.tickIdle(sec)
			lastTickSec = sec
		}
		if ec.X == nil || ec.Y == nil {
			continue
		}
		attacker := byte(ec.PlayerID)
		x, y := *ec.X, *ec.Y
		// Build positions arrive in tile-space; convert to pixels so
		// the polygon comparison matches the pixel-space PolygonGeom.
		if ec.Kind == cmdenrich.KindMakeBuilding {
			x = x*32 + 16
			y = y*32 + 16
		}
		pi := pointInPolyGeom(polys, x, y)
		// Spatial commands outside any polygon fall back to the
		// globally nearest base. Mirrors legacy pointToOwnershipBase
		// semantics: every spatial command gets attributed to *some*
		// base so pressure tracking, drop/recall/nuke spotting, and
		// scout pre-pass all see the activity. Only ownership claims
		// (in BuildOwnership) stay strict polygon-only.
		if pi < 0 {
			pi = nearestPolyGeom(polys, x, y)
		}
		if pi < 0 {
			continue
		}
		defender := ownerAtSec(pi, sec)
		if defender == attacker {
			continue
		}

		// Worker scout pre-pass: spatial command into an enemy
		// start/natural before any combat unit was trained. Catches
		// lone-worker scouts the pressure threshold misses.
		if defender != neutralPID &&
			sec < firstCombatSecOr(combatStart, attacker) &&
			IsScoutPolygon(polys[pi]) {
			emitScout(attacker, pi, sec, ec.Frame, x, y, defender)
		}

		switch ec.Kind {
		case cmdenrich.KindUnloadAll:
			out = append(out, CandidateAttack{
				Type: "drop", Frame: ec.Frame, Second: sec,
				Attacker: attacker, Defender: defender,
				PolyID: pi, X: x, Y: y,
			})
			continue
		case cmdenrich.KindCast:
			subjLower := strings.ToLower(ec.Subject)
			if strings.Contains(subjLower, "recall") {
				out = append(out, CandidateAttack{
					Type: "recall", Frame: ec.Frame, Second: sec,
					Attacker: attacker, Defender: defender,
					PolyID: pi, X: x, Y: y,
				})
				continue
			}
			if strings.Contains(subjLower, "nuke") || strings.Contains(subjLower, "nuclear") {
				out = append(out, CandidateAttack{
					Type: "nuke", Frame: ec.Frame, Second: sec,
					Attacker: attacker, Defender: defender,
					PolyID: pi, X: x, Y: y,
				})
				continue
			}
		}

		opening := AttackOpeningPressure(ec)
		sustain := AttackSustainAfterOpen(ec)
		if !opening && !sustain {
			continue
		}
		key := attackRangeKey(attacker, defender, pi)
		opened := tracker.recordEnemyBaseCommand(key, sec, opening, sustain)
		if !opened {
			continue
		}
		eventType := "attack"
		if sec < scoutCutoffSec {
			eventType = "scout"
		}
		out = append(out, CandidateAttack{
			Type: eventType, Frame: ec.Frame, Second: sec,
			Attacker: attacker, Defender: defender,
			PolyID: pi, X: x, Y: y,
		})
	}
	return out
}

// Rolling-window attack pressure tracker. A range opens when an attacker
// issues attackPressureMinCount opening-class commands at the same enemy
// polygon within attackPressureWindowSec, and closes after
// attackRangeEndIdleSec without any sustain command.
const (
	attackPressureWindowSec = 60
	attackPressureMinCount  = 10
	attackRangeEndIdleSec   = 90
)

type attackPressureRange struct {
	open                bool
	lastSustainSec      int
	pendingOpeningTimes []int
}

type attackRangeTracker struct {
	byKey map[string]*attackPressureRange
}

func newAttackRangeTracker() *attackRangeTracker {
	return &attackRangeTracker{byKey: make(map[string]*attackPressureRange)}
}

func attackRangeKey(attackerPID, defenderPID byte, polyID int) string {
	return fmt.Sprintf("%d|%d|%d", attackerPID, defenderPID, polyID)
}

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

func (t *attackRangeTracker) recordEnemyBaseCommand(key string, sec int, openingPressure, sustainAfterOpen bool) bool {
	r := t.byKey[key]
	if r == nil {
		r = &attackPressureRange{lastSustainSec: -1 << 30}
		t.byKey[key] = r
	}
	if r.open {
		if openingPressure || sustainAfterOpen {
			r.lastSustainSec = sec
		}
		return false
	}
	if openingPressure {
		r.pendingOpeningTimes = append(r.pendingOpeningTimes, sec)
		// Trim window.
		cutoff := sec - attackPressureWindowSec
		i := 0
		for i < len(r.pendingOpeningTimes) && r.pendingOpeningTimes[i] < cutoff {
			i++
		}
		if i > 0 {
			r.pendingOpeningTimes = r.pendingOpeningTimes[i:]
		}
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
