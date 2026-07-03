package worldstate

import (
	"fmt"
	"math"
	"sort"
)

// 1v1 bilateral-fight attack model (issue #186). For exactly-two-opposing-
// player games, an attack is a bilateral space-time cluster of aggressive
// commands: a real fight needs both sides active in the same neighbourhood.
// Each fight is located by the base its centroid lands in (point-in-polygon →
// real kind/clock/owner) or by inter-base-axis relational prose in open field,
// and gated on per-side command count plus duration to minimise false
// positives. Multiplayer keeps the pressure-tracker path (BuildAttacks /
// emitAttackIfImportant) where this relational geography doesn't generalise.
const (
	fightRadiusPx      = 420  // commands within this of a live fight join it
	fightGapSec        = 16   // a fight stays live this long after its last cmd
	fightMinCmdPerSide = 5    // each side needs >= this many cmds (FP-min)
	fightMinDurSec     = 12   // a fight must last >= this to emit
	fightAtBasePx      = 640  // centroid within this of a base => "at" that base
	fightMiddleBand    = 0.15 // |t-0.5| under this => mutual / "the middle"
)

type fightCmd struct {
	sec  int
	pid  byte
	x, y float64
}

type fight struct {
	startSec, lastSec int
	cntByPID          map[byte]int
	pts               []fightCmd
	sumX, sumY        float64
	n                 int
}

func (f *fight) cx() float64 { return f.sumX / float64(f.n) }
func (f *fight) cy() float64 { return f.sumY / float64(f.n) }

// singleOpponents returns the two opposing human players for a 1v1 (each with
// a known start base), and ok=false for any other configuration.
func (e *Engine) singleOpponents() (byte, byte, bool) {
	var ps []byte
	for _, pid := range e.humanPlayerIDs {
		if si, ok := e.startBaseByPID[pid]; ok && si >= 0 && si < len(e.bases) {
			ps = append(ps, pid)
		}
	}
	if len(ps) != 2 || e.sameTeam(ps[0], ps[1]) {
		return 0, 0, false
	}
	return ps[0], ps[1], true
}

// emit1v1Attacks detects bilateral fights from the enriched stream and emits
// them as "attack" events with base or relational-axis locations.
func (e *Engine) emit1v1Attacks(ownership []PolyOwnership, aPID, bPID byte) {
	timelineByPoly := make(map[int][]OwnEvent, len(ownership))
	for _, t := range ownership {
		timelineByPoly[t.PolyID] = t.Events
	}
	ownerAtSec := func(poly, sec int) byte {
		owner := neutralPID
		for _, ev := range timelineByPoly[poly] {
			if ev.Sec > sec {
				break
			}
			owner = ev.Owner
		}
		return owner
	}

	var cmds []fightCmd
	for _, ec := range e.stream {
		if ec.X == nil || ec.Y == nil || !AttackOpeningPressure(ec) {
			continue
		}
		p := byte(ec.PlayerID)
		if p != aPID && p != bPID {
			continue
		}
		if ls, left := e.leaveSec[p]; left && ec.Second > ls {
			continue
		}
		cmds = append(cmds, fightCmd{ec.Second, p, float64(*ec.X), float64(*ec.Y)})
	}
	sort.SliceStable(cmds, func(i, j int) bool { return cmds[i].sec < cmds[j].sec })

	fights := clusterFights(cmds)

	sa := e.bases[e.startBaseByPID[aPID]]
	sb := e.bases[e.startBaseByPID[bPID]]
	abx, aby := sb.CenterX-sa.CenterX, sb.CenterY-sa.CenterY
	ab2 := abx*abx + aby*aby
	proj := func(x, y float64) float64 {
		if ab2 == 0 {
			return 0.5
		}
		return ((x-sa.CenterX)*abx + (y-sa.CenterY)*aby) / ab2
	}

	for _, f := range fights {
		if f.cntByPID[aPID] < fightMinCmdPerSide || f.cntByPID[bPID] < fightMinCmdPerSide {
			continue
		}
		if f.lastSec-f.startSec < fightMinDurSec {
			continue
		}
		e.emitOneFight(f, aPID, bPID, ownerAtSec, proj)
	}
}

// clusterFights groups time-ordered aggressive commands into space-time
// clusters (sweep-line, join within radius/gap), then iteratively merges
// clusters whose time ranges overlap and whose centroids sit within radius —
// fusing a single battle that fragmented across the join radius.
func clusterFights(cmds []fightCmd) []*fight {
	var live, done []*fight
	flush := func(now int) {
		var keep []*fight
		for _, c := range live {
			if now-c.lastSec > fightGapSec {
				done = append(done, c)
			} else {
				keep = append(keep, c)
			}
		}
		live = keep
	}
	for _, cm := range cmds {
		flush(cm.sec)
		var best *fight
		bestD := math.MaxFloat64
		for _, c := range live {
			if d := math.Hypot(c.cx()-cm.x, c.cy()-cm.y); d <= fightRadiusPx && d < bestD {
				best, bestD = c, d
			}
		}
		if best == nil {
			best = &fight{startSec: cm.sec, cntByPID: map[byte]int{}}
			live = append(live, best)
		}
		best.lastSec = cm.sec
		best.cntByPID[cm.pid]++
		best.pts = append(best.pts, cm)
		best.sumX += cm.x
		best.sumY += cm.y
		best.n++
	}
	done = append(done, live...)

	for {
		merged := false
		for i := 0; i < len(done); i++ {
			for j := i + 1; j < len(done); j++ {
				a, b := done[i], done[j]
				if a.startSec > b.lastSec+fightGapSec || b.startSec > a.lastSec+fightGapSec {
					continue
				}
				if math.Hypot(a.cx()-b.cx(), a.cy()-b.cy()) > fightRadiusPx {
					continue
				}
				if b.startSec < a.startSec {
					a.startSec = b.startSec
				}
				if b.lastSec > a.lastSec {
					a.lastSec = b.lastSec
				}
				for pid, c := range b.cntByPID {
					a.cntByPID[pid] += c
				}
				a.pts = append(a.pts, b.pts...)
				a.sumX += b.sumX
				a.sumY += b.sumY
				a.n += b.n
				done = append(done[:j], done[j+1:]...)
				merged = true
				j--
			}
		}
		if !merged {
			break
		}
	}
	for _, c := range done {
		sort.SliceStable(c.pts, func(i, j int) bool { return c.pts[i].sec < c.pts[j].sec })
	}
	sort.SliceStable(done, func(i, j int) bool { return done[i].startSec < done[j].startSec })
	return done
}

