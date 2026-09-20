import React from 'react';
import { useT } from '../lib/i18nContext';

// Each entry is one switch, described in a single short line: these are
// previews, so the honest description is vague anyway, and a wall of
// explanation for something half-built reads worse than a hint.
//
// Empty for now. Completing your replays graduated out of Labs and is on by
// default, with an opt-out on the Folder tab. The Settings modal hides the tab
// while this list is empty, so adding the next preview here is all it takes to
// bring it back.
export const LABS_FEATURES = [];

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
