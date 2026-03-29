const API_BASE = '/api';

export const api = {
  // Dashboard endpoints
  listDashboards: async () => {
    const response = await fetch(`${API_BASE}/dashboard`);
    if (!response.ok) throw new Error('Failed to list dashboards');
    return response.json();
  },

  getDashboard: async (url, variableValues = null) => {
    const options = {
      method: variableValues ? 'POST' : 'GET',
      headers: { 'Content-Type': 'application/json' },
    };
    if (variableValues) {
      options.body = JSON.stringify({ variable_values: variableValues });
    }
    const response = await fetch(`${API_BASE}/dashboard/${url}`, options);
    if (!response.ok) throw new Error('Failed to get dashboard');
    return response.json();
  },

  createDashboard: async (data) => {
    const response = await fetch(`${API_BASE}/dashboard`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to create dashboard');
    }
    return response.json();
  },

  updateDashboard: async (url, data) => {
    const response = await fetch(`${API_BASE}/dashboard/${url}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to update dashboard');
    }
  },

  deleteDashboard: async (url) => {
    const response = await fetch(`${API_BASE}/dashboard/${url}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to delete dashboard');
    }
  },

  // Widget endpoints
  createWidget: async (dashboardUrl, prompt) => {
    const body = prompt ? { Prompt: prompt } : {};
    const response = await fetch(`${API_BASE}/dashboard/${dashboardUrl}/widget`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to create widget');
    }
    return response.json();
  },

  updateWidget: async (dashboardUrl, widgetId, data) => {
    const response = await fetch(`${API_BASE}/dashboard/${dashboardUrl}/widget/${widgetId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to update widget');
    }
  },

  deleteWidget: async (dashboardUrl, widgetId) => {
    const response = await fetch(`${API_BASE}/dashboard/${dashboardUrl}/widget/${widgetId}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to delete widget');
    }
  },

  executeQuery: async (query, variableValues = null, dashboardUrl = null) => {
    const response = await fetch(`${API_BASE}/query`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query,
        variable_values: variableValues || {},
        dashboard_url: dashboardUrl || '',
      }),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to execute query');
    }
    return response.json();
  },

  getQueryVariables: async (query, dashboardUrl = null) => {
    const response = await fetch(`${API_BASE}/query/variables`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query, dashboard_url: dashboardUrl || '' }),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get query variables');
    }
    return response.json();
  },

  startIngest: async (data) => {
    const response = await fetch(`${API_BASE}/ingest`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data || {}),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to start ingestion');
    }
    return response.json();
  },

  listWorkflowGames: async ({ limit = 20, offset = 0, filters = {} } = {}) => {
    const params = new URLSearchParams();
    params.set('limit', String(limit));
    params.set('offset', String(offset));
    const playerFilters = Array.isArray(filters.player) ? filters.player : [];
    const mapFilters = Array.isArray(filters.map) ? filters.map : [];
    const durationFilters = Array.isArray(filters.duration) ? filters.duration : [];
    const featuringFilters = Array.isArray(filters.featuring) ? filters.featuring : [];
    playerFilters.forEach((value) => {
      if (String(value || '').trim()) params.append('player', String(value).trim());
    });
    mapFilters.forEach((value) => {
      if (String(value || '').trim()) params.append('map', String(value).trim());
    });
    durationFilters.forEach((value) => {
      if (String(value || '').trim()) params.append('duration', String(value).trim());
    });
    featuringFilters.forEach((value) => {
      if (String(value || '').trim()) params.append('featuring', String(value).trim());
    });
    const response = await fetch(`${API_BASE}/workflow/games?${params.toString()}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to list workflow games');
    }
    return response.json();
  },

  getWorkflowGame: async (replayId) => {
    const response = await fetch(`${API_BASE}/workflow/games/${encodeURIComponent(replayId)}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get workflow game');
    }
    return response.json();
  },

  getWorkflowPlayer: async (playerKey) => {
    const response = await fetch(`${API_BASE}/workflow/players/${encodeURIComponent(playerKey)}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get workflow player');
    }
    return response.json();
  },

  getWorkflowPlayerMetrics: async (playerKey) => {
    const response = await fetch(`${API_BASE}/workflow/players/${encodeURIComponent(playerKey)}/metrics`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get workflow player metrics');
    }
    return response.json();
  },

  getWorkflowPlayerOutliers: async (playerKey) => {
    const response = await fetch(`${API_BASE}/workflow/players/${encodeURIComponent(playerKey)}/outliers`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get workflow player outliers');
    }
    return response.json();
  },

  getWorkflowPlayerColors: async () => {
    const response = await fetch(`${API_BASE}/workflow/player-colors`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get workflow player colors');
    }
    return response.json();
  },

  askWorkflowGame: async (replayId, question) => {
    const response = await fetch(`${API_BASE}/workflow/games/${encodeURIComponent(replayId)}/ask`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question }),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to ask workflow game question');
    }
    return response.json();
  },

  askWorkflowPlayer: async (playerKey, question) => {
    const response = await fetch(`${API_BASE}/workflow/players/${encodeURIComponent(playerKey)}/ask`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question }),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to ask workflow player question');
    }
    return response.json();
  },
};
