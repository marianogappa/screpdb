package moments

import "slices"

// A Combination folds a part event into a lead event when both tell one happening: the Lurkers a drop set down
// are the ones that then kill undetected. Only the listed pairs combine; an Eraser or an Overlord hunt is a
// story of its own. Eligible must prove the link from the units both events share, not from time and place
// alone, so a combination needs digests that name units (engine.Digest.UnitIDsKnown).
type Combination struct {
	Lead, Part []string
	Eligible   func(lead, part Event) bool
}

var dropTypes = []string{"drop", "reaver_drop", "dt_drop", "ht_drop", "lurker_drop"}

// Combinations is applied in order, so a drop folds into its elevator before kills fold into either.
var Combinations = []Combination{
	{Lead: []string{InsightElevator}, Part: dropTypes, Eligible: sameDelivery},
	{Lead: append([]string{InsightElevator}, dropTypes...), Part: []string{InsightUndetectedCloak}, Eligible: killedByDelivery},
	{Lead: []string{InsightLurkerContain}, Part: []string{InsightUndetectedCloak}, Eligible: killedByContain},
}

// A part may start this long before its lead: kills begin as the first transport unloads, and a drop is dated
// by its first unload.
const combineLeadIn = 10

func sameSides(lead, part Event) bool { return lead.Actor == part.Actor && lead.Target == part.Target }

// mostlyFrom says at least half of units are among from.
func mostlyFrom(units, from []uint32) bool {
	n := 0
	for _, u := range units {
		if slices.Contains(from, u) {
			n++
		}
	}
	return len(units) > 0 && 2*n >= len(units)
}

func startsDuring(lead, part Event, after int) bool {
	return part.Second >= lead.Second-combineLeadIn && part.Second <= lead.Second+lead.Seconds+after
}

func sameDelivery(lead, part Event) bool {
	return sameSides(lead, part) && startsDuring(lead, part, elevatorWindow) && mostlyFrom(part.Members, lead.Members)
}

func killedByDelivery(lead, part Event) bool {
	return sameSides(lead, part) && startsDuring(lead, part, fightFollowUp) && mostlyFrom(part.Members, lead.Members)
}

func killedByContain(lead, part Event) bool {
	return sameSides(lead, part) && startsDuring(lead, part, containMinSeconds/2) && mostlyFrom(part.Members, lead.Members)
}

// Combine folds each part event into the eligible lead sharing most of its units, oldest lead first on a
// tie. why indexes events, -1 for none; the returned index follows a why that was folded into its lead.
func Combine(events []Event, why int) ([]Event, int) {
	out := slices.Clone(events)
	for _, c := range Combinations {
		for p := 0; p < len(out); p++ {
			if !slices.Contains(c.Part, out[p].Type) {
				continue
			}
			lead, best := -1, 0
			for l := range out {
				if l == p || !slices.Contains(c.Lead, out[l].Type) || !c.Eligible(out[l], out[p]) {
					continue
				}
				if n := shared(out[l].Members, out[p].Members); n > best {
					lead, best = l, n
				}
			}
			if lead < 0 {
				continue
			}
			out[lead] = fold(out[lead], out[p])
			switch {
			case why == p:
				why = lead
			case why > p:
				why--
			}
			if lead > p {
				lead--
			}
			out = slices.Delete(out, p, p+1)
			p--
		}
	}
	return out, why
}

func shared(a, b []uint32) int {
	n := 0
	for _, u := range b {
		if slices.Contains(a, u) {
			n++
		}
	}
	return n
}

// fold tells part as a detail of lead: the lead keeps its place and moment, its span and frame grow to take
// in the part, and the outcome counts whichever of the two saw more, since both count the same deaths.
func fold(lead, part Event) Event {
	end := max(lead.Second+lead.Seconds, part.Second+part.Seconds)
	lead.Seconds = end - lead.Second
	lead.Region = unionRegion(lead.Region, part.Region)
	o, q := &lead.Outcome, part.Outcome
	o.Killed, o.Lost = max(o.Killed, q.Killed), max(o.Lost, q.Lost)
	o.Workers, o.Buildings, o.KilledUnits = max(o.Workers, q.Workers), max(o.Buildings, q.Buildings), max(o.KilledUnits, q.KilledUnits)
	o.TownHall = o.TownHall || q.TownHall
	lead.Insight = lead.Insight || part.Insight
	for _, u := range part.Members {
		if !slices.Contains(lead.Members, u) {
			lead.Members = append(lead.Members, u)
		}
	}
	part.Members = nil
	lead.Parts = append(lead.Parts, part)
	return lead
}

func unionRegion(a, b Region) Region {
	if a.W == 0 || a.H == 0 {
		return b
	}
	if b.W == 0 || b.H == 0 {
		return a
	}
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return Region{X: x, Y: y, W: max(a.X+a.W, b.X+b.W) - x, H: max(a.Y+a.H, b.Y+b.H) - y}
}

// pushGap is how long after a hit ends the next may start and still be the same push.
const pushGap = 120

var pushTypes = []string{"attack", "worker_kills"}

// ChainPushes folds the hits one army keeps making on one base into the first of them: Hydralisks killing
// workers in a main, leaving and coming back a minute later, are one push, not two events that differ only
// in their counts. A push into another base stays its own event, with its own map. Hits chain while each
// starts within pushGap of the last one ending and shares a unit type with the push. Each hit counts its own
// deaths, so the push adds them up.
func ChainPushes(events []Event) []Event {
	order := make([]int, len(events))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return events[a].Second - events[b].Second })
	folded := make([]bool, len(events))
	out := slices.Clone(events)
	for oi, l := range order {
		if folded[l] || !slices.Contains(pushTypes, out[l].Type) {
			continue
		}
		for _, p := range order[oi+1:] {
			lead, part := out[l], out[p]
			end := lead.Second + lead.Seconds
			for _, q := range lead.Parts {
				end = max(end, q.Second+q.Seconds)
			}
			if part.Second > end+pushGap {
				break
			}
			if folded[p] || !slices.Contains(pushTypes, part.Type) || !sameSides(lead, part) || lead.Base != part.Base || !sharesUnit(lead.Units, part.Units) {
				continue
			}
			out[l] = push(lead, part)
			folded[p] = true
		}
	}
	var kept []Event
	for i, e := range out {
		if !folded[i] {
			kept = append(kept, e)
		}
	}
	return kept
}

func sharesUnit(a, b []uint16) bool {
	return slices.ContainsFunc(b, func(t uint16) bool { return slices.Contains(a, t) })
}

func push(lead, part Event) Event {
	lead.Seconds = max(lead.Seconds, part.Second+part.Seconds-lead.Second)
	lead.Region = unionRegion(lead.Region, part.Region)
	o, q := &lead.Outcome, part.Outcome
	o.Killed, o.Lost = o.Killed+q.Killed, o.Lost+q.Lost
	o.Workers, o.Buildings, o.KilledUnits = o.Workers+q.Workers, o.Buildings+q.Buildings, o.KilledUnits+q.KilledUnits
	o.TownHall = o.TownHall || q.TownHall
	for _, t := range part.Units {
		if !slices.Contains(lead.Units, t) {
			lead.Units = append(lead.Units, t)
		}
	}
	lead.Parts = append(lead.Parts, part)
	return lead
}
