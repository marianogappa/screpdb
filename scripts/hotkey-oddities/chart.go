package main

import (
	"math"
	"sort"
	"strings"

	"github.com/marianogappa/screpdb/internal/hotkeystream"
	"github.com/marianogappa/screpdb/internal/propack"
)

// Key habits are read from the same signature the Hotkeys chart draws, so a habit never claims a key holds
// something the chart under it does not show. A pro must clear a strict bar to be said to keep something on a
// key; another pro counts as doing the same as soon as they clear a loose one, so "none of the other pros do
// this" is never true only because the others also used that key for something else later in the game.
const (
	// strictMinutes, strictMinuteShare and strictPressShare are how long, and how much of the key, the content
	// must hold for a pro to be said to keep it there.
	strictMinutes     = 4
	strictMinuteShare = 0.5
	strictPressShare  = 0.6
	// strictKeyShare is the share of all presses the key needs, so a line never rests on a key the pro barely
	// touches.
	strictKeyShare = 0.05
	// looseMinutes and loosePressShare are enough for another pro to count as doing the same.
	looseMinutes    = 3
	loosePressShare = 0.2
	// strictMainHallStart is when the main base must already hold the key; a hall that arrives later may be a
	// hatch-first natural or an expansion. looseMainHallStart lets a peer count as doing the same a little later.
	strictMainHallStart = 0
	looseMainHallStart  = 2
	// workerMinMinutes and workerMinPressesPerGame keep worker lines to workers a pro really keeps on a key, not
	// the one-off bind every pro makes in the first seconds.
	workerMinMinutes        = 3
	workerMinPressesPerGame = 3
	looseWorkerMinutes      = 2
)

var workers = map[string]bool{"Probe": true, "SCV": true, "Drone": true}

var halls = map[string]bool{"Nexus": true, "Command Center": true, "Hatchery": true, "Lair": true, "Hive": true}

// notUnits are the larvae, eggs and cocoons a Zerg selects to morph: never a group.
var notUnits = map[string]bool{"Larva": true, "Egg": true, "Lurker Egg": true, "Cocoon": true}

// sameUnit merges unit types that are one unit to a player.
var sameUnit = map[string]string{"Siege Tank (Tank Mode)": "Siege Tank", "Terran Siege Tank (Siege Mode)": "Siege Tank"}

func unitOf(r hotkeystream.KeyRun) string {
	if u, ok := sameUnit[r.Unit]; ok {
		return u
	}
	return r.Unit
}

// excludedProIDs are labels that do not name one player.
var excludedProIDs = map[string]bool{"sai": true}

type habit struct {
	kind string
	key  int
	unit string
}

type card struct {
	id, label, race string
	sig             *hotkeystream.Signature
	totalPresses    int
}

func categoryOf(r hotkeystream.KeyRun) string {
	if halls[r.Unit] {
		return hotkeystream.CategoryHall
	}
	base, _, _ := strings.Cut(r.Category, "/")
	return base
}

func (h habit) matches(r hotkeystream.KeyRun) bool {
	switch h.kind {
	case propack.OddityWorkerKey:
		return workers[r.Unit]
	case propack.OddityUnitKey:
		return unitOf(r) == h.unit
	case propack.OddityProdKey:
		return categoryOf(r) == hotkeystream.CategoryProd
	}
	return categoryOf(r) == hotkeystream.CategoryHall
}

type keyStats struct {
	keyMinutes, keyPresses, minutes, presses, firstStart int
}

func (c card) stats(h habit) (keyStats, bool) {
	for _, k := range c.sig.Keys {
		if k.Key != h.key {
			continue
		}
		s := keyStats{firstStart: -1}
		for _, r := range k.Runs {
			span := r.EndMin - r.StartMin + 1
			s.keyMinutes += span
			s.keyPresses += r.Presses
			if h.matches(r) {
				s.minutes += span
				s.presses += r.Presses
				if s.firstStart < 0 {
					s.firstStart = r.StartMin
				}
			}
		}
		return s, s.keyPresses > 0
	}
	return keyStats{}, false
}

