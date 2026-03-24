import React, { useState, useEffect, useRef } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import WidgetCreationSpinner from './components/WidgetCreationSpinner';
import PieChart from './components/charts/PieChart';
import Gauge from './components/charts/Gauge';
import Table from './components/charts/Table';
import BarChart from './components/charts/BarChart';
import LineChart from './components/charts/LineChart';
import ScatterPlot from './components/charts/ScatterPlot';
import Histogram from './components/charts/Histogram';
import Heatmap from './components/charts/Heatmap';
import probeImg from './assets/units/probe.png';
import scvImg from './assets/units/scv.png';
import droneImg from './assets/units/drone.png';
import carrierImg from './assets/units/carrier.png';
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

const AUTO_INGEST_SETTINGS_KEY = 'dashboard_auto_ingest_settings';

const getStoredAutoIngestSettings = () => {
  try {
    const stored = localStorage.getItem(AUTO_INGEST_SETTINGS_KEY);
    if (!stored) {
      return { enabled: true, intervalSeconds: 60 };
    }
    const parsed = JSON.parse(stored);
    const interval = Number.isFinite(parsed?.intervalSeconds) && parsed.intervalSeconds >= 60
      ? Math.floor(parsed.intervalSeconds)
      : 60;
    return {
      enabled: parsed?.enabled !== false,
      intervalSeconds: interval,
    };
  } catch (e) {
    console.error('Failed to load auto-ingest settings from localStorage:', e);
    return { enabled: true, intervalSeconds: 60 };
  }
};

const saveAutoIngestSettings = (settings) => {
  try {
    localStorage.setItem(AUTO_INGEST_SETTINGS_KEY, JSON.stringify(settings));
  } catch (e) {
    console.error('Failed to save auto-ingest settings to localStorage:', e);
  }
};

