package moments

import (
	"slices"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// A Force is what a turning point was done with, told loosely: how big it was next to the army that could
// answer it, and what it was mostly made of.
type Force struct {
	Value int32
	// Defence is the target side's whole army at the time, wherever it stood.
	Defence int32
	Count   int
	// Units are the force's unit types, most valuable share first.
	Units []UnitShare
	// Mostly is the unit type or class that makes up most of the force's value: a type when one does, else
	// ClassAir, ClassBio or ClassMech, else empty with MostlyPair naming its two main types.
	Mostly     uint16
	Class      string
	MostlyPair [2]uint16
	// Recalls is how many Recalls the actor cast in the four minutes up to the event.
	Recalls int
}

type UnitShare struct {
	Type  uint16
	Count int
	Value int32
}

const (
	ClassAir  = "air"
	ClassBio  = "bio"
	ClassMech = "mech"
)

const (
	forceReach      = 8 * tile
	forceMostlyType = 60
	forceMostlyKind = 70
	forceMostlyPair = 75
)

var unitClass = map[uint16]string{
	ut(bwgame.UnitTypesTerranWraith): ClassAir, ut(bwgame.UnitTypesTerranValkyrie): ClassAir, ut(bwgame.UnitTypesTerranBattlecruiser): ClassAir,
	ut(bwgame.UnitTypesTerranScienceVessel): ClassAir, ut(bwgame.UnitTypesProtossScout): ClassAir, ut(bwgame.UnitTypesProtossCarrier): ClassAir,
	ut(bwgame.UnitTypesProtossCorsair): ClassAir, ut(bwgame.UnitTypesProtossArbiter): ClassAir, ut(bwgame.UnitTypesZergMutalisk): ClassAir,
	ut(bwgame.UnitTypesZergGuardian): ClassAir, ut(bwgame.UnitTypesZergDevourer): ClassAir, ut(bwgame.UnitTypesZergScourge): ClassAir,
	ut(bwgame.UnitTypesTerranMarine): ClassBio, ut(bwgame.UnitTypesTerranFirebat): ClassBio, ut(bwgame.UnitTypesTerranMedic): ClassBio,
	ut(bwgame.UnitTypesTerranGhost): ClassBio,
	ut(bwgame.UnitTypesTerranVulture): ClassMech, ut(bwgame.UnitTypesTerranSiegeTankTankMode): ClassMech,
	ut(bwgame.UnitTypesTerranSiegeTankSiegeMode): ClassMech, ut(bwgame.UnitTypesTerranGoliath): ClassMech,
}

var transports = map[uint16]bool{ut(bwgame.UnitTypesTerranDropship): true, ut(bwgame.UnitTypesProtossShuttle): true, ut(bwgame.UnitTypesZergOverlord): true}

// force measures an event's force at its biggest between its start and its peak: what a drop set down when the
// engine named the units, else the actor's army in reach of the event.
func (g *game) force(e Event, events []Event) Force {
	from, to := e.Second-5, max(e.Second, e.SnapshotSecond)+5
	if e.Seconds > 0 {
		to = max(to, e.Second+e.Seconds)
	}
	area := e.Region
	if area.W == 0 || area.H == 0 {
		area = around(e.X, e.Y, overrunFightReach)
	}
	inReach := func(x, y int32) bool {
		return x >= area.X-forceReach && x <= area.X+area.W+forceReach && y >= area.Y-forceReach && y <= area.Y+area.H+forceReach
	}
	var best Force
	var bestUnits map[uint16]*UnitShare
	for _, snap := range g.d.Army {
		sec := second(snap.Frame)
		if sec < from {
			continue
		}
		if sec > to {
			break
		}
		f, units := Force{}, map[uint16]*UnitShare{}
		for _, u := range snap.Units {
			if !g.enemies(u.Owner, e.Target) && g.players[u.Owner].Human && mobileArmy(u.Type) {
				f.Defence += CostOf(u.Type).Value()
			}
			if !g.ofForce(e, u) {
				continue
			}
			if len(e.Members) == 0 && !inReach(u.X*engine.ArmyCellPixels, u.Y*engine.ArmyCellPixels) {
				continue
			}
			share := units[u.Type]
			if share == nil {
				share = &UnitShare{Type: u.Type}
				units[u.Type] = share
			}
			share.Count++
			share.Value += CostOf(u.Type).Value()
			f.Value += CostOf(u.Type).Value()
			f.Count++
		}
		if f.Value > best.Value {
			best, bestUnits = f, units
		}
	}
	for _, s := range bestUnits {
		best.Units = append(best.Units, *s)
	}
	slices.SortFunc(best.Units, func(a, b UnitShare) int {
		if a.Value != b.Value {
			return int(b.Value - a.Value)
		}
		return int(a.Type) - int(b.Type)
	})
	best.describe()
	for _, other := range events {
		if other.Type == "recall" && other.Actor == e.Actor && other.Second <= e.Second && other.Second >= e.Second-whyLookBack {
			best.Recalls++
		}
	}
	return best
}

// ofForce is a unit the event's force is made of: one of the units a drop set down, else the actor's army.
// Transports carry a force but are not what it fights with.
func (g *game) ofForce(e Event, u engine.ArmyUnit) bool {
	if transports[u.Type] {
		return false
	}
	if len(e.Members) > 0 {
		return slices.Contains(e.Members, u.ID)
	}
	return u.Owner == e.Actor && mobileArmy(u.Type)
}

func (f *Force) describe() {
	if f.Value <= 0 {
		return
	}
	pct := func(v int32) int32 { return 100 * v / f.Value }
	if pct(f.Units[0].Value) >= forceMostlyType {
		f.Mostly = f.Units[0].Type
		return
	}
	classes := map[string]int32{}
	for _, s := range f.Units {
		if c := unitClass[s.Type]; c != "" {
			classes[c] += s.Value
		}
	}
	for _, c := range []string{ClassAir, ClassBio, ClassMech} {
		if pct(classes[c]) >= forceMostlyKind {
			f.Class = c
			return
		}
	}
	if len(f.Units) >= 2 && pct(f.Units[0].Value+f.Units[1].Value) >= forceMostlyPair {
		f.MostlyPair = [2]uint16{f.Units[0].Type, f.Units[1].Type}
	}
}
