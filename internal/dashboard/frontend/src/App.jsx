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
      const config = widget.config || { type: 'table', colors: [] };
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
            </div>
          </div>

          <form onSubmit={handleCreateWidget} className="prompt-form">
            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem', width: '100%' }}>
              <input
                type="text"
                value={newWidgetPrompt}
                onChange={(e) => setNewWidgetPrompt(e.target.value)}
                placeholder={openaiEnabled ? "Ask to add a new graph or chart..." : "OpenAI API key required to use prompts"}
                className="prompt-input"
                disabled={creatingWidget || !openaiEnabled}
                style={{
                  opacity: openaiEnabled ? 1 : 0.5,
                  cursor: openaiEnabled ? 'text' : 'not-allowed',
                }}
              />
              {!openaiEnabled && (
                <label style={{ fontSize: '0.875rem', color: '#999', marginTop: '-0.25rem' }}>
                  To use AI-powered widget creation, start the dashboard with the --openai-api-key flag
                </label>
              )}
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button
                  type="submit"
                  disabled={creatingWidget || !newWidgetPrompt.trim() || !openaiEnabled}
                  className="btn-create"
                  style={{
                    opacity: (!openaiEnabled || !newWidgetPrompt.trim()) ? 0.5 : 1,
                    cursor: (!openaiEnabled || !newWidgetPrompt.trim()) ? 'not-allowed' : 'pointer',
                  }}
                >
                  Create Widget with AI
                </button>
                <button
                  type="button"
                  onClick={handleCreateWidgetWithoutPrompt}
                  disabled={creatingWidget}
                  className="btn-create"
                  style={{
                    backgroundColor: '#4a5568',
                  }}
                >
                  Create Widget Manually
                </button>
              </div>
            </div>
          </form>
          
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
    </div>
  );
}

export default App;

