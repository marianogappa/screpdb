// Package macro scores each player's macro from an engine digest: supply and resource efficiency. It
// reads only the digest, so a change here needs no new engine passes.
package macro

import (
	"math"
	"slices"

	"github.com/marianogappa/openbw-go/bwgame"
	"github.com/marianogappa/screpdb/internal/engine"
)

const (
	framesPerSecond = engine.EconomyEveryFrames
	// Openings are scripted and income is tiny before 2:00; including them would score build orders.
	steadyFromSecond = 120
	// A game cut short before 3:00 of trusted data says nothing about macro.
	minSeconds = 180
	// Past 60 workers a town hall standing idle is a choice, not a miss.
	workerTarget = 60
	maxSupply    = 400
	// Supply values are doubled: a free supply below 2 cannot make even one worker.
	oneSupply      = 2
	minBlockLength = 2
	larvaPerHatch  = 3
	secondsPerMin  = 60
	raceZerg       = 0
)

type Player struct {
	Slot    int8
	Race    uint8
	Seconds int

	SupplyBlockedSeconds int
	SupplyBlocks         int
	// BlocksPer100Supply and the other ratios are NaN when the game gives no basis for them.
	BlocksPer100Supply float64
	SpareSupply        float64

	SpendingQuotient float64
	IncomePerMinute  float64
	AverageUnspent   float64

	WorkerIdleShare     float64
	ProductionIdleShare float64

	// Income is minerals plus gas gathered in each full minute.
	Income []int
}

func Compute(d *engine.Digest) []Player {
	ends := playerEnds(d)
	var out []Player
	for _, pl := range d.Players {
		end := ends[pl.Slot]
		samples := economyOf(d, pl.Slot, end)
		if len(samples) < minSeconds {
			continue
		}
		p := Player{Slot: pl.Slot, Race: pl.Race, Seconds: len(samples)}
		var timeline facilityTimeline
		if pl.Race == raceZerg {
			timeline = zergTimeline(d, pl.Slot, samples)
		} else {
			timeline = buildingTimeline(d, pl.Slot, len(samples))
		}
		scoreSupply(&p, samples, timeline)
		scoreResources(&p, samples)
		p.WorkerIdleShare, p.ProductionIdleShare = timeline.workerIdle, timeline.productionIdle
		out = append(out, p)
	}
	return out
}

// playerEnds is each player's last trusted second: the earliest of the game end, the trusted-until
// frame of a desynced game, and the player leaving or being defeated.
func playerEnds(d *engine.Digest) map[int8]int {
	trusted := min(d.TrustedUntil(), d.LastFrame+1)
	ends := map[int8]int{}
	for _, p := range d.Players {
		ends[p.Slot] = int(trusted) / framesPerSecond
	}
	for _, o := range d.Outcomes {
		if o.Kind == engine.OutcomeWon {
			continue
		}
		if s, ok := ends[o.Owner]; ok {
			ends[o.Owner] = min(s, int(o.Frame)/framesPerSecond)
		}
	}
	return ends
}

func economyOf(d *engine.Digest, slot int8, endSecond int) []engine.EconomySample {
	var out []engine.EconomySample
	for _, s := range d.Economy {
		if s.Owner == slot && int(s.Frame)/framesPerSecond < endSecond {
			out = append(out, s)
		}
	}
	return out
}

// facilityTimeline says, per second, whether the player could have started production right then.
type facilityTimeline struct {
	productionFree []bool
	workerIdle     float64
	productionIdle float64
}

func scoreSupply(p *Player, samples []engine.EconomySample, t facilityTimeline) {
	run := 0
	peak := int32(0)
	closeRun := func() {
		if run >= minBlockLength {
			p.SupplyBlocks++
		}
		run = 0
	}
	var spare, spareSeconds float64
	for i, s := range samples {
		peak = max(peak, s.SupplyUsed)
		blocked := s.SupplyMax < maxSupply && s.SupplyMax-s.SupplyUsed < oneSupply && t.productionFree[i]
		if blocked {
			p.SupplyBlockedSeconds++
			run++
		} else {
			closeRun()
		}
		if i >= steadyFromSecond && s.SupplyMax < maxSupply {
			spare += float64(s.SupplyMax-s.SupplyUsed) / 2
			spareSeconds++
		}
	}
	closeRun()
	p.BlocksPer100Supply = math.NaN()
	if peakSupply := float64(peak) / 2; peakSupply >= 20 {
		p.BlocksPer100Supply = float64(p.SupplyBlocks) / (peakSupply / 100)
	}
	p.SpareSupply = math.NaN()
	if spareSeconds > 0 {
		p.SpareSupply = spare / spareSeconds
	}
}

// scoreResources ports StarCraft II's spending quotient: 35 × (0.00137 × income per minute − ln(average
// unspent)) + 240. The constants are SC2's; the pro ranges, not the formula, say what a BW value means.
func scoreResources(p *Player, samples []engine.EconomySample) {
	gathered := func(s engine.EconomySample) int { return int(s.GatheredMinerals + s.GatheredGas) }
	for m := 1; m*secondsPerMin < len(samples); m++ {
		p.Income = append(p.Income, gathered(samples[m*secondsPerMin])-gathered(samples[(m-1)*secondsPerMin]))
	}
	steady := samples[min(steadyFromSecond, len(samples)-1):]
	minutes := float64(len(steady)) / secondsPerMin
	var unspent float64
	for _, s := range steady {
		unspent += float64(s.Minerals + s.Gas)
	}
	p.AverageUnspent = unspent / float64(len(steady))
	p.IncomePerMinute = float64(gathered(steady[len(steady)-1])-gathered(steady[0])) / minutes
	p.SpendingQuotient = 35*(0.00137*p.IncomePerMinute-math.Log(max(p.AverageUnspent, 1))) + 240
}

