import React, { useState, useEffect } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import WidgetCreationSpinner from './components/WidgetCreationSpinner';
import './styles.css';

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

  const loadDashboard = async (url) => {
    try {
      setLoading(true);
      setError(null);
      const data = await api.getDashboard(url);
      setDashboard(data);
      setCurrentDashboardUrl(url);
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
    loadDashboard('default');
    loadDashboards();
  }, []);

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
    try {
      await api.updateWidget(currentDashboardUrl, widgetId, data);
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSwitchDashboard = (url) => {
    loadDashboard(url);
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
            <input
              type="text"
              value={newWidgetPrompt}
              onChange={(e) => setNewWidgetPrompt(e.target.value)}
              placeholder="Ask to add a new graph or chart..."
              className="prompt-input"
              disabled={creatingWidget}
            />
            <button
              type="submit"
              disabled={creatingWidget || !newWidgetPrompt.trim()}
              className="btn-create"
            >
              Create Widget
            </button>
          </form>
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
    </div>
  );
}

export default App;

