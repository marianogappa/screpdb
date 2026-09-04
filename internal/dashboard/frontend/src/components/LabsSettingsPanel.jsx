import React from 'react';
import { useT } from '../lib/i18nContext';

// Each entry is one switch. A single short line beats a title plus a paragraph:
// these are previews, so the honest description is vague anyway, and a wall of
// explanation for something half-built reads worse than a hint.
export const LABS_FEATURES = [
  {
    key: 'gaming_session',
    labelKey: 'labs.feature.gamingSession',
  },
];

function LabsSettingsPanel({ flags, saving, message, messageIsError, onToggle }) {
  const t = useT();
  return (
    <div className="labs-panel">
      <div className="workflow-inline-warning">
        <span aria-hidden="true">⚠️</span>
        {t('labs.warning')}
      </div>
      {message ? (
        <div className={messageIsError ? 'error-message' : 'workflow-subtle-note'}>{message}</div>
      ) : null}
      <div className="labs-list">
        {LABS_FEATURES.map((feature) => (
          <label key={feature.key} className="labs-item">
            <input
              type="checkbox"
              checked={Boolean(flags?.[feature.key])}
              disabled={saving}
              onChange={(e) => onToggle(feature.key, e.target.checked)}
            />
            <span>{t(feature.labelKey)}</span>
          </label>
        ))}
      </div>
    </div>
  );
}

export default LabsSettingsPanel;
