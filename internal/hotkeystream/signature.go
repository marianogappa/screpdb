package hotkeystream

import (
	"fmt"
	"sort"
	"strings"
)

// Signature aggregation: a player's hotkey pattern is a schedule, not a static
// layout — the same key reliably holds different things in different game
// phases. Each key becomes a sequence of runs over game minutes, where a run
// is a stretch in which presses of that key kept selecting the same category
// of content. Categories: hall / prod / tech / comsat / units.

// signatureMaxMinute caps ribbons: games rarely carry signal past this point.
const signatureMaxMinute = 24

const (
	CategoryHall   = "hall"
	CategoryProd   = "prod"
	CategoryTech   = "tech"
	CategoryComsat = "comsat"
	CategoryUnits  = "units"
)

// CategoryOf maps a building name to its signature category.
func CategoryOf(building string) string {
	switch building {
	case "Hatchery", "Lair", "Hive", "Command Center", "Nexus":
		return CategoryHall
	case "Comsat Station":
		return CategoryComsat
	case "Gateway", "Barracks", "Factory", "Starport", "Stargate", "Robotics Facility":
		return CategoryProd
	}
	return CategoryTech
}

// KeyRun is one stretch of minutes in which a key's presses selected the same
// category of content, aggregated across games.
type KeyRun struct {
	StartMin int     `json:"start_min"`
	EndMin   int     `json:"end_min"`
	Category string  `json:"category"`
	Share    float64 `json:"share"`
	Presses  int     `json:"presses"`
}

// KeySignature is one hotkey group's aggregated behaviour.
type KeySignature struct {
	Key       int      `json:"key"`
	Runs      []KeyRun `json:"runs"`
	Uses      int      `json:"uses"`
	UsedGames int      `json:"used_games"`
	// MedianCount is the median selection size of unit assigns on this key.
	MedianCount int `json:"median_count"`
}

// Signature is a player's aggregated hotkey pattern for one race.
type Signature struct {
	Race  string         `json:"race"`
	Games int            `json:"games"`
	Keys  []KeySignature `json:"keys"`
	// TemporalScore is the share of key presses explained by the per-minute
	// modal content of their key.
	TemporalScore float64 `json:"temporal_score"`
	Prose         string  `json:"prose"`
}

