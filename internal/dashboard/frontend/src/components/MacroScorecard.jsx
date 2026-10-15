import React from 'react';
import { useT } from '../lib/i18nContext';
import { formatDuration } from '../lib/formatters';

const percent = (v) => `${Math.round(v * 100)}%`;
const whole = (v) => String(Math.round(v));
const oneDecimal = (v) => (Math.round(v * 10) / 10).toFixed(1);

// best marks the better value of a row so a pace race reads at a glance; ties and lone values mark nothing.
const best = (values, better) => {
  const present = values.filter((v) => v != null);
  if (present.length < 2) return null;
  const top = better === 'high' ? Math.max(...present) : Math.min(...present);
  return Number.isFinite(top) && present.filter((v) => v === top).length === 1 ? top : null;
};

function IncomeSparkline({ curve, color }) {
  if (!Array.isArray(curve) || curve.length < 2) return null;
  const w = 120;
  const h = 28;
  const top = Math.max(...curve, 1);
  const points = curve.map((v, i) => `${(i / (curve.length - 1)) * w},${h - (v / top) * (h - 2) - 1}`).join(' ');
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="macro-sparkline" aria-hidden="true">
      <polyline points={points} fill="none" stroke={color || 'currentColor'} strokeWidth="1.5" />
    </svg>
  );
}

// ProRangeMarker places the player's value on the pros' spread for their race: whiskers from the 10th to
// the 90th percentile, a box over the middle half, and a dot for the player. A value past the whiskers
// sits on the edge as a hollow dot.
function ProRangeMarker({ range, value, color, title }) {
  if (!range) return null;
  const w = 72;
  const h = 10;
  const pad = 4;
  const spread = range.p90 - range.p10 || Math.abs(range.p50) || 1;
  const lo = range.p10 - spread * 0.25;
  const hi = range.p90 + spread * 0.25;
  const x = (v) => pad + ((Math.min(Math.max(v, lo), hi) - lo) / (hi - lo)) * (w - 2 * pad);
  const outside = value != null && (value < lo || value > hi);
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} className="macro-range" role="img" aria-label={title}>
      <title>{title}</title>
      <line x1={x(range.p10)} x2={x(range.p90)} y1={h / 2} y2={h / 2} className="macro-range-whisker" />
      <rect x={x(range.p25)} width={Math.max(x(range.p75) - x(range.p25), 1)} y={2} height={h - 4} className="macro-range-band" />
      <line x1={x(range.p50)} x2={x(range.p50)} y1={1} y2={h - 1} className="macro-range-median" />
      {value != null && Number.isFinite(value) ? (
        <circle
          cx={x(value)}
          cy={h / 2}
          r={3}
          fill={outside ? 'none' : color || 'currentColor'}
          stroke={color || 'currentColor'}
          strokeWidth={outside ? 1.5 : 0}
        />
      ) : null}
    </svg>
  );
}

