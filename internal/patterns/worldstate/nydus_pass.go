package worldstate

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Offensive-nydus tuning parameters.
//
// The defining act is a Nydus Canal exit (BuildNydusExit) placed forward — in
// or on the enemy's territory — to funnel an army onto them. The teleport
// itself (EnterNydusCanal) is effectively unobservable: StarCraft records
// nydus traversal as a contextual right-click, so screp emits no
// EnterNydusCanal order in practice. We therefore confirm the exit is a real
// army insertion (not a stray vision/map-control canal) the same way the drops
// pass confirms a drop: an attack-pressure coincidence at the exit, or the
// attacker's own sustained activity there just after placing it.
const (
	// nydusMinTeleports gates the (rare) case where explicit EnterNydusCanal
	// orders ARE present — a burst this size corroborates on its own.
	nydusMinTeleports = 2

	// nydusTeleportWindowSec bounds how long after an exit an EnterNydusCanal
	// is attributed to it.
	nydusTeleportWindowSec = 120

	// Attack-coincidence window around the exit placement. The push lands at or
	// shortly after the forward exit goes up.
	nydusAttackPreSec  = 30
	nydusAttackPostSec = 180

	// Post-placement activity proxy: attacker spatial commands landing in the
	// exit's polygon within the window. The threshold is a touch higher than
	// drops' (3) because placing the exit itself contributes commands there.
	nydusActivityWindowSec   = 120
	nydusActivityMinCommands = 5

	// nydusDedupWindowSec collapses repeat forward exits by the same player
	// onto the same target base into one event.
	nydusDedupWindowSec = 180
)

// nydusExit is one BuildNydusExit placement classified as offensive (forward,
// in enemy territory). Defender is the enemy whose territory it landed in.
type nydusExit struct {
	sec      int
	frame    int32
	x, y     int
	polyID   int
	defender byte
}

// NydusCluster is the output unit of the offensive-nydus pass: one forward
// exit confirmed as an army insertion. The exit's position is the attack
// epicenter — the army "arrives" there — mirroring how drops use the unload
// point.
type NydusCluster struct {
	PID      byte
	Defender byte
	Sec      int
	Frame    int32
	Count    int // EnterNydusCanal waves observed (usually 0)
	X        int
	Y        int
	PolyID   int
	Via      string // "t" teleport burst, "a" attack-coincidence, "p" post-activity
}

