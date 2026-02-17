import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import SqlEditor from './SqlEditor';
import QueryBuilder from './QueryBuilder';
import { renderChart } from '../utils/chartRenderer';
import { WIDGET_TYPES, CHART_TYPE_FIELDS } from '../constants/chartTypes';

function EditWidgetFullscreen({ widget, onClose, onSave, dashboardUrl }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [query, setQuery] = useState('');
  const [config, setConfig] = useState({ type: 'table' });
  const [previewData, setPreviewData] = useState([]);
  const [previewColumns, setPreviewColumns] = useState([]);
  const [previewError, setPreviewError] = useState(null);
  const [isExecuting, setIsExecuting] = useState(false);
  const [lastExecutedQuery, setLastExecutedQuery] = useState('');
  const [variables, setVariables] = useState({});
  const [variableValues, setVariableValues] = useState({});
  const [activeTab, setActiveTab] = useState('query');
  const [queryMode, setQueryMode] = useState('sql');

  useEffect(() => {
    if (widget) {
      setName(widget.name || '');
      setDescription(widget.description?.valid ? widget.description.string || '' : '');
      setQuery(widget.query || '');
      setConfig(widget.config || { type: 'table' });
      if (widget.results) setPreviewData(widget.results);
      if (widget.columns) setPreviewColumns(widget.columns);
    }
  }, [widget]);

  useEffect(() => {
    const fetchVariables = async () => {
      if (!query.trim()) { setVariables({}); setVariableValues({}); return; }
      try {
        const response = await api.getQueryVariables(query, dashboardUrl);
        const vars = response.variables || {};
        setVariables(vars);
        setVariableValues(prev => {
          const newValues = { ...prev };
          Object.keys(vars).forEach(varName => {
            if (!newValues[varName] && vars[varName].possible_values?.length > 0) {
              newValues[varName] = vars[varName].possible_values[0];
            }
          });
          return newValues;
        });
      } catch {
        setVariables({});
      }
    };
    const timeoutId = setTimeout(fetchVariables, 300);
    return () => clearTimeout(timeoutId);
  }, [query, dashboardUrl]);

  const executeQuery = useCallback(async (sqlQuery, varValues = null) => {
    if (!sqlQuery.trim()) {
      setPreviewData([]); setPreviewColumns([]); setPreviewError(null);
      return;
    }
    setIsExecuting(true);
    setPreviewError(null);
    try {
      const response = await api.executeQuery(sqlQuery, varValues || variableValues, dashboardUrl);
      setPreviewData(response.results || []);
      setPreviewColumns(response.columns || []);
      setLastExecutedQuery(sqlQuery);
    } catch (err) {
      setPreviewError(err.message);
      setPreviewData([]);
      setPreviewColumns([]);
    } finally {
      setIsExecuting(false);
    }
  }, [variableValues, dashboardUrl]);

  useEffect(() => {
    const timeoutId = setTimeout(() => {
      if (query !== lastExecutedQuery) executeQuery(query);
    }, 500);
    return () => clearTimeout(timeoutId);
  }, [query, executeQuery, lastExecutedQuery, variableValues]);

  const handleVariableChange = (varName, value) => {
    setVariableValues(prev => ({ ...prev, [varName]: value }));
    if (query?.trim()) executeQuery(query, { ...variableValues, [varName]: value });
  };

  const updateConfig = (field, value) => {
    setConfig(prev => ({ ...prev, [field]: value }));
  };

  const handleSave = () => {
    onSave({ name, description: description || null, query, config });
  };

  const handleQueryFromBuilder = (sql) => {
    setQuery(sql);
    setQueryMode('sql');
    setActiveTab('query');
  };

  const renderColumnSelect = (field, currentValue) => {
    if (previewColumns.length === 0) {
      return (
        <input
          type="text" value={currentValue || ''} className="form-input"
          onChange={(e) => updateConfig(field.key, e.target.value)}
          placeholder="Run query first to see columns"
        />
      );
    }
    return (
      <select
        value={currentValue || ''} className="form-input"
        onChange={(e) => updateConfig(field.key, e.target.value)}
      >
        <option value="">Select column...</option>
        {previewColumns.map(col => (
          <option key={col} value={col}>{col}</option>
        ))}
      </select>
    );
  };

  const renderConfigFields = () => {
    const fields = CHART_TYPE_FIELDS[config.type] || [];
    if (fields.length === 0) return null;
    return fields.map(field => {
      if (field.type === 'checkbox') {
        return (
          <label key={field.key} className="form-checkbox-label">
            <input
              type="checkbox"
              checked={config[field.key] || false}
              onChange={(e) => updateConfig(field.key, e.target.checked)}
            />
            <span>{field.label}</span>
          </label>
        );
      }
      if (field.type === 'select') {
        return (
          <div key={field.key} className="form-group">
            <label>{field.label}</label>
            <select
              value={config[field.key] || field.options[0]?.value}
              onChange={(e) => updateConfig(field.key, e.target.value)}
              className="form-input"
            >
              {field.options.map(o => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </select>
          </div>
        );
      }
      if (field.type === 'column') {
        return (
          <div key={field.key} className="form-group">
            <label>{field.label}{field.required && <span className="required-mark">*</span>}</label>
            {renderColumnSelect(field, config[field.key])}
          </div>
        );
      }
      if (field.type === 'columns') {
        return (
          <div key={field.key} className="form-group">
            <label>{field.label}{field.required && <span className="required-mark">*</span>}</label>
            <input
              type="text"
              value={Array.isArray(config[field.key]) ? config[field.key].join(', ') : (config[field.key] || '')}
              onChange={(e) => updateConfig(field.key, e.target.value ? e.target.value.split(',').map(s => s.trim()).filter(s => s) : [])}
              className="form-input"
              placeholder={previewColumns.length > 0 ? previewColumns.join(', ') : 'col1, col2'}
            />
          </div>
        );
      }
      if (field.type === 'number') {
        return (
          <div key={field.key} className="form-group">
            <label>{field.label}</label>
            <input
              type="number"
              value={config[field.key] ?? ''}
              onChange={(e) => updateConfig(field.key, e.target.value ? parseFloat(e.target.value) : undefined)}
              className="form-input"
            />
          </div>
        );
      }
      return (
        <div key={field.key} className="form-group">
          <label>{field.label}</label>
          <input
            type="text"
            value={config[field.key] || ''}
            onChange={(e) => updateConfig(field.key, e.target.value)}
            className="form-input"
          />
        </div>
      );
    });
  };

  const renderPreview = () => {
    if (previewError) {
      return <div className="chart-empty chart-error">Error: {previewError}</div>;
    }
    if (isExecuting) {
      return <div className="chart-empty">Executing query...</div>;
    }
    if (previewData.length === 0 && query.trim()) {
      return <div className="chart-empty">No data returned</div>;
    }
    if (previewData.length === 0) {
      return <div className="chart-empty">Write a SQL query to see a preview</div>;
    }
    return renderChart({ data: previewData, config, columns: previewColumns });
  };

  return (
    <div className="fullscreen-editor">
      <div className="fullscreen-editor-header">
        <div className="editor-header-title">
          <input
            type="text" value={name} onChange={(e) => setName(e.target.value)}
            className="editor-name-input" placeholder="Widget name..."
          />
        </div>
        <div className="fullscreen-editor-actions">
          <button onClick={handleSave} className="btn-primary">Save</button>
          <button onClick={onClose} className="btn-secondary">Discard</button>
        </div>
      </div>

      <div className="editor-tabs">
        <button
          className={`editor-tab ${activeTab === 'query' ? 'active' : ''}`}
          onClick={() => setActiveTab('query')}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>
          </svg>
          Query
        </button>
        <button
          className={`editor-tab ${activeTab === 'chart' ? 'active' : ''}`}
          onClick={() => setActiveTab('chart')}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M3 3v18h18"/><path d="M7 16l4-8 4 4 4-12"/>
          </svg>
          Chart Type
        </button>
        <button
          className={`editor-tab ${activeTab === 'details' ? 'active' : ''}`}
          onClick={() => setActiveTab('details')}
        >
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
          </svg>
          Details
        </button>
      </div>

      <div className="fullscreen-editor-content">
        <div className="fullscreen-editor-left">
          {activeTab === 'query' && (
            <div className="fullscreen-editor-section">
              <div className="query-mode-toggle">
                <button
                  className={`query-mode-btn ${queryMode === 'sql' ? 'active' : ''}`}
                  onClick={() => setQueryMode('sql')}
                >
                  SQL Editor
                </button>
                <button
                  className={`query-mode-btn ${queryMode === 'builder' ? 'active' : ''}`}
                  onClick={() => setQueryMode('builder')}
                >
                  Visual Builder
                </button>
              </div>

              {queryMode === 'sql' ? (
                <>
                  {Object.keys(variables).length > 0 && (
                    <div className="editor-variables">
                      <div className="editor-variables-title">Variables</div>
                      <div className="editor-variables-grid">
                        {Object.entries(variables).map(([varName, variable]) => (
                          <div key={varName} className="editor-variable-item">
                            <label>{variable.display_name}</label>
                            <select
                              value={variableValues[varName] || ''}
                              onChange={(e) => handleVariableChange(varName, e.target.value)}
                              className="form-input"
                            >
                              {variable.possible_values?.map((value, idx) => (
                                <option key={idx} value={value}>{value}</option>
                              ))}
                            </select>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  <SqlEditor
                    value={query} onChange={setQuery}
                    placeholder="SELECT * FROM replays..."
                    className="sql-editor"
                  />
                  {isExecuting && <div className="query-status">Executing...</div>}
                  {previewData.length > 0 && !isExecuting && (
                    <div className="query-status query-status-success">
                      {previewData.length} row{previewData.length !== 1 ? 's' : ''} returned
                      {previewColumns.length > 0 && ` | ${previewColumns.length} column${previewColumns.length !== 1 ? 's' : ''}: ${previewColumns.join(', ')}`}
                    </div>
                  )}
                </>
              ) : (
                <QueryBuilder
                  onQueryGenerated={handleQueryFromBuilder}
                  initialMode="visual"
                />
              )}
            </div>
          )}

          {activeTab === 'chart' && (
            <div className="fullscreen-editor-section">
              <div className="chart-type-grid">
                {WIDGET_TYPES.map(type => (
                  <button
                    key={type.value}
                    className={`chart-type-card ${config.type === type.value ? 'active' : ''}`}
                    onClick={() => setConfig(prev => ({ ...prev, type: type.value }))}
                  >
                    <span className="chart-type-name">{type.label}</span>
                    <span className="chart-type-desc">{type.description}</span>
                  </button>
                ))}
              </div>

              <div className="chart-config-section">
                {renderConfigFields()}
              </div>
            </div>
          )}

          {activeTab === 'details' && (
            <div className="fullscreen-editor-section">
              <div className="form-group">
                <label>Widget Name</label>
                <input
                  type="text" value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="form-input" required
                />
              </div>
              <div className="form-group">
                <label>Description</label>
                <input
                  type="text" value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="form-input" placeholder="What does this widget show?"
                />
              </div>
            </div>
          )}
        </div>

        <div className="fullscreen-editor-right">
          <div className="fullscreen-editor-preview">
            <div className="preview-header">
              <h3>Preview</h3>
              {previewData.length > 0 && (
                <span className="preview-badge">{previewData.length} rows</span>
              )}
            </div>
            <div className="fullscreen-editor-preview-content">
              {renderPreview()}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default EditWidgetFullscreen;
