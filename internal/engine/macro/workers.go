package macro

import (
	"slices"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

// Worker production is measured from 2:00, once making workers stops being automatic (the first two
// minutes every player trains from one base, and Zerg larva is the limit), until a game's outcome
// starts deciding who can still make workers: 10:00, or 8:00 on money maps, where economies saturate early.
const (
	workerWindowFrom        = 120 * framesPerSecond
	WorkerWindowEnd         = 600
	MoneyMapWorkerWindowEnd = 480
	MoneyMapBankedWindowEnd = 1200
)

var baseTypes = map[uint16]bool{
	uint16(bwgame.UnitTypesTerranCommandCenter): true, uint16(bwgame.UnitTypesProtossNexus): true,
	uint16(bwgame.UnitTypesZergHatchery): true, uint16(bwgame.UnitTypesZergLair): true, uint16(bwgame.UnitTypesZergHive): true,
}

// WorkerProduction is how steadily a player made workers between 2:00 and 10:00, counting each Command
// Center, Nexus or Hatchery (Lair, Hive) from completion until it dies. Terran and Protoss get HallBusy, the
// share of that time spent training workers; Zerg get LarvaPerHatchery, the larva left waiting per hatchery
// on average (lower is better). ok is false when the window is not covered.
type WorkerProduction struct {
	HallBusy         float64
	LarvaPerHatchery float64
	Zerg             bool
}

type lifetime struct{ ready, gone int32 }

// WorkerProductionOf measures worker production from 2:00 to untilSecond.
func WorkerProductionOf(d *engine.Digest, slot int8, untilSecond int) (WorkerProduction, bool) {
	end := min(int32(untilSecond*framesPerSecond), d.TrustedUntil())
	if end <= workerWindowFrom {
		return WorkerProduction{}, false
	}
	died := map[uint32]int32{}
	for _, x := range d.Deaths {
		if _, seen := died[x.Unit]; !seen {
			died[x.Unit] = x.Frame
		}
	}
	halls := map[uint32]lifetime{}
	addBase := func(unit uint32, ready int32) {
		if _, ok := halls[unit]; ok {
			return
		}
		gone := end
		if f, ok := died[unit]; ok {
			gone = min(gone, f)
		}
		halls[unit] = lifetime{ready, gone}
	}
	for _, u := range d.Initial {
		if u.Owner == slot && baseTypes[u.Type] {
			addBase(u.Unit, 0)
		}
	}
	for _, pr := range d.Productions {
		if pr.Owner == slot && baseTypes[pr.Type] && pr.Complete != engine.NoFrame {
			addBase(pr.Unit, pr.Complete)
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
		for id, life := range halls {
			alive += overlap(life.ready, life.gone)
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
				busy += overlap(pr.Start, min(stop, life.gone))
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
		for _, life := range halls {
			if life.ready <= e.Frame && e.Frame < life.gone {
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

// FirstSeconds is the digest cut at a game second, so the scorecard measures that opening window only.
func FirstSeconds(d *engine.Digest, seconds int) *engine.Digest {
	cut := *d
	cut.LastFrame = min(d.LastFrame, int32(seconds*framesPerSecond-1))
	return &cut
}
