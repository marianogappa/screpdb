// compositionPill renders the per-game attacker-composition pill family.
// One pill per phase (Early / Mid / Late) carrying:
//   - a 6-slot proportional unit-icon strip (no percentages — counts are
//     converted to slot fills via largest-remainder distribution so the
//     visible width is always exactly SLOT_COUNT icons)
//   - a separator
//   - a right strip of caster icons (spell-casters that cast in this
//     phase, plus signature non-casters that were built — all deduped,
//     server-side)
//
// Backend source:
//   - boundaries persisted at ingest as replay-level markers
//     (mid_game_starts, late_game_starts)
//   - composition rows computed at request time from those boundaries
//     plus the Train/Unit Morph/Cast command stream
//     (internal/dashboard/unit_composition.go)
//
// Distinct from regular pattern pills (markerRegistry.js) because those
// render a single icon + interpolated string; we need a multi-icon
// strip layout that the templated DSL can't express.

import React from 'react';
import { getUnitIcon } from './gameAssets';

const PHASE_RANK = { early: 0, mid: 1, late: 2 };
// SLOT_COUNT_DEFAULT is the per-player pill width — compact so the
// row of player strips stays readable on dense game-summary surfaces
// while still showing the dominant 1-3 units clearly.
//
// Callers override this for the replay-aggregate pills on the per-game
// summary, where there's only one row of three pills total and a wider
// strip reads better. See SLOT_COUNT_AGGREGATE in App.jsx integration.
const SLOT_COUNT_DEFAULT = 6;

const formatPhaseLabel = (phase) => {
  switch (phase) {
    case 'early': return 'Early';
    case 'mid':   return 'Mid';
    case 'late':  return 'Late';
    default:      return phase || '';
  }
};

// sortPhasesByRank stable-sorts a list of phase entries early -> mid -> late.
export const sortPhasesByRank = (phases) =>
  [...(phases || [])].sort((a, b) => (PHASE_RANK[a.phase] ?? 99) - (PHASE_RANK[b.phase] ?? 99));

