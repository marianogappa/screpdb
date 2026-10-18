import React from 'react';
import { useT } from '../lib/i18nContext';

// SkillTab places each player on four tiers fitted to ladder MMR (internal/skill). Positions are on a shared
// tier axis, 0 to 4, where each whole number is a cut between tiers, so players of different races compare
// directly. Each strip zooms to the tiers its players span: in most games they are close.

export const TIER_KEYS = ['beginner', 'intermediate', 'advanced', 'pro'];

const percent = (v) => `${Math.round(v * 100)}%`;

// Worker production is measured differently per race (town halls busy, or larva waiting per hatchery), so
// its strip shows positions only and names each value on hover.
const METRICS = [
  { key: 'eapm', format: (v) => Math.round(v) },
  { key: 'hotkey_recalls_per_min', format: (v) => Math.round(v) },
  { key: 'viewport_switches_per_min', format: (v) => v.toFixed(1) },
  { key: 'spending_quotient', format: (v) => Math.round(v) },
  { key: 'worker_production', inline: false },
  { key: 'float_share', format: percent },
];

const ROW_PX = 22;
const BAND_LABEL_PX = 16;
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

function Help({ text }) {
  return (
    <span className="workflow-help-wrap skill-help">
      <span className="workflow-metric-help">ⓘ</span>
      <span className="workflow-help-bubble">{text}</span>
    </span>
  );
}

function Strip({ entries, playerColor, describe, inline, hero }) {
  const t = useT();
  const positions = entries.map((e) => e.placement.position);
  const lo = Math.max(0, Math.floor(Math.min(...positions) - 0.25));
  const hi = Math.min(4, Math.max(lo + 1, Math.ceil(Math.max(...positions) + 0.25)));
  const band = (pos) => (pos - lo) / (hi - lo);
  // Markers keep a small margin from the strip's edges so a value at the end of the scale stays visible.
  const share = (pos) => 0.015 + 0.97 * band(pos);
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
    <div
      className={`skill-strip${hero ? ' skill-strip--hero' : ''}`}
      style={{ height: `${BAND_LABEL_PX + Math.max(1, rows.length) * ROW_PX + 4}px` }}
    >
      {tiers.map((i) => (
        <div key={i} className={`skill-band skill-band-${TIER_KEYS[i]}`} style={{ left: `${band(i) * 100}%`, width: `${(band(i + 1) - band(i)) * 100}%` }}>
          <span className="skill-band-label">{t(`skill.tier.${TIER_KEYS[i]}`)}</span>
        </div>
      ))}
      {placed.map(({ player, placement, x, row }) => {
        const right = x > 1 - LABEL_SHARE;
        const value = describe ? describe(placement) : '';
        return (
          <div
            key={player.player_id}
            className={`skill-marker${right ? ' skill-marker--right' : ''}`}
            style={{ left: `${x * 100}%`, top: `${BAND_LABEL_PX + row * ROW_PX}px` }}
            title={value ? `${player.name}: ${value}` : player.name}
          >
            <span className="skill-marker-dot" style={{ background: playerColor(player.color) }} />
            <span className="skill-player-label">
              {player.name}
              {inline && value ? <span className="skill-player-value"> {value}</span> : null}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export default function SkillTab({ game, players, playerColor }) {
  const t = useT();
  const status = game?.skill_status;
  if (status !== 'ok') {
    const key = { too_short: 'skill.status.tooShort', custom_rules: 'skill.status.customRules' }[status] || 'skill.status.unavailable';
    return (
      <div className="workflow-card skill-tab">
        <div className="chart-empty">{t(key)}</div>
      </div>
    );
  }
  const rated = (players || []).filter((p) => p.skill);
  const caveats = game?.skill_caveats || [];
  const entriesFor = (key) => rated.map((player) => ({ player, placement: placementOf(player, key) })).filter((e) => e.placement);
  const describeWorkers = (p) => (p.metric === 'larva_per_hatch'
    ? t('skill.value.larvaPerHatch', { value: p.value.toFixed(2) })
    : t('skill.value.hallBusy', { value: percent(p.value) }));
  const combined = entriesFor('combined');
  return (
    <div className="workflow-card skill-tab">
      {caveats.length ? (
        <div className="workflow-section-warning skill-caveat">
          {caveats.map((c) => t(`skill.caveat.${c}`)).join(' ')}
        </div>
      ) : null}
      {combined.length ? (
        <section className="skill-hero">
          <div className="skill-hero-head">
            <span className="skill-hero-title">{t('skill.metric.combined.title')}</span>
            <Help text={`${t('skill.metric.combined.hint')} ${t('skill.footnote')}`} />
          </div>
          <div className="skill-verdicts">
            {rated.filter((p) => p.skill?.combined).map((p) => (
              <span key={p.player_id} className="skill-verdict">
                <span className="skill-marker-dot" style={{ background: playerColor(p.color) }} />
                <span className="skill-verdict-name">{p.name}</span>
                <span className="skill-verdict-tier">{tierLabel(t, p.skill)}</span>
              </span>
            ))}
          </div>
          <Strip entries={combined} playerColor={playerColor} hero />
        </section>
      ) : null}
      <div className="skill-rows">
        {METRICS.map((m) => {
          const entries = entriesFor(m.key);
          if (!entries.length) return null;
          return (
            <React.Fragment key={m.key}>
              <div className="skill-row-label">
                <span>{t(`skill.metric.${m.key}.title`)}</span>
                <Help text={t(`skill.metric.${m.key}.hint`)} />
              </div>
              <Strip
                entries={entries}
                playerColor={playerColor}
                inline={m.inline !== false}
                describe={m.key === 'worker_production' ? describeWorkers : (p) => String(m.format(p.value))}
              />
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
}
