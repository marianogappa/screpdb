import React from 'react';
import { useT } from '../lib/i18nContext';
import { markerName, renderAggregatePillText } from '../lib/markerRegistry';
import { getUnitIcon } from '../lib/gameAssets';

const DAY = 24 * 60 * 60 * 1000;

// sinceLabel says how far back an insight reads, in the coarsest unit that still says it: days, weeks, months.
const sinceLabel = (from, t) => {
  const days = Math.max(1, Math.ceil((Date.now() - Date.parse(from)) / DAY));
  if (!Number.isFinite(days)) return '';
  if (days < 14) return t.plural('insights.span.days', days);
  if (days < 63) return t.plural('insights.span.weeks', Math.round(days / 7));
  return t.plural('insights.span.months', Math.round(days / 30.4));
};

const floatTime = (seconds, t) => (seconds >= 90
  ? t.plural('insights.minutes', Math.round(seconds / 60))
  : t('insights.seconds', { count: Math.max(10, Math.round(seconds / 10) * 10) }));

const insightView = (insight, registry, t) => {
  const p = insight.params || {};
  switch (insight.key) {
    case 'floating':
      return {
        tone: 'bank',
        statEmoji: '💰',
        icon: getUnitIcon('Command Center'),
        title: t('insights.floating.title'),
        body: t('insights.floating.body'),
        stat: t('insights.floating.stat', { you: floatTime(p.seconds_per_10min, t), pro: floatTime(p.pro_seconds_per_10min, t) }),
      };
    case 'opener': {
      const opener = markerName(registry?.[p.opener]) || p.opener;
      return {
        tone: 'opener',
        statEmoji: '📉',
        icon: renderAggregatePillText(registry?.[p.opener])?.icon || null,
        title: t('insights.opener.title', { opener }),
        body: t('insights.opener.body', { matchup: p.matchup }),
        stat: t('insights.opener.stat', { vs: p.win_vs, rest: p.win_rest }),
      };
    }
    case 'early_turrets':
      return {
        tone: 'turrets',
        statEmoji: '🛡️',
        icon: getUnitIcon('Missile Turret'),
        title: t('insights.earlyTurrets.title'),
        body: t('insights.earlyTurrets.body'),
        stat: t('insights.earlyTurrets.stat', { you: Math.round(p.turrets), pro: p.pro_turrets }),
      };
    default:
      return null;
  }
};

function InsightCard({ insight, registry, onOpenGame }) {
  const t = useT();
  const view = insightView(insight, registry, t);
  if (!view) return null;
  const examples = Array.isArray(insight.examples) ? insight.examples : [];
  return (
    <div className={`insight-card insight-card--${view.tone}`}>
      {view.icon ? <img className="insight-card-icon" src={view.icon} alt="" /> : null}
      <div className="insight-card-main">
        <div className="insight-card-title">{view.title}</div>
        <div className="insight-card-body">{view.body}</div>
        {view.stat ? (
          <div className="insight-card-stat">
            <span className="insight-card-stat-emoji" aria-hidden="true">{view.statEmoji}</span>
            <span>{view.stat}</span>
          </div>
        ) : null}
        {examples.length > 0 ? (
          <div className="insight-card-examples">
            <span className="insight-card-examples-label">{t('insights.examples')}</span>
            {examples.map((ex) => (
              <button
                key={ex.replay_id}
                type="button"
                className="insight-example"
                onClick={() => onOpenGame(ex.replay_id, ex.tab)}
              >
                <span>{ex.won ? '✅' : '❌'}</span>
                <span className="insight-example-opp">{ex.opponent}</span>
                <span className="insight-example-map">{ex.map}</span>
              </button>
            ))}
          </div>
        ) : null}
        {insight.from ? (
          <div className="insight-card-foot">{t('insights.basedOn', { count: insight.games, span: sinceLabel(insight.from, t) })}</div>
        ) : null}
      </div>
    </div>
  );
}

export default function PlayerInsights({ insights, registry, onOpenGame }) {
  return (
    <div className="insight-list">
      {insights.map((insight) => (
        <InsightCard key={insight.key} insight={insight} registry={registry} onOpenGame={onOpenGame} />
      ))}
    </div>
  );
}