// ComputeSignature aggregates one player's games (all the same race) into a
// per-key temporal signature. Race is the full race name (Zerg/Terran/Protoss).
func ComputeSignature(playerName, race string, games [][]Event) *Signature {
	sig := &Signature{Race: race, Games: len(games)}
	if len(games) == 0 {
		return sig
	}

	type minuteAgg struct {
		counts map[string]int
		games  int
	}
	// perKeyMin[key][minute]
	perKeyMin := map[int]map[int]*minuteAgg{}
	usedGames := map[int]int{}
	uses := map[int]int{}
	unitCounts := map[int][]int{}

	for _, events := range games {
		// Decoded streams are time-ordered already; sorting keeps the
		// "current content" replay correct for any caller.
		events = append([]Event(nil), events...)
		sort.SliceStable(events, func(i, j int) bool { return events[i].Sec < events[j].Sec })
		curCat := map[int]string{}
		gameMin := map[int]map[int]map[string]int{}
		assigned := map[int]bool{}
		for _, e := range events {
			if e.Group > 9 {
				continue
			}
			k := int(e.Group)
			switch e.Type {
			case TypeAssignUnits:
				curCat[k] = CategoryUnits
				assigned[k] = true
				if e.Count > 0 {
					unitCounts[k] = append(unitCounts[k], int(e.Count))
				}
			case TypeAssignBuilding:
				curCat[k] = CategoryOf(BuildingName(e.Building))
				assigned[k] = true
			default:
				uses[k]++
				cat := curCat[k]
				if cat == "" {
					continue
				}
				m := int(e.Sec) / 60
				if m > signatureMaxMinute {
					continue
				}
				if gameMin[k] == nil {
					gameMin[k] = map[int]map[string]int{}
				}
				if gameMin[k][m] == nil {
					gameMin[k][m] = map[string]int{}
				}
				gameMin[k][m][cat]++
			}
		}
		for k := range assigned {
			usedGames[k]++
		}
		for k, mins := range gameMin {
			if perKeyMin[k] == nil {
				perKeyMin[k] = map[int]*minuteAgg{}
			}
			for m, counts := range mins {
				agg := perKeyMin[k][m]
				if agg == nil {
					agg = &minuteAgg{counts: map[string]int{}}
					perKeyMin[k][m] = agg
				}
				agg.games++
				for c, n := range counts {
					agg.counts[c] += n
				}
			}
		}
	}

	minGames := max(2, (len(games)*15+99)/100)
	weighted, total := 0.0, 0.0
	for _, k := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 0} {
		if usedGames[k] < (len(games)*3+9)/10 && uses[k] < 50 {
			continue
		}
		type cell struct {
			cat   string
			share float64
			n     int
		}
		cells := make([]*cell, signatureMaxMinute+1)
		for m := 0; m <= signatureMaxMinute; m++ {
			agg := perKeyMin[k][m]
			if agg == nil || agg.games < minGames {
				continue
			}
			tot, best, bestN := 0, "", 0
			for c, n := range agg.counts {
				tot += n
				if n > bestN {
					best, bestN = c, n
				}
			}
			if tot < 3 {
				continue
			}
			share := float64(bestN) / float64(tot)
			cells[m] = &cell{cat: best, share: share, n: tot}
			weighted += share * float64(tot)
			total += float64(tot)
		}
		// smooth single-minute blips surrounded by the same category
		var idxs []int
		for m, c := range cells {
			if c != nil {
				idxs = append(idxs, m)
			}
		}
		for j := 1; j < len(idxs)-1; j++ {
			a, b, c := idxs[j-1], idxs[j], idxs[j+1]
			if cells[a].cat == cells[c].cat && cells[b].cat != cells[a].cat && b-a <= 2 && c-b <= 2 {
				cells[b].cat = cells[a].cat
			}
		}
		var runs []KeyRun
		for _, m := range idxs {
			c := cells[m]
			if len(runs) > 0 && runs[len(runs)-1].Category == c.cat {
				last := &runs[len(runs)-1]
				last.EndMin = m
				last.Share = (last.Share*float64(last.Presses) + c.share*float64(c.n)) / float64(last.Presses+c.n)
				last.Presses += c.n
			} else {
				runs = append(runs, KeyRun{StartMin: m, EndMin: m, Category: c.cat, Share: c.share, Presses: c.n})
			}
		}
		if len(runs) == 0 {
			continue
		}
		ks := KeySignature{Key: k, Runs: runs, Uses: uses[k], UsedGames: usedGames[k]}
		if counts := unitCounts[k]; len(counts) > 0 {
			sort.Ints(counts)
			ks.MedianCount = counts[len(counts)/2]
		}
		sig.Keys = append(sig.Keys, ks)
	}
	if total > 0 {
		sig.TemporalScore = weighted / total
	}
	sig.Prose = signatureProse(playerName, race, sig)
	return sig
}

var raceHall = map[string]string{"Zerg": "Hatchery", "Terran": "Command Center", "Protoss": "Nexus"}

var categoryNoun = map[string]string{
	CategoryHall: "town hall", CategoryProd: "production", CategoryTech: "tech",
	CategoryComsat: "Comsat", CategoryUnits: "army",
}

