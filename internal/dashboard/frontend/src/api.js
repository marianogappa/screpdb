const API_BASE = '/api';
const API_CUSTOM = `${API_BASE}/custom`;
const buildWebSocketURL = (path) => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  return `${protocol}//${window.location.host}${path}`;
};

export const api = {
  startIngest: async (data) => {
    const response = await fetch(`${API_CUSTOM}/ingest`, {
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

  getIngestSettings: async () => {
    const response = await fetch(`${API_CUSTOM}/ingest/settings`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to load ingest settings');
    }
    return response.json();
  },

  updateIngestSettings: async (data) => {
    const response = await fetch(`${API_CUSTOM}/ingest/settings`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to update ingest settings');
    }
    return response.json();
  },

  loadSampleSet: async () => {
    const response = await fetch(`${API_CUSTOM}/sample-set/load`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to load sample set');
    }
    return response.json();
  },

  createIngestLogsSocket: () => new WebSocket(buildWebSocketURL(`${API_CUSTOM}/ingest/logs`)),

  getStaleReplaysCount: async () => {
    const response = await fetch(`${API_CUSTOM}/replays/stale-count`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to load stale replays count');
    }
    return response.json();
  },

  listAliases: async () => {
    const response = await fetch(`${API_CUSTOM}/aliases`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to list aliases');
    }
    return response.json();
  },

  importAliases: async (aliasesPayload) => {
    const response = await fetch(`${API_CUSTOM}/aliases`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ aliases: aliasesPayload }),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to import aliases');
    }
    return response.json();
  },

  upsertAliasEntry: async (entry) => {
    const response = await fetch(`${API_CUSTOM}/aliases/entry`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(entry),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to upsert alias entry');
    }
    return response.json();
  },

  deleteAliasEntry: async (id) => {
    const response = await fetch(`${API_CUSTOM}/aliases/${id}`, {
      method: 'DELETE',
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to delete alias entry');
    }
    return response.json();
  },

  getHealth: async () => {
    const response = await fetch(`${API_BASE}/health`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to load health status');
    }
    return response.json();
  },

  quit: async () => {
    const response = await fetch(`${API_CUSTOM}/quit`, { method: 'POST' });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to stop screpdb');
    }
    return response.json();
  },

  getUpdateStatus: async () => {
    const response = await fetch(`${API_CUSTOM}/update/status`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to load update status');
    }
    return response.json();
  },

  applyUpdate: async () => {
    const response = await fetch(`${API_CUSTOM}/update/apply`, { method: 'POST' });
    let payload = null;
    try {
      payload = await response.json();
    } catch (_err) {
      payload = null;
    }
    if (!response.ok) {
      throw new Error(payload?.error || 'Failed to apply update');
    }
    return payload;
  },

  getGlobalReplayFilter: async () => {
    const response = await fetch(`${API_CUSTOM}/global-replay-filter`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get global replay filter');
    }
    return response.json();
  },

  updateGlobalReplayFilter: async (data) => {
    const response = await fetch(`${API_CUSTOM}/global-replay-filter`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to update global replay filter');
    }
    return response.json();
  },

  listGames: async ({ limit = 20, offset = 0, filters = {} } = {}) => {
    const params = new URLSearchParams();
    params.set('limit', String(limit));
    params.set('offset', String(offset));
    const playerFilters = Array.isArray(filters.player) ? filters.player : [];
    const mapFilters = Array.isArray(filters.map) ? filters.map : [];
    const durationFilters = Array.isArray(filters.duration) ? filters.duration : [];
    const featuringFilters = Array.isArray(filters.featuring) ? filters.featuring : [];
    const matchupFilters = Array.isArray(filters.matchup) ? filters.matchup : [];
    const mapKindFilters = Array.isArray(filters.mapKind) ? filters.mapKind : [];
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
    matchupFilters.forEach((value) => {
      if (String(value || '').trim()) params.append('matchup', String(value).trim());
    });
    mapKindFilters.forEach((value) => {
      if (String(value || '').trim()) params.append('map_kind', String(value).trim());
    });
    const response = await fetch(`${API_BASE}/games?${params.toString()}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to list games');
    }
    return response.json();
  },

  listPlayers: async ({
    limit = 20,
    offset = 0,
    sortBy = 'games',
    sortDir = 'desc',
    filters = {},
  } = {}) => {
    const params = new URLSearchParams();
    params.set('limit', String(limit));
    params.set('offset', String(offset));
    params.set('sort_by', String(sortBy || 'games'));
    params.set('sort_dir', String(sortDir || 'desc'));

    const name = String(filters.name || '').trim();
    if (name) params.set('name', name);

    const lastPlayedFilters = Array.isArray(filters.lastPlayed) ? filters.lastPlayed : [];
    lastPlayedFilters.forEach((value) => {
      const v = String(value || '').trim();
      if (v) params.append('last_played', v);
    });

    if (filters.onlyFivePlus) params.set('only_5_plus', '1');

    const response = await fetch(`${API_BASE}/players?${params.toString()}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to list players');
    }
    return response.json();
  },

  getPlayersApmHistogram: async () => {
    const response = await fetch(`${API_BASE}/players/insights/apm-histogram`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get players APM histogram');
    }
    return response.json();
  },

  getPlayersUnitProductionCadence: async ({ filter = 'strict', minGames = 4, limit = 0 } = {}) => {
    const params = new URLSearchParams();
    if (String(filter || '').trim()) params.set('filter', String(filter).trim());
    if (Number.isFinite(Number(minGames)) && Number(minGames) > 0) params.set('min_games', String(Math.floor(Number(minGames))));
    if (Number.isFinite(Number(limit)) && Number(limit) >= 0) params.set('limit', String(Math.floor(Number(limit))));
    const query = params.toString();
    const response = await fetch(`${API_BASE}/players/insights/unit-production-cadence${query ? `?${query}` : ''}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get players unit production cadence');
    }
    return response.json();
  },

  getPlayersViewportMultitasking: async () => {
    const response = await fetch(`${API_BASE}/players/insights/viewport-multitasking`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get players viewport multitasking');
    }
    return response.json();
  },

  getGame: async (replayId) => {
    const response = await fetch(`${API_BASE}/games/${encodeURIComponent(replayId)}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get game');
    }
    return response.json();
  },

  seeGame: async (replayId) => {
    const response = await fetch(`${API_BASE}/games/${encodeURIComponent(replayId)}/see`, {
      method: 'POST',
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to stage replay for watch');
    }
    return response.json();
  },

  getPlayer: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player');
    }
    return response.json();
  },

  getPlayerRecentGames: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/recent-games`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player recent games');
    }
    return response.json();
  },

  getPlayerChatSummary: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/chat-summary`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player chat summary');
    }
    return response.json();
  },

  getPlayerInsight: async (playerKey, type) => {
    const params = new URLSearchParams();
    if (String(type || '').trim()) params.set('type', String(type).trim());
    const query = params.toString();
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/insight${query ? `?${query}` : ''}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player insight');
    }
    return response.json();
  },

  getPlayerOutliers: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/outliers`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player outliers');
    }
    return response.json();
  },

  getPlayerSummaryPerMatchup: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/summary/per-matchup`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player matchup summary');
    }
    return response.json();
  },

  getPlayerSummarySpecial: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/summary/special`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player special summary');
    }
    return response.json();
  },

  getPlayerSummaryOutliers: async (playerKey, category) => {
    const params = new URLSearchParams({ category });
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/summary/outliers?${params.toString()}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player summary outliers');
    }
    return response.json();
  },

  getPlayerApmHistogram: async (playerKey) => {
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/insights/apm-histogram`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player APM histogram');
    }
    return response.json();
  },

  getPlayerUnitProductionCadence: async (playerKey, { filter = 'strict' } = {}) => {
    const params = new URLSearchParams();
    if (String(filter || '').trim()) params.set('filter', String(filter).trim());
    const query = params.toString();
    const response = await fetch(`${API_BASE}/players/${encodeURIComponent(playerKey)}/insights/unit-production-cadence${query ? `?${query}` : ''}`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player unit production cadence');
    }
    return response.json();
  },

  getPlayerColors: async () => {
    const response = await fetch(`${API_BASE}/player-colors`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get player colors');
    }
    return response.json();
  },

  getScrepColors: async () => {
    const response = await fetch(`${API_BASE}/screp-colors`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get screp colors');
    }
    return response.json();
  },

  getMarkerDefinitions: async () => {
    const response = await fetch(`${API_CUSTOM}/markers/definitions`);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Failed to get marker definitions');
    }
    return response.json();
  },
};
