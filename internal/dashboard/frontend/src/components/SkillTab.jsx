import React from 'react';
import { useT } from '../lib/i18nContext';
import SkillExplainerLink from './SkillExplainer';
import { spreadShares, tugOf } from '../lib/skillRank';

// SkillTab places each player on four tiers fitted to ladder MMR (internal/skill). Positions are on a shared
// tier axis, 0 to 4, where each whole number is a cut between tiers, so players of different races compare
// directly. Each strip zooms to the tiers its players span: in most games they are close.

export const TIER_KEYS = ['beginner', 'intermediate', 'advanced', 'pro'];

// LEFT_EARLY_MARK stands in for a player who left before the 8:00 Skill needs; game clock notation reads the
// same in every language.
export const LEFT_EARLY_MARK = '<8:00';

const METRICS = [
  { key: 'eapm' },
  { key: 'used_recalls_per_min' },
  { key: 'army_cadence' },
  { key: 'production_idle_share' },
  { key: 'idle_army_share' },
  { key: 'micro_actions_per_min' },
];

const ROW_PX = 22;
const BAND_LABEL_PX = 20;
// A strip that stops short of the scale's end shows this much of the next tier, faded, so a game where
// everyone is a Beginner still shows there is more above.
const PEEK = 0.3;
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

// SkillTierChip is a player's Skill tier for list columns; empty until they have enough rated games.
export function SkillTierChip({ placement }) {
  const t = useT();
  if (!placement) return null;
  return (
    <span className={`wpt-skill-tier wpt-skill-tier--static wpt-skill-tier--${TIER_KEYS[placement.tier]}`}>
      {tierLabel(t, { combined: placement })}
    </span>
  );
}

// MetricLabel is a submetric's row label.
function MetricLabel({ metric }) {
  const t = useT();
  return (
    <div className="skill-row-label skill-row-label--sub">
      <span className="skill-sub-title">{t(`skill.metric.${metric.key}.title`)}</span>
    </div>
  );
}

// HeroLabel is the Skill estimate's row label, with the way into "How Skill works" under it.
function HeroLabel({ model }) {
  const t = useT();
  return (
    <div className="skill-row-label skill-row-label--hero">
      <span>{t('skill.metric.combined.title')}</span>
      <SkillExplainerLink model={model} className="skill-explainer-link--sub" />
    </div>
  );
}

// SubmetricsHead introduces the metric rows as the parts the Skill estimate is made of, and marks which end
// of the strips is the worst and which the best, or which player pulls which way.
function SubmetricsHead({ pair }) {
  const t = useT();
  return (
    <>
      <div className="skill-submetrics-head skill-submetrics-head--label">{t('skill.submetrics.title')}</div>
      {pair ? (
        <div className="skill-compare-axis skill-tug-axis">
          {pair.map((p) => (
            <span key={p.key}>
              <span className="skill-compare-dot" style={{ background: p.color }} />
              {p.name}
            </span>
          ))}
        </div>
      ) : (
        <div className="skill-compare-axis" aria-hidden="true">
          <span>{t('skill.relative.worst')}</span>
          <span>{t('skill.relative.best')}</span>
        </div>
      )}
    </>
  );
}

// tierRange is the span of whole tiers every chart shows: the same for all of them, so they line up, and
// only the tiers some player reaches on some chart.
const tierRange = (positions) => {
  const lo = Math.max(0, Math.floor(Math.min(...positions) - 0.25));
  const hi = Math.min(4, Math.max(lo + 1, Math.ceil(Math.max(...positions) + 0.25)));
  return [lo, hi];
};

