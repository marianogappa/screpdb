import React, { useEffect, useState } from 'react';
import { useT } from '../lib/i18nContext';
import { createPortal } from 'react-dom';

const RACES = [['T', 'race.terran'], ['P', 'race.protoss'], ['Z', 'race.zerg']];
const COLORS = ['#60a5fa', '#2dd4bf', '#4ade80', '#a78bfa', '#f87171', '#f0c674'];
const percent = (v) => `${Math.round(v * 100)}%`;

// SkillExplainerContent is "How Skill works": where the tiers come from and what each metric adds, with the
// live model's numbers (model is the API's skill_model).
export function SkillExplainerContent({ model }) {
  const t = useT();
  const metrics = model?.metrics || [];
  return (
    <div className="skill-explainer-body">
      <p className="skill-explainer-lede">{t('skill.explain.intro')}</p>
      <div className="skill-explainer-points">
        <section>
          <h3>{t('skill.explain.corpus.title')}</h3>
          <p>{t('skill.explain.corpus.body', { games: (model?.games || 0).toLocaleString() })}</p>
        </section>
        <section>
          <h3>{t('skill.explain.race.title')}</h3>
          <p>{t('skill.explain.race.body')}</p>
        </section>
        <section>
          <h3>{t('skill.explain.picked.title')}</h3>
          <p>{t('skill.explain.picked.body')}</p>
        </section>
      </div>

      <h3>{t('skill.explain.metrics.title')}</h3>
      <div className="skill-explainer-metrics">
        {metrics.map((m, i) => (
          <div key={m.key} className="skill-explainer-metric">
            <div className="skill-explainer-metric-head">
              <span className="skill-explainer-swatch" style={{ background: COLORS[i % COLORS.length] }} />
              <b>{t(`skill.metric.${m.key}.title`)}</b>
              <span className="skill-explainer-direction">{t(m.higher_is_better ? 'skill.explain.higherBetter' : 'skill.explain.lowerBetter')}</span>
            </div>
            <p>{t(`skill.metric.${m.key}.hint`)}</p>
            <div className="skill-explainer-follow">
              <span>{t('skill.explain.follows')}</span>
              <div className="skill-explainer-follow-track"><div style={{ width: `${Math.abs(m.rho) * 100}%` }} /></div>
              <span className="skill-explainer-num">{Math.abs(m.rho).toFixed(2)}</span>
            </div>
          </div>
        ))}
      </div>
      <p className="skill-explainer-note">{t('skill.explain.follows.note')}</p>

      <h3>{t('skill.explain.weights.title')}</h3>
      <p>{t('skill.explain.weights.body')}</p>
      <div className="skill-explainer-stacks">
        {RACES.map(([race, label]) => (
          <div key={race} className="skill-explainer-stack-row">
            <span>{t(label)}</span>
            <div className="skill-explainer-stack">
              {metrics.map((m, i) => (
                <div
                  key={m.key}
                  style={{ width: percent(m.share?.[race] || 0), background: COLORS[i % COLORS.length] }}
                  title={`${t(`skill.metric.${m.key}.title`)}: ${percent(m.share?.[race] || 0)}`}
                >
                  {percent(m.share?.[race] || 0)}
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>

      <h3>{t('skill.explain.oneGame.title')}</h3>
      <p>{t('skill.explain.oneGame.body')}</p>
    </div>
  );
}

function SkillExplainer({ model, onClose }) {
  const t = useT();
  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape') onClose(); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content skill-explainer" role="dialog" aria-modal="true" aria-labelledby="skill-explainer-title" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2 id="skill-explainer-title">{t('skill.explain.title')}</h2>
          <button type="button" className="btn-close" onClick={onClose} aria-label={t('skill.explain.close')}>×</button>
        </div>
        <SkillExplainerContent model={model} />
      </div>
    </div>
  );
}

// SkillExplainerLink opens the explainer; it renders nothing without the model to explain.
export default function SkillExplainerLink({ model, className = '', compact = false }) {
  const t = useT();
  const [open, setOpen] = useState(false);
  if (!model) return null;
  return (
    <>
      <button
        type="button"
        className={`skill-explainer-link${compact ? ' skill-explainer-link--compact' : ''} ${className}`.trim()}
        onClick={(e) => { e.stopPropagation(); setOpen(true); }}
        aria-label={t('skill.explain.open')}
        title={t('skill.explain.open')}
      >
        {compact ? 'ⓘ' : t('skill.explain.open')}
      </button>
      {open ? createPortal(<SkillExplainer model={model} onClose={() => setOpen(false)} />, document.body) : null}
    </>
  );
}
