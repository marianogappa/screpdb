import React from 'react';

const STATUS_LABELS = {
  idle: 'Idle',
  running: 'Running',
  completed: 'Completed',
  failed: 'Failed',
};

function IngestModal({
  ingestForm,
  ingestMessage,
  ingestStatus,
  ingestLogs,
  ingestInputDir,
  ingestInputDirDirty,
  ingestSettingsLoading,
  ingestSettingsSaving,
  ingestSocketState,
  onClose,
  onSubmit,
  onChange,
  onInputDirChange,
  onSaveInputDir,
}) {
  const statusLabel = STATUS_LABELS[ingestStatus] || 'Idle';
  const messageClassName = ingestStatus === 'completed' ? 'success-message' : 'error-message';

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content global-filter-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Ingest</h2>
          <button onClick={onClose} className="btn-close">×</button>
        </div>
        <div className="edit-form ingest-form">
          {ingestMessage ? <div className={messageClassName}>{ingestMessage}</div> : null}
          <div className="ingest-section">
            <div className="ingest-header">
              <div>
                <div className="ingest-title">Replay folder</div>
                <div className="ingest-subtitle">This saved folder is shared by manual ingest and auto-ingest.</div>
              </div>
              <div className={`ingest-status ingest-status-${ingestStatus || 'idle'}`}>{statusLabel}</div>
            </div>
            <div className="ingest-field ingest-path-field">
              <span>Folder path</span>
              <div className="ingest-path-row">
                <input
                  type="text"
                  value={ingestInputDir}
                  placeholder={ingestSettingsLoading ? 'Loading replay folder...' : '/path/to/replays'}
                  disabled={ingestSettingsLoading || ingestSettingsSaving}
                  onChange={(e) => onInputDirChange(e.target.value)}
                />
                <button
                  type="button"
                  className="btn-save"
                  disabled={ingestSettingsLoading || ingestSettingsSaving || !ingestInputDirDirty}
                  onClick={onSaveInputDir}
                >
                  {ingestSettingsSaving ? 'Saving...' : 'Save Folder'}
                </button>
              </div>
              <div className="ingest-helper-row">
                <span className="ingest-helper-text">Validated recursively: the folder must contain at least one `.rep` file.</span>
                <span className="ingest-helper-text">Log stream: {ingestSocketState}</span>
              </div>
            </div>
          </div>

          <form onSubmit={onSubmit} className="ingest-section">
            <div className="ingest-header">
              <div className="ingest-title">Manual ingest</div>
              <div className="ingest-subtitle">This starts a one-off ingest and keeps this dialog open for live progress.</div>
            </div>
            <div className="ingest-grid">
              <label className="ingest-field">
                <span>Ingest last N replays</span>
                <input
                  type="number"
                  min="1"
                  value={ingestForm.stopAfterN}
                  onChange={(e) => onChange({ ...ingestForm, stopAfterN: parseInt(e.target.value || '0', 10) })}
                />
              </label>
              <label className="ingest-field ingest-checkbox">
                <span>Erase existing data</span>
                <input
                  type="checkbox"
                  checked={ingestForm.clean}
                  onChange={(e) => onChange({ ...ingestForm, clean: e.target.checked })}
                />
              </label>
              <label className="ingest-field ingest-checkbox">
                <span>Store Right Click commands</span>
                <input
                  type="checkbox"
                  checked={ingestForm.storeRightClicks}
                  onChange={(e) => onChange({ ...ingestForm, storeRightClicks: e.target.checked })}
                />
              </label>
              <label className="ingest-field ingest-checkbox">
                <span>Skip Hotkey commands</span>
                <input
                  type="checkbox"
                  checked={ingestForm.skipHotkeys}
                  onChange={(e) => onChange({ ...ingestForm, skipHotkeys: e.target.checked })}
                />
              </label>
            </div>
            <div className="form-actions">
              <button type="button" onClick={onClose} className="btn-cancel">
                Cancel
              </button>
              <button type="submit" className="btn-save" disabled={ingestSettingsLoading || ingestSettingsSaving}>
                {ingestStatus === 'running' ? 'Ingest Running...' : 'Start Ingest'}
              </button>
            </div>
          </form>

          <div className="ingest-section ingest-section-muted">
            <div className="ingest-header">
              <div className="ingest-title">Auto-ingest settings</div>
              <div className="ingest-subtitle">These settings are saved immediately and apply independently from manual ingest.</div>
            </div>
            <div className="ingest-grid">
              <label className="ingest-field ingest-checkbox">
                <span>Auto-ingest latest replay</span>
                <input
                  type="checkbox"
                  checked={ingestForm.autoIngestEnabled}
                  onChange={(e) => onChange({ ...ingestForm, autoIngestEnabled: e.target.checked })}
                />
              </label>
              <label className="ingest-field">
                <span>Auto-ingest interval (seconds)</span>
                <input
                  type="number"
                  min="60"
                  step="1"
                  value={ingestForm.autoIngestIntervalSeconds}
                  disabled={!ingestForm.autoIngestEnabled}
                  onChange={(e) => onChange({ ...ingestForm, autoIngestIntervalSeconds: parseInt(e.target.value || '60', 10) })}
                />
              </label>
            </div>
          </div>

          <div className="ingest-section ingest-section-muted">
            <div className="ingest-header">
              <div className="ingest-title">Live log</div>
              <div className="ingest-subtitle">Streaming stderr output while this dialog stays open.</div>
            </div>
            <div className="ingest-log-panel" role="log" aria-live="polite">
              {ingestLogs.length === 0 ? (
                <div className="ingest-log-empty">Logs will appear here once ingest starts.</div>
              ) : (
                ingestLogs.map((entry, idx) => (
                  <div key={`ingest-log-${idx}`} className={`ingest-log-line ingest-log-${entry.level || 'info'}`}>
                    {entry.message}
                  </div>
                ))
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default IngestModal;
