import React from 'react';
import { useT } from '../lib/i18nContext';
import SkillExplainerLink from './SkillExplainer';

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
      <SkillExplainerLink model={game?.skill_model} className="skill-explainer-link--corner" />
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

const PILL_TIER_KEYS = { no_hotkeys: 0, never_checks_hotkeys: 0, banks_resources: 0, hotkey_cycler: 3, nonstop_production: 3 };

// PlayerSkillRow is the player summary's one line of Skill: the average tier and how many games it rests on.
// It opens the player's Skill tab; onHover lets the page point at that tab.
export function PlayerSkillRow({ summary, onOpen, onHover }) {
  const t = useT();
  if (!summary?.combined) return null;
  return (
    <button
      type="button"
      className="skill-player-row"
      onClick={onOpen}
      onMouseEnter={() => onHover?.(true)}
      onMouseLeave={() => onHover?.(false)}
      onFocus={() => onHover?.(true)}
      onBlur={() => onHover?.(false)}
    >
      <span className="skill-player-row-text">
        <span className="skill-player-row-label">{t('game.tabs.skill')}</span>
        <span className="skill-player-row-basis">{t('skill.player.basis', { count: summary.games })}</span>
      </span>
      <span className={`skill-player-row-tier wpt-skill-tier--${TIER_KEYS[summary.combined.tier]}`}>{tierLabel(t, summary)}</span>
      <span className="skill-player-row-open" aria-hidden="true">›</span>
    </button>
  );
}

// The chart and its trend need this many games: fewer is a handful of dots, not a direction.
const MIN_TREND_GAMES = 10;
const DAY_MS = 86400000;
// A trend needs two months of games, and is only called a change when the line moves a third of a tier
// across them: less is within what one tier's thirds already show.
const TREND_MIN_DAYS = 60;
const TREND_MIN_CHANGE = 1 / 3;

function useWidth(fallback) {
  const ref = React.useRef(null);
  const [width, setWidth] = React.useState(fallback);
  React.useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return undefined;
    if (el.clientWidth) setWidth(Math.max(320, el.clientWidth));
    if (typeof ResizeObserver === 'undefined') return undefined;
    const ro = new ResizeObserver(([entry]) => setWidth(Math.max(320, Math.round(entry.contentRect.width))));
    ro.observe(el);
    return () => ro.disconnect();
  }, []);
  return [ref, width];
}

// trendOf fits a straight line to position over time; slope is in tiers per 30 days.
const trendOf = (points) => {
  const n = points.length;
  const xs = points.map((p) => p.ms / DAY_MS);
  const mx = xs.reduce((a, b) => a + b, 0) / n;
  const my = points.reduce((a, p) => a + p.position, 0) / n;
  let sxy = 0, sxx = 0;
  points.forEach((p, i) => { sxy += (xs[i] - mx) * (p.position - my); sxx += (xs[i] - mx) ** 2; });
  const slope = sxx > 0 ? sxy / sxx : 0;
  const days = n ? (points[n - 1].ms - points[0].ms) / DAY_MS : 0;
  return { at: (ms) => my + slope * (ms / DAY_MS - mx), days, change: slope * days };
};

export const skillTrendDirection = (timeline) => {
  if (!timeline || timeline.length < MIN_TREND_GAMES) return '';
  const { days, change } = trendOf(timeline.map((p) => ({ ms: Date.parse(p.date), position: p.position })));
  if (days < TREND_MIN_DAYS) return '';
  if (change >= TREND_MIN_CHANGE) return 'up';
  if (change <= -TREND_MIN_CHANGE) return 'down';
  return 'flat';
};

const monthTicks = (fromMs, toMs) => {
  const start = new Date(fromMs);
  const months = (new Date(toMs).getFullYear() - start.getFullYear()) * 12 + new Date(toMs).getMonth() - start.getMonth();
  const step = [1, 2, 3, 6, 12].find((s) => months / s <= 7) || 12;
  const out = [];
  for (let d = new Date(start.getFullYear(), start.getMonth() + 1, 1); d.getTime() <= toMs; d = new Date(d.getFullYear(), d.getMonth() + step, 1)) {
    out.push(d);
  }
  return out;
};

const SMOOTH_SAMPLES = 90;

