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
