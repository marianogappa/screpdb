import React from 'react';
import { useT } from '../lib/i18nContext';
import { formatDuration } from '../lib/formatters';

const percent = (v) => `${Math.round(v * 100)}%`;
const whole = (v) => String(Math.round(v));
const oneDecimal = (v) => (Math.round(v * 10) / 10).toFixed(1);

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
  const rows = [
    {
      key: 'supplyBlocked',
      value: (m) => formatDuration(m.supply_blocked_seconds),
    },
    {
      key: 'supplyBlocks',
      value: (m) => (m.blocks_per_100_supply == null
        ? String(m.supply_blocks)
        : t('macro.blocksValue', { count: m.supply_blocks, per100: oneDecimal(m.blocks_per_100_supply) })),
    },
    { key: 'spareSupply', value: (m) => (m.spare_supply == null ? na : oneDecimal(m.spare_supply)) },
    { key: 'spendingQuotient', value: (m) => (m.spending_quotient == null ? na : whole(m.spending_quotient)) },
    { key: 'incomePerMinute', value: (m) => (m.income_per_minute == null ? na : whole(m.income_per_minute)) },
    { key: 'averageUnspent', value: (m) => (m.average_unspent == null ? na : whole(m.average_unspent)) },
    { key: 'workerIdle', value: (m) => (m.worker_idle_share == null ? na : percent(m.worker_idle_share)) },
    {
      key: 'productionIdle',
      value: (m, p) => (m.production_idle_share == null ? na : percent(m.production_idle_share)),
      hint: (p) => (String(p.race).toLowerCase() === 'zerg' ? t('macro.productionIdleZergHint') : t('macro.productionIdleHint')),
    },
  ];
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
          {rows.map((row) => (
            <tr key={row.key}>
              <th scope="row" title={t(`macro.hint.${row.key}`)}>{t(`macro.metric.${row.key}`)}</th>
              {scored.map((p) => (
                <td key={p.player_id} title={row.hint ? row.hint(p) : undefined}>{row.value(p.macro, p)}</td>
              ))}
            </tr>
          ))}
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
    </div>
  );
}
