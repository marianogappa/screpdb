import React, { useState, useEffect, useCallback } from 'react';
import { api } from '../api';
import SqlEditor from './SqlEditor';
import Gauge from './charts/Gauge';
import Table from './charts/Table';
import PieChart from './charts/PieChart';
import BarChart from './charts/BarChart';
import LineChart from './charts/LineChart';
import ScatterPlot from './charts/ScatterPlot';
import Histogram from './charts/Histogram';
import Heatmap from './charts/Heatmap';

const WIDGET_TYPES = [
  { value: 'gauge', label: 'Gauge' },
  { value: 'table', label: 'Table' },
  { value: 'pie_chart', label: 'Pie Chart' },
  { value: 'bar_chart', label: 'Bar Chart' },
  { value: 'line_chart', label: 'Line Chart' },
  { value: 'scatter_plot', label: 'Scatter Plot' },
  { value: 'histogram', label: 'Histogram' },
  { value: 'heatmap', label: 'Heatmap' },
];

function EditWidgetFullscreen({ widget, onClose, onSave, dashboardUrl }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [query, setQuery] = useState('');
  const [config, setConfig] = useState({
    type: 'table',
  });
  const [previewData, setPreviewData] = useState([]);
  const [previewColumns, setPreviewColumns] = useState([]);
  const [previewError, setPreviewError] = useState(null);
  const [isExecuting, setIsExecuting] = useState(false);
  const [lastExecutedQuery, setLastExecutedQuery] = useState('');
  const [variables, setVariables] = useState({});
  const [variableValues, setVariableValues] = useState({});

  useEffect(() => {
    if (widget) {
      setName(widget.name || '');
      setDescription(widget.description?.valid ? widget.description.string || '' : '');
      setQuery(widget.query || '');
      setConfig(widget.config || { type: 'table' });
      if (widget.results) {
        setPreviewData(widget.results);
      }
      if (widget.columns) {
        setPreviewColumns(widget.columns);
      }
    }
  }, [widget]);

  // Fetch variables when query changes
  useEffect(() => {
    const fetchVariables = async () => {
      if (!query.trim()) {
        setVariables({});
        setVariableValues({});
        return;
      }

      try {
        const response = await api.getQueryVariables(query, dashboardUrl);
        const vars = response.variables || {};
        setVariables(vars);
        
        // Initialize variable values with first option if not set
        setVariableValues(prev => {
          const newValues = { ...prev };
          Object.keys(vars).forEach(varName => {
            if (!newValues[varName] && vars[varName].possible_values?.length > 0) {
              newValues[varName] = vars[varName].possible_values[0];
            }
          });
          return newValues;
        });
      } catch (err) {
        console.error('Failed to fetch variables:', err);
        setVariables({});
      }
    };

    const timeoutId = setTimeout(fetchVariables, 300);
    return () => clearTimeout(timeoutId);
  }, [query, dashboardUrl]);

  const executeQuery = useCallback(async (sqlQuery, varValues = null) => {
    if (!sqlQuery.trim()) {
      setPreviewData([]);
      setPreviewColumns([]);
      setPreviewError(null);
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

  // Debounced query execution
  useEffect(() => {
    const timeoutId = setTimeout(() => {
      if (query !== lastExecutedQuery) {
        executeQuery(query);
      }
    }, 500); // 500ms debounce

    return () => clearTimeout(timeoutId);
  }, [query, executeQuery, lastExecutedQuery, variableValues]);

  const handleVariableChange = (varName, value) => {
    setVariableValues(prev => ({ ...prev, [varName]: value }));
    // Re-execute query with new variable value
    if (query && query.trim()) {
      executeQuery(query, { ...variableValues, [varName]: value });
    }
  };

  const updateConfig = (field, value) => {
    setConfig(prev => ({ ...prev, [field]: value }));
  };

  const handleSave = () => {
    onSave({
      name,
      description: description || null,
      query,
      config,
    });
  };

  const renderTypeSpecificFields = () => {
    switch (config.type) {
      case 'gauge':
        return (
          <>
            <div className="form-group">
              <label>Value Column</label>
              <input
                type="text"
                value={config.gauge_value_column || ''}
                onChange={(e) => updateConfig('gauge_value_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Min Value (optional)</label>
              <input
                type="number"
                value={config.gauge_min || ''}
                onChange={(e) => updateConfig('gauge_min', e.target.value ? parseFloat(e.target.value) : undefined)}
                className="form-input"
              />
            </div>
            <div className="form-group">
              <label>Max Value (optional)</label>
              <input
                type="number"
                value={config.gauge_max || ''}
                onChange={(e) => updateConfig('gauge_max', e.target.value ? parseFloat(e.target.value) : undefined)}
                className="form-input"
              />
            </div>
            <div className="form-group">
              <label>Label (optional)</label>
              <input
                type="text"
                value={config.gauge_label || ''}
                onChange={(e) => updateConfig('gauge_label', e.target.value)}
                className="form-input"
              />
            </div>
          </>
        );
      case 'table':
        return null;
      case 'pie_chart':
        return (
          <>
            <div className="form-group">
              <label>Label Column</label>
              <input
                type="text"
                value={config.pie_label_column || ''}
                onChange={(e) => updateConfig('pie_label_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Value Column</label>
              <input
                type="text"
                value={config.pie_value_column || ''}
                onChange={(e) => updateConfig('pie_value_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
          </>
        );
      case 'bar_chart':
        return (
          <>
            <div className="form-group">
              <label>Label Column</label>
              <input
                type="text"
                value={config.bar_label_column || ''}
                onChange={(e) => updateConfig('bar_label_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Value Column</label>
              <input
                type="text"
                value={config.bar_value_column || ''}
                onChange={(e) => updateConfig('bar_value_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>
                <input
                  type="checkbox"
                  checked={config.bar_horizontal || false}
                  onChange={(e) => updateConfig('bar_horizontal', e.target.checked)}
                />
                {' '}Horizontal bars
              </label>
            </div>
          </>
        );
      case 'line_chart':
        return (
          <>
            <div className="form-group">
              <label>X Column</label>
              <input
                type="text"
                value={config.line_x_column || ''}
                onChange={(e) => updateConfig('line_x_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Y Columns (comma-separated)</label>
              <input
                type="text"
                value={config.line_y_columns?.join(', ') || ''}
                onChange={(e) => updateConfig('line_y_columns', e.target.value ? e.target.value.split(',').map(s => s.trim()).filter(s => s) : [])}
                className="form-input"
                placeholder="col1, col2, col3"
              />
            </div>
            <div className="form-group">
              <label>X Axis Type</label>
              <select
                value={config.line_x_axis_type || 'numeric'}
                onChange={(e) => updateConfig('line_x_axis_type', e.target.value)}
                className="form-input"
              >
                <option value="numeric">Numeric</option>
                <option value="seconds_from_game_start">Seconds from Game Start</option>
                <option value="timestamp">Timestamp</option>
              </select>
            </div>
            <div className="form-group">
              <label>
                <input
                  type="checkbox"
                  checked={config.line_y_axis_from_zero || false}
                  onChange={(e) => updateConfig('line_y_axis_from_zero', e.target.checked)}
                />
                {' '}Y axis starts from zero
              </label>
            </div>
          </>
        );
      case 'scatter_plot':
        return (
          <>
            <div className="form-group">
              <label>X Column</label>
              <input
                type="text"
                value={config.scatter_x_column || ''}
                onChange={(e) => updateConfig('scatter_x_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Y Column</label>
              <input
                type="text"
                value={config.scatter_y_column || ''}
                onChange={(e) => updateConfig('scatter_y_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Size Column (optional)</label>
              <input
                type="text"
                value={config.scatter_size_column || ''}
                onChange={(e) => updateConfig('scatter_size_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Color Column (optional)</label>
              <input
                type="text"
                value={config.scatter_color_column || ''}
                onChange={(e) => updateConfig('scatter_color_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
          </>
        );
      case 'histogram':
        return (
          <>
            <div className="form-group">
              <label>Value Column</label>
              <input
                type="text"
                value={config.histogram_value_column || ''}
                onChange={(e) => updateConfig('histogram_value_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Bins (optional, auto if empty)</label>
              <input
                type="number"
                value={config.histogram_bins || ''}
                onChange={(e) => updateConfig('histogram_bins', e.target.value ? parseInt(e.target.value) : undefined)}
                className="form-input"
              />
            </div>
          </>
        );
      case 'heatmap':
        return (
          <>
            <div className="form-group">
              <label>X Column</label>
              <input
                type="text"
                value={config.heatmap_x_column || ''}
                onChange={(e) => updateConfig('heatmap_x_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Y Column</label>
              <input
                type="text"
                value={config.heatmap_y_column || ''}
                onChange={(e) => updateConfig('heatmap_y_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
            <div className="form-group">
              <label>Value Column</label>
              <input
                type="text"
                value={config.heatmap_value_column || ''}
                onChange={(e) => updateConfig('heatmap_value_column', e.target.value)}
                className="form-input"
                placeholder="column_name"
              />
            </div>
          </>
        );
      default:
        return null;
    }
  };

  const renderChart = () => {
    if (!config || !config.type) {
      return <div className="chart-empty">Select a widget type</div>;
    }

    if (previewError) {
      return <div className="chart-empty" style={{ color: '#ff6666' }}>Error: {previewError}</div>;
    }

    if (isExecuting) {
      return <div className="chart-empty">Executing query...</div>;
    }

    if (previewData.length === 0 && query.trim()) {
      return <div className="chart-empty">No data returned</div>;
    }

    if (previewData.length === 0) {
      return <div className="chart-empty">Enter a SQL query to see preview</div>;
    }

    const chartProps = {
      data: previewData,
      config: config,
    };

    switch (config.type) {
      case 'gauge':
        return <Gauge {...chartProps} />;
      case 'table':
        return <Table {...chartProps} columns={previewColumns} />;
      case 'pie_chart':
        return <PieChart {...chartProps} />;
      case 'bar_chart':
        return <BarChart {...chartProps} />;
      case 'line_chart':
        return <LineChart {...chartProps} />;
      case 'scatter_plot':
        return <ScatterPlot {...chartProps} />;
      case 'histogram':
        return <Histogram {...chartProps} />;
      case 'heatmap':
        return <Heatmap {...chartProps} />;
      default:
        return <div className="chart-empty">Unknown widget type: {config.type}</div>;
    }
  };

  return (
    <div className="fullscreen-editor">
      <div className="fullscreen-editor-header">
        <h2>Edit Widget: {name || 'Untitled'}</h2>
        <div className="fullscreen-editor-actions">
          <button onClick={handleSave} className="btn-save">
            Save
          </button>
          <button onClick={onClose} className="btn-cancel">
            Discard
          </button>
        </div>
      </div>

      <div className="fullscreen-editor-content">
        <div className="fullscreen-editor-left">
          <div className="fullscreen-editor-section">
            <h3>Widget Settings</h3>
            <div className="form-group">
              <label>Name</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                className="form-input"
              />
            </div>

            <div className="form-group">
              <label>Description</label>
              <input
                type="text"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="form-input"
              />
            </div>

            <div className="form-group">
              <label>Widget Type</label>
              <select
                value={config.type || 'table'}
                onChange={(e) => {
                  const newType = e.target.value;
                  setConfig({ ...config, type: newType });
                }}
                className="form-input"
              >
                {WIDGET_TYPES.map(type => (
                  <option key={type.value} value={type.value}>{type.label}</option>
                ))}
              </select>
            </div>

            {renderTypeSpecificFields()}
          </div>

          <div className="fullscreen-editor-section">
            <h3>SQL Query</h3>
            {Object.keys(variables).length > 0 && (
              <div className="form-group" style={{ marginBottom: '1rem' }}>
                <label>Variables</label>
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
                  {Object.entries(variables).map(([varName, variable]) => (
                    <div key={varName} style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                      <label htmlFor={`var-${varName}`} style={{ fontSize: '0.875rem', fontWeight: '500' }}>
                        {variable.display_name}
                      </label>
                      <select
                        id={`var-${varName}`}
                        value={variableValues[varName] || ''}
                        onChange={(e) => handleVariableChange(varName, e.target.value)}
                        style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #ccc', backgroundColor: '#1a1a1a', color: '#fff' }}
                      >
                        {variable.possible_values?.map((value, idx) => (
                          <option key={idx} value={value}>
                            {value}
                          </option>
                        ))}
                      </select>
                    </div>
                  ))}
                </div>
              </div>
            )}
            <div className="form-group">
              <SqlEditor
                value={query}
                onChange={setQuery}
                placeholder="SELECT * FROM ..."
                className="sql-editor"
              />
              {isExecuting && (
                <div className="query-status">Executing...</div>
              )}
            </div>
          </div>
        </div>

        <div className="fullscreen-editor-right">
          <div className="fullscreen-editor-preview">
            <h3>Preview</h3>
            <div className="fullscreen-editor-preview-content">
              {renderChart()}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default EditWidgetFullscreen;
