/** Main workflow SPA route (query string). */

export const MAIN_VIEWS = ['games', 'players', 'game', 'player', 'dashboards'];

export const MAIN_GAME_TABS = [
  'summary',
  'events',
  'units',
  'timings',
  'build-orders',
  'first-unit-efficiency',
  'unit-production-cadence',
  'viewport-multitasking',
];

export const MAIN_PLAYERS_TABS = [
  'summary',
  'apm-histogram',
  'first-unit-delay',
  'unit-production-cadence',
  'viewport-multitasking',
];

const normalizeSearch = (search) => {
  if (!search || search === '?') return '';
  return String(search).startsWith('?') ? search.slice(1) : search;
};

export const parseReplayIdParam = (raw) => {
  if (raw == null) return null;
  const s = String(raw).trim();
  if (!s) return null;
  const n = Number(s);
  if (Number.isFinite(n)) return n;
  return s;
};

export const normalizePlayerKeyParam = (raw) => String(raw || '').trim().toLowerCase();

const pickEnum = (value, allowed, fallback) => {
  const v = String(value || '').trim().toLowerCase();
  return allowed.includes(v) ? v : fallback;
};

/**
 * @param {string} search - full `location.search` or without `?`
 * @returns {{
 *   view: string,
 *   replayId: number|string|null,
 *   playerKey: string,
 *   gameTab: string,
 *   playersTab: string,
 *   dash: string|null,
 * }}
 */
export function parseMainRouteSearch(search) {
  const params = new URLSearchParams(normalizeSearch(search));
  let view = pickEnum(params.get('view'), MAIN_VIEWS, 'games');

  const replayRaw = params.get('replay');
  const playerRaw = params.get('player');
  const gameTabRaw = params.get('gameTab');
  const playersTabRaw = params.get('playersTab');
  const dashRaw = params.get('dash');

  let replayId = view === 'game' ? parseReplayIdParam(replayRaw) : null;
  let playerKey = view === 'player' ? normalizePlayerKeyParam(playerRaw) : '';
  let gameTab = pickEnum(gameTabRaw, MAIN_GAME_TABS, 'summary');
  let playersTab = pickEnum(playersTabRaw, MAIN_PLAYERS_TABS, 'summary');
  let dash = dashRaw != null && String(dashRaw).trim() !== '' ? String(dashRaw).trim() : null;

  if (view === 'game' && replayId == null) {
    view = 'games';
    gameTab = 'summary';
    replayId = null;
  }
  if (view === 'player' && !playerKey) {
    view = 'games';
    playerKey = '';
  }
  if (view === 'dashboards' && !dash) {
    dash = 'default';
  }

  return {
    view,
    replayId,
    playerKey,
    gameTab,
    playersTab,
    dash: view === 'dashboards' ? dash || 'default' : null,
  };
}

/**
 * @param {{
 *   activeView: string,
 *   selectedReplayId: number|string|null,
 *   selectedPlayerKey: string,
 *   mainGameTab: string,
 *   mainPlayersTab: string,
 *   currentDashboardUrl: string,
 * }} s
 * @returns {string} query string without leading `?` (empty = default games home)
 */
export function buildMainRouteSearch(s) {
  const view = pickEnum(s.activeView, MAIN_VIEWS, 'games');
  const params = new URLSearchParams();

  if (view === 'games') {
    return '';
  }

  params.set('view', view);

  if (view === 'game' && s.selectedReplayId != null && String(s.selectedReplayId).trim() !== '') {
    params.set('replay', String(s.selectedReplayId));
    const tab = pickEnum(s.mainGameTab, MAIN_GAME_TABS, 'summary');
    if (tab !== 'summary') {
      params.set('gameTab', tab);
    }
  }

  if (view === 'player' && s.selectedPlayerKey) {
    params.set('player', s.selectedPlayerKey);
  }

  if (view === 'players') {
    const tab = pickEnum(s.mainPlayersTab, MAIN_PLAYERS_TABS, 'summary');
    if (tab !== 'summary') {
      params.set('playersTab', tab);
    }
  }

  if (view === 'dashboards') {
    const dash = String(s.currentDashboardUrl || 'default').trim() || 'default';
    if (dash !== 'default') {
      params.set('dash', dash);
    }
  }

  return params.toString();
}

export function mainRouteHref(searchWithoutQuestion) {
  const path = typeof window !== 'undefined' ? window.location.pathname || '/' : '/';
  if (!searchWithoutQuestion) return path || '/';
  return `${path}?${searchWithoutQuestion}`;
}

/** Compare two query strings (with or without `?`) for same key/value pairs. */
export function mainRouteSearchEquivalent(a, b) {
  const sa = new URLSearchParams(normalizeSearch(a));
  const sb = new URLSearchParams(normalizeSearch(b));
  const keysA = [...new Set([...sa.keys()])].sort();
  const keysB = [...new Set([...sb.keys()])].sort();
  if (keysA.length !== keysB.length) return false;
  for (let i = 0; i < keysA.length; i += 1) {
    if (keysA[i] !== keysB[i]) return false;
    if (sa.get(keysA[i]) !== sb.get(keysA[i])) return false;
  }
  return true;
}

/** Semantic equality (e.g. `view=games` vs empty search both mean games home). */
export function mainRouteSnapshotEqual(searchA, searchB) {
  const ra = parseMainRouteSearch(searchA);
  const rb = parseMainRouteSearch(searchB);
  const dashA = ra.view === 'dashboards' ? ra.dash || 'default' : null;
  const dashB = rb.view === 'dashboards' ? rb.dash || 'default' : null;
  return ra.view === rb.view
    && String(ra.replayId ?? '') === String(rb.replayId ?? '')
    && ra.playerKey === rb.playerKey
    && ra.gameTab === rb.gameTab
    && ra.playersTab === rb.playersTab
    && dashA === dashB;
}