// Strip is one chart: players on the tier axis, zoomed to the tiers range spans, with a faded peek at the
// next tier on each side the scale goes on.
function Strip({ entries, range, playerColor, hero, tierLabels, labelShare = LABEL_SHARE }) {
  const t = useT();
  const lo = range[0] > 0 ? range[0] - PEEK : range[0];
  const hi = range[1] < TIER_KEYS.length ? range[1] + PEEK : range[1];
  const band = (pos) => (Math.min(hi, Math.max(lo, pos)) - lo) / (hi - lo);
  // Markers keep a small margin from the strip's edges so a value at the end of the scale stays visible.
  const share = (e) => 0.015 + 0.97 * band(e.placement.position);
  const rows = [];
  const placed = entries.map((e) => ({ ...e, x: share(e) })).sort((a, b) => a.x - b.x).map((e) => {
    const { x } = e;
    let row = 0;
    while (rows[row] !== undefined && x - rows[row] < labelShare) row += 1;
    rows[row] = x;
    return { ...e, x, row };
  });
  const tiers = [];
  for (let i = Math.floor(lo); i < Math.ceil(hi); i += 1) tiers.push(i);
  return (
    <div
      className={`skill-strip${hero ? ' skill-strip--hero' : ''}`}
      style={{ height: `${(tierLabels ? BAND_LABEL_PX : 4) + Math.max(1, rows.length) * ROW_PX + 4}px` }}
    >
      {tiers.map((i) => {
        const peek = i < range[0] ? ' skill-band--peek skill-band--peek-left' : i >= range[1] ? ' skill-band--peek skill-band--peek-right' : '';
        return (
          <div key={i} className={`skill-band skill-band-${TIER_KEYS[i]}${peek}`} style={{ left: `${band(i) * 100}%`, width: `${(band(i + 1) - band(i)) * 100}%` }}>
            {tierLabels ? <span className="skill-band-label">{t(`skill.tier.${TIER_KEYS[i]}`)}</span> : null}
          </div>
        );
      })}
      {placed.map(({ player, placement, x, row }) => {
        const right = x > 1 - labelShare;
        return (
          <div
            key={player.player_id}
            className={`skill-marker${right ? ' skill-marker--right' : ''}`}
            style={{ left: `${x * 100}%`, top: `${(tierLabels ? BAND_LABEL_PX : 4) + row * ROW_PX}px` }}
            title={player.name}
          >
            <span className="skill-marker-dot" style={{ background: playerColor(player.color) }} />
            <span className="skill-player-label">{player.name}</span>
          </div>
        );
      })}
    </div>
  );
}

const COMPARE_PAD_PX = 6;
const COMPARE_ROW_PX = 20;

// CompareStrip sets players against each other on one submetric, with no values: the worst on the left edge,
// the best on the right and the rest in proportion between them. focus highlights one player and dims the rest.
function CompareStrip({ entries, focus, onOpen }) {
  const [ref, width] = useWidth(720);
  const shares = spreadShares(entries.map((e) => e.placement.position));
  const inset = EDGE_PX + MARKER_PX / 2;
  const placed = packLabels(entries.map((e, i) => ({ ...e, share: (inset + shares[i] * (width - 2 * inset)) / width })), width, false);
  const rows = placed.reduce((n, e) => Math.max(n, e.row + 1), 1);
  return (
    <div ref={ref} className={`skill-compare${focus ? ' skill-compare--focus' : ''}`} style={{ height: `${2 * COMPARE_PAD_PX + rows * COMPARE_ROW_PX}px` }}>
      {placed.map(({ key, name, color, share: x, row, right }) => {
        const Tag = onOpen ? 'button' : 'div';
        return (
          <Tag
            key={key}
            {...(onOpen ? { type: 'button', onClick: () => onOpen(key) } : {})}
            className={`skill-compare-marker${right ? ' skill-compare-marker--right' : ''}${focus === key ? ' skill-compare-marker--focused' : ''}`}
            style={{ left: `${x * 100}%`, top: `${COMPARE_PAD_PX + row * COMPARE_ROW_PX}px` }}
          >
            <span className="skill-compare-dot" style={color ? { background: color } : undefined} />
            <span>{name}</span>
          </Tag>
        );
      })}
    </div>
  );
}

// TugStrip sets two players against each other on one submetric: a bar grows from the middle toward the one
// ahead, so it shows who leads and by how much, with no values.
function TugStrip({ left, right }) {
  const { side, share } = tugOf(left.placement.position, right.placement.position);
  const leader = side === 'right' ? right : left;
  return (
    <div className="skill-tug" title={leader.name}>
      <div className={`skill-tug-bar skill-tug-bar--${side}`} style={{ width: `${share * 100}%`, background: leader.color || undefined }} />
    </div>
  );
}

