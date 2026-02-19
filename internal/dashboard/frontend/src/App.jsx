import React, { useState, useEffect, useCallback, useRef } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import WidgetCreationSpinner from './components/WidgetCreationSpinner';
import { ToastProvider, useToast } from './components/Toast';
import './styles.css';

const getStoredVariableValues = (dashboardUrl) => {
  try {
    const stored = localStorage.getItem(`dashboard_vars_${dashboardUrl}`);
    return stored ? JSON.parse(stored) : null;
  } catch { return null; }
};

const saveVariableValues = (dashboardUrl, values) => {
  try {
    localStorage.setItem(`dashboard_vars_${dashboardUrl}`, JSON.stringify(values));
  } catch { /* ignore */ }
};

function AppContent() {
  const { addToast } = useToast();
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
  const [showActionMenu, setShowActionMenu] = useState(false);
  const [showVariables, setShowVariables] = useState(true);
  const [ingestForm, setIngestForm] = useState({ watch: false, stopAfterN: 50, clean: false });
  const [dragState, setDragState] = useState({ dragging: null, over: null });
  const actionMenuRef = useRef(null);
  const widgetInputRef = useRef(null);
  const dragHandleActive = useRef(false);

  useEffect(() => {
    const handler = (e) => {
      if (actionMenuRef.current && !actionMenuRef.current.contains(e.target)) {
        setShowActionMenu(false);
      }
    };
    const keyHandler = (e) => {
      if (e.key === 'Escape') setShowActionMenu(false);
    };
    document.addEventListener('mousedown', handler);
    document.addEventListener('keydown', keyHandler);
    return () => {
      document.removeEventListener('mousedown', handler);
      document.removeEventListener('keydown', keyHandler);
    };
  }, []);

  const loadDashboard = useCallback(async (url, varValues = null, skipVarInit = false) => {
    try {
      setLoading(true);
      setError(null);
      if (!varValues) {
        const stored = getStoredVariableValues(url);
        if (stored && Object.keys(stored).length > 0) varValues = stored;
      }
      const data = await api.getDashboard(url, varValues);
      setDashboard(data);
      setCurrentDashboardUrl(url);
      if (varValues) {
        setVariableValues(varValues);
        saveVariableValues(url, varValues);
      } else if (data.variables && !skipVarInit) {
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
          saveVariableValues(url, newVarValues);
          await loadDashboard(url, newVarValues, true);
          return;
        }
        setVariableValues(newVarValues);
        saveVariableValues(url, newVarValues);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadDashboards = useCallback(async () => {
    try {
      const data = await api.listDashboards();
      setDashboards(data);
    } catch (err) {
      console.error('Failed to load dashboards:', err);
    }
  }, []);

  useEffect(() => {
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
    } catch { /* ignore */ }
  };

  const handleCreateWidget = async (e) => {
    e.preventDefault();
    if (!newWidgetPrompt.trim() || creatingWidget) return;
    try {
      setCreatingWidget(true);
      setError(null);
      await api.createWidget(currentDashboardUrl, newWidgetPrompt);
      setNewWidgetPrompt('');
      addToast('Widget created successfully', 'success');
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      addToast(err.message, 'error');
    } finally {
      setCreatingWidget(false);
    }
  };

  const handleCreateWidgetManually = () => {
    if (creatingWidget) return;
    setEditingWidget({
      id: null,
      name: 'New Widget',
      description: null,
      query: '',
      config: { type: 'table' },
      results: [],
    });
  };

  const handleUpdateDashboard = async (data) => {
    try {
      await api.updateDashboard(currentDashboardUrl, data);
      setShowEditDashboard(false);
      addToast('Dashboard updated', 'success');
      await loadDashboard(currentDashboardUrl);
      await loadDashboards();
    } catch (err) {
      addToast(err.message, 'error');
    }
  };

  const handleDeleteWidget = async (widgetId) => {
    if (!confirm('Delete this widget?')) return;
    try {
      await api.deleteWidget(currentDashboardUrl, widgetId);
      addToast('Widget deleted', 'success');
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      addToast(err.message, 'error');
    }
  };

  const handleEditWidget = (widget) => {
    setEditingWidget(widget);
  };

  const handleSaveWidget = async (data) => {
    try {
      if (editingWidget.id === null) {
        await api.createWidget(currentDashboardUrl, '');
        const refreshed = await api.getDashboard(currentDashboardUrl);
        const newest = refreshed.widgets?.reduce((a, b) => (a.id > b.id ? a : b), { id: 0 });
        if (newest?.id) {
          await api.updateWidget(currentDashboardUrl, newest.id, data);
        }
      } else {
        if (data.prompt) data = { prompt: data.prompt };
        await api.updateWidget(currentDashboardUrl, editingWidget.id, data);
      }
      setEditingWidget(null);
      await loadDashboard(currentDashboardUrl);
    } catch (err) {
      addToast(err.message, 'error');
    }
  };

  const handleIngestSubmit = async (e) => {
    e.preventDefault();
    try {
      await api.startIngest({
        watch: ingestForm.watch,
        stop_after_n_reps: ingestForm.stopAfterN || 0,
        clean: ingestForm.clean,
      });
      addToast('Ingestion started in the background', 'success');
      setShowIngestPanel(false);
    } catch (err) {
      addToast(err.message || 'Failed to start ingestion', 'error');
    }
  };

  const handleSwitchDashboard = (url) => {
    setVariableValues({});
    loadDashboard(url);
  };

  const handleVariableChange = async (varName, value) => {
    const newVarValues = { ...variableValues, [varName]: value };
    setVariableValues(newVarValues);
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

  const handleDragStart = (e, widgetId) => {
    if (!dragHandleActive.current) {
      e.preventDefault();
      return;
    }
    setDragState({ dragging: widgetId, over: null });
    e.dataTransfer.effectAllowed = 'move';
    e.dataTransfer.setData('text/plain', String(widgetId));
  };

  const handleDragOver = (e, widgetId) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
    if (dragState.over !== widgetId) {
      setDragState(prev => ({ ...prev, over: widgetId }));
    }
  };

  const handleDrop = async (e, targetId) => {
    e.preventDefault();
    const sourceId = dragState.dragging;
    if (!sourceId || sourceId === targetId) {
      setDragState({ dragging: null, over: null });
      return;
    }
    const widgets = [...sortedWidgets];
    const sourceIdx = widgets.findIndex(w => w.id === sourceId);
    const targetIdx = widgets.findIndex(w => w.id === targetId);
    if (sourceIdx === -1 || targetIdx === -1) {
      setDragState({ dragging: null, over: null });
      return;
    }
    const [moved] = widgets.splice(sourceIdx, 1);
    widgets.splice(targetIdx, 0, moved);
    setDragState({ dragging: null, over: null });
    try {
      await Promise.all(widgets.map((w, i) =>
        api.updateWidgetOrder(currentDashboardUrl, w.id, i + 1)
      ));
      await loadDashboard(currentDashboardUrl);
    } catch {
      addToast('Failed to reorder widgets', 'error');
    }
  };

  const handleDragEnd = () => {
    setDragState({ dragging: null, over: null });
    dragHandleActive.current = false;
  };

  const handleHandleMouseDown = () => {
    dragHandleActive.current = true;
  };

  const variableCount = dashboard?.variables ? Object.keys(dashboard.variables).length : 0;
  const dashboardDesc = dashboard?.description?.valid ? dashboard.description.string : null;

  if (loading && !dashboard) {
    return (
      <div className="app">
        <div className="loading">
          <div className="loading-spinner"></div>
          <span>Loading dashboard...</span>
        </div>
      </div>
    );
  }

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <div className="header-brand">
            <svg className="header-logo" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M3 3v18h18"/>
              <path d="M7 16l4-8 4 4 4-12"/>
            </svg>
            <select
              value={currentDashboardUrl}
              onChange={(e) => handleSwitchDashboard(e.target.value)}
              className="header-dashboard-select"
            >
              {dashboards.map((d) => (
                <option key={d.url} value={d.url}>{d.name}</option>
              ))}
            </select>
          </div>
          {dashboardDesc && (
            <span className="header-description" title={dashboardDesc}>{dashboardDesc}</span>
          )}
        </div>

        <div className="header-right">
          <div className="header-replay-count">
            {replayCount !== null ? `${replayCount.toLocaleString()} replays` : ''}
          </div>
          <div className="action-menu-wrapper" ref={actionMenuRef}>
            <button
              className="header-menu-btn"
              onClick={() => setShowActionMenu(!showActionMenu)}
              title="Menu"
              aria-expanded={showActionMenu}
              aria-haspopup="true"
            >
              <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor">
                <circle cx="10" cy="4" r="2"/>
                <circle cx="10" cy="10" r="2"/>
                <circle cx="10" cy="16" r="2"/>
              </svg>
            </button>
            {showActionMenu && (
              <div className="action-menu" role="menu">
                <button role="menuitem" onClick={() => { setShowIngestPanel(true); setShowActionMenu(false); }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
                  Import Replays
                </button>
                <button role="menuitem" onClick={() => { setShowDashboardManager(true); setShowActionMenu(false); }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/></svg>
                  Manage Dashboards
                </button>
                <button role="menuitem" onClick={() => { setShowEditDashboard(true); setShowActionMenu(false); }}>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
                  Edit Dashboard
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      <div className="dashboard-container">
        {showIngestPanel && (
          <div className="ingest-panel slide-down">
            <div className="ingest-panel-inner">
              <div className="ingest-header">
                <div>
                  <div className="ingest-title">Import Replays</div>
                  <div className="ingest-subtitle">
                    {replayCount !== null ? `${replayCount.toLocaleString()} replays in database. ` : ''}
                    Runs in the background.
                  </div>
                </div>
                <button className="btn-icon-close" onClick={() => setShowIngestPanel(false)}>
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
                </button>
              </div>
              <form onSubmit={handleIngestSubmit} className="ingest-form">
                <div className="ingest-grid">
                  <label className="ingest-field">
                    <span>How many replays to import</span>
                    <input
                      type="number" min="1" value={ingestForm.stopAfterN}
                      onChange={(e) => setIngestForm({ ...ingestForm, stopAfterN: parseInt(e.target.value || '0', 10) })}
                    />
                  </label>
                  <label className="ingest-field ingest-checkbox">
                    <input type="checkbox" checked={ingestForm.clean}
                      onChange={(e) => setIngestForm({ ...ingestForm, clean: e.target.checked })}
                    />
                    <span>Clear existing data first</span>
                  </label>
                </div>
                <div className="ingest-actions">
                  <button type="submit" className="btn-primary">Start Import</button>
                  <button type="button" className="btn-secondary" onClick={() => setShowIngestPanel(false)}>Cancel</button>
                </div>
              </form>
            </div>
          </div>
        )}

        <div className="widget-creation-section">
          {openaiEnabled ? (
            <form onSubmit={handleCreateWidget} className="widget-creation-form">
              <div className="widget-creation-input-group">
                <input
                  ref={widgetInputRef}
                  type="text" value={newWidgetPrompt}
                  onChange={(e) => setNewWidgetPrompt(e.target.value)}
                  placeholder="Describe a chart you want (e.g. 'win rate by race')..."
                  className="widget-creation-input" disabled={creatingWidget}
                />
                <button type="submit" disabled={creatingWidget || !newWidgetPrompt.trim()} className="btn-primary">
                  Create with AI
                </button>
                <div className="widget-creation-divider">or</div>
                <button type="button" onClick={handleCreateWidgetManually}
                  disabled={creatingWidget} className="btn-secondary">
                  Create Manually
                </button>
              </div>
            </form>
          ) : (
            <div className="widget-creation-form">
              <div className="widget-creation-input-group">
                <button type="button" onClick={handleCreateWidgetManually}
                  disabled={creatingWidget} className="btn-primary">
                  + New Widget
                </button>
                <div className="widget-creation-info">
                  <span className="info-text">AI-powered creation requires --openai-api-key flag</span>
                </div>
              </div>
            </div>
          )}
        </div>

        {variableCount > 0 && (
          <div className="variables-bar">
            <button className="variables-toggle" onClick={() => setShowVariables(!showVariables)}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <polygon points="22 3 2 3 10 12.46 10 19 14 21 14 12.46 22 3"/>
              </svg>
              Filters ({variableCount})
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"
                style={{ transform: showVariables ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }}>
                <polyline points="6 9 12 15 18 9"/>
              </svg>
            </button>
            {showVariables && (
              <div className="variables-content">
                {Object.entries(dashboard.variables).map(([varName, variable]) => (
                  <div key={varName} className="variable-item">
                    <label htmlFor={`var-${varName}`}>{variable.display_name}</label>
                    <select
                      id={`var-${varName}`}
                      value={variableValues[varName] || ''}
                      onChange={(e) => handleVariableChange(varName, e.target.value)}
                    >
                      {variable.possible_values?.map((value, idx) => (
                        <option key={idx} value={value}>{value}</option>
                      ))}
                    </select>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {error && <div className="error-message">{error}</div>}

        {sortedWidgets.length === 0 && !loading ? (
          <div className="empty-state">
            <div className="empty-state-icon">
              <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                <line x1="3" y1="9" x2="21" y2="9"/>
                <line x1="9" y1="21" x2="9" y2="9"/>
              </svg>
            </div>
            <h2>Your dashboard is empty</h2>
            <p>Create your first widget to start visualizing your StarCraft replay data.</p>
            <div className="empty-state-actions">
              {openaiEnabled && (
                <button className="btn-primary" onClick={() => widgetInputRef.current?.focus()}>
                  Describe a Chart
                </button>
              )}
              <button className="btn-secondary" onClick={handleCreateWidgetManually}>
                Create Widget Manually
              </button>
            </div>
            {replayCount === 0 && (
              <div className="empty-state-hint">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/>
                </svg>
                No replays found. Import replays first using the menu above.
              </div>
            )}
          </div>
        ) : (
          <div className="widgets-grid">
            {sortedWidgets.map((widget) => (
              <div
                key={widget.id}
                className={`widget-drag-wrapper${dragState.dragging === widget.id ? ' dragging' : ''}${dragState.over === widget.id && dragState.dragging !== widget.id ? ' drag-over' : ''}`}
                draggable
                onDragStart={(e) => handleDragStart(e, widget.id)}
                onDragOver={(e) => handleDragOver(e, widget.id)}
                onDrop={(e) => handleDrop(e, widget.id)}
                onDragEnd={handleDragEnd}
                onMouseDown={(e) => {
                  if (e.target.closest('.widget-drag-handle')) {
                    handleHandleMouseDown();
                  }
                }}
              >
                <Widget
                  widget={widget}
                  onDelete={handleDeleteWidget}
                  onEdit={handleEditWidget}
                  showDragHandle
                />
              </div>
            ))}
          </div>
        )}
      </div>

      {creatingWidget && <WidgetCreationSpinner />}

      {showDashboardManager && (
        <DashboardManager
          dashboards={dashboards} currentUrl={currentDashboardUrl}
          onClose={() => setShowDashboardManager(false)}
          onRefresh={loadDashboards} onSwitch={handleSwitchDashboard}
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
          widget={editingWidget} dashboardUrl={currentDashboardUrl}
          onClose={() => { setEditingWidget(null); loadDashboard(currentDashboardUrl); }}
          onSave={(data) => handleSaveWidget(data)}
        />
      )}
    </div>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <AppContent />
    </ToastProvider>
  );
}
