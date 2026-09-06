package worldstate

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Ownership state machine, a batch pass over the enriched command stream:
//
//   - Frame 0: each player's start polygon is owned by them.
//   - ANY spatial command by the owner inside their polygon refreshes the
//     inactivity clock, not just builds — pure movement/attack commands used to
//     leave ownership un-refreshed.
//   - A KindMakeBuilding by another player into a polygon is a contested-takeover
//     signal. Takeover needs the owner quiet for ≥ contestedSwitchSec AND either
//     ≥ minContestedBuildSignalsOnStart signals inside
//     contestedInvadeBuildWindowSec on a starting polygon, or a resource building
//     (a decisive flip regardless of the starting flag). The takeover timestamp
//     is the earliest signal in the window.
//   - An owner quiet for more than ownershipTimeoutSec reverts to neutral with
//     reason "timeout". Kept at screpdb's 180s, NOT the donor's 600s viewer-tint
//     value.
//   - A town hall placed into a non-starting polygon the player owns and hasn't
//     expanded to before emits an "expansion" reason.
const (
	ownershipTimeoutSec             = 180
	contestedSwitchSec              = 45
	minContestedBuildSignalsOnStart = 3
	contestedInvadeBuildWindowSec   = 180
)

// OwnEvent is one transition in a polygon's ownership timeline. Owner is a raw
// replay byte PlayerID, or neutralPID for unowned. Sec is game-clock seconds,
// not frames, so the compose layer maps straight to ReplayEvent.Second.
type OwnEvent struct {
	Sec    int
	Owner  byte
	Reason string // "init" | "start" | "claim" | "takeover" | "expansion" | "timeout"
}

type PolyOwnership struct {
	PolyID int
	Events []OwnEvent
}

// ProductionSignal is one "the producing building is alive here" datapoint: a
// Train/Morph at second Sec from a building at tile (X, Y) when Anchored, or
// from one whose location couldn't be pinned when not — those resolve to the
// player's start base. A signal only REFRESHES the inactivity clock of a base
// the player still owns; it never claims, contests, or resurrects one. Derived
// from selection-tag tracking in internal/unittags.
type ProductionSignal struct {
	PlayerID byte
	Sec      int
	X, Y     int
	Anchored bool
}

// PolygonGeom is the minimum the state machine needs: pixel-space bounding box
// and ordered vertices for ray-cast point-in-polygon. Kind carries the layout
// role ("start", "natural", "expa") so detectors can decide which polygons
// matter — scouts, for instance, only target start and natural.
type PolygonGeom struct {
	ID       int
	Kind     string
	Vertices []geomPoint
	BBox     [4]int
	Center   geomPoint
	IsStart  bool
}

type geomPoint struct {
	X, Y int
}

// PlayerStart is a player's seed: the byte PlayerID used as OwnEvent.Owner, and
// the pixel start position used to seed the starting polygon.
type PlayerStart struct {
	PlayerID byte
	X, Y     int
}

// IsResourceBuilding reports a town hall, and thus a resource expansion. Mirror
// of screpdb's commitment-build resource subset.
func IsResourceBuilding(name string) bool {
	switch name {
	case "Command Center", "Nexus", "Hatchery", "Lair", "Hive":
		return true
	}
	return false
}