// Submetrics are the rows that compare players on each part of the Skill estimate. Two players, as in a 1v1,
// pull a tug bar each row instead: on a strip they would always sit on opposite edges, however close.
function Submetrics({ metrics, entriesFor, focus, onOpen, pair }) {
  const shown = metrics.map((m) => ({ m, entries: entriesFor(m.key) })).filter((r) => r.entries.length);
  if (!shown.length) return null;
  const pairOf = (entries) => (pair && entries.length === 2
    ? pair.map((p) => entries.find((e) => e.key === p.key)).filter(Boolean)
    : null);
  return (
    <div className="skill-subs">
      <SubmetricsHead pair={pair} />
      {shown.map(({ m, entries }) => {
        const tug = pairOf(entries);
        return (
          <React.Fragment key={m.key}>
            <MetricLabel metric={m} />
            {tug?.length === 2 ? <TugStrip left={tug[0]} right={tug[1]} /> : <CompareStrip entries={entries} focus={focus} onOpen={onOpen} />}
          </React.Fragment>
        );
      })}
    </div>
  );
}

// LeftEarly lists the players a rated game leaves out for leaving before 8:00, under the Skill estimate.
function LeftEarly({ players, playerColor }) {
  if (!players.length) return null;
  return (
    <div className="skill-left-early">
      {players.map((player) => (
        <span key={player.player_id} className="skill-left-early-player">
          <span className="skill-left-early-dot" style={{ borderColor: playerColor(player.color) }} />
          {player.name}
          <span className="skill-left-early-mark">{LEFT_EARLY_MARK}</span>
        </span>
      ))}
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
  const range = tierRange(combined.map((e) => e.placement.position));
  const asEntry = ({ player, placement }) => ({ key: player.player_id, name: player.name, color: playerColor(player.color), placement });
  const pair = rated.length === 2 ? rated.map((player) => asEntry({ player })) : null;
  return (
    <div className="workflow-card skill-tab">
      {caveats.length ? (
        <div className="workflow-section-warning skill-caveat">{t('skill.caveat')}</div>
      ) : null}
      {combined.length ? (
        <div className="skill-rows">
          <HeroLabel model={game?.skill_model} />
          <Strip
            entries={combined}
            range={range}
            playerColor={playerColor}
            hero
            tierLabels
          />
          <LeftEarly players={(players || []).filter((p) => p.skill_left_early)} playerColor={playerColor} />
          <Submetrics
            metrics={METRICS}
            entriesFor={(key) => entriesFor(key).map(asEntry)}
            pair={pair}
          />
        </div>
      ) : null}
    </div>
  );
}

const PILL_TIER_KEYS = { no_hotkeys: 0, never_checks_hotkeys: 0, banks_resources: 0, hotkey_cycler: 3, nonstop_production: 3 };

// The sparkline and the trend note need this many games: fewer is a handful of dots, not a direction.
const MIN_TREND_GAMES = 10;
const DAY_MS = 86400000;
// The note reads only the last three months, so it says where a player is heading now rather than since
// their first game. Those games must span a month, and it only calls a change when the fitted line moves
// a third of a tier: less is within what one tier's thirds already show.
const TREND_WINDOW_DAYS = 91;
const TREND_MIN_DAYS = 30;
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
  return { days, change: slope * days };
};

