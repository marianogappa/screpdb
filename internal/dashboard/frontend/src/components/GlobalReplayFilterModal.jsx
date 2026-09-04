import React, { useEffect, useState } from 'react';
import LabsSettingsPanel from './LabsSettingsPanel';

const GAME_TYPE_OPTIONS = [
  { value: 'top_vs_bottom', label: 'Top vs Bottom' },
  { value: 'melee', label: 'Melee' },
  { value: 'one_on_one', label: 'One on One' },
  { value: 'free_for_all', label: 'Free For All' },
];

const MAP_KIND_OPTIONS = [
  { value: 'regular', label: 'Regular' },
  { value: 'money', label: 'Money' },
];

const ALL_GAME_TYPES = GAME_TYPE_OPTIONS.map((o) => o.value);
const ALL_MAP_KINDS = MAP_KIND_OPTIONS.map((o) => o.value);

const DEFAULT_CONFIG = {
  game_types: [...ALL_GAME_TYPES],
  exclude_short_games: true,
  exclude_computers: true,
  map_kinds: [...ALL_MAP_KINDS],
};

const normalizeStringList = (values) => {
  if (!Array.isArray(values)) return [];
  return Array.from(
    new Set(
      values
        .map((value) => String(value || '').trim().toLowerCase())
        .filter(Boolean),
    ),
  ).sort((a, b) => a.localeCompare(b));
};

const normalizeConfig = (config) => ({
  game_types: normalizeStringList(config?.game_types),
  exclude_short_games: config?.exclude_short_games !== false,
  exclude_computers: config?.exclude_computers !== false,
  map_kinds: normalizeStringList(config?.map_kinds),
});

