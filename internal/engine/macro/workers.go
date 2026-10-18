package macro

import (
	"slices"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// Worker production is measured from 2:00 to 10:00, when every player is building an economy and before
// a game's outcome decides who can still make workers.
const (
	workerWindowFrom = 120 * framesPerSecond
	workerWindowTo   = 600 * framesPerSecond
)

var townHallTypes = map[uint16]bool{
	uint16(bwgame.UnitTypesTerranCommandCenter): true, uint16(bwgame.UnitTypesProtossNexus): true,
	uint16(bwgame.UnitTypesZergHatchery): true, uint16(bwgame.UnitTypesZergLair): true, uint16(bwgame.UnitTypesZergHive): true,
}

// WorkerProduction is how steadily a player made workers between 2:00 and 10:00. Terran and Protoss get
// HallBusy, the share of town hall time spent training workers; Zerg get LarvaPerHatchery, the larva
// left waiting per hatchery on average (lower is better). ok is false when the window is not covered.
type WorkerProduction struct {
	HallBusy         float64
	LarvaPerHatchery float64
	Zerg             bool
}

func WorkerProductionOf(d *engine.Digest, slot int8) (WorkerProduction, bool) {
	end := min(int32(workerWindowTo), d.TrustedUntil())
	if end <= workerWindowFrom {
		return WorkerProduction{}, false
	}
	halls := map[uint32]int32{}
	for _, u := range d.Initial {
		if u.Owner == slot && townHallTypes[u.Type] {
			halls[u.Unit] = 0
		}
	}
	for _, pr := range d.Productions {
		if pr.Owner == slot && townHallTypes[pr.Type] && pr.Complete != engine.NoFrame {
			if _, ok := halls[pr.Unit]; !ok {
				halls[pr.Unit] = pr.Complete
			}
		}
	}
	var out WorkerProduction
	for _, p := range d.Players {
		if p.Slot == slot && p.Race == raceZerg {
			out.Zerg = true
		}
	}
	overlap := func(a, b int32) float64 { return float64(max(0, min(b, end)-max(a, workerWindowFrom))) }
	if !out.Zerg {
		var alive, busy float64
		for id, ready := range halls {
			alive += overlap(ready, end)
			for _, pr := range d.Productions {
				if pr.Producer != id || !slices.Contains(workerTypes, pr.Type) {
					continue
				}
				stop := pr.Complete
				if stop == engine.NoFrame {
					stop = pr.Cancel
				}
				if stop == engine.NoFrame {
					stop = end
				}
				busy += overlap(pr.Start, stop)
			}
		}
		if alive == 0 {
			return WorkerProduction{}, false
		}
		out.HallBusy = busy / alive
		return out, true
	}
	var larva, n float64
	for _, e := range d.Economy {
		if e.Owner != slot || e.Frame < workerWindowFrom || e.Frame >= end {
			continue
		}
		count := 0
		for _, ready := range halls {
			if ready <= e.Frame {
				count++
			}
		}
		if count > 0 {
			larva += float64(e.Larva) / float64(count)
			n++
		}
	}
	if n == 0 {
		return WorkerProduction{}, false
	}
	out.LarvaPerHatchery = larva / n
	return out, true
}