// skillTrendDirection reads the three months up to the latest game, the same games the Skill average reads.
export const skillTrendDirection = (timeline) => {
  const points = (timeline || []).map((p) => ({ ms: Date.parse(p.date), position: p.position }));
  const latest = points.reduce((m, p) => Math.max(m, p.ms), -Infinity);
  const recent = points.filter((p) => latest - p.ms <= TREND_WINDOW_DAYS * DAY_MS);
  if (recent.length < MIN_TREND_GAMES) return '';
  const { days, change } = trendOf(recent);
  if (days < TREND_MIN_DAYS) return '';
  if (change >= TREND_MIN_CHANGE) return 'up';
  if (change <= -TREND_MIN_CHANGE) return 'down';
  return 'flat';
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

// The card is one SVG with its colors inline, so the PNG it saves to is exactly what the page shows.
const CARD = {
  width: 360,
  pad: 24,
  bg: '#15171d',
  line: '#2a2e38',
  text: '#e5e7eb',
  muted: '#8b93a3',
  faint: '#4b5263',
  font: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Apple SD Gothic Neo', 'Malgun Gothic', sans-serif, 'Apple Color Emoji', 'Segoe UI Emoji', 'Noto Color Emoji'",
};
const TIER_COLORS = ['#94a3b8', '#60a5fa', '#a78bfa', '#f0c674'];
const TIER_TEXT = ['#cbd5e1', '#bfdbfe', '#ddd6fe', '#fde7b0'];
const TREND_COLORS = { up: '#86efac', down: '#fca5a5', flat: 'rgba(255,255,255,0.75)' };
const PILL_COLORS = { strong: '#86efac', plain: '#cbd5e1' };

let cardMeasure;
const textWidth = (text, size, weight = 400) => {
  if (typeof document === 'undefined') return text.length * size * 0.55;
  cardMeasure = cardMeasure || document.createElement('canvas').getContext('2d');
  if (!cardMeasure) return text.length * size * 0.55;
  cardMeasure.font = `${weight} ${size}px ${CARD.font}`;
  return cardMeasure.measureText(text).width;
};

// layoutPills puts the habit pills in centered rows that fit the card.
const layoutPills = (labels, size, maxWidth) => {
  const rows = [[]];
  let used = 0;
  labels.forEach((label) => {
    const w = textWidth(label.text, size) + 16;
    if (used && used + 6 + w > maxWidth) {
      rows.push([]);
      used = 0;
    }
    rows[rows.length - 1].push({ ...label, w });
    used += (used ? 6 : 0) + w;
  });
  return rows.filter((r) => r.length);
};

// Sparkline is the Skill estimate over time on the card, with the games the average reads shaded.
function Sparkline({ timeline, windowFrom, top, height }) {
  const left = CARD.pad, right = CARD.width - CARD.pad;
  const points = timeline.map((p) => ({ ms: Date.parse(p.date), position: p.position }));
  const first = points[0].ms, last = points[points.length - 1].ms;
  const span = Math.max(DAY_MS, last - first);
  const x = (ms) => left + ((ms - first) / span) * (right - left);
  const [lo, hi] = tierRange(points.map((p) => p.position));
  const y = (pos) => top + (1 - (Math.max(lo, Math.min(hi, pos)) - lo) / (hi - lo)) * height;
  const windowX = windowFrom ? x(Math.max(first, Date.parse(windowFrom))) : null;
  return (
    <g>
      <rect x={left} y={top} width={right - left} height={height} rx="4" fill="rgba(255,255,255,0.025)" />
      {windowX !== null ? <rect x={windowX} y={top} width={right - windowX} height={height} fill="rgba(255,255,255,0.06)" /> : null}
      {points.map((p, i) => <circle key={i} cx={x(p.ms)} cy={y(p.position)} r="1.5" fill="rgba(203,213,225,0.35)" />)}
      <path d={smoothPath(points, first, span, x, y)} fill="none" stroke="#60a5fa" strokeWidth="1.75" strokeLinejoin="round" strokeLinecap="round" />
    </g>
  );
}

// MetricBar is one submetric on the four tiers, with no values: a dot where the player sits.
function MetricBar({ label, placement, top, barLeft }) {
  const left = CARD.pad, right = CARD.width - CARD.pad;
  const segment = (right - barLeft) / TIER_KEYS.length;
  const pos = Math.min(TIER_KEYS.length, Math.max(0, placement.position));
  return (
    <g>
      <text x={left} y={top + 4} fill={CARD.muted} fontSize="11" dominantBaseline="middle">{label}</text>
      {TIER_KEYS.map((key, i) => (
        <rect key={key} x={barLeft + i * segment} y={top} width={segment - 1} height="8" fill={TIER_COLORS[i]} opacity="0.2" />
      ))}
      <circle cx={barLeft + (pos / TIER_KEYS.length) * (right - barLeft)} cy={top + 4} r="5" fill={TIER_TEXT[placement.tier]} stroke={CARD.bg} strokeWidth="2" />
    </g>
  );
}

// SkillCard is a player's Skill as one card: their tier, races and APM, how they moved, the six metrics behind
// the tier and their habits. A player with too few games for a direction gets the card without the sparkline
// and the trend; onDownload, given only on the player's own page, saves it as a PNG.
export function SkillCard({ summary, name, races, apm, onDownload }) {
  const t = useT();
  const svgRef = React.useRef(null);
  if (!summary?.combined) return null;
  const { combined } = summary;
  const timeline = summary.timeline || [];
  const charted = timeline.length >= MIN_TREND_GAMES;
  const direction = charted ? skillTrendDirection(timeline) : '';
  const lang = typeof document !== 'undefined' ? document.documentElement.lang || undefined : undefined;
  const latest = timeline.length ? new Date(timeline[timeline.length - 1].date) : new Date();
  const asOf = latest.toLocaleDateString(lang, { month: 'short', year: 'numeric' });
  const metrics = METRICS.filter((m) => summary.metrics?.[m.key]).map((m) => ({ ...m, label: t(`skill.metric.${m.key}.title`) }));
  const barLeft = Math.min(CARD.width / 2, CARD.pad + Math.max(0, ...metrics.map((m) => textWidth(m.label, 11))) + 14);
  const pills = layoutPills((summary.pills || []).map((pill) => ({
    key: pill.key,
    text: pill.key === 'no_hotkeys' ? t('server.marker.never_used_hotkeys.summary_player.label') : t(`skill.pill.${pill.key}`),
    hint: t(`skill.pill.${pill.key}.hint`, { games: pill.games, of: pill.of }),
    color: PILL_COLORS[PILL_TIER_KEYS[pill.key] === 3 ? 'strong' : 'plain'],
  })), 10.5, CARD.width - 2 * CARD.pad);

  const mid = CARD.width / 2;
  const apmText = apm ? `${Math.round(apm)} APM` : '';
  const ICON = 18;
  const identityWidth = races.length * (ICON + 4) + (apmText ? (races.length ? 6 : 0) + textWidth(apmText, 12) : 0);
  let identityX = mid - identityWidth / 2;
  const medalY = 128, medalR = 50;
  let y = medalY + medalR + 22;
  const sparkTop = y;
  if (charted) y += 46 + 16;
  else y += 4;
  const metricTop = y;
  y += metrics.length * 22;
  const pillTop = y + 6;
  if (pills.length) y = pillTop + pills.length * 26;
  const footerY = y + 18;
  const height = footerY + 16;

  return (
    <div className="skill-card">
      <svg ref={svgRef} viewBox={`0 0 ${CARD.width} ${height}`} width={CARD.width} height={height} fontFamily={CARD.font} role="img" aria-label={`${t('game.tabs.skill')}: ${tierLabel(t, summary)}`}>
        <rect x="0.5" y="0.5" width={CARD.width - 1} height={height - 1} rx="14" fill={CARD.bg} stroke={CARD.line} />
        <text x={mid} y="38" fill={CARD.text} fontSize="18" fontWeight="600" textAnchor="middle">{name}</text>
        {races.map((race) => {
          const x = identityX;
          identityX += ICON + 4;
          return <image key={race.race} href={race.icon} x={x} y="52" width={ICON} height={ICON} />;
        })}
        {apmText ? <text x={identityX + (races.length ? 2 : 0)} y="61" fill={CARD.muted} fontSize="12" dominantBaseline="middle">{apmText}</text> : null}
        <circle cx={mid} cy={medalY} r={medalR} fill={TIER_COLORS[combined.tier]} fillOpacity="0.1" stroke={TIER_COLORS[combined.tier]} strokeOpacity="0.65" strokeWidth="3" />
        <text x={mid} y={direction ? medalY - 3 : medalY + 5} fill={TIER_TEXT[combined.tier]} fontSize="16" fontWeight="600" textAnchor="middle">{tierLabel(t, summary)}</text>
        {direction ? <text x={mid} y={medalY + 16} fill={TREND_COLORS[direction]} fontSize="11" fontWeight="600" textAnchor="middle">{t(`skill.trend.${direction}`)}</text> : null}
        {charted ? <Sparkline timeline={timeline} windowFrom={summary.window_from} top={sparkTop} height={46} /> : null}
        {metrics.map((m, i) => (
          <MetricBar key={m.key} label={m.label} placement={summary.metrics[m.key]} top={metricTop + i * 22} barLeft={barLeft} />
        ))}
        {pills.map((row, r) => {
          const rowWidth = row.reduce((sum, p) => sum + p.w, 0) + 6 * (row.length - 1);
          let px = mid - rowWidth / 2;
          return row.map((pill) => {
            const x = px;
            px += pill.w + 6;
            return (
              <g key={pill.key}>
                <title>{pill.hint}</title>
                <rect x={x} y={pillTop + r * 26} width={pill.w} height="20" rx="10" fill="none" stroke={pill.color} strokeOpacity="0.45" />
                <text x={x + pill.w / 2} y={pillTop + r * 26 + 10} fill={pill.color} fontSize="10.5" textAnchor="middle" dominantBaseline="central">{pill.text}</text>
              </g>
            );
          });
        })}
        <text x={CARD.pad} y={footerY} fill={CARD.faint} fontSize="10">{asOf}</text>
        <text x={mid} y={footerY} fill={CARD.faint} fontSize="10" textAnchor="middle">{t('skill.player.basis', { count: summary.games })}</text>
        <text x={CARD.width - CARD.pad} y={footerY} fill={CARD.faint} fontSize="10" textAnchor="end">screpdb</text>
      </svg>
      {onDownload ? (
        <button type="button" className="skill-card-download" aria-label={`${t('game.tabs.skill')} PNG`} title={`${t('game.tabs.skill')} PNG`} onClick={() => onDownload(svgRef.current, latest)}>
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="M12 4v11M7 10l5 5 5-5M5 19h14" />
          </svg>
        </button>
      ) : null}
    </div>
  );
}

const FULL_RANGE = [0, 4];

const LIBRARY_PLAYERS_SHOWN = 30;
const MARKER_PX = 16;
const LABEL_GAP_PX = 10;
const EDGE_PX = 6;
const POPULATION_ROW_PX = 19;

let measureContext;
const labelWidth = (text, hero) => {
  if (typeof document === 'undefined') return text.length * 7;
  measureContext = measureContext || document.createElement('canvas').getContext('2d');
  if (!measureContext) return text.length * 7;
  const family = getComputedStyle(document.body).fontFamily || 'sans-serif';
  measureContext.font = `${hero ? 600 : 400} ${hero ? 13 : 12}px ${family}`;
  return measureContext.measureText(text).width;
};

// packLabels gives every marker the first row where its label fits beside it, on its right or else on its
// left, without running into another label or past the strip's edges.
const packLabels = (entries, width, hero) => {
  const rows = [];
  const free = (row, start, end) => start >= EDGE_PX && end <= width - EDGE_PX
    && (rows[row] || []).every(([s, e]) => end + LABEL_GAP_PX <= s || start >= e + LABEL_GAP_PX);
  return [...entries].sort((a, b) => a.placement.position - b.placement.position).map((e) => {
    const x = e.share * width;
    const w = MARKER_PX + labelWidth(e.name, hero);
    const sides = [[false, x - MARKER_PX / 2, x + w], [true, x - w, x + MARKER_PX / 2]];
    for (let row = 0; ; row += 1) {
      const fit = sides.find(([, start, end]) => free(row, start, end));
      if (fit || row > entries.length) {
        const [right, start, end] = fit || sides[x + w > width - EDGE_PX ? 1 : 0];
        rows[row] = [...(rows[row] || []), [start, end]];
        return { ...e, row, right };
      }
    }
  });
};

// PopulationStrip is one Skill metric across the library: the four tiers, the library's players, a dotted line
// at the median of their averages and a star per built-in progamer.
function PopulationStrip({ median, players, stars, describe, hero, tierLabels, focus, onFocus, onOpenPlayer }) {
  const t = useT();
  const [ref, width] = useWidth(900);
  const [lo, hi] = FULL_RANGE;
  const band = (pos) => Math.min(1, Math.max(0, (pos - lo) / (hi - lo)));
  const share = (pos) => 0.015 + 0.97 * band(pos);
  const entries = [
    ...players.map((p) => ({ ...p, pro: false, share: share(p.placement.position) })),
    ...stars.map((p) => ({ ...p, pro: true, share: share(p.placement.position) })),
  ].filter((e) => hero || !focus || e.key === focus);
  const placed = packLabels(entries, width, hero);
  const rows = placed.reduce((n, e) => Math.max(n, e.row + 1), 1);
  const top = tierLabels ? BAND_LABEL_PX + 4 : 4;
  const medianText = median ? t('skill.players.median', { count: median.players }) : '';
  return (
    <div
      ref={ref}
      className={`skill-strip skill-strip--population${hero ? ' skill-strip--hero' : ''}${focus ? ' skill-strip--focus' : ''}`}
      style={{ height: `${top + rows * POPULATION_ROW_PX + 6}px` }}
    >
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
      {placed.map(({ key, name, placement, share: x, row, right, pro }) => {
        const value = describe ? describe(placement) : '';
        const focused = focus === key;
        return (
          <button
            type="button"
            key={key}
            className={`skill-marker skill-marker--population${pro ? ' skill-marker--star' : ' skill-marker--library'}${right ? ' skill-marker--right' : ''}${focused ? ' skill-marker--focused' : ''}`}
            style={{ left: `${x * 100}%`, top: `${top + row * POPULATION_ROW_PX}px` }}
            title={value ? `${name}: ${value}` : name}
            onMouseEnter={hero ? () => onFocus?.(key) : undefined}
            onMouseLeave={hero ? () => onFocus?.(null) : undefined}
            onFocus={hero ? () => onFocus?.(key) : undefined}
            onBlur={hero ? () => onFocus?.(null) : undefined}
            onClick={() => onOpenPlayer?.(key)}
          >
            {pro ? <span className="skill-star" aria-hidden="true">★</span> : <span className="skill-marker-dot skill-library-dot" />}
            <span className="skill-player-label">
              {name}
              {focused && value ? <span className="skill-player-value"> {value}</span> : null}
            </span>
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
  const [focus, setFocus] = React.useState(null);
  if (loading && !data) return <div className="workflow-card skill-tab"><div className="chart-empty">{t('players.loading')}</div></div>;
  if (error) return <div className="workflow-card skill-tab"><div className="chart-empty">{error}</div></div>;
  const featured = data?.featured_players || [];
  const pros = showFeatured ? featured : [];
  const starsFor = (key) => pros
    .map((p) => ({ key: p.player_key, name: p.player_name, placement: key === 'combined' ? p.combined : p.metrics?.[key] }))
    .filter((s) => s.placement);
  const library = [...(data?.library_players || [])].sort((a, b) => b.games - a.games).slice(0, LIBRARY_PLAYERS_SHOWN);
  const playersFor = (key) => library
    .map((p) => ({ key: p.player_key, name: p.player_name, placement: key === 'combined' ? p.combined : p.metrics?.[key] }))
    .filter((d) => d.placement);
  if (!data?.combined && !pros.length) {
    return (
      <div className="workflow-card skill-tab">
        <div className="chart-empty">{emptyNotice || t('skill.player.none')}</div>
        {featured.length ? <div className="skill-players-legend"><span className="skill-players-legend-toggle">{featuredToggle(featured.length)}</span></div> : null}
      </div>
    );
  }
  return (
    <div className="workflow-card skill-tab skill-players-tab">
      <div className="skill-rows">
        <HeroLabel model={data?.model} />
        <PopulationStrip median={data?.combined} players={playersFor('combined')} stars={starsFor('combined')} hero tierLabels focus={focus} onFocus={setFocus} onOpenPlayer={onOpenPlayer} />
        <div className="skill-players-legend skill-players-legend--under">
          {data?.combined ? (
            <span className="skill-players-legend-median">{t('skill.players.median', { count: data.combined.players })}</span>
          ) : null}
          {pros.length ? <span>{t('players.featuredNote')}</span> : null}
          {featured.length ? <span className="skill-players-legend-toggle">{featuredToggle(featured.length)}</span> : null}
        </div>
        <Submetrics metrics={METRICS} entriesFor={playersFor} focus={focus} onOpen={onOpenPlayer} />
      </div>
    </div>
  );
}