// smoothPath is the running average as a curve over time: a Gaussian-weighted average of nearby games,
// about a month wide, so a session of ten games on one evening is one point rather than a cliff. It breaks
// where there are no games nearby.
function smoothPath(points, first, span, x, y) {
  const width = Math.max(14 * DAY_MS, span / 10);
  let d = '';
  let drawing = false;
  for (let k = 0; k <= SMOOTH_SAMPLES; k++) {
    const at = first + (span * k) / SMOOTH_SAMPLES;
    let sw = 0, sp = 0;
    for (const p of points) {
      const w = Math.exp(-0.5 * ((p.ms - at) / width) ** 2);
      sw += w;
      sp += w * p.position;
    }
    if (sw < 1) {
      drawing = false;
      continue;
    }
    d += `${drawing ? 'L' : 'M'}${x(at).toFixed(1)},${y(sp / sw).toFixed(1)}`;
    drawing = true;
  }
  return d;
}

// SkillTrend plots every rated game by date on the tier axis, with a running average and a straight trend
// line.
function SkillTrend({ timeline }) {
  const t = useT();
  const [ref, W] = useWidth(720);
  const H = 176, L = 84, R = 10, T = 10, B = 24;
  const points = timeline.map((p) => ({ ...p, ms: Date.parse(p.date) }));
  const first = points[0].ms, last = points[points.length - 1].ms;
  const span = Math.max(DAY_MS, last - first);
  const x = (ms) => L + ((ms - first) / span) * (W - L - R);
  const [lo, hi] = tierRange(points.map((p) => p.position));
  const y = (pos) => T + (1 - (Math.max(lo, Math.min(hi, pos)) - lo) / (hi - lo)) * (H - T - B);
  const avgPath = smoothPath(points, first, span, x, y);
  const trend = trendOf(points);
  const lang = typeof document !== 'undefined' ? document.documentElement.lang || undefined : undefined;
  const tickLabel = (d, i) => d.toLocaleDateString(lang, i === 0 || d.getMonth() === 0 ? { month: 'short', year: 'numeric' } : { month: 'short' });
  const dateLabel = (ms) => new Date(ms).toLocaleDateString(lang, { day: 'numeric', month: 'short', year: 'numeric' });
  const bands = TIER_KEYS.map((key, i) => ({ key, i })).filter(({ i }) => i >= lo && i < hi);
  return (
    <div className="skill-trend-card">
      <div className="skill-trend-head">
        <h3>{t('skill.chart.title')}</h3>
        <div className="skill-trend-legend">
          <span><i className="skill-trend-key skill-trend-key--avg" />{t('skill.chart.average')}</span>
          {trend.days >= TREND_MIN_DAYS ? <span><i className="skill-trend-key skill-trend-key--trend" />{t('skill.chart.trend')}</span> : null}
        </div>
      </div>
      <div ref={ref} className="skill-trend-plot">
        <svg width={W} height={H} viewBox={`0 0 ${W} ${H}`} role="img" aria-label={t('skill.chart.title')}>
          {bands.map(({ key, i }) => (
            <rect key={key} x={L} y={y(i + 1)} width={W - L - R} height={y(i) - y(i + 1)} className={`skill-trend-band skill-trend-band--${key}`} />
          ))}
          {bands.map(({ key, i }) => (
            <text key={`l-${key}`} x={L - 10} y={(y(i) + y(i + 1)) / 2} className="skill-trend-axis" textAnchor="end" dominantBaseline="middle">{t(`skill.tier.${key}`)}</text>
          ))}
          {Array.from({ length: hi - lo + 1 }, (_, k) => lo + k).map((v) => (
            <line key={`g-${v}`} x1={L} x2={W - R} y1={y(v)} y2={y(v)} className={v === lo || v === hi ? 'skill-trend-frame' : 'skill-trend-cut'} />
          ))}
          {monthTicks(first, last).map((d, i) => (
            <g key={d.getTime()}>
              <line x1={x(d.getTime())} x2={x(d.getTime())} y1={H - B} y2={H - B + 4} className="skill-trend-frame" />
              <text x={x(d.getTime())} y={H - 6} className="skill-trend-axis" textAnchor="middle">{tickLabel(d, i)}</text>
            </g>
          ))}
          {points.map((p, i) => (
            <circle key={`${p.replay_id}-${i}`} cx={x(p.ms)} cy={y(p.position)} r="2" className="skill-trend-dot">
              <title>{dateLabel(p.ms)}</title>
            </circle>
          ))}
          <path d={avgPath} className="skill-trend-avg" />
          {trend.days >= TREND_MIN_DAYS ? (
            <line x1={x(first)} x2={x(last)} y1={y(trend.at(first))} y2={y(trend.at(last))} className="skill-trend-fit" />
          ) : null}
        </svg>
      </div>
    </div>
  );
}

