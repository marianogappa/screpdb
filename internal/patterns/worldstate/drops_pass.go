package worldstate

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/cmdenrich"
)

// Drop-detection tuning parameters.
//
// dropClusterGapSec: consecutive unloads by the same player ≤ this many
// seconds apart, dropping at the same base polygon, are folded into a single
// drop event. Aligns with the user's "many unloads in the same base within 10
// seconds of each other are part of the same drop" requirement.
//
// loadStaleSec: a pending Load older than this many seconds is dropped from
// the pairing pool. Bounds the worst-case attribution window — if a player
// loaded units 5 minutes ago and only now issues an Unload, the original Load
// is no longer the right anchor.
const (
	dropClusterGapSec = 10
	loadStaleSec      = 180
)

// loadRecord is one Load event the worldstate drop pass has buffered for
// later pairing with an Unload. Coords are the transport's last-known
// position (initialized at the Load's right-click target point, updated by
// any subsequent command that references the same UnitTag).
type loadRecord struct {
	tag    uint16
	hasTag bool
	x      int
	y      int
	sec    int
}

// unloadRecord is one Unload-class command (KindUnloadAll covers Unload /
// UnloadAll / MoveUnload) the drop pass has anchored to a destination point.
// hasOwnCoord is true when the command itself carried X,Y (MoveUnload);
// false when we backfilled from lastCoordByTag or playerLastCoord.
type unloadRecord struct {
	x           int
	y           int
	sec         int
	frame       int32
	polyID      int
	hasOwnCoord bool
}

// DropCluster is the output unit of the drop pass — one cluster of unloads
// (same player, same destination base polygon, ≤ dropClusterGapSec gap
// between consecutive unloads) paired with the Loads believed to have fed
// them.
//
// SourceX / SourceY are the centroid of paired-load coordinates. SourceBaseIdx
// is the base polygon that centroid falls in (or -1 when outside any base —
// rare, e.g. mid-map staging). DstX / DstY are the centroid of unload coords.
type DropCluster struct {
	PID            byte
	Defender       byte
	FirstSec       int
	LastSec        int
	Frame          int32
	Count          int
	SourceX        int
	SourceY        int
	SourceBaseIdx  int
	DstX           int
	DstY           int
	DstPolyID      int
	HasSource      bool
	PairedLoads    []loadRecord
	// Unloads holds every individual unload point [x,y] in the cluster (pixel
	// space). Cliff-drop classification tests these points, not the centroid:
	// a cluster can merge a corner cliff drop with a nearby edge unload, which
	// drags the centroid off the cliff (e.g. corner unloads at x~84 averaged
	// with top-edge unloads at x~390 → centroid x=246, outside the cliff box).
	Unloads [][2]int
}

