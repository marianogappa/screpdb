// compositionPill renders the per-game attacker-composition family: one
// fixed-width module split into early | mid | late zones side by side. Each zone
// is a proportional stacked bar — a segment per distinct unit, width by share of
// that phase's production, with the icon inside and a % label when wide enough.
// A phase the game never reached renders as a faint "–".
//
// Spellcasters are NOT in the bars: the backend emits a per-phase list of
// distinct (unit, spell) casts, surfaced instead as a "Spellcasts" pill on the
// Featuring strip and a chip block on the wider summary surface. The same unit
// icon can appear under several spells (Vessel → Irradiate + EMP).
//
// Boundaries are persisted at ingest as replay-level markers; the composition
// rows are computed at request time in internal/dashboard/unit_composition.go.

import React from 'react';
import { getUnitIcon } from './gameAssets';
import { slugKey } from './i18n';
import { useT } from './i18nContext';

export const PHASE_ORDER = ['early', 'mid', 'late'];
const PHASE_RANK = { early: 0, mid: 1, late: 2 };

// Segment fills step down by position within a phase bar, dominant unit first.
// The icon already identifies the unit and the width already gives its share, so
// hue carried nothing: six saturated fills per row made the bars the loudest
// thing on the summary while saying no more than a greyscale ramp. The ramp still
// separates adjacent segments and leaves the unit sprites legible on top.
const SEG_PALETTE = [
  'rgba(255, 255, 255, 0.24)',
  'rgba(255, 255, 255, 0.17)',
  'rgba(255, 255, 255, 0.125)',
  'rgba(255, 255, 255, 0.095)',
  'rgba(255, 255, 255, 0.07)',
  'rgba(255, 255, 255, 0.05)',
];
const SEG_TAIL_FILL = 'rgba(255, 255, 255, 0.035)';

const formatPhaseLabel = (t, phase) => {
  switch (phase) {
    case 'early': return t('composition.phase.early');
    case 'mid':   return t('composition.phase.mid');
    case 'late':  return t('composition.phase.late');
    default:      return phase || '';
  }
};

const unitName = (t, name) => t.server(`server.name.${slugKey(name)}`, name);

export const sortPhasesByRank = (phases) =>
  [...(phases || [])].sort((a, b) => (PHASE_RANK[a.phase] ?? 99) - (PHASE_RANK[b.phase] ?? 99));

// Sums per-player counts and unions spells across a replay's rows, returning
// per-phase entries shaped identically to a single player's.
export const computeReplayAggregatePhases = (rows) => {
  if (!Array.isArray(rows) || rows.length === 0) return [];
  const byPhase = new Map(); // phase -> { units: Map<name,count>, spells: Map<key,{unit,spell}> }
  for (const row of rows) {
    if (!row || !row.phase) continue;
    let entry = byPhase.get(row.phase);
    if (!entry) {
      entry = { units: new Map(), spells: new Map() };
      byPhase.set(row.phase, entry);
    }
    for (const u of (row.units || [])) {
      if (!u || !u.name) continue;
      entry.units.set(u.name, (entry.units.get(u.name) || 0) + Number(u.count || 0));
    }
    for (const s of (row.spells || [])) {
      if (!s || !s.spell) continue;
      entry.spells.set(`${s.unit}\x00${s.spell}`, { unit: s.unit, spell: s.spell });
    }
  }
  const phases = [];
  for (const [phase, entry] of byPhase.entries()) {
    const units = Array.from(entry.units.entries())
      .map(([name, count]) => ({ name, count }))
      .sort((a, b) => b.count - a.count || a.name.localeCompare(b.name));
    const spells = sortSpells(Array.from(entry.spells.values()));
    phases.push({ phase, units, spells });
  }
  return sortPhasesByRank(phases);
};

const sortSpells = (spells) =>
  [...(spells || [])].sort((a, b) =>
    (a.unit || '').localeCompare(b.unit || '') || (a.spell || '').localeCompare(b.spell || ''));

// Flattens the distinct (unit, spell) casts across a player's phases into one
// ordered list for the Spellcasts pill.
export const collectPlayerSpells = (phases) => {
  const seen = new Map();
  for (const p of (phases || [])) {
    for (const s of (p?.spells || [])) {
      if (!s || !s.spell) continue;
      seen.set(`${s.unit}\x00${s.spell}`, { unit: s.unit, spell: s.spell });
    }
  }
  return sortSpells(Array.from(seen.values()));
};

// withProportions orders segments by count desc then name. A single-unit zone
// gets no % label — the lone icon already reads as "all of this".
//
// Beyond `cap` distinct units the long tail is split off for the renderer to
// collapse into a "+k" chip rather than overflowing the zone. Percentages are
// always computed against the FULL phase total, so shown shares stay honest and
// sum to under 100 when a tail exists.
const withProportions = (units, cap) => {
  const safe = (units || []).filter((u) => u && u.name && Number(u.count) > 0);
  const total = safe.reduce((acc, u) => acc + Number(u.count), 0);
  if (total === 0) return { shown: [], hidden: [] };
  const sorted = [...safe].sort((a, b) =>
    Number(b.count) - Number(a.count) || a.name.localeCompare(b.name));
  const segs = sorted.map((u) => ({
    name: u.name,
    count: Number(u.count),
    pct: Math.round((Number(u.count) * 100) / total),
  }));
  if (typeof cap === 'number' && cap > 0 && segs.length > cap) {
    // The "+k" chip takes a slot, so keep at most `cap` visual items — never cap+1,
    // which would overflow.
    return { shown: segs.slice(0, cap - 1), hidden: segs.slice(cap - 1) };
  }
  return { shown: segs, hidden: [] };
};

