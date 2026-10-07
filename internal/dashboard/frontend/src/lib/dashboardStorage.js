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

// Which "plays unlike this handle" notes the viewer has dismissed, keyed by
// replay and player so a dismissal never hides the note on another game.
const DISMISSED_UNLIKE_HANDLE_KEY = 'dashboard_dismissed_unlike_handle';

export const unlikeHandleDismissKey = (replayId, playerKey) => `${replayId}:${playerKey}`;

export const getStoredDismissedUnlikeHandle = () => {
  try {
    const parsed = JSON.parse(localStorage.getItem(DISMISSED_UNLIKE_HANDLE_KEY) || '[]');
    return new Set(Array.isArray(parsed) ? parsed : []);
  } catch (e) {
    return new Set();
  }
};

export const saveDismissedUnlikeHandle = (dismissed) => {
  try {
    localStorage.setItem(DISMISSED_UNLIKE_HANDLE_KEY, JSON.stringify([...dismissed]));
  } catch (e) {
    console.error('Failed to save dismissed notes to localStorage:', e);
  }
};
