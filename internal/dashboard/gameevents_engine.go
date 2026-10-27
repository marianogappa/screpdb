//go:build !js

package dashboard

import (
	"math"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/icza/screp/rep/repcmd"
	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/screpdb/internal/crashreport"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/moments"
	"github.com/marianogappa/screpdb/internal/library"
	"github.com/marianogappa/screpdb/internal/models"
)

// The engine source of game events: moments.Detect on the game's engine digest, from exact unit
// positions, deaths, unloads and teleports. A game with a trusted digest takes all its events from here,
// except the command meta events (openers, leaves, alliances), which are facts, not guesses.

// commandMetaEvents are the command-side events that are not guesses about the game state (who left,
// who stopped playing, alliances, the openers). An engine game keeps these and takes every other event
// from the engine; the two detectors are never mixed for one game.
var commandMetaEvents = map[string]bool{
	"player_start": true, "bo_openers": true, "leave_game": true, "player_dropped": true, "mass_disconnect": true,
	"player_stopped_playing": true, "late_alliance": true, "team_stacking_detected": true,
}

func (d *Dashboard) applyEngineEvents(detail *workflowGameDetail, layout *models.MapContextLayout, startClockByPlayerID map[int64]int) {
	dg, rec := d.engineDigest(detail.ReplayID)
	if dg == nil {
		return
	}
	playersBySlot := map[int8]workflowGameEventPlayer{}
	for ordinal, p := range rec.Players {
		id := library.PlayerID(rec.ID, uint8(ordinal))
		for _, gp := range detail.Players {
			if gp.PlayerID == id {
				playersBySlot[int8(p.Slot)] = workflowGameEventPlayer{PlayerID: id, Name: gp.Name, Color: gp.Color}
			}
		}
	}
	players := momentPlayers(rec)
	player := func(slot int8) *workflowGameEventPlayer {
		if p, ok := playersBySlot[slot]; ok {
			return &p
		}
		return nil
	}
	startClock := func(p *workflowGameEventPlayer) *int64 {
		if p == nil {
			return nil
		}
		if c, ok := startClockByPlayerID[p.PlayerID]; ok {
			v := int64(c)
			return &v
		}
		return nil
	}
	metas := overlayBaseMetasFromLayout(layout)
	bases := momentBases(metas)
	base := func(i int) *workflowGameEventBase {
		if i == moments.NoBase || i >= len(metas) {
			return nil
		}
		b := metas[i].Base
		return &b
	}

	slotByPlayerID := map[int64]int8{}
	for ordinal, p := range rec.Players {
		slotByPlayerID[library.PlayerID(rec.ID, uint8(ordinal))] = int8(p.Slot)
	}
	kept := detail.GameEvents[:0]
	for _, e := range detail.GameEvents {
		if commandMetaEvents[e.Type] {
			e.Engine = &workflowEngineEvent{SnapshotSecond: e.Second, EndSecond: e.Second}
			if e.Base != nil {
				e.Engine.Region = regionAroundBase(e.Base)
			}
			if slot, ok := slotByPlayerID[actorID(e.Actor)]; ok && leaveEvents[e.Type] {
				if b := base(leaverBase(dg, bases, int(e.Second), slot)); b != nil {
					center := b.Center
					e.Engine.Point = &center
					e.Engine.Region = regionAroundBase(b)
				}
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
		e.ActorStartClock, e.TargetStartClock = startClock(e.Actor), startClock(e.Target)
		if o := m.Outcome; o != (moments.Outcome{}) {
			e.Outcome = &workflowGameEventOutcome{Killed: int64(o.Killed), Lost: int64(o.Lost), Workers: int64(o.Workers), Buildings: int64(o.Buildings), TownHall: o.TownHall, First: o.First}
		}
		e.DurationSeconds = int64(m.Seconds)
		switch m.Type {
		case "recall":
			e.Base, e.TargetBase, e.TargetOwner = base(m.SourceBase), base(m.Base), player(m.Target)
			e.TargetPoint = e.Engine.Point
		}
		if e.Base == nil && m.Type != "first_corsair" && m.Type != "first_reaver" && m.Type != "speedlot" {
			e.Base = openFieldBase(m, metas, rec, playersBySlot, startClockByPlayerID, detail.Players)
		}
		if e.Actor == nil {
			continue
		}
		detail.GameEvents = append(detail.GameEvents, e)
	}
	sort.SliceStable(detail.GameEvents, func(i, j int) bool { return detail.GameEvents[i].Second < detail.GameEvents[j].Second })
}

// leaveEvents are the meta events whose player is gone from the engine's snapshot of that second.
var leaveEvents = map[string]bool{"leave_game": true, "player_dropped": true, "mass_disconnect": true, "player_stopped_playing": true}

func actorID(p *workflowGameEventPlayer) int64 {
	if p == nil {
		return 0
	}
	return p.PlayerID
}

// leaverBase is where a player who left or stopped playing is marked: the base they held the second
// before, their main first, else their start base. The engine removes a leaver's units on the very
// frame they leave, so the event's own snapshot shows nothing of theirs.
func leaverBase(dg *engine.Digest, bases []moments.Base, sec int, slot int8) int {
	if b := heldBase(dg, bases, max(sec-1, 0), slot); b != moments.NoBase {
		return b
	}
	return heldBase(dg, bases, 0, slot)
}

func heldBase(dg *engine.Digest, bases []moments.Base, sec int, slot int8) int {
	held := moments.NoBase
	for _, o := range moments.BaseOwners(dg, bases, sec, coarseHalls(moments.SnapshotAt(dg, sec))) {
		if o.Owner != slot {
			continue
		}
		if bases[o.Base].Kind == moments.BaseStart {
			return o.Base
		}
		if held == moments.NoBase {
			held = o.Base
		}
	}
	return held
}

func momentPlayers(rec *library.Replay) []moments.Player {
	var players []moments.Player
	for _, p := range rec.Players {
		players = append(players, moments.Player{Slot: int8(p.Slot), Race: engineRace(p.Race), Human: p.IsHuman() && !p.IsObserver()})
	}
	return players
}

func momentBases(metas []overlayBaseMeta) []moments.Base {
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
	return bases
}

// listedEngineEvents are the engine's game events that the games list shows and filters on.
var listedEngineEvents = map[string]bool{"drop": true, "reaver_drop": true, "dt_drop": true}

// engineFeatures runs the game-event detectors on a fresh digest for the games list. Bases come from the
// layout the library already holds, so no replay file is read.
func (d *Dashboard) engineFeatures(checksum [32]byte, dg *engine.Digest) (features []string) {
	defer func() {
		if p := recover(); p != nil {
			crashreport.ReportPanic(p, debug.Stack())
			features = nil
		}
	}()
	rec, ok := d.library.lib.Snapshot().ByChecksum(checksum)
	if !ok {
		return nil
	}
	if rec.Layout == nil {
		return nil
	}
	features = []string{}
	seen := map[string]bool{}
	for _, m := range moments.Detect(dg, libraryBases(rec.Layout), momentPlayers(rec)) {
		if listedEngineEvents[m.Type] && !seen[m.Type] {
			seen[m.Type] = true
			features = append(features, m.Type)
		}
	}
	return features
}

func libraryBases(layout *library.MapLayout) []moments.Base {
	naturals := map[string]bool{}
	for _, b := range layout.Bases {
		if isStartKind(b.Kind) {
			naturals[b.NaturalExpansion] = true
		}
	}
	bases := make([]moments.Base, 0, len(layout.Bases))
	for _, b := range layout.Bases {
		kind := moments.BaseExpansion
		switch {
		case isStartKind(b.Kind):
			kind = moments.BaseStart
		case naturals[b.Name]:
			kind = moments.BaseNatural
		}
		poly := make([][2]int32, len(b.Polygon))
		for i, pt := range b.Polygon {
			poly[i] = [2]int32{pt.X, pt.Y}
		}
		bases = append(bases, moments.Base{Kind: kind, X: b.CenterX, Y: b.CenterY, Polygon: poly})
	}
	return bases
}

func isStartKind(kind string) bool {
	return strings.EqualFold(kind, "start") || strings.EqualFold(kind, "starting")
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

// openFieldNear is how close the nearest base must be for a place in the open to be named after it.
const openFieldNear = 30 * 32

// openFieldBase names a place away from any base. In a 1v1, by where it sits on the line between the
// two mains, as the command-inferred attacks do; otherwise after the nearest base, or the middle of the
// map. Name is the English label; OpenField lets the browser say it in the reader's language.
func openFieldBase(m moments.Event, metas []overlayBaseMeta, rec *library.Replay, players map[int8]workflowGameEventPlayer, startClockByPlayerID map[int64]int, roster []workflowGamePlayer) *workflowGameEventBase {
	out := &workflowGameEventBase{Kind: "open_field", Center: workflowGameEventPoint{X: float64(m.X), Y: float64(m.Y)}}
	set := func(relation, ref, name string) *workflowGameEventBase {
		out.OpenField = &workflowOpenField{Relation: relation, Ref: ref}
		out.Name = name
		return out
	}
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
	if aok && bok && len(mains) == 2 {
		ta, tb := players[m.Target].Name, players[m.Actor].Name
		ax, ay := b[0]-a[0], b[1]-a[1]
		if length := ax*ax + ay*ay; length > 0 {
			t := ((float64(m.X)-a[0])*ax + (float64(m.Y)-a[1])*ay) / length
			switch {
			case t < 0.30:
				return set("near_player_base", ta, "near "+ta+"'s base")
			case t < 0.45:
				return set("player_side", ta, "on "+ta+"'s side of the map")
			case t <= 0.55:
				return set("middle", "", "in the middle")
			case t < 0.70:
				return set("player_side", tb, "on "+tb+"'s side of the map")
			default:
				return set("near_player_base", tb, "near "+tb+"'s base")
			}
		}
	}
	best, bestDist := -1, float64(openFieldNear)
	for i, meta := range metas {
		if d := math.Hypot(meta.Base.Center.X-float64(m.X), meta.Base.Center.Y-float64(m.Y)); d <= bestDist {
			best, bestDist = i, d
		}
	}
	if best < 0 {
		return set("middle", "", "in the middle")
	}
	near := []workflowGameEvent{{Base: &workflowGameEventBase{Name: metas[best].Base.Name, Kind: metas[best].Base.Kind, NaturalOfClock: metas[best].Base.NaturalOfClock}}}
	name := resolveNaturalOwnerLabels(near, roster, startClockByPlayerID)[0].Base.Name
	return set("near_base", name, "near "+name)
}
