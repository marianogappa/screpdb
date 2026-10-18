import React from 'react';
import { useT } from '../lib/i18nContext';

// SkillTab places each player of a 1v1 on four tiers fitted to ladder MMR (internal/skill). Positions are on
// a shared tier axis, 0 to 4, where each whole number is a cut between tiers, so players of different races
// compare directly. Each strip zooms to the tiers its players span: in most games they are close.

export const TIER_KEYS = ['beginner', 'intermediate', 'advanced', 'pro'];

const MECHANICS = [
  { key: 'eapm', format: (v) => Math.round(v) },
  { key: 'hotkey_recalls_per_min', format: (v) => Math.round(v) },
  { key: 'viewport_switches_per_min', format: (v) => v.toFixed(1) },
];
const MACRO = [
  { key: 'spending_quotient', format: (v) => Math.round(v) },
  { key: 'worker_production', format: (v, metric) => (metric === 'larva_per_hatch' ? v.toFixed(2) : `${Math.round(v * 100)}%`) },
  { key: 'float_share', format: (v) => `${Math.round(v * 100)}%` },
];

const ROW_PX = 22;
const AXIS_PX = 18;
// A label takes about this share of the strip; closer players go on the next row.
const LABEL_SHARE = 0.16;

const placementOf = (player, key) => {
  const metrics = player?.skill?.metrics || {};
  if (key === 'combined') return player?.skill?.combined ? { ...player.skill.combined, metric: 'combined' } : null;
  if (key === 'worker_production') {
    if (metrics.hall_worker_busy) return { ...metrics.hall_worker_busy, metric: 'hall_worker_busy' };
    if (metrics.larva_per_hatch) return { ...metrics.larva_per_hatch, metric: 'larva_per_hatch' };
    return null;
  }
  return metrics[key] ? { ...metrics[key], metric: key } : null;
};

// The tier a combined score names, or two when it sits near a cut: the model is only trusted away from cuts.
export const tierLabel = (t, skill) => {
  const combined = skill?.combined;
  if (!combined) return '';
  const tier = t(`skill.tier.${TIER_KEYS[combined.tier]}`);
  if (skill.confident) return tier;
  const frac = combined.position - Math.floor(combined.position);
  const other = Math.max(0, Math.min(3, frac < 0.5 ? combined.tier - 1 : combined.tier + 1));
  if (other === combined.tier) return tier;
  const [a, b] = other < combined.tier ? [other, combined.tier] : [combined.tier, other];
  return t('skill.between', { a: t(`skill.tier.${TIER_KEYS[a]}`), b: t(`skill.tier.${TIER_KEYS[b]}`) });
};

function Strip({ entries, playerColor, format }) {
  const t = useT();
  const positions = entries.map((e) => e.placement.position);
  const lo = Math.max(0, Math.floor(Math.min(...positions) - 0.25));
  const hi = Math.min(4, Math.max(lo + 1, Math.ceil(Math.max(...positions) + 0.25)));
  // Markers keep a small margin from the strip's edges so a value at the end of the scale stays visible.
  const share = (pos) => 0.015 + 0.97 * ((pos - lo) / (hi - lo));
  const band = (pos) => (pos - lo) / (hi - lo);
  const rows = [];
  const placed = [...entries].sort((a, b) => a.placement.position - b.placement.position).map((e) => {
    const x = share(e.placement.position);
    let row = 0;
    while (rows[row] !== undefined && x - rows[row] < LABEL_SHARE) row += 1;
    rows[row] = x;
    return { ...e, x, row };
  });
  const tiers = [];
  for (let i = lo; i < hi; i += 1) tiers.push(i);
  return (
    <div className="skill-strip" style={{ height: `${AXIS_PX + Math.max(1, rows.length) * ROW_PX + 6}px` }}>
      {tiers.map((i) => (
        <div
          key={i}
          className={`skill-band skill-band-${TIER_KEYS[i]}`}
          style={{ left: `${band(i) * 100}%`, width: `${(band(i + 1) - band(i)) * 100}%` }}
        >
          <span className="skill-band-label">{t(`skill.tier.${TIER_KEYS[i]}`)}</span>
        </div>
      ))}
      {placed.map(({ player, placement, x, row }) => {
        const right = x > 1 - LABEL_SHARE;
        return (
          <div
            key={player.player_id}
            className={`skill-marker${right ? ' skill-marker--right' : ''}`}
            style={{ left: `${x * 100}%`, top: `${AXIS_PX + row * ROW_PX}px` }}
          >
            <span className="skill-marker-dot" style={{ background: playerColor(player.color) }} />
            <span className="skill-player-label">
              {player.name} <span className="skill-player-value">{format(placement.value, placement.metric)}</span>
            </span>
          </div>
        );
      })}
    </div>
  );
}

function Metric({ id, players, playerColor, format }) {
  const t = useT();
  const entries = players.map((player) => ({ player, placement: placementOf(player, id) })).filter((e) => e.placement);
  if (!entries.length) return null;
  return (
    <div className="skill-metric">
      <div className="skill-metric-head">
        <span className="skill-metric-title">{t(`skill.metric.${id}.title`)}</span>
        <span className="skill-metric-hint">{t(`skill.metric.${id}.hint`)}</span>
      </div>
      <Strip entries={entries} playerColor={playerColor} format={format} />
    </div>
  );
}

export default function SkillTab({ game, players, playerColor }) {
  const t = useT();
  const status = game?.skill_status;
  if (status !== 'ok') {
    return (
      <div className="workflow-card skill-tab">
        <div className="chart-empty">{t(status === 'not_1v1' ? 'skill.status.not1v1' : status === 'too_short' ? 'skill.status.tooShort' : 'skill.status.unavailable')}</div>
      </div>
    );
  }
  const rated = (players || []).filter((p) => p.skill);
  const hasMacro = rated.some((p) => MACRO.some((m) => placementOf(p, m.key)));
  return (
    <div className="workflow-card skill-tab">
      <div className="skill-summary">
        {rated.map((p) => (
          <div key={p.player_id} className="skill-summary-player">
            <span className="skill-summary-dot" style={{ background: playerColor(p.color) }} />
            <span className="skill-summary-name">{p.name}</span>
            <span className="skill-summary-tier">{tierLabel(t, p.skill)}</span>
          </div>
        ))}
      </div>
      <Metric id="combined" players={rated} playerColor={playerColor} format={() => ''} />
      <h3 className="skill-section">{t('skill.section.mechanics')}</h3>
      {MECHANICS.map((m) => <Metric key={m.key} id={m.key} players={rated} playerColor={playerColor} format={m.format} />)}
      {hasMacro ? (
        <>
          <h3 className="skill-section">{t('skill.section.macro')}</h3>
          {MACRO.map((m) => <Metric key={m.key} id={m.key} players={rated} playerColor={playerColor} format={m.format} />)}
        </>
      ) : null}
      <p className="skill-footnote">{t('skill.footnote')}</p>
    </div>
  );
}
