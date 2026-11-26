//go:build !js

package dashboard

import (
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/utils"
	"math"
	"runtime/debug"
	"slices"
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
	bases := withChokes(momentBases(metas), layout)
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

	var toEvent func(m moments.Event) (workflowGameEvent, bool)
	toEvent = func(m moments.Event) (workflowGameEvent, bool) {
		e := workflowGameEvent{
			Type:   momentEventType(m, rec),
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
			e.Outcome = &workflowGameEventOutcome{Killed: int64(o.Killed), Lost: int64(o.Lost), Workers: int64(o.Workers), Buildings: int64(o.Buildings), TownHall: o.TownHall, First: o.First, KilledUnits: int64(o.KilledUnits)}
		}
		e.DurationSeconds = int64(m.Seconds)
		e.Result, e.UnitCount, e.Hallucinated = m.Result, int64(m.Count), int64(m.Hallucinated)
		switch m.Type {
		case "recall":
			e.Base, e.TargetBase, e.TargetOwner = base(m.SourceBase), base(m.Base), player(m.Target)
			e.TargetPoint = e.Engine.Point
		}
		if e.Base == nil && m.Type != "first_corsair" && m.Type != "first_reaver" && m.Type != "speedlot" {
			e.Base = openFieldBase(m, metas, rec, playersBySlot, startClockByPlayerID, detail.Players)
		}
		for _, p := range m.Path {
			e.Path = append(e.Path, workflowGameEventPoint{X: float64(p[0]), Y: float64(p[1])})
		}
		if m.Count > 0 && e.UnitCount == 0 {
			e.UnitCount = int64(m.Count)
		}
		switch m.Type {
		case "drop", "reaver_drop", "dt_drop", "ht_drop", "lurker_drop":
			e.UnitCount = 0
			if m.Count > 1 {
				e.DropCount = int64(m.Count)
			}
		}
		e.Defenders = int64(m.Defenders)
		e.RebuiltAfterSecond = int64(m.RebuiltAfter)
		for _, p := range m.Parts {
			if part, ok := toEvent(p); ok {
				part.Base, part.Engine = nil, nil
				e.Parts = append(e.Parts, part)
			}
		}
		return e, e.Actor != nil
	}
	events, _ := moments.Combine(append(moments.ChainPushes(moments.Detect(dg, bases, players)), moments.DetectInsights(dg, bases, players).Events...), -1)
	you := d.youSlot(rec)
	tp := moments.TurningPoints(dg, bases, players, events, you)
	roles := turningPointRoles(tp, you, detail.OwnCopyFileName != "")
	type teller struct {
		told workflowGameEvent
		keep string
	}
	var tellers []teller
	for i, m := range events {
		e, ok := toEvent(m)
		if !ok {
			continue
		}
		e.WhyRoles = roles[i]
		e.Why = len(roles[i]) > 0
		if slices.Contains(roles[i], whyRoleYou) && tp.Ally != moments.NoPlayer {
			e.WhyAfterAlly, e.WhyLeaver = player(tp.Ally), player(you)
		}
		if len(roles[i]) > 0 {
			e.WhyYou, e.WhyAllies = player(you), int64(tp.LiveAllies)
			w := tp.Final
			if tp.You != nil && tp.You.Event == i {
				w = tp.You
			}
			if w != nil {
				e.WhyForce, e.WhyTargetBases = whyForce(w.Force), int64(w.TargetBases)
			}
		}
		if m.Insight {
			e.Engine.EndSecond = int64(m.Second + m.Seconds)
			tellers = append(tellers, teller{told: e})
			for _, p := range e.Parts {
				told := e
				told.Type = p.Type
				tellers = append(tellers, teller{told: told, keep: e.Type})
			}
		}
		detail.GameEvents = append(detail.GameEvents, e)
	}
	for _, t := range tellers {
		detail.GameEvents = eventsTold(detail.GameEvents, t.told, t.keep)
	}
	sort.SliceStable(detail.GameEvents, func(i, j int) bool { return detail.GameEvents[i].Second < detail.GameEvents[j].Second })
}