// openingMinutes is how long the content holds the key without a break from minute 0.
func (c card) openingMinutes(h habit) (int, int) {
	for _, k := range c.sig.Keys {
		if k.Key != h.key {
			continue
		}
		minutes, presses, next := 0, 0, 0
		for _, r := range k.Runs {
			if r.StartMin != next || !h.matches(r) {
				break
			}
			minutes += r.EndMin - r.StartMin + 1
			presses += r.Presses
			next = r.EndMin + 1
		}
		return minutes, presses
	}
	return 0, 0
}

func (c card) strict(h habit) bool {
	s, ok := c.stats(h)
	if !ok || float64(s.keyPresses) < strictKeyShare*float64(c.totalPresses) {
		return false
	}
	if h.kind == propack.OddityWorkerKey {
		minutes, presses := c.openingMinutes(h)
		return minutes >= workerMinMinutes && presses >= workerMinPressesPerGame*c.sig.Games &&
			float64(presses) >= strictPressShare*float64(s.keyPresses)
	}
	if h.kind == propack.OddityHallKey && s.firstStart > strictMainHallStart {
		return false
	}
	return s.minutes >= strictMinutes &&
		float64(s.minutes) > strictMinuteShare*float64(s.keyMinutes) &&
		float64(s.presses) >= strictPressShare*float64(s.keyPresses)
}

func (c card) loose(h habit) bool {
	s, ok := c.stats(h)
	if !ok {
		return false
	}
	if h.kind == propack.OddityWorkerKey {
		return s.minutes >= looseWorkerMinutes && float64(s.presses) >= loosePressShare*float64(s.keyPresses)
	}
	if h.kind == propack.OddityHallKey && s.firstStart > looseMainHallStart {
		return false
	}
	return s.minutes >= looseMinutes && float64(s.presses) >= loosePressShare*float64(s.keyPresses)
}

// candidates are the habits the card clears the strict bar for: where the main base goes, a worker kept on a
// key through the opening, the main key of a unit type and a production key. The last two are about the key
// number only, so their lines say which key most players use instead.
func (c card) candidates() []habit {
	var out []habit
	mainHall, mainHallPresses := habit{}, -1
	for _, k := range c.sig.Keys {
		units := map[string]bool{}
		for _, r := range k.Runs {
			if u := unitOf(r); u != "" && !notUnits[u] && !workers[u] && !halls[u] && hotkeystream.BuildingID(u) == 0 {
				units[u] = true
			}
		}
		for u := range units {
			if h := (habit{propack.OddityUnitKey, k.Key, u}); c.strict(h) {
				out = append(out, h)
			}
		}
		if h := (habit{kind: propack.OddityProdKey, key: k.Key}); c.strict(h) {
			out = append(out, h)
		}
		if h := (habit{kind: propack.OddityWorkerKey, key: k.Key}); c.strict(h) {
			out = append(out, h)
		}
		if h := (habit{kind: propack.OddityHallKey, key: k.Key}); c.strict(h) {
			if s, _ := c.stats(h); s.presses > mainHallPresses {
				mainHall, mainHallPresses = h, s.presses
			}
		}
	}
	if mainHallPresses >= 0 {
		out = append(out, mainHall)
	}
	return out
}

