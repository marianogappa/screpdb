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
  isSampleSet,
  detectedReplayDir,
  sampleSetLoading,
  onClose,
  onSubmit,
  onChange,
  onInputDirChange,
  onSaveInputDir,
  onLoadSampleSet,
  onUseDetectedFolder,
  onDismissMessage,
}) {
  const statusLabel = STATUS_LABELS[ingestStatus] || 'Idle';
  // Disable every action while an ingest is running: a second ingest would
  // hit a locked SQLite DB and fail anyway.
  const busy = ingestSettingsLoading || ingestSettingsSaving || sampleSetLoading || ingestStatus === 'running';
  // Colour by the message itself, not ingestStatus: an error can occur while a
  // prior ingest still reads "completed", and it must still show red.
  const messageIsSuccess = ingestMessage === 'Ingestion completed.' || ingestMessage === 'Replay folder saved.';
  // "On the example set" only while the saved sample folder is also the one in
  // the box. The moment the user types a different folder, treat it as their
  // own source: reveal "Ingest now" (Start Ingest saves the typed path first).
  const onSampleSource = isSampleSet && !ingestInputDirDirty;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content global-filter-modal ingest-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Ingest</h2>
          {ingestMessage ? (
            <div
              className={`ingest-header-message ${messageIsSuccess ? 'is-success' : 'is-error'}`}
              role="status"
            >
              <span className="ingest-header-message-text" title={ingestMessage}>{ingestMessage}</span>
              <button
                type="button"
                className="ingest-message-dismiss"
                aria-label="Dismiss"
                onClick={onDismissMessage}
              >
                ×
              </button>
            </div>
          ) : null}
          <button type="button" onClick={onClose} className="btn-close">×</button>
        </div>
        <div className="edit-form ingest-form ingest-form-plain">
          {/* 1. Replay folder path — the single "where do replays come from" decision. */}
          <div className="ingest-plain-block">
            <div className="ingest-title">Replay folder path</div>
            <div className="ingest-field ingest-path-field">
              <div className="ingest-path-row">
                <input
                  type="text"
                  value={ingestInputDir}
                  placeholder={ingestSettingsLoading ? 'Loading replay folder...' : '/path/to/replays'}
                  disabled={busy}
                  onChange={(e) => onInputDirChange(e.target.value)}
                />
                <button
                  type="button"
                  className="btn-save"
                  disabled={busy || !ingestInputDirDirty}
                  onClick={onSaveInputDir}
                >
                  {ingestSettingsSaving ? 'Saving...' : 'Save Folder'}
                </button>
              </div>
              <span className="ingest-helper-text">Folder must contain at least one `.rep` file (recursively)</span>
            </div>
          </div>

          {/* 2 + 3 side by side, leaving more vertical room for the log below. */}
          <div className="ingest-section-row">
          {/* 2. Ingest now — the action. While the example set is active,
              ingesting "last N" of the examples is meaningless, so this space
              instead offers a way back to the user's detected replay folder. */}
          {onSampleSource ? (
            <div className="ingest-plain-block ingest-col">
              <div className="ingest-title">Your replays</div>
              {detectedReplayDir ? (
                <>
                  <span className="ingest-helper-text">Switch back to the StarCraft replay folder we found on this computer:</span>
                  <button
                    type="button"
                    className="btn-save ingest-start"
                    disabled={busy}
                    onClick={onUseDetectedFolder}
                  >
                    Use my replay folder
                  </button>
                  <span className="ingest-helper-text" title={detectedReplayDir}>{detectedReplayDir}</span>
                </>
              ) : (
                <span className="ingest-helper-text">Set your replay folder above to ingest your own games.</span>
              )}
            </div>
          ) : (
          <form onSubmit={onSubmit} className="ingest-plain-block ingest-col">
            <div className="ingest-title">Ingest now</div>
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
              <div className="ingest-destructive-group">
                <label className="ingest-manual-row ingest-manual-check ingest-destructive">
                  <input
                    type="checkbox"
                    checked={ingestForm.clean}
                    onChange={(e) => onChange({ ...ingestForm, clean: e.target.checked })}
                  />
                  <span>Erase existing data</span>
                </label>
                <span className="ingest-helper-text ingest-destructive-help">
                  Clears screpdb's database, not your replay files. Useful after upgrading, to re-analyze with the latest detection improvements.
                </span>
              </div>
              <div className="ingest-manual-row">
                <button type="submit" className="btn-save ingest-start" disabled={busy}>
                  {ingestStatus === 'running' ? 'Ingest Running...' : 'Start Ingest'}
                </button>
              </div>
            </div>
          </form>
          )}

          {/* 3. Keep up to date — an ongoing behavior, not a one-shot. */}
          <div className="ingest-plain-block ingest-col">
            <div className="ingest-title">Keep up to date</div>
            <label className="ingest-auto-inline">
              <input
                type="checkbox"
                checked={ingestForm.autoIngestEnabled}
                onChange={(e) => onChange({ ...ingestForm, autoIngestEnabled: e.target.checked })}
              />
              <span>Auto-ingest latest replay</span>
            </label>

            {/* Example replays live here to fill the column's vertical space. */}
            <div className="ingest-title ingest-subheading">Example replays</div>
            {!isSampleSet ? (
              <div className="ingest-sample-col">
                <button
                  type="button"
                  className="btn-save ingest-load-sample"
                  disabled={busy}
                  onClick={onLoadSampleSet}
                >
                  {sampleSetLoading ? 'Loading example replays...' : 'Load example replays'}
                </button>
                <span className="ingest-helper-text">A few example games to try every feature. Replaces your current screpdb data (not your .rep files).</span>
              </div>
            ) : (
              // Always offer a re-ingest action so a wiped DB on a machine with
              // no detected replay folder isn't a dead end.
              <div className="ingest-sample-col">
                <span className="ingest-helper-text ingest-sample-active">You're using the built-in example replays.</span>
                <button
                  type="button"
                  className="btn-save ingest-load-sample"
                  disabled={busy}
                  onClick={onLoadSampleSet}
                >
                  {sampleSetLoading ? 'Loading example replays...' : 'Re-ingest example replays'}
                </button>
              </div>
            )}
          </div>
          </div>

          {/* 4. Progress — status + log + connection state, together. */}
          <div className="ingest-plain-block ingest-progress-block">
            <div className="ingest-plain-heading">
              <div className="ingest-title">Progress</div>
              <div className={`ingest-status ingest-status-${ingestStatus || 'idle'}`}>{statusLabel}</div>
            </div>
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
            <span className="ingest-helper-text">Log stream: {ingestSocketState}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default IngestModal;
