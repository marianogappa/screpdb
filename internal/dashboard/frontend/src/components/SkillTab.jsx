import React from 'react';
import { useT } from '../lib/i18nContext';

// SkillTab places each player on four tiers fitted to ladder MMR (internal/skill). Positions are on a shared
// tier axis, 0 to 4, where each whole number is a cut between tiers, so players of different races compare
// directly. Each strip zooms to the tiers its players span: in most games they are close.

export const TIER_KEYS = ['beginner', 'intermediate', 'advanced', 'pro'];

const percent = (v) => `${Math.round(v * 100)}%`;

const METRICS = [
  { key: 'eapm', format: (v) => Math.round(v) },
  { key: 'used_recalls_per_min', format: (v) => Math.round(v) },
  { key: 'check_recalls_per_min', format: (v) => Math.round(v) },
  { key: 'army_cadence', format: percent },
  { key: 'unspent_per_income', format: (v) => v.toFixed(2) },
  { key: 'production_idle_share', format: percent },
];

const ROW_PX = 22;
const BAND_LABEL_PX = 16;
// A label takes about this share of the strip; closer players go on the next row.
const LABEL_SHARE = 0.16;

const placementOf = (player, key) => {
  const metrics = player?.skill?.metrics || {};
  if (key === 'combined') return player?.skill?.combined ? { ...player.skill.combined, metric: 'combined' } : null;
  return metrics[key] ? { ...metrics[key], metric: key } : null;
};

const PRO_TIER = 3;

// The third of a tier the combined score falls in: "-", "" or "+". Pro stays one band.
export const tierThird = (combined) => {
  if (!combined || combined.tier >= PRO_TIER) return '';
  const within = Math.min(1, Math.max(0, combined.position - combined.tier));
  if (within < 1 / 3) return '-';
  return within < 2 / 3 ? '' : '+';
};

// The tier the combined score falls in, with the third of it.
export const tierLabel = (t, skill) => {
  const combined = skill?.combined;
  return combined ? `${t(`skill.tier.${TIER_KEYS[combined.tier]}`)}${tierThird(combined)}` : '';
};

function Help({ text }) {
  return (
    <span className="workflow-help-wrap skill-help">
      <span className="workflow-metric-help">ⓘ</span>
      <span className="workflow-help-bubble">{text}</span>
    </span>
  );
}

// tierRange is the span of whole tiers every chart shows: the same for all of them, so they line up, and
// only the tiers some player reaches on some chart.
const tierRange = (positions) => {
  const lo = Math.max(0, Math.floor(Math.min(...positions) - 0.25));
  const hi = Math.min(4, Math.max(lo + 1, Math.ceil(Math.max(...positions) + 0.25)));
  return [lo, hi];
};

function Strip({ entries, range, playerColor, describe, inline, hero, tierLabels, labelShare = LABEL_SHARE }) {
  const t = useT();
  const [lo, hi] = range;
  const band = (pos) => (pos - lo) / (hi - lo);
  // Markers keep a small margin from the strip's edges so a value at the end of the scale stays visible.
  const share = (pos) => 0.015 + 0.97 * band(pos);
  const rows = [];
  const placed = [...entries].sort((a, b) => a.placement.position - b.placement.position).map((e) => {
    const x = share(e.placement.position);
    let row = 0;
    while (rows[row] !== undefined && x - rows[row] < labelShare) row += 1;
    rows[row] = x;
    return { ...e, x, row };
  });
  const tiers = [];
  for (let i = lo; i < hi; i += 1) tiers.push(i);
  return (
    <div
      className={`skill-strip${hero ? ' skill-strip--hero' : ''}`}
      style={{ height: `${(tierLabels ? BAND_LABEL_PX : 4) + Math.max(1, rows.length) * ROW_PX + 4}px` }}
    >
      {tiers.map((i) => (
        <div key={i} className={`skill-band skill-band-${TIER_KEYS[i]}`} style={{ left: `${band(i) * 100}%`, width: `${(band(i + 1) - band(i)) * 100}%` }}>
          {tierLabels ? <span className="skill-band-label">{t(`skill.tier.${TIER_KEYS[i]}`)}</span> : null}
        </div>
      ))}
      {placed.map(({ player, placement, x, row }) => {
        const right = x > 1 - labelShare;
        const value = describe ? describe(placement) : '';
        return (
          <div
            key={player.player_id}
            className={`skill-marker${right ? ' skill-marker--right' : ''}`}
            style={{ left: `${x * 100}%`, top: `${(tierLabels ? BAND_LABEL_PX : 4) + row * ROW_PX}px` }}
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
  const combined = entriesFor('combined');
  const range = tierRange(['combined', ...METRICS.map((m) => m.key)].flatMap((k) => entriesFor(k).map((e) => e.placement.position)));
  return (
    <div className="workflow-card skill-tab">
      {caveats.length ? (
        <div className="workflow-section-warning skill-caveat">{t('skill.caveat')}</div>
      ) : null}
      <div className="skill-rows">
        {combined.length ? (
          <>
            <div className="skill-row-label skill-row-label--hero">
              <span>{t('skill.metric.combined.title')}</span>
              <Help text={`${t('skill.metric.combined.hint')} ${t('skill.footnote')}`} />
            </div>
            <Strip
              entries={combined}
              range={range}
              playerColor={playerColor}
              hero
              tierLabels
            />
          </>
        ) : null}
        {METRICS.filter((m) => entriesFor(m.key).length).map((m, index) => {
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
                range={range}
                playerColor={playerColor}
                inline
                tierLabels={index === 0}
                describe={(p) => String(m.format(p.value))}
              />
            </React.Fragment>
          );
        })}
      </div>
    </div>
  );
}