func chartOddities(pack *propack.Pack) map[string]map[string][]propack.HotkeyOddity {
	var cards []card
	for i := range pack.Pros {
		pro := &pack.Pros[i]
		if excludedProIDs[pro.ID] {
			continue
		}
		for _, sig := range pro.Hotkeys {
			if !inPool(pro.ID, sig.Race) {
				continue
			}
			c := card{id: pro.ID, label: pro.Label, race: sig.Race, sig: sig}
			for _, k := range sig.Keys {
				for _, r := range k.Runs {
					c.totalPresses += r.Presses
				}
			}
			cards = append(cards, c)
		}
	}
	out := map[string]map[string][]propack.HotkeyOddity{}
	add := func(c card, o propack.HotkeyOddity) {
		if out[c.id] == nil {
			out[c.id] = map[string][]propack.HotkeyOddity{}
		}
		out[c.id][c.race] = append(out[c.id][c.race], o)
	}
	for _, c := range cards {
		var peers []card
		for _, o := range cards {
			if o.race == c.race && o.id != c.id {
				peers = append(peers, o)
			}
		}
		for _, h := range c.candidates() {
			// Any peer clearing the loose bar counts against the habit, but a peer is only named when they clear
			// the strict one too: naming someone on loose evidence could claim a habit they do not have.
			var named []string
			sharedCount := 0
			for _, o := range peers {
				if o.loose(h) {
					sharedCount++
					if o.strict(h) {
						named = append(named, o.label)
					}
				}
			}
			if sharedCount > maxSharedWith {
				continue
			}
			sort.Strings(named)
			if len(named) != sharedCount {
				named = nil
			}
			o := propack.HotkeyOddity{Kind: h.kind, Value: h.key, Unit: h.unit, SharedWith: named, SharedCount: sharedCount, Population: len(peers) + 1}
			if h.kind == propack.OddityUnitKey || h.kind == propack.OddityProdKey {
				norm, majority, ok := usualKey(h, peers)
				if !ok {
					continue
				}
				o.NormKey, o.NormMajority = &norm, majority
			}
			if h.kind == propack.OddityWorkerKey {
				minutes, _ := c.openingMinutes(h)
				key := h.key
				o.Key, o.Value = &key, minutes
			}
			add(c, o)
		}
		if o, ok := fewKeysOddity(c, peers); ok {
			add(c, o)
		}
	}
	return out
}

// fewKeysMax is the most hotkeys a pro may use for that to be a habit: nearly every pro uses all ten, so only
// a pro who leaves several keys empty stands out.
const fewKeysMax = 6

func usedKeys(sig *hotkeystream.Signature) int {
	n := 0
	for _, k := range sig.Keys {
		if len(k.Runs) > 0 {
			n++
		}
	}
	return n
}

func fewKeysOddity(c card, peers []card) (propack.HotkeyOddity, bool) {
	keys := usedKeys(c.sig)
	if keys == 0 || keys > fewKeysMax {
		return propack.HotkeyOddity{}, false
	}
	fewestOther := 10
	for _, o := range peers {
		n := usedKeys(o.sig)
		if n <= keys {
			return propack.HotkeyOddity{}, false
		}
		fewestOther = min(fewestOther, n)
	}
	return propack.HotkeyOddity{Kind: propack.OddityFewKeys, Value: keys, Baseline: fewestOther, Population: len(peers) + 1}, true
}

// usualKey is the key other players of the race most often use for the habit's content: their busiest key for
// a unit type, and their first key for production. majority says whether more than half of them use it; ok is
// false when too few use any key for it, or no key gets at least a third of them.
func usualKey(h habit, peers []card) (int, bool, bool) {
	votes := map[int]int{}
	voters := 0
	for _, o := range peers {
		key, presses := -1, 0
		for _, k := range o.sig.Keys {
			p := 0
			for _, r := range k.Runs {
				if h.matches(r) {
					p += r.Presses
				}
			}
			if p == 0 {
				continue
			}
			if h.kind == propack.OddityProdKey {
				if s, ok := o.stats(habit{kind: h.kind, key: k.Key}); ok && o.loose(habit{kind: h.kind, key: k.Key}) && (key < 0 || keyOrder(k.Key) < keyOrder(key)) {
					key, presses = k.Key, s.presses
				}
			} else if p > presses {
				key, presses = k.Key, p
			}
		}
		if key >= 0 {
			votes[key]++
			voters++
		}
	}
	best, bestVotes := -1, 0
	for k, v := range votes {
		if v > bestVotes || (v == bestVotes && keyOrder(k) < keyOrder(best)) {
			best, bestVotes = k, v
		}
	}
	return best, 2*bestVotes > voters, voters >= 3 && best != h.key && 3*bestVotes >= voters
}

// keyOrder places 0 after 9, as on the keyboard.
func keyOrder(k int) int {
	if k == 0 {
		return 10
	}
	return k
}

