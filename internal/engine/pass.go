package engine

import (
	"fmt"

	openbw "github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpsim"
)

const maxPlayerSlot = 7

type Data = openbw.Data

func LoadData(dir string) (*Data, error) { return openbw.LoadData(dir) }

func Pass(replay []byte, data *Data) (*Digest, error) {
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
}

func newPassState(g *screpsim.Game) *passState {
	e := g.Engine()
	p := &passState{
		g:            g,
		d:            &Digest{EngineVersion: EngineVersion, EndFrame: e.EndFrame(), Drift: Drift{ErrorFrame: NoFrame, FirstMissing: NoFrame, DesyncFrame: NoFrame}},
		openUnits:    map[openbw.UnitID]int{},
		lastByUnit:   map[openbw.UnitID]int{},
		openResearch: map[researchKey]int{},
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
	for u := range e.Units() {
		if u.Owner > maxPlayerSlot || u.Dying {
			continue
		}
		info := e.UnitTypeInfo(u.Type)
		switch {
		case u.Type == larvaType:
			larva[u.Owner]++
		case info.Worker:
			if u.Completed {
				workers[u.Owner]++
			}
		case snapshot && isArmy(u, info):
			army = append(army, ArmyUnit{Owner: u.Owner, Type: uint16(u.Type), X: u.X / ArmyCellPixels, Y: u.Y / ArmyCellPixels})
		}
	}
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

const (
	larvaType = openbw.UnitType(bwgame.UnitTypesZergLarva)
	eggType   = openbw.UnitType(bwgame.UnitTypesZergEgg)
)

func isArmy(u openbw.Unit, info openbw.UnitTypeInfo) bool {
	return u.Completed && !u.Loaded && !u.Hallucination && !info.Building && u.Type != eggType && info.Supply > 0
}

func (p *passState) finish(err error) {
	p.d.Drift.DesyncFrame = desyncOnset(p.selects)
	if err == nil {
		return
	}
	p.d.Drift.EngineError = err.Error()
	p.d.Drift.ErrorFrame = p.d.LastFrame
}