const formatDuration = (seconds) => {
  const total = Number(seconds) || 0;
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${String(secs).padStart(2, '0')}`;
};

const formatRelativeReplayDate = (value) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;

  const now = new Date();
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const startOfDate = new Date(date.getFullYear(), date.getMonth(), date.getDate());
  const diffDays = Math.floor((startOfToday.getTime() - startOfDate.getTime()) / 86400000);

  let dayLabel = '';
  if (diffDays === 0) dayLabel = 'Today';
  else if (diffDays === 1) dayLabel = 'Yesterday';
  else if (diffDays > 1) dayLabel = `${diffDays} days ago`;
  else dayLabel = date.toLocaleDateString();

  const hours = date.getHours();
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const hour12 = hours % 12 || 12;
  const ampm = hours >= 12 ? 'pm' : 'am';
  return `${dayLabel} at ${hour12}.${minutes}${ampm}`;
};

const getRaceIcon = (race) => {
  const value = String(race || '').toLowerCase();
  if (value === 'protoss') return probeImg;
  if (value === 'terran') return scvImg;
  if (value === 'zerg') return droneImg;
  return null;
};

const TEAM_COLORS = ['#60A5FA', '#F472B6', '#34D399', '#FBBF24', '#A78BFA', '#22D3EE', '#FB7185', '#4ADE80'];

const getTeamColor = (team) => {
  const n = Number(team) || 0;
  return TEAM_COLORS[Math.abs(n) % TEAM_COLORS.length];
};

function App() {
  const storedAutoIngest = getStoredAutoIngestSettings();
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
    autoIngestEnabled: storedAutoIngest.enabled,
    autoIngestIntervalSeconds: storedAutoIngest.intervalSeconds,
  });
  const autoIngestInFlight = useRef(false);
  const [activeView, setActiveView] = useState('games');
  const [workflowGames, setWorkflowGames] = useState([]);
  const [workflowGamesLoading, setWorkflowGamesLoading] = useState(false);
  const [workflowGameDetailLoading, setWorkflowGameDetailLoading] = useState(false);
  const [workflowPlayerLoading, setWorkflowPlayerLoading] = useState(false);
  const [selectedReplayId, setSelectedReplayId] = useState(null);
  const [selectedPlayerKey, setSelectedPlayerKey] = useState('');
  const [workflowGame, setWorkflowGame] = useState(null);
  const [workflowPlayer, setWorkflowPlayer] = useState(null);
  const [workflowQuestion, setWorkflowQuestion] = useState('');
  const [workflowAnswer, setWorkflowAnswer] = useState(null);
  const [askingWorkflow, setAskingWorkflow] = useState(false);
  const [topPlayerColors, setTopPlayerColors] = useState({});

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

  const loadWorkflowGames = async () => {
    try {
      setWorkflowGamesLoading(true);
      const data = await api.listWorkflowGames({ limit: 30, offset: 0 });
      const items = data?.items || [];
      setWorkflowGames(items);
      if (!selectedReplayId && items.length > 0) {
        setSelectedReplayId(items[0].replay_id);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowGamesLoading(false);
    }
  };

  const loadTopPlayerColors = async () => {
    try {
      const data = await api.getWorkflowPlayerColors();
      setTopPlayerColors(data?.player_colors || {});
    } catch (err) {
      console.error('Failed to load top player colors:', err);
    }
  };

  const openWorkflowGame = async (replayId) => {
    try {
      setWorkflowGameDetailLoading(true);
      setError(null);
      const data = await api.getWorkflowGame(replayId);
      setWorkflowGame(data);
      setSelectedReplayId(replayId);
      setWorkflowAnswer(null);
      setWorkflowQuestion('');
      setActiveView('game');
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowGameDetailLoading(false);
    }
  };

  const openWorkflowPlayer = async (playerKey) => {
    try {
      setWorkflowPlayerLoading(true);
      setError(null);
      const data = await api.getWorkflowPlayer(playerKey);
      setWorkflowPlayer(data);
      setSelectedPlayerKey(playerKey);
      setWorkflowAnswer(null);
      setWorkflowQuestion('');
      setActiveView('player');
    } catch (err) {
      setError(err.message);
    } finally {
      setWorkflowPlayerLoading(false);
    }
  };

  useEffect(() => {
    // Load dashboard with stored variable values if available
    const stored = getStoredVariableValues('default');
    loadDashboard('default', stored || undefined);
    loadDashboards();
    loadWorkflowGames();
    loadTopPlayerColors();
    checkOpenAIStatus();
  }, []);

  useEffect(() => {
    saveAutoIngestSettings({
      enabled: ingestForm.autoIngestEnabled,
      intervalSeconds: ingestForm.autoIngestIntervalSeconds,
    });
  }, [ingestForm.autoIngestEnabled, ingestForm.autoIngestIntervalSeconds]);

  useEffect(() => {
    if (!ingestForm.autoIngestEnabled) {
      return undefined;
    }

    const intervalSeconds = Math.max(60, Number(ingestForm.autoIngestIntervalSeconds) || 60);
    let cancelled = false;

    const runAutoIngest = async () => {
      if (cancelled || autoIngestInFlight.current) return;
      autoIngestInFlight.current = true;
      try {
        await api.startIngest({
          watch: false,
          stop_after_n_reps: 1,
          clean: false,
        });
        await loadWorkflowGames();
      } catch (err) {
        console.error('Auto-ingest failed:', err);
      } finally {
        autoIngestInFlight.current = false;
      }
    };

    runAutoIngest();
    const timer = window.setInterval(runAutoIngest, intervalSeconds * 1000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [ingestForm.autoIngestEnabled, ingestForm.autoIngestIntervalSeconds]);

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
      await loadWorkflowGames();
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

  const handleWorkflowAsk = async (e) => {
    e.preventDefault();
    const question = workflowQuestion.trim();
    if (!question || askingWorkflow) return;
    try {
      setAskingWorkflow(true);
      setWorkflowAnswer(null);
      if (activeView === 'game' && workflowGame?.replay_id) {
        const response = await api.askWorkflowGame(workflowGame.replay_id, question);
        setWorkflowAnswer(response);
      } else if (activeView === 'player' && workflowPlayer?.player_key) {
        const response = await api.askWorkflowPlayer(workflowPlayer.player_key, question);
        setWorkflowAnswer(response);
      }
    } catch (err) {
      setWorkflowAnswer({
        title: 'AI Error',
        description: 'The question could not be answered.',
        config: { type: 'text' },
        text_answer: `Failed to ask AI: ${err.message}`,
        results: [],
        columns: [],
      });
    } finally {
      setAskingWorkflow(false);
    }
  };

  const playerAccentColor = (nameOrKey) => {
    const key = String(nameOrKey || '').trim().toLowerCase();
    return topPlayerColors[key] || '';
  };

  const renderPlayerLabel = (name) => {
    const color = playerAccentColor(name);
    if (!color) return <span>{name}</span>;
    return <span style={{ color, fontWeight: 600 }}>{name}</span>;
  };

  const renderPlayersMatchup = (label) => {
    const sides = String(label || '').split(' vs ');
    return sides.map((side, sideIndex) => (
      <span key={`${side}-${sideIndex}`}>
        {side.split(', ').map((name, idx) => (
          <span key={`${name}-${idx}`}>
            {renderPlayerLabel(name)}
            {idx < side.split(', ').length - 1 ? ', ' : ''}
          </span>
        ))}
        {sideIndex < sides.length - 1 ? ' vs ' : ''}
      </span>
    ));
  };

  const renderWorkflowAiResult = () => {
    if (!workflowAnswer) return null;
    const config = workflowAnswer.config || { type: 'text' };
    const data = workflowAnswer.results || [];
    const columns = workflowAnswer.columns || [];
    const chartProps = { data, config };

    if (config.type === 'text') {
      return (
        <div className="workflow-answer">
          {workflowAnswer.title ? <div className="workflow-answer-title">{workflowAnswer.title}</div> : null}
          <div>{workflowAnswer.text_answer || workflowAnswer.description || 'No text answer returned.'}</div>
        </div>
      );
    }

    let content = null;
    switch (config.type) {
      case 'gauge':
        content = <Gauge {...chartProps} />;
        break;
      case 'table':
        content = <Table {...chartProps} columns={columns} />;
        break;
      case 'pie_chart':
        content = <PieChart {...chartProps} />;
        break;
      case 'bar_chart':
        content = <BarChart {...chartProps} />;
        break;
      case 'line_chart':
        content = <LineChart {...chartProps} />;
        break;
      case 'scatter_plot':
        content = <ScatterPlot {...chartProps} />;
        break;
      case 'histogram':
        content = <Histogram {...chartProps} />;
        break;
      case 'heatmap':
        content = <Heatmap {...chartProps} />;
        break;
      default:
        content = <div className="chart-empty">Unknown AI chart type: {String(config.type || '')}</div>;
        break;
    }

    return (
      <div className="workflow-answer-chart">
        {workflowAnswer.title ? <div className="workflow-answer-title">{workflowAnswer.title}</div> : null}
        {workflowAnswer.description ? <div className="workflow-answer-description">{workflowAnswer.description}</div> : null}
        <div className="workflow-answer-visual">{content}</div>
      </div>
    );
  };

  const sortedWidgets = dashboard?.widgets
    ? [...dashboard.widgets].sort((a, b) => {
      const orderA = a.widget_order?.valid ? a.widget_order.int64 : 0;
      const orderB = b.widget_order?.valid ? b.widget_order.int64 : 0;
      return orderA - orderB;
    })
    : [];

  if (loading && !dashboard && activeView === 'dashboards') {
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
        <div className="workflow-nav">
          <button className={`btn-manage ${activeView === 'games' ? 'workflow-nav-active' : ''}`} onClick={() => setActiveView('games')}>Games</button>
          <button className={`btn-manage ${activeView === 'dashboards' ? 'workflow-nav-active' : ''}`} onClick={() => setActiveView('dashboards')}>Custom Dashboards</button>
          <button onClick={() => setShowIngestPanel((prev) => !prev)} className="btn-manage">{showIngestPanel ? 'Close Ingest' : 'Ingest'}</button>
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
                <label className="ingest-field ingest-checkbox">
                  <span>Auto-ingest latest replay</span>
                  <input
                    type="checkbox"
                    checked={ingestForm.autoIngestEnabled}
                    onChange={(e) => setIngestForm({ ...ingestForm, autoIngestEnabled: e.target.checked })}
                  />
                </label>
                <label className="ingest-field">
                  <span>Auto-ingest interval (seconds)</span>
                  <input
                    type="number"
                    min="60"
                    value={ingestForm.autoIngestIntervalSeconds}
                    onChange={(e) => setIngestForm({
                      ...ingestForm,
                      autoIngestIntervalSeconds: parseInt(e.target.value || '60', 10),
                    })}
                    disabled={!ingestForm.autoIngestEnabled}
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

        {error && <div className="error-message">{error}</div>}

        {activeView === 'games' && (
          <div className="workflow-panel">
            <h2>Latest Games</h2>
            {workflowGamesLoading ? (
              <div className="loading">Loading games...</div>
            ) : (
              <table className="data-table workflow-table">
                <thead>
                  <tr>
                    <th>Played</th>
                    <th>Players</th>
                    <th>Map</th>
                    <th>Duration</th>
                    <th>Winner</th>
                  </tr>
                </thead>
                <tbody>
                  {workflowGames.map((game) => (
                    <tr key={game.replay_id} className={selectedReplayId === game.replay_id ? 'workflow-selected-row' : ''} onClick={() => openWorkflowGame(game.replay_id)}>
                      <td>{formatRelativeReplayDate(game.replay_date)}</td>
                      <td>{renderPlayersMatchup(game.players_label)}</td>
                      <td>{game.map_name}</td>
                      <td>{formatDuration(game.duration_seconds)}</td>
                      <td>{game.winners_label ? renderPlayersMatchup(game.winners_label) : '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {activeView === 'game' && (
          <div className="workflow-panel">
            {workflowGameDetailLoading ? (
              <div className="loading">Loading game report...</div>
            ) : workflowGame ? (
              <>
                <div className="workflow-header-row">
                  <button className="btn-switch" onClick={() => setActiveView('games')}>Back to games</button>
                </div>
                <h2>{renderPlayersMatchup(workflowGame.players?.map((p) => p.name).join(' vs '))}</h2>
                <div className="workflow-meta">
                  <span>{formatRelativeReplayDate(workflowGame.replay_date)}</span>
                  <span>{workflowGame.map_name}</span>
                  <span>{formatDuration(workflowGame.duration_seconds)}</span>
                </div>
                <div className="workflow-cards">
                  {workflowGame.players?.map((player) => (
                    <div key={player.player_id} className="workflow-card" style={{ borderLeft: `3px solid ${getTeamColor(player.team)}` }}>
                      <div className="workflow-card-title">
                        <button
                          className="btn-switch"
                          style={playerAccentColor(player.player_key) ? { color: playerAccentColor(player.player_key), borderColor: playerAccentColor(player.player_key) } : undefined}
                          onClick={() => openWorkflowPlayer(player.player_key)}
                        >
                          {player.name}
                        </button>
                        <span className="workflow-inline">
                          {getRaceIcon(player.race) ? <img src={getRaceIcon(player.race)} alt={player.race} className="unit-icon-inline" /> : null}
                          <span>Team {player.team}</span>
                          {player.is_winner ? <strong className="workflow-winner">Winner</strong> : null}
                        </span>
                      </div>
                      <div className="workflow-metric-row"><strong>APM</strong><span>{player.apm}</span><strong>EAPM</strong><span>{player.eapm}</span></div>
                      <div className="workflow-metric-row"><strong>Commands</strong><span>{player.command_count}</span><strong>Hotkeys</strong><span>{player.hotkey_command_count}</span></div>
                      <div className="workflow-inline">
                        <img src={carrierImg} alt="Carrier" className="unit-icon-inline" />
                        <strong>Carrier commands</strong>
                        <span>{player.carrier_command_count}</span>
                      </div>
                      {player.detected_patterns?.length > 0 && (
                        <div className="workflow-patterns">
                          <strong>Detected patterns</strong>
                          {player.detected_patterns.map((pattern, idx) => (
                            <div key={`${pattern.pattern_name}-${idx}`} className="workflow-pattern-row">
                              <span>{pattern.pattern_name}</span>
                              <span>{pattern.value}</span>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}
                </div>
                {(workflowGame.replay_patterns?.length > 0 || workflowGame.team_patterns?.length > 0) && (
                  <div className="workflow-cards">
                    {workflowGame.replay_patterns?.length > 0 && (
                      <div className="workflow-card">
                        <div className="workflow-card-title"><span>Replay detected patterns</span></div>
                        {workflowGame.replay_patterns.map((pattern, idx) => (
                          <div key={`${pattern.pattern_name}-${idx}`} className="workflow-pattern-row">
                            <span>{pattern.pattern_name}</span>
                            <span>{pattern.value}</span>
                          </div>
                        ))}
                      </div>
                    )}
                    {workflowGame.team_patterns?.length > 0 && (
                      <div className="workflow-card">
                        <div className="workflow-card-title"><span>Team detected patterns</span></div>
                        {workflowGame.team_patterns.map((pattern, idx) => (
                          <div key={`${pattern.team}-${pattern.pattern_name}-${idx}`} className="workflow-pattern-row">
                            <span className="workflow-inline">
                              <span className="team-dot" style={{ backgroundColor: getTeamColor(pattern.team) }}></span>
                              Team {pattern.team} / {pattern.pattern_name}
                            </span>
                            <span>{pattern.value}</span>
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                )}
                <div className="workflow-cards">
                  <div className="workflow-card chart-card">
                    <div className="workflow-card-title"><span>Commands by player</span></div>
                    <PieChart
                      data={(workflowGame.players || []).map((player) => ({ label: player.name, value: player.command_count }))}
                      config={{ pie_label_column: 'label', pie_value_column: 'value' }}
                    />
                  </div>
                  <div className="workflow-card chart-card">
                    <div className="workflow-card-title"><span>Commands by team</span></div>
                    <PieChart
                      data={Object.entries((workflowGame.players || []).reduce((acc, player) => {
                        const key = `Team ${player.team}`;
                        acc[key] = (acc[key] || 0) + Number(player.command_count || 0);
                        return acc;
                      }, {})).map(([label, value]) => ({ label, value }))}
                      config={{ pie_label_column: 'label', pie_value_column: 'value' }}
                    />
                  </div>
                </div>
                <div className="workflow-hints">
                  {workflowGame.narrative_hints?.map((hint, idx) => <div key={idx}>{hint}</div>)}
                </div>
              </>
            ) : (
              <div className="chart-empty">Select a game from the Games tab.</div>
            )}

            <form onSubmit={handleWorkflowAsk} className="workflow-ask-form">
              <input
                className="widget-creation-input"
                value={workflowQuestion}
                onChange={(e) => setWorkflowQuestion(e.target.value)}
                placeholder={openaiEnabled ? 'Ask AI about this game...' : 'Enable AI to ask questions'}
                disabled={!openaiEnabled || askingWorkflow}
              />
              <button className="btn-create-ai" type="submit" disabled={!openaiEnabled || askingWorkflow || !workflowQuestion.trim()}>
                {askingWorkflow ? 'Asking...' : 'Ask AI'}
              </button>
            </form>
            {renderWorkflowAiResult()}
          </div>
        )}

        {activeView === 'player' && (
          <div className="workflow-panel">
            {workflowPlayerLoading ? (
              <div className="loading">Loading player report...</div>
            ) : workflowPlayer ? (
              <>
                <div className="workflow-header-row">
                  <button className="btn-switch" onClick={() => setActiveView('games')}>Back to games</button>
                </div>
                <h2 style={playerAccentColor(workflowPlayer.player_key) ? { color: playerAccentColor(workflowPlayer.player_key) } : undefined}>{workflowPlayer.player_name}</h2>
                <div className="workflow-meta">
                  <span><strong>Games</strong> {workflowPlayer.games_played}</span>
                  <span><strong>Win rate</strong> {(workflowPlayer.win_rate * 100).toFixed(1)}%</span>
                  <span><strong>APM</strong> {workflowPlayer.average_apm?.toFixed(1)}</span>
                  <span><strong>EAPM</strong> {workflowPlayer.average_eapm?.toFixed(1)}</span>
                </div>
                <div className="workflow-cards">
                  <div className="workflow-card chart-card">
                    <div className="workflow-card-title"><span>Race breakdown</span></div>
                    {workflowPlayer.race_breakdown?.map((r) => (
                      <div key={r.race} className="workflow-inline">
                        {getRaceIcon(r.race) ? <img src={getRaceIcon(r.race)} alt={r.race} className="unit-icon-inline" /> : null}
                        <strong>{r.race}</strong>
                        <span>{r.game_count} games</span>
                        <span>{r.wins} wins</span>
                      </div>
                    ))}
                    <PieChart
                      data={(workflowPlayer.race_breakdown || []).map((r) => ({ label: r.race, value: r.game_count }))}
                      config={{ pie_label_column: 'label', pie_value_column: 'value' }}
                    />
                  </div>
                  <div className="workflow-card">
                    <div className="workflow-card-title"><span>Win / Loss</span></div>
                    <PieChart
                      data={[
                        { label: 'Wins', value: workflowPlayer.wins || 0 },
                        { label: 'Losses', value: Math.max((workflowPlayer.games_played || 0) - (workflowPlayer.wins || 0), 0) },
                      ]}
                      config={{ pie_label_column: 'label', pie_value_column: 'value' }}
                    />
                  </div>
                  <div className="workflow-card">
                    <div className="workflow-card-title"><span>Recent games</span></div>
                    {workflowPlayer.recent_games?.slice(0, 6).map((g) => (
                      <div key={g.replay_id}>
                        <button className="btn-switch" onClick={() => openWorkflowGame(g.replay_id)}>{formatRelativeReplayDate(g.replay_date)} - {g.map_name}</button>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="workflow-hints">
                  {workflowPlayer.narrative_hints?.map((hint, idx) => <div key={idx}>{hint}</div>)}
                </div>
              </>
            ) : (
              <div className="chart-empty">Select a player from a game report.</div>
            )}
            <form onSubmit={handleWorkflowAsk} className="workflow-ask-form">
              <input
                className="widget-creation-input"
                value={workflowQuestion}
                onChange={(e) => setWorkflowQuestion(e.target.value)}
                placeholder={openaiEnabled ? 'Ask AI about this player...' : 'Enable AI to ask questions'}
                disabled={!openaiEnabled || askingWorkflow}
              />
              <button className="btn-create-ai" type="submit" disabled={!openaiEnabled || askingWorkflow || !workflowQuestion.trim()}>
                {askingWorkflow ? 'Asking...' : 'Ask AI'}
              </button>
            </form>
            {renderWorkflowAiResult()}
          </div>
        )}

        {activeView === 'dashboards' && (
          <>
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

            <div className="widgets-grid">
              {sortedWidgets.map((widget) => (
                <Widget
                  key={widget.id}
                  widget={widget}
                  dashboardUrl={currentDashboardUrl}
                  variableValues={variableValues}
                  onDelete={handleDeleteWidget}
                  onUpdate={handleUpdateWidget}
                />
              ))}
            </div>
          </>
        )}
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
          dashboardUrl={currentDashboardUrl}
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
