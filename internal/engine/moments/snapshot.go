package moments

import (
	"slices"

	"github.com/marianogappa/screpdb/internal/engine"
)

type Building struct {
	Type     uint16
	Owner    int8
	X, Y     int32
	Complete bool
}

type Unit struct {
	Type  uint16
	Owner int8
	X, Y  int32
}

// Snapshot is the map at one second: every building standing, exact, and the army as of the last army
// sample, which the digest keeps every 10 s.
type Snapshot struct {
	Second     int
	ArmySecond int
	Buildings  []Building
	Units      []Unit
}

// SnapshotAt replays each building's productions up to sec: a new construction is incomplete until it
// finishes and gone if cancelled; a morph (Hatchery to Lair, Creep to Sunken) shows the old building,
// complete, until it finishes, and a cancelled morph leaves the old building standing.
func SnapshotAt(d *engine.Digest, sec int) Snapshot {
	snap := Snapshot{Second: sec, ArmySecond: -1}
	deaths := map[uint32][]int{}
	for _, x := range d.Deaths {
		deaths[x.Unit] = append(deaths[x.Unit], second(x.Frame))
	}
	state := map[uint32]*Building{}
	since := map[uint32]int{}
	var order []uint32
	track := func(unit uint32, b *Building) {
		if _, seen := state[unit]; !seen {
			order = append(order, unit)
		}
		state[unit] = b
	}
	for _, u := range d.Initial {
		if _, ok := FootprintOf(u.Type); ok {
			track(u.Unit, &Building{Type: u.Type, Owner: u.Owner, X: u.X, Y: u.Y, Complete: true})
		}
	}
	for _, pr := range d.Productions {
		if _, ok := FootprintOf(pr.Type); !ok || second(pr.Start) > sec {
			continue
		}
		prev := state[pr.Unit]
		if prev != nil && slices.ContainsFunc(deaths[pr.Unit], func(dead int) bool { return dead >= since[pr.Unit] && dead <= second(pr.Start) }) {
			prev = nil
		}
		morph := prev != nil && prev.Complete
		cancelled := pr.Cancel != engine.NoFrame && second(pr.Cancel) <= sec
		done := pr.Complete != engine.NoFrame && second(pr.Complete) <= sec
		if !morph && !cancelled {
			since[pr.Unit] = second(pr.Start)
		}
		switch {
		case cancelled && morph:
		case cancelled:
			track(pr.Unit, nil)
		case done:
			x, y := pr.X, pr.Y
			if prev != nil {
				x, y = prev.X, prev.Y
			}
			track(pr.Unit, &Building{Type: pr.Type, Owner: pr.Owner, X: x, Y: y, Complete: true})
		case morph:
		default:
			track(pr.Unit, &Building{Type: pr.Type, Owner: pr.Owner, X: pr.X, Y: pr.Y})
		}
	}
	for _, unit := range order {
		b := state[unit]
		if b == nil {
			continue
		}
		if slices.ContainsFunc(deaths[unit], func(dead int) bool { return dead >= since[unit] && dead <= sec }) {
			continue
		}
		snap.Buildings = append(snap.Buildings, *b)
	}
	for _, a := range d.Army {
		if second(a.Frame) > sec {
			break
		}
		snap.ArmySecond = second(a.Frame)
		snap.Units = snap.Units[:0]
		for _, u := range a.Units {
			snap.Units = append(snap.Units, Unit{Type: u.Type, Owner: u.Owner, X: u.X*engine.ArmyCellPixels + engine.ArmyCellPixels/2, Y: u.Y*engine.ArmyCellPixels + engine.ArmyCellPixels/2})
		}
	}
	return snap
}
