/** Main workflow SPA route (query string). */

export const MAIN_VIEWS = ['games', 'players', 'session', 'game', 'player'];

export const MAIN_GAME_TABS = [
  'summary',
  'events',
  'units',
  'hotkeys',
  'skill',
  'supply-timeline',
  'timings',
  'build-orders',
  'mutalisk-timing',
  'alliances',
];

export const MAIN_PLAYERS_TABS = [
  'summary',
  'skill',
];

export const MAIN_PLAYER_TABS = [
  'summary',
  'skill',
  'hotkeys',
  'chat-summary',
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
 *   buildOrder: string,
 *   playersTab: string,
 *   playerTab: string,
 *   playerSubtab: string,
 * }}
 */
export function parseMainRouteSearch(search) {
  const params = new URLSearchParams(normalizeSearch(search));
  let view = pickEnum(params.get('view'), MAIN_VIEWS, 'games');

  const replayRaw = params.get('replay');
  const playerRaw = params.get('player');
  const gameTabRaw = params.get('gameTab');
  const buildOrderRaw = params.get('bo');
  const playersTabRaw = params.get('playersTab');
  const playerTabRaw = params.get('playerTab');
  const playerSubtabRaw = params.get('playerSubtab');

  let replayId = view === 'game' ? parseReplayIdParam(replayRaw) : null;
  let playerKey = view === 'player' ? normalizePlayerKeyParam(playerRaw) : '';
  // The Skill tab replaced Macro and the Skill proxies views; their old links open it, on the game, the
  // Players page and the player page alike.
  const legacyGameTab = { macro: 'skill', 'first-unit-efficiency': 'skill', 'unit-production-cadence': 'skill', 'viewport-multitasking': 'skill' };
  let gameTab = pickEnum(legacyGameTab[gameTabRaw] || gameTabRaw, MAIN_GAME_TABS, 'summary');
  let buildOrder = gameTab === 'build-orders' ? String(buildOrderRaw || '').trim() : '';
  const legacyPlayersTab = { 'apm-histogram': 'skill', 'unit-production-cadence': 'skill', 'viewport-multitasking': 'skill' };
  let playersTab = pickEnum(legacyPlayersTab[playersTabRaw] || playersTabRaw, MAIN_PLAYERS_TABS, 'summary');
  let playerTab = pickEnum(playerTabRaw === 'skill-proxies' ? 'skill' : playerTabRaw, MAIN_PLAYER_TABS, 'summary');
  // playerSubtab is a race subtab of the summary: dynamic, so kept as given.
  let playerSubtab = playerTab === 'summary' ? String(playerSubtabRaw || '').trim().toLowerCase() : '';

  if (view === 'game' && replayId == null) {
    view = 'games';
    gameTab = 'summary';
    buildOrder = '';
    replayId = null;
  }
  if (view === 'player' && !playerKey) {
    view = 'games';
    playerKey = '';
  }

  return {
    view,
    replayId,
    playerKey,
    gameTab,
    buildOrder,
    playersTab,
    playerTab,
    playerSubtab,
  };
}

/**
 * @param {{
 *   activeView: string,
 *   selectedReplayId: number|string|null,
 *   selectedPlayerKey: string,
 *   mainGameTab: string,
 *   mainGameBuildOrder?: string,
 *   mainPlayersTab: string,
 *   mainPlayerTab: string,
 *   mainPlayerSubtab: string,
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
    const buildOrder = String(s.mainGameBuildOrder || '').trim();
    if (tab === 'build-orders' && buildOrder) {
      params.set('bo', buildOrder);
    }
  }

  if (view === 'player' && s.selectedPlayerKey) {
    params.set('player', s.selectedPlayerKey);
    const tab = pickEnum(s.mainPlayerTab, MAIN_PLAYER_TABS, 'summary');
    if (tab !== 'summary') {
      params.set('playerTab', tab);
    }
    const sub = String(s.mainPlayerSubtab || '').trim().toLowerCase();
    if (tab === 'summary' && sub) {
      // Race subtab — opaque; persist only when set.
      params.set('playerSubtab', sub);
    }
  }

  if (view === 'players') {
    const tab = pickEnum(s.mainPlayersTab, MAIN_PLAYERS_TABS, 'summary');
    if (tab !== 'summary') {
      params.set('playersTab', tab);
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
  return ra.view === rb.view
    && String(ra.replayId ?? '') === String(rb.replayId ?? '')
    && ra.playerKey === rb.playerKey
    && ra.gameTab === rb.gameTab
    && ra.buildOrder === rb.buildOrder
    && ra.playersTab === rb.playersTab
    && ra.playerTab === rb.playerTab
    && ra.playerSubtab === rb.playerSubtab;
}

/** A build order section's key in the `bo` parameter: one player's one build order. */
export const buildOrderKey = (bo) => `${bo?.player_id}:${bo?.feature_key}`;