// signatureProse renders the signature as a short natural-language summary.
func signatureProse(playerName, race string, sig *Signature) string {
	var army []int
	bldg := map[string][]int{}
	type phasedKey struct {
		key  int
		runs []KeyRun
	}
	var phased []phasedKey
	for _, ks := range sig.Keys {
		cats := map[string]bool{}
		for _, r := range ks.Runs {
			cats[r.Category] = true
		}
		switch {
		case len(cats) == 1 && cats[CategoryUnits]:
			army = append(army, ks.Key)
		case len(cats) == 1:
			cat := ks.Runs[0].Category
			bldg[cat] = append(bldg[cat], ks.Key)
		default:
			phased = append(phased, phasedKey{key: ks.Key, runs: ks.Runs})
		}
	}

	var parts, frag []string
	if len(army) > 0 {
		frag = append(frag, fmt.Sprintf("armies on %s", keyRanges(army)))
	}
	for _, cat := range []string{CategoryHall, CategoryComsat, CategoryProd, CategoryTech} {
		keys, ok := bldg[cat]
		if !ok {
			continue
		}
		var noun string
		switch cat {
		case CategoryHall:
			noun = pluralize(raceHall[race], len(keys))
		case CategoryComsat:
			noun = pluralize("Comsat", len(keys))
		case CategoryProd:
			noun = "production"
		default:
			noun = "tech"
		}
		frag = append(frag, fmt.Sprintf("%s on %s", noun, keyRanges(keys)))
	}
	if len(frag) > 0 {
		style := "a stable layout"
		if len(phased) > 0 {
			style = "a phase-driven layout"
		}
		parts = append(parts, fmt.Sprintf("%s plays %s: %s.", playerName, style, strings.Join(frag, ", ")))
	}

	// Group repurposed keys sharing the same category sequence.
	type seqGroup struct {
		keys  []int
		items []phasedKey
	}
	groups := map[string]*seqGroup{}
	var order []string
	for _, pk := range phased {
		var cats []string
		for _, r := range pk.runs {
			cats = append(cats, r.Category)
		}
		sk := strings.Join(cats, ">")
		g := groups[sk]
		if g == nil {
			g = &seqGroup{}
			groups[sk] = g
			order = append(order, sk)
		}
		g.keys = append(g.keys, pk.key)
		g.items = append(g.items, pk)
	}
	sort.SliceStable(order, func(i, j int) bool { return len(groups[order[i]].keys) > len(groups[order[j]].keys) })
	for _, sk := range order {
		g := groups[sk]
		nRuns := len(g.items[0].runs)
		var segs []string
		for i := 0; i < nRuns; i++ {
			noun := categoryNoun[g.items[0].runs[i].Category]
			if i == 0 {
				segs = append(segs, noun+" early")
				continue
			}
			var starts []int
			for _, pk := range g.items {
				starts = append(starts, pk.runs[i].StartMin)
			}
			sort.Ints(starts)
			segs = append(segs, fmt.Sprintf("%s from ~min %d", noun, starts[len(starts)/2]))
		}
		subject := fmt.Sprintf("Key %d is", g.keys[0])
		if len(g.keys) > 1 {
			subject = fmt.Sprintf("Keys %s are", keyRanges(g.keys))
		}
		parts = append(parts, fmt.Sprintf("%s repurposed: %s.", subject, strings.Join(segs, ", then ")))
	}

	var meds []int
	for _, ks := range sig.Keys {
		if ks.MedianCount >= 2 {
			for _, k := range army {
				if k == ks.Key {
					meds = append(meds, ks.MedianCount)
				}
			}
		}
	}
	if len(meds) > 0 {
		sort.Ints(meds)
		parts = append(parts, fmt.Sprintf("Army groups typically hold ~%d units.", meds[len(meds)/2]))
	}
	if sig.TemporalScore > 0 {
		parts = append(parts, fmt.Sprintf("Minute for minute, %d%% of key presses match this pattern (%d games).",
			int(sig.TemporalScore*100+0.5), sig.Games))
	}
	return strings.Join(parts, " ")
}

func pluralize(noun string, n int) string {
	if n == 1 {
		return "a " + noun
	}
	if strings.HasSuffix(noun, "y") {
		return noun[:len(noun)-1] + "ies"
	}
	if strings.HasSuffix(noun, "s") {
		return noun + "es"
	}
	return noun + "s"
}

// keyRanges renders hotkey numbers compactly, treating 0 as the key after 9
// (keyboard order): [1,2,3,7,8,9,0] -> "1–3, 7–0".
func keyRanges(keys []int) string {
	order := func(k int) int {
		if k == 0 {
			return 10
		}
		return k
	}
	sorted := append([]int(nil), keys...)
	sort.Slice(sorted, func(i, j int) bool { return order(sorted[i]) < order(sorted[j]) })
	var runs [][]int
	run := []int{sorted[0]}
	for _, k := range sorted[1:] {
		if order(k) == order(run[len(run)-1])+1 {
			run = append(run, k)
		} else {
			runs = append(runs, run)
			run = []int{k}
		}
	}
	runs = append(runs, run)
	var out []string
	for _, r := range runs {
		switch len(r) {
		case 1:
			out = append(out, fmt.Sprintf("%d", r[0]))
		case 2:
			out = append(out, fmt.Sprintf("%d and %d", r[0], r[1]))
		default:
			out = append(out, fmt.Sprintf("%d–%d", r[0], r[len(r)-1]))
		}
	}
	return strings.Join(out, ", ")
}