// computeReplayAggregatePhases sums per-player counts and unions casters
// across all rows for a single replay, returning per-phase entries
// shaped identically to a single-player phase. Source:
// detail.unit_composition_markers.
export const computeReplayAggregatePhases = (rows) => {
  if (!Array.isArray(rows) || rows.length === 0) return [];
  const byPhase = new Map(); // phase -> { units: Map<name,count>, casters: Set<string> }
  for (const row of rows) {
    if (!row || !row.phase) continue;
    let entry = byPhase.get(row.phase);
    if (!entry) {
      entry = { units: new Map(), casters: new Set() };
      byPhase.set(row.phase, entry);
    }
    for (const u of (row.units || [])) {
      if (!u || !u.name) continue;
      entry.units.set(u.name, (entry.units.get(u.name) || 0) + Number(u.count || 0));
    }
    for (const c of (row.casters || [])) {
      if (typeof c === 'string' && c) entry.casters.add(c);
    }
  }
  const phases = [];
  for (const [phase, entry] of byPhase.entries()) {
    const units = Array.from(entry.units.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
    const casters = Array.from(entry.casters).sort();
    phases.push({ phase, units, casters });
  }
  return sortPhasesByRank(phases);
};

// distributeSlots turns raw unit counts into an ordered list of
// SLOT_COUNT unit-name slots. Largest-remainder method: each unit gets
// floor(count*N/total) slots, then any remaining slots are handed out
// to the units with the largest fractional part (ties broken by count
// desc, then name asc — same ordering as the units list itself).
//
// Concrete example: { Marine: 168, Battlecruiser: 8, Nuke: 4 }, N=10:
//   total=180; exact = [9.333, 0.444, 0.222]; floor = [9, 0, 0];
//   remainder = 1; goes to Marine (largest fraction) → [10, 0, 0].
//
// Returned as a flat array of unit names so the renderer can map each
// slot to one icon without re-tracking counts.
export const distributeSlots = (units, slotCount = SLOT_COUNT_DEFAULT) => {
  const total = (units || []).reduce((acc, u) => acc + (Number(u.count) || 0), 0);
  if (total === 0 || slotCount <= 0) return [];

  const sorted = [...units].sort((a, b) =>
    (Number(b.count) || 0) - (Number(a.count) || 0) || a.name.localeCompare(b.name));
  const exact = sorted.map((u) => ({
    name: u.name,
    count: Number(u.count) || 0,
    raw: ((Number(u.count) || 0) * slotCount) / total,
  }));
  const floors = exact.map((e) => ({ ...e, slots: Math.floor(e.raw), frac: e.raw - Math.floor(e.raw) }));
  let assigned = floors.reduce((acc, f) => acc + f.slots, 0);
  // Hand out remainder to highest fractional parts. Stable tiebreak by
  // count desc, then name asc — so the largest unit class wins ties.
  if (assigned < slotCount) {
    const remainderOrder = [...floors].sort((a, b) =>
      (b.frac - a.frac) || (b.count - a.count) || a.name.localeCompare(b.name));
    for (const item of remainderOrder) {
      if (assigned >= slotCount) break;
      const target = floors.find((f) => f.name === item.name);
      target.slots += 1;
      assigned += 1;
    }
  }

  const result = [];
  for (const f of floors) {
    for (let i = 0; i < f.slots; i++) result.push(f.name);
  }
  // Defensive: if rounding overshoots (shouldn't, but cheap guard), trim.
  return result.slice(0, slotCount);
};

// CompositionPill renders one phase pill: phase label, N-slot unit
// strip, optional caster strip on the right. Tooltip carries the full
// unit-count breakdown + caster list for power users who want to audit.
//
// slotCount: width of the unit strip in chips. Defaults to
// SLOT_COUNT_DEFAULT (per-player); the replay-aggregate pills on the
// per-game summary pass a larger value (10) since there's a single
// row of three pills with room to spare.
//
// maxCasters: optional cap on the right-strip length. Per-player rows
// pass 4 to keep individual player strips compact; the replay-aggregate
// pills leave it unbounded so the full set is visible there.
// Truncation always shows the head of the (alphabetical) list — the
// tooltip carries the full set regardless.
export const CompositionPill = ({ phase, units, casters, maxCasters, slotCount }) => {
  const safeUnits = Array.isArray(units) ? units : [];
  const safeCasters = Array.isArray(casters) ? casters : [];
  const effectiveSlotCount = (typeof slotCount === 'number' && slotCount > 0) ? slotCount : SLOT_COUNT_DEFAULT;
  const slots = distributeSlots(safeUnits, effectiveSlotCount);
  const visibleCasters = (typeof maxCasters === 'number' && maxCasters >= 0)
    ? safeCasters.slice(0, maxCasters)
    : safeCasters;
  if (slots.length === 0 && visibleCasters.length === 0) return null;

  const total = safeUnits.reduce((acc, u) => acc + (Number(u.count) || 0), 0);
  const tooltipParts = [`${formatPhaseLabel(phase)} game`];
  if (safeUnits.length > 0) {
    tooltipParts.push('Composition:');
    for (const u of safeUnits) {
      const pct = total > 0 ? ((Number(u.count) || 0) * 100) / total : 0;
      tooltipParts.push(`${u.name}: ${u.count} (${pct.toFixed(0)}%)`);
    }
  }
  if (safeCasters.length > 0) {
    // Tooltip always shows the full caster list, even when the visible
    // strip is truncated, so power users can still see what was hidden.
    tooltipParts.push('Notable: ' + safeCasters.join(', '));
  }
  const tooltip = tooltipParts.join('\n');

  return (
    <span className="workflow-pattern-pill workflow-pattern-pill-strong workflow-composition-pill workflow-pill-legended" title={tooltip}>
      <span className="workflow-pill-legend">{formatPhaseLabel(phase)} Composition</span>
      <span className="workflow-composition-pill-units">
        {slots.map((name, idx) => {
          const icon = getUnitIcon(name);
          return (
            <span key={`${name}-${idx}`} className="workflow-composition-pill-slot" title={name}>
              {icon ? <img className="workflow-composition-pill-icon" src={icon} alt="" /> : null}
            </span>
          );
        })}
      </span>
      {visibleCasters.length > 0 ? (
        <span className="workflow-composition-pill-casters" title={safeCasters.join(', ')}>
          {visibleCasters.map((name) => {
            const icon = getUnitIcon(name);
            return (
              <span key={name} className="workflow-composition-pill-caster" title={name}>
                {icon ? <img className="workflow-composition-pill-icon" src={icon} alt="" /> : null}
              </span>
            );
          })}
        </span>
      ) : null}
    </span>
  );
};

// CompositionPhasesRow renders up to 3 phase pills in early -> mid -> late
// order. Returns null when nothing renders so callers can chain `?`.
//
// maxCasters / slotCount: forwarded to each CompositionPill. Per-player
// strips use the defaults (compact); replay-aggregate rows pass larger
// values since they have a single row to themselves.
export const CompositionPhasesRow = ({ phases, maxCasters, slotCount }) => {
  const sorted = sortPhasesByRank(phases);
  const visible = sorted.filter((p) => (p.units && p.units.length > 0) || (p.casters && p.casters.length > 0));
  if (visible.length === 0) return null;
  return (
    <span className="workflow-composition-pill-row">
      {visible.map((p) => (
        <CompositionPill
          key={p.phase}
          phase={p.phase}
          units={p.units}
          casters={p.casters}
          maxCasters={maxCasters}
          slotCount={slotCount}
        />
      ))}
    </span>
  );
};