// BuildOwnership is linear over (commands, polygons): each command does an
// O(n_polys) point-in-polygon with a bbox prefilter plus an O(1) state update.
// All time comparisons are in seconds; Frame is incidental and only orders
// things inside EnrichFromCommands.
func BuildOwnership(stream []cmdenrich.EnrichedCommand, polys []PolygonGeom, players []PlayerStart, prodSignals []ProductionSignal, durationSec int) []PolyOwnership {
	timelines := make([][]OwnEvent, len(polys))
	for i := range timelines {
		timelines[i] = []OwnEvent{{Sec: 0, Owner: neutralPID, Reason: "init"}}
	}

	owner := make([]byte, len(polys))
	lastOwningSec := make([]map[byte]int, len(polys))
	for i := range owner {
		owner[i] = neutralPID
		lastOwningSec[i] = map[byte]int{}
	}
	playerExpanded := map[byte]map[int]bool{}
	startPolyByPlayer := map[byte]int{}
	// hasBuiltResource[poly][player] tracks whether a player ever EXPLICITLY placed
	// a town hall there. The game's spawned starting hall does not count: if an
	// opponent later plants one in a polygon whose nominal owner only ever had the
	// spawn seed, the spawn must be gone — otherwise the opponent couldn't have
	// built there — which makes the plant a clean expansion rather than a takeover
	// from a real active base.
	hasBuiltResource := map[int]map[byte]bool{}
	markBuiltResource := func(pi int, p byte) {
		if hasBuiltResource[pi] == nil {
			hasBuiltResource[pi] = map[byte]bool{}
		}
		hasBuiltResource[pi][p] = true
	}

	for _, ps := range players {
		pi := pointInPolyGeom(polys, ps.X, ps.Y)
		if pi < 0 {
			pi = nearestPolyGeom(polys, ps.X, ps.Y)
		}
		if pi < 0 {
			continue
		}
		owner[pi] = ps.PlayerID
		lastOwningSec[pi][ps.PlayerID] = 0
		startPolyByPlayer[ps.PlayerID] = pi
		playerExpanded[ps.PlayerID] = map[int]bool{}
		timelines[pi] = append(timelines[pi], OwnEvent{Sec: 0, Owner: ps.PlayerID, Reason: "start"})
	}

	type invaderKey struct {
		Poly   int
		Player byte
	}
	invadeSecs := map[invaderKey][]int{}

	flushTimeouts := func(currentSec int) {
		for pi, o := range owner {
			if o == neutralPID {
				continue
			}
			lastSec := lastOwningSec[pi][o]
			if currentSec-lastSec > ownershipTimeoutSec {
				owner[pi] = neutralPID
				timelines[pi] = append(timelines[pi], OwnEvent{Sec: currentSec, Owner: neutralPID, Reason: "timeout"})
			}
		}
	}

	// Production signals refresh the clock of a base the player still owns: a
	// Train/Morph proves the producing building, and thus the base, is alive. They
	// are interleaved with the command stream in time order and behave like a
	// refresh-only command (timeout first, then refresh-if-still-owned). Only
	// lastOwningSec is touched, so takeover and expansion logic is untouched.
	signals := append([]ProductionSignal(nil), prodSignals...)
	sort.Slice(signals, func(i, j int) bool { return signals[i].Sec < signals[j].Sec })
	sigIdx := 0
	drainSignals := func(upTo int) {
		for sigIdx < len(signals) && signals[sigIdx].Sec <= upTo {
			sig := signals[sigIdx]
			sigIdx++
			flushTimeouts(sig.Sec)
			pi := -1
			if sig.Anchored {
				pi = pointInPolyGeom(polys, sig.X*32+16, sig.Y*32+16)
			} else if sp, ok := startPolyByPlayer[sig.PlayerID]; ok {
				pi = sp
			}
			if pi >= 0 && owner[pi] == sig.PlayerID {
				lastOwningSec[pi][sig.PlayerID] = sig.Sec
			}
		}
	}

	for _, ec := range stream {
		drainSignals(ec.Second)
		flushTimeouts(ec.Second)

		if ec.X == nil || ec.Y == nil {
			continue
		}
		p := byte(ec.PlayerID)
		x, y := *ec.X, *ec.Y
		pi := pointInPolyGeom(polys, x, y)
		if pi < 0 {
			continue
		}
		cur := owner[pi]
		isBuild := ec.Kind == cmdenrich.KindMakeBuilding
		isResource := isBuild && IsResourceBuilding(ec.Subject)

		if cur == p {
			lastOwningSec[pi][p] = ec.Second
			if isResource {
				markBuiltResource(pi, p)
				if pi != startPolyByPlayer[p] {
					if playerExpanded[p] == nil {
						playerExpanded[p] = map[int]bool{}
					}
					if !playerExpanded[p][pi] {
						playerExpanded[p][pi] = true
						timelines[pi] = append(timelines[pi], OwnEvent{Sec: ec.Second, Owner: p, Reason: "expansion"})
					}
				}
			}
			continue
		}

		if !isBuild {
			continue
		}

		if cur == neutralPID {
			owner[pi] = p
			lastOwningSec[pi][p] = ec.Second
			reason := "claim"
			if isResource {
				markBuiltResource(pi, p)
				if pi != startPolyByPlayer[p] {
					if playerExpanded[p] == nil {
						playerExpanded[p] = map[int]bool{}
					}
					if !playerExpanded[p][pi] {
						playerExpanded[p][pi] = true
						reason = "expansion"
					}
				}
			}
			timelines[pi] = append(timelines[pi], OwnEvent{Sec: ec.Second, Owner: p, Reason: reason})
			continue
		}

		// An opponent's resource building in a polygon whose nominal owner never built
		// a town hall there means the spawn-seeded base must be gone (they couldn't
		// physically build otherwise), so this is a fresh expansion rather than a
		// takeover. Nullify the old owner's claim immediately.
		if isResource && !hasBuiltResource[pi][cur] {
			owner[pi] = p
			lastOwningSec[pi][p] = ec.Second
			markBuiltResource(pi, p)
			delete(invadeSecs, invaderKey{Poly: pi, Player: p})
			reason := "claim"
			if pi != startPolyByPlayer[p] {
				if playerExpanded[p] == nil {
					playerExpanded[p] = map[int]bool{}
				}
				if !playerExpanded[p][pi] {
					playerExpanded[p][pi] = true
					reason = "expansion"
				}
			}
			timelines[pi] = append(timelines[pi], OwnEvent{Sec: ec.Second, Owner: p, Reason: reason})
			continue
		}

		key := invaderKey{Poly: pi, Player: p}
		secs := invadeSecs[key]
		cutoff := ec.Second - contestedInvadeBuildWindowSec
		pruned := secs[:0]
		for _, s := range secs {
			if s >= cutoff {
				pruned = append(pruned, s)
			}
		}
		pruned = append(pruned, ec.Second)
		invadeSecs[key] = pruned

		lastOwnerSec := lastOwningSec[pi][cur]
		ownerQuietSec := ec.Second - lastOwnerSec
		eligible := ownerQuietSec >= contestedSwitchSec
		needSignals := 1
		if polys[pi].IsStart {
			needSignals = minContestedBuildSignalsOnStart
		}
		invadeOK := isResource || len(pruned) >= needSignals
		if eligible && invadeOK {
			takeoverSec := pruned[0]
			owner[pi] = p
			lastOwningSec[pi][p] = ec.Second
			delete(invadeSecs, key)
			if playerExpanded[p] == nil {
				playerExpanded[p] = map[int]bool{}
			}
			// A takeover already conveys "this player now controls the polygon", so mark it
			// expanded and a later town-hall plant by them won't double-emit.
			playerExpanded[p][pi] = true
			if isResource {
				markBuiltResource(pi, p)
			}
			timelines[pi] = append(timelines[pi], OwnEvent{Sec: takeoverSec, Owner: p, Reason: "takeover"})
		}
	}

	// Drain after the last command: a player whose final activity is a Train/Morph,
	// with no later movement or build, must still refresh.
	drainSignals(1 << 30)

	// Intentionally NO end-of-replay flushTimeouts: the legacy emit-as-you-go
	// semantics let players keep their main at end-of-game, and only mid-game
	// inactivity emits location_inactive.
	_ = durationSec

	// Collapse adjacent same-owner entries so only real transitions survive —
	// EXCEPT "expansion": a player who already owns the polygon and then plants a
	// town hall is meaningfully expanding, and dropping it would silently lose their
	// commitment-to-resources signal.
	out := make([]PolyOwnership, 0, len(polys))
	for i, evs := range timelines {
		c := evs[:0]
		var lastOwner byte = 254
		first := true
		for _, e := range evs {
			if !first && e.Owner == lastOwner && e.Reason != "expansion" {
				continue
			}
			c = append(c, e)
			lastOwner = e.Owner
			first = false
		}
		out = append(out, PolyOwnership{PolyID: i, Events: c})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PolyID < out[j].PolyID })
	return out
}

