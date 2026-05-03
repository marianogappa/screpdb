import React, { useEffect, useState } from 'react';
import AliasesSettingsPanel from './AliasesSettingsPanel';

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
  aliases,
  aliasesLoading,
  aliasesMessage,
  aliasesMessageIsError,
  aliasForm,
  aliasSaving,
  aliasSources,
  aliasEditOriginal,
  onAliasFormChange,
  onAliasSave,
  onAliasDelete,
  onAliasImportFile,
  onAliasSourcesToggle,
  onAliasEdit,
  onAliasCancelEdit,
  onAliasExport,
  customDashboardsEnabled = false,
}) {
  const [formState, setFormState] = useState(DEFAULT_CONFIG);
  const [settingsTab, setSettingsTab] = useState('scope');

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
              aria-selected={settingsTab === 'scope'}
              className={`workflow-production-tab${settingsTab === 'scope' ? ' workflow-production-tab-active' : ''}`}
              onClick={() => setSettingsTab('scope')}
            >
              Replay Filtering
            </button>
            <button
              type="button"
              role="tab"
              aria-selected={settingsTab === 'aliases'}
              className={`workflow-production-tab${settingsTab === 'aliases' ? ' workflow-production-tab-active' : ''}`}
              onClick={() => setSettingsTab('aliases')}
            >
              Aliases
            </button>
          </div>
        </div>
        {settingsTab === 'scope' ? (
          <form onSubmit={handleSubmit} className="edit-form settings-modal-tab-panel">
            {error ? <div className="error-message">{error}</div> : null}
            {customDashboardsEnabled ? (
              <div className="global-filter-note">
                Custom Dashboards are configured separately.
              </div>
            ) : null}

            <div className="global-filter-toggle-grid">
              <label className="global-filter-toggle">
                <input
                  type="checkbox"
                  checked={formState.exclude_short_games}
                  onChange={(e) => setFormState((prev) => ({ ...prev, exclude_short_games: e.target.checked }))}
                />
                <span>Exclude games that last less than 2 minutes</span>
              </label>
              <label className="global-filter-toggle">
                <input
                  type="checkbox"
                  checked={formState.exclude_computers}
                  onChange={(e) => setFormState((prev) => ({ ...prev, exclude_computers: e.target.checked }))}
                />
                <span>Exclude games with Computers</span>
              </label>
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
            <AliasesSettingsPanel
              aliases={aliases}
              aliasesLoading={aliasesLoading}
              aliasesMessage={aliasesMessage}
              aliasesMessageIsError={aliasesMessageIsError}
              aliasForm={aliasForm}
              aliasSaving={aliasSaving}
              aliasSources={aliasSources}
              aliasEditOriginal={aliasEditOriginal}
              onAliasFormChange={onAliasFormChange}
              onAliasSave={onAliasSave}
              onAliasDelete={onAliasDelete}
              onAliasImportFile={onAliasImportFile}
              onAliasSourcesToggle={onAliasSourcesToggle}
              onAliasEdit={onAliasEdit}
              onAliasCancelEdit={onAliasCancelEdit}
              onAliasExport={onAliasExport}
            />
          </div>
        )}
      </div>
    </div>
  );
}

export default GlobalReplayFilterModal;
