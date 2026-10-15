//go:build !js

package dashboard

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"sync"

	"github.com/gorilla/mux"

	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpdb/internal/engine"
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

// commandMetaEvents are the command-side events that are not guesses about the game state (who left,
// who stopped playing, alliances, the openers). An engine game keeps these and takes every other event
// from the engine; the two detectors are never mixed for one game.
var commandMetaEvents = map[string]bool{
	"player_start": true, "bo_openers": true, "leave_game": true, "player_dropped": true, "mass_disconnect": true,
	"player_stopped_playing": true, "late_alliance": true, "team_stacking_detected": true,
}

func (d *Dashboard) applyEngineEvents(detail *workflowGameDetail, layout *models.MapContextLayout) {
	dg, rec := d.engineDigest(detail.ReplayID)
	if dg == nil {
		return
	}
	playersBySlot := map[int8]workflowGameEventPlayer{}
	var players []moments.Player
	for ordinal, p := range rec.Players {
		id := library.PlayerID(rec.ID, uint8(ordinal))
		for _, gp := range detail.Players {
			if gp.PlayerID == id {
				playersBySlot[int8(p.Slot)] = workflowGameEventPlayer{PlayerID: id, Name: gp.Name, Color: gp.Color}
			}
		}
		players = append(players, moments.Player{Slot: int8(p.Slot), Race: engineRace(p.Race), Human: p.IsHuman() && !p.IsObserver()})
	}
	player := func(slot int8) *workflowGameEventPlayer {
		if p, ok := playersBySlot[slot]; ok {
			return &p
		}
		return nil
	}
	metas := overlayBaseMetasFromLayout(layout)
	bases := make([]moments.Base, len(metas))
	for i, m := range metas {
		kind := moments.BaseExpansion
		switch {
		case m.IsStarting:
			kind = moments.BaseStart
		case m.Base.NaturalOfClock != nil:
			kind = moments.BaseNatural
		}
		poly := make([][2]int32, len(m.Base.Polygon))
		for j, pt := range m.Base.Polygon {
			poly[j] = [2]int32{int32(pt.X), int32(pt.Y)}
		}
		bases[i] = moments.Base{Kind: kind, X: int32(m.Base.Center.X), Y: int32(m.Base.Center.Y), Polygon: poly}
	}
	base := func(i int) *workflowGameEventBase {
		if i == moments.NoBase || i >= len(metas) {
			return nil
		}
		b := metas[i].Base
		return &b
	}

	kept := detail.GameEvents[:0]
	for _, e := range detail.GameEvents {
		if commandMetaEvents[e.Type] {
			e.Engine = &workflowEngineEvent{SnapshotSecond: e.Second, EndSecond: e.Second}
			if e.Base != nil {
				e.Engine.Region = regionAroundBase(e.Base)
			}
			kept = append(kept, e)
		}
	}
	detail.GameEvents = kept

	for _, m := range moments.Detect(dg, bases, players) {
		e := workflowGameEvent{
			Type:   m.Type,
			Second: int64(m.Second),
			Actor:  player(m.Actor),
			Target: player(m.Target),
			Base:   base(m.Base),
			Engine: &workflowEngineEvent{
				SnapshotSecond: int64(m.SnapshotSecond),
				EndSecond:      int64(m.Second),
				Region:         &workflowEngineRegion{X: int64(m.Region.X), Y: int64(m.Region.Y), W: int64(m.Region.W), H: int64(m.Region.H)},
				Point:          &workflowGameEventPoint{X: float64(m.X), Y: float64(m.Y)},
			},
		}
		for _, t := range m.Units {
			if u := repcmd.UnitByID(t); u != nil {
				e.AttackUnitTypes = append(e.AttackUnitTypes, u.Name)
			}
		}
		switch m.Type {
		case "recall":
			e.Base, e.TargetBase, e.TargetOwner = base(m.SourceBase), base(m.Base), player(m.Target)
			e.TargetPoint = e.Engine.Point
		case "attack":
			if e.Base == nil {
				e.Base = openFieldBase(m, bases, rec, playersBySlot)
			}
		}
		if e.Actor == nil {
			continue
		}
		detail.GameEvents = append(detail.GameEvents, e)
	}
	sort.SliceStable(detail.GameEvents, func(i, j int) bool { return detail.GameEvents[i].Second < detail.GameEvents[j].Second })
}

func engineRace(r library.Race) openbw.Race {
	switch r {
	case library.RaceZerg:
		return openbw.RaceZerg
	case library.RaceTerran:
		return openbw.RaceTerran
	case library.RaceProtoss:
		return openbw.RaceProtoss
	}
	return openbw.RaceRandom
}

func regionAroundBase(b *workflowGameEventBase) *workflowEngineRegion {
	minX, minY, maxX, maxY := b.Center.X, b.Center.Y, b.Center.X, b.Center.Y
	for _, p := range b.Polygon {
		minX, minY, maxX, maxY = math.Min(minX, p.X), math.Min(minY, p.Y), math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	const margin, minSide = 8 * 32, 24 * 32
	r := &workflowEngineRegion{X: int64(minX) - margin, Y: int64(minY) - margin, W: int64(maxX-minX) + 2*margin, H: int64(maxY-minY) + 2*margin}
	if r.W < minSide {
		r.X -= (minSide - r.W) / 2
		r.W = minSide
	}
	if r.H < minSide {
		r.Y -= (minSide - r.H) / 2
		r.H = minSide
	}
	return r
}

// openFieldBase labels a battle away from any base the way the command-inferred 1v1 attacks do: by where
// it sits on the line between the two mains.
func openFieldBase(m moments.Event, bases []moments.Base, rec *library.Replay, players map[int8]workflowGameEventPlayer) *workflowGameEventBase {
	mains := map[int8][2]float64{}
	for _, u := range rec.Players {
		if u.IsObserver() {
			continue
		}
		if x, y := float64(u.StartX), float64(u.StartY); x != 0 || y != 0 {
			mains[int8(u.Slot)] = [2]float64{x, y}
		}
	}
	a, aok := mains[m.Target]
	b, bok := mains[m.Actor]
	ta, tb := players[m.Target], players[m.Actor]
	if !aok || !bok || len(mains) != 2 {
		return nil
	}
	ax, ay := b[0]-a[0], b[1]-a[1]
	length := ax*ax + ay*ay
	if length == 0 {
		return nil
	}
	t := ((float64(m.X)-a[0])*ax + (float64(m.Y)-a[1])*ay) / length
	var where string
	switch {
	case t < 0.30:
		where = "near " + ta.Name + "'s base"
	case t < 0.45:
		where = "on " + ta.Name + "'s side of the map"
	case t <= 0.55:
		where = "in the middle"
	case t < 0.70:
		where = "on " + tb.Name + "'s side of the map"
	default:
		where = "near " + tb.Name + "'s base"
	}
	return &workflowGameEventBase{Name: where, Kind: "open_field", Center: workflowGameEventPoint{X: float64(m.X), Y: float64(m.Y)}}
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