func pointInPolyGeom(polys []PolygonGeom, x, y int) int {
	for _, p := range polys {
		if x < p.BBox[0] || x > p.BBox[2] || y < p.BBox[1] || y > p.BBox[3] {
			continue
		}
		if rayCastGeom(p.Vertices, x, y) {
			return p.ID
		}
	}
	return -1
}

func nearestPolyGeom(polys []PolygonGeom, x, y int) int {
	if len(polys) == 0 {
		return -1
	}
	best := -1
	bestD := -1
	for _, p := range polys {
		dx := p.Center.X - x
		dy := p.Center.Y - y
		d := dx*dx + dy*dy
		if best < 0 || d < bestD {
			best = p.ID
			bestD = d
		}
	}
	return best
}

func rayCastGeom(verts []geomPoint, x, y int) bool {
	inside := false
	n := len(verts)
	if n < 3 {
		return false
	}
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := verts[i].X, verts[i].Y
		xj, yj := verts[j].X, verts[j].Y
		if (yi > y) != (yj > y) {
			t := float64(x-xi)*float64(yj-yi) - float64(y-yi)*float64(xj-xi)
			if (yj > yi) == (t > 0) {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// polygonGeomFromBases adapts the engine's internal []base layout. IDs are
// array indices, matching the engine's biOwnership / biEvent indices, so callers
// can cross-reference against the original bases slice.
func polygonGeomFromBases(bases []base) []PolygonGeom {
	out := make([]PolygonGeom, 0, len(bases))
	for i, b := range bases {
		verts := make([]geomPoint, 0, len(b.Polygon))
		minX, minY := int(1<<30), int(1<<30)
		maxX, maxY := int(-(1 << 30)), int(-(1 << 30))
		for _, p := range b.Polygon {
			x, y := int(p.X), int(p.Y)
			verts = append(verts, geomPoint{X: x, Y: y})
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
		if len(verts) == 0 {
			minX, minY = int(b.CenterX), int(b.CenterY)
			maxX, maxY = minX, minY
		}
		out = append(out, PolygonGeom{
			ID:       i,
			Kind:     b.Kind,
			Vertices: verts,
			BBox:     [4]int{minX, minY, maxX, maxY},
			Center:   geomPoint{X: int(b.CenterX), Y: int(b.CenterY)},
			IsStart:  b.IsStarting,
		})
	}
	return out
}