const (
	// firstSupplyMaxStart is the latest minute the first Pylon or Supply Depot may arrive on a key.
	firstSupplyMaxStart = 1
	// uncommonMaxShared is how many others may share an uncommon habit: hotkeying the first supply building
	// or using four Comsat keys is not unique, but rare enough to say so with the count.
	uncommonMaxShared = 4
	// firstSupplyMaxShare is the largest share of the race that may hotkey the first supply building for the
	// line to be shown; the line states the share.
	firstSupplyMaxShare = 0.3
	// comsatKeysMin is how many keys must hold a Comsat for at least comsatKeyMinutes each.
	comsatKeysMin    = 4
	comsatKeyMinutes = 2
	// firstSupplyMinPressesPerGame keeps the line to players who press the key while it holds the building,
	// not a bind seen once in a handful of games.
	firstSupplyMinPressesPerGame = 3
)

var supplyBuildings = map[string]bool{"Pylon": true, "Supply Depot": true}

// firstSupplyKey is the key the card puts its first Pylon or Supply Depot on, if any.
func firstSupplyKey(sig *hotkeystream.Signature) (int, string, bool) {
	for _, k := range sig.Keys {
		for _, r := range k.Runs {
			if supplyBuildings[r.Unit] && r.StartMin <= firstSupplyMaxStart {
				return k.Key, r.Unit, true
			}
		}
	}
	return 0, "", false
}

