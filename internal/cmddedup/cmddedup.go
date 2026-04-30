// Package cmddedup removes duplicate research and upgrade commands from a
// replay's command stream.
//
// In a clean game each research/upgrade should appear at most once (Tech and
// one-shot Upgrade) or up to three times in level order (tiered Upgrade like
// Ground Weapons). Replays sometimes record duplicates: a Forge gets killed
// mid-research and the player re-clicks the upgrade on a new Forge; a player
// double-clicks Lurker Aspect; etc.
//
// Algorithm — for each (player, name) group, walking events in time order:
//
//   - Tech, or Upgrade with MaxLevel == 1: keep the latest occurrence; drop
//     all earlier ones.
//   - Upgrade with MaxLevel == 3: walk events; for each new event, if it
//     arrives within the research duration of the last accepted level, treat
//     it as spam and replace the last accepted (keep latest). If it arrives
//     after the duration, treat it as the next level and append. Cap at
//     MaxLevel; further occurrences are dropped.
//
// Commands not in {Tech, Upgrade} pass through unchanged. Unknown names (not
// in models.LookupTech / models.LookupUpgrade) also pass through — we'd rather
// surface an unknown event than silently swallow it.
package cmddedup

import (
	"sort"

	"github.com/marianogappa/screpdb/internal/models"
)

// Dedup returns a new slice with duplicate Tech/Upgrade commands removed,
// preserving relative order of all kept commands. Input is not mutated.
func Dedup(commands []*models.Command) []*models.Command {
	if len(commands) == 0 {
		return commands
	}

	dropped := decideDrops(commands)
	if len(dropped) == 0 {
		return commands
	}

	out := make([]*models.Command, 0, len(commands)-len(dropped))
	for i, cmd := range commands {
		if dropped[i] {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

// decideDrops returns a map[index]true for command indices that should be
// dropped. We compute this with a per-(player,name) walk in time order, then
// project back onto original-stream indices.
func decideDrops(commands []*models.Command) map[int]bool {
	type key struct {
		player int64
		name   string
		kind   string // "tech" or "upgrade"
	}
	type entry struct {
		idx    int
		second int
	}

	groups := map[key][]entry{}
	for i, cmd := range commands {
		if cmd == nil {
			continue
		}
		switch cmd.ActionType {
		case "Tech":
			if cmd.TechName == nil {
				continue
			}
			k := key{player: cmd.PlayerID, name: *cmd.TechName, kind: "tech"}
			groups[k] = append(groups[k], entry{idx: i, second: cmd.SecondsFromGameStart})
		case "Upgrade":
			if cmd.UpgradeName == nil {
				continue
			}
			k := key{player: cmd.PlayerID, name: *cmd.UpgradeName, kind: "upgrade"}
			groups[k] = append(groups[k], entry{idx: i, second: cmd.SecondsFromGameStart})
		}
	}

	dropped := map[int]bool{}
	for k, entries := range groups {
		if len(entries) <= 1 {
			continue
		}
		// Stable sort: by (second, original index) so ties resolve to natural order.
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].second != entries[j].second {
				return entries[i].second < entries[j].second
			}
			return entries[i].idx < entries[j].idx
		})

		switch k.kind {
		case "tech":
			// One-shot: keep the latest only.
			if _, ok := models.LookupTech(k.name); !ok {
				// Unknown tech (default ability or unused enum) — pass through.
				continue
			}
			for _, e := range entries[:len(entries)-1] {
				dropped[e.idx] = true
			}
		case "upgrade":
			meta, ok := models.LookupUpgrade(k.name)
			if !ok {
				// Unknown upgrade name — pass through.
				continue
			}
			if meta.MaxLevel == 1 {
				for _, e := range entries[:len(entries)-1] {
					dropped[e.idx] = true
				}
				continue
			}
			// Tiered: walk in order, maintaining the index of the last
			// accepted entry within `entries`. When an event arrives inside
			// the current level's research window, replace last accepted (mark
			// the previously-accepted one as dropped). After max level reached,
			// drop further occurrences.
			lastAccepted := -1
			levelsAccepted := 0
			for i := range entries {
				if lastAccepted < 0 {
					lastAccepted = i
					levelsAccepted = 1
					continue
				}
				prevSecond := entries[lastAccepted].second
				prevLevelDur := meta.Levels[levelsAccepted-1].DurationS
				if float64(entries[i].second) < float64(prevSecond)+prevLevelDur {
					// Within research window of the current level — spam. Drop
					// the previously accepted, accept this one in its place.
					dropped[entries[lastAccepted].idx] = true
					lastAccepted = i
					continue
				}
				// Outside the window — this is the next level.
				if levelsAccepted >= meta.MaxLevel {
					// Already at max; further occurrences are extraneous.
					dropped[entries[i].idx] = true
					continue
				}
				lastAccepted = i
				levelsAccepted++
			}
		}
	}
	return dropped
}
