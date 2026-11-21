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

const LEARN_SITES = { 'liquipedia.net': 'Liquipedia', 'classic.battle.net': 'Battle.net' };

const learnSite = (url) => {
  try {
    return LEARN_SITES[new URL(url).hostname] || new URL(url).hostname;
  } catch {
    return '';
  }
};

const exampleDate = (date) => {
  const d = new Date(date);
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
};

const floatTime =(seconds, t) => (seconds >= 90
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
    default: {
      const tiered = TIERED[insight.key];
      if (!tiered) return null;
      return {
        tone: tiered.tone,
        emoji: tiered.emoji,
        title: t(`insights.${insight.key}.title`),
        body: t(`insights.${insight.key}.body`),
        statEmoji: '📊',
        stat: t(`insights.${insight.key}.stat`, p),
      };
    }
  }
};

const TIERED = {
  no_hotkeys: { tone: 'hotkeys', emoji: '⌨️' },
  army_hotkeys: { tone: 'hotkeys', emoji: '⌨️' },
  production_hotkeys: { tone: 'hotkeys', emoji: '⌨️' },
  keep_producing: { tone: 'production', emoji: '🏭' },
  steady_army: { tone: 'production', emoji: '🔁' },
  use_army: { tone: 'army', emoji: '🚩' },
  bank: { tone: 'bank', emoji: '💰' },
};

function InsightCard({ insight, registry, onOpenGame, improved, strength }) {
  const t = useT();
  const view = insightView(insight, registry, t);
  if (!view) return null;
  if (strength) {
    return (
      <div className="insight-card insight-card--strength">
        {view.emoji ? <span className="insight-card-emoji" aria-hidden="true">{view.emoji}</span> : null}
        <div className="insight-card-main">
          <div className="insight-card-title">{t(`insights.${insight.key}.strength`)}</div>
        </div>
      </div>
    );
  }
  if (improved) {
    return (
      <div className={`insight-card insight-card--improved`}>
        {view.emoji ? <span className="insight-card-emoji" aria-hidden="true">{view.emoji}</span> : null}
        <div className="insight-card-main">
          <div className="insight-card-title">{view.title}</div>
          <div className="insight-card-body">{t('insights.improved.body')}</div>
        </div>
      </div>
    );
  }
  const examples = Array.isArray(insight.examples) ? insight.examples : [];
  return (
    <div className={`insight-card insight-card--${view.tone}`}>
      {view.icon ? <img className="insight-card-icon" src={view.icon} alt="" /> : null}
      {!view.icon && view.emoji ? <span className="insight-card-emoji" aria-hidden="true">{view.emoji}</span> : null}
      <div className="insight-card-main">
        <div className="insight-card-title">{view.title}</div>
        <div className="insight-card-body">
          {view.body}
          {insight.learn ? (
            <>
              {' '}
              <a className="insight-learn" href={insight.learn} target="_blank" rel="noreferrer">{learnSite(insight.learn)} <span aria-hidden="true">↗</span></a>
            </>
          ) : null}
        </div>
        {view.stat ? (
          <div className="insight-card-stat">
            <span className="insight-card-stat-emoji" aria-hidden="true">{view.statEmoji}</span>
            <span>{view.stat}</span>
          </div>
        ) : null}
        {insight.trend ? (
          <span className={`insight-trend insight-trend--${insight.trend}`}>{t(`insights.trend.${insight.trend}`)}</span>
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
                <span className="insight-example-opp">{ex.opponent || exampleDate(ex.date)}</span>
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

export default function PlayerInsights({ insights, improved = [], strengths = [], registry, onOpenGame }) {
  const t = useT();
  return (
    <div className="insight-sections">
      <div className="insight-list">
        {insights.map((insight) => (
          <InsightCard key={insight.key} insight={insight} registry={registry} onOpenGame={onOpenGame} />
        ))}
      </div>
      {improved.length > 0 ? (
        <div className="insight-improved">
          <div className="insight-improved-label">{t('insights.improved.heading')}</div>
          <div className="insight-list">
            {improved.map((insight) => (
              <InsightCard key={insight.key} insight={insight} registry={registry} onOpenGame={onOpenGame} improved />
            ))}
          </div>
        </div>
      ) : null}
      {strengths.length > 0 ? (
        <div className="insight-improved">
          <div className="insight-improved-label">👍 {t('insights.ok.heading')}</div>
          <div className="insight-list">
            {strengths.map((insight) => (
              <InsightCard key={insight.key} insight={insight} registry={registry} onOpenGame={onOpenGame} strength />
            ))}
          </div>
        </div>
      ) : null}
    </div>
  );
}
