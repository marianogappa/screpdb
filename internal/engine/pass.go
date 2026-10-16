package engine

import (
	"context"
	"fmt"
	"slices"

	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/gamedata"
	"github.com/marianogappa/screpsim"
)

const maxPlayerSlot = 7

type Data = openbw.Data

func LoadData(dir string) (*Data, error) { return openbw.LoadData(dir) }

// OpenGameData extracts the game data from the StarCraft: Remastered install when needed and loads it.
// gamedata.ErrNoInstall means there is no install: the engine pass stays off and only the fast pass runs.
func OpenGameData() (*Data, error) {
	dir, _, err := gamedata.Prepare()
	if err != nil {
		return nil, err
	}
	return LoadData(dir)
}

// Pass runs the replay through the engine. It stops early with ctx's error, at the next game second.
func Pass(ctx context.Context, replay []byte, data *Data) (*Digest, error) {
	g, err := screpsim.Open(replay, screpsim.Options{Engine: openbw.Options{
		Data:      data,
		Kinds:     openbw.DefaultKinds | openbw.KindCommandResult | openbw.KindTick,
		TickEvery: EconomyEveryFrames,
	}})
	if err != nil {
		return nil, fmt.Errorf("engine pass: %w", err)
	}
	p := newPassState(g)
	for e := range g.Events() {
		p.handle(e)
		if _, tick := e.(*openbw.Tick); tick && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	p.finish(g.Err())
	return p.d, nil
}

type researchKey struct {
	owner int8
	r     openbw.Research
}

type passState struct {
	g            *screpsim.Game
	d            *Digest
	actionToCmd  []int32
	slots        []int8
	openUnits    map[openbw.UnitID]int
	lastByUnit   map[openbw.UnitID]int
	openResearch map[researchKey]int
	ticks        int
	selects      []selectResult
	carriedBy    map[openbw.UnitID]openbw.UnitID
	lastSeen     map[openbw.UnitID]seenAt
}

type seenAt struct {
	x, y  int32
	owner int8
	typ   openbw.UnitType
}

func newPassState(g *screpsim.Game) *passState {
	e := g.Engine()
	p := &passState{
		g:            g,
		d:            &Digest{EngineVersion: EngineVersion, EndFrame: e.EndFrame(), Drift: Drift{ErrorFrame: NoFrame, FirstMissing: NoFrame, DesyncFrame: NoFrame}},
		openUnits:    map[openbw.UnitID]int{},
		lastByUnit:   map[openbw.UnitID]int{},
		openResearch: map[researchKey]int{},
		carriedBy:    map[openbw.UnitID]openbw.UnitID{},
		lastSeen:     map[openbw.UnitID]seenAt{},
	}
	for _, s := range e.PlayingSlots() {
		if s > maxPlayerSlot {
			continue
		}
		p.slots = append(p.slots, s)
		pl := Player{Slot: s, PlayerID: -1, Race: uint8(e.Player(s).Race)}
		if rp := g.Player(s); rp != nil {
			pl.PlayerID = int32(rp.ID)
		}
		p.d.Players = append(p.d.Players, pl)
	}
	for i, l := range g.CommandLinks() {
		if l.Action < 0 {
			p.d.Drift.UnlinkedCommands++
			continue
		}
		p.d.Drift.LinkedCommands++
		for int(l.Action) >= len(p.actionToCmd) {
			p.actionToCmd = append(p.actionToCmd, NoCmd)
		}
		p.actionToCmd[l.Action] = int32(i)
	}
	return p
}

func (p *passState) cmdOf(b *openbw.EventBase) int32 {
	if b.Cmd < 0 || int(b.Cmd) >= len(p.actionToCmd) {
		return NoCmd
	}
	return p.actionToCmd[b.Cmd]
}

func (p *passState) handle(e openbw.Event) {
	switch e := e.(type) {
	case *openbw.UnitCreated:
		p.unitCreated(e)
	case *openbw.MorphStarted:
		p.morphStarted(e)
	case *openbw.UnitCompleted:
		p.closeUnit(e.Unit, func(pr *Production) { pr.Complete = e.Frame })
	case *openbw.ProductionCancelled:
		p.productionCancelled(e)
	case *openbw.UnitDestroyed:
		p.unitDestroyed(e)
	case *openbw.UnitOwnerChanged:
		if e.From <= maxPlayerSlot || e.To <= maxPlayerSlot {
			p.d.OwnerChanges = append(p.d.OwnerChanges, OwnerChange{Frame: e.Frame, Unit: uint32(e.Unit), Type: uint16(e.Type), From: e.From, To: e.To, Reason: uint8(e.Reason)})
		}
	case *openbw.ResearchStarted:
		p.openResearch[researchKey{e.Owner, e.Research}] = len(p.d.Research)
		p.d.Research = append(p.d.Research, Research{Owner: e.Owner, Upgrade: e.Research.Upgrade, ID: e.Research.ID, Level: e.Level, Producer: uint32(e.Unit), Start: e.Frame, Complete: NoFrame, Cancel: NoFrame})
	case *openbw.ResearchCompleted:
		p.closeResearch(e.Owner, e.Research, func(r *Research) { r.Complete = e.Frame })
	case *openbw.PlayerLeft:
		kind := OutcomeLeft
		if e.Dropped {
			kind = OutcomeDropped
		}
		p.d.Outcomes = append(p.d.Outcomes, Outcome{Frame: e.Frame, Owner: e.Owner, Kind: kind})
	case *openbw.PlayerDefeated:
		p.d.Outcomes = append(p.d.Outcomes, Outcome{Frame: e.Frame, Owner: e.Owner, Kind: OutcomeDefeated})
	case *openbw.PlayerWon:
		p.d.Outcomes = append(p.d.Outcomes, Outcome{Frame: e.Frame, Owner: e.Owner, Kind: OutcomeWon})
	case *screpsim.CommandResult:
		p.commandResult(e)
	case *openbw.Tick:
		p.tick(e.Frame)
	}
	p.d.LastFrame = e.Base().Frame
}

func isProduction(e *openbw.UnitCreated) bool {
	if e.Owner > maxPlayerSlot || e.Class != openbw.ClassNormal {
		return false
	}
	switch e.Source {
	case openbw.SourceTrained, openbw.SourceBuilt, openbw.SourceSecondEggUnit:
		return true
	}
	return false
}

func (p *passState) unitCreated(e *openbw.UnitCreated) {
	if e.Source == openbw.SourceStart && e.Owner <= maxPlayerSlot && e.Class == openbw.ClassNormal {
		p.d.Initial = append(p.d.Initial, InitialUnit{Unit: uint32(e.Unit), Type: uint16(e.Type), Owner: e.Owner, X: e.X, Y: e.Y})
		return
	}
	if !isProduction(e) {
		return
	}
	pr := Production{Owner: e.Owner, Type: uint16(e.Type), From: uint16(openbw.UnitTypeNone), Unit: uint32(e.Unit), Producer: uint32(e.Parent), X: e.X, Y: e.Y, Start: e.Frame, Complete: NoFrame, Cancel: NoFrame, Cmd: p.cmdOf(&e.EventBase)}
	if e.Source == openbw.SourceSecondEggUnit {
		if i, ok := p.lastByUnit[e.Parent]; ok {
			pr.Start, pr.Cmd, pr.From = p.d.Productions[i].Start, p.d.Productions[i].Cmd, p.d.Productions[i].From
		}
	}
	p.open(e.Unit, pr)
}

func (p *passState) morphStarted(e *openbw.MorphStarted) {
	if e.Owner > maxPlayerSlot || e.Class != openbw.ClassNormal {
		return
	}
	pr := Production{Owner: e.Owner, Type: uint16(e.Into), From: uint16(e.From), Unit: uint32(e.Unit), Producer: uint32(e.Morpher), Start: e.Frame, Complete: NoFrame, Cancel: NoFrame, Cmd: p.cmdOf(&e.EventBase)}
	if u, ok := p.g.Engine().Unit(e.Unit); ok {
		pr.X, pr.Y = u.X, u.Y
	}
	p.open(e.Unit, pr)
}

func (p *passState) open(id openbw.UnitID, pr Production) {
	p.openUnits[id] = len(p.d.Productions)
	p.lastByUnit[id] = len(p.d.Productions)
	p.d.Productions = append(p.d.Productions, pr)
}

func (p *passState) closeUnit(id openbw.UnitID, set func(*Production)) {
	i, ok := p.openUnits[id]
	if !ok {
		return
	}
	delete(p.openUnits, id)
	set(&p.d.Productions[i])
}

func (p *passState) closeResearch(owner int8, r openbw.Research, set func(*Research)) {
	k := researchKey{owner, r}
	i, ok := p.openResearch[k]
	if !ok {
		return
	}
	delete(p.openResearch, k)
	set(&p.d.Research[i])
}

func (p *passState) productionCancelled(e *openbw.ProductionCancelled) {
	switch e.What {
	case openbw.ProductionResearch, openbw.ProductionUpgrade:
		p.closeResearch(e.Owner, e.Research, func(r *Research) { r.Cancel = e.Frame })
	default:
		if e.Unit != 0 {
			p.closeUnit(e.Unit, func(pr *Production) { pr.Cancel = e.Frame })
		}
	}
}

func (p *passState) unitDestroyed(e *openbw.UnitDestroyed) {
	if e.Reason == openbw.ReasonCancelled {
		p.closeUnit(e.Unit, func(pr *Production) { pr.Cancel = e.Frame })
	} else {
		delete(p.openUnits, e.Unit)
	}
	if e.Owner > maxPlayerSlot || e.Class == openbw.ClassSpell {
		return
	}
	p.d.Deaths = append(p.d.Deaths, Death{
		Frame: e.Frame, Unit: uint32(e.Unit), Type: uint16(e.Type), Owner: e.Owner, Class: uint8(e.Class),
		X: e.X, Y: e.Y, Reason: uint8(e.Reason), KillerType: uint16(e.KillerType), KillerOwner: e.KillerOwner,
	})
}

func (p *passState) tick(frame int32) {
	e := p.g.Engine()
	var workers, larva [maxPlayerSlot + 1]int32
	snapshot := p.ticks%(ArmyEveryFrames/EconomyEveryFrames) == 0
	p.ticks++
	var army []ArmyUnit
	carried := map[openbw.UnitID]openbw.UnitID{}
	seen := map[openbw.UnitID]seenAt{}
	for u := range e.Units() {
		if u.Owner > maxPlayerSlot || u.Dying {
			continue
		}
		info := e.UnitTypeInfo(u.Type)
		if u.Type == larvaType {
			larva[u.Owner]++
		}
		if info.Worker && u.Completed {
			workers[u.Owner]++
		}
		if u.Loaded && u.Container != 0 {
			carried[u.ID] = u.Container
			continue
		}
		if !onMap(u, info) {
			continue
		}
		seen[u.ID] = seenAt{x: u.X, y: u.Y, owner: u.Owner, typ: u.Type}
		if snapshot && !info.Worker {
			army = append(army, ArmyUnit{Owner: u.Owner, Type: uint16(u.Type), X: u.X / ArmyCellPixels, Y: u.Y / ArmyCellPixels})
		}
	}
	p.trackUnloads(frame, carried)
	p.trackTeleports(frame, seen)
	for _, s := range p.slots {
		pl := e.Player(s)
		p.d.Economy = append(p.d.Economy, EconomySample{
			Frame: frame, Owner: s, Minerals: pl.Minerals, Gas: pl.Gas,
			GatheredMinerals: pl.GatheredMinerals, GatheredGas: pl.GatheredGas,
			SupplyUsed: int32(pl.SupplyUsed * 2), SupplyMax: int32(pl.SupplyMax * 2),
			Workers: workers[s], Larva: larva[s],
		})
	}
	if snapshot {
		p.d.Army = append(p.d.Army, ArmySnapshot{Frame: frame, Units: army})
	}
}

// trackUnloads compares who rides in which transport with a second ago: units that left a Dropship,
// Shuttle or Overlord and are alive on the map were unloaded. A Bunker is not a transport.
func (p *passState) trackUnloads(frame int32, carried map[openbw.UnitID]openbw.UnitID) {
	e := p.g.Engine()
	byTransport := map[openbw.UnitID]*Unload{}
	var order []openbw.UnitID
	for unit, transport := range p.carriedBy {
		if carried[unit] == transport {
			continue
		}
		cargo, alive := e.Unit(unit)
		t, tok := e.Unit(transport)
		if !alive || cargo.Loaded || !tok || !isTransport(t.Type) {
			continue
		}
		u, ok := byTransport[transport]
		if !ok {
			u = &Unload{Frame: frame, Transport: uint32(transport), TransportType: uint16(t.Type), Owner: t.Owner, X: t.X, Y: t.Y}
			byTransport[transport] = u
			order = append(order, transport)
		}
		u.Cargo = append(u.Cargo, uint16(cargo.Type))
	}
	slices.Sort(order)
	for _, id := range order {
		slices.Sort(byTransport[id].Cargo)
		p.d.Unloads = append(p.d.Unloads, *byTransport[id])
	}
	p.carriedBy = carried
}

const teleportPixels = 16 * 32

// trackTeleports finds units that crossed more than 16 tiles in one second, which no unit can walk or
// fly: a Recall or a Nydus Canal. Units just unloaded were not on the map a second ago, so they never count.
func (p *passState) trackTeleports(frame int32, seen map[openbw.UnitID]seenAt) {
	byOwner := map[int8]*Teleport{}
	var owners []int8
	var sums = map[int8]*[4]int64{}
	for id, now := range seen {
		before, ok := p.lastSeen[id]
		if !ok || before.owner != now.owner {
			continue
		}
		dx, dy := int64(now.x-before.x), int64(now.y-before.y)
		if dx*dx+dy*dy <= teleportPixels*teleportPixels {
			continue
		}
		t, ok := byOwner[now.owner]
		if !ok {
			t = &Teleport{Frame: frame, Owner: now.owner}
			byOwner[now.owner] = t
			sums[now.owner] = &[4]int64{}
			owners = append(owners, now.owner)
		}
		t.Units = append(t.Units, uint16(now.typ))
		s := sums[now.owner]
		s[0] += int64(before.x)
		s[1] += int64(before.y)
		s[2] += int64(now.x)
		s[3] += int64(now.y)
	}
	slices.Sort(owners)
	for _, o := range owners {
		t, s := byOwner[o], sums[o]
		n := int64(len(t.Units))
		t.FromX, t.FromY, t.ToX, t.ToY = int32(s[0]/n), int32(s[1]/n), int32(s[2]/n), int32(s[3]/n)
		slices.Sort(t.Units)
		p.d.Teleports = append(p.d.Teleports, *t)
	}
	p.lastSeen = seen
}

func isTransport(t openbw.UnitType) bool {
	switch bwgame.UnitTypes(t) {
	case bwgame.UnitTypesTerranDropship, bwgame.UnitTypesProtossShuttle, bwgame.UnitTypesZergOverlord:
		return true
	}
	return false
}

const (
	larvaType     = openbw.UnitType(bwgame.UnitTypesZergLarva)
	eggType       = openbw.UnitType(bwgame.UnitTypesZergEgg)
	cocoonType    = openbw.UnitType(bwgame.UnitTypesZergCocoon)
	lurkerEggType = openbw.UnitType(bwgame.UnitTypesZergLurkerEgg)
)

// onMap is a unit the digest tracks for detection: built, on the ground or in the air, and not a building,
// whose life the productions and deaths already give exactly.
func onMap(u openbw.Unit, info openbw.UnitTypeInfo) bool {
	return u.Completed && !u.Loaded && !u.Hallucination && !info.Building && u.Type != eggType && u.Type != larvaType && u.Type != cocoonType && u.Type != lurkerEggType
}

func (p *passState) finish(err error) {
	p.d.Drift.DesyncFrame = desyncOnset(p.selects)
	if err == nil {
		return
	}
	p.d.Drift.EngineError = err.Error()
	p.d.Drift.ErrorFrame = p.d.LastFrame
}