function PillRow({ heading, options, selectedValues, onToggle }) {
  const selected = Array.isArray(selectedValues) ? selectedValues : [];
  return (
    <div className="global-filter-dimension">
      <h3>{heading}</h3>
      <div className="global-filter-option-list">
        {options.map((option) => {
          const isSelected = selected.includes(option.value);
          const className = `workflow-filter-pill${isSelected ? ' workflow-filter-pill-active' : ''}`;
          return (
            <button
              key={`${heading}-${option.value}`}
              type="button"
              className={className}
              onClick={() => onToggle(option.value)}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function GlobalReplayFilterModal({
  config,
  saving,
  error,
  onClose,
  onSave,
  featureFlags,
  featureFlagsSaving,
  featureFlagsMessage,
  featureFlagsMessageIsError,
  onFeatureFlagToggle,
  libraryMessage,
  replayDirInput,
  savedReplayDir,
  librarySettingsLoading,
  librarySettingsSaving,
  isSampleSet,
  detectedReplayDir,
  sampleSetLoading,
  onReplayDirChange,
  onSaveReplayDir,
  onLoadSampleSet,
  onUseDetectedFolder,
  onDismissMessage,
}) {
  const [formState, setFormState] = useState(DEFAULT_CONFIG);
  const [settingsTab, setSettingsTab] = useState('folder');

  const folderBusy = librarySettingsLoading || librarySettingsSaving || sampleSetLoading;
  const replayDirDirty = String(replayDirInput || '').trim() !== String(savedReplayDir || '').trim();
  const detectedIsCurrent = Boolean(detectedReplayDir) && String(detectedReplayDir).trim() === String(savedReplayDir || '').trim();
  // Colour by the message itself, not the library status: an error can arrive
  // while the folder is still happily loading, and it must still show red.
  const libraryMessageIsSuccess = libraryMessage === 'Replay folder saved.' || libraryMessage === 'Switched to the example replays.';

  useEffect(() => {
    setFormState(normalizeConfig(config || DEFAULT_CONFIG));
  }, [config]);

  const toggleArrayValue = (field, value) => {
    setFormState((prev) => {
      const current = Array.isArray(prev[field]) ? prev[field] : [];
      const next = current.includes(value)
        ? current.filter((entry) => entry !== value)
        : [...current, value].sort((a, b) => a.localeCompare(b));
      return {
        ...prev,
        [field]: next,
      };
    });
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    onSave(normalizeConfig(formState));
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content global-filter-modal settings-filter-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header settings-modal-header">
          <div className="settings-modal-header-row">
            <h2>Settings</h2>
            <button type="button" onClick={onClose} className="btn-close">×</button>
          </div>
          <div className="workflow-production-tabs settings-modal-main-tabs" role="tablist">
            <button
              type="button"
              role="tab"
              aria-selected={settingsTab === 'folder'}
              className={`workflow-production-tab${settingsTab === 'folder' ? ' workflow-production-tab-active' : ''}`}
              onClick={() => setSettingsTab('folder')}
            >
              Replay Folder
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={settingsTab === 'scope'}
              className={`workflow-production-tab${settingsTab === 'scope' ? ' workflow-production-tab-active' : ''}`}
              onClick={() => setSettingsTab('scope')}
            >
              Replay Filtering
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={settingsTab === 'labs'}
              className={`workflow-production-tab${settingsTab === 'labs' ? ' workflow-production-tab-active' : ''}`}
              onClick={() => setSettingsTab('labs')}
            >
              Labs
            </button>
          </div>
        </div>
        {settingsTab === 'folder' ? (
          <div className="edit-form ingest-form ingest-form-plain settings-modal-tab-panel">
            {libraryMessage ? (
              <div
                className={`ingest-header-message ${libraryMessageIsSuccess ? 'is-success' : 'is-error'}`}
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

            <div className="ingest-plain-block">
              <div className="ingest-title">Replay folder path</div>
              <div className="ingest-field ingest-path-field">
                <div className="ingest-path-row">
                  <input
                    type="text"
                    value={replayDirInput}
                    placeholder={librarySettingsLoading ? 'Loading replay folder...' : '/path/to/replays'}
                    disabled={folderBusy}
                    onChange={(e) => onReplayDirChange(e.target.value)}
                  />
                  <button
                    type="button"
                    className="btn-save"
                    disabled={folderBusy || !replayDirDirty}
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
                      disabled={folderBusy}
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
                      disabled={folderBusy}
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
                      disabled={folderBusy}
                      onClick={onLoadSampleSet}
                    >
                      {sampleSetLoading ? 'Switching to example replays...' : 'Reload example replays'}
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
        ) : settingsTab === 'scope' ? (
          <form onSubmit={handleSubmit} className="edit-form settings-modal-tab-panel">
            {error ? <div className="error-message">{error}</div> : null}

            <div className="global-filter-dimension">
              <h3>Exclude</h3>
              <div className="global-filter-toggle-grid">
                <label className="global-filter-toggle">
                  <input
                    type="checkbox"
                    checked={formState.exclude_short_games}
                    onChange={(e) => setFormState((prev) => ({ ...prev, exclude_short_games: e.target.checked }))}
                  />
                  <span>Games that last less than 2 minutes</span>
                </label>
                <label className="global-filter-toggle">
                  <input
                    type="checkbox"
                    checked={formState.exclude_computers}
                    onChange={(e) => setFormState((prev) => ({ ...prev, exclude_computers: e.target.checked }))}
                  />
                  <span>Games with Computers</span>
                </label>
              </div>
            </div>

            <PillRow
              heading="Game Type"
              options={GAME_TYPE_OPTIONS}
              selectedValues={formState.game_types}
              onToggle={(value) => toggleArrayValue('game_types', value)}
            />

            <PillRow
              heading="Map Type"
              options={MAP_KIND_OPTIONS}
              selectedValues={formState.map_kinds}
              onToggle={(value) => toggleArrayValue('map_kinds', value)}
            />

            <div className="form-actions">
              <button type="button" onClick={onClose} className="btn-cancel">
                Cancel
              </button>
              <button type="submit" className="btn-save" disabled={saving}>
                {saving ? 'Saving...' : 'Save'}
              </button>
            </div>
          </form>
        ) : (
          <div className="edit-form ingest-form settings-modal-tab-panel">
            <LabsSettingsPanel
              flags={featureFlags}
              saving={featureFlagsSaving}
              message={featureFlagsMessage}
              messageIsError={featureFlagsMessageIsError}
              onToggle={onFeatureFlagToggle}
            />
          </div>
        )}
      </div>
    </div>
  );
}

export default GlobalReplayFilterModal;
