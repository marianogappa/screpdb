import React, { useState, useEffect } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import WidgetCreationSpinner from './components/WidgetCreationSpinner';
import './styles.css';

// Helper functions for localStorage
const getStoredVariableValues = (dashboardUrl) => {
  try {
    const key = `dashboard_vars_${dashboardUrl}`;
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : null;
  } catch (e) {
    console.error('Failed to load variable values from localStorage:', e);
    return null;
  }
};

const saveVariableValues = (dashboardUrl, values) => {
  try {
    const key = `dashboard_vars_${dashboardUrl}`;
    localStorage.setItem(key, JSON.stringify(values));
  } catch (e) {
    console.error('Failed to save variable values to localStorage:', e);
  }
};

function App() {
  const [currentDashboardUrl, setCurrentDashboardUrl] = useState('default');
  const [dashboard, setDashboard] = useState(null);
  const [dashboards, setDashboards] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showDashboardManager, setShowDashboardManager] = useState(false);
  const [showEditDashboard, setShowEditDashboard] = useState(false);
  const [newWidgetPrompt, setNewWidgetPrompt] = useState('');
  const [creatingWidget, setCreatingWidget] = useState(false);
  const [variableValues, setVariableValues] = useState({});
  const [openaiEnabled, setOpenaiEnabled] = useState(false);
  const [editingWidget, setEditingWidget] = useState(null);
  const [replayCount, setReplayCount] = useState(null);
  const [showIngestPanel, setShowIngestPanel] = useState(false);
  const [ingestMessage, setIngestMessage] = useState('');
  const [ingestForm, setIngestForm] = useState({
    watch: false,
    stopAfterN: 50,
    clean: false,
  });

  const loadDashboard = async (url, varValues = null, skipVarInit = false) => {
    try {
      setLoading(true);
      setError(null);

      // If no varValues provided, try to load from localStorage
      if (!varValues) {
        const stored = getStoredVariableValues(url);
        if (stored && Object.keys(stored).length > 0) {
          varValues = stored;
        }
      }

      const data = await api.getDashboard(url, varValues);
      setDashboard(data);
      setCurrentDashboardUrl(url);

      // Update variable values state
      if (varValues) {
        setVariableValues(varValues);
        // Save to localStorage
        saveVariableValues(url, varValues);
      } else if (data.variables && !skipVarInit) {
        // Initialize variable values with first option if not set
        const newVarValues = {};
        let needsReload = false;
        Object.keys(data.variables).forEach(varName => {
          if (data.variables[varName].possible_values?.length > 0) {
            newVarValues[varName] = data.variables[varName].possible_values[0];
            needsReload = true;
          }
        });
        if (needsReload && Object.keys(newVarValues).length > 0) {
          setVariableValues(newVarValues);
          // Save to localStorage
          saveVariableValues(url, newVarValues);
          // Reload with initialized values
          await loadDashboard(url, newVarValues, true);
          return;
        }
        setVariableValues(newVarValues);
        // Save to localStorage
        saveVariableValues(url, newVarValues);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const loadDashboards = async () => {
    try {
      const data = await api.listDashboards();
      setDashboards(data);
    } catch (err) {
      console.error('Failed to load dashboards:', err);
    }
  };

  useEffect(() => {
    // Load dashboard with stored variable values if available
    const stored = getStoredVariableValues('default');
    loadDashboard('default', stored || undefined);
    loadDashboards();
    checkOpenAIStatus();
  }, []);

  const checkOpenAIStatus = async () => {
    try {
      const response = await fetch('/api/health');
      if (response.ok) {
        const data = await response.json();
        setOpenaiEnabled(data.openai_enabled || false);
        setReplayCount(typeof data.total_replays === 'number' ? data.total_replays : 0);
      }
    } catch (err) {
      console.error('Failed to check OpenAI status:', err);
    }
  };

  const handleCreateWidget = async (e) => {
    e.preventDefault();
    if (!newWidgetPrompt.trim() || creatingWidget) return;

    try {
      setCreatingWidget(true);
      setError(null);
      await api.createWidget(currentDashboardUrl, newWidgetPrompt);
      setNewWidgetPrompt('');
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    } finally {
      setCreatingWidget(false);
    }
  };

  const handleCreateWidgetWithoutPrompt = async () => {
    if (creatingWidget) return;

    try {
      setCreatingWidget(true);
      setError(null);
      const widget = await api.createWidget(currentDashboardUrl, '');
      setCreatingWidget(false);
      // Config should already be parsed as an object from the backend
      const config = widget.config || { type: 'table' };
      // Open the edit widget fullscreen for the newly created widget
      setEditingWidget({
        id: widget.id,
        name: widget.name,
        description: widget.description ? { valid: true, string: widget.description } : null,
        query: widget.query || '',
        config: config,
        results: [],
      });
    } catch (err) {
      setError(err.message);
      setCreatingWidget(false);
    }
  };

  const handleUpdateDashboard = async (data) => {
    try {
      await api.updateDashboard(currentDashboardUrl, data);
      setShowEditDashboard(false);
      await loadDashboard(currentDashboardUrl);
      await loadDashboards();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDeleteWidget = async (widgetId) => {
    if (!confirm('Are you sure you want to delete this widget?')) return;

    try {
      await api.deleteWidget(currentDashboardUrl, widgetId);
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUpdateWidget = async (widgetId, data) => {
    if (data.prompt) {
      data = { prompt: data.prompt }
    }
    try {
      await api.updateWidget(currentDashboardUrl, widgetId, data);
      setEditingWidget(null);
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleIngestSubmit = async (e) => {
    e.preventDefault();
    setIngestMessage('');
    try {
      await api.startIngest({
        watch: ingestForm.watch,
        stop_after_n_reps: ingestForm.stopAfterN || 0,
        clean: ingestForm.clean,
      });
      setIngestMessage('Ingestion started in the background.');
      setShowIngestPanel(false);
    } catch (err) {
      setIngestMessage(err.message || 'Failed to start ingestion.');
    }
  };

  const handleSwitchDashboard = (url) => {
    setVariableValues({});
    loadDashboard(url);
  };

  const handleVariableChange = async (varName, value) => {
    const newVarValues = { ...variableValues, [varName]: value };
    setVariableValues(newVarValues);
    // Save to localStorage
    saveVariableValues(currentDashboardUrl, newVarValues);
    await loadDashboard(currentDashboardUrl, newVarValues);
  };

  const sortedWidgets = dashboard?.widgets
    ? [...dashboard.widgets].sort((a, b) => {
      const orderA = a.widget_order?.valid ? a.widget_order.int64 : 0;
      const orderB = b.widget_order?.valid ? b.widget_order.int64 : 0;
      return orderA - orderB;
    })
    : [];

  if (loading && !dashboard) {
    return (
      <div className="app">
        <div className="loading">Loading dashboard...</div>
      </div>
    );
  }

  return (
    <div className="app">
      <div className="stars-background"></div>

      <div className="dashboard-container">
        <div className="dashboard-header">
          <div className="dashboard-title">
            <div className="dashboard-title-left">
              <h1>{dashboard?.name || 'Dashboard'}</h1>
              <button
                onClick={() => setShowEditDashboard(true)}
                className="btn-edit-dashboard"
                title="Edit dashboard"
              >
                ✎
              </button>
            </div>
            <div className="dashboard-actions">
              <select
                value={currentDashboardUrl}
                onChange={(e) => handleSwitchDashboard(e.target.value)}
                className="dashboard-select"
              >
                {dashboards.map((d) => (
                  <option key={d.url} value={d.url}>
                    {d.name}
                  </option>
                ))}
              </select>
              <button
                onClick={() => setShowDashboardManager(true)}
                className="btn-manage"
              >
                Manage Dashboards
              </button>
              <button
                onClick={() => setShowIngestPanel((prev) => !prev)}
                className="btn-manage"
              >
                {showIngestPanel ? 'Close Ingest' : 'Ingest'}
              </button>
            </div>
          </div>

          <div className="widget-creation-section">
            {openaiEnabled ? (
              <form onSubmit={handleCreateWidget} className="widget-creation-form">
                <div className="widget-creation-input-group">
                  <input
                    type="text"
                    value={newWidgetPrompt}
                    onChange={(e) => setNewWidgetPrompt(e.target.value)}
                    placeholder="Ask to add a new graph or chart..."
                    className="widget-creation-input"
                    disabled={creatingWidget}
                  />
                  <button
                    type="submit"
                    disabled={creatingWidget || !newWidgetPrompt.trim()}
                    className="btn-create-ai"
                  >
                    <span className="btn-icon">✨</span>
                    Create with AI
                  </button>
                  <div className="widget-creation-divider">or</div>
                  <button
                    type="button"
                    onClick={handleCreateWidgetWithoutPrompt}
                    disabled={creatingWidget}
                    className="btn-create-manual"
                  >
                    Create Manually
                  </button>
                </div>
              </form>
            ) : (
              <div className="widget-creation-form">
                <div className="widget-creation-input-group">
                  <button
                    type="button"
                    onClick={handleCreateWidgetWithoutPrompt}
                    disabled={creatingWidget}
                    className="btn-create-manual-primary"
                  >
                    Create Widget
                  </button>
                  <div className="widget-creation-info">
                    <span className="info-icon">ℹ️</span>
                    <span className="info-text">AI-powered creation requires --openai-api-key flag</span>
                  </div>
                </div>
              </div>
            )}
          </div>

          {showIngestPanel && (
            <div className="ingest-panel">
              <div className="ingest-header">
                <div className="ingest-title">Ingest Replays</div>
                <div className="ingest-subtitle">Ingestion happens in the background.</div>
              </div>
              <form onSubmit={handleIngestSubmit} className="ingest-form">
                <div className="ingest-grid">
                  <label className="ingest-field">
                    <span>Ingest last N replays</span>
                    <input
                      type="number"
                      min="1"
                      value={ingestForm.stopAfterN}
                      onChange={(e) => setIngestForm({ ...ingestForm, stopAfterN: parseInt(e.target.value || '0', 10) })}
                    />
                  </label>
                  <label className="ingest-field ingest-checkbox">
                    <span>Erase existing data</span>
                    <input
                      type="checkbox"
                      checked={ingestForm.clean}
                      onChange={(e) => setIngestForm({ ...ingestForm, clean: e.target.checked })}
                    />
                  </label>
                </div>
                <div className="ingest-actions">
                  <button type="submit" className="btn-create-ai">
                    Start Ingestion
                  </button>
                  <button
                    type="button"
                    className="btn-create-manual"
                    onClick={() => setShowIngestPanel(false)}
                  >
                    Cancel
                  </button>
                </div>
              </form>
            </div>
          )}

          {dashboard?.variables && Object.keys(dashboard.variables).length > 0 && (
            <div className="variables-container" style={{ display: 'flex', gap: '1rem', flexWrap: 'wrap', marginTop: '1rem' }}>
              {Object.entries(dashboard.variables).map(([varName, variable]) => (
                <div key={varName} className="variable-select" style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                  <label htmlFor={`var-${varName}`} style={{ fontSize: '0.875rem', fontWeight: '500' }}>
                    {variable.display_name}
                  </label>
                  <select
                    id={`var-${varName}`}
                    value={variableValues[varName] || ''}
                    onChange={(e) => handleVariableChange(varName, e.target.value)}
                    style={{ padding: '0.5rem', borderRadius: '4px', border: '1px solid #ccc', minWidth: '200px' }}
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
          )}
        </div>

        {error && <div className="error-message">{error}</div>}

        <div className="widgets-grid">
          {sortedWidgets.map((widget) => (
            <Widget
              key={widget.id}
              widget={widget}
              onDelete={handleDeleteWidget}
              onUpdate={handleUpdateWidget}
            />
          ))}
        </div>
      </div>

      {creatingWidget && (
        <WidgetCreationSpinner />
      )}

      {showDashboardManager && (
        <DashboardManager
          dashboards={dashboards}
          currentUrl={currentDashboardUrl}
          onClose={() => setShowDashboardManager(false)}
          onRefresh={loadDashboards}
          onSwitch={handleSwitchDashboard}
        />
      )}

      {showEditDashboard && dashboard && (
        <EditDashboardModal
          dashboard={dashboard}
          onClose={() => setShowEditDashboard(false)}
          onSave={handleUpdateDashboard}
        />
      )}

      {editingWidget && (
        <EditWidgetFullscreen
          widget={editingWidget}
          onClose={() => {
            setEditingWidget(null);
            loadDashboard(currentDashboardUrl);
          }}
          onSave={(data) => handleUpdateWidget(editingWidget.id, data)}
        />
      )}

      <div className="app-footer">
        <div className="footer-left">
          {replayCount !== null
            ? `${replayCount.toLocaleString()} replays in database. You can trigger an ingestion using the button above.`
            : 'Loading replay count...'}
        </div>
        {ingestMessage && <div className="footer-right">{ingestMessage}</div>}
      </div>
    </div>
  );
}

export default App;