const CompositionZone = ({ units, cap }) => {
  const t = useT();
  const { shown, hidden } = withProportions(units, cap);
  // A phase the game never reached stays a visible hole so the three-bar rhythm
  // holds across rows.
  if (shown.length === 0) {
    return <span className="workflow-composition-bar workflow-composition-bar-empty" title={t('composition.noArmyProduction')} />;
  }
  const segmentTooltip = (seg) =>
    t('composition.segmentTooltip', { name: unitName(t, seg.name), count: seg.count, pct: seg.pct });
  const hiddenWeight = hidden.reduce((acc, h) => acc + h.count, 0);
  const hiddenTooltip = hidden.map(segmentTooltip).join('\n');
  return (
    <span className="workflow-composition-bar">
      {shown.map((seg, idx) => {
        const icon = getUnitIcon(seg.name);
        return (
          <span
            key={`${seg.name}-${idx}`}
            className="workflow-composition-seg"
            style={{ flexGrow: seg.count, background: SEG_PALETTE[idx % SEG_PALETTE.length] }}
            title={segmentTooltip(seg)}
          >
            {icon ? <img className="workflow-composition-seg-icon" src={icon} alt="" /> : null}
          </span>
        );
      })}
      {hidden.length > 0 ? (
        <span
          className="workflow-composition-seg workflow-composition-seg-more"
          style={{ flexGrow: hiddenWeight, background: SEG_TAIL_FILL }}
          title={t('composition.moreTooltip', { count: hidden.length, list: hiddenTooltip })}
        >+{hidden.length}</span>
      ) : null}
    </span>
  );
};

// Render once above a stack of CompositionZones rows.
export const CompositionZonesHeader = ({ slim }) => {
  const t = useT();
  return (
    <span className={`workflow-composition-zones workflow-composition-zones-header${slim ? ' workflow-composition-zones-slim' : ''}`}>
      {PHASE_ORDER.map((phase) => (
        <span key={phase} className="workflow-composition-zone-label">{formatPhaseLabel(t, phase)}</span>
      ))}
    </span>
  );
};

// Always three fixed columns so rows line up under the shared header; a missing
// phase shows a faint "–".
export const CompositionZones = ({ phases, slim }) => {
  const byPhase = new Map();
  for (const p of (phases || [])) {
    if (p && p.phase) byPhase.set(p.phase, p);
  }
  const hasAny = PHASE_ORDER.some((phase) => (byPhase.get(phase)?.units || []).length > 0);
  if (!hasAny) return null;
  // Per-player zones are ~60px against the summary's ~230px, so they hold far
  // fewer icons before overflowing.
  const cap = slim ? 3 : 7;
  return (
    <span className={`workflow-composition-zones${slim ? ' workflow-composition-zones-slim' : ''}`}>
      {PHASE_ORDER.map((phase) => (
        <span key={phase} className="workflow-composition-zone">
          <CompositionZone units={byPhase.get(phase)?.units} cap={cap} />
        </span>
      ))}
    </span>
  );
};

const spellTooltip = (t, s) =>
  t('composition.spellTooltip', { unit: unitName(t, s.unit), spell: unitName(t, s.spell) });

const spellIcon = (unit) => {
  const icon = getUnitIcon(unit);
  return icon ? <img className="workflow-spellcast-icon" src={icon} alt="" /> : null;
};

// Icons only, one per distinct cast, so the same unit repeats for distinct
// spells. Spell names live in the tooltip.
export const SpellcastsPill = ({ spells }) => {
  const t = useT();
  const safe = sortSpells(spells);
  if (safe.length === 0) return null;
  const tooltip = safe.map((s) => spellTooltip(t, s)).join('\n');
  return (
    <span className="workflow-pattern-pill workflow-pattern-pill-strong workflow-spellcasts-pill workflow-pill-legended" title={tooltip}>
      <span className="workflow-pill-legend">{t('composition.castsLegend')}</span>
      {safe.map((s, idx) => (
        <span key={`${s.unit}-${s.spell}-${idx}`} className="workflow-spellcast-icon-wrap" title={spellTooltip(t, s)}>
          {spellIcon(s.unit)}
        </span>
      ))}
    </span>
  );
};

// Standard summary pills (icon + spell name), matching this tab's Featuring
// pills, one per cast.
export const SpellcastsChips = ({ spells }) => {
  const t = useT();
  const safe = sortSpells(spells);
  if (safe.length === 0) return null;
  return (
    <>
      {safe.map((s, idx) => {
        const icon = getUnitIcon(s.unit);
        return (
          <span
            key={`${s.unit}-${s.spell}-${idx}`}
            className="workflow-pattern-pill workflow-pattern-pill-strong workflow-summary-feature-pill"
            title={spellTooltip(t, s)}
          >
            {icon ? <img className="workflow-pattern-icon" src={icon} alt="" /> : null}
            <span>{unitName(t, s.spell)}</span>
          </span>
        );
      })}
    </>
  );
};
