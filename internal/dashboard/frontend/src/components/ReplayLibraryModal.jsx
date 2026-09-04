import React from 'react';
import {
  formatCount,
  formatLoaded,
  isLibraryLoading,
  percent,
  phaseLabel,
  statusLabel,
} from '../lib/libraryProgress';

const STATUS_CLASS = {
  idle: 'idle',
  loading: 'running',
  watching: 'completed',
  failed: 'failed',
};

function ReplayLibraryModal({
  libraryMessage,
  libraryStatus,
  libraryProgress,
  libraryLogs,
  librarySocketState,
  replayDirInput,
  savedReplayDir,
  librarySettingsLoading,
  librarySettingsSaving,
  isSampleSet,
  detectedReplayDir,
  sampleSetLoading,
  onClose,
  onReplayDirChange,
  onSaveReplayDir,
  onLoadSampleSet,
  onUseDetectedFolder,
  onDismissMessage,
}) {
  const status = String(libraryStatus || 'idle').toLowerCase();
  const busy = librarySettingsLoading || librarySettingsSaving || sampleSetLoading;
  const replayDirDirty = String(replayDirInput || '').trim() !== String(savedReplayDir || '').trim();
  // Colour by the message itself, not the library status: an error can arrive
  // while the library is still happily loading, and it must still show red.
  const messageIsSuccess = libraryMessage === 'Replay folder saved.' || libraryMessage === 'Switched to the example replays.';
  const detectedIsCurrent = Boolean(detectedReplayDir) && String(detectedReplayDir).trim() === String(savedReplayDir || '').trim();
  const loading = isLibraryLoading(status);
  const progress = libraryProgress || null;
  const loaded = Number(progress?.loaded || 0);
  const total = Number(progress?.total || 0);
  const failed = Number(progress?.failed || 0);
  const skipped = Number(progress?.skipped || 0);
  const phase = progress?.phase ? phaseLabel(progress.phase) : statusLabel(status);
  const barPercent = status === 'watching' && total === 0 ? 100 : percent(loaded, total);
  const logs = Array.isArray(libraryLogs) ? libraryLogs : [];

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content global-filter-modal ingest-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Replay library</h2>
          {libraryMessage ? (
            <div
              className={`ingest-header-message ${messageIsSuccess ? 'is-success' : 'is-error'}`}
              role="status"
            >
              <span className="ingest-header-message-text" title={libraryMessage}>{libraryMessage}</span>
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
          <div className="ingest-plain-block">
            <div className="ingest-title">Replay folder path</div>
            <div className="ingest-field ingest-path-field">
              <div className="ingest-path-row">
                <input
                  type="text"
                  value={replayDirInput}
                  placeholder={librarySettingsLoading ? 'Loading replay folder...' : '/path/to/replays'}
                  disabled={busy}
                  onChange={(e) => onReplayDirChange(e.target.value)}
                />
                <button
                  type="button"
                  className="btn-save"
                  disabled={busy || !replayDirDirty}
                  onClick={onSaveReplayDir}
                >
                  {librarySettingsSaving ? 'Saving...' : 'Save Folder'}
                </button>
              </div>
              <span className="ingest-helper-text">
                Folder must contain at least one `.rep` file (subfolders included).
              </span>
            </div>
          </div>

          <div className="ingest-section-row">
            <div className="ingest-plain-block ingest-col">
              <div className="ingest-title">Your replays</div>
              {detectedReplayDir && !detectedIsCurrent ? (
                <>
                  <span className="ingest-helper-text">Switch to the StarCraft replay folder we found on this computer:</span>
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
              ) : detectedIsCurrent ? (
                <span className="ingest-helper-text">You are on the StarCraft replay folder we found on this computer.</span>
              ) : (
                <span className="ingest-helper-text">Set your replay folder above to analyze your own games.</span>
              )}
            </div>

            <div className="ingest-plain-block ingest-col">
              <div className="ingest-title">Example replays</div>
              {!isSampleSet ? (
                <div className="ingest-sample-col">
                  <button
                    type="button"
                    className="btn-save ingest-load-sample"
                    disabled={busy}
                    onClick={onLoadSampleSet}
                  >
                    {sampleSetLoading ? 'Switching to example replays...' : 'Load example replays'}
                  </button>
                  <span className="ingest-helper-text">A few example games to try every feature. Switches the replay folder to the bundled examples; your own .rep files are not touched.</span>
                </div>
              ) : (
                <div className="ingest-sample-col">
                  <span className="ingest-helper-text ingest-sample-active">You are using the built-in example replays.</span>
                  <button
                    type="button"
                    className="btn-save ingest-load-sample"
                    disabled={busy}
                    onClick={onLoadSampleSet}
                  >
                    {sampleSetLoading ? 'Switching to example replays...' : 'Reload example replays'}
                  </button>
                </div>
              )}
            </div>
          </div>

          <div className="ingest-plain-block ingest-progress-block">
            <div className="ingest-plain-heading">
              <div className="ingest-title">Progress</div>
              <div className={`ingest-status ingest-status-${STATUS_CLASS[status] || 'idle'}`}>{phase}</div>
            </div>
            <div
              className={`library-progress-bar${loading ? ' library-progress-bar--active' : ''}`}
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={barPercent}
            >
              <div className="library-progress-bar-fill" style={{ width: `${barPercent}%` }} />
            </div>
            <div className="library-progress-text">
              <span>{formatLoaded(loaded, total)}</span>
              {failed > 0 ? <span className="library-progress-failed">{formatCount(failed)} failed</span> : null}
              {skipped > 0 ? <span className="library-progress-skipped">{formatCount(skipped)} skipped</span> : null}
              {progress?.replay_dir ? (
                <span className="library-progress-dir" title={progress.replay_dir}>{progress.replay_dir}</span>
              ) : null}
            </div>
            <div className="ingest-log-panel ingest-log-panel-plain" role="log" aria-live="polite">
              {logs.length === 0 ? (
                <div className="ingest-log-empty">Logs will appear here as replays load.</div>
              ) : (
                logs.map((entry, idx) => (
                  <div key={`library-log-${idx}`} className={`ingest-log-line ingest-log-${entry.level || 'info'}`}>
                    {entry.message}
                  </div>
                ))
              )}
            </div>
            <span className="ingest-helper-text">Log stream: {librarySocketState}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default ReplayLibraryModal;
