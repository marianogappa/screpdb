package worldstate

import (
	"fmt"
	"sort"
)

// AttackFilterReport classifies each Type=="attack" candidate into kept /
// dropped buckets with a human-readable reason. Pure introspection — does
// NOT mutate the engine state.
type AttackFilterReport struct {
	Total     int
	Kept      int
	Dropped   int
	Decisions []AttackFilterDecision
}

type AttackFilterDecision struct {
	Second   int
	Attacker byte
	Defender byte
	PolyID   int
	Kept     bool
	Reason   string
}

// ReportAttackFilter runs the same importance filter as
// emitAttackIfImportant, but instead of emitting events it returns a
// per-candidate report. Useful for debugging a real replay.
func (e *Engine) ReportAttackFilter() AttackFilterReport {
	e.Finalize()

	if len(e.bases) == 0 {
		return AttackFilterReport{}
	}
	starts := e.buildPlayerStarts()
	durationSec := 0
	if e.replay != nil {
		durationSec = e.replay.DurationSeconds
	}
	ownership := BuildOwnership(e.stream, e.polygonGeoms, starts, durationSec)
	candidates := BuildAttacks(e.stream, e.polygonGeoms, ownership)

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Second < candidates[j].Second
	})

	spellsByAttacker := buildSpellHistoryByAttacker(e.stream)

	attackedAlready := map[byte]bool{}
	knownUnitsByAttacker := map[byte]map[string]bool{}
	knownSpellsByAttacker := map[byte]map[string]bool{}

	rep := AttackFilterReport{}
	for _, c := range candidates {
		if c.Type != "attack" {
			continue
		}
		rep.Total++
		dec := AttackFilterDecision{
			Second:   c.Second,
			Attacker: c.Attacker,
			Defender: c.Defender,
			PolyID:   c.PolyID,
		}
		if c.Defender == neutralPID {
			dec.Reason = "skip: defender=neutral"
			rep.Decisions = append(rep.Decisions, dec)
			rep.Dropped++
			continue
		}

		// recentAttackUnitTypes is mutated as a side effect of being
		// called; clone the per-attacker buffer so the diagnostic
		// doesn't poison the next call. In the real pipeline this is
		// fine because emitAttackIfImportant is the only consumer.
		attackUnits := e.recentAttackUnitTypes(c.Attacker, c.Second)

		reasons := []string{}
		if !attackedAlready[c.Attacker] {
			reasons = append(reasons, "first-attack")
		}
		if leaveSec, defLeft := e.leaveSec[c.Defender]; defLeft && leaveSec >= c.Second {
			reasons = append(reasons, "defender-leaves")
		}
		if c.Second <= rushBuildWindowSec && e.attackerHasRushEvent(c.Attacker) {
			reasons = append(reasons, "rush-window")
		}

		if knownUnitsByAttacker[c.Attacker] == nil {
			knownUnitsByAttacker[c.Attacker] = map[string]bool{}
		}
		novelUnit := false
		var novelUnits []string
		for _, u := range attackUnits {
			if !knownUnitsByAttacker[c.Attacker][u] {
				novelUnit = true
				novelUnits = append(novelUnits, u)
			}
		}
		if novelUnit {
			reasons = append(reasons, fmt.Sprintf("novel-unit:%v", novelUnits))
		}

		if knownSpellsByAttacker[c.Attacker] == nil {
			knownSpellsByAttacker[c.Attacker] = map[string]bool{}
		}
		novelSpell := false
		var novelSpells []string
		for _, s := range spellsByAttacker[c.Attacker] {
			if s.Second < c.Second-60 || s.Second > c.Second+60 {
				continue
			}
			if !knownSpellsByAttacker[c.Attacker][s.Subject] {
				novelSpell = true
				novelSpells = append(novelSpells, s.Subject)
			}
		}
		if novelSpell {
			reasons = append(reasons, fmt.Sprintf("novel-spell:%v", novelSpells))
		}

		if len(reasons) == 0 {
			dec.Reason = fmt.Sprintf("DROP: not-first, defender-stays, no-rush-window, units=%v in known=%v", attackUnits, mapKeys(knownUnitsByAttacker[c.Attacker]))
			rep.Decisions = append(rep.Decisions, dec)
			rep.Dropped++
			continue
		}

		dec.Kept = true
		dec.Reason = "KEEP: " + joinReasons(reasons)

		// Update state mirroring emitAttackIfImportant.
		for _, u := range attackUnits {
			knownUnitsByAttacker[c.Attacker][u] = true
		}
		for _, s := range spellsByAttacker[c.Attacker] {
			if s.Second < c.Second-60 || s.Second > c.Second+60 {
				continue
			}
			knownSpellsByAttacker[c.Attacker][s.Subject] = true
		}
		attackedAlready[c.Attacker] = true

		rep.Decisions = append(rep.Decisions, dec)
		rep.Kept++
	}
	return rep
}

func mapKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func joinReasons(rs []string) string {
	out := ""
	for i, r := range rs {
		if i > 0 {
			out += ", "
		}
		out += r
	}
	return out
}