// PlayerSkillTab is the player page's Skill tab: the player's tier up top, how it moved over time, then the
// six metrics behind it.
export function PlayerSkillTab({ summary, name, color }) {
  const t = useT();
  if (!summary?.combined) return null;
  const player = { player_id: 'player', name, color };
  const entryFor = (placement) => (placement ? [{ player, placement }] : []);
  const placements = [summary.combined, ...METRICS.map((m) => summary.metrics?.[m.key]).filter(Boolean)];
  const range = tierRange(placements.map((p) => p.position));
  const plain = () => color || '#9ca3af';
  const timeline = summary.timeline || [];
  const direction = skillTrendDirection(timeline);
  return (
    <div className="workflow-card skill-player-tab">
      <div className="skill-hero">
        <span className={`skill-hero-tier wpt-skill-tier--${TIER_KEYS[summary.combined.tier]}`}>{tierLabel(t, summary)}</span>
        <div className="skill-hero-facts">
          <span>{t('skill.player.basis', { count: summary.games })}</span>
          {direction ? <span className={`skill-hero-trend skill-hero-trend--${direction}`}>{t(`skill.trend.${direction}`)}</span> : null}
        </div>
        <SkillExplainerLink model={summary.model} className="skill-hero-explain" />
      </div>
      {summary.pills?.length ? (
        <div className="skill-player-pills">
          {summary.pills.map((pill) => (
            <span
              key={pill.key}
              className={`workflow-pattern-pill skill-habit-pill skill-habit-pill--${PILL_TIER_KEYS[pill.key] === 3 ? 'strong' : 'plain'}`}
              title={t(`skill.pill.${pill.key}.hint`, { games: pill.games, of: pill.of })}
            >
              {t(`skill.pill.${pill.key}`)}
            </span>
          ))}
        </div>
      ) : null}
      {timeline.length >= MIN_TREND_GAMES ? <SkillTrend timeline={timeline} /> : null}
      <div className="skill-rows skill-player-breakdown">
        {METRICS.filter((m) => summary.metrics?.[m.key]).map((m, index) => (
          <React.Fragment key={m.key}>
            <div className="skill-row-label">
              <span>{t(`skill.metric.${m.key}.title`)}</span>
              <Help text={t(`skill.metric.${m.key}.hint`)} />
            </div>
            <Strip entries={entryFor(summary.metrics[m.key])} range={range} playerColor={plain} inline tierLabels={index === 0} describe={(p) => String(m.format(p.value))} />
          </React.Fragment>
        ))}
      </div>
    </div>
  );
}

const FULL_RANGE = [0, 4];

const DOT_SHARE = 0.012;
const DOT_ROW_PX = 9;
const DOT_ROWS = 4;

// PopulationStrip is one Skill metric across the library: the four tiers, a dot per library player, a dotted
// line at the median of their averages and a star per built-in progamer.
function PopulationStrip({ median, dots, stars, describe, hero, tierLabels, onOpenPlayer }) {
  const t = useT();
  const [lo, hi] = FULL_RANGE;
  const band = (pos) => (pos - lo) / (hi - lo);
  const share = (pos) => 0.015 + 0.97 * band(pos);
  const rows = [];
  const placed = [...stars].sort((a, b) => a.placement.position - b.placement.position).map((s) => {
    const x = share(s.placement.position);
    let row = 0;
    while (rows[row] !== undefined && x - rows[row] < LABEL_SHARE) row += 1;
    rows[row] = x;
    return { ...s, x, row };
  });
  const dotRows = [];
  const placedDots = [...dots].sort((a, b) => a.placement.position - b.placement.position).map((d) => {
    const x = share(d.placement.position);
    let row = 0;
    while (row < DOT_ROWS - 1 && dotRows[row] !== undefined && x - dotRows[row] < DOT_SHARE) row += 1;
    dotRows[row] = x;
    return { ...d, x, row };
  });
  const labelTop = tierLabels ? BAND_LABEL_PX : 4;
  const dotLane = dotRows.length ? dotRows.length * DOT_ROW_PX + 4 : 0;
  const top = labelTop + dotLane;
  const medianText = median ? t('skill.players.median', { count: median.players }) : '';
  return (
    <div className={`skill-strip skill-strip--population${hero ? ' skill-strip--hero' : ''}`} style={{ height: `${top + (rows.length || (dotLane ? 0 : 1)) * ROW_PX + 4}px` }}>
      {TIER_KEYS.map((key, i) => (
        <div key={key} className={`skill-band skill-band-${key}`} style={{ left: `${band(i) * 100}%`, width: `${(band(i + 1) - band(i)) * 100}%` }}>
          {tierLabels ? <span className="skill-band-label">{t(`skill.tier.${key}`)}</span> : null}
        </div>
      ))}
      {median ? (
        <div
          className="skill-median"
          style={{ left: `${share(median.position) * 100}%` }}
          title={describe ? `${medianText}: ${describe(median)}` : medianText}
        />
      ) : null}
      {placedDots.map(({ key, name, placement, x, row }) => {
        const value = describe ? describe(placement) : '';
        return (
          <button
            type="button"
            key={key}
            className="skill-dot"
            style={{ left: `${x * 100}%`, top: `${labelTop + 2 + row * DOT_ROW_PX}px` }}
            title={value ? `${name}: ${value}` : name}
            aria-label={name}
            onClick={() => onOpenPlayer?.(key)}
          />
        );
      })}
      {placed.map(({ key, name, placement, x, row }) => {
        const right = x > 1 - LABEL_SHARE;
        const value = describe ? describe(placement) : '';
        return (
          <button
            type="button"
            key={key}
            className={`skill-marker skill-marker--star${right ? ' skill-marker--right' : ''}`}
            style={{ left: `${x * 100}%`, top: `${top + row * ROW_PX}px` }}
            title={value ? `${name}: ${value}` : name}
            onClick={() => onOpenPlayer?.(key)}
          >
            <span className="skill-star" aria-hidden="true">★</span>
            <span className="skill-player-label">{name}</span>
          </button>
        );
      })}
    </div>
  );
}

