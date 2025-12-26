const API_BASE = '/api';

export const api = {
  // Dashboard endpoints
  listDashboards: async () => {
    const response = await fetch(`${API_BASE}/dashboard`);
    if (!response.ok) throw new Error('Failed to list dashboards');
    return response.json();
  },

  getDashboard: async (url) => {
    const response = await fetch(`${API_BASE}/dashboard/${url}`);
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
      method: 'POST',
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
    const response = await fetch(`${API_BASE}/dashboard/${dashboardUrl}/widget`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ Prompt: prompt }),
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
};

