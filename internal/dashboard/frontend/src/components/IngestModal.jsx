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
      <div className="modal-content global-filter-modal ingest-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Ingest</h2>
          <button type="button" onClick={onClose} className="btn-close">×</button>
        </div>
        <div className="edit-form ingest-form ingest-form-plain">
          {ingestMessage ? <div className={messageClassName}>{ingestMessage}</div> : null}

          <div className="ingest-plain-block">
            <div className="ingest-plain-heading">
              <div className="ingest-title">Replay folder</div>
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
                <span className="ingest-helper-text">Folder must contain at least one `.rep` file (recursively)</span>
                <span className="ingest-helper-text">Log stream: {ingestSocketState}</span>
              </div>
            </div>
            <label className="ingest-auto-inline">
              <input
                type="checkbox"
                checked={ingestForm.autoIngestEnabled}
                onChange={(e) => onChange({ ...ingestForm, autoIngestEnabled: e.target.checked })}
              />
              <span>Auto-ingest latest replay</span>
            </label>
          </div>

          <form onSubmit={onSubmit} className="ingest-plain-block ingest-manual-block">
            <div className="ingest-title">Manual ingest</div>
            <div className="ingest-manual-stack">
              <div className="ingest-manual-row">
                <span className="ingest-manual-caption">Ingest last N replays</span>
                <input
                  className="ingest-manual-number"
                  type="number"
                  min="1"
                  value={ingestForm.stopAfterN}
                  onChange={(e) => onChange({ ...ingestForm, stopAfterN: parseInt(e.target.value || '0', 10) })}
                />
              </div>
              <label className="ingest-manual-row ingest-manual-check">
                <input
                  type="checkbox"
                  checked={ingestForm.clean}
                  onChange={(e) => onChange({ ...ingestForm, clean: e.target.checked })}
                />
                <span>Erase existing data</span>
              </label>
              <div className="ingest-manual-row">
                <button type="submit" className="btn-save" disabled={ingestSettingsLoading || ingestSettingsSaving}>
                  {ingestStatus === 'running' ? 'Ingest Running...' : 'Start Ingest'}
                </button>
              </div>
            </div>
          </form>

          <div className="ingest-plain-block">
            <div className="ingest-title">Live log</div>
            <div className="ingest-log-panel ingest-log-panel-plain" role="log" aria-live="polite">
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