// plainEventsTold are the plain events an insight already tells when it happened at the same base, by the
// same player, in its window: the Eraser's worker kills, the elevator's drop.
var plainEventsTold = map[string]map[string]bool{
	moments.InsightElevator:        {"drop": true, "reaver_drop": true, "dt_drop": true, "ht_drop": true, "lurker_drop": true, "attack": true, "worker_kills": true},
	moments.InsightEraser:          {"worker_kills": true, "attack": true},
	moments.InsightUndefendedAir:   {"worker_kills": true, "attack": true},
	moments.InsightUndetectedCloak: {"worker_kills": true, "attack": true, "dt_drop": true},
	moments.InsightOverrun:         {"worker_kills": true, "attack": true},
}

// keep is a type never removed: a combined event's own, which a part folded into it may otherwise tell.
func eventsTold(events []workflowGameEvent, told workflowGameEvent, keep string) []workflowGameEvent {
	types := plainEventsTold[told.Type]
	if types == nil {
		return events
	}
	kept := events[:0]
	for _, e := range events {
		same := types[e.Type] && e.Type != keep && len(e.WhyRoles) == 0 && actorID(e.Actor) == actorID(told.Actor) && e.Base != nil && told.Base != nil && e.Base.Name == told.Base.Name
		if same && e.Second >= told.Second-toldBefore && e.Second <= told.Engine.EndSecond+toldAfter {
			continue
		}
		kept = append(kept, e)
	}
	return kept
}

const (
	toldBefore = 20
	toldAfter  = 30
)

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
	teams := playerTeams(rec)
	var players []moments.Player
	for i, p := range rec.Players {
		result := int8(0)
		switch p.TeamOutcome() {
		case models.OutcomeWon:
			result = moments.ResultWon
		case models.OutcomeLost:
			result = moments.ResultLost
		}
		players = append(players, moments.Player{Slot: int8(p.Slot), Race: engineRace(p.Race), Human: p.IsHuman() && !p.IsObserver(), Team: teams[i], Result: result, LeftAt: int(p.LeaveSec)})
	}
	return players
}