// fightDrift returns the projection of the first third of points vs the last
// third, so a positive delta means the fight moved from A toward B.
func fightDrift(f *fight, proj func(x, y float64) float64) float64 {
	k := len(f.pts) / 3
	if k < 1 {
		k = 1
	}
	var e0, eN float64
	for _, p := range f.pts[:k] {
		e0 += proj(p.x, p.y)
	}
	for _, p := range f.pts[len(f.pts)-k:] {
		eN += proj(p.x, p.y)
	}
	return eN/float64(k) - e0/float64(k)
}

func (e *Engine) emitOneFight(f *fight, aPID, bPID byte, ownerAtSec func(int, int) byte, proj func(x, y float64) float64) {
	cx, cy := f.cx(), f.cy()
	baseIdx := pointInPolyGeom(e.polygonGeoms, int(cx), int(cy))
	if baseIdx >= 0 && math.Hypot(e.bases[baseIdx].CenterX-cx, e.bases[baseIdx].CenterY-cy) > fightAtBasePx {
		baseIdx = -1 // inside a polygon but far from its centre — treat as open field
	}

	t := proj(cx, cy)
	attacker, defender, mutual := e.fightAggressor(t, f.cntByPID[aPID], f.cntByPID[bPID], aPID, bPID)

	var desc, relLoc string
	locBaseIdx := -1
	if baseIdx >= 0 {
		locBaseIdx = baseIdx
		if owner := ownerAtSec(baseIdx, f.startSec); owner == aPID || owner == bPID {
			defender = owner
			attacker = e.otherPlayer(owner, aPID, bPID)
			mutual = false
		}
		desc = fmt.Sprintf("%s attacks %s %s", e.playerName(attacker), e.playerName(defender), e.bases[baseIdx].DisplayName)
	} else {
		relLoc = relationalLocation(t, fightDrift(f, proj), e.playerName(aPID), e.playerName(bPID))
		if mutual {
			desc = fmt.Sprintf("%s and %s fight %s", e.playerName(aPID), e.playerName(bPID), relLoc)
		} else {
			desc = fmt.Sprintf("%s attacks %s %s", e.playerName(attacker), e.playerName(defender), relLoc)
		}
	}

	units := e.attackUnitsCombined(CandidateAttack{
		Attacker: attacker, Second: f.startSec, OpenSec: f.startSec, CloseSec: f.lastSec,
	})
	prevLen := len(e.replayEvents)
	e.emitEvent("attack", f.startSec, desc, e.playerRef(attacker), e.playerRef(defender), locBaseIdx, units)
	// Open-field fights have no base columns; carry the relational location
	// ("in the middle", "near White's base") in the payload so the dashboard
	// can label them (it reconstructs location from columns otherwise).
	if relLoc != "" && len(e.replayEvents) > prevLen {
		payload := fmt.Sprintf(`{"loc":%q,"x":%d,"y":%d}`, relLoc, int(cx), int(cy))
		e.replayEvents[len(e.replayEvents)-1].Payload = &payload
	}
}

// fightAggressor attributes the attacker by which player's half of the axis
// the fight sits in (the other player pushed in). Near the midpoint it is a
// mutual engagement with no clear aggressor; the more-committed side (more
// commands) is reported as the nominal source.
func (e *Engine) fightAggressor(t float64, ca, cb int, aPID, bPID byte) (attacker, defender byte, mutual bool) {
	if math.Abs(t-0.5) < fightMiddleBand {
		if cb > ca {
			return bPID, aPID, true
		}
		return aPID, bPID, true
	}
	if t < 0.5 { // fight on A's side => B advanced into it
		return bPID, aPID, false
	}
	return aPID, bPID, false
}

func (e *Engine) otherPlayer(pid, aPID, bPID byte) byte {
	if pid == aPID {
		return bPID
	}
	return aPID
}

// relationalLocation describes an open-field fight relative to the line
// between the two players' bases, optionally noting the direction it drifted.
func relationalLocation(t, drift float64, nameA, nameB string) string {
	var where string
	switch {
	case t < 0.30:
		where = "near " + nameA + "'s base"
	case t < 0.45:
		where = "on " + nameA + "'s side of the map"
	case t <= 0.55:
		where = "in the middle"
	case t < 0.70:
		where = "on " + nameB + "'s side of the map"
	default:
		where = "near " + nameB + "'s base"
	}
	if math.Abs(drift) >= 0.08 {
		if drift > 0 {
			where += ", pushing toward " + nameB
		} else {
			where += ", pushing toward " + nameA
		}
	}
	return where
}