var (
	townHalls = []uint16{
		uint16(bwgame.UnitTypesTerranCommandCenter), uint16(bwgame.UnitTypesProtossNexus),
	}
	armyProduction = []uint16{
		uint16(bwgame.UnitTypesTerranBarracks), uint16(bwgame.UnitTypesTerranFactory), uint16(bwgame.UnitTypesTerranStarport),
		uint16(bwgame.UnitTypesProtossGateway), uint16(bwgame.UnitTypesProtossRoboticsFacility), uint16(bwgame.UnitTypesProtossStargate),
	}
	hatcheries = []uint16{uint16(bwgame.UnitTypesZergHatchery)}
)

type interval struct{ from, to int }

type building struct {
	unit     uint32
	army     bool
	alive    interval
	busy     []interval
	idleSecs []bool
}

func second(frame int32) int { return int(frame) / framesPerSecond }

// buildingTimeline tracks every Terran or Protoss production building from completion to death and
// marks the seconds it trained, researched or built an add-on.
func buildingTimeline(d *engine.Digest, slot int8, seconds int) facilityTimeline {
	deaths := deathSeconds(d)
	byUnit := map[uint32]*building{}
	add := func(unit uint32, t uint16, from int) {
		army := slices.Contains(armyProduction, t)
		if !army && !slices.Contains(townHalls, t) {
			return
		}
		to := seconds
		if dead, ok := deaths[unit]; ok {
			to = min(to, dead)
		}
		byUnit[unit] = &building{unit: unit, army: army, alive: interval{from, to}}
	}
	for _, u := range d.Initial {
		if u.Owner == slot {
			add(u.Unit, u.Type, 0)
		}
	}
	for _, pr := range d.Productions {
		if pr.Owner == slot && pr.Complete != engine.NoFrame {
			add(pr.Unit, pr.Type, second(pr.Complete))
		}
	}
	busy := func(producer uint32, start, done, cancel int32) {
		b, ok := byUnit[producer]
		if !ok {
			return
		}
		end := seconds
		for _, f := range []int32{done, cancel} {
			if f != engine.NoFrame {
				end = min(end, second(f))
			}
		}
		b.busy = append(b.busy, interval{second(start), end})
	}
	workers := map[int]bool{}
	for _, pr := range d.Productions {
		if pr.Owner == slot {
			busy(pr.Producer, pr.Start, pr.Complete, pr.Cancel)
		}
	}
	for _, r := range d.Research {
		if r.Owner == slot {
			busy(r.Producer, r.Start, r.Complete, r.Cancel)
		}
	}
	samples := economyOf(d, slot, seconds)
	for i, s := range samples {
		workers[i] = s.Workers < workerTarget
	}

	t := facilityTimeline{productionFree: make([]bool, seconds)}
	var hallIdle, hallSeconds, armyIdle, armySeconds float64
	for i := range seconds {
		for _, b := range byUnit {
			if i < b.alive.from || i >= b.alive.to {
				continue
			}
			idle := !b.busyAt(i)
			if idle {
				t.productionFree[i] = true
			}
			switch {
			case b.army:
				armySeconds++
				if idle {
					armyIdle++
				}
			case workers[i]:
				hallSeconds++
				if idle {
					hallIdle++
				}
			}
		}
	}
	t.workerIdle, t.productionIdle = share(hallIdle, hallSeconds), share(armyIdle, armySeconds)
	return t
}

func (b *building) busyAt(s int) bool {
	for _, iv := range b.busy {
		if s >= iv.from && s < iv.to {
			return true
		}
	}
	return false
}

// zergTimeline uses larva: production is free whenever a larva waits, and idle when every hatchery sits
// at its 3-larva cap, so no larva is being made at all.
func zergTimeline(d *engine.Digest, slot int8, samples []engine.EconomySample) facilityTimeline {
	deaths := deathSeconds(d)
	var halls []interval
	addHall := func(unit uint32, from int) {
		to := len(samples)
		if dead, ok := deaths[unit]; ok {
			to = min(to, dead)
		}
		halls = append(halls, interval{from, to})
	}
	for _, u := range d.Initial {
		if u.Owner == slot && slices.Contains(hatcheries, u.Type) {
			addHall(u.Unit, 0)
		}
	}
	for _, pr := range d.Productions {
		if pr.Owner == slot && slices.Contains(hatcheries, pr.Type) && pr.Complete != engine.NoFrame {
			addHall(pr.Unit, second(pr.Complete))
		}
	}
	t := facilityTimeline{productionFree: make([]bool, len(samples)), workerIdle: math.NaN()}
	var capped, counted float64
	for i, s := range samples {
		t.productionFree[i] = s.Larva > 0
		n := 0
		for _, h := range halls {
			if i >= h.from && i < h.to {
				n++
			}
		}
		if n == 0 {
			continue
		}
		counted++
		if int(s.Larva) >= larvaPerHatch*n {
			capped++
		}
	}
	t.productionIdle = share(capped, counted)
	return t
}

func deathSeconds(d *engine.Digest) map[uint32]int {
	out := map[uint32]int{}
	for _, x := range d.Deaths {
		if _, seen := out[x.Unit]; !seen {
			out[x.Unit] = second(x.Frame)
		}
	}
	return out
}

func share(part, whole float64) float64 {
	if whole == 0 {
		return math.NaN()
	}
	return part / whole
}
