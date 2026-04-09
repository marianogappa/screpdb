export const getStoredVariableValues = (dashboardUrl) => {
  try {
    const key = `dashboard_vars_${dashboardUrl}`;
    const stored = localStorage.getItem(key);
    return stored ? JSON.parse(stored) : null;
  } catch (e) {
    console.error('Failed to load variable values from localStorage:', e);
    return null;
  }
};

export const saveVariableValues = (dashboardUrl, values) => {
  try {
    const key = `dashboard_vars_${dashboardUrl}`;
    localStorage.setItem(key, JSON.stringify(values));
  } catch (e) {
    console.error('Failed to save variable values to localStorage:', e);
  }
};

const AUTO_INGEST_SETTINGS_KEY = 'dashboard_auto_ingest_settings';

export const getStoredAutoIngestSettings = () => {
  try {
    const stored = localStorage.getItem(AUTO_INGEST_SETTINGS_KEY);
    if (!stored) {
      return { enabled: false, intervalSeconds: 60 };
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
    return { enabled: false, intervalSeconds: 60 };
  }
};

export const saveAutoIngestSettings = (settings) => {
  try {
    localStorage.setItem(AUTO_INGEST_SETTINGS_KEY, JSON.stringify(settings));
  } catch (e) {
    console.error('Failed to save auto-ingest settings to localStorage:', e);
  }
};
