package worldstate

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Ownership state machine — donor-derived batch pass over an enriched
// command stream.
//
// Semantics:
//
//   - Frame 0: each player's start polygon is owned by them.
//   - Any spatial command by the current owner inside their polygon
//     refreshes the inactivity clock (not just builds — fixes a screpdb
//     bug where pure movement/attack commands didn't refresh ownership).
//   - Any KindMakeBuilding by another player into a polygon is a
//     contested-takeover signal. Takeover happens when:
//       (a) ≥ minContestedBuildSignalsOnStart (3) signals in the last
//           contestedInvadeBuildWindowSec (180) AND polygon is starting; OR
//       (b) the build is a resource building (CC/Nexus/Hatchery), regardless
//           of starting flag (decisive ownership flip).
//     AND the current owner has been quiet for ≥ contestedSwitchSec (45s).
//     Takeover timestamp = earliest signal in the window.
//   - If a polygon's owner has been quiet for > ownershipTimeoutSec
//     (180s — kept screpdb-aligned, NOT donor's 600s viewer-tint value),
//     ownership reverts to neutral with reason "timeout".
//   - Town hall placed by a player into a non-starting polygon they own
//     (and haven't expanded to before) emits an "expansion" reason.
//
const (
	ownershipTimeoutSec             = 180
	contestedSwitchSec              = 45
	minContestedBuildSignalsOnStart = 3
	contestedInvadeBuildWindowSec   = 180
)

// OwnEvent is one transition in a polygon's ownership timeline.
//
// Owner is a raw replay byte PlayerID, or neutralPID (255) for unowned.
// Sec is the second (game-clock time) at which the transition takes effect.
// Stored as seconds (not frames) so that compose layer maps events
// directly to ReplayEvent.Second without converting.
type OwnEvent struct {
	Sec    int
	Owner  byte
	Reason string // "init" | "start" | "claim" | "takeover" | "expansion" | "timeout"
}

// PolyOwnership is the per-polygon emitted timeline.
type PolyOwnership struct {
	PolyID int
	Events []OwnEvent
}

// PolygonGeom is the minimum the state machine needs from a polygon:
// pixel-space bounding box (BBox = [minX, minY, maxX, maxY]) and ordered
// vertices for ray-cast point-in-polygon. Kind carries the layout role
// ("start", "natural", "expa") so detectors can decide which polygons
// matter (e.g. scouts only target start/natural).
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

// PlayerStart describes a player's seed: byte PlayerID used as Owner in
// OwnEvent, and pixel start position used to seed the starting polygon.
type PlayerStart struct {
	PlayerID byte
	X, Y     int
}

// IsResourceBuilding tells whether a building name represents a town hall
// (and thus a resource expansion). Mirror of screpdb's commitment-build
// resource subset.
func IsResourceBuilding(name string) bool {
	switch name {
	case "Command Center", "Nexus", "Hatchery", "Lair", "Hive":
		return true
	}
	return false
}

// BuildOwnership runs the state machine over the enriched stream and
// returns the per-polygon timeline. Linear time over (commands, polygons):
// each command does an O(n_polys) point-in-polygon (with a bbox prefilter)
// and an O(1) state update.
//
// All time comparisons are in seconds (ec.Second). Frame is incidental and
// only used for ordering inside EnrichFromCommands.
func BuildOwnership(stream []cmdenrich.EnrichedCommand, polys []PolygonGeom, players []PlayerStart, durationSec int) []PolyOwnership {
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

	for _, ec := range stream {
		flushTimeouts(ec.Second)

		if ec.X == nil || ec.Y == nil {
			continue
		}
		p := byte(ec.PlayerID)
		x, y := *ec.X, *ec.Y
		if ec.Kind == cmdenrich.KindMakeBuilding {
			x = x*32 + 16
			y = y*32 + 16
		}
		pi := pointInPolyGeom(polys, x, y)
		if pi < 0 {
			continue
		}
		cur := owner[pi]
		isBuild := ec.Kind == cmdenrich.KindMakeBuilding
		isResource := isBuild && IsResourceBuilding(ec.Subject)

		if cur == p {
			lastOwningSec[pi][p] = ec.Second
			if isResource && pi != startPolyByPlayer[p] {
				if playerExpanded[p] == nil {
					playerExpanded[p] = map[int]bool{}
				}
				if !playerExpanded[p][pi] {
					playerExpanded[p][pi] = true
					timelines[pi] = append(timelines[pi], OwnEvent{Sec: ec.Second, Owner: p, Reason: "expansion"})
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
			if isResource && pi != startPolyByPlayer[p] {
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
			timelines[pi] = append(timelines[pi], OwnEvent{Sec: takeoverSec, Owner: p, Reason: "takeover"})
		}
	}

	// Intentionally NO end-of-replay flushTimeouts call. The legacy
	// emit-as-you-go semantics let players keep ownership of their main
	// at end-of-game; only mid-game inactivity emits location_inactive.
	_ = durationSec

	out := make([]PolyOwnership, 0, len(polys))
	for i, evs := range timelines {
		c := evs[:0]
		var lastOwner byte = 254
		first := true
		for _, e := range evs {
			if !first && e.Owner == lastOwner {
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

// polygonGeomFromBases adapts the engine's internal []base layout into the
// PolygonGeom shape consumed by BuildOwnership / BuildAttacks.
//
// IDs are array indices (matching the engine's biOwnership / biEvent
// indices) so callers can cross-reference against the original bases slice.
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