// buildNydusClusters walks the enriched stream for offensive nydus exits and
// confirms each as a real army insertion via teleport burst, attack
// coincidence, or post-placement activity. Unconfirmed forward exits (a lone
// vision/map-control canal) are dropped, mirroring the drops pass's
// homecoming-suppression.
func (e *Engine) buildNydusClusters(ownership []PolyOwnership, candidates []CandidateAttack) []NydusCluster {
	timelineByPoly := make(map[int][]OwnEvent, len(ownership))
	for _, t := range ownership {
		timelineByPoly[t.PolyID] = t.Events
	}
	ownerAtSec := func(polyID int, sec int) byte {
		evs := timelineByPoly[polyID]
		owner := neutralPID
		for _, ev := range evs {
			if ev.Sec > sec {
				break
			}
			owner = ev.Owner
		}
		return owner
	}

	startOwnerByBase := make(map[int]byte, len(e.startBaseByPID))
	for pid, idx := range e.startBaseByPID {
		startOwnerByBase[idx] = pid
	}

	exitsByPlayer := map[byte][]nydusExit{}
	entersByPlayer := map[byte][]int{}

	for _, ec := range e.stream {
		pid := byte(ec.PlayerID)
		switch ec.Kind {
		case cmdenrich.KindBuildNydusExit:
			if ec.X == nil || ec.Y == nil {
				continue
			}
			polyID := pointToEventBase(float64(*ec.X), float64(*ec.Y), e.bases)
			if polyID < 0 {
				continue
			}
			defender, offensive := e.classifyOffensiveExit(pid, polyID, ec.Second, ownerAtSec, startOwnerByBase)
			if !offensive {
				continue
			}
			exitsByPlayer[pid] = append(exitsByPlayer[pid], nydusExit{
				sec:      ec.Second,
				frame:    ec.Frame,
				x:        *ec.X,
				y:        *ec.Y,
				polyID:   polyID,
				defender: defender,
			})
		case cmdenrich.KindEnterNydusCanal:
			entersByPlayer[pid] = append(entersByPlayer[pid], ec.Second)
		}
	}

	out := []NydusCluster{}
	for pid, exits := range exitsByPlayer {
		sort.Slice(exits, func(i, j int) bool { return exits[i].sec < exits[j].sec })
		// {targetPolyID} -> last emitted exit second, for the window dedup.
		lastSecByTarget := map[int]int{}
		for _, ex := range exits {
			if last, ok := lastSecByTarget[ex.polyID]; ok && ex.sec-last < nydusDedupWindowSec {
				continue
			}
			via, count := e.confirmNydusExit(pid, ex, entersByPlayer[pid], candidates)
			if via == "" {
				continue
			}
			lastSecByTarget[ex.polyID] = ex.sec
			out = append(out, NydusCluster{
				PID:      pid,
				Defender: ex.defender,
				Sec:      ex.sec,
				Frame:    ex.frame,
				Count:    count,
				X:        ex.x,
				Y:        ex.y,
				PolyID:   ex.polyID,
				Via:      via,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Sec < out[j].Sec })
	return out
}

// confirmNydusExit decides whether a forward exit was a real army insertion and
// returns the corroboration tag ("t"/"a"/"p") plus any observed teleport-wave
// count. Empty tag means unconfirmed.
func (e *Engine) confirmNydusExit(pid byte, ex nydusExit, enters []int, candidates []CandidateAttack) (string, int) {
	// Tier 0: explicit teleport burst (rare — usually no EnterNydusCanal at all).
	count := 0
	for _, es := range enters {
		if es >= ex.sec-nydusTeleportWindowSec && es <= ex.sec+nydusTeleportWindowSec {
			count++
		}
	}
	if count >= nydusMinTeleports {
		return "t", count
	}

	// Tier 1: attack-pressure coincidence at the exit polygon.
	lo, hi := ex.sec-nydusAttackPreSec, ex.sec+nydusAttackPostSec
	for _, c := range candidates {
		if c.Attacker != pid || c.PolyID != ex.polyID {
			continue
		}
		if c.Type != "attack" && c.Type != "nuke" {
			continue
		}
		cLo, cHi := c.OpenSec, c.CloseSec
		if cHi < cLo {
			cHi = cLo
		}
		if cHi >= lo && cLo <= hi {
			return "a", count
		}
	}

	// Tier 2: the attacker's own sustained activity at the exit just after
	// placing it — units emerged and are operating there.
	activity := 0
	aLo, aHi := ex.sec, ex.sec+nydusActivityWindowSec
	for _, ec := range e.stream {
		if ec.Second < aLo {
			continue
		}
		if ec.Second > aHi {
			break
		}
		if byte(ec.PlayerID) != pid || ec.X == nil || ec.Y == nil {
			continue
		}
		if pointToEventBase(float64(*ec.X), float64(*ec.Y), e.bases) == ex.polyID {
			activity++
		}
	}
	if activity >= nydusActivityMinCommands {
		return "p", count
	}
	return "", count
}

// classifyOffensiveExit decides whether a nydus exit dropped at polyID at sec
// is forward (offensive) and, if so, which enemy it threatens.
func (e *Engine) classifyOffensiveExit(pid byte, polyID, sec int, ownerAtSec func(int, int) byte, startOwnerByBase map[int]byte) (byte, bool) {
	owner := ownerAtSec(polyID, sec)
	if owner != neutralPID && owner != pid && !e.sameTeam(pid, owner) {
		return owner, true
	}
	if ownerPID, ok := startOwnerByBase[polyID]; ok && ownerPID != pid && !e.sameTeam(pid, ownerPID) {
		return ownerPID, true
	}
	if ownerPID, ok := e.naturalOwnerByBase[polyID]; ok && ownerPID != pid && !e.sameTeam(pid, ownerPID) {
		return ownerPID, true
	}
	return neutralPID, false
}
