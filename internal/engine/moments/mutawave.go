package moments

import (
	"math"

	"github.com/marianogappa/openbw-go"
	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

const (
	mutaArrivalRadius = 10 * tile
	mutaWaveFrames    = 60 * framesPerSecond
)

// MutaWave is how a Zerg's Mutalisks met a Terran: when the first ones reached a Terran base, what anti-air
// stood there by then, and what Mutalisks killed and lost over the game. Frames are engine frames.
type MutaWave struct {
	FirstHatch      int32
	Arrival         int32
	MutasAtArrival  int
	TurretsUp       int
	TurretsBuilding int
	TurretsReady    []int32
	Wave            MutaLedger
	Game            MutaLedger
	Kills           []MutaKill
}

// MutaLedger counts Terran losses to Mutalisks and the Mutalisks Zerg lost, with what killed them.
type MutaLedger struct {
	Workers, Turrets, OtherBuildings, Units int
	Killed                                  Cost
	MutasLost                               int
	MutasLostToTurrets                      int
	Lost                                    Cost
}

// MutaKill is one death on either side of the harass: a Terran unit a Mutalisk killed, or a Mutalisk.
type MutaKill struct {
	Frame int32
	Type  uint16
	Muta  bool
}

var (
	mutaType   = ut(bwgame.UnitTypesZergMutalisk)
	turretType = ut(bwgame.UnitTypesTerranMissileTurret)
	ccType     = ut(bwgame.UnitTypesTerranCommandCenter)
)

// DetectMutaWave is nil when zerg never hatched a Mutalisk inside the trusted part of the replay.
func DetectMutaWave(d *engine.Digest, zerg, terran int8) *MutaWave {
	trusted := d.TrustedUntil()
	w := &MutaWave{FirstHatch: engine.NoFrame, Arrival: engine.NoFrame}
	for _, p := range d.Productions {
		if p.Owner == zerg && p.Type == mutaType && p.Complete != engine.NoFrame && p.Complete < trusted &&
			(w.FirstHatch == engine.NoFrame || p.Complete < w.FirstHatch) {
			w.FirstHatch = p.Complete
		}
	}
	if w.FirstHatch == engine.NoFrame {
		return nil
	}
	died := map[uint32]int32{}
	for _, dt := range d.Deaths {
		died[dt.Unit] = dt.Frame
	}
	halls := terranHalls(d, terran, died)
	w.Arrival = mutaArrival(d, zerg, terran, halls, w.FirstHatch, trusted)
	if w.Arrival != engine.NoFrame {
		w.countAtArrival(d, zerg, terran, died)
	}
	w.ledger(d, zerg, terran, trusted)
	return w
}

type hallLife struct {
	x, y       int32
	from, upTo int32
}

func terranHalls(d *engine.Digest, terran int8, died map[uint32]int32) []hallLife {
	alive := func(unit uint32) int32 {
		if f, ok := died[unit]; ok {
			return f
		}
		return math.MaxInt32
	}
	var halls []hallLife
	for _, u := range d.Initial {
		if u.Owner == terran && u.Type == ccType {
			halls = append(halls, hallLife{u.X, u.Y, 0, alive(u.Unit)})
		}
	}
	for _, p := range d.Productions {
		if p.Owner == terran && p.Type == ccType && p.Complete != engine.NoFrame {
			halls = append(halls, hallLife{p.X, p.Y, p.Complete, alive(p.Unit)})
		}
	}
	return halls
}

func nearHall(halls []hallLife, frame, x, y int32) bool {
	for _, h := range halls {
		if frame >= h.from && frame < h.upTo && math.Hypot(float64(x-h.x), float64(y-h.y)) <= mutaArrivalRadius {
			return true
		}
	}
	return false
}

// mutaArrival is the first frame a Mutalisk came within reach of a standing Terran town hall. Army positions
// come every ArmyEveryFrames, so the crossing is interpolated along the Mutalisk's path from the snapshot
// before; a Terran unit a Mutalisk killed by a town hall caps it from below.
func mutaArrival(d *engine.Digest, zerg, terran int8, halls []hallLife, firstHatch, trusted int32) int32 {
	arrival := int32(engine.NoFrame)
	earlier := func(f int32) {
		if f >= firstHatch && f < trusted && (arrival == engine.NoFrame || f < arrival) {
			arrival = f
		}
	}
	for _, dt := range d.Deaths {
		if dt.Owner == terran && dt.KillerOwner == zerg && dt.KillerType == mutaType && nearHall(halls, dt.Frame, dt.X, dt.Y) {
			earlier(dt.Frame)
			break
		}
	}
	prev := map[uint32][2]int32{}
	prevFrame := int32(engine.NoFrame)
	for _, s := range d.Army {
		if s.Frame < firstHatch {
			continue
		}
		if s.Frame >= trusted || arrival != engine.NoFrame && s.Frame >= arrival {
			break
		}
		cur := map[uint32][2]int32{}
		for _, u := range s.Units {
			if u.Owner != zerg || u.Type != mutaType {
				continue
			}
			x, y := u.X*engine.ArmyCellPixels+engine.ArmyCellPixels/2, u.Y*engine.ArmyCellPixels+engine.ArmyCellPixels/2
			cur[u.ID] = [2]int32{x, y}
			if !nearHall(halls, s.Frame, x, y) {
				continue
			}
			f := s.Frame
			if from, ok := prev[u.ID]; ok && prevFrame != engine.NoFrame {
				f = crossing(halls, prevFrame, s.Frame, from, [2]int32{x, y})
			}
			earlier(f)
		}
		prev, prevFrame = cur, s.Frame
	}
	return arrival
}

func crossing(halls []hallLife, f0, f1 int32, a, b [2]int32) int32 {
	const steps = 20
	for i := 1; i <= steps; i++ {
		t := float64(i) / steps
		x := a[0] + int32(t*float64(b[0]-a[0]))
		y := a[1] + int32(t*float64(b[1]-a[1]))
		f := f0 + int32(t*float64(f1-f0))
		if nearHall(halls, f, x, y) {
			return f
		}
	}
	return f1
}

func (w *MutaWave) countAtArrival(d *engine.Digest, zerg, terran int8, died map[uint32]int32) {
	gone := func(unit uint32, at int32) bool {
		f, ok := died[unit]
		return ok && f <= at
	}
	for _, p := range d.Productions {
		cancelled := p.Cancel != engine.NoFrame && p.Cancel <= w.Arrival
		switch {
		case p.Owner == zerg && p.Type == mutaType:
			if p.Complete != engine.NoFrame && p.Complete <= w.Arrival && !gone(p.Unit, w.Arrival) {
				w.MutasAtArrival++
			}
		case p.Owner == terran && p.Type == turretType:
			if p.Complete != engine.NoFrame && p.Complete <= w.Arrival+mutaWaveFrames {
				w.TurretsReady = append(w.TurretsReady, p.Complete)
			}
			if cancelled || p.Start > w.Arrival || gone(p.Unit, w.Arrival) {
				continue
			}
			if p.Complete != engine.NoFrame && p.Complete <= w.Arrival {
				w.TurretsUp++
			} else {
				w.TurretsBuilding++
			}
		}
	}
}

func (w *MutaWave) ledger(d *engine.Digest, zerg, terran int8, trusted int32) {
	for _, dt := range d.Deaths {
		if dt.Frame >= trusted {
			break
		}
		inWave := w.Arrival != engine.NoFrame && dt.Frame >= w.Arrival && dt.Frame < w.Arrival+mutaWaveFrames
		switch {
		case dt.Owner == terran && dt.KillerOwner == zerg && dt.KillerType == mutaType:
			w.Game.addKill(dt.Type)
			if inWave {
				w.Wave.addKill(dt.Type)
			}
			w.Kills = append(w.Kills, MutaKill{Frame: dt.Frame, Type: dt.Type})
		case dt.Owner == zerg && dt.Type == mutaType && dt.KillerOwner == terran:
			w.Game.addLoss(dt.KillerType)
			if inWave {
				w.Wave.addLoss(dt.KillerType)
			}
			w.Kills = append(w.Kills, MutaKill{Frame: dt.Frame, Type: dt.KillerType, Muta: true})
		}
	}
}

func (l *MutaLedger) addKill(t uint16) {
	switch {
	case isWorker(t):
		l.Workers++
	case t == turretType:
		l.Turrets++
	case isBuilding(t):
		l.OtherBuildings++
	default:
		l.Units++
	}
	c := CostOf(t)
	l.Killed.Minerals += c.Minerals
	l.Killed.Gas += c.Gas
}

func (l *MutaLedger) addLoss(killer uint16) {
	l.MutasLost++
	if killer == turretType {
		l.MutasLostToTurrets++
	}
	c := CostOf(mutaType)
	l.Lost.Minerals += c.Minerals
	l.Lost.Gas += c.Gas
}

func isBuilding(t uint16) bool {
	_, ok := FootprintOf(t)
	return ok
}

// OneVsOneTvZ is the Zerg and Terran slots of a game with exactly those two players.
func OneVsOneTvZ(d *engine.Digest) (zerg, terran int8, ok bool) {
	if len(d.Players) != 2 {
		return 0, 0, false
	}
	zerg, terran = -1, -1
	for _, p := range d.Players {
		switch openbw.Race(p.Race) {
		case openbw.RaceZerg:
			zerg = p.Slot
		case openbw.RaceTerran:
			terran = p.Slot
		}
	}
	return zerg, terran, zerg >= 0 && terran >= 0
}
