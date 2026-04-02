import React, { useEffect, useMemo, useState } from 'react';

const DEFAULT_CONFIG = {
  game_types: [],
  game_types_mode: 'only_these',
  exclude_short_games: true,
  exclude_computers: true,
  maps: [],
  map_filter_mode: 'only_these',
  players: [],
  player_filter_mode: 'only_these',
};

const GAME_TYPE_OPTIONS = [
  { value: 'top_vs_bottom', label: 'Top vs Bottom' },
  { value: 'melee', label: 'Melee' },
  { value: 'one_on_one', label: 'One on One' },
  { value: 'free_for_all', label: 'Free For All' },
  { value: 'ums', label: 'Use Map Settings' },
];

const FILTER_MODE_OPTIONS = [
  { value: 'only_these', label: 'Only these' },
  { value: 'all_except_these', label: 'All except these' },
];

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
  game_types_mode: String(config?.game_types_mode || DEFAULT_CONFIG.game_types_mode).trim().toLowerCase() || DEFAULT_CONFIG.game_types_mode,
  exclude_short_games: config?.exclude_short_games !== false,
  exclude_computers: config?.exclude_computers !== false,
  maps: normalizeStringList(config?.maps),
  map_filter_mode: String(config?.map_filter_mode || DEFAULT_CONFIG.map_filter_mode).trim().toLowerCase() || DEFAULT_CONFIG.map_filter_mode,
  players: normalizeStringList(config?.players),
  player_filter_mode: String(config?.player_filter_mode || DEFAULT_CONFIG.player_filter_mode).trim().toLowerCase() || DEFAULT_CONFIG.player_filter_mode,
});

function OptionGroup({ title, mode, onModeChange, selectedValues, onToggle, topOptions, otherOptions }) {
  const [showMore, setShowMore] = useState(false);
  const allOptions = useMemo(() => [...(topOptions || []), ...(otherOptions || [])], [topOptions, otherOptions]);
  const selected = Array.isArray(selectedValues) ? selectedValues : [];

  return (
    <div className="global-filter-option-group">
      <div className="global-filter-option-title">{title}</div>
      <div className="global-filter-mode-row">
        {FILTER_MODE_OPTIONS.map((option) => (
          <button
            key={`${title}-${option.value}`}
            type="button"
            className={`workflow-filter-pill${mode === option.value ? ' workflow-filter-pill-active' : ''}`}
            onClick={() => onModeChange(option.value)}
          >
            {option.label}
          </button>
        ))}
      </div>
      <div className="global-filter-option-list">
        {allOptions.length === 0 && (
          <div className="global-filter-empty">No options available yet.</div>
        )}
        {(topOptions || []).map((option) => (
          <button
            key={`${title}-${option.value}`}
            type="button"
            className={`workflow-filter-pill${selected.includes(option.value) ? ' workflow-filter-pill-active' : ''}`}
            onClick={() => onToggle(option.value)}
          >
            {option.label}
            {Number.isFinite(option.count) ? ` (${option.count})` : ''}
          </button>
        ))}
      </div>
      {(otherOptions || []).length > 0 && (
        <div className="global-filter-more">
          <button type="button" className="btn-switch" onClick={() => setShowMore((prev) => !prev)}>
            {showMore ? 'Hide More' : `Show More (${otherOptions.length})`}
          </button>
          {showMore && (
            <div className="global-filter-option-list global-filter-option-list-expanded">
              {otherOptions.map((option) => (
                <button
                  key={`${title}-more-${option.value}`}
                  type="button"
                  className={`workflow-filter-pill${selected.includes(option.value) ? ' workflow-filter-pill-active' : ''}`}
                  onClick={() => onToggle(option.value)}
                >
                  {option.label}
                  {Number.isFinite(option.count) ? ` (${option.count})` : ''}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function FilterDimension({ heading, mode, onModeChange, values, onToggle, topOptions, otherOptions }) {
  return (
    <div className="global-filter-dimension">
      <h3>{heading}</h3>
      <OptionGroup
        title={heading}
        mode={mode}
        onModeChange={onModeChange}
        selectedValues={values}
        onToggle={onToggle}
        topOptions={topOptions}
        otherOptions={otherOptions}
      />
    </div>
  );
}

function GlobalReplayFilterModal({ config, options, saving, error, onClose, onSave }) {
  const [formState, setFormState] = useState(DEFAULT_CONFIG);

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
      <div className="modal-content global-filter-modal" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Settings</h2>
          <button onClick={onClose} className="btn-close">×</button>
        </div>
        <form onSubmit={handleSubmit} className="edit-form">
          {error ? <div className="error-message">{error}</div> : null}
          <div className="global-filter-warning">
            These settings apply to the Games and Players dashboards. Custom Dashboards are configured separately.
          </div>

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

          <FilterDimension
            heading="Game Type"
            mode={formState.game_types_mode}
            onModeChange={(value) => setFormState((prev) => ({ ...prev, game_types_mode: value }))}
            values={formState.game_types}
            onToggle={(value) => toggleArrayValue('game_types', value)}
            topOptions={GAME_TYPE_OPTIONS}
            otherOptions={[]}
          />

          <FilterDimension
            heading="Maps"
            mode={formState.map_filter_mode}
            onModeChange={(value) => setFormState((prev) => ({ ...prev, map_filter_mode: value }))}
            values={formState.maps}
            onToggle={(value) => toggleArrayValue('maps', value)}
            topOptions={options?.top_maps || []}
            otherOptions={options?.other_maps || []}
          />

          <FilterDimension
            heading="Players"
            mode={formState.player_filter_mode}
            onModeChange={(value) => setFormState((prev) => ({ ...prev, player_filter_mode: value }))}
            values={formState.players}
            onToggle={(value) => toggleArrayValue('players', value)}
            topOptions={options?.top_players || []}
            otherOptions={options?.other_players || []}
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
      </div>
    </div>
  );
}

export default GlobalReplayFilterModal;
