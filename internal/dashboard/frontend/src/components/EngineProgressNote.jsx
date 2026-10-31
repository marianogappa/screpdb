import React from 'react';
import { useT } from '../lib/i18nContext';

// EngineProgressNote tells a view built on engine data that the engine pass is still going, and offers the
// all cores switch on hover rather than in the user's face.
export default function EngineProgressNote({ coverage, allCores, saving, onEnableAllCores }) {
  const t = useT();
  const games = Number(coverage?.games) || 0;
  const total = Number(coverage?.total) || 0;
  if (!coverage || coverage.state === 'off' || coverage.state === 'failed' || total === 0 || games >= total) return null;
  return (
    <div className="engine-progress-note" role="status">
      <span>{t('engine.progress.skill', { games, total })}</span>
      {!allCores && onEnableAllCores ? (
        <span className="engine-speedup">
          <span className="engine-speedup-link">{t('engine.speedup.link')}</span>
          <span className="engine-speedup-bubble">
            <span>{t('engine.speedup.body')}</span>
            <button type="button" className="btn-save" disabled={saving} onClick={onEnableAllCores}>{t('engine.speedup.action')}</button>
          </span>
        </span>
      ) : null}
    </div>
  );
}
