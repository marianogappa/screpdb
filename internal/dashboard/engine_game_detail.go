//go:build !js

package dashboard

import (
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gorilla/mux"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/scmapanalyzer/lib/scmapanalyzer"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
)

const recentDigests = 8

// digestMemo keeps the last few full digests: a game page asks for several views of one game.
type digestMemo struct {
	mu    sync.Mutex
	order [][32]byte
	byKey map[[32]byte]*engine.Digest
}

func (m *digestMemo) get(checksum [32]byte, load func() (*engine.Digest, bool)) *engine.Digest {
	m.mu.Lock()
	if d, ok := m.byKey[checksum]; ok {
		m.mu.Unlock()
		return d
	}
	m.mu.Unlock()
	d, ok := load()
	if !ok {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.byKey == nil {
		m.byKey = map[[32]byte]*engine.Digest{}
	}
	if len(m.order) >= recentDigests {
		delete(m.byKey, m.order[0])
		m.order = m.order[1:]
	}
	m.byKey[checksum] = d
	m.order = append(m.order, checksum)
	return d
}

// engineDigest is the full digest of a replay in engine mode with high confidence. A desynced game keeps
// its command-inferred views: engine data after the desync is wrong, and half of each would mislead.
func (d *Dashboard) engineDigest(replayID int64) (*engine.Digest, *library.Replay) {
	runner := d.enginePass.get()
	if runner == nil || d.library == nil {
		return nil, nil
	}
	rec, ok := d.library.lib.Snapshot().ByID(replayID)
	if !ok || rec.EngineData == nil || rec.EngineData.LowConfidence {
		return nil, nil
	}
	dg := d.digests.get(rec.Checksum, func() (*engine.Digest, bool) { return runner.Digest(rec.Checksum) })
	return dg, rec
}

func frameSecond(frame int32) int64 { return int64(frame) * 42 / 1000 }

// applyEngineToGameDetail swaps the build order, the production timeline and the per-slice unit counts
// for what the game engine says was made: no phantom builds, real completion times and cancels.
func (d *Dashboard) applyEngineToGameDetail(detail *workflowGameDetail) {
	dg, rec := d.engineDigest(detail.ReplayID)
	if dg == nil {
		return
	}
	playerBySlot := map[int8]int64{}
	for ordinal, p := range rec.Players {
		playerBySlot[int8(p.Slot)] = library.PlayerID(rec.ID, uint8(ordinal))
	}
	rows := make([]unitProductionRow, 0, len(dg.Productions))
	for _, pr := range dg.Productions {
		playerID, ok := playerBySlot[pr.Owner]
		if !ok {
			continue
		}
		unit := repcmd.UnitByID(pr.Type)
		if unit == nil || unit.Name == "" {
			continue
		}
		row := unitProductionRow{PlayerID: playerID, Second: frameSecond(pr.Start), UnitType: unit.Name, Count: 1, Cancelled: pr.Cancel != engine.NoFrame}
		if pr.Complete != engine.NoFrame {
			completed := frameSecond(pr.Complete)
			row.CompletedSecond = &completed
		}
		rows = append(rows, row)
	}
	buildUnitsBySlice(detail, rows)
	populateSupplyCurves(detail, dg, playerBySlot)
}

const supplyCapDoubled = 400

func populateSupplyCurves(detail *workflowGameDetail, dg *engine.Digest, playerBySlot map[int8]int64) {
	curves := map[int64][]workflowSupplyPoint{}
	last := map[int64][2]int32{}
	for _, s := range dg.Economy {
		playerID, ok := playerBySlot[s.Owner]
		if !ok {
			continue
		}
		state := [2]int32{s.SupplyUsed, min(s.SupplyMax, supplyCapDoubled)}
		if prev, seen := last[playerID]; seen && prev == state {
			continue
		}
		last[playerID] = state
		curves[playerID] = append(curves[playerID], workflowSupplyPoint{Second: frameSecond(s.Frame), Used: float64(state[0]) / 2, Cap: float64(state[1]) / 2})
	}
	for i := range detail.Players {
		if c, ok := curves[detail.Players[i].PlayerID]; ok {
			detail.Players[i].SupplyCurve = c
		}
	}
}

type engineSnapshotBuilding struct {
	UnitType string `json:"unit_type"`
	PlayerID int64  `json:"player_id"`
	X        int64  `json:"x"`
	Y        int64  `json:"y"`
	W        int64  `json:"w"`
	H        int64  `json:"h"`
	Complete bool   `json:"complete"`
	TownHall bool   `json:"town_hall,omitempty"`
	// Progress is how far a construction or morph is, 0..1; absent when there is none.
	Progress *float64 `json:"progress,omitempty"`
	Lifted   bool     `json:"lifted,omitempty"`
}

type engineSnapshotUnit struct {
	UnitType string `json:"unit_type"`
	PlayerID int64  `json:"player_id"`
	X        int64  `json:"x"`
	Y        int64  `json:"y"`
	Burrowed bool   `json:"burrowed,omitempty"`
}

// engineSnapshot is the map at one second, in map pixels. A coarse one (from the digest) has buildings
// exact and the army as of ArmySecond; an exact one comes from an engine pass to that very second.
type engineSnapshot struct {
	Second     int64                    `json:"second"`
	ArmySecond int64                    `json:"army_second"`
	Exact      bool                     `json:"exact"`
	Buildings  []engineSnapshotBuilding `json:"buildings"`
	Units      []engineSnapshotUnit     `json:"units"`
	Creep      *engineSnapshotCreep     `json:"creep,omitempty"`
	Bases      []engineSnapshotBase     `json:"bases"`
}

// engineSnapshotBase is a base someone holds at the snapshot's second. Kind is main, natural or expansion;
// NaturalOf is the player whose main the natural belongs to.
type engineSnapshotBase struct {
	Name        string                   `json:"name"`
	Kind        string                   `json:"kind"`
	Clock       int64                    `json:"clock,omitempty"`
	NaturalOf   int64                    `json:"natural_of,omitempty"`
	MineralOnly bool                     `json:"mineral_only,omitempty"`
	Polygon     []workflowGameEventPoint `json:"polygon"`
	PlayerID    int64                    `json:"player_id"`
	TownHall    string                   `json:"town_hall"`
	Complete    bool                     `json:"complete"`
	Since       *int64                   `json:"since,omitempty"`
}

// engineSnapshotCreep is the map's creep, one bit per tile, row by row, least significant bit first.
// Edges are the creep edge frames the game draws over tiles next to creep, as tx, ty, frame triples;
// frame k is the 32 px cell at x = 32*(k-1) of the tileset's edge art.
type engineSnapshotCreep struct {
	W       int    `json:"w"`
	Bits    string `json:"bits"`
	Tileset string `json:"tileset,omitempty"`
	Edges   []int  `json:"edges"`
}

// creepEdges finds every tile without creep that borders creep and the edge frame the game draws on it.
func creepEdges(snap engine.Snapshot) []int {
	if snap.CreepW == 0 {
		return nil
	}
	h := len(snap.Creep) * 8 / snap.CreepW
	neighbors := [8][2]int{{1, 1}, {0, 1}, {-1, 1}, {1, 0}, {-1, 0}, {1, -1}, {0, -1}, {-1, -1}}
	var out []int
	for ty := range h {
		for tx := range snap.CreepW {
			if snap.HasCreep(tx, ty) {
				continue
			}
			var mask scmapanalyzer.CreepNeighbors
			for bit, d := range neighbors {
				if snap.HasCreep(tx+d[0], ty+d[1]) {
					mask |= 1 << bit
				}
			}
			if frame := scmapanalyzer.CreepEdgeFrame(mask); mask != 0 && frame > 0 {
				out = append(out, tx, ty, frame)
			}
		}
	}
	return out
}

func unitName(t uint16) string {
	if u := repcmd.UnitByID(t); u != nil {
		return u.Name
	}
	return ""
}

func snapshotBuilding(t uint16, playerID int64, x, y int32, complete bool) (engineSnapshotBuilding, bool) {
	fp, ok := moments.FootprintOf(t)
	if !ok {
		return engineSnapshotBuilding{}, false
	}
	return engineSnapshotBuilding{
		UnitType: unitName(t), PlayerID: playerID, X: int64(x), Y: int64(y), W: int64(fp.W) * 32, H: int64(fp.H) * 32,
		Complete: complete, TownHall: moments.IsTownHall(t),
	}, true
}

func coarseSnapshotWire(snap moments.Snapshot, playerBySlot map[int8]int64) engineSnapshot {
	out := engineSnapshot{Second: int64(snap.Second), ArmySecond: int64(snap.ArmySecond), Buildings: []engineSnapshotBuilding{}, Units: []engineSnapshotUnit{}}
	for _, b := range snap.Buildings {
		if id, ok := playerBySlot[b.Owner]; ok {
			if wire, ok := snapshotBuilding(b.Type, id, b.X, b.Y, b.Complete); ok {
				if !b.Complete {
					p := min(1, max(0, b.Progress))
					wire.Progress = &p
				}
				out.Buildings = append(out.Buildings, wire)
			}
		}
	}
	for _, u := range snap.Units {
		if id, ok := playerBySlot[u.Owner]; ok {
			out.Units = append(out.Units, engineSnapshotUnit{UnitType: unitName(u.Type), PlayerID: id, X: int64(u.X), Y: int64(u.Y)})
		}
	}
	return out
}

func exactSnapshotWire(snap engine.Snapshot, playerBySlot map[int8]int64) engineSnapshot {
	out := engineSnapshot{Second: int64(snap.Second), ArmySecond: int64(snap.Second), Exact: true, Buildings: []engineSnapshotBuilding{}, Units: []engineSnapshotUnit{}}
	for _, u := range snap.Units {
		id, ok := playerBySlot[u.Owner]
		if !ok {
			continue
		}
		if wire, building := snapshotBuilding(u.Type, id, u.X, u.Y, u.Completed); building {
			if u.Progress != engine.ProgressDone {
				p := float64(u.Progress) / float64(engine.ProgressDone-1)
				wire.Progress = &p
			}
			wire.Lifted = u.Lifted
			out.Buildings = append(out.Buildings, wire)
			continue
		}
		out.Units = append(out.Units, engineSnapshotUnit{UnitType: unitName(u.Type), PlayerID: id, X: int64(u.X), Y: int64(u.Y), Burrowed: u.Burrowed})
	}
	if len(snap.Creep) > 0 {
		out.Creep = &engineSnapshotCreep{W: snap.CreepW, Bits: base64.StdEncoding.EncodeToString(snap.Creep), Tileset: tilesetFolder(snap.Tileset), Edges: creepEdges(snap)}
	}
	return out
}

func (d *Dashboard) handlerEngineSnapshot(w http.ResponseWriter, r *http.Request) {
	replayID, err := strconv.ParseInt(mux.Vars(r)["replayID"], 10, 64)
	if err != nil {
		http.Error(w, "bad replay id", http.StatusBadRequest)
		return
	}
	sec, err := strconv.Atoi(r.URL.Query().Get("second"))
	if err != nil || sec < 0 {
		http.Error(w, "bad second", http.StatusBadRequest)
		return
	}
	dg, rec := d.engineDigest(replayID)
	if dg == nil {
		http.Error(w, "no engine data for this game", http.StatusNotFound)
		return
	}
	playerBySlot := map[int8]int64{}
	for ordinal, p := range rec.Players {
		playerBySlot[int8(p.Slot)] = library.PlayerID(rec.ID, uint8(ordinal))
	}
	var out engineSnapshot
	exact := false
	if r.URL.Query().Get("exact") == "1" {
		seconds := []int{sec}
		for _, field := range strings.Split(r.URL.Query().Get("seconds"), ",") {
			if v, err := strconv.Atoi(strings.TrimSpace(field)); err == nil && v >= 0 {
				seconds = append(seconds, v)
			}
		}
		runner := d.enginePass.get()
		if runner == nil {
			http.Error(w, "the engine is off", http.StatusServiceUnavailable)
			return
		}
		snaps, err := runner.Snapshots(r.Context(), rec.Checksum, rec.Path(), seconds)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// A second past the engine's last frame has no exact snapshot; the digest still has one.
		if snap, ok := snaps[sec]; ok {
			out = exactSnapshotWire(snap, playerBySlot)
			out.Bases = snapshotBases(dg, rec, sec, exactHalls(snap), playerBySlot)
			exact = true
		}
	}
	if !exact {
		coarse := moments.SnapshotAt(dg, sec)
		out = coarseSnapshotWire(coarse, playerBySlot)
		out.Bases = snapshotBases(dg, rec, sec, coarseHalls(coarse), playerBySlot)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func exactHalls(snap engine.Snapshot) []moments.Hall {
	var halls []moments.Hall
	for _, u := range snap.Units {
		if moments.IsTownHall(u.Type) {
			halls = append(halls, moments.Hall{Type: u.Type, Owner: u.Owner, X: u.X, Y: u.Y, Complete: u.Completed, Lifted: u.Lifted})
		}
	}
	return halls
}

func coarseHalls(snap moments.Snapshot) []moments.Hall {
	var halls []moments.Hall
	for _, b := range snap.Buildings {
		if moments.IsTownHall(b.Type) {
			halls = append(halls, moments.Hall{Type: b.Type, Owner: b.Owner, X: b.X, Y: b.Y, Complete: b.Complete})
		}
	}
	return halls
}

// snapshotBases says who holds which base at the snapshot's second, from the town halls it shows.
func snapshotBases(dg *engine.Digest, rec *library.Replay, sec int, halls []moments.Hall, playerBySlot map[int8]int64) []engineSnapshotBase {
	out := []engineSnapshotBase{}
	if rec.Layout == nil {
		return out
	}
	layout := rec.Layout.Bases
	naturalOf := map[string]int64{}
	naturals := map[string]bool{}
	for _, b := range layout {
		if !isStartKind(b.Kind) || b.NaturalExpansion == "" {
			continue
		}
		naturals[b.NaturalExpansion] = true
		for ordinal, p := range rec.Players {
			if !p.IsObserver() && math.Hypot(float64(p.StartX)-float64(b.CenterX), float64(p.StartY)-float64(b.CenterY)) < 10*32 {
				naturalOf[b.NaturalExpansion] = library.PlayerID(rec.ID, uint8(ordinal))
			}
		}
	}
	for _, o := range moments.BaseOwners(dg, libraryBases(rec.Layout), sec, halls) {
		id, ok := playerBySlot[o.Owner]
		if !ok {
			continue
		}
		b := layout[o.Base]
		wire := engineSnapshotBase{Name: b.Name, Kind: "expansion", Clock: int64(b.Clock), MineralOnly: b.MineralOnly, PlayerID: id, TownHall: unitName(o.TownHall), Complete: o.Complete}
		switch {
		case isStartKind(b.Kind):
			wire.Kind = "main"
		case naturals[b.Name]:
			wire.Kind, wire.NaturalOf = "natural", naturalOf[b.Name]
		}
		for _, pt := range b.Polygon {
			wire.Polygon = append(wire.Polygon, workflowGameEventPoint{X: float64(pt.X), Y: float64(pt.Y)})
		}
		if o.Since >= 0 {
			since := int64(o.Since)
			wire.Since = &since
		}
		out = append(out, wire)
	}
	return out
}