// MacroScorecard shows each player's supply and resource efficiency from engine data. It is a scorecard
// of habits, so it reads as numbers per player rather than a verdict.
export default function MacroScorecard({ game, players, playerColor }) {
  const t = useT();
  const scored = (players || []).filter((p) => p.macro);
  if (game?.mode !== 'engine') {
    return <div className="workflow-card"><div className="chart-empty">{t('macro.pending')}</div></div>;
  }
  if (scored.length === 0) {
    return <div className="workflow-card"><div className="chart-empty">{t('macro.tooShort')}</div></div>;
  }
  const na = t('macro.na');
  const seconds = (v) => formatDuration(Math.round(v));
  const rows = [
    {
      key: 'workersBy6Min',
      raw: (m) => m.workers_by_6min,
      better: 'high',
      range: 'workers_by_6min',
      fmt: whole,
      value: (m) => (m.workers_by_6min == null ? na : String(m.workers_by_6min)),
    },
    {
      key: 'secondsTo40Workers',
      raw: (m) => (m.seconds_to_40_workers == null ? Infinity : m.seconds_to_40_workers),
      better: 'low',
      range: 'seconds_to_40_workers',
      fmt: seconds,
      value: (m) => (m.seconds_to_40_workers == null ? t('macro.notReached') : formatDuration(m.seconds_to_40_workers)),
    },
    {
      key: 'supplyBlocked',
      raw: (m) => m.supply_blocked_seconds,
      better: 'low',
      range: 'supply_blocked_seconds',
      fmt: seconds,
      value: (m) => formatDuration(m.supply_blocked_seconds),
    },
    {
      key: 'floatTime',
      raw: (m) => m.float_seconds,
      better: 'low',
      range: 'float_seconds',
      fmt: seconds,
      value: (m) => formatDuration(m.float_seconds || 0),
    },
    {
      key: 'supplyBlocks',
      range: 'blocks_per_100_supply',
      fmt: oneDecimal,
      value: (m) => (m.blocks_per_100_supply == null
        ? String(m.supply_blocks)
        : t('macro.blocksValue', { count: m.supply_blocks, per100: oneDecimal(m.blocks_per_100_supply) })),
    },
    { key: 'spareSupply', range: 'spare_supply', fmt: oneDecimal, value: (m) => (m.spare_supply == null ? na : oneDecimal(m.spare_supply)) },
    { key: 'spendingQuotient', range: 'spending_quotient', fmt: whole, value: (m) => (m.spending_quotient == null ? na : whole(m.spending_quotient)) },
    { key: 'incomePerMinute', range: 'income_per_minute', fmt: whole, value: (m) => (m.income_per_minute == null ? na : whole(m.income_per_minute)) },
    { key: 'averageUnspent', range: 'average_unspent', fmt: whole, value: (m) => (m.average_unspent == null ? na : whole(m.average_unspent)) },
    { key: 'workerIdle', range: 'worker_idle_share', fmt: percent, value: (m) => (m.worker_idle_share == null ? na : percent(m.worker_idle_share)) },
    {
      key: 'productionIdle',
      range: 'production_idle_share',
      fmt: percent,
      value: (m, p) => (m.production_idle_share == null ? na : percent(m.production_idle_share)),
      hint: (p) => (String(p.race).toLowerCase() === 'zerg' ? t('macro.productionIdleZergHint') : t('macro.productionIdleHint')),
    },
  ];
  const rangeMarker = (row, p) => {
    const range = row.range ? p.macro.pro_ranges?.[row.range] : null;
    if (!range) return null;
    const value = p.macro[row.range];
    const race = t(`race.${String(p.race).toLowerCase()}`);
    const title = t('macro.proRange', {
      race,
      p25: row.fmt(range.p25),
      p75: row.fmt(range.p75),
      p50: row.fmt(range.p50),
      games: range.games,
    });
    return <ProRangeMarker range={range} value={value} color={playerColor ? playerColor(p.color) : undefined} title={title} />;
  };
  const hasRanges = scored.some((p) => p.macro.pro_ranges);
  return (
    <div className="workflow-card macro-scorecard">
      {game.engine_confidence === 'low' ? (
        <div className="workflow-section-warning">
          {t('macro.lowConfidence', { time: formatDuration(game.engine_trusted_until_seconds || 0) })}
        </div>
      ) : null}
      <table className="data-table macro-table">
        <thead>
          <tr>
            <th aria-label="" />
            {scored.map((p) => (
              <th key={p.player_id} style={{ color: playerColor ? playerColor(p.color) : undefined }}>{p.name}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const top = row.raw ? best(scored.map((p) => row.raw(p.macro)), row.better) : null;
            return (
              <tr key={row.key}>
                <th scope="row" title={t(`macro.hint.${row.key}`)}>{t(`macro.metric.${row.key}`)}</th>
                {scored.map((p) => (
                  <td
                    key={p.player_id}
                    title={row.hint ? row.hint(p) : undefined}
                    className={top != null && row.raw(p.macro) === top ? 'macro-best' : undefined}
                  >
                    <div className="macro-cell">
                      <span>{row.value(p.macro, p)}</span>
                      {rangeMarker(row, p)}
                    </div>
                  </td>
                ))}
              </tr>
            );
          })}
          <tr>
            <th scope="row" title={t('macro.hint.workersCurve')}>{t('macro.metric.workersCurve')}</th>
            {scored.map((p) => (
              <td key={p.player_id}>
                <IncomeSparkline curve={p.macro.workers_curve} color={playerColor ? playerColor(p.color) : undefined} />
              </td>
            ))}
          </tr>
          <tr>
            <th scope="row" title={t('macro.hint.incomeCurve')}>{t('macro.metric.incomeCurve')}</th>
            {scored.map((p) => (
              <td key={p.player_id}>
                <IncomeSparkline curve={p.macro.income_curve} color={playerColor ? playerColor(p.color) : undefined} />
              </td>
            ))}
          </tr>
        </tbody>
      </table>
      {hasRanges ? <div className="macro-range-legend">{t('macro.proRangeLegend')}</div> : null}
    </div>
  );
}
