import React, { useState, useEffect } from 'react';

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

function EditWidgetModal({ widget, onClose, onSave }) {
  const [prompt, setPrompt] = useState('');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [query, setQuery] = useState('');
  const [config, setConfig] = useState({
    type: 'table',
  });

  useEffect(() => {
    if (widget) {
      setPrompt(widget.prompt || '');
      setName(widget.name || '');
      setDescription(widget.description?.valid ? widget.description.string || '' : '');
      setQuery(widget.query || '');
      setConfig(widget.config || { type: 'table' });
    }
  }, [widget]);

  const updateConfig = (field, value) => {
    setConfig(prev => ({ ...prev, [field]: value }));
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    onSave({
      prompt,
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
            <div className="form-group">
              <label>Refine via AI with Prompt</label>
              <input
                type="text"
                value={prompt || ''}
                onChange={(e) => setPrompt(e.target.value)}
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

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Edit Widget</h2>
          <button onClick={onClose} className="btn-close">×</button>
        </div>

        <form onSubmit={handleSubmit} className="edit-form">
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

          <div className="form-group">
            <label>SQL Query</label>
            <textarea
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              rows="8"
              className="form-textarea"
            />
          </div>

          <div className="form-actions">
            <button type="button" onClick={onClose} className="btn-cancel">
              Cancel
            </button>
            <button type="submit" className="btn-save">
              Save
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default EditWidgetModal;