// playerTeams gives each player, by ordinal, the side it ended the game on: the last alliance the replay
// observed, else the lobby teams when they group players, else none (0, every player on its own).
func playerTeams(rec *library.Replay) []int {
	teams := make([]int, len(rec.Players))
	if a := rec.Alliance; a != nil && len(a.Snapshots) > 0 {
		for t, members := range a.Snapshots[len(a.Snapshots)-1].Teams {
			for _, ordinal := range members {
				if int(ordinal) < len(teams) {
					teams[ordinal] = t + 1
				}
			}
		}
		return teams
	}
	distinct, shared := map[uint8]int{}, false
	for _, p := range rec.Players {
		if !p.IsObserver() {
			distinct[p.Team]++
			shared = shared || distinct[p.Team] > 1
		}
	}
	if len(distinct) < 2 || !shared {
		return teams
	}
	for i, p := range rec.Players {
		teams[i] = int(p.Team) + 1
	}
	return teams
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

// withChokes adds each base's chokes from the layout the bases were built from, in the same order.
func withChokes(bases []moments.Base, layout *models.MapContextLayout) []moments.Base {
	if layout == nil || len(layout.Bases) != len(bases) {
		return bases
	}
	index := map[string]int{}
	for i, b := range layout.Bases {
		index[b.Name] = i
	}
	for i, b := range layout.Bases {
		for _, c := range b.Chokes {
			to, ok := index[c.ConnectsTo]
			if !ok || c.ConnectsTo == "" {
				to = moments.NoBase
			}
			bases[i].Chokes = append(bases[i].Chokes, moments.Choke{X: int32(c.X), Y: int32(c.Y), To: to})
		}
	}
	return bases
}

// listedEngineEvents are the engine's game events that the games list shows and filters on.
var listedEngineEvents = map[string]bool{"drop": true, "reaver_drop": true, "dt_drop": true, "ht_drop": true, "lurker_drop": true, "mech_transition": true}

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
	bases, players := libraryBases(rec.Layout), momentPlayers(rec)
	for _, m := range moments.Detect(dg, bases, players) {
		if t := momentEventType(m, rec); listedEngineEvents[m.Type] && !seen[t] {
			seen[t] = true
			features = append(features, t)
		}
	}
	for _, m := range moments.DetectInsights(dg, bases, players).Events {
		if insightFeatures[m.Type] && !seen[m.Type] {
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
		var chokes []moments.Choke
		for _, c := range b.Chokes {
			chokes = append(chokes, moments.Choke{X: c.X, Y: c.Y, To: c.To})
		}
		bases = append(bases, moments.Base{Kind: kind, X: b.CenterX, Y: b.CenterY, Polygon: poly, Chokes: chokes})
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
	nearBase := &workflowGameEventBase{Name: metas[best].Base.Name, Kind: metas[best].Base.Kind, Clock: metas[best].Base.Clock, NaturalOfClock: metas[best].Base.NaturalOfClock}
	name := resolveNaturalOwnerLabels([]workflowGameEvent{{Base: nearBase}}, roster, startClockByPlayerID)[0].Base.Name
	out = set("near_base", name, "near "+name)
	out.OpenField.near = nearBase
	return out
}

const (
	whyRoleYou   = "you"
	whyRoleFinal = "final"
)

// turningPointRoles marks the turning points the summary shows. A viewer sees their own; why the game ended
// shows too when no viewer is known, or when a complete recording carries the game on past the viewer.
func turningPointRoles(tp moments.Perspective, you int8, completeCopy bool) map[int][]string {
	roles := map[int][]string{}
	if tp.You != nil {
		roles[tp.You.Event] = append(roles[tp.You.Event], whyRoleYou)
	}
	if tp.Final != nil && (you == moments.NoPlayer || completeCopy) {
		roles[tp.Final.Event] = append(roles[tp.Final.Event], whyRoleFinal)
	}
	return roles
}

func whyForce(f moments.Force) *workflowWhyForce {
	if f.Value <= 0 && f.Recalls == 0 {
		return nil
	}
	name := func(t uint16) string {
		if u := repcmd.UnitByID(t); u != nil {
			return u.Name
		}
		return ""
	}
	out := &workflowWhyForce{Value: int64(f.Value), Defence: int64(f.Defence), Count: int64(f.Count), Class: f.Class, Recalls: int64(f.Recalls)}
	for _, u := range f.Units {
		out.Units = append(out.Units, workflowWhyForceUnit{Name: name(u.Type), Count: int64(u.Count)})
	}
	if f.Mostly != 0 {
		out.Mostly = name(f.Mostly)
	}
	if f.MostlyPair[0] != 0 {
		out.Pair = []string{name(f.MostlyPair[0]), name(f.MostlyPair[1])}
	}
	return out
}

// youSlot is the slot of the user in a game, or NoPlayer when they did not play in it.
func (d *Dashboard) youSlot(rec *library.Replay) int8 {
	for _, p := range rec.Players {
		if !p.IsObserver() && d.isYouKey(p.Key) {
			return int8(p.Slot)
		}
	}
	return moments.NoPlayer
}

// momentEventType is a moment's event type, with a Dropship drop that lands on one of Big Game Hunters' corner
// cliffs named a cliff drop: the engine knows where every unload landed but has no notion of a cliff.
func momentEventType(m moments.Event, rec *library.Replay) string {
	if m.Type != "drop" || len(m.Units) == 0 || m.Units[0] != uint16(bwgame.UnitTypesTerranDropship) {
		return m.Type
	}
	if !utils.IsBigGameHuntersMap(library.Strings.Name(rec.Map)) ||
		!utils.IsCliffDropPosition(int(m.X), int(m.Y), int(rec.MapWidth)*32, int(rec.MapHeight)*32) {
		return m.Type
	}
	return "cliff_drop"
}
