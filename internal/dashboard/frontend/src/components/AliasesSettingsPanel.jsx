import React, { useMemo } from 'react';

const SOURCE_OPTIONS = [
  { value: 'you', label: 'you (CSettings)' },
  { value: 'manual', label: 'manual' },
  { value: 'imported', label: 'imported' },
];

function AliasesSettingsPanel({
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
}) {
  const filteredAliases = useMemo(() => {
    const list = Array.isArray(aliases) ? aliases : [];
    const selected = Array.isArray(aliasSources) ? aliasSources : [];
    if (selected.length === 0) {
      return [];
    }
    return list.filter((row) => selected.includes(String(row.source || '').toLowerCase()));
  }, [aliases, aliasSources]);

  return (
    <div className="aliases-settings-root">
      {aliasesMessage ? (
        <div className={aliasesMessageIsError ? 'error-message' : 'success-message'}>{aliasesMessage}</div>
      ) : null}
      {aliasEditOriginal ? (
        <div className="aliases-editing-banner">
          Editing alias #{aliasEditOriginal.id}
          <button type="button" className="btn-cancel aliases-editing-cancel" onClick={onAliasCancelEdit}>Cancel edit</button>
        </div>
      ) : null}

      <div className="aliases-inline-form">
        <label className="aliases-inline-field">
          <span>Alias</span>
          <input type="text" value={aliasForm.canonical_alias} onChange={(e) => onAliasFormChange({ ...aliasForm, canonical_alias: e.target.value })} placeholder="e.g. Bisu or you" />
        </label>
        <label className="aliases-inline-field">
          <span>Name in replay</span>
          <input type="text" value={aliasForm.battle_tag} onChange={(e) => onAliasFormChange({ ...aliasForm, battle_tag: e.target.value })} placeholder="e.g. lIlIlIlIIIll" />
        </label>
        <label className="aliases-inline-field">
          <span>Aurora ID (optional)</span>
          <input type="number" value={aliasForm.aurora_id} onChange={(e) => onAliasFormChange({ ...aliasForm, aurora_id: e.target.value })} placeholder="optional" />
        </label>
        <div className="aliases-inline-add-wrap">
          <button type="button" className="btn-save" disabled={aliasSaving} onClick={onAliasSave}>
            {aliasSaving ? 'Saving...' : aliasEditOriginal ? 'Save' : 'Add'}
          </button>
        </div>
      </div>

      <div className="aliases-source-checkboxes" role="group" aria-label="Filter by source">
        {SOURCE_OPTIONS.map((opt) => (
          <label key={opt.value} className="aliases-source-check">
            <input
              type="checkbox"
              checked={aliasSources.includes(opt.value)}
              onChange={() => onAliasSourcesToggle(opt.value)}
            />
            <span>{opt.label}</span>
          </label>
        ))}
        <span className="ingest-helper-text aliases-count">
          {filteredAliases.length}
          {' '}
          shown
        </span>
      </div>

      {aliasesLoading ? (
        <div className="loading">Loading aliases...</div>
      ) : (
        <div className="aliases-table-wrap aliases-table-wrap-plain">
          <table className="aliases-table">
            <thead>
              <tr>
                <th>Name in replay</th>
                <th>Alias</th>
                <th>Source</th>
                <th>Updated</th>
                <th className="aliases-table-actions"> </th>
              </tr>
            </thead>
            <tbody>
              {filteredAliases.map((row) => (
                <tr key={row.id} className={aliasEditOriginal && aliasEditOriginal.id === row.id ? 'aliases-row-active' : ''}>
                  <td className="aliases-cell-mono">{row.battle_tag_raw}</td>
                  <td>{row.canonical_alias}</td>
                  <td><span className={`aliases-source-pill aliases-source-${row.source}`}>{row.source}</span></td>
                  <td className="aliases-cell-muted">{row.updated_at || '—'}</td>
                  <td className="aliases-table-actions">
                    <button type="button" className="btn-switch aliases-row-btn" onClick={() => onAliasEdit(row)}>Edit</button>
                    <button type="button" className="btn-cancel aliases-row-btn" onClick={() => onAliasDelete(row.id)}>Remove</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {filteredAliases.length === 0 ? <div className="ingest-log-empty">No aliases match this filter.</div> : null}
        </div>
      )}

      <div className="aliases-io-actions">
        <label className="btn-switch aliases-io-btn">
          Import JSON
          <input
            type="file"
            accept="application/json,.json"
            style={{ display: 'none' }}
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) onAliasImportFile(file);
              e.target.value = '';
            }}
          />
        </label>
        <button type="button" className="btn-switch aliases-io-btn" onClick={onAliasExport} disabled={!aliases || aliases.length === 0}>
          Export JSON
        </button>
      </div>
    </div>
  );
}

export default AliasesSettingsPanel;
