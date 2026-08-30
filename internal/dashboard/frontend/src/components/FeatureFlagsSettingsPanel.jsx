import React from 'react';

// Preview features, off by default. Each entry is a flag the backend persists
// in the settings row; the UI here is deliberately plain, because a flag list
// is a list of switches and nothing more.
export const FEATURE_FLAGS = [
  {
    key: 'gaming_session',
    label: 'Gaming Session',
    description:
      'Adds a Gaming Session tab that appears when you have played in the last few hours, '
      + 'summarising that run of games: your record, the matchups, and who you faced.',
  },
];

function FeatureFlagsSettingsPanel({ flags, saving, message, messageIsError, onToggle }) {
  return (
    <div className="feature-flags-panel">
      <p className="feature-flags-intro">
        Previews in progress. They are off by default and may change or disappear.
      </p>
      {message ? (
        <div className={messageIsError ? 'error-message' : 'workflow-subtle-note'}>{message}</div>
      ) : null}
      <ul className="feature-flags-list">
        {FEATURE_FLAGS.map((flag) => (
          <li key={flag.key} className="feature-flags-item">
            <label className="feature-flags-toggle">
              <input
                type="checkbox"
                checked={Boolean(flags?.[flag.key])}
                disabled={saving}
                onChange={(e) => onToggle(flag.key, e.target.checked)}
              />
              <span className="feature-flags-label">{flag.label}</span>
            </label>
            <div className="feature-flags-description">{flag.description}</div>
          </li>
        ))}
      </ul>
    </div>
  );
}

export default FeatureFlagsSettingsPanel;
