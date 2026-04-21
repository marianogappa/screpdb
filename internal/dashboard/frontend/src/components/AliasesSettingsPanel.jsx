import React, { useMemo } from 'react';

function AliasesSettingsPanel({
  savedIngestInputDir,
  aliases,
  aliasesLoading,
  aliasesMessage,
  aliasesMessageIsError,
  aliasForm,
  aliasSaving,
  aliasSourceFilter,
  aliasEditOriginal,
  onAliasFormChange,
  onAliasSave,
  onAliasDelete,
  onAliasImportFile,
  onAliasSourceFilterChange,
  onAliasEdit,
  onAliasCancelEdit,
  onAliasExport,
}) {
  const filteredAliases = useMemo(() => {
    const list = Array.isArray(aliases) ? aliases : [];
    if (aliasSourceFilter === 'all') {
      return list;
    }
    return list.filter((row) => String(row.source || '').toLowerCase() === aliasSourceFilter);
  }, [aliases, aliasSourceFilter]);

  return (
    <div className="ingest-section">
      {aliasesMessage ? (
        <div className={aliasesMessageIsError ? 'error-message' : 'success-message'}>{aliasesMessage}</div>
      ) : null}
      <div className="ingest-header">
        <div>
          <div className="ingest-title">Alias management</div>
          <div className="ingest-subtitle">
            Review player aliases, import JSON shared with this app, export a snapshot, and edit or remove rows.
            The canonical alias <code>you</code> is filled automatically from the first <code>CSettings.json</code> found
            when walking up from the saved replay folder (StarCraft: Remastered; usually alongside <code>Maps/Replays</code>).
            Only the <code>account</code> string from recent-gateway login objects in that file is used.
            Canonical alias must differ from battle tag for every row.
          </div>
        </div>
      </div>
      {String(savedIngestInputDir || '').trim() ? (
        <div className="aliases-you-hint ingest-helper-text">
          Saved replay folder: <span className="aliases-you-hint-path">{savedIngestInputDir}</span>
        </div>
      ) : (
        <div className="aliases-you-hint ingest-helper-text">
          Save a replay folder from <strong>Ingest</strong> so automatic <code>you</code> aliases can resolve from <code>CSettings.json</code>.
        </div>
      )}
      {aliasEditOriginal ? (
        <div className="aliases-editing-banner">
          Editing alias #{aliasEditOriginal.id}
          <button type="button" className="btn-cancel aliases-editing-cancel" onClick={onAliasCancelEdit}>Cancel edit</button>
        </div>
      ) : null}
      <div className="ingest-grid">
        <label className="ingest-field">
          <span>Canonical alias</span>
          <input type="text" value={aliasForm.canonical_alias} onChange={(e) => onAliasFormChange({ ...aliasForm, canonical_alias: e.target.value })} placeholder="e.g. Bisu or you" />
        </label>
        <label className="ingest-field">
          <span>Battle tag</span>
          <input type="text" value={aliasForm.battle_tag} onChange={(e) => onAliasFormChange({ ...aliasForm, battle_tag: e.target.value })} placeholder="e.g. lIlIlIlIIIll" />
        </label>
        <label className="ingest-field">
          <span>Aurora ID (optional)</span>
          <input type="number" value={aliasForm.aurora_id} onChange={(e) => onAliasFormChange({ ...aliasForm, aurora_id: e.target.value })} placeholder="optional" />
        </label>
      </div>
      <div className="form-actions aliases-form-actions">
        <button type="button" className="btn-save" disabled={aliasSaving} onClick={onAliasSave}>
          {aliasSaving ? 'Saving...' : aliasEditOriginal ? 'Save Changes' : 'Add Alias'}
        </button>
        <label className="btn-switch" style={{ cursor: 'pointer' }}>
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
        <button type="button" className="btn-switch" onClick={onAliasExport} disabled={!aliases || aliases.length === 0}>
          Export JSON
        </button>
      </div>

      <div className="aliases-toolbar">
        <label className="aliases-filter">
          <span>Source</span>
          <select value={aliasSourceFilter} onChange={(e) => onAliasSourceFilterChange(e.target.value)}>
            <option value="all">All</option>
            <option value="you">you (CSettings)</option>
            <option value="manual">manual</option>
            <option value="imported">imported</option>
          </select>
        </label>
        <span className="ingest-helper-text aliases-count">
          {filteredAliases.length}
          {' '}
          shown
        </span>
      </div>

      {aliasesLoading ? (
        <div className="loading">Loading aliases...</div>
      ) : (
        <div className="aliases-table-wrap">
          <table className="aliases-table">
            <thead>
              <tr>
                <th>Battle tag</th>
                <th>Canonical</th>
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
    </div>
  );
}

export default AliasesSettingsPanel;
