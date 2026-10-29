package main

import (
	"fmt"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
	"github.com/marianogappa/screpdb/internal/engine/macro"
)

// windowedUnspent measures unspent resources against income up to fixed game seconds, so long games, where
// banks pile up, do not read as worse spending than short ones.
func windowedUnspent(d *engine.Digest) map[int8]map[string]float64 {
	out := map[int8]map[string]float64{}
	for _, w := range []int{600, 720, 900} {
		for _, m := range macro.Compute(firstSeconds(d, w)) {
			if m.IncomePerMinute <= 0 {
				continue
			}
			if out[m.Slot] == nil {
				out[m.Slot] = map[string]float64{}
			}
			out[m.Slot][fmt.Sprintf("w%d_unspent_per_income", w)] = m.AverageUnspent / m.IncomePerMinute
			out[m.Slot][fmt.Sprintf("w%d_seconds", w)] = float64(m.Seconds)
		}
	}
	return out
}

const framesPerSecond = engine.EconomyEveryFrames

var hatcheryTypes = map[uint16]bool{
	uint16(bwgame.UnitTypesZergHatchery): true, uint16(bwgame.UnitTypesZergLair): true, uint16(bwgame.UnitTypesZergHive): true,
}

// candidateValues measures alternatives to the Skill tab's macro metrics, and what kind of game a player had,
// over the player's trusted seconds.
func candidateValues(d *engine.Digest, slot int8, endSecond int, zerg bool) map[string]float64 {
	var samples []engine.EconomySample
	for _, s := range d.Economy {
		if s.Owner == slot && int(s.Frame)/framesPerSecond < endSecond {
			samples = append(samples, s)
		}
	}
	if len(samples) < 180 {
		return nil
	}
	out := map[string]float64{"tracked_seconds": float64(len(samples))}
	var maxed, peak float64
	for _, s := range samples {
		peak = max(peak, float64(s.SupplyUsed)/2)
		if s.SupplyUsed >= 2*190 {
			maxed++
		}
	}
	out["peak_supply"], out["maxed_share"] = peak, maxed/float64(len(samples))
	if !zerg {
		return out
	}
	deaths := map[uint32]int{}
	for _, x := range d.Deaths {
		if _, seen := deaths[x.Unit]; !seen {
			deaths[x.Unit] = int(x.Frame) / framesPerSecond
		}
	}
	type life struct{ from, to int }
	halls := map[uint32]life{}
	add := func(unit uint32, from int) {
		if _, ok := halls[unit]; ok {
			return
		}
		to := len(samples)
		if dead, ok := deaths[unit]; ok {
			to = min(to, dead)
		}
		halls[unit] = life{from, to}
	}
	for _, u := range d.Initial {
		if u.Owner == slot && hatcheryTypes[u.Type] {
			add(u.Unit, 0)
		}
	}
	for _, pr := range d.Productions {
		if pr.Owner == slot && hatcheryTypes[pr.Type] && pr.Complete != engine.NoFrame {
			add(pr.Unit, int(pr.Complete)/framesPerSecond)
		}
	}
	type acc struct{ capped, fill, waiting, lb, ub, n float64 }
	var all, steady acc
	for i, s := range samples {
		n := 0
		for _, h := range halls {
			if i >= h.from && i < h.to {
				n++
			}
		}
		if n == 0 {
			continue
		}
		l, fn := float64(s.Larva), float64(n)
		x := acc{n: 1}
		if l >= 3*fn {
			x.capped = 1
		}
		x.fill = min(1, l/(3*fn))
		x.waiting = min(l, fn) / fn
		x.lb = max(0, l-2*fn) / fn
		x.ub = min(fn, float64(int(l)/3)) / fn
		for _, a := range []*acc{&all, &steady} {
			if a == &steady && i < 120 {
				continue
			}
			a.capped += x.capped
			a.fill += x.fill
			a.waiting += x.waiting
			a.lb += x.lb
			a.ub += x.ub
			a.n++
		}
	}
	put := func(prefix string, a acc) {
		if a.n == 0 {
			return
		}
		out[prefix+"capped"] = a.capped / a.n
		out[prefix+"fill"] = a.fill / a.n
		out[prefix+"waiting"] = a.waiting / a.n
		out[prefix+"lb"] = a.lb / a.n
		out[prefix+"ub"] = a.ub / a.n
	}
	put("zi_", all)
	put("zi2_", steady)
	return out
}