// BuildDrops walks the enriched stream and produces DropClusters. Mirrors
// the shape of BuildAttacks / buildRecallClusters — single-pass, per-player
// state, no orchestrator hooks needed.
//
// The stream is in chronological order, so:
//   - KindLoad commands append to that player's pendingLoads.
//   - Any spatial command whose TargetUnitTag is a transport we're tracking
//     refreshes lastCoordByTag (catches subsequent Loads onto the same
//     dropship, plus the rare TargetedOrder that references the transport).
//   - KindUnloadAll emits a per-player unload record. Destination X,Y comes
//     from the command itself when present (MoveUnload), otherwise from
//     lastCoordByTag for the most-recent pending Load's transport, otherwise
//     from playerLastCoord (the player's last spatial command).
//
// After the walk, per-player unload streams are bucketed into clusters by
// polygon + time gap. Each cluster pairs with as many pending Loads as it
// has unloads (FIFO best-effort).
func BuildDrops(stream []cmdenrich.EnrichedCommand, polys []PolygonGeom, bases []base, ownership []PolyOwnership, teams map[byte]byte) []DropCluster {
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

	type point struct{ x, y int }
	pendingLoads := map[byte][]loadRecord{}
	lastCoordByTag := map[byte]map[uint16]point{}
	playerLastCoord := map[byte]point{}
	hasPlayerLastCoord := map[byte]bool{}
	unloadsByPlayer := map[byte][]unloadRecord{}

	updateTagCoord := func(pid byte, tag uint16, x, y int) {
		m := lastCoordByTag[pid]
		if m == nil {
			m = map[uint16]point{}
			lastCoordByTag[pid] = m
		}
		m[tag] = point{x, y}
	}

	for _, ec := range stream {
		pid := byte(ec.PlayerID)

		// Refresh player's last spatial coord — used as the last-resort
		// destination fallback for Unload/UnloadAll commands that carry no
		// X,Y of their own. Skip KindMakeBuilding: Build commands carry
		// coordinates in TILE units (everything else is pixels) and a
		// building placement is not a unit-unload location. Letting a Build
		// through resolved Bunker unloads to a stray building tile read as a
		// pixel coordinate — e.g. a Missile Turret at tile (26,18) became
		// pixel (26,18), dead in the top-left corner box → false cliff drop.
		if ec.X != nil && ec.Y != nil && ec.Kind != cmdenrich.KindMakeBuilding {
			playerLastCoord[pid] = point{*ec.X, *ec.Y}
			hasPlayerLastCoord[pid] = true
		}

		// Refresh transport tag → coord mapping when the command targets a
		// tag we are currently tracking. Catches: subsequent Loads onto
		// the same transport, TargetedOrders that reference the transport
		// directly.
		if ec.TargetUnitTag != nil && ec.X != nil && ec.Y != nil {
			if _, tracking := lastCoordByTag[pid][*ec.TargetUnitTag]; tracking {
				updateTagCoord(pid, *ec.TargetUnitTag, *ec.X, *ec.Y)
			}
		}

		switch ec.Kind {
		case cmdenrich.KindLoad:
			if ec.X == nil || ec.Y == nil {
				continue
			}
			lr := loadRecord{
				x:   *ec.X,
				y:   *ec.Y,
				sec: ec.Second,
			}
			if ec.TargetUnitTag != nil {
				lr.tag = *ec.TargetUnitTag
				lr.hasTag = true
				updateTagCoord(pid, *ec.TargetUnitTag, *ec.X, *ec.Y)
			}
			pendingLoads[pid] = append(pendingLoads[pid], lr)
		case cmdenrich.KindUnloadAll:
			// Destination resolution: prefer the unload's own X,Y
			// (MoveUnload); fall back to the freshest pending load's
			// transport coord; finally to the player's last spatial coord.
			var dstX, dstY int
			hasDst := false
			if ec.X != nil && ec.Y != nil {
				dstX, dstY = *ec.X, *ec.Y
				hasDst = true
			}
			if !hasDst && len(pendingLoads[pid]) > 0 {
				// Latest pending load's tag → its tracked coord.
				latest := pendingLoads[pid][len(pendingLoads[pid])-1]
				if latest.hasTag {
					if p, ok := lastCoordByTag[pid][latest.tag]; ok {
						dstX, dstY = p.x, p.y
						hasDst = true
					}
				}
			}
			if !hasDst && hasPlayerLastCoord[pid] {
				p := playerLastCoord[pid]
				dstX, dstY = p.x, p.y
				hasDst = true
			}
			if !hasDst {
				continue
			}
			polyID := pointToEventBase(float64(dstX), float64(dstY), bases)
			if polyID < 0 {
				continue
			}
			unloadsByPlayer[pid] = append(unloadsByPlayer[pid], unloadRecord{
				x:           dstX,
				y:           dstY,
				sec:         ec.Second,
				frame:       ec.Frame,
				polyID:      polyID,
				hasOwnCoord: ec.X != nil && ec.Y != nil,
			})
		}
	}

	// Cluster unloads per player by (polygon, time gap). Pair each cluster
	// with as many pending loads as it consumed, FIFO best-effort.
	out := []DropCluster{}
	for pid, unloads := range unloadsByPlayer {
		if len(unloads) == 0 {
			continue
		}
		// Defensive sort — stream is already chronological but unloads are
		// appended per-player and reordering would corrupt clustering.
		sort.Slice(unloads, func(i, j int) bool { return unloads[i].sec < unloads[j].sec })

		// Compute a per-player loadIdx cursor so each unload pulls one load
		// from the FIFO pool (stale loads are skipped).
		loads := pendingLoads[pid]
		loadIdx := 0

		var cluster *DropCluster
		flush := func() {
			if cluster == nil {
				return
			}
			if cluster.Count == 0 {
				cluster = nil
				return
			}
			// Source = centroid of paired loads. If no loads were paired
			// (rare — player issued Unload without any prior Load this
			// game), fall back to dst as a degenerate source.
			if len(cluster.PairedLoads) > 0 {
				sx, sy := 0, 0
				for _, lr := range cluster.PairedLoads {
					sx += lr.x
					sy += lr.y
				}
				cluster.SourceX = sx / len(cluster.PairedLoads)
				cluster.SourceY = sy / len(cluster.PairedLoads)
				cluster.SourceBaseIdx = pointToEventBase(float64(cluster.SourceX), float64(cluster.SourceY), bases)
				cluster.HasSource = true
			}
			// Defender at the destination at first-unload second. Skip
			// drops where defender is the attacker themselves or a
			// teammate (mirrors emitAttackCandidates).
			cluster.Defender = ownerAtSec(cluster.DstPolyID, cluster.FirstSec)
			if cluster.Defender == cluster.PID {
				cluster = nil
				return
			}
			if sameTeamByMap(teams, cluster.PID, cluster.Defender) {
				cluster = nil
				return
			}
			out = append(out, *cluster)
			cluster = nil
		}

		for _, u := range unloads {
			// Centroid of cluster destination: running mean of unload X,Y.
			if cluster == nil ||
				cluster.DstPolyID != u.polyID ||
				u.sec-cluster.LastSec > dropClusterGapSec {
				flush()
				cluster = &DropCluster{
					PID:       pid,
					FirstSec:  u.sec,
					LastSec:   u.sec,
					Frame:     u.frame,
					Count:     0,
					DstPolyID: u.polyID,
					DstX:      u.x,
					DstY:      u.y,
				}
			}
			cluster.LastSec = u.sec
			cluster.Unloads = append(cluster.Unloads, [2]int{u.x, u.y})
			// Running centroid: ((cur*count + new) / (count+1))
			cluster.DstX = (cluster.DstX*cluster.Count + u.x) / (cluster.Count + 1)
			cluster.DstY = (cluster.DstY*cluster.Count + u.y) / (cluster.Count + 1)
			cluster.Count++
			// Pair one Load from the FIFO pool. Skip stale loads.
			for loadIdx < len(loads) {
				lr := loads[loadIdx]
				if u.sec-lr.sec > loadStaleSec {
					loadIdx++
					continue
				}
				if lr.sec > u.sec {
					// Load happens after this unload — impossible, but
					// guard against parser oddities.
					break
				}
				cluster.PairedLoads = append(cluster.PairedLoads, lr)
				loadIdx++
				break
			}
		}
		flush()
	}

	// Final stable sort by FirstSec so the resulting candidate stream is
	// chronological — keeps downstream emission deterministic and matches
	// the convention used by BuildAttacks.
	sort.SliceStable(out, func(i, j int) bool { return out[i].FirstSec < out[j].FirstSec })
	return out
}

// dropClustersToCandidateAttacks converts drop clusters into CandidateAttack
// records so cross-event inference (notably Recall's attack-coincidence pass)
// continues to see drops as targetable attack-class events. Mirrors the
// shape of the legacy drop emission in BuildAttacks before this refactor.
func dropClustersToCandidateAttacks(clusters []DropCluster) []CandidateAttack {
	out := make([]CandidateAttack, 0, len(clusters))
	for _, c := range clusters {
		out = append(out, CandidateAttack{
			Type:     "drop",
			Frame:    c.Frame,
			Second:   c.FirstSec,
			OpenSec:  c.FirstSec,
			CloseSec: c.LastSec,
			Attacker: c.PID,
			Defender: c.Defender,
			PolyID:   c.DstPolyID,
			X:        c.DstX,
			Y:        c.DstY,
		})
	}
	return out
}
