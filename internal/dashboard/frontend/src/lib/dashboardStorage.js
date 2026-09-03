const AUTO_INGEST_SETTINGS_KEY = 'dashboard_auto_ingest_settings';

export const getStoredAutoIngestSettings = () => {
  try {
    const stored = localStorage.getItem(AUTO_INGEST_SETTINGS_KEY);
    if (!stored) {
      return { enabled: false };
    }
    const parsed = JSON.parse(stored);
    return {
      enabled: parsed?.enabled === true,
    };
  } catch (e) {
    console.error('Failed to load auto-ingest settings from localStorage:', e);
    return { enabled: false };
  }
};

export const saveAutoIngestSettings = (settings) => {
  try {
    localStorage.setItem(AUTO_INGEST_SETTINGS_KEY, JSON.stringify({
      enabled: settings?.enabled === true,
    }));
  } catch (e) {
    console.error('Failed to save auto-ingest settings to localStorage:', e);
  }
};

// Whether the skill-proxy distributions overlay the built-in progamer
// profiles. A view preference, not data: it lives in this browser only.
const SHOW_FEATURED_PROS_KEY = 'dashboard_show_featured_pros';

export const getStoredShowFeaturedPros = () => {
  try {
    const stored = localStorage.getItem(SHOW_FEATURED_PROS_KEY);
    return stored === null ? true : stored === '1';
  } catch (e) {
    return true;
  }
};

export const saveShowFeaturedPros = (enabled) => {
  try {
    localStorage.setItem(SHOW_FEATURED_PROS_KEY, enabled ? '1' : '0');
  } catch (e) {
    console.error('Failed to save featured pros preference to localStorage:', e);
  }
};
