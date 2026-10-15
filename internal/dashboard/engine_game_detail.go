//go:build !js

package dashboard

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/gorilla/mux"

	"github.com/icza/screp/rep/repcmd"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/macro"
	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
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
	if d.enginePass == nil || d.library == nil {
		return nil, nil
	}
	rec, ok := d.library.lib.Snapshot().ByID(replayID)
	if !ok || rec.EngineData == nil || rec.EngineData.LowConfidence {
		return nil, nil
	}
	runner := d.enginePass
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

// Command-inferred event types the engine replaces: it knows who fought, who expanded and who lost a base.
var engineReplacedEventTypes = map[string]bool{"attack": true, "expansion": true, "takeover": true}

const baseMatchRadius = 15 * 32

func (d *Dashboard) applyEngineEvents(detail *workflowGameDetail, layout *models.MapContextLayout) {
	dg, rec := d.engineDigest(detail.ReplayID)
	if dg == nil {
		return
	}
	players := map[int8]workflowGameEventPlayer{}
	for ordinal, p := range rec.Players {
		id := library.PlayerID(rec.ID, uint8(ordinal))
		for _, gp := range detail.Players {
			if gp.PlayerID == id {
				players[int8(p.Slot)] = workflowGameEventPlayer{PlayerID: id, Name: gp.Name, Color: gp.Color}
			}
		}
	}
	player := func(slot int8) *workflowGameEventPlayer {
		if p, ok := players[slot]; ok {
			return &p
		}
		return nil
	}
	bases := overlayBaseMetasFromLayout(layout)
	nearestBase := func(x, y int32) *workflowGameEventBase {
		var best *workflowGameEventBase
		bestDist := float64(baseMatchRadius * baseMatchRadius)
		for i := range bases {
			dx, dy := bases[i].Base.Center.X-float64(x), bases[i].Base.Center.Y-float64(y)
			if dist := dx*dx + dy*dy; dist <= bestDist {
				best, bestDist = &bases[i].Base, dist
			}
		}
		return best
	}

	kept := detail.GameEvents[:0]
	for _, e := range detail.GameEvents {
		if !engineReplacedEventTypes[e.Type] {
			kept = append(kept, e)
		}
	}
	detail.GameEvents = kept

	heldBases := map[int8]map[*workflowGameEventBase]bool{}
	hold := func(owner int8, b *workflowGameEventBase) {
		if heldBases[owner] == nil {
			heldBases[owner] = map[*workflowGameEventBase]bool{}
		}
		heldBases[owner][b] = true
	}
	for _, u := range dg.Initial {
		if b := nearestBase(u.X, u.Y); b != nil && moments.IsTownHall(u.Type) {
			hold(u.Owner, b)
		}
	}
	for _, m := range moments.Find(dg, macro.Compute(dg)) {
		if _, ok := players[m.Actor]; !ok {
			continue
		}
		e := workflowGameEvent{
			Type:   string(m.Kind),
			Second: int64(m.Second),
			Actor:  player(m.Actor),
			Target: player(m.Target),
			Engine: &workflowEngineEvent{SnapshotSecond: int64(m.SnapshotSecond), EndSecond: int64(m.EndSecond), Cancelled: m.Cancelled},
		}
		if m.Region != nil {
			e.Engine.Region = &workflowEngineRegion{X: int64(m.Region.X), Y: int64(m.Region.Y), W: int64(m.Region.W), H: int64(m.Region.H)}
		}
		if m.UnitType != 0 {
			if u := repcmd.UnitByID(m.UnitType); u != nil {
				e.Engine.UnitType = u.Name
			}
			e.Engine.Point = &workflowGameEventPoint{X: float64(m.X), Y: float64(m.Y)}
			e.Base = nearestBase(m.X, m.Y)
		}
		// A town hall away from every base, or at a base the player holds, is production, not an expansion.
		if m.Kind == moments.KindExpansion && len(bases) > 0 {
			if e.Base == nil || heldBases[m.Actor][e.Base] {
				continue
			}
			if !m.Cancelled {
				hold(m.Actor, e.Base)
			}
		}
		if m.Kind == moments.KindFight && m.Region != nil {
			e.Base = nearestBase(m.Region.X+m.Region.W/2, m.Region.Y+m.Region.H/2)
		}
		for _, s := range m.Sides {
			p := player(s.Owner)
			if p == nil {
				continue
			}
			side := workflowEngineSide{Player: *p, LostMinerals: int64(s.Lost.Minerals), LostGas: int64(s.Lost.Gas), Killed: int64(s.Killed.Value()), UnitsLost: []workflowUnitCount{}}
			for t, n := range s.UnitsLost {
				if u := repcmd.UnitByID(t); u != nil {
					side.UnitsLost = append(side.UnitsLost, workflowUnitCount{UnitType: u.Name, Count: int64(n)})
				}
			}
			sort.Slice(side.UnitsLost, func(i, j int) bool { return side.UnitsLost[i].Count > side.UnitsLost[j].Count })
			e.Engine.Sides = append(e.Engine.Sides, side)
		}
		detail.GameEvents = append(detail.GameEvents, e)
	}
	sort.SliceStable(detail.GameEvents, func(i, j int) bool { return detail.GameEvents[i].Second < detail.GameEvents[j].Second })
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
}

type engineSnapshotUnit struct {
	UnitType string `json:"unit_type"`
	PlayerID int64  `json:"player_id"`
	X        int64  `json:"x"`
	Y        int64  `json:"y"`
}

// engineSnapshot is the map at one second, in map pixels: buildings exact, army as of ArmySecond.
type engineSnapshot struct {
	Second     int64                    `json:"second"`
	ArmySecond int64                    `json:"army_second"`
	Buildings  []engineSnapshotBuilding `json:"buildings"`
	Units      []engineSnapshotUnit     `json:"units"`
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
	snap := moments.SnapshotAt(dg, sec)
	out := engineSnapshot{Second: int64(snap.Second), ArmySecond: int64(snap.ArmySecond), Buildings: []engineSnapshotBuilding{}, Units: []engineSnapshotUnit{}}
	name := func(t uint16) string {
		if u := repcmd.UnitByID(t); u != nil {
			return u.Name
		}
		return ""
	}
	for _, b := range snap.Buildings {
		id, ok := playerBySlot[b.Owner]
		fp, hasFootprint := moments.FootprintOf(b.Type)
		if !ok || !hasFootprint {
			continue
		}
		out.Buildings = append(out.Buildings, engineSnapshotBuilding{
			UnitType: name(b.Type), PlayerID: id, X: int64(b.X), Y: int64(b.Y), W: int64(fp.W) * 32, H: int64(fp.H) * 32,
			Complete: b.Complete, TownHall: moments.IsTownHall(b.Type),
		})
	}
	for _, u := range snap.Units {
		if id, ok := playerBySlot[u.Owner]; ok {
			out.Units = append(out.Units, engineSnapshotUnit{UnitType: name(u.Type), PlayerID: id, X: int64(u.X), Y: int64(u.Y)})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