// PlayersSkillTab is the Players page Skill tab: where each of the library's players sits on the Skill tiers,
// with the median of their averages, beside the built-in progamers.
export function PlayersSkillTab({ data, loading, error, emptyNotice, showFeatured, featuredToggle, onOpenPlayer }) {
  const t = useT();
  if (loading && !data) return <div className="workflow-card skill-tab"><div className="chart-empty">{t('players.loading')}</div></div>;
  if (error) return <div className="workflow-card skill-tab"><div className="chart-empty">{error}</div></div>;
  const featured = data?.featured_players || [];
  const pros = showFeatured ? featured : [];
  const starsFor = (key) => pros
    .map((p) => ({ key: p.player_key, name: p.player_name, placement: key === 'combined' ? p.combined : p.metrics?.[key] }))
    .filter((s) => s.placement);
  const dotsFor = (key) => (data?.library_players || [])
    .map((p) => ({ key: p.player_key, name: p.player_name, placement: key === 'combined' ? p.combined : p.metrics?.[key] }))
    .filter((d) => d.placement);
  const metrics = METRICS.filter((m) => data?.metrics?.[m.key] || starsFor(m.key).length);
  if (!data?.combined && !pros.length) {
    return <div className="workflow-card skill-tab"><div className="chart-empty">{emptyNotice || t('skill.player.none')}</div></div>;
  }
  return (
    <div className="workflow-card skill-tab skill-players-tab">
      <SkillExplainerLink model={data?.model} className="skill-explainer-link--corner" />
      <div className="skill-rows">
        <div className="skill-row-label skill-row-label--hero">
          <span>{t('skill.metric.combined.title')}</span>
          <Help text={`${t('skill.metric.combined.hint')} ${t('skill.footnote')}`} />
        </div>
        <PopulationStrip median={data?.combined} dots={dotsFor('combined')} stars={starsFor('combined')} hero tierLabels onOpenPlayer={onOpenPlayer} />
        {metrics.map((m, index) => (
          <React.Fragment key={m.key}>
            <div className="skill-row-label">
              <span>{t(`skill.metric.${m.key}.title`)}</span>
              <Help text={t(`skill.metric.${m.key}.hint`)} />
            </div>
            <PopulationStrip
              median={data?.metrics?.[m.key]}
              dots={dotsFor(m.key)}
              stars={starsFor(m.key)}
              tierLabels={index === 0}
              describe={(p) => String(m.format(p.value))}
              onOpenPlayer={onOpenPlayer}
            />
          </React.Fragment>
        ))}
      </div>
      <div className="skill-players-legend">
        {data?.combined ? (
          <span className="skill-players-legend-median">{t('skill.players.median', { count: data.combined.players })}</span>
        ) : null}
        {pros.length ? <span>{t('players.featuredNote')}</span> : null}
        {featured.length ? <span className="skill-players-legend-toggle">{featuredToggle(featured.length)}</span> : null}
      </div>
    </div>
  );
}