func comsatKeys(sig *hotkeystream.Signature) []int {
	var keys []int
	for _, k := range sig.Keys {
		minutes := 0
		for _, r := range k.Runs {
			if categoryOf(r) == hotkeystream.CategoryComsat {
				minutes += r.EndMin - r.StartMin + 1
			}
		}
		if minutes >= comsatKeyMinutes {
			keys = append(keys, k.Key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keyOrder(keys[i]) < keyOrder(keys[j]) })
	return keys
}

// unitMinutes is how many chart minutes any key holds the unit.
func unitMinutes(sig *hotkeystream.Signature, unit string) int {
	best := 0
	for _, k := range sig.Keys {
		m := 0
		for _, r := range k.Runs {
			if unitOf(r) == unit {
				m += r.EndMin - r.StartMin + 1
			}
		}
		best = max(best, m)
	}
	return best
}

// uncommonOddities are the habits few players share: the first supply building on a key, Comsats on four
// keys, and a unit almost no other player of the race keeps on any key.
func uncommonOddities(pack *propack.Pack) map[string]map[string][]propack.HotkeyOddity {
	type entry struct {
		id, race string
		sig      *hotkeystream.Signature
	}
	var cards []entry
	for i := range pack.Pros {
		pro := &pack.Pros[i]
		if excludedProIDs[pro.ID] {
			continue
		}
		for _, sig := range pro.Hotkeys {
			if inPool(pro.ID, sig.Race) {
				cards = append(cards, entry{pro.ID, sig.Race, sig})
			}
		}
	}
	out := map[string]map[string][]propack.HotkeyOddity{}
	add := func(e entry, o propack.HotkeyOddity) {
		if out[e.id] == nil {
			out[e.id] = map[string][]propack.HotkeyOddity{}
		}
		out[e.id][e.race] = append(out[e.id][e.race], o)
	}
	for _, e := range cards {
		var peers []entry
		for _, o := range cards {
			if o.race == e.race && o.id != e.id {
				peers = append(peers, o)
			}
		}
		if key, building, ok := firstSupplyKey(e.sig); ok && supplyPresses(e.sig, key) >= firstSupplyMinPressesPerGame*e.sig.Games {
			shared := 0
			for _, o := range peers {
				if _, _, ok := firstSupplyKey(o.sig); ok {
					shared++
				}
			}
			if float64(shared+1) <= firstSupplyMaxShare*float64(len(peers)+1) {
				add(e, propack.HotkeyOddity{Kind: propack.OddityFirstSupply, Value: key, Unit: building, SharedCount: shared, Population: len(peers) + 1})
			}
		}
		if keys := comsatKeys(e.sig); len(keys) >= comsatKeysMin {
			shared := 0
			for _, o := range peers {
				if len(comsatKeys(o.sig)) >= len(keys) {
					shared++
				}
			}
			if shared <= uncommonMaxShared {
				add(e, propack.HotkeyOddity{Kind: propack.OddityComsatKeys, Value: len(keys), Keys: keys, SharedCount: shared, Population: len(peers) + 1})
			}
		}
		for _, kept := range timeKeptUnits(e.sig) {
			shared := 0
			for _, o := range peers {
				if unitMinutes(o.sig, kept.unit) >= looseMinutes {
					shared++
				}
			}
			if shared <= 1 {
				add(e, propack.HotkeyOddity{Kind: propack.OddityRareUnit, Value: kept.key, Unit: kept.unit, FromMinute: kept.from, SharedCount: shared, Population: len(peers) + 1})
			}
		}
	}
	return out
}

type keptUnit struct {
	key, from int
	unit      string
}

// timeKeptUnits are the units a card keeps on a key for most of the minutes the chart draws for it. Casters
// and scouts are pressed far less than an army, so their claim rests on time rather than on presses.
func timeKeptUnits(sig *hotkeystream.Signature) []keptUnit {
	total := 0
	for _, k := range sig.Keys {
		for _, r := range k.Runs {
			total += r.Presses
		}
	}
	var out []keptUnit
	for _, k := range sig.Keys {
		keyMinutes, keyPresses := 0, 0
		minutes := map[string]int{}
		from := map[string]int{}
		for _, r := range k.Runs {
			span := r.EndMin - r.StartMin + 1
			keyMinutes += span
			keyPresses += r.Presses
			if u := unitOf(r); u != "" && !notUnits[u] && !workers[u] && !halls[u] && hotkeystream.BuildingID(u) == 0 {
				if _, seen := from[u]; !seen {
					from[u] = r.StartMin
				}
				minutes[u] += span
			}
		}
		if float64(keyPresses) < strictKeyShare*float64(total) {
			continue
		}
		for u, m := range minutes {
			if m >= strictMinutes && float64(m) > strictMinuteShare*float64(keyMinutes) {
				out = append(out, keptUnit{key: k.Key, from: from[u], unit: u})
			}
		}
	}
	return out
}

// standardLayoutMaxRank keeps the standard-layout line to the few players whose charts sit closest to the
// average chart of their race.
const standardLayoutMaxRank = 5

// standardLayoutOddities ranks every card by how close its layout is to the average layout of the other
// players of its race.
func standardLayoutOddities(pack *propack.Pack) map[string]map[string][]propack.HotkeyOddity {
	type entry struct {
		id, race string
		v        []float64
	}
	byRace := map[string][]entry{}
	for i := range pack.Pros {
		pro := &pack.Pros[i]
		if excludedProIDs[pro.ID] {
			continue
		}
		for _, sig := range pro.Hotkeys {
			if !inPool(pro.ID, sig.Race) {
				continue
			}
			v := hotkeystream.LayoutVector(sig)
			norm := 0.0
			for _, x := range v {
				norm += x * x
			}
			if norm == 0 {
				continue
			}
			for j := range v {
				v[j] /= math.Sqrt(norm)
			}
			byRace[sig.Race] = append(byRace[sig.Race], entry{pro.ID, sig.Race, v})
		}
	}
	out := map[string]map[string][]propack.HotkeyOddity{}
	for race, entries := range byRace {
		type scored struct {
			id  string
			sim float64
		}
		var rows []scored
		for i, e := range entries {
			centroid := make([]float64, len(e.v))
			for j, o := range entries {
				if i != j {
					for k := range centroid {
						centroid[k] += o.v[k]
					}
				}
			}
			dot, norm := 0.0, 0.0
			for k := range centroid {
				dot += e.v[k] * centroid[k]
				norm += centroid[k] * centroid[k]
			}
			rows = append(rows, scored{e.id, dot / math.Sqrt(norm)})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].sim > rows[j].sim })
		for rank, r := range rows {
			if rank >= standardLayoutMaxRank {
				break
			}
			if out[r.id] == nil {
				out[r.id] = map[string][]propack.HotkeyOddity{}
			}
			out[r.id][race] = append(out[r.id][race], propack.HotkeyOddity{Kind: propack.OddityStandardLayout, Value: rank + 1, Rank: rank + 1, Population: len(rows)})
		}
	}
	return out
}

func supplyPresses(sig *hotkeystream.Signature, key int) int {
	n := 0
	for _, k := range sig.Keys {
		if k.Key != key {
			continue
		}
		for _, r := range k.Runs {
			if supplyBuildings[r.Unit] {
				n += r.Presses
			}
		}
	}
	return n
}
