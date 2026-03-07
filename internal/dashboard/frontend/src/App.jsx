import React, { useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import { OverlaySpinner, LoadingScreen } from './components/ui/Spinner';
import EmptyState from './components/ui/EmptyState';
import Icon from './components/ui/Icon';
import Button from './components/ui/Button';
import { ToastProvider, useToast } from './components/Toast';

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
  const [isRefreshing, setIsRefreshing] = useState(false);
  const actionMenuRef = useRef(null);
  const widgetInputRef = useRef(null);

  const AUTO_REFRESH_SECONDS = 30;

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

  const loadDashboard = useCallback(async (url, varValues = null, skipVarInit = false, options = {}) => {
    const background = options.background === true;
    try {
      if (!background) setLoading(true);
      else setIsRefreshing(true);
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
      if (!background) setError(err.message);
    } finally {
      if (!background) setLoading(false);
      else setIsRefreshing(false);
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

  const reloadDashboard = useCallback(() => loadDashboard(currentDashboardUrl, variableValues), [currentDashboardUrl, variableValues, loadDashboard]);

  useEffect(() => {
    const stored = getStoredVariableValues('default');
    loadDashboard('default', stored || undefined);
    loadDashboards();
    checkOpenAIStatus();
  }, []);

  const handleRefresh = useCallback(() => {
    if (!currentDashboardUrl || isRefreshing) return;
    loadDashboard(currentDashboardUrl, variableValues, true, { background: true });
    checkOpenAIStatus();
  }, [currentDashboardUrl, variableValues, isRefreshing, loadDashboard]);

  useEffect(() => {
    if (!dashboard || !currentDashboardUrl) return;
    const intervalId = setInterval(() => {
      loadDashboard(currentDashboardUrl, variableValues, true, { background: true });
      checkOpenAIStatus();
    }, AUTO_REFRESH_SECONDS * 1000);
    return () => clearInterval(intervalId);
  }, [currentDashboardUrl, dashboard, variableValues, loadDashboard]);

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
      await loadDashboard(currentDashboardUrl, variableValues);
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
      await reloadDashboard();
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
      await reloadDashboard();
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
        const refreshed = await api.getDashboard(currentDashboardUrl, variableValues);
        const newest = refreshed.widgets?.reduce((a, b) => (a.id > b.id ? a : b), { id: 0 });
        if (newest?.id) {
          await api.updateWidget(currentDashboardUrl, newest.id, data);
        }
      } else {
        if (data.prompt) data = { prompt: data.prompt };
        await api.updateWidget(currentDashboardUrl, editingWidget.id, data);
      }
      setEditingWidget(null);
      await reloadDashboard();
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

  const sortedWidgets = useMemo(() => {
    if (!dashboard?.widgets) return [];
    return [...dashboard.widgets].sort((a, b) => (a.widget_order ?? 0) - (b.widget_order ?? 0));
  }, [dashboard?.widgets]);

  const dragWrapperClass = (widgetId) =>
    `widget-drag-wrapper${dragState.dragging === widgetId ? ' dragging' : ''}${dragState.over === widgetId && dragState.dragging !== widgetId ? ' drag-over' : ''}`;

  const handleDragStart = (e, widgetId) => {
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
      await reloadDashboard();
    } catch {
      addToast('Failed to reorder widgets', 'error');
    }
  };

  const handleDragEnd = () => {
    setDragState({ dragging: null, over: null });
  };

  const variableCount = dashboard?.variables ? Object.keys(dashboard.variables).length : 0;
  const dashboardDesc = dashboard?.description ?? null;

  if (loading && !dashboard) {
    return (
      <div className="app">
        <LoadingScreen message="Loading dashboard..." />
      </div>
    );
  }

  return (
    <div className="app">
      <header className="app-header">
        <div className="header-left">
          <div className="header-brand">
            <Icon name="chart" size={24} className="header-logo" />
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
          <button
            className="header-refresh-btn"
            onClick={handleRefresh}
            disabled={isRefreshing}
            title="Refresh dashboard data"
            aria-label="Refresh dashboard"
          >
            <Icon name="refresh" size={18} style={{ opacity: isRefreshing ? 0.6 : 1 }} />
            {isRefreshing && <span className="header-refreshing-label">Refreshing...</span>}
          </button>
          <div className="action-menu-wrapper" ref={actionMenuRef}>
            <button
              className="header-menu-btn"
              onClick={() => setShowActionMenu(!showActionMenu)}
              title="Menu"
              aria-expanded={showActionMenu}
              aria-haspopup="true"
            >
              <Icon name="menu" size={20} />
            </button>
            {showActionMenu && (
              <div className="action-menu" role="menu">
                <button role="menuitem" onClick={() => { setShowIngestPanel(true); setShowActionMenu(false); }}>
                  <Icon name="download" size={16} />
                  Import Replays
                </button>
                <button role="menuitem" onClick={() => { setShowDashboardManager(true); setShowActionMenu(false); }}>
                  <Icon name="grid" size={16} />
                  Manage Dashboards
                </button>
                <button role="menuitem" onClick={() => { setShowEditDashboard(true); setShowActionMenu(false); }}>
                  <Icon name="edit" size={16} />
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
                  <Icon name="close" size={18} />
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
                  <Button variant="primary" type="submit">Start Import</Button>
                  <Button variant="secondary" type="button" onClick={() => setShowIngestPanel(false)}>Cancel</Button>
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
                <Button variant="primary" type="submit" disabled={creatingWidget || !newWidgetPrompt.trim()}>
                  Create with AI
                </Button>
                <div className="widget-creation-divider">or</div>
                <Button variant="secondary" type="button" onClick={handleCreateWidgetManually} disabled={creatingWidget}>
                  Create Manually
                </Button>
              </div>
            </form>
          ) : (
            <div className="widget-creation-form">
              <div className="widget-creation-input-group">
                <Button variant="primary" type="button" onClick={handleCreateWidgetManually} disabled={creatingWidget}>
                  + New Widget
                </Button>
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
              <Icon name="filter" size={14} />
              Filters ({variableCount})
              <Icon name="chevronDown" size={12} style={{ transform: showVariables ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s' }} />
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
          <EmptyState
            icon="dashboard"
            title="Your dashboard is empty"
            description="Create your first widget to start visualizing your StarCraft replay data."
            actions={
              <>
                {openaiEnabled && (
                  <Button variant="primary" onClick={() => widgetInputRef.current?.focus()}>
                    Describe a Chart
                  </Button>
                )}
                <Button variant="secondary" onClick={handleCreateWidgetManually}>
                  Create Widget Manually
                </Button>
              </>
            }
            hint={replayCount === 0 ? 'No replays found. Import replays first using the menu above.' : undefined}
          />
        ) : (
          <div className="widgets-grid">
            {sortedWidgets.map((widget) => (
              <div
                key={widget.id}
                className={dragWrapperClass(widget.id)}
                draggable
                title="Drag to reorder"
                onDragStart={(e) => handleDragStart(e, widget.id)}
                onDragOver={(e) => handleDragOver(e, widget.id)}
                onDrop={(e) => handleDrop(e, widget.id)}
                onDragEnd={handleDragEnd}
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

      {creatingWidget && <OverlaySpinner />}

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
          onClose={() => { setEditingWidget(null); reloadDashboard(); }}
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
