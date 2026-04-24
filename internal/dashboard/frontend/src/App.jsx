import React, { useState, useEffect, useMemo, useRef } from 'react';
import { api } from './api';
import Widget from './components/Widget';
import DashboardManager from './components/DashboardManager';
import EditDashboardModal from './components/EditDashboardModal';
import EditWidgetFullscreen from './components/EditWidgetFullscreen';
import GlobalReplayFilterModal from './components/GlobalReplayFilterModal';
import IngestModal from './components/IngestModal';
import WidgetCreationSpinner from './components/WidgetCreationSpinner';
import PieChart from './components/charts/PieChart';
import Gauge from './components/charts/Gauge';
import Table from './components/charts/Table';
import BarChart from './components/charts/BarChart';
import LineChart from './components/charts/LineChart';
import ScatterPlot from './components/charts/ScatterPlot';
import Histogram from './components/charts/Histogram';
import Heatmap from './components/charts/Heatmap';
import TimingScatterRows from './components/charts/TimingScatterRows';
import FirstUnitEfficiencyTimelineRows from './components/charts/FirstUnitEfficiencyTimelineRows';
import BuildOrderTimelineRows from './components/charts/BuildOrderTimelineRows';
import { getUnitIcon, normalizeUnitName } from './lib/gameAssets';
import {
  getStoredVariableValues,
  saveVariableValues,
  getStoredAutoIngestSettings,
  saveAutoIngestSettings,
} from './lib/dashboardStorage';
import {
  formatDuration,
  formatRelativeReplayDate,
  formatDaysAgoCompact,
  formatPercent,
} from './lib/formatters';
import {
  parseMainRouteSearch,
  buildMainRouteSearch,
  mainRouteHref,
  mainRouteSnapshotEqual,
  MAIN_GAME_TABS,
} from './lib/mainRoute';
import './styles.css';

const buildHistogramSummaryFromPlayers = (players) => {
  const safePlayers = Array.isArray(players)
    ? players
      .map((player) => ({
        ...player,
        player_key: String(player?.player_key || '').trim().toLowerCase(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.average_apm || 0),
        games_played: Number(player?.games_played || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0)
    : [];

  if (safePlayers.length === 0) {
    return {
      points: [],
      bins: [],
      mean: 0,
      stddev: 0,
      playersIncluded: 0,
      maxGames: 5,
    };
  }

  const values = safePlayers.map((player) => player.average_apm).sort((a, b) => a - b);
  const mean = values.reduce((sum, value) => sum + value, 0) / values.length;
  const variance = values.reduce((sum, value) => {
    const diff = value - mean;
    return sum + (diff * diff);
  }, 0) / values.length;
  const stddev = Math.sqrt(variance);

  let binCount = Math.round(Math.sqrt(values.length));
  if (binCount < 8) binCount = 8;
  if (binCount > 24) binCount = 24;

  const minValue = values[0];
  const maxValue = values[values.length - 1];
  let bins = [];
  if (maxValue <= minValue) {
    bins = [{ x0: minValue, x1: minValue + 1, count: values.length }];
  } else {
    let width = (maxValue - minValue) / binCount;
    if (width <= 0) width = 1;
    bins = Array.from({ length: binCount }, (_, idx) => {
      const x0 = minValue + (idx * width);
      const x1 = idx === binCount - 1 ? maxValue : minValue + ((idx + 1) * width);
      return { x0, x1, count: 0 };
    });
    values.forEach((value) => {
      let idx = Math.floor((value - minValue) / width);
      if (idx < 0) idx = 0;
      if (idx >= binCount) idx = binCount - 1;
      bins[idx].count += 1;
    });
  }

  const maxGames = safePlayers.reduce((maxValue, player) => Math.max(maxValue, player.games_played), 5);
  return {
    points: safePlayers,
    bins,
    mean,
    stddev,
    playersIncluded: safePlayers.length,
    maxGames,
  };
};

const getRaceIcon = (race) => {
  const value = String(race || '').toLowerCase();
  if (value === 'protoss') return getUnitIcon('probe');
  if (value === 'terran') return getUnitIcon('scv');
  if (value === 'zerg') return getUnitIcon('drone');
  return null;
};

const normalizeEventType = (eventType) => String(eventType || '').trim().toLowerCase();

/** Aligns with NeverUsedHotkeysPlayerDetector (7+ minute replays). */
const GAME_SUMMARY_NEGATION_MIN_SECONDS = 7 * 60;

const MAIN_GAME_SKILL_PROXY_TABS = ['first-unit-efficiency', 'unit-production-cadence', 'viewport-multitasking'];

const isMainGameSkillProxyTab = (tab) => MAIN_GAME_SKILL_PROXY_TABS.includes(tab);

const SKILL_PROXY_CADENCE_INFO_TEXT = 'ℹ️ How smoothly you keep adding army from the mid game on—not just how much, but how evenly you queue it. Formula: units/min ÷ (1 + gap CV).';

const SKILL_PROXY_VIEWPORT_INFO_TEXT = 'ℹ️ How many times a player switches between places on average per minute.';

const DROP_ACTOR_EVENT_TYPES = ['drop', 'reaver_drop', 'dt_drop'];

const playerIsActorForGameEventTypes = (events, playerID, wantedTypes) => {
  const pid = Number(playerID);
  const wanted = new Set((wantedTypes || []).map((t) => normalizeEventType(t)));
  return (events || []).some((ev) => {
    if (!wanted.has(normalizeEventType(ev?.type))) return false;
    const aid = ev?.actor?.player_id;
    return aid != null && Number(aid) === pid;
  });
};

const dropTransportIconForRace = (race) => {
  const r = String(race || '').toLowerCase();
  if (r === 'terran') return getUnitIcon('dropship');
  if (r === 'protoss') return getUnitIcon('shuttle');
  if (r === 'zerg') return getUnitIcon('overlord');
  return getUnitIcon('dropship');
};

const playerGameSummarySignalParts = (player, gameEvents) => {
  const positive = [];
  const pid = player?.player_id;
  if (pid == null) return { positive: [], noScout: null };
  const events = gameEvents || [];
  const hasGameEvents = Array.isArray(gameEvents) && gameEvents.length > 0;
  if (!hasGameEvents) {
    return { positive: [], noScout: null };
  }

  if (playerIsActorForGameEventTypes(events, pid, DROP_ACTOR_EVENT_TYPES)) {
    positive.push({
      key: `ge-drop-${pid}`,
      icon: dropTransportIconForRace(player?.race),
      label: 'Dropped',
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['cannon_rush'])) {
    positive.push({
      key: `ge-cannon-${pid}`,
      icon: getUnitIcon('photoncannon'),
      label: 'Cannon rush',
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['bunker_rush'])) {
    positive.push({
      key: `ge-bunker-${pid}`,
      icon: getUnitIcon('bunker'),
      label: 'Bunker rush',
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }

  return { positive, noScout: null };
};

const renderGameSummarySignalPill = (pill) => (
  <span key={pill.key} className={pill.className} title={pill.title}>
    {pill.icon ? <img src={pill.icon} alt="" className="workflow-pattern-icon" /> : null}
    <span>{pill.label}</span>
  </span>
);

const isStructuralGameEventType = (eventType) => ['player_start', 'location_inactive'].includes(normalizeEventType(eventType));

const isActorAtOwnNaturalBase = (event) => {
  const kind = String(event?.base?.kind || '').toLowerCase();
  if (kind === 'starting') {
    return false;
  }
  const actorStart = Number(event?.actor_start_clock);
  const naturalOf = event?.base?.natural_of_clock;
  if (naturalOf == null || !Number.isFinite(actorStart)) {
    return false;
  }
  const naturalOfNum = Number(naturalOf);
  return Number.isFinite(naturalOfNum) && actorStart === naturalOfNum;
};

const gameEventLocationLabel = (event) => {
  const baseName = String(event?.base?.name || '').trim();
  if (baseName) {
    const isMineralOnly = event?.base?.mineral_only === true;
    if (isMineralOnly && !baseName.toLowerCase().includes('mineral only')) {
      return `${baseName} (mineral only)`;
    }
    return baseName;
  }
  return '';
};

const gameEventDescription = (event) => {
  const eventType = normalizeEventType(event?.type);
  const actor = String(event?.actor?.name || '').trim();
  const target = String(event?.target?.name || '').trim();
  const location = gameEventLocationLabel(event);

  if (eventType === 'player_start') {
    if (actor && location) return `${actor} starts at ${location}`;
    if (actor) return `${actor} starts`;
    return 'Player start';
  }
  if (eventType === 'leave_game') return actor ? `${actor} leaves the game` : 'Player leaves the game';
  if (eventType === 'location_inactive') return location ? `Location inactive: ${location}` : 'Location inactive';
  if (eventType === 'expansion') {
    if (actor && isActorAtOwnNaturalBase(event)) return `${actor} expands to their natural`;
    return actor && location ? `${actor} expands to ${location}` : 'Expansion';
  }
  if (eventType === 'attack') return actor && target && location ? `${actor} attacks ${target} at ${location}` : 'Attack';
  if (eventType === 'scout') return actor && target && location ? `${actor} scouts ${target} at ${location}` : 'Scout';
  if (eventType === 'drop' || eventType === 'reaver_drop' || eventType === 'dt_drop') {
    return actor && target && location ? `${actor} drops on ${target} at ${location}` : 'Drop';
  }
  if (eventType === 'recall') return actor && target && location ? `${actor} recalls into ${target} at ${location}` : 'Recall';
  if (eventType === 'nuke') return actor && target && location ? `${actor} nukes ${target} at ${location}` : 'Nuke';
  if (eventType === 'cannon_rush' || eventType === 'bunker_rush' || eventType === 'zergling_rush') {
    const rushKind = eventType === 'cannon_rush' ? 'cannon' : eventType === 'bunker_rush' ? 'bunker' : 'zergling';
    if (actor && target) return `${actor} ${rushKind} rushes ${target}`;
    if (actor && location) return `${actor} ${rushKind} rushes at ${location}`;
    if (actor) return `${actor} ${rushKind} rushes`;
    return 'Rush';
  }
  if (eventType === 'takeover') {
    if (actor && isActorAtOwnNaturalBase(event)) return `${actor} takes over their natural`;
    return actor && location ? `${actor} takes over ${location}` : 'Takeover';
  }
  if (eventType === 'proxy_gate' || eventType === 'proxy_rax' || eventType === 'proxy_factory') {
    return actor && location ? `${actor} proxies at ${location}` : 'Proxy';
  }
  if (eventType === 'became_terran') return actor ? `${actor} became Terran` : 'Became Terran';
  if (eventType === 'became_zerg') return actor ? `${actor} became Zerg` : 'Became Zerg';
  return prettyPatternName(event?.type || 'event');
};

const gameEventSearchText = (event) => {
  const parts = [
    gameEventDescription(event),
    event?.type,
    event?.actor?.name,
    event?.target?.name,
    gameEventLocationLabel(event),
    event?.actor_start_clock != null ? String(event.actor_start_clock) : '',
    event?.base?.natural_of_clock != null ? String(event.base.natural_of_clock) : '',
  ];
  return parts.filter(Boolean).join(' ');
};

const gameEventTopicKey = (topicIndex) => `game-event-${topicIndex}`;

const parseGameEventTopicKey = (key) => {
  const m = /^game-event-(\d+)$/.exec(String(key || ''));
  if (!m) return null;
  const idx = Number(m[1]);
  return Number.isFinite(idx) ? idx : null;
};

const SC_PLAYER_COLOR_MAP = {
  red: '#ef4444',
  blue: '#3b82f6',
  teal: '#14b8a6',
  purple: '#8b5cf6',
  orange: '#f97316',
  brown: '#92400e',
  white: '#e5e7eb',
  yellow: '#facc15',
  green: '#22c55e',
  paleyellow: '#fde68a',
  tan: '#d6b18b',
  aqua: '#22d3ee',
};

const playerColorToCss = (colorValue) => {
  const value = String(colorValue || '').trim();
  if (!value) return '#9ca3af';
  if (value.startsWith('#')) return value;
  const key = value.toLowerCase().replace(/\s+/g, '');
  return SC_PLAYER_COLOR_MAP[key] || value.toLowerCase();
};

const legendTextStyle = (rawColorValue, foregroundColor) => {
  const color = playerColorToCss(foregroundColor);
  const key = String(rawColorValue || '').toLowerCase().replace(/\s+/g, '');
  const needsShadow = key === 'black' || key === 'navy' || key === 'darkblue';
  if (!needsShadow) {
    return { color };
  }
  return {
    color,
    textShadow: '0px 0px 4px rgba(255, 255, 255, 1.8)',
  };
};

/** In-game summary UI, use the replay player colour (not DB replay-count heat). */
const gamePlayerNameStyle = (player) => ({
  ...legendTextStyle(String(player?.color || '').trim(), playerColorToCss(player?.color)),
  fontWeight: 600,
});

const renderSummaryMapStack = ({
  legendItems,
  showLegend = true,
  imageUrl,
  mapAlt,
  bounds,
  startPolygons,
}) => (
  <>
    {showLegend && (legendItems || []).length > 0 ? (
      <div className="workflow-event-map-legend workflow-summary-map-legend">
        {(legendItems || []).map((item) => (
          <span
            key={`sum-leg-${item.name}`}
            className="workflow-event-map-legend-item"
            style={legendTextStyle(item.rawColor, item.color)}
          >
            {item.name}
          </span>
        ))}
      </div>
    ) : null}
    <div className="workflow-event-map-frame workflow-summary-map-frame">
      <img src={imageUrl} alt={mapAlt} className="workflow-event-map-image" />
      {bounds && (startPolygons || []).length > 0 ? (
        <svg className="workflow-event-map-overlay" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
          {(startPolygons || []).map((overlay) => (
            <polygon
              key={overlay.key}
              points={overlay.points}
              className="workflow-event-map-base-polygon"
              style={{ fill: `${overlay.ownerColor}66`, stroke: overlay.ownerColor }}
            >
              <title>{overlay.ownerName}</title>
            </polygon>
          ))}
        </svg>
      ) : null}
    </div>
  </>
);

const MAIN_GAME_FEATURING_ORDER = [
  { key: 'carriers', label: 'Carrier', iconKey: 'carrier' },
  { key: 'battlecruisers', label: 'Battlecruiser', iconKey: 'battlecruiser' },
  { key: 'cannon_rush', label: 'Cannon rush', iconKey: 'photoncannon' },
  { key: 'bunker_rush', label: 'Bunker rush', iconKey: 'bunker' },
  { key: 'zergling_rush', label: 'Zergling rush', iconKey: 'zergling' },
  { key: 'mind_control', label: 'Mind control', iconKey: 'darkarchon' },
  { key: 'nukes', label: 'Nukes', iconKey: 'ghost' },
  { key: 'recalls', label: 'Recalls', iconKey: 'arbiter' },
  // Build order pills — keys mirror internal/patterns/markers FeatureKey.
  { key: 'bo_4_pool', label: '4 Pool', iconKey: 'spawningpool' },
  { key: 'bo_9_pool', label: '9 Pool', iconKey: 'spawningpool' },
  { key: 'bo_9_pool_hatch', label: '9 Pool → Hatch', iconKey: 'hatchery' },
  { key: 'bo_9_hatch', label: '9 Hatch', iconKey: 'hatchery' },
  { key: 'bo_12_hatch', label: '12 Hatch', iconKey: 'hatchery' },
  { key: 'bo_nexus_first', label: 'Nexus First', iconKey: 'nexus' },
  { key: 'bo_forge_expa', label: 'Forge Expand', iconKey: 'forge' },
  { key: 'bo_2_gate', label: '2 Gate', iconKey: 'gateway' },
];

// Maps a stored BO pattern_name (e.g. "Build Order: 9 Pool") to its featuring
// key (e.g. "bo_9_pool"). Keys are normalizeUnitName()'d so punctuation and
// spaces are already stripped — kept in sync with internal/patterns/markers/definitions.go.
const BUILD_ORDER_PATTERN_TO_FEATURE_KEY = {
  buildorder4pool: 'bo_4_pool',
  buildorder9pool: 'bo_9_pool',
  buildorder9poolintohatchery: 'bo_9_pool_hatch',
  buildorder9hatch: 'bo_9_hatch',
  buildorder12hatch: 'bo_12_hatch',
  buildordernexusfirst: 'bo_nexus_first',
  buildorderforgeexpand: 'bo_forge_expa',
  buildorder2gate: 'bo_2_gate',
};

const collectFeaturingKeysFromMainGame = (mainGame) => {
  const found = new Set();
  const events = mainGame?.game_events || [];
  events.forEach((ev) => {
    const t = normalizeEventType(ev?.type);
    if (t === 'zergling_rush') found.add('zergling_rush');
    if (t === 'cannon_rush') found.add('cannon_rush');
    if (t === 'bunker_rush') found.add('bunker_rush');
  });
  const considerPattern = (pat) => {
    const n = normalizeUnitName(pat?.pattern_name);
    if (!isPatternTruthy(pat?.value)) return;
    if (n === 'carriers') found.add('carriers');
    if (n === 'battlecruisers') found.add('battlecruisers');
    if (n === 'maderecalls') found.add('recalls');
    if (n === 'threwnukes') found.add('nukes');
    if (n === 'becameterran' || n === 'becamezerg') found.add('mind_control');
    const boFeatureKey = BUILD_ORDER_PATTERN_TO_FEATURE_KEY[n];
    if (boFeatureKey) found.add(boFeatureKey);
  };
  (mainGame?.players || []).forEach((p) => {
    (p.detected_patterns || []).forEach(considerPattern);
  });
  return found;
};

const buildMainGameFeaturingPills = (mainGame) => {
  if (!mainGame) return [];
  const keys = collectFeaturingKeysFromMainGame(mainGame);
  return MAIN_GAME_FEATURING_ORDER.filter((entry) => keys.has(entry.key));
};

const renderFeaturingPill = (pill, keyPrefix) => {
  const icon = pill.iconKey ? getUnitIcon(pill.iconKey) : null;
  return (
    <span key={`${keyPrefix}-${pill.key}`} className="workflow-pattern-pill workflow-pattern-pill-strong workflow-summary-feature-pill">
      {icon ? <img src={icon} alt="" className="workflow-pattern-icon" /> : null}
      <span>{pill.label}</span>
    </span>
  );
};

// Prefer fixed map-dimension bounds when the API provides them. Polygon coords
// from scmapanalyzer are in pixels on a map sized MapWidth*32 x MapHeight*32
// (1 map-tile = 32 px = 4 minitiles, minitile is scmapanalyzer's TilePoint
// unit). Previously we fit bounds to the extent of polygon points which
// stretched overlays away from their real positions when bases didn't span
// the whole map.
const mapBoundsFromDimensions = (widthPixels, heightPixels) => {
  const w = Number(widthPixels);
  const h = Number(heightPixels);
  if (!Number.isFinite(w) || !Number.isFinite(h) || w <= 0 || h <= 0) return null;
  return { minX: 0, minY: 0, maxX: w, maxY: h };
};

const mapBoundsFromGameEvents = (events) => {
  const points = [];
  (Array.isArray(events) ? events : []).forEach((event) => {
    const center = event?.base?.center;
    if (Number.isFinite(center?.x) && Number.isFinite(center?.y)) {
      points.push({ x: Number(center.x), y: Number(center.y) });
    }
    const polygon = Array.isArray(event?.base?.polygon) ? event.base.polygon : [];
    polygon.forEach((point) => {
      if (Number.isFinite(point?.x) && Number.isFinite(point?.y)) {
        points.push({ x: Number(point.x), y: Number(point.y) });
      }
    });
    const ownership = Array.isArray(event?.ownership) ? event.ownership : [];
    ownership.forEach((entry) => {
      const baseCenter = entry?.base?.center;
      if (Number.isFinite(baseCenter?.x) && Number.isFinite(baseCenter?.y)) {
        points.push({ x: Number(baseCenter.x), y: Number(baseCenter.y) });
      }
      const basePolygon = Array.isArray(entry?.base?.polygon) ? entry.base.polygon : [];
      basePolygon.forEach((point) => {
        if (Number.isFinite(point?.x) && Number.isFinite(point?.y)) {
          points.push({ x: Number(point.x), y: Number(point.y) });
        }
      });
    });
  });
  if (points.length === 0) return null;
  let minX = points[0].x;
  let minY = points[0].y;
  let maxX = points[0].x;
  let maxY = points[0].y;
  points.forEach((point) => {
    minX = Math.min(minX, point.x);
    minY = Math.min(minY, point.y);
    maxX = Math.max(maxX, point.x);
    maxY = Math.max(maxY, point.y);
  });
  const pad = 32;
  minX -= pad;
  minY -= pad;
  maxX += pad;
  maxY += pad;
  if (maxX - minX < 1) maxX = minX + 1;
  if (maxY - minY < 1) maxY = minY + 1;
  return { minX, minY, maxX, maxY };
};

const mapPointToPercent = (point, bounds) => {
  if (!point || !bounds) return null;
  const x = Number(point?.x);
  const y = Number(point?.y);
  if (!Number.isFinite(x) || !Number.isFinite(y)) return null;
  const width = bounds.maxX - bounds.minX;
  const height = bounds.maxY - bounds.minY;
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) return null;
  const px = ((x - bounds.minX) / width) * 100;
  const py = ((y - bounds.minY) / height) * 100;
  const clamp = (value) => Math.max(0, Math.min(100, value));
  return { x: clamp(px), y: clamp(py) };
};

const isArrowEventType = (eventType) => ['attack', 'scout', 'drop', 'reaver_drop', 'dt_drop', 'recall', 'nuke', 'cannon_rush', 'bunker_rush', 'zergling_rush'].includes(String(eventType || '').toLowerCase());

const fallbackOverlayUnitNamesForEvent = (eventType) => {
  const normalized = normalizeEventType(eventType);
  if (normalized === 'zergling_rush') return ['zergling'];
  if (normalized === 'cannon_rush') return ['photoncannon'];
  if (normalized === 'bunker_rush') return ['bunker'];
  if (normalized === 'reaver_drop') return ['reaver'];
  if (normalized === 'dt_drop') return ['darktemplar'];
  if (normalized === 'drop') return ['dropship'];
  if (normalized === 'nuke') return ['ghost'];
  return [];
};

// Return clock as a number in [0, 12]. 0 represents scmapanalyzer's "center
// base" (rich middle expansion). 1..12 are regular dial positions. Null
// means unknown — fall back to other lookups.
const eventBaseClock = (event) => {
  const rawClock = Number(event?.base?.clock);
  if (Number.isFinite(rawClock) && rawClock >= 0 && rawClock <= 12) return rawClock;
  const name = String(event?.base?.name || '');
  if (/\bcenter base\b/i.test(name)) return 0;
  const match = name.match(/\b([1-9]|1[0-2])\b/);
  if (!match) return null;
  const parsed = Number(match[1]);
  if (!Number.isFinite(parsed) || parsed < 1 || parsed > 12) return null;
  return parsed;
};

const syntheticPointForClock = (clock) => {
  const safeClock = Number(clock);
  if (!Number.isFinite(safeClock) || safeClock < 0 || safeClock > 12) return null;
  // clock==0 is the center base — place it literally at the map center
  // rather than projecting onto the 12-hour dial circle.
  if (safeClock === 0) return { x: 50, y: 50 };
  const angle = ((safeClock % 12) / 12) * (Math.PI * 2) - (Math.PI / 2);
  const radius = 34;
  return {
    x: 50 + (Math.cos(angle) * radius),
    y: 50 + (Math.sin(angle) * radius),
  };
};

const syntheticPolygonForCenter = (center, radius = 6) => {
  if (!center) return [];
  const out = [];
  for (let idx = 0; idx < 6; idx += 1) {
    const angle = (idx / 6) * (Math.PI * 2);
    out.push({
      x: center.x + (Math.cos(angle) * radius),
      y: center.y + (Math.sin(angle) * radius),
    });
  }
  return out;
};

const eventBaseKey = (event) => {
  const kind = String(event?.base?.kind || '').trim().toLowerCase();
  const clock = eventBaseClock(event);
  // clock can legitimately be 0 (center base). Check for null/undefined,
  // not truthiness, or we collapse center bases into name-based keys.
  if (clock !== null && clock !== undefined) return `${kind || 'base'}:${clock}`;
  const name = String(event?.base?.name || '').trim().toLowerCase();
  if (name) return `${kind || 'base'}:${name}`;
  return '';
};

const eventActorID = (event) => {
  const id = Number(event?.actor?.player_id);
  return Number.isFinite(id) && id > 0 ? id : null;
};

const raceRank = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return 0;
  if (value === 'zerg') return 1;
  if (value === 'protoss') return 2;
  return 3;
};

const getGasMarkerIconForRace = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return getUnitIcon('refinery');
  if (value === 'zerg') return getUnitIcon('extractor');
  if (value === 'protoss') return getUnitIcon('assimilator');
  return getUnitIcon('extractor');
};

const getExpansionMarkerIconForRace = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return getUnitIcon('commandcenter');
  if (value === 'zerg') return getUnitIcon('hatchery');
  if (value === 'protoss') return getUnitIcon('nexus');
  return null;
};

const normalizeTimingDisplayLabel = (label) => {
  const text = String(label || '').trim();
  const match = text.match(/\(([^)]+)\)/);
  if (match && match[1]) return match[1].trim();
  return text;
};

const INLINE_UPGRADE_LABEL_MAP = {
  'Protoss Air Armor': 'Air Armor',
  'Protoss Air Weapons': 'Air ⚔️',
  'Protoss Ground Armor': 'Grnd Armor',
  'Protoss Ground Weapons': 'Grnd ⚔️',
  'Protoss Plasma Shields': 'Shields',
  'Terran Ship Weapons': 'Ship ⚔️',
  'Terran Vehicle Plating': 'Vehicle 🛡️',
  'Terran Vehicle Weapons': 'Vehicle ⚔️',
  'Zerg Carapace': '🛡️',
  'Zerg Flyer Attacks': '🦋 ⚔️',
  'Zerg Melee Attacks': 'Melee ⚔️',
  'Zerg Missile Attacks': 'Missile ⚔️',
};

const inlineTimingUpgradeLabel = (label, order) => {
  const base = String(label || '').trim();
  const abbreviated = INLINE_UPGRADE_LABEL_MAP[base];
  if (!abbreviated) return normalizeTimingDisplayLabel(base);
  const level = Math.max(1, Number(order) || 1);
  return `${abbreviated} +${level}`;
};

const HP_UPGRADE_NAMES = new Set([
  'Terran Infantry Armor',
  'Terran Vehicle Plating',
  'Terran Ship Plating',
  'Zerg Carapace',
  'Zerg Flyer Carapace',
  'Protoss Ground Armor',
  'Protoss Air Armor',
  'Terran Infantry Weapons',
  'Terran Vehicle Weapons',
  'Terran Ship Weapons',
  'Zerg Melee Attacks',
  'Zerg Missile Attacks',
  'Zerg Flyer Attacks',
  'Protoss Ground Weapons',
  'Protoss Air Weapons',
  'Protoss Plasma Shields',
]);

const DEFAULT_HP_UPGRADE_BY_RACE = {
  terran: 'Terran Vehicle Weapons',
  protoss: 'Protoss Ground Weapons',
  zerg: 'Zerg Carapace',
};

const racePrefixForUpgrade = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (!value) return '';
  return `${value.charAt(0).toUpperCase()}${value.slice(1)} `;
};

const setHasUpgradeLoose = (upgradeSet, upgradeName) => {
  const value = String(upgradeName || '').trim();
  if (!value) return false;
  if (upgradeSet.has(value)) return true;
  for (const known of upgradeSet) {
    if (value.startsWith(`${known} `) || value.startsWith(`${known}+`) || value.startsWith(`${known} +`)) {
      return true;
    }
  }
  return false;
};

const UNIT_RANGE_UPGRADE_NAMES = new Set([
  'U-238 Shells (Marine Range)',
  'Ocular Implants (Ghost Sight)',
  'Antennae (Overlord Sight)',
  'Grooved Spines (Hydralisk Range)',
  'Singularity Charge (Dragoon Range)',
  'Sensor Array (Observer Sight)',
  'Charon Boosters (Goliath Range)',
  'Apial Sensors (Scout Sight)',
]);

const UNIT_SPEED_UPGRADE_NAMES = new Set([
  'Ion Thrusters (Vulture Speed)',
  'Pneumatized Carapace (Overlord Speed)',
  'Metabolic Boost (Zergling Speed)',
  'Muscular Augments (Hydralisk Speed)',
  'Leg Enhancement (Zealot Speed)',
  'Gravitic Drive (Shuttle Speed)',
  'Gravitic Booster (Observer Speed)',
  'Gravitic Thrusters (Scout Speed)',
  'Anabolic Synthesis (Ultralisk Speed)',
]);

const ENERGY_UPGRADE_NAMES = new Set([
  'Titan Reactor (Science Vessel Energy)',
  'Moebius Reactor (Ghost Energy)',
  'Apollo Reactor (Wraith Energy)',
  'Colossus Reactor (Battle Cruiser Energy)',
  'Gamete Meiosis (Queen Energy)',
  'Defiler Energy',
  'Khaydarin Core (Arbiter Energy)',
  'Argus Jewel (Corsair Energy)',
  'Khaydarin Amulet (Templar Energy)',
  'Argus Talisman (Dark Archon Energy)',
  'Caduceus Reactor (Medic Energy)',
]);

const CAPACITY_COOLDOWN_DAMAGE_UPGRADE_NAMES = new Set([
  'Scarab Damage',
  'Reaver Capacity',
  'Carrier Capacity',
  'Chitinous Plating (Ultralisk Armor)',
  'Adrenal Glands (Zergling Attack)',
  'Ventral Sacs (Overlord Transport)',
]);

const upgradeCategoryForName = (upgradeName) => {
  const value = String(upgradeName || '').trim();
  if (setHasUpgradeLoose(HP_UPGRADE_NAMES, value)) return 'hp_upgrades';
  if (setHasUpgradeLoose(UNIT_RANGE_UPGRADE_NAMES, value)) return 'unit_range';
  if (setHasUpgradeLoose(UNIT_SPEED_UPGRADE_NAMES, value)) return 'unit_speed';
  if (setHasUpgradeLoose(ENERGY_UPGRADE_NAMES, value)) return 'energy';
  if (setHasUpgradeLoose(CAPACITY_COOLDOWN_DAMAGE_UPGRADE_NAMES, value)) return 'capacity_cooldown_damage';
  return 'capacity_cooldown_damage';
};

const TIMING_CATEGORY_CONFIG = [
  { id: 'expansion', label: 'Expansion', title: 'Expansion timings (1st-4th)', source: 'expansion', markerMode: 'image', markerLabel: 'Expansion' },
  { id: 'gas', label: 'Gas', title: 'Gas timings (1st-4th)', source: 'gas', markerMode: 'image', markerLabel: 'Gas structure' },
  { id: 'hp_upgrades', label: 'HP Upgrades', title: 'HP upgrades timings', source: 'upgrades' },
  { id: 'unit_range', label: 'Unit Range', title: 'Unit range upgrades timings', source: 'upgrades' },
  { id: 'unit_speed', label: 'Unit Speed', title: 'Unit speed upgrades timings', source: 'upgrades' },
  { id: 'energy', label: 'Energy', title: 'Energy upgrades timings', source: 'upgrades' },
  { id: 'capacity_cooldown_damage', label: 'Capacity/Cooldown/Damage', title: 'Capacity, cooldown and damage upgrades timings', source: 'upgrades' },
  { id: 'tech', label: 'Tech', title: 'Tech research timings', source: 'tech' },
];

const TIMING_RACE_ORDER = ['terran', 'zerg', 'protoss'];
const FIRST_UNIT_EFFICIENCY_GROUP_CONFIG = [
  { race: 'protoss', buildingName: 'Forge', unitNames: ['Photon Cannon'] },
  { race: 'protoss', buildingName: 'Gateway', unitNames: ['Zealot'] },
  { race: 'protoss', buildingName: 'Stargate', unitNames: ['Corsair', 'Scout'] },
  { race: 'protoss', buildingName: 'Fleet Beacon', unitNames: ['Carrier'] },
  { race: 'protoss', buildingName: 'Arbiter Tribunal', unitNames: ['Arbiter'] },
  { race: 'terran', buildingName: 'Barracks', unitNames: ['Marine'] },
  { race: 'terran', buildingName: 'Factory', unitNames: ['Vulture', 'Siege Tank'] },
  { race: 'terran', buildingName: 'Physics Lab', unitNames: ['Battlecruiser'] },
  { race: 'zerg', buildingName: 'Spawning Pool', unitNames: ['Zergling'] },
  { race: 'zerg', buildingName: 'Hydralisk Den', unitNames: ['Hydralisk'] },
  { race: 'zerg', buildingName: 'Spire', unitNames: ['Mutalisk', 'Scourge'] },
  { race: 'zerg', buildingName: 'Ultralisk Cavern', unitNames: ['Ultralisk'] },
  { race: 'zerg', buildingName: 'Defiler Mound', unitNames: ['Defiler'] },
];

const prettyRaceName = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return 'Terran';
  if (value === 'zerg') return 'Zerg';
  if (value === 'protoss') return 'Protoss';
  return race || 'Unknown';
};

const BUILDING_TYPE_KEYS = new Set([
  'academy', 'arbitertribunal', 'armory', 'assimilator', 'barracks', 'bunker', 'citadelofadun', 'comsat', 'commandcenter',
  'controltower', 'covertops', 'creepcolony', 'cyberneticscore', 'defilermound', 'engineeringbay', 'evolutionchamber',
  'extractor', 'factory', 'fleetbeacon', 'forge', 'gateway', 'greaterspire', 'hatchery', 'hive', 'hydraliskden', 'infestedcc',
  'lair', 'machineshop', 'missileturret', 'nexus', 'nyduscanal', 'observatory', 'photoncannon', 'physicslab', 'pylon',
  'queensnest', 'refinery', 'roboticsfacility', 'roboticssupportbay', 'sciencefacility', 'shieldbattery', 'spawningpool', 'spire',
  'sporecolony', 'stargate', 'starport', 'sunkencolony', 'supplydepot', 'templararchives', 'ultraliskcavern',
]);

const WORKER_UNIT_KEYS = new Set(['scv', 'drone', 'probe']);
const SPELLCASTER_UNIT_KEYS = new Set([
  'ghost', 'medic', 'sciencevessel', 'queen', 'defiler', 'hightemplar', 'darkarchon', 'arbiter',
]);

const UNIT_TIER_MAP = {
  scv: 1, drone: 1, probe: 1, marine: 1, firebat: 1, medic: 1, vulture: 1, goliath: 2, ghost: 2, wraith: 2, valkyrie: 2,
  siegetank: 2, siegetanktankmode: 2, siegetankturrettankmode: 2, terransiegetanksiegemode: 2, siegetankturretsiegemode: 2,
  sciencevessel: 2, dropship: 2, battlecruiser: 3,
  zergling: 1, hydralisk: 1, lurker: 2, mutalisk: 2, scourge: 2, queen: 2, defiler: 2, guardian: 3, devourer: 3, ultralisk: 3,
  zealot: 1, dragoon: 1, darktemplar: 2, hightemplar: 2, reaver: 2, shuttle: 2, observer: 2, corsair: 2, scout: 2, archon: 3, arbiter: 3, carrier: 3,
};

const BUILDING_TIER_MAP = {
  commandcenter: 1, supplydepot: 1, barracks: 1, refinery: 1, engineeringbay: 1, missileturret: 1, bunker: 1, academy: 1,
  factory: 2, armory: 2, starport: 2, comsat: 2, machineshop: 2, controltower: 2, sciencefacility: 2, physicslab: 3, covertops: 3,
  nexus: 1, pylon: 1, gateway: 1, assimilator: 1, forge: 1, photoncannon: 1, cyberneticscore: 1, shieldbattery: 1,
  roboticsfacility: 2, citadelofadun: 2, stargate: 2, observatory: 2, roboticssupportbay: 2, templararchives: 2, fleetbeacon: 3, arbitertribunal: 3,
  hatchery: 1, spawningpool: 1, extractor: 1, evolutionchamber: 1, creepcolony: 1, hydraliskden: 1, lair: 2, sporecolony: 2, sunkencolony: 2,
  nyduscanal: 2, queensnest: 2, hive: 3, spire: 2, greaterspire: 3, ultraliskcavern: 3, defilermound: 3, infestedcc: 3,
};
const DEFENSIVE_BUILDING_KEYS = new Set([
  'photoncannon',
  'sporecolony',
  'sunkencolony',
  'creepcolony',
  'missileturret',
]);

const DEFAULT_SUMMARY_FILTERS = {
  nuke: false,
  drop: false,
  recall: false,
  becameRace: false,
  rush: false,
  scout: false,
};

const SUMMARY_TOPIC_PATTERNS = {
  nuke: /\bnuke|nuclear\b/i,
  drop: /\bdrop|dropship|shuttle\b/i,
  recall: /\brecall\b/i,
  becameRace: /\b(became|becomes)\s+(terran|zerg)\b|\bbecame_(terran|zerg)\b/i,
  rush: /\brush|all[\s-]?in|cheese\b/i,
  scout: /\bscouts?\b|\bscout\b/i,
};

const isPatternTruthy = (value) => {
  const normalized = String(value || '').trim().toLowerCase();
  return normalized === 'yes' || normalized === 'true';
};

const prettyPatternName = (patternName) => {
  const trimmed = String(patternName || '').trim();
  if (!trimmed) return '';
  if (/used\s+hotkey\s+groups/i.test(trimmed)) return 'Hotkeys';
  const splitUppercase = trimmed.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
  return splitUppercase
    .replace(/_/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase());
};

const patternIconForName = (patternName) => {
  const normalized = normalizeUnitName(patternName);
  if (normalized.includes('battlecruiser')) return getUnitIcon('battlecruiser');
  if (normalized.includes('carrier')) return getUnitIcon('carrier');
  // Build-order patterns: reuse the featuring-order icon registry.
  // normalizeUnitName strips punctuation, so the normalized prefix is
  // "buildorder" without a colon.
  if (normalized.startsWith('buildorder')) {
    const featureKey = BUILD_ORDER_PATTERN_TO_FEATURE_KEY[normalized];
    if (featureKey) {
      const entry = MAIN_GAME_FEATURING_ORDER.find((item) => item.key === featureKey);
      if (entry && entry.iconKey) return getUnitIcon(entry.iconKey);
    }
  }
  return getUnitIcon(patternName);
};

const minuteFromValue = (value) => {
  const trimmed = String(value || '').trim();
  const clockMatch = trimmed.match(/^(\d+):(\d{2})$/);
  if (clockMatch) return Number(clockMatch[1]);
  const asNumber = Number(trimmed);
  if (Number.isFinite(asNumber)) return Math.floor(asNumber / 60);
  return null;
};

const formatPatternPillText = (rawName, rawValue, isTruthy) => {
  if (isTruthy) {
    if (rawName.toLowerCase() === 'never researched') return 'Never Researched';
    // Build-order patterns: render the BO label cleanly ("9 pool") instead of
    // "Did Build Order: 9 Pool".
    if (rawName.toLowerCase().startsWith('build order:')) {
      return rawName.slice('Build Order:'.length).trim();
    }
    return `Did ${rawName}`;
  }
  const lowerName = rawName.toLowerCase();
  if (lowerName === 'hotkeys' || lowerName.includes('used hotkey groups')) {
    return rawValue ? `Hotkeys ${rawValue}` : 'Hotkeys';
  }
  if (lowerName.includes('made drops') || lowerName.includes('made recalls')) {
    const minute = minuteFromValue(rawValue);
    if (minute !== null) return `${rawName} at min ${minute}`;
  }
  if (lowerName.includes('threw nukes')) {
    const minute = minuteFromValue(rawValue);
    if (minute !== null) return `${rawName} at ${minute} mins`;
  }
  return `${rawName} at ${rawValue}`;
};

const shouldHidePatternFromSummaryPills = (pattern, trustGameEventsForDrops) => {
  const normalizedPatternName = normalizeUnitName(pattern?.pattern_name);
  if (normalizedPatternName === 'viewportmultitasking') return true;
  if (trustGameEventsForDrops && normalizedPatternName === 'madedrops') return true;
  // Retired pattern pills — detectors are unregistered but pre-existing rows
  // may still be in the DB; keep the UI clean.
  if (normalizedPatternName === 'fastexpa') return true;
  if (normalizedPatternName === 'gatethenforge') return true;
  if (normalizedPatternName === 'forgethengate') return true;
  if (normalizedPatternName === 'hatchbeforepool') return true;
  return false;
};

const filterSummaryPillPatterns = (patterns, trustGameEventsForDrops = false) => (
  (patterns || []).filter((pattern) => !shouldHidePatternFromSummaryPills(pattern, trustGameEventsForDrops))
);

const renderPatternPill = (pattern, keyPrefix, team) => {
  const rawName = prettyPatternName(pattern?.pattern_name);
  if (!rawName) return null;
  const normalizedPatternName = normalizeUnitName(pattern?.pattern_name);
  if (normalizedPatternName === 'neverresearched') {
    const rawValue = String(pattern?.value || '').trim();
    if (!rawValue || rawValue === '-' || rawValue.toLowerCase() === 'no' || rawValue.toLowerCase() === 'false') {
      return null;
    }
    const key = `${keyPrefix}-never-researched`;
    return (
      <span
        key={key}
        className="workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey"
        title="No Tech commands in this replay for this player (10+ minute games)."
      >
        <span>🚫 researches</span>
      </span>
    );
  }
  if (normalizedPatternName === 'neverupgraded') {
    const rawValue = String(pattern?.value || '').trim();
    if (!rawValue || rawValue === '-' || rawValue.toLowerCase() === 'no' || rawValue.toLowerCase() === 'false') {
      return null;
    }
    const key = `${keyPrefix}-never-upgraded`;
    return (
      <span
        key={key}
        className="workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey"
        title="No Upgrade commands in this replay for this player (10+ minute games)."
      >
        <span>🚫 upgrades</span>
      </span>
    );
  }
  if (normalizedPatternName === 'neverusedhotkeys') {
    const rawValue = String(pattern?.value || '').trim();
    if (!rawValue || rawValue === '-' || rawValue.toLowerCase() === 'no' || rawValue.toLowerCase() === 'false') {
      return null;
    }
    const key = `${keyPrefix}-never-hotkeys`;
    return (
      <span
        key={key}
        className="workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey"
        title="No hotkey-group commands in this replay (same 7+ minute gate as the detector)."
      >
        <span>🚫 hotkeys</span>
      </span>
    );
  }
  const rawValue = String(pattern?.value || '').trim();
  if (normalizedPatternName === 'becameterran' || normalizedPatternName === 'becamezerg') {
    if (!rawValue || rawValue === '-' || rawValue.toLowerCase() === 'no' || rawValue.toLowerCase() === 'false') {
      return null;
    }
    const minute = minuteFromValue(rawValue);
    const raceWord = normalizedPatternName === 'becameterran' ? 'Terran' : 'Zerg';
    const da = getUnitIcon('darkarchon');
    const key = `${keyPrefix}-became-${normalizedPatternName}-${pattern?.value}`;
    const label = minute !== null ? `${raceWord} at ${minute} mins` : raceWord;
    return (
      <span key={key} className="workflow-pattern-pill workflow-pattern-pill-strong" title={String(pattern?.pattern_name || '').trim()}>
        {da ? <img src={da} alt="" className="workflow-pattern-icon" /> : null}
        <span>{label}</span>
      </span>
    );
  }
  if (!rawValue || rawValue === '-' || rawValue.toLowerCase() === 'no' || rawValue.toLowerCase() === 'false') {
    return null;
  }
  const isTruthy = isPatternTruthy(pattern?.value);
  let icon = patternIconForName(pattern?.pattern_name);
  const text = formatPatternPillText(rawName, rawValue, isTruthy);
  let content = <span>{text}</span>;
  if (isTruthy) {
    if (normalizedPatternName === 'quickfactory') {
      icon = null;
      content = (
        <span className="workflow-pattern-pill-inline">
          <span>Quick</span>
          {getUnitIcon('factory') ? <img src={getUnitIcon('factory')} alt="Factory" className="workflow-pattern-icon" /> : null}
        </span>
      );
    } else if (normalizedPatternName === 'carriers' || normalizedPatternName === 'battlecruisers') {
      content = null;
    } else if (normalizedPatternName.startsWith('buildorder')) {
      // BO pills: icon + short name ("9 Pool"); text already stripped of the
      // "Build Order:" prefix by formatPatternPillText.
    } else if (icon) {
      content = <span>Did</span>;
    }
  }
  const key = `${keyPrefix}-${team ? `team-${team}-` : ''}${pattern?.pattern_name}-${pattern?.value}`;
  return (
    <span key={key} className={`workflow-pattern-pill${isTruthy ? ' workflow-pattern-pill-strong' : ''}`}>
      {team !== undefined ? <span className="team-dot" style={{ backgroundColor: getTeamColor(team) }}></span> : null}
      {icon ? <img src={icon} alt={rawName} className="workflow-pattern-icon" /> : null}
      {content}
    </span>
  );
};

const formatSigned = (value) => {
  const n = Number(value) || 0;
  if (n > 0) return `+${n.toFixed(2)}`;
  return n.toFixed(2);
};

const PLAYER_OUTLIER_HELP = [
  'Baselines are computed against human, non-observer players of the same primary race only.',
  'For Protoss players, non-Protoss techs/upgrades and non-Protoss cast orders are excluded to avoid mind-control leakage.',
  'Orders use share of total order instances. Build, train, morph, tech, and upgrade items use the share of same-race games where the item appears at least once.',
  'An item appears if it passes either threshold: "Rare signature" (TF-IDF) or "Much more frequent than peers" (ratio vs baseline).',
].join(' ');

const PLAYER_INSIGHT_TYPES = {
  apm: 'apm',
  firstUnitDelay: 'first-unit-delay',
  unitProductionCadence: 'unit-production-cadence',
  viewportSwitchRate: 'viewport-switch-rate',
};

const VIEWPORT_SWITCH_RATE_CONFIG = {
  title: 'Viewport Switch Rate',
  playerField: 'average_viewport_switch_rate',
  gameField: 'viewport_switch_rate',
  axisLabel: 'Average switches per minute',
  overlayValueLabel: 'switches/min',
  valueFormatter: (value) => `${Number(value || 0).toFixed(2)} switches/min`,
  summaryFormatter: (value) => `${Number(value || 0).toFixed(2)}`,
  interpretation: 'Higher means the player more often jumps outside the prior viewport-sized area during the mid-game window.',
};

const LOW_USAGE_THRESHOLD = 0.1;

const HelpTooltip = ({ text, label }) => (
  <span className="workflow-help-wrap" aria-label={label || 'Explanation'}>
    <span className="workflow-metric-help">ⓘ</span>
    <span className="workflow-help-bubble">{text}</span>
  </span>
);

const outlierQualifierClassName = (qualifier) => {
  const normalized = String(qualifier || '').toLowerCase();
  if (normalized.includes('rare signature')) return 'workflow-outlier-pill workflow-outlier-pill-rare';
  if (normalized.includes('much more frequent than peers')) return 'workflow-outlier-pill workflow-outlier-pill-frequent';
  return 'workflow-outlier-pill';
};

const insightScoreColor = (percentile) => {
  const clamped = Math.max(0, Math.min(100, Number(percentile) || 0));
  const hue = (clamped / 100) * 120;
  return `hsl(${hue}, 78%, 52%)`;
};

const insightScoreLabel = (percentile) => {
  const score = Number(percentile) || 0;
  if (score >= 90) return 'Elite';
  if (score >= 75) return 'Strong';
  if (score >= 55) return 'Solid';
  if (score >= 35) return 'Mixed';
  return 'Needs work';
};

const insightSummaryLabel = (percentile) => {
  const score = Math.max(0, Math.min(100, Number(percentile) || 0));
  if (score >= 99) return 'Best in sample';
  if (score >= 80) return `Top ${Math.max(1, Math.round(100 - score))}%`;
  return `Better than ${Math.round(score)}%`;
};

const playerInsightDestinationTab = (insightType) => {
  switch (String(insightType || '').trim()) {
    case PLAYER_INSIGHT_TYPES.apm:
      return 'apm-histogram';
    case PLAYER_INSIGHT_TYPES.firstUnitDelay:
      return 'first-unit-delay';
    case PLAYER_INSIGHT_TYPES.unitProductionCadence:
      return 'unit-production-cadence';
    case PLAYER_INSIGHT_TYPES.viewportSwitchRate:
      return 'viewport-multitasking';
    default:
      return 'summary';
  }
};

const prettyMetricValue = (metric) => {
  const value = Number(metric?.player_value) || 0;
  if (String(metric?.metric || '').toLowerCase().includes('%')) {
    if (Math.abs(value) <= 1) return formatPercent(value);
    return `${value.toFixed(1)}%`;
  }
  if (String(metric?.metric || '').toLowerCase().includes('seconds')) {
    return formatDuration(value);
  }
  return value.toFixed(2);
};

const TEAM_COLORS = ['#60A5FA', '#F472B6', '#34D399', '#FBBF24', '#A78BFA', '#22D3EE', '#FB7185', '#4ADE80'];

const getTeamColor = (team) => {
  const n = Number(team) || 0;
  return TEAM_COLORS[Math.abs(n) % TEAM_COLORS.length];
};

const teamColorRgba = (team, alpha = 0.14) => {
  const hex = getTeamColor(team).replace('#', '');
  const expanded = hex.length === 3 ? hex.split('').map((c) => `${c}${c}`).join('') : hex;
  const r = parseInt(expanded.slice(0, 2), 16);
  const g = parseInt(expanded.slice(2, 4), 16);
  const b = parseInt(expanded.slice(4, 6), 16);
  return `rgba(${Number.isNaN(r) ? 96 : r}, ${Number.isNaN(g) ? 165 : g}, ${Number.isNaN(b) ? 250 : b}, ${alpha})`;
};

const MAIN_GAMES_PAGE_SIZE = 30;
const MAIN_PLAYERS_PAGE_SIZE = 30;

const toggleFilterValue = (values, value) => {
  const normalized = String(value || '').trim();
  if (!normalized) return values;
  if (values.includes(normalized)) {
    return values.filter((item) => item !== normalized);
  }
  return [...values, normalized];
};

const teamGroupsFromPlayers = (players) => {
  const groups = [];
  const byTeam = new Map();
  (players || []).forEach((player) => {
    const team = Number(player?.team || 0);
    if (!byTeam.has(team)) {
      byTeam.set(team, []);
      groups.push(byTeam.get(team));
    }
    byTeam.get(team).push(player);
  });
  return groups;
};

const playersHaveDistinctTeams = (players) => new Set((players || []).map((p) => Number(p?.team || 0))).size > 1;

const mergeIngestLogEntries = (entries, event) => {
  if (!event || !event.message) {
    return entries;
  }

  if (event.append && entries.length > 0 && entries[entries.length - 1].append) {
    const next = [...entries];
    const last = next[next.length - 1];
    next[next.length - 1] = {
      ...last,
      level: event.level || last.level,
      message: `${last.message}${event.message}`,
      append: true,
    };
    return next;
  }

  return [...entries, {
    level: event.level || 'info',
    message: event.message,
    append: Boolean(event.append),
  }];
};

const hydrateIngestLogEntries = (events = []) => (
  (events || []).reduce((entries, event) => mergeIngestLogEntries(entries, event), [])
);

const sleep = (ms) => new Promise((resolve) => window.setTimeout(resolve, ms));

function App() {
  const storedAutoIngest = getStoredAutoIngestSettings();
  const initialMainRoute = useMemo(
    () => parseMainRouteSearch(typeof window !== 'undefined' ? window.location.search : ''),
    [],
  );
  const [currentDashboardUrl, setCurrentDashboardUrl] = useState(() => (
    initialMainRoute.view === 'dashboards' && initialMainRoute.dash ? initialMainRoute.dash : 'default'
  ));
  const [dashboard, setDashboard] = useState(null);
  const [dashboards, setDashboards] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showDashboardManager, setShowDashboardManager] = useState(false);
  const [showEditDashboard, setShowEditDashboard] = useState(false);
  const [showGlobalReplayFilter, setShowGlobalReplayFilter] = useState(false);
  const [newWidgetPrompt, setNewWidgetPrompt] = useState('');
  const [creatingWidget, setCreatingWidget] = useState(false);
  const [variableValues, setVariableValues] = useState({});
  const [openaiEnabled, setOpenaiEnabled] = useState(false);
  const [editingWidget, setEditingWidget] = useState(null);
  const [replayCount, setReplayCount] = useState(null);
  const [globalReplayFilterConfig, setGlobalReplayFilterConfig] = useState(null);
  const [globalReplayFilterOptions, setGlobalReplayFilterOptions] = useState({
    top_maps: [],
    other_maps: [],
    top_players: [],
    other_players: [],
  });
  const [globalReplayFilterSaving, setGlobalReplayFilterSaving] = useState(false);
  const [globalReplayFilterError, setGlobalReplayFilterError] = useState('');
  const [showIngestPanel, setShowIngestPanel] = useState(false);
  const [ingestMessage, setIngestMessage] = useState('');
  const [ingestStatus, setIngestStatus] = useState('idle');
  const [ingestLogs, setIngestLogs] = useState([]);
  const [ingestInputDir, setIngestInputDir] = useState('');
  const [savedIngestInputDir, setSavedIngestInputDir] = useState('');
  const [ingestSettingsLoading, setIngestSettingsLoading] = useState(false);
  const [ingestSettingsSaving, setIngestSettingsSaving] = useState(false);
  const [ingestSocketState, setIngestSocketState] = useState('closed');
  const [aliases, setAliases] = useState([]);
  const [aliasesLoading, setAliasesLoading] = useState(false);
  const [aliasesMessage, setAliasesMessage] = useState('');
  const [aliasesMessageIsError, setAliasesMessageIsError] = useState(false);
  const [aliasSaving, setAliasSaving] = useState(false);
  const [aliasSources, setAliasSources] = useState(['you', 'manual', 'imported']);
  const [aliasEditOriginal, setAliasEditOriginal] = useState(null);
  const [aliasForm, setAliasForm] = useState({
    canonical_alias: '',
    battle_tag: '',
    aurora_id: '',
  });
  const [autoIngestNotice, setAutoIngestNotice] = useState('');
  const [ingestForm, setIngestForm] = useState({
    watch: false,
    stopAfterN: 50,
    clean: false,
    autoIngestEnabled: storedAutoIngest.enabled,
  });
  const autoIngestInFlight = useRef(false);
  const ingestSocketRef = useRef(null);
  const autoIngestNoticeTimerRef = useRef(null);
  const [activeView, setActiveView] = useState(() => initialMainRoute.view);
  const [mainGames, setMainGames] = useState([]);
  const [mainGamesLoading, setMainGamesLoading] = useState(false);
  const [mainGamesPage, setMainGamesPage] = useState(1);
  const [mainGamesTotal, setMainGamesTotal] = useState(0);
  const [mainGamesFilterOptions, setMainGamesFilterOptions] = useState({
    players: [],
    maps: [],
    durations: [],
    featuring: [],
  });
  const [mainGamesFilters, setMainGamesFilters] = useState({
    player: [],
    map: [],
    duration: [],
    featuring: [],
  });
  const [mainGameDetailLoading, setMainGameDetailLoading] = useState(false);
  const [mainPlayerLoading, setMainPlayerLoading] = useState(false);
  const [selectedReplayId, setSelectedReplayId] = useState(() => initialMainRoute.replayId);
  const [selectedPlayerKey, setSelectedPlayerKey] = useState(() => initialMainRoute.playerKey || '');
  const [mainGame, setMainGame] = useState(null);
  const [mainGameTab, setMainGameTab] = useState(() => initialMainRoute.gameTab);
  const [mainEventsPlayerEnabledById, setMainEventsPlayerEnabledById] = useState({});
  const [mainSelectedGameEventKey, setMainSelectedGameEventKey] = useState('');
  const [mainGameSeeLoading, setMainGameSeeLoading] = useState(false);
  const [mainGameSeeNotice, setMainGameSeeNotice] = useState('');
  const [mainGameSeeNoticeError, setMainGameSeeNoticeError] = useState(false);
  const mainGameSeeNoticeTimerRef = useRef(null);
  const suppressUrlSyncRef = useRef(false);
  const openMainGameRef = useRef(null);
  const openMainPlayerRef = useRef(null);
  const loadDashboardRef = useRef(null);
  const [mainPlayer, setMainPlayer] = useState(null);
  const [mainPlayerRecentGames, setMainPlayerRecentGames] = useState([]);
  const [mainPlayerRecentGamesLoading, setMainPlayerRecentGamesLoading] = useState(false);
  const [mainPlayerRecentGamesError, setMainPlayerRecentGamesError] = useState('');
  const [mainPlayerChatSummary, setMainPlayerChatSummary] = useState(null);
  const [mainPlayerChatSummaryLoading, setMainPlayerChatSummaryLoading] = useState(false);
  const [mainPlayerChatSummaryError, setMainPlayerChatSummaryError] = useState('');
  const [mainPlayerMetrics, setMainPlayerMetrics] = useState(null);
  const [mainPlayerMetricsLoading, setMainPlayerMetricsLoading] = useState(false);
  const [mainPlayerMetricsError, setMainPlayerMetricsError] = useState('');
  const [mainPlayerOutliers, setMainPlayerOutliers] = useState(null);
  const [mainPlayerOutliersLoading, setMainPlayerOutliersLoading] = useState(false);
  const [mainPlayerOutliersError, setMainPlayerOutliersError] = useState('');
  const [mainPlayers, setMainPlayers] = useState([]);
  const [mainPlayersLoading, setMainPlayersLoading] = useState(false);
  const [mainPlayersPage, setMainPlayersPage] = useState(1);
  const [mainPlayersTotal, setMainPlayersTotal] = useState(0);
  const [mainPlayersSortBy, setMainPlayersSortBy] = useState('games');
  const [mainPlayersSortDir, setMainPlayersSortDir] = useState('desc');
  const [mainPlayersTab, setMainPlayersTab] = useState(() => initialMainRoute.playersTab);
  const [mainPlayersFilterOptions, setMainPlayersFilterOptions] = useState({
    races: [],
    last_played: [],
  });
  const [mainPlayersFilters, setMainPlayersFilters] = useState({
    name: '',
    onlyFivePlus: false,
    lastPlayed: [],
  });
  const [mainPlayersApmHistogram, setMainPlayersApmHistogram] = useState(null);
  const [mainPlayersApmHistogramLoading, setMainPlayersApmHistogramLoading] = useState(false);
  const [mainPlayersApmHistogramError, setMainPlayersApmHistogramError] = useState('');
  const [mainPlayersApmMinGames, setMainPlayersApmMinGames] = useState(5);
  const [mainPlayersDelayHistogram, setMainPlayersDelayHistogram] = useState(null);
  const [mainPlayersDelayHistogramLoading, setMainPlayersDelayHistogramLoading] = useState(false);
  const [mainPlayersDelayHistogramError, setMainPlayersDelayHistogramError] = useState('');
  const [mainPlayersDelayMinSamples, setMainPlayersDelayMinSamples] = useState(5);
  const [mainPlayersDelaySelectedCases, setMainPlayersDelaySelectedCases] = useState(['all']);
  const [mainPlayersCadenceHistogram, setMainPlayersCadenceHistogram] = useState(null);
  const [mainPlayersCadenceHistogramLoading, setMainPlayersCadenceHistogramLoading] = useState(false);
  const [mainPlayersCadenceHistogramError, setMainPlayersCadenceHistogramError] = useState('');
  const [mainPlayersCadenceMinGames, setMainPlayersCadenceMinGames] = useState(4);
  const [mainPlayersViewportHistogram, setMainPlayersViewportHistogram] = useState(null);
  const [mainPlayersViewportHistogramLoading, setMainPlayersViewportHistogramLoading] = useState(false);
  const [mainPlayersViewportHistogramError, setMainPlayersViewportHistogramError] = useState('');
  const [mainPlayersViewportMinGames, setMainPlayersViewportMinGames] = useState(4);
  const [mainPlayerApmInsight, setMainPlayerApmInsight] = useState(null);
  const [mainPlayerApmInsightLoading, setMainPlayerApmInsightLoading] = useState(false);
  const [mainPlayerApmInsightError, setMainPlayerApmInsightError] = useState('');
  const [mainPlayerDelayInsight, setMainPlayerDelayInsight] = useState(null);
  const [mainPlayerDelayInsightLoading, setMainPlayerDelayInsightLoading] = useState(false);
  const [mainPlayerDelayInsightError, setMainPlayerDelayInsightError] = useState('');
  const [mainPlayerCadenceInsight, setMainPlayerCadenceInsight] = useState(null);
  const [mainPlayerCadenceInsightLoading, setMainPlayerCadenceInsightLoading] = useState(false);
  const [mainPlayerCadenceInsightError, setMainPlayerCadenceInsightError] = useState('');
  const [mainPlayerViewportInsight, setMainPlayerViewportInsight] = useState(null);
  const [mainPlayerViewportInsightLoading, setMainPlayerViewportInsightLoading] = useState(false);
  const [mainPlayerViewportInsightError, setMainPlayerViewportInsightError] = useState('');
  const [mainQuestion, setMainQuestion] = useState('');
  const [mainAnswer, setMainAnswer] = useState(null);
  const [mainAskLoading, setMainAskLoading] = useState(false);
  const [topPlayerColors, setTopPlayerColors] = useState({});
  const [mainSummaryFilters, setMainSummaryFilters] = useState(DEFAULT_SUMMARY_FILTERS);
  const [mainProductionTab, setMainProductionTab] = useState('units');
  const [mainUnitFilterMode, setMainUnitFilterMode] = useState('all');
  const [mainUnitNameFilter, setMainUnitNameFilter] = useState('');
  const [mainBuildingFilterMode, setMainBuildingFilterMode] = useState('all');
  const [mainBuildingNameFilter, setMainBuildingNameFilter] = useState('');
  const [mainTimingCategory, setMainTimingCategory] = useState('expansion');
  const [mainHpUpgradeFilters, setMainHpUpgradeFilters] = useState({
    terran: DEFAULT_HP_UPGRADE_BY_RACE.terran,
    zerg: DEFAULT_HP_UPGRADE_BY_RACE.zerg,
    protoss: DEFAULT_HP_UPGRADE_BY_RACE.protoss,
  });

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

  const loadGlobalReplayFilterConfig = async () => {
    const data = await api.getGlobalReplayFilter();
    setGlobalReplayFilterConfig(data);
    return data;
  };

  const loadGlobalReplayFilterOptions = async () => {
    const data = await api.getGlobalReplayFilterOptions();
    setGlobalReplayFilterOptions({
      top_maps: data?.top_maps || [],
      other_maps: data?.other_maps || [],
      top_players: data?.top_players || [],
      other_players: data?.other_players || [],
    });
    return data;
  };

  const loadMainGames = async ({ page = mainGamesPage, filters = mainGamesFilters } = {}) => {
    try {
      setMainGamesLoading(true);
      const safePage = Math.max(1, Number(page) || 1);
      const offset = (safePage - 1) * MAIN_GAMES_PAGE_SIZE;
      const data = await api.listGames({
        limit: MAIN_GAMES_PAGE_SIZE,
        offset,
        filters,
      });
      const items = data?.items || [];
      setMainGames(items);
      setMainGamesTotal(Number(data?.total) || 0);
      if (data?.filter_options) {
        setMainGamesFilterOptions(data.filter_options);
      }
      if (!selectedReplayId && items.length > 0) {
        setSelectedReplayId(items[0].replay_id);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setMainGamesLoading(false);
    }
  };

  const loadMainPlayers = async ({
    page = mainPlayersPage,
    filters = mainPlayersFilters,
    sortBy = mainPlayersSortBy,
    sortDir = mainPlayersSortDir,
  } = {}) => {
    try {
      setMainPlayersLoading(true);
      const safePage = Math.max(1, Number(page) || 1);
      const offset = (safePage - 1) * MAIN_PLAYERS_PAGE_SIZE;
      const data = await api.listPlayers({
        limit: MAIN_PLAYERS_PAGE_SIZE,
        offset,
        sortBy,
        sortDir,
        filters,
      });
      setMainPlayers(data?.items || []);
      setMainPlayersTotal(Number(data?.total) || 0);
      if (data?.filter_options) {
        setMainPlayersFilterOptions(data.filter_options);
      }
    } catch (err) {
      setError(err.message);
    } finally {
      setMainPlayersLoading(false);
    }
  };

  const loadMainPlayersApmHistogram = async () => {
    try {
      setMainPlayersApmHistogramLoading(true);
      setMainPlayersApmHistogramError('');
      const data = await api.getPlayersApmHistogram();
      setMainPlayersApmHistogram(data);
    } catch (err) {
      setMainPlayersApmHistogramError(err.message || 'Failed to load players histogram');
      setMainPlayersApmHistogram(null);
    } finally {
      setMainPlayersApmHistogramLoading(false);
    }
  };

  const loadMainPlayersDelayHistogram = async () => {
    try {
      setMainPlayersDelayHistogramLoading(true);
      setMainPlayersDelayHistogramError('');
      const data = await api.getPlayersFirstUnitDelay();
      setMainPlayersDelayHistogram(data);
      setMainPlayersDelaySelectedCases(['all']);
    } catch (err) {
      setMainPlayersDelayHistogramError(err.message || 'Failed to load players delay');
      setMainPlayersDelayHistogram(null);
      setMainPlayersDelaySelectedCases(['all']);
    } finally {
      setMainPlayersDelayHistogramLoading(false);
    }
  };

  const loadMainPlayersCadenceHistogram = async () => {
    try {
      setMainPlayersCadenceHistogramLoading(true);
      setMainPlayersCadenceHistogramError('');
      const data = await api.getPlayersUnitProductionCadence({ filter: 'strict', minGames: 4, limit: 0 });
      setMainPlayersCadenceHistogram(data);
    } catch (err) {
      setMainPlayersCadenceHistogramError(err.message || 'Failed to load players unit production cadence');
      setMainPlayersCadenceHistogram(null);
    } finally {
      setMainPlayersCadenceHistogramLoading(false);
    }
  };

  const loadMainPlayersViewportHistogram = async () => {
    try {
      setMainPlayersViewportHistogramLoading(true);
      setMainPlayersViewportHistogramError('');
      const data = await api.getPlayersViewportMultitasking();
      setMainPlayersViewportHistogram(data);
    } catch (err) {
      setMainPlayersViewportHistogramError(err.message || 'Failed to load players viewport multitasking');
      setMainPlayersViewportHistogram(null);
    } finally {
      setMainPlayersViewportHistogramLoading(false);
    }
  };

  const loadTopPlayerColors = async () => {
    try {
      const data = await api.getPlayerColors();
      setTopPlayerColors(data?.player_colors || {});
    } catch (err) {
      console.error('Failed to load top player colors:', err);
    }
  };

  const openMainGame = async (replayId, options = {}) => {
    try {
      setMainGameDetailLoading(true);
      setError(null);
      if (mainGameSeeNoticeTimerRef.current) {
        window.clearTimeout(mainGameSeeNoticeTimerRef.current);
        mainGameSeeNoticeTimerRef.current = null;
      }
      setMainGameSeeNotice('');
      setMainGameSeeNoticeError(false);
      const data = await api.getGame(replayId);
      setMainGame(data);
      const wantTab = options.initialGameTab;
      let nextTab = wantTab && MAIN_GAME_TABS.includes(String(wantTab).trim().toLowerCase())
        ? String(wantTab).trim().toLowerCase()
        : 'summary';
      // Build Orders tab is hidden when no BOs were detected; don't leave the
      // user stranded on an invisible tab.
      const hasBuildOrders = Array.isArray(data?.build_orders) && data.build_orders.length > 0;
      if (nextTab === 'build-orders' && !hasBuildOrders) {
        nextTab = 'summary';
      }
      setMainGameTab(nextTab);
      setMainEventsPlayerEnabledById(
        Object.fromEntries((data.players || []).map((p) => [String(p.player_id), true])),
      );
      setMainSelectedGameEventKey('');
      setSelectedReplayId(replayId);
      setMainAnswer(null);
      setMainQuestion('');
      setMainSummaryFilters(DEFAULT_SUMMARY_FILTERS);
      setMainProductionTab('units');
      setMainUnitFilterMode('all');
      setMainUnitNameFilter('');
      setMainBuildingFilterMode('all');
      setMainBuildingNameFilter('');
      setMainTimingCategory('expansion');
      setMainHpUpgradeFilters({
        terran: DEFAULT_HP_UPGRADE_BY_RACE.terran,
        zerg: DEFAULT_HP_UPGRADE_BY_RACE.zerg,
        protoss: DEFAULT_HP_UPGRADE_BY_RACE.protoss,
      });
      navigateMainView('game');
    } catch (err) {
      setError(err.message);
    } finally {
      setMainGameDetailLoading(false);
    }
  };

  const copyMainGameToWatchMe = async () => {
    const replayId = mainGame?.replay_id;
    if (!replayId || mainGameSeeLoading) return;
    if (mainGameSeeNoticeTimerRef.current) {
      window.clearTimeout(mainGameSeeNoticeTimerRef.current);
      mainGameSeeNoticeTimerRef.current = null;
    }
    try {
      setMainGameSeeLoading(true);
      setMainGameSeeNotice('');
      setMainGameSeeNoticeError(false);
      await api.seeGame(replayId);
      setMainGameSeeNotice('Copied to _watch_me.rep in your ingest folder.');
      mainGameSeeNoticeTimerRef.current = window.setTimeout(() => {
        setMainGameSeeNotice('');
        mainGameSeeNoticeTimerRef.current = null;
      }, 5000);
    } catch (err) {
      setMainGameSeeNotice(err.message || 'Failed to copy replay');
      setMainGameSeeNoticeError(true);
    } finally {
      setMainGameSeeLoading(false);
    }
  };

  const loadMainPlayerRecentGames = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerRecentGamesLoading(true);
      setMainPlayerRecentGamesError('');
      const data = await api.getPlayerRecentGames(normalizedPlayerKey);
      setMainPlayerRecentGames(data?.recent_games || []);
    } catch (err) {
      setMainPlayerRecentGamesError(err.message || 'Failed to load recent games');
      setMainPlayerRecentGames([]);
    } finally {
      setMainPlayerRecentGamesLoading(false);
    }
  };

  const loadMainPlayerChatSummary = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerChatSummaryLoading(true);
      setMainPlayerChatSummaryError('');
      const data = await api.getPlayerChatSummary(normalizedPlayerKey);
      setMainPlayerChatSummary(data?.chat_summary || null);
    } catch (err) {
      setMainPlayerChatSummaryError(err.message || 'Failed to load chat summary');
      setMainPlayerChatSummary(null);
    } finally {
      setMainPlayerChatSummaryLoading(false);
    }
  };

  const loadMainPlayerMetrics = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerMetricsLoading(true);
      setMainPlayerMetricsError('');
      const metricsData = await api.getPlayerMetrics(normalizedPlayerKey);
      setMainPlayerMetrics(metricsData);
    } catch (err) {
      setMainPlayerMetricsError(err.message || 'Failed to load metrics');
      setMainPlayerMetrics(null);
    } finally {
      setMainPlayerMetricsLoading(false);
    }
  };

  const loadMainPlayerApmInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerApmInsightLoading(true);
      setMainPlayerApmInsightError('');
      const insightData = await api.getPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.apm);
      setMainPlayerApmInsight(insightData);
    } catch (err) {
      setMainPlayerApmInsightError(err.message || 'Failed to load APM insight');
      setMainPlayerApmInsight(null);
    } finally {
      setMainPlayerApmInsightLoading(false);
    }
  };

  const loadMainPlayerOutliers = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerOutliersLoading(true);
      setMainPlayerOutliersError('');
      const outlierData = await api.getPlayerOutliers(normalizedPlayerKey);
      setMainPlayerOutliers(outlierData);
    } catch (err) {
      setMainPlayerOutliersError(err.message || 'Failed to load outliers');
      setMainPlayerOutliers(null);
    } finally {
      setMainPlayerOutliersLoading(false);
    }
  };

  const loadMainPlayerDelayInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerDelayInsightLoading(true);
      setMainPlayerDelayInsightError('');
      const delayData = await api.getPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.firstUnitDelay);
      setMainPlayerDelayInsight(delayData);
    } catch (err) {
      setMainPlayerDelayInsightError(err.message || 'Failed to load delay insight');
      setMainPlayerDelayInsight(null);
    } finally {
      setMainPlayerDelayInsightLoading(false);
    }
  };

  const loadMainPlayerCadenceInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerCadenceInsightLoading(true);
      setMainPlayerCadenceInsightError('');
      const cadenceData = await api.getPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.unitProductionCadence);
      setMainPlayerCadenceInsight(cadenceData);
    } catch (err) {
      setMainPlayerCadenceInsightError(err.message || 'Failed to load cadence insight');
      setMainPlayerCadenceInsight(null);
    } finally {
      setMainPlayerCadenceInsightLoading(false);
    }
  };

  const loadMainPlayerViewportInsight = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerViewportInsightLoading(true);
      setMainPlayerViewportInsightError('');
      const viewportData = await api.getPlayerInsight(normalizedPlayerKey, PLAYER_INSIGHT_TYPES.viewportSwitchRate);
      setMainPlayerViewportInsight(viewportData);
    } catch (err) {
      setMainPlayerViewportInsightError(err.message || 'Failed to load viewport insight');
      setMainPlayerViewportInsight(null);
    } finally {
      setMainPlayerViewportInsightLoading(false);
    }
  };

  const openMainPlayer = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    try {
      setMainPlayerLoading(true);
      setError(null);
      const data = await api.getPlayer(playerKey);
      setMainPlayer(data);
      setMainPlayerRecentGames([]);
      setMainPlayerRecentGamesError('');
      setMainPlayerRecentGamesLoading(false);
      setMainPlayerChatSummary(null);
      setMainPlayerChatSummaryError('');
      setMainPlayerChatSummaryLoading(false);
      setMainPlayerMetrics(null);
      setMainPlayerMetricsError('');
      setMainPlayerMetricsLoading(false);
      setMainPlayerOutliers(null);
      setMainPlayerOutliersError('');
      setMainPlayerOutliersLoading(false);
      setMainPlayerApmInsight(null);
      setMainPlayerApmInsightError('');
      setMainPlayerApmInsightLoading(false);
      setMainPlayerDelayInsight(null);
      setMainPlayerDelayInsightError('');
      setMainPlayerDelayInsightLoading(false);
      setMainPlayerCadenceInsight(null);
      setMainPlayerCadenceInsightError('');
      setMainPlayerCadenceInsightLoading(false);
      setMainPlayerViewportInsight(null);
      setMainPlayerViewportInsightError('');
      setMainPlayerViewportInsightLoading(false);
      setSelectedPlayerKey(normalizedPlayerKey);
      setMainAnswer(null);
      setMainQuestion('');
      navigateMainView('player');
      loadMainPlayerRecentGames(normalizedPlayerKey);
      loadMainPlayerChatSummary(normalizedPlayerKey);
      loadMainPlayerMetrics(normalizedPlayerKey);
      loadMainPlayerOutliers(normalizedPlayerKey);
      loadMainPlayerApmInsight(normalizedPlayerKey);
      loadMainPlayerDelayInsight(normalizedPlayerKey);
      loadMainPlayerCadenceInsight(normalizedPlayerKey);
      loadMainPlayerViewportInsight(normalizedPlayerKey);
    } catch (err) {
      setError(err.message);
    } finally {
      setMainPlayerLoading(false);
    }
  };

  const loadIngestSettings = async () => {
    try {
      setIngestSettingsLoading(true);
      const data = await api.getIngestSettings();
      const nextInputDir = String(data?.input_dir || '');
      setIngestInputDir(nextInputDir);
      setSavedIngestInputDir(nextInputDir);
      return nextInputDir;
    } catch (err) {
      setIngestMessage(err.message || 'Failed to load ingest settings.');
      return '';
    } finally {
      setIngestSettingsLoading(false);
    }
  };

  const persistIngestInputDir = async (inputDir = ingestInputDir) => {
    const trimmedInputDir = String(inputDir || '').trim();
    if (!trimmedInputDir) {
      throw new Error('Replay folder is required');
    }

    setIngestSettingsSaving(true);
    try {
      const data = await api.updateIngestSettings({ input_dir: trimmedInputDir });
      const nextInputDir = String(data?.input_dir || trimmedInputDir);
      setIngestInputDir(nextInputDir);
      setSavedIngestInputDir(nextInputDir);
      return nextInputDir;
    } finally {
      setIngestSettingsSaving(false);
    }
  };

  const normalizeAliasBattleTag = (value) => String(value || '').trim().toLowerCase();

  const loadAliases = async () => {
    try {
      setAliasesLoading(true);
      const data = await api.listAliases();
      setAliases(Array.isArray(data?.aliases) ? data.aliases : []);
    } catch (err) {
      setAliasesMessage(err.message || 'Failed to load aliases');
      setAliasesMessageIsError(true);
    } finally {
      setAliasesLoading(false);
    }
  };

  const handleAliasSave = async () => {
    const canonicalAlias = String(aliasForm.canonical_alias || '').trim();
    const battleTag = String(aliasForm.battle_tag || '').trim();
    if (!canonicalAlias || !battleTag) {
      setAliasesMessage('Alias and name in replay are required.');
      setAliasesMessageIsError(true);
      return;
    }
    if (canonicalAlias.trim().toLowerCase() === battleTag.trim().toLowerCase()) {
      setAliasesMessage('Alias must differ from name in replay.');
      setAliasesMessageIsError(true);
      return;
    }
    let source = 'manual';
    if (aliasEditOriginal) {
      if (aliasEditOriginal.source === 'you') {
        source = 'manual';
      } else {
        source = aliasEditOriginal.source;
      }
    }
    const wasEditing = Boolean(aliasEditOriginal);
    try {
      setAliasSaving(true);
      setAliasesMessage('');
      setAliasesMessageIsError(false);
      const auroraIdRaw = String(aliasForm.aurora_id || '').trim();
      await api.upsertAliasEntry({
        canonical_alias: canonicalAlias,
        battle_tag: battleTag,
        source,
        aurora_id: auroraIdRaw ? Number(auroraIdRaw) : undefined,
      });
      if (aliasEditOriginal) {
        const prevNorm = normalizeAliasBattleTag(aliasEditOriginal.battle_tag_normalized);
        const tripleChanged =
          normalizeAliasBattleTag(battleTag) !== prevNorm ||
          canonicalAlias !== aliasEditOriginal.canonical_alias ||
          source !== aliasEditOriginal.source;
        if (tripleChanged && aliasEditOriginal.id != null) {
          await api.deleteAliasEntry(aliasEditOriginal.id);
        }
      }
      setAliasForm({ canonical_alias: '', battle_tag: '', aurora_id: '' });
      setAliasEditOriginal(null);
      setAliasesMessage(wasEditing ? 'Alias updated.' : 'Alias saved.');
      await loadAliases();
    } catch (err) {
      setAliasesMessage(err.message || 'Failed to save alias');
      setAliasesMessageIsError(true);
    } finally {
      setAliasSaving(false);
    }
  };

  const handleAliasEdit = (row) => {
    setAliasesMessage('');
    setAliasesMessageIsError(false);
    setAliasEditOriginal({
      id: row.id,
      canonical_alias: row.canonical_alias,
      battle_tag_normalized: row.battle_tag_normalized,
      battle_tag_raw: row.battle_tag_raw,
      source: row.source,
    });
    setAliasForm({
      canonical_alias: row.canonical_alias || '',
      battle_tag: row.battle_tag_raw || '',
      aurora_id: row.aurora_id != null ? String(row.aurora_id) : '',
    });
  };

  const handleAliasCancelEdit = () => {
    setAliasEditOriginal(null);
    setAliasForm({ canonical_alias: '', battle_tag: '', aurora_id: '' });
    setAliasesMessage('');
    setAliasesMessageIsError(false);
  };

  const handleAliasSourceToggle = (value) => {
    setAliasSources((prev) => {
      if (prev.includes(value)) {
        return prev.filter((v) => v !== value);
      }
      return [...prev, value].sort((a, b) => a.localeCompare(b));
    });
  };

  const handleAliasExport = () => {
    const byCanonical = {};
    for (const row of aliases || []) {
      const key = row.canonical_alias || '';
      if (!Object.prototype.hasOwnProperty.call(byCanonical, key)) {
        byCanonical[key] = [];
      }
      const entry = { battle_tag: row.battle_tag_raw || '' };
      if (row.aurora_id != null) {
        entry.aurora_id = row.aurora_id;
      }
      byCanonical[key].push(entry);
    }
    const blob = new Blob([JSON.stringify(byCanonical, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement('a');
    anchor.href = url;
    anchor.download = 'aliases-export.json';
    anchor.click();
    URL.revokeObjectURL(url);
  };

  const handleAliasDelete = async (id) => {
    try {
      setAliasesMessage('');
      setAliasesMessageIsError(false);
      await api.deleteAliasEntry(id);
      if (aliasEditOriginal && aliasEditOriginal.id === id) {
        setAliasEditOriginal(null);
        setAliasForm({ canonical_alias: '', battle_tag: '', aurora_id: '' });
      }
      setAliasesMessage('Alias removed.');
      await loadAliases();
    } catch (err) {
      setAliasesMessage(err.message || 'Failed to delete alias');
      setAliasesMessageIsError(true);
    }
  };

  const handleAliasImportFile = async (file) => {
    try {
      setAliasesMessage('');
      setAliasesMessageIsError(false);
      const text = await file.text();
      const parsed = JSON.parse(text);
      const payload =
        parsed &&
        typeof parsed === 'object' &&
        !Array.isArray(parsed) &&
        parsed.aliases &&
        typeof parsed.aliases === 'object' &&
        !Array.isArray(parsed.aliases)
          ? parsed.aliases
          : parsed;
      await api.importAliases(payload);
      setAliasesMessage('Alias file imported.');
      await loadAliases();
    } catch (err) {
      setAliasesMessage(err.message || 'Failed to import alias file');
      setAliasesMessageIsError(true);
    }
  };

  const showAutoIngestNotice = (message) => {
    if (autoIngestNoticeTimerRef.current) {
      window.clearTimeout(autoIngestNoticeTimerRef.current);
    }
    setAutoIngestNotice(message);
    autoIngestNoticeTimerRef.current = window.setTimeout(() => {
      setAutoIngestNotice('');
      autoIngestNoticeTimerRef.current = null;
    }, 3500);
  };

  const pollForReplayCountIncrease = async (baselineCount, intervalSeconds) => {
    const maxWaitMs = Math.max(5000, Math.floor(intervalSeconds * 1000 * 0.5));
    const stepMs = 3000;
    const attempts = Math.max(1, Math.floor(maxWaitMs / stepMs));

    for (let attempt = 0; attempt < attempts; attempt += 1) {
      await sleep(stepMs);
      try {
        const health = await api.getHealth();
        const totalReplays = Number(health?.total_replays || 0);
        if (totalReplays >= baselineCount + 1) {
          setReplayCount(totalReplays);
          setOpenaiEnabled(Boolean(health?.openai_enabled));
          return true;
        }
      } catch (err) {
        console.error('Failed to poll replay count after auto-ingest:', err);
      }
    }

    return false;
  };

  openMainGameRef.current = openMainGame;
  openMainPlayerRef.current = openMainPlayer;
  loadDashboardRef.current = loadDashboard;

  useEffect(() => {
    // Load dashboard with stored variable values if available (initial URL only; switches use loadDashboard directly).
    const url = currentDashboardUrl;
    const stored = getStoredVariableValues(url);
    loadDashboard(url, stored || undefined);
    loadDashboards();
    loadGlobalReplayFilterConfig().catch((err) => {
      console.error('Failed to load global replay filter config:', err);
    });
    loadGlobalReplayFilterOptions().catch((err) => {
      console.error('Failed to load global replay filter options:', err);
    });
    loadTopPlayerColors();
    checkOpenAIStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount-only; deep links set currentDashboardUrl before first paint.
  }, []);

  useEffect(() => {
    if (initialMainRoute.view === 'game' && initialMainRoute.replayId != null) {
      void openMainGame(initialMainRoute.replayId, { initialGameTab: initialMainRoute.gameTab });
    } else if (initialMainRoute.view === 'player' && initialMainRoute.playerKey) {
      void openMainPlayer(initialMainRoute.playerKey);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one-time hydration from initial URL.
  }, []);

  useEffect(() => {
    if (suppressUrlSyncRef.current) return;
    const next = buildMainRouteSearch({
      activeView,
      selectedReplayId,
      selectedPlayerKey,
      mainGameTab,
      mainPlayersTab,
      currentDashboardUrl,
    });
    if (typeof window !== 'undefined' && mainRouteSnapshotEqual(window.location.search, next && next.length ? `?${next}` : '')) {
      return;
    }
    if (typeof window === 'undefined') return;
    window.history.pushState({ __spa: 1 }, '', mainRouteHref(next));
  }, [activeView, selectedReplayId, selectedPlayerKey, mainGameTab, mainPlayersTab, currentDashboardUrl]);

  useEffect(() => {
    const onPopState = () => {
      suppressUrlSyncRef.current = true;
      const r = parseMainRouteSearch(window.location.search);
      setActiveView(r.view);
      setSelectedReplayId(r.replayId);
      setSelectedPlayerKey(r.playerKey || '');
      setMainGameTab(r.gameTab);
      setMainPlayersTab(r.playersTab);
      setCurrentDashboardUrl(r.view === 'dashboards' && r.dash ? r.dash : 'default');
      const finish = () => {
        suppressUrlSyncRef.current = false;
      };
      if (r.view === 'game' && r.replayId != null) {
        const p = openMainGameRef.current?.(r.replayId, { initialGameTab: r.gameTab });
        if (p && typeof p.finally === 'function') {
          p.finally(finish);
        } else {
          finish();
        }
      } else if (r.view === 'player' && r.playerKey) {
        const p = openMainPlayerRef.current?.(r.playerKey);
        if (p && typeof p.finally === 'function') {
          p.finally(finish);
        } else {
          finish();
        }
      } else if (r.view === 'dashboards') {
        const dashUrl = r.dash || 'default';
        const stored = getStoredVariableValues(dashUrl);
        const p = loadDashboardRef.current?.(dashUrl, stored || undefined);
        if (p && typeof p.finally === 'function') {
          p.finally(finish);
        } else {
          finish();
        }
      } else {
        finish();
      }
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    loadMainGames({ page: mainGamesPage, filters: mainGamesFilters });
  }, [mainGamesPage, mainGamesFilters]);

  useEffect(() => {
    loadMainPlayers({
      page: mainPlayersPage,
      filters: mainPlayersFilters,
      sortBy: mainPlayersSortBy,
      sortDir: mainPlayersSortDir,
    });
  }, [mainPlayersPage, mainPlayersFilters, mainPlayersSortBy, mainPlayersSortDir]);

  useEffect(() => {
    if (activeView !== 'players' || mainPlayersTab !== 'apm-histogram') return;
    if (!mainPlayersApmHistogram && !mainPlayersApmHistogramLoading && !mainPlayersApmHistogramError) {
      loadMainPlayersApmHistogram();
    }
  }, [
    activeView,
    mainPlayersTab,
    mainPlayersApmHistogram,
    mainPlayersApmHistogramLoading,
    mainPlayersApmHistogramError,
  ]);

  useEffect(() => {
    if (activeView !== 'players' || mainPlayersTab !== 'first-unit-delay') return;
    if (!mainPlayersDelayHistogram && !mainPlayersDelayHistogramLoading && !mainPlayersDelayHistogramError) {
      loadMainPlayersDelayHistogram();
    }
  }, [
    activeView,
    mainPlayersTab,
    mainPlayersDelayHistogram,
    mainPlayersDelayHistogramLoading,
    mainPlayersDelayHistogramError,
  ]);

  useEffect(() => {
    if (activeView !== 'players' || mainPlayersTab !== 'unit-production-cadence') return;
    if (!mainPlayersCadenceHistogram && !mainPlayersCadenceHistogramLoading && !mainPlayersCadenceHistogramError) {
      loadMainPlayersCadenceHistogram();
    }
  }, [
    activeView,
    mainPlayersTab,
    mainPlayersCadenceHistogram,
    mainPlayersCadenceHistogramLoading,
    mainPlayersCadenceHistogramError,
  ]);

  useEffect(() => {
    if (activeView !== 'players' || mainPlayersTab !== 'viewport-multitasking') return;
    if (!mainPlayersViewportHistogram && !mainPlayersViewportHistogramLoading && !mainPlayersViewportHistogramError) {
      loadMainPlayersViewportHistogram();
    }
  }, [
    activeView,
    mainPlayersTab,
    mainPlayersViewportHistogram,
    mainPlayersViewportHistogramLoading,
    mainPlayersViewportHistogramError,
  ]);

  useEffect(() => {
    saveAutoIngestSettings({
      enabled: ingestForm.autoIngestEnabled,
    });
  }, [ingestForm.autoIngestEnabled]);

  useEffect(() => {
    if (!showIngestPanel) {
      setIngestSocketState('closed');
      return undefined;
    }

    setIngestMessage('');
    void loadIngestSettings();
    setIngestSocketState('connecting');

    const socket = api.createIngestLogsSocket();
    ingestSocketRef.current = socket;

    socket.onopen = () => {
      setIngestSocketState('open');
    };

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        if (message.type === 'snapshot') {
          setIngestStatus(message.status || 'idle');
          setIngestLogs(hydrateIngestLogEntries(message.logs || []));
          if (message.error) {
            setIngestMessage(message.error);
          }
          return;
        }

        if (message.type === 'log' && message.log) {
          setIngestLogs((current) => mergeIngestLogEntries(current, message.log));
          return;
        }

        if (message.type === 'status') {
          setIngestStatus(message.status || 'idle');
          if (message.error) {
            setIngestMessage(message.error);
          } else if (message.status === 'running') {
            setIngestMessage('');
          } else if (message.status === 'completed') {
            setIngestMessage('Ingestion completed.');
            void refreshDataAfterGlobalReplayFilterSave();
          }
        }
      } catch (err) {
        console.error('Failed to parse ingest stream message:', err);
      }
    };

    socket.onerror = () => {
      setIngestSocketState('error');
    };

    socket.onclose = () => {
      if (ingestSocketRef.current === socket) {
        ingestSocketRef.current = null;
      }
      setIngestSocketState('closed');
    };

    return () => {
      if (ingestSocketRef.current === socket) {
        ingestSocketRef.current = null;
      }
      socket.close();
    };
  }, [showIngestPanel]);

  useEffect(() => {
    if (!showGlobalReplayFilter) {
      return undefined;
    }
    setAliasesMessage('');
    setAliasesMessageIsError(false);
    setAliasEditOriginal(null);
    setAliasForm({ canonical_alias: '', battle_tag: '', aurora_id: '' });
    setAliasSources(['you', 'manual', 'imported']);
    void loadIngestSettings();
    void loadAliases();
    return undefined;
  }, [showGlobalReplayFilter]);

  useEffect(() => {
    if (!ingestForm.autoIngestEnabled) {
      return undefined;
    }

    const intervalSeconds = 60;
    let cancelled = false;

    const runAutoIngest = async () => {
      if (cancelled || autoIngestInFlight.current || showIngestPanel) return;
      autoIngestInFlight.current = true;
      try {
        const health = await api.getHealth();
        const baselineCount = Number(health?.total_replays || 0);
        const ingestResponse = await api.startIngest({
          watch: false,
          stop_after_n_reps: 1,
          clean: false,
          store_right_clicks: false,
          skip_hotkeys: false,
        });
        if (!ingestResponse?.started) {
          return;
        }

        const didIncrease = await pollForReplayCountIncrease(baselineCount, intervalSeconds);
        if (didIncrease) {
          await refreshDataAfterGlobalReplayFilterSave();
          showAutoIngestNotice('auto-ingested new replays');
        }
      } catch (err) {
        console.error('Auto-ingest failed:', err);
      } finally {
        autoIngestInFlight.current = false;
      }
    };

    const timer = window.setInterval(runAutoIngest, intervalSeconds * 1000);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [ingestForm.autoIngestEnabled, showIngestPanel]);

  useEffect(() => () => {
    if (autoIngestNoticeTimerRef.current) {
      window.clearTimeout(autoIngestNoticeTimerRef.current);
    }
  }, []);

  useEffect(() => () => {
    if (mainGameSeeNoticeTimerRef.current) {
      window.clearTimeout(mainGameSeeNoticeTimerRef.current);
    }
  }, []);

  const checkOpenAIStatus = async () => {
    try {
      const data = await api.getHealth();
      setOpenaiEnabled(Boolean(data?.openai_enabled));
      setReplayCount(Number(data?.total_replays || 0));
      return data;
    } catch (err) {
      console.error('Failed to check OpenAI status:', err);
      return null;
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
      let nextInputDir = String(ingestInputDir || '').trim();
      if (!nextInputDir) {
        throw new Error('Replay folder is required');
      }
      if (nextInputDir !== String(savedIngestInputDir || '').trim()) {
        nextInputDir = await persistIngestInputDir(nextInputDir);
      }

      const response = await api.startIngest({
        input_dir: nextInputDir,
        watch: ingestForm.watch,
        stop_after_n_reps: ingestForm.stopAfterN || 0,
        clean: ingestForm.clean,
        store_right_clicks: false,
        skip_hotkeys: false,
      });

      if (response?.started) {
        setIngestStatus('running');
        setIngestLogs([]);
        setIngestMessage('');
        return;
      }

      if (response?.in_progress) {
        setIngestStatus('running');
        setIngestMessage('Ingestion is already in progress.');
        return;
      }
    } catch (err) {
      setIngestMessage(err.message || 'Failed to start ingestion.');
    }
  };

  const handleSaveIngestInputDir = async () => {
    setIngestMessage('');
    try {
      await persistIngestInputDir(ingestInputDir);
      setIngestMessage('Replay folder saved.');
      void loadAliases();
    } catch (err) {
      setIngestMessage(err.message || 'Failed to save replay folder.');
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

  const refreshDataAfterGlobalReplayFilterSave = async () => {
    await Promise.all([
      loadMainGames({ page: mainGamesPage, filters: mainGamesFilters }),
      loadMainPlayers({
        page: mainPlayersPage,
        filters: mainPlayersFilters,
        sortBy: mainPlayersSortBy,
        sortDir: mainPlayersSortDir,
      }),
      loadDashboard(currentDashboardUrl, variableValues, true),
      loadTopPlayerColors(),
      checkOpenAIStatus(),
      loadGlobalReplayFilterOptions(),
    ]);

    if (activeView === 'game' && selectedReplayId) {
      try {
        await openMainGame(selectedReplayId);
      } catch (err) {
        console.error('Failed to reload main game after global filter save:', err);
      }
    }
    if (activeView === 'player' && selectedPlayerKey) {
      try {
        await openMainPlayer(selectedPlayerKey);
      } catch (err) {
        console.error('Failed to reload main player after global filter save:', err);
      }
    }
    if (mainPlayersApmHistogram) {
      loadMainPlayersApmHistogram();
    }
    if (mainPlayersDelayHistogram) {
      loadMainPlayersDelayHistogram();
    }
    if (mainPlayersCadenceHistogram) {
      loadMainPlayersCadenceHistogram();
    }
  };

  const handleSaveGlobalReplayFilter = async (nextConfig) => {
    try {
      setGlobalReplayFilterSaving(true);
      setGlobalReplayFilterError('');
      const saved = await api.updateGlobalReplayFilter(nextConfig);
      setGlobalReplayFilterConfig(saved);
      await refreshDataAfterGlobalReplayFilterSave();
      setShowGlobalReplayFilter(false);
    } catch (err) {
      setGlobalReplayFilterError(err.message || 'Failed to save main config');
    } finally {
      setGlobalReplayFilterSaving(false);
    }
  };

  const setMainGameSingleFilter = (name, nextValue) => {
    setMainGamesPage(1);
    setMainGamesFilters((prev) => ({
      ...prev,
      [name]: nextValue ? [nextValue] : [],
    }));
  };

  const toggleMainGameMultiFilter = (name, value) => {
    setMainGamesPage(1);
    setMainGamesFilters((prev) => ({
      ...prev,
      [name]: toggleFilterValue(prev[name] || [], value),
    }));
  };

  const clearMainGamesFilters = () => {
    setMainGamesPage(1);
    setMainGamesFilters({
      player: [],
      map: [],
      duration: [],
      featuring: [],
    });
  };

  const setMainPlayersSingleFilter = (name, nextValue) => {
    setMainPlayersPage(1);
    setMainPlayersFilters((prev) => ({
      ...prev,
      [name]: nextValue,
    }));
  };

  const toggleMainPlayersMultiFilter = (name, value) => {
    setMainPlayersPage(1);
    setMainPlayersFilters((prev) => ({
      ...prev,
      [name]: toggleFilterValue(prev[name] || [], value),
    }));
  };

  const clearMainPlayersFilters = () => {
    setMainPlayersPage(1);
    setMainPlayersFilters({
      name: '',
      onlyFivePlus: false,
      lastPlayed: [],
    });
    setMainPlayersSortBy('games');
    setMainPlayersSortDir('desc');
  };

  const setMainPlayersSort = (sortBy) => {
    setMainPlayersPage(1);
    setMainPlayersSortBy((prevSortBy) => {
      if (prevSortBy === sortBy) {
        setMainPlayersSortDir((prevDir) => (prevDir === 'asc' ? 'desc' : 'asc'));
        return prevSortBy;
      }
      setMainPlayersSortDir(sortBy === 'games' || sortBy === 'last_played' ? 'desc' : 'asc');
      return sortBy;
    });
  };

  const toggleMainPlayersDelayCase = (caseKey) => {
    const normalized = String(caseKey || '').trim();
    if (!normalized) return;
    setMainPlayersDelaySelectedCases((prev) => {
      const current = Array.isArray(prev) ? prev : ['all'];
      if (normalized === 'all') return ['all'];
      const withoutAll = current.filter((value) => value && value !== 'all');
      const already = withoutAll.includes(normalized);
      if (already) {
        const next = withoutAll.filter((value) => value !== normalized);
        return next.length === 0 ? ['all'] : next;
      }
      return [...withoutAll, normalized];
    });
  };

  const navigateMainView = (nextView) => {
    setActiveView((currentView) => {
      if (currentView === nextView) return currentView;
      return nextView;
    });
  };

  const goBackMainView = () => {
    setActiveView((currentView) => (currentView === 'player' ? 'players' : currentView));
  };

  const openMainPlayersSubview = (tab) => {
    const nextTab = String(tab || 'summary');
    setMainPlayersTab(nextTab);
    navigateMainView('players');
  };

  const handleMainAsk = async (e) => {
    e.preventDefault();
    const question = mainQuestion.trim();
    if (!question || mainAskLoading) return;
    try {
      setMainAskLoading(true);
      setMainAnswer(null);
      if (activeView === 'game' && mainGame?.replay_id) {
        const response = await api.askGame(mainGame.replay_id, question);
        setMainAnswer(response);
      } else if (activeView === 'player' && mainPlayer?.player_key) {
        const response = await api.askPlayer(mainPlayer.player_key, question);
        setMainAnswer(response);
      }
    } catch (err) {
      setMainAnswer({
        title: 'AI Error',
        description: 'The question could not be answered.',
        config: { type: 'text' },
        text_answer: `Failed to ask AI: ${err.message}`,
        results: [],
        columns: [],
      });
    } finally {
      setMainAskLoading(false);
    }
  };

  const playerAccentColor = (nameOrKey) => {
    const raw = String(nameOrKey || '').trim().toLowerCase();
    if (!raw) {
      return '';
    }
    if (topPlayerColors[raw]) {
      return topPlayerColors[raw];
    }
    // Display names append " (alias)" after the replay header name; /api/player-colors keys are player_key (normalized raw name).
    const withoutDisplaySuffix = raw.replace(/ \([^)]+\)$/, '').trim().toLowerCase();
    if (withoutDisplaySuffix && withoutDisplaySuffix !== raw && topPlayerColors[withoutDisplaySuffix]) {
      return topPlayerColors[withoutDisplaySuffix];
    }
    return '';
  };

  const renderPlayerLabel = (name, colorLookupKey) => {
    const color = playerAccentColor(colorLookupKey || name);
    if (!color) return <span>{name}</span>;
    return <span style={{ color, fontWeight: 600 }}>{name}</span>;
  };

  const renderPlayersMatchup = (label) => {
    const sides = String(label || '').split(' vs ');
    return sides.map((side, sideIndex) => {
      const names = String(side || '')
        .trim()
        .split(', ')
        .map((n) => n.trim())
        .filter(Boolean);
      return (
        <span key={`${side}-${sideIndex}`}>
          {names.map((name, idx) => (
            <span key={`${name}-${idx}`}>
              {renderPlayerLabel(name)}
              {idx < names.length - 1 ? ', ' : ''}
            </span>
          ))}
          {sideIndex < sides.length - 1 ? ' vs ' : ''}
        </span>
      );
    });
  };

  const renderMainGameListPlayers = (game) => {
    const players = Array.isArray(game?.players) ? game.players : [];
    if (players.length === 0) {
      return renderPlayersMatchup(game?.players_label || '');
    }
    if (!playersHaveDistinctTeams(players)) {
      return (
        <span>
          {players.map((player, idx) => (
            <span key={`${player.player_id}-${idx}`}>
              {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
              {renderPlayerLabel(player.name, player.player_key)}
              {idx < players.length - 1 ? ', ' : ''}
            </span>
          ))}
        </span>
      );
    }
    const groups = teamGroupsFromPlayers(players);
    return (
      <span className="workflow-team-matchup">
        {groups.map((group, groupIdx) => (
          <React.Fragment key={`team-${groupIdx}`}>
            {groupIdx > 0 ? <span className="workflow-team-vs">vs</span> : null}
            <span className="workflow-team-side">
              {group.map((player) => (
                <span
                  key={player.player_id}
                  className="workflow-team-player-pill"
                  style={{ backgroundColor: teamColorRgba(player.team, 0.24) }}
                >
                  {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                  {renderPlayerLabel(player.name, player.player_key)}
                </span>
              ))}
            </span>
          </React.Fragment>
        ))}
      </span>
    );
  };

  const renderMainAiResult = () => {
    if (!mainAnswer) return null;
    const config = mainAnswer.config || { type: 'text' };
    const data = mainAnswer.results || [];
    const columns = mainAnswer.columns || [];
    const chartProps = { data, config };

    if (config.type === 'text') {
      return (
        <div className="workflow-answer">
          {mainAnswer.title ? <div className="workflow-answer-title">{mainAnswer.title}</div> : null}
          <div>{mainAnswer.text_answer || mainAnswer.description || 'No text answer returned.'}</div>
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
        {mainAnswer.title ? <div className="workflow-answer-title">{mainAnswer.title}</div> : null}
        {mainAnswer.description ? <div className="workflow-answer-description">{mainAnswer.description}</div> : null}
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

  const summaryTextMatches = (text) => {
    const value = String(text || '').toLowerCase();
    const activeTopics = Object.entries(SUMMARY_TOPIC_PATTERNS)
      .filter(([key]) => mainSummaryFilters[key])
      .map(([, matcher]) => matcher);
    if (activeTopics.length > 0 && !activeTopics.some((matcher) => matcher.test(value))) {
      return false;
    }
    return true;
  };

  const mainPlayerOutlierItems = mainPlayerOutliers?.items || [];

  const topicFilteredGameEvents = useMemo(() => {
    const allEvents = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    const visibleEvents = allEvents.filter((event) => {
      if (isStructuralGameEventType(event?.type)) {
        return false;
      }
      if (normalizeEventType(event?.type) === 'takeover') {
        return false;
      }
      return summaryTextMatches(gameEventSearchText(event));
    });
    const deduped = [];
    for (let idx = 0; idx < visibleEvents.length; idx += 1) {
      const event = visibleEvents[idx];
      const prev = deduped.length > 0 ? deduped[deduped.length - 1] : null;
      if (prev && gameEventDescription(prev) === gameEventDescription(event)) {
        continue;
      }
      deduped.push(event);
    }
    return deduped;
  }, [mainGame?.game_events, mainSummaryFilters]);

  const filteredGameEvents = useMemo(() => (
    topicFilteredGameEvents.filter((event) => {
      const actorId = eventActorID(event);
      if (actorId != null && mainEventsPlayerEnabledById[String(actorId)] === false) {
        return false;
      }
      return true;
    })
  ), [topicFilteredGameEvents, mainEventsPlayerEnabledById]);
  const gameEventTopicAvailability = useMemo(() => {
    const base = {
      nuke: false,
      drop: false,
      recall: false,
      scout: false,
      becameRace: false,
      rush: false,
    };
    const allEvents = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    for (const event of allEvents) {
      if (isStructuralGameEventType(event?.type)) continue;
      const nt = normalizeEventType(event?.type);
      if (nt === 'takeover') continue;
      const text = gameEventSearchText(event);
      if (SUMMARY_TOPIC_PATTERNS.nuke.test(text)) base.nuke = true;
      if (SUMMARY_TOPIC_PATTERNS.drop.test(text)) base.drop = true;
      if (SUMMARY_TOPIC_PATTERNS.recall.test(text)) base.recall = true;
      if (nt === 'scout' || SUMMARY_TOPIC_PATTERNS.scout.test(text)) base.scout = true;
      if (SUMMARY_TOPIC_PATTERNS.becameRace.test(text) || nt === 'became_terran' || nt === 'became_zerg') base.becameRace = true;
      if (SUMMARY_TOPIC_PATTERNS.rush.test(text)) base.rush = true;
    }
    return base;
  }, [mainGame?.game_events]);
  const mainMapVisual = mainGame?.map_visual || {};
  const mainMapVisualURL = String(mainMapVisual?.url || '').trim();
  const mainMapVisualThumbURL = String(mainMapVisual?.thumbnail_url || mainMapVisualURL).trim();
  const mainMapVisualAvailable = Boolean(mainMapVisual?.available && mainMapVisualURL);
  const mainEventMapBounds = useMemo(
    () =>
      mapBoundsFromDimensions(mainGame?.map_width_pixels, mainGame?.map_height_pixels) ||
      mapBoundsFromGameEvents(mainGame?.game_events || []),
    [mainGame?.game_events, mainGame?.map_width_pixels, mainGame?.map_height_pixels],
  );
  const selectedMainGameEvent = useMemo(() => {
    if (!topicFilteredGameEvents.length) return null;
    const topicIdx = parseGameEventTopicKey(mainSelectedGameEventKey);
    if (topicIdx != null && topicIdx >= 0 && topicIdx < topicFilteredGameEvents.length) {
      return topicFilteredGameEvents[topicIdx];
    }
    return topicFilteredGameEvents[0];
  }, [topicFilteredGameEvents, mainSelectedGameEventKey]);
  const mainGamePlayers = mainGame?.players || [];
  const selectedMainGameEventKeyResolved = useMemo(() => {
    if (!selectedMainGameEvent) return '';
    const idx = topicFilteredGameEvents.indexOf(selectedMainGameEvent);
    if (idx < 0) return '';
    return gameEventTopicKey(idx);
  }, [topicFilteredGameEvents, selectedMainGameEvent]);
  useEffect(() => {
    if (topicFilteredGameEvents.length === 0) {
      if (mainSelectedGameEventKey) setMainSelectedGameEventKey('');
      return;
    }
    const topicIdx = parseGameEventTopicKey(mainSelectedGameEventKey);
    if (topicIdx != null && topicIdx >= 0 && topicIdx < topicFilteredGameEvents.length) {
      return;
    }
    const firstRowVisibleIdx = topicFilteredGameEvents.findIndex((event) => {
      const actorId = eventActorID(event);
      return actorId == null || mainEventsPlayerEnabledById[String(actorId)] !== false;
    });
    const preferredIdx = firstRowVisibleIdx >= 0 ? firstRowVisibleIdx : 0;
    setMainSelectedGameEventKey(gameEventTopicKey(preferredIdx));
  }, [topicFilteredGameEvents, mainEventsPlayerEnabledById, mainSelectedGameEventKey]);
  const selectedMainGameOwnershipPolygons = useMemo(() => {
    const ownership = Array.isArray(selectedMainGameEvent?.ownership) ? selectedMainGameEvent.ownership : [];
    return ownership
      .map((entry, idx) => {
        const polygon = Array.isArray(entry?.base?.polygon) ? entry.base.polygon : [];
        if (polygon.length < 3 || !entry?.owner || !mainEventMapBounds) return null;
        const points = polygon
          .map((point) => mapPointToPercent(point, mainEventMapBounds))
          .filter(Boolean)
          .map((point) => `${point.x},${point.y}`)
          .join(' ');
        if (!points) return null;
        const ownerColor = playerColorToCss(entry.owner.color);
        return {
          key: `ownership-${idx}-${entry.base?.name || 'base'}`,
          points,
          ownerName: entry.owner.name,
          ownerColor,
        };
      })
      .filter(Boolean);
  }, [selectedMainGameEvent, mainEventMapBounds]);
  const selectedMainGameLegend = useMemo(() => {
    return (Array.isArray(mainGamePlayers) ? mainGamePlayers : [])
      .map((player) => ({
        name: player?.name || '',
        rawColor: player?.color || '',
        color: playerColorToCss(player?.color),
      }))
      .filter((player) => player.name);
  }, [mainGamePlayers]);
  const summaryMapStartPolygons = useMemo(() => {
    const bounds = mainEventMapBounds;
    if (!bounds) return [];
    const events = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    const acc = [];
    events.forEach((ev, idx) => {
      if (normalizeEventType(ev?.type) !== 'player_start') return;
      if (!ev?.actor) return;
      const polygon = Array.isArray(ev?.base?.polygon) ? ev.base.polygon : [];
      if (polygon.length < 3) return;
      const points = polygon
        .map((point) => mapPointToPercent(point, bounds))
        .filter(Boolean)
        .map((point) => `${point.x},${point.y}`)
        .join(' ');
      if (!points) return;
      const pid = eventActorID(ev);
      acc.push({
        key: `sum-start-poly-${pid != null ? pid : idx}`,
        points,
        ownerName: String(ev.actor.name || '').trim() || 'Player',
        ownerColor: playerColorToCss(ev.actor.color),
      });
    });
    return acc;
  }, [mainGame?.game_events, mainEventMapBounds]);
  const mainGameFeaturingPillsList = useMemo(() => buildMainGameFeaturingPills(mainGame), [mainGame]);
  const selectedMainGameArrow = useMemo(() => {
    if (!selectedMainGameEvent || !isArrowEventType(selectedMainGameEvent.type)) return null;
    const from = mapPointToPercent(selectedMainGameEvent?.actor_origin, mainEventMapBounds);
    const to = mapPointToPercent(selectedMainGameEvent?.base?.center, mainEventMapBounds);
    if (!from || !to) return null;
    return {
      from,
      to,
      color: playerColorToCss(selectedMainGameEvent?.actor?.color),
    };
  }, [selectedMainGameEvent, mainEventMapBounds]);
  const selectedMainGameSyntheticOverlay = useMemo(() => {
    if (!selectedMainGameEvent) {
      return { ownershipPolygons: [], arrow: null, leaveFlagPoint: null };
    }
    const allEvents = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    if (allEvents.length === 0) {
      return { ownershipPolygons: [], arrow: null, leaveFlagPoint: null };
    }
    const selectedEventSecond = Number(selectedMainGameEvent?.second || 0);
    const ownershipByBase = new Map();
    const startPointByPlayerID = new Map();

    allEvents.forEach((event) => {
      const second = Number(event?.second || 0);
      if (second > selectedEventSecond) return;
      const type = normalizeEventType(event?.type);
      const baseKey = eventBaseKey(event);
      const baseClock = eventBaseClock(event);
      const baseCenter = syntheticPointForClock(baseClock);
      const actorID = eventActorID(event);
      const actor = event?.actor || null;
      if (type === 'player_start' && actorID && baseCenter) {
        startPointByPlayerID.set(actorID, baseCenter);
      }
      if (!baseKey || !baseCenter) {
        if (type === 'leave_game' && actorID) {
          Array.from(ownershipByBase.entries()).forEach(([key, value]) => {
            if (Number(value?.owner?.player_id) === actorID) ownershipByBase.delete(key);
          });
        }
        return;
      }
      if ((type === 'player_start' || type === 'expansion' || type === 'takeover') && actorID && actor) {
        ownershipByBase.set(baseKey, {
          baseKey,
          baseKind: String(event?.base?.kind || ''),
          center: baseCenter,
          owner: actor,
        });
      } else if (type === 'location_inactive') {
        ownershipByBase.delete(baseKey);
      } else if (type === 'leave_game' && actorID) {
        Array.from(ownershipByBase.entries()).forEach(([key, value]) => {
          if (Number(value?.owner?.player_id) === actorID) ownershipByBase.delete(key);
        });
      }
    });

    const ownershipPolygons = Array.from(ownershipByBase.values()).map((entry, idx) => ({
      key: `synthetic-ownership-${idx}-${entry.baseKey}`,
      points: syntheticPolygonForCenter(entry.center).map((pt) => `${pt.x},${pt.y}`).join(' '),
      ownerName: String(entry?.owner?.name || ''),
      ownerColor: playerColorToCss(entry?.owner?.color),
    })).filter((entry) => entry.points);

    let arrow = null;
    if (isArrowEventType(selectedMainGameEvent?.type)) {
      const target = syntheticPointForClock(eventBaseClock(selectedMainGameEvent));
      const actorID = eventActorID(selectedMainGameEvent);
      const from = actorID ? startPointByPlayerID.get(actorID) : null;
      if (from && target) {
        arrow = {
          from,
          to: target,
          color: playerColorToCss(selectedMainGameEvent?.actor?.color),
        };
      }
    }

    let leaveFlagPoint = null;
    if (normalizeEventType(selectedMainGameEvent?.type) === 'leave_game') {
      const actorID = eventActorID(selectedMainGameEvent);
      if (actorID) {
        const ownedEntries = Array.from(ownershipByBase.values()).filter((entry) => Number(entry?.owner?.player_id) === actorID);
        const preferred = ownedEntries.find((entry) => String(entry?.baseKind || '').toLowerCase() === 'starting') || ownedEntries[0];
        if (preferred?.center) leaveFlagPoint = preferred.center;
      }
    }

    return { ownershipPolygons, arrow, leaveFlagPoint };
  }, [selectedMainGameEvent, mainGame?.game_events]);
  const effectiveMainGameOwnershipPolygons = useMemo(
    () => (selectedMainGameOwnershipPolygons.length > 0 ? selectedMainGameOwnershipPolygons : selectedMainGameSyntheticOverlay.ownershipPolygons),
    [selectedMainGameOwnershipPolygons, selectedMainGameSyntheticOverlay],
  );
  const effectiveMainGameArrow = useMemo(
    () => selectedMainGameArrow || selectedMainGameSyntheticOverlay.arrow,
    [selectedMainGameArrow, selectedMainGameSyntheticOverlay],
  );
  const selectedMainGameArrowUnits = useMemo(() => {
    if (!effectiveMainGameArrow || !selectedMainGameEvent) return [];
    const unitNames = Array.isArray(selectedMainGameEvent.attack_unit_types) && selectedMainGameEvent.attack_unit_types.length > 0
      ? selectedMainGameEvent.attack_unit_types
      : fallbackOverlayUnitNamesForEvent(selectedMainGameEvent.type);
    return unitNames
      .map((name) => ({ name, icon: getUnitIcon(name) }))
      .filter((item) => item.icon)
      .slice(0, 4);
  }, [effectiveMainGameArrow, selectedMainGameEvent]);
  const selectedMainGameLeaveFlag = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'leave_game' || !mainEventMapBounds) return null;
    const actorID = Number(selectedMainGameEvent?.actor?.player_id || 0);
    if (!Number.isFinite(actorID) || actorID <= 0) return null;
    const ownership = Array.isArray(selectedMainGameEvent?.ownership) ? selectedMainGameEvent.ownership : [];
    const ownedBases = ownership.filter((entry) => Number(entry?.owner?.player_id || 0) === actorID && entry?.base?.center);
    if (ownedBases.length === 0) return null;
    const preferredBase = ownedBases.find((entry) => String(entry?.base?.kind || '').toLowerCase() === 'starting') || ownedBases[0];
    return mapPointToPercent(preferredBase?.base?.center, mainEventMapBounds);
  }, [selectedMainGameEvent, mainEventMapBounds]);
  const effectiveMainGameLeaveFlag = useMemo(
    () => selectedMainGameLeaveFlag || selectedMainGameSyntheticOverlay.leaveFlagPoint,
    [selectedMainGameLeaveFlag, selectedMainGameSyntheticOverlay],
  );
  const selectedMainGameExpansionOverlay = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'expansion') return null;
    const baseCenter = selectedMainGameEvent?.base?.center;
    if (!baseCenter) return null;
    const playerID = Number(selectedMainGameEvent?.actor?.player_id || 0);
    const actorRow = mainGamePlayers.find((player) => Number(player?.player_id || 0) === playerID);
    const icon = getExpansionMarkerIconForRace(actorRow?.race);
    if (!icon) return null;
    const point = mapPointToPercent(baseCenter, mainEventMapBounds);
    if (!point) return null;
    return { icon, point };
  }, [selectedMainGameEvent, mainGamePlayers, mainEventMapBounds]);

  const mainPlayerInsights = [
    mainPlayerViewportInsight,
    mainPlayerApmInsight,
    mainPlayerDelayInsight,
    mainPlayerCadenceInsight,
  ].filter(Boolean);
  const mainPlayerInsightLoading = mainPlayerApmInsightLoading || mainPlayerDelayInsightLoading || mainPlayerCadenceInsightLoading || mainPlayerViewportInsightLoading;
  const mainPlayerInsightErrors = [
    mainPlayerApmInsightError,
    mainPlayerDelayInsightError,
    mainPlayerCadenceInsightError,
    mainPlayerViewportInsightError,
  ].filter(Boolean);
  const mainPlayerUsagePills = useMemo(() => {
    const pills = [];
    if ((Number(mainPlayer?.hotkey_usage_rate) || 0) < LOW_USAGE_THRESHOLD) {
      pills.push({
        key: 'no-hotkeys',
        label: '🚫 hotkeys',
        title: `Detected in ${(Number(mainPlayer?.hotkey_usage_rate) * 100).toFixed(1)}% of this player's games.`,
        className: 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey',
      });
    }
    if ((Number(mainPlayer?.queued_game_rate) || 0) < LOW_USAGE_THRESHOLD) {
      pills.push({
        key: 'no-queued-orders',
        label: 'Doesn\'t use queued orders',
        title: `Detected in ${(Number(mainPlayer?.queued_game_rate) * 100).toFixed(1)}% of this player's games.`,
        className: 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-queued',
      });
    }
    return pills;
  }, [mainPlayer]);
  const mainPlayerNameWidthCh = useMemo(() => {
    const longestNameLength = mainGamePlayers.reduce((longest, player) => {
      const nameLength = String(player?.name || '').trim().length;
      return Math.max(longest, nameLength);
    }, 0);
    if (!longestNameLength) return 15;
    return Math.max(12, Math.min(24, longestNameLength + 3));
  }, [mainGamePlayers]);
  const mainPlayersById = useMemo(
    () => new Map(mainGamePlayers.map((player) => [player.player_id, player])),
    [mainGamePlayers],
  );
  const hasTeamInfo = useMemo(() => {
    const uniqueTeams = new Set(mainGamePlayers.map((player) => player.team));
    return uniqueTeams.size > 1;
  }, [mainGamePlayers]);
  const mainTimingCategoryConfig = useMemo(
    () => TIMING_CATEGORY_CONFIG.find((cfg) => cfg.id === mainTimingCategory) || TIMING_CATEGORY_CONFIG[0],
    [mainTimingCategory],
  );
  const mainTimingSeries = useMemo(() => {
    const timings = mainGame?.timings || {};
    const sourceSeries = Array.isArray(timings?.[mainTimingCategoryConfig.source])
      ? timings[mainTimingCategoryConfig.source]
      : [];
    const sortedSeries = [...sourceSeries].sort((a, b) => {
      const raceDiff = raceRank(mainPlayersById.get(a?.player_id)?.race) - raceRank(mainPlayersById.get(b?.player_id)?.race);
      if (raceDiff !== 0) return raceDiff;
      const nameA = String(a?.name || '').toLowerCase();
      const nameB = String(b?.name || '').toLowerCase();
      if (nameA !== nameB) return nameA.localeCompare(nameB);
      return Number(a?.player_id || 0) - Number(b?.player_id || 0);
    });

    return sortedSeries.map((playerSeries) => {
      const playerRace = String(mainPlayersById.get(playerSeries?.player_id)?.race || '').trim();
      const sourcePoints = Array.isArray(playerSeries?.points) ? playerSeries.points : [];
      const mappedPoints = sourcePoints
        .map((point) => {
          const second = Number(point?.second);
          if (!Number.isFinite(second)) return null;
          const order = Number(point?.order) || 0;
          const rawLabel = String(point?.label || '').trim();
          const upgradeCategory = mainTimingCategoryConfig.source === 'upgrades' ? upgradeCategoryForName(rawLabel) : '';
          if (mainTimingCategoryConfig.source === 'upgrades' && upgradeCategory !== mainTimingCategory) return null;
          return {
            ...point,
            second,
            order,
            label: rawLabel,
            upgrade_category: upgradeCategory,
          };
        })
        .filter(Boolean);

      // Post-process noisy repeated commands:
      // - HP upgrades are repeatable up to 3 levels, so keep latest 3 per label.
      // - Other upgrades and tech are effectively one-off, so keep latest 1 per label.
      const pointsAfterPostProcess = (() => {
        const sourceType = mainTimingCategoryConfig.source;
        if (sourceType !== 'upgrades' && sourceType !== 'tech') return mappedPoints;
        const byLabel = new Map();
        mappedPoints.forEach((point) => {
          const key = String(point?.label || '').trim();
          if (!key) return;
          if (!byLabel.has(key)) byLabel.set(key, []);
          byLabel.get(key).push(point);
        });
        const collapsed = [];
        byLabel.forEach((items) => {
          const sortedBySecond = [...items].sort((a, b) => {
            if (a.second === b.second) return a.order - b.order;
            return a.second - b.second;
          });
          const keepCount = sourceType === 'upgrades' && mainTimingCategory === 'hp_upgrades' ? 3 : 1;
          const kept = sortedBySecond.slice(-keepCount);
          kept.forEach((item, idx) => {
            collapsed.push({
              ...item,
              order: idx + 1,
            });
          });
        });
        return collapsed.sort((a, b) => {
          if (a.second === b.second) return String(a.label || '').localeCompare(String(b.label || ''));
          return a.second - b.second;
        });
      })();

      const points = pointsAfterPostProcess.map((point) => {
        const order = Number(point?.order) || 0;
        const rawLabel = String(point?.label || '').trim();
        const upgradeCategory = String(point?.upgrade_category || '').trim();
        let displayLabel = rawLabel;
        let categoryLabel = 'Timing';
        let markerImage = null;
        let markerLabel = '';
        let isRepeatable = false;
        let maxLevel = 1;

        if (mainTimingCategoryConfig.source === 'upgrades') {
          displayLabel = inlineTimingUpgradeLabel(rawLabel, order);
          categoryLabel = mainTimingCategoryConfig.label;
          isRepeatable = upgradeCategory === 'hp_upgrades';
          maxLevel = isRepeatable ? 3 : 1;
        } else if (mainTimingCategoryConfig.source === 'tech') {
          displayLabel = normalizeTimingDisplayLabel(rawLabel);
          categoryLabel = 'Tech';
        } else if (mainTimingCategory === 'gas') {
          displayLabel = `Gas #${order || 1}`;
          categoryLabel = 'Gas';
          markerImage = getGasMarkerIconForRace(playerRace);
          markerLabel = mainTimingCategoryConfig.markerLabel || 'Gas';
        } else if (mainTimingCategory === 'expansion') {
          displayLabel = `Expansion #${order || 1}`;
          categoryLabel = 'Expansion';
          markerImage = getExpansionMarkerIconForRace(playerRace);
          markerLabel = mainTimingCategoryConfig.markerLabel || 'Expansion';
        }

        return {
          ...point,
          order,
          label: rawLabel,
          display_label: displayLabel,
          category: upgradeCategory || mainTimingCategory,
          category_label: categoryLabel,
          race: playerRace,
          marker_image: markerImage,
          marker_label: markerLabel,
          is_repeatable: isRepeatable,
          max_level: maxLevel,
        };
      });

      return {
        ...playerSeries,
        race: playerRace,
        race_icon: getRaceIcon(playerRace),
        points,
      };
    });
  }, [mainGame?.timings, mainTimingCategoryConfig, mainTimingCategory, mainPlayersById]);
  const mainTimingUsesLabelColors = useMemo(
    () => ['hp_upgrades', 'unit_range', 'unit_speed', 'energy', 'capacity_cooldown_damage', 'tech'].includes(mainTimingCategory),
    [mainTimingCategory],
  );
  const mainTimingAxisMode = useMemo(
    () => (['hp_upgrades', 'unit_range', 'unit_speed', 'energy', 'capacity_cooldown_damage', 'tech'].includes(mainTimingCategory) ? 'compressed15' : 'linear'),
    [mainTimingCategory],
  );
  const mainTimingInlineLegend = useMemo(
    () => ['hp_upgrades', 'unit_range', 'unit_speed', 'energy', 'capacity_cooldown_damage', 'tech'].includes(mainTimingCategory),
    [mainTimingCategory],
  );
  const mainTimingAxisTrimMaxSecond = useMemo(() => {
    if (!['gas', 'expansion'].includes(mainTimingCategory)) return undefined;
    const maxPointSecond = mainTimingSeries.reduce((maxSecond, playerSeries) => {
      const playerMax = (playerSeries?.points || []).reduce((innerMax, point) => {
        const second = Number(point?.second);
        return Number.isFinite(second) ? Math.max(innerMax, second) : innerMax;
      }, 0);
      return Math.max(maxSecond, playerMax);
    }, 0);
    return maxPointSecond > 0 ? maxPointSecond : undefined;
  }, [mainTimingCategory, mainTimingSeries]);
  const mainTimingNotice = useMemo(
    () => (mainTimingCategory === 'expansion'
      ? '⚠️ These are base expansions, not just Nexus/Hatchery/CC buildings.'
      : ''),
    [mainTimingCategory],
  );
  const mainHpTimingByRace = useMemo(() => {
    if (mainTimingCategory !== 'hp_upgrades') return [];
    return TIMING_RACE_ORDER.map((race) => {
      const raceSeries = mainTimingSeries.filter((playerSeries) => String(playerSeries?.race || '').trim().toLowerCase() === race);
      const racePrefix = racePrefixForUpgrade(race);
      const labelOptions = Array.from(new Set(
        raceSeries.flatMap((playerSeries) => (playerSeries?.points || []).map((point) => String(point?.label || '').trim()))
          .filter((label) => {
            if (!label) return false;
            if (!racePrefix) return true;
            return label.startsWith(racePrefix);
          }),
      )).sort((a, b) => a.localeCompare(b));
      const selectedValue = String(mainHpUpgradeFilters[race] || '').trim();
      const defaultForRace = String(DEFAULT_HP_UPGRADE_BY_RACE[race] || '').trim();
      const selected = labelOptions.includes(selectedValue)
        ? selectedValue
        : (labelOptions.includes(defaultForRace) ? defaultForRace : (labelOptions[0] || ''));
      const filteredSeries = raceSeries.map((playerSeries) => ({
        ...playerSeries,
        points: (playerSeries?.points || [])
          .filter((point) => selected && String(point?.label || '').trim() === selected)
          .map((point) => ({
            ...point,
            display_label: `+${Math.max(1, Number(point?.order) || 1)}`,
          })),
      }));
      return {
        race,
        raceLabel: prettyRaceName(race),
        labelOptions,
        selected,
        series: filteredSeries,
      };
    }).filter((entry) => entry.series.some((playerSeries) => (playerSeries?.points || []).length > 0));
  }, [mainTimingCategory, mainTimingSeries, mainHpUpgradeFilters]);
  const mainFirstUnitEfficiencyGroups = useMemo(() => {
    const sourcePlayers = Array.isArray(mainGame?.first_unit_efficiency) ? mainGame.first_unit_efficiency : [];
    const normalizedPlayers = sourcePlayers.map((playerEntry) => ({
      ...playerEntry,
      race: String(playerEntry?.race || '').trim().toLowerCase(),
      entries: Array.isArray(playerEntry?.entries) ? playerEntry.entries : [],
    }));
    return FIRST_UNIT_EFFICIENCY_GROUP_CONFIG.map((cfg) => {
      const unitKeySet = new Set(cfg.unitNames.map((name) => normalizeUnitName(name)));
      const rows = normalizedPlayers
        .filter((playerEntry) => playerEntry.race === cfg.race)
        .map((playerEntry) => {
          const matched = playerEntry.entries.find((entry) => (
            normalizeUnitName(entry?.building_name) === normalizeUnitName(cfg.buildingName)
            && unitKeySet.has(normalizeUnitName(entry?.unit_name))
          ));
          if (!matched) return null;
          return {
            player_id: playerEntry.player_id,
            player_name: playerEntry.name,
            player_key: playerEntry.player_key,
            race: playerEntry.race,
            ...matched,
            building_icon: getUnitIcon(matched?.building_name || cfg.buildingName),
            unit_icon: getUnitIcon(matched?.unit_name),
          };
        })
        .filter(Boolean)
        .sort((a, b) => String(a?.player_name || '').localeCompare(String(b?.player_name || '')));
      if (rows.length === 0) return null;
      return {
        id: `${cfg.race}-${normalizeUnitName(cfg.buildingName)}`,
        race: cfg.race,
        building_name: cfg.buildingName,
        building_icon: getUnitIcon(cfg.buildingName),
        unit_names: cfg.unitNames,
        unit_icons: cfg.unitNames
          .map((unitName) => getUnitIcon(unitName))
          .filter(Boolean),
        rows,
      };
    }).filter(Boolean);
  }, [mainGame?.first_unit_efficiency]);

  const filterProductionEntries = (entries, view) => {
    const mode = view === 'units' ? mainUnitFilterMode : mainBuildingFilterMode;
    const nameNeedle = String(view === 'units' ? mainUnitNameFilter : mainBuildingNameFilter).trim().toLowerCase();
    return (entries || []).filter((entry) => {
      const unitType = String(entry?.unit_type || '');
      const key = normalizeUnitName(unitType);
      const isBuilding = BUILDING_TYPE_KEYS.has(key);
      if (view === 'units' && isBuilding) return false;
      if (view === 'buildings' && !isBuilding) return false;
      if (nameNeedle && !unitType.toLowerCase().includes(nameNeedle)) return false;
      if (mode === 'all') return true;
      if (view === 'units') {
        if (mode === 'workers') return WORKER_UNIT_KEYS.has(key);
        if (mode === 'non-workers') return !WORKER_UNIT_KEYS.has(key);
        if (mode === 'spellcasters') return SPELLCASTER_UNIT_KEYS.has(key);
        if (mode === 'tier-1') return UNIT_TIER_MAP[key] === 1;
        if (mode === 'tier-2') return UNIT_TIER_MAP[key] === 2;
        if (mode === 'tier-3') return UNIT_TIER_MAP[key] === 3;
      } else {
        if (mode === 'defenses') return DEFENSIVE_BUILDING_KEYS.has(key);
        if (mode === 'tier-1') return BUILDING_TIER_MAP[key] === 1;
        if (mode === 'tier-2') return BUILDING_TIER_MAP[key] === 2;
        if (mode === 'tier-3') return BUILDING_TIER_MAP[key] === 3;
      }
      return true;
    });
  };

  const mainGamesTotalPages = Math.max(1, Math.ceil((Number(mainGamesTotal) || 0) / MAIN_GAMES_PAGE_SIZE));
  const mainGamesFrom = mainGames.length === 0 ? 0 : ((mainGamesPage - 1) * MAIN_GAMES_PAGE_SIZE) + 1;
  const mainGamesTo = mainGames.length === 0
    ? 0
    : Math.min((mainGamesPage - 1) * MAIN_GAMES_PAGE_SIZE + mainGames.length, Number(mainGamesTotal) || 0);
  const mainPlayersTotalPages = Math.max(1, Math.ceil((Number(mainPlayersTotal) || 0) / MAIN_PLAYERS_PAGE_SIZE));
  const mainPlayersFrom = mainPlayers.length === 0 ? 0 : ((mainPlayersPage - 1) * MAIN_PLAYERS_PAGE_SIZE) + 1;
  const mainPlayersTo = mainPlayers.length === 0
    ? 0
    : Math.min((mainPlayersPage - 1) * MAIN_PLAYERS_PAGE_SIZE + mainPlayers.length, Number(mainPlayersTotal) || 0);
  const playersApmHistogramPoints = useMemo(() => (
    (mainPlayersApmHistogram?.players || [])
      .map((player) => ({
        value: Number(player?.average_apm),
        label: String(player?.player_name || '').trim(),
        player_key: String(player?.player_key || '').trim(),
        games_played: Number(player?.games_played || 0),
      }))
      .filter((player) => Number.isFinite(player.value) && player.label)
  ), [mainPlayersApmHistogram]);
  const mainPlayersApmProcessed = useMemo(() => {
    const minGames = Math.max(5, Number(mainPlayersApmMinGames) || 5);
    const filtered = playersApmHistogramPoints
      .filter((player) => Number(player.games_played || 0) >= minGames)
      .map((player) => ({
        player_key: player.player_key,
        player_name: player.label,
        average_apm: player.value,
        games_played: player.games_played,
      }));
    return buildHistogramSummaryFromPlayers(filtered);
  }, [playersApmHistogramPoints, mainPlayersApmMinGames]);
  const mainPlayersDelayCaseOptions = useMemo(() => (
    (mainPlayersDelayHistogram?.case_options || [])
      .map((entry) => ({
        case_key: String(entry?.case_key || '').trim(),
        building_name: String(entry?.building_name || '').trim(),
        unit_name: String(entry?.unit_name || '').trim(),
        sample_count: Number(entry?.sample_count || 0),
      }))
      .filter((entry) => entry.case_key && entry.building_name && entry.unit_name)
  ), [mainPlayersDelayHistogram]);
  const playersDelayHistogramPoints = useMemo(() => {
    const selected = new Set((mainPlayersDelaySelectedCases || []).filter((value) => value && value !== 'all'));
    const useAll = selected.size === 0 || (mainPlayersDelaySelectedCases || []).includes('all');
    return (mainPlayersDelayHistogram?.players || [])
      .map((player) => {
        const caseAverages = Array.isArray(player?.case_averages) ? player.case_averages : [];
        const matched = caseAverages.filter((entry) => {
          const caseKey = String(entry?.case_key || '').trim();
          if (!caseKey) return false;
          if (useAll) return true;
          return selected.has(caseKey);
        });
        if (matched.length === 0) return null;
        const sampleCount = matched.reduce((sum, entry) => sum + (Number(entry?.sample_count || 0)), 0);
        if (sampleCount <= 0) return null;
        const weightedSum = matched.reduce((sum, entry) => (
          sum + (Number(entry?.average_delay_seconds || 0) * Number(entry?.sample_count || 0))
        ), 0);
        const avgDelay = weightedSum / sampleCount;
        return {
          value: avgDelay,
          label: String(player?.player_name || '').trim(),
          player_key: String(player?.player_key || '').trim(),
          sample_count: sampleCount,
        };
      })
      .filter((player) => player && Number.isFinite(player.value) && player.label);
  }, [mainPlayersDelayHistogram, mainPlayersDelaySelectedCases]);
  const mainPlayersDelayProcessed = useMemo(() => {
    const minSamples = Math.max(5, Number(mainPlayersDelayMinSamples) || 5);
    const filtered = playersDelayHistogramPoints
      .filter((player) => Number(player.sample_count || 0) >= minSamples)
      .map((player) => ({
        player_key: player.player_key,
        player_name: player.label,
        average_apm: player.value,
        games_played: player.sample_count,
      }));
    return buildHistogramSummaryFromPlayers(filtered);
  }, [playersDelayHistogramPoints, mainPlayersDelayMinSamples]);
  const playersCadenceHistogramPoints = useMemo(() => (
    (mainPlayersCadenceHistogram?.players || [])
      .map((player) => ({
        value: Number(player?.average_cadence_score),
        label: String(player?.player_name || '').trim(),
        player_key: String(player?.player_key || '').trim(),
        games_played: Number(player?.games_used || 0),
        average_rate_per_min: Number(player?.average_rate_per_min || 0),
        average_cv_gap: Number(player?.average_cv_gap || 0),
        average_burstiness: Number(player?.average_burstiness || 0),
        average_idle20_ratio: Number(player?.average_idle20_ratio || 0),
      }))
      .filter((player) => Number.isFinite(player.value) && player.label)
  ), [mainPlayersCadenceHistogram]);
  const mainPlayersCadenceProcessed = useMemo(() => {
    const minGames = Math.max(4, Number(mainPlayersCadenceMinGames) || 4);
    const filtered = playersCadenceHistogramPoints
      .filter((player) => Number(player.games_played || 0) >= minGames)
      .map((player) => ({
        player_key: player.player_key,
        player_name: player.label,
        average_apm: player.value,
        games_played: player.games_played,
        average_rate_per_min: player.average_rate_per_min,
        average_cv_gap: player.average_cv_gap,
        average_burstiness: player.average_burstiness,
        average_idle20_ratio: player.average_idle20_ratio,
      }));
    return buildHistogramSummaryFromPlayers(filtered);
  }, [playersCadenceHistogramPoints, mainPlayersCadenceMinGames]);
  const mainPlayersViewportProcessed = useMemo(() => {
    const minGames = Math.max(4, Number(mainPlayersViewportMinGames) || 4);
    const filtered = (mainPlayersViewportHistogram?.players || [])
      .filter((player) => Number(player?.games_played || 0) >= minGames)
      .map((player) => ({
        player_key: String(player?.player_key || '').trim(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.[VIEWPORT_SWITCH_RATE_CONFIG.playerField] || 0),
        games_played: Number(player?.games_played || 0),
        average_viewport_switch_rate: Number(player?.average_viewport_switch_rate || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0);
    return buildHistogramSummaryFromPlayers(filtered);
  }, [mainPlayersViewportHistogram, mainPlayersViewportMinGames]);
  const mainGameCadenceProcessed = useMemo(() => {
    const rows = (mainGame?.unit_production_cadence || [])
      .filter((player) => Boolean(player?.eligible))
      .map((player) => ({
        player_key: String(player?.player_key || '').trim(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.cadence_score || 0),
        games_played: Number(player?.units_produced || 0),
        average_rate_per_min: Number(player?.rate_per_minute || 0),
        average_cv_gap: Number(player?.cv_gap || 0),
        average_burstiness: Number(player?.burstiness || 0),
        average_idle20_ratio: Number(player?.idle20_ratio || 0),
        window_seconds: Number(player?.window_seconds || 0),
        gap_count: Number(player?.gap_count || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm > 0);
    return buildHistogramSummaryFromPlayers(rows);
  }, [mainGame]);
  const mainGameViewportProcessed = useMemo(() => {
    const rows = (mainGame?.viewport_multitasking || [])
      .filter((player) => Boolean(player?.eligible))
      .map((player) => ({
        player_key: String(player?.player_key || '').trim(),
        player_name: String(player?.player_name || '').trim(),
        average_apm: Number(player?.[VIEWPORT_SWITCH_RATE_CONFIG.gameField] || 0),
        games_played: 1,
        viewport_switch_rate: Number(player?.viewport_switch_rate || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0);
    return buildHistogramSummaryFromPlayers(rows);
  }, [mainGame]);
  const mainPlayersSortIndicator = (sortBy) => {
    if (mainPlayersSortBy !== sortBy) return '';
    return mainPlayersSortDir === 'asc' ? '↑' : '↓';
  };

  if (loading && !dashboard && activeView === 'dashboards') {
    return (
      <div className="app">
        <div className="loading">Loading dashboard...</div>
      </div>
    );
  }

  return (
    <div className="app">
      <div className="dashboard-container">
        <div className="workflow-nav workflow-nav-app">
          <div className="workflow-nav-group">
            <button type="button" className={`btn-manage ${activeView === 'games' ? 'workflow-nav-active' : ''}`} onClick={() => navigateMainView('games')}>Games</button>
            <button type="button" className={`btn-manage ${activeView === 'players' ? 'workflow-nav-active' : ''}`} onClick={() => navigateMainView('players')}>Players</button>
          </div>
          <div className="workflow-nav-group">
            <button
              type="button"
              onClick={() => {
                setGlobalReplayFilterError('');
                loadGlobalReplayFilterConfig().catch((err) => {
                  console.error('Failed to refresh global replay filter config:', err);
                });
                loadGlobalReplayFilterOptions().catch((err) => {
                  console.error('Failed to refresh global replay filter options:', err);
                });
                setShowGlobalReplayFilter(true);
              }}
              className="workflow-nav-text-action"
            >
              ⚙️ Settings
            </button>
            <button type="button" onClick={() => setShowIngestPanel(true)} className="workflow-nav-text-action">
              📥 Ingest
            </button>
          </div>
          <div className="workflow-nav-group">
            <button type="button" className={`btn-manage ${activeView === 'dashboards' ? 'workflow-nav-active' : ''}`} onClick={() => navigateMainView('dashboards')}>Custom Dashboards</button>
          </div>
        </div>

        {error && <div className="error-message">{error}</div>}

        {activeView === 'games' && (
          <div className="workflow-panel">
            <div className="workflow-summary-filter-row workflow-games-filter-row">
              <select
                className="workflow-summary-filter-select"
                value={mainGamesFilters.player[0] || ''}
                onChange={(e) => setMainGameSingleFilter('player', e.target.value)}
              >
                <option value="">Any player (5+ games)</option>
                {(mainGamesFilterOptions.players || []).map((option) => (
                  <option key={`wf-player-${option.key}`} value={option.key}>
                    {option.label} ({option.games})
                  </option>
                ))}
              </select>
              <select
                className="workflow-summary-filter-select"
                value={mainGamesFilters.map[0] || ''}
                onChange={(e) => setMainGameSingleFilter('map', e.target.value)}
              >
                <option value="">Any map (top 15)</option>
                {(mainGamesFilterOptions.maps || []).map((option) => (
                  <option key={`wf-map-${option.key}`} value={option.key}>
                    {option.label} ({option.games})
                  </option>
                ))}
              </select>
              <div className="workflow-pattern-pills workflow-games-filter-pills">
                {(mainGamesFilterOptions.durations || []).map((option) => {
                  const active = (mainGamesFilters.duration || []).includes(option.key);
                  return (
                    <button
                      key={`wf-duration-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleMainGameMultiFilter('duration', option.key)}
                    >
                      {option.label} ({option.games})
                    </button>
                  );
                })}
              </div>
              <div className="workflow-pattern-pills workflow-games-filter-pills">
                {(mainGamesFilterOptions.featuring || []).map((option) => {
                  const active = (mainGamesFilters.featuring || []).includes(option.key);
                  return (
                    <button
                      key={`wf-feature-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleMainGameMultiFilter('featuring', option.key)}
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
              <button type="button" className="btn-create-manual" onClick={clearMainGamesFilters}>Clear filters</button>
            </div>
            {mainGamesLoading ? (
              <div className="loading">Loading games...</div>
            ) : (
              <>
                <table className="data-table workflow-table">
                  <thead>
                    <tr>
                      <th>Played</th>
                      <th>Players</th>
                      <th>Map</th>
                      <th>Duration</th>
                      <th>Featuring</th>
                    </tr>
                  </thead>
                  <tbody>
                    {mainGames.map((game) => (
                      <tr key={game.replay_id} className={selectedReplayId === game.replay_id ? 'workflow-selected-row' : ''} onClick={() => openMainGame(game.replay_id)}>
                        <td>{formatRelativeReplayDate(game.replay_date)}</td>
                        <td>{renderMainGameListPlayers(game)}</td>
                        <td>{game.map_name}</td>
                        <td>{formatDuration(game.duration_seconds)}</td>
                        <td>
                          {(game.featuring || []).length === 0 ? (
                            <span className="workflow-empty-inline">-</span>
                          ) : (
                            <div className="workflow-pattern-pills">
                              {(game.featuring || []).map((pill) => (
                                <span key={`${game.replay_id}-${pill}`} className="workflow-pattern-pill workflow-feature-pill">
                                  <span>{pill}</span>
                                </span>
                              ))}
                            </div>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                <div className="workflow-pagination-row">
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={mainGamesPage <= 1 || mainGamesLoading}
                    onClick={() => setMainGamesPage((prev) => Math.max(1, prev - 1))}
                  >
                    Previous
                  </button>
                  <span>
                    Page {mainGamesPage} / {mainGamesTotalPages} - Showing {mainGamesFrom}-{mainGamesTo} of {mainGamesTotal}
                  </span>
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={mainGamesPage >= mainGamesTotalPages || mainGamesLoading}
                    onClick={() => setMainGamesPage((prev) => Math.min(mainGamesTotalPages, prev + 1))}
                  >
                    Next
                  </button>
                </div>
              </>
            )}
          </div>
        )}

        {activeView === 'players' && (
          <div className="workflow-panel">
            <div className="workflow-players-tab-stack">
              <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label="Players sections">
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'summary'}
                  className={`workflow-production-tab ${mainPlayersTab === 'summary' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('summary')}
                >
                  Summary
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'apm-histogram'}
                  className={`workflow-production-tab ${mainPlayersTab === 'apm-histogram' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('apm-histogram')}
                >
                  APM Histogram
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'first-unit-delay'}
                  className={`workflow-production-tab ${mainPlayersTab === 'first-unit-delay' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('first-unit-delay')}
                >
                  First Unit Delay
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'unit-production-cadence'}
                  className={`workflow-production-tab ${mainPlayersTab === 'unit-production-cadence' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('unit-production-cadence')}
                >
                  Unit Production Cadence
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'viewport-multitasking'}
                  className={`workflow-production-tab ${mainPlayersTab === 'viewport-multitasking' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('viewport-multitasking')}
                >
                  Viewport Multitasking
                </button>
              </div>
              {mainPlayersTab === 'unit-production-cadence' ? (
                <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                  {SKILL_PROXY_CADENCE_INFO_TEXT}
                </div>
              ) : null}
              {mainPlayersTab === 'viewport-multitasking' ? (
                <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                  {SKILL_PROXY_VIEWPORT_INFO_TEXT}
                </div>
              ) : null}
            </div>

            {mainPlayersTab === 'summary' ? (
              <>
                <div className="workflow-summary-filter-row workflow-games-filter-row">
                  <input
                    type="text"
                    className="workflow-summary-filter-input"
                    placeholder="Filter by player name..."
                    value={mainPlayersFilters.name}
                    onChange={(e) => setMainPlayersSingleFilter('name', e.target.value)}
                  />
                  <label className="workflow-summary-filter-check">
                    <input
                      type="checkbox"
                      checked={Boolean(mainPlayersFilters.onlyFivePlus)}
                      onChange={(e) => setMainPlayersSingleFilter('onlyFivePlus', e.target.checked)}
                    />
                    <span>Only 5+ games</span>
                  </label>
                  <div className="workflow-pattern-pills workflow-games-filter-pills">
                    {(mainPlayersFilterOptions.last_played || []).map((option) => {
                      const active = (mainPlayersFilters.lastPlayed || []).includes(option.key);
                      return (
                        <button
                          key={`wf-player-last-${option.key}`}
                          type="button"
                          className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                          onClick={() => toggleMainPlayersMultiFilter('lastPlayed', option.key)}
                        >
                          {option.label} ({option.count})
                        </button>
                      );
                    })}
                  </div>
                  <button type="button" className="btn-create-manual" onClick={clearMainPlayersFilters}>Clear filters</button>
                </div>
                {mainPlayersLoading ? (
                  <div className="loading">Loading players...</div>
                ) : (
                  <>
                    <table className="data-table workflow-table">
                      <thead>
                        <tr>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('name')}>Name {mainPlayersSortIndicator('name')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('race')}>Race {mainPlayersSortIndicator('race')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('games')}>Games {mainPlayersSortIndicator('games')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('apm')}>Avg APM {mainPlayersSortIndicator('apm')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('last_played')}>Last played {mainPlayersSortIndicator('last_played')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {mainPlayers.map((player) => (
                          <tr key={player.player_key} className={selectedPlayerKey === player.player_key ? 'workflow-selected-row' : ''} onClick={() => openMainPlayer(player.player_key)}>
                            <td style={playerAccentColor(player.player_key) ? { color: playerAccentColor(player.player_key), fontWeight: 600 } : undefined}>{player.player_name}</td>
                            <td>{player.race}</td>
                            <td>{player.games_played}</td>
                            <td>{Number(player.average_apm || 0).toFixed(1)}</td>
                            <td>{formatDaysAgoCompact(player.last_played_days_ago)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    <div className="workflow-pagination-row">
                      <button
                        type="button"
                        className="btn-switch"
                        disabled={mainPlayersPage <= 1 || mainPlayersLoading}
                        onClick={() => setMainPlayersPage((prev) => Math.max(1, prev - 1))}
                      >
                        Previous
                      </button>
                      <span>
                        Page {mainPlayersPage} / {mainPlayersTotalPages} - Showing {mainPlayersFrom}-{mainPlayersTo} of {mainPlayersTotal}
                      </span>
                      <button
                        type="button"
                        className="btn-switch"
                        disabled={mainPlayersPage >= mainPlayersTotalPages || mainPlayersLoading}
                        onClick={() => setMainPlayersPage((prev) => Math.min(mainPlayersTotalPages, prev + 1))}
                      >
                        Next
                      </button>
                    </div>
                  </>
                )}
              </>
            ) : mainPlayersTab === 'apm-histogram' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {mainPlayersApmHistogramLoading ? <div className="chart-empty">Loading APM histogram...</div> : null}
                {!mainPlayersApmHistogramLoading && mainPlayersApmHistogramError ? <div className="chart-empty">{mainPlayersApmHistogramError}</div> : null}
                {!mainPlayersApmHistogramLoading && !mainPlayersApmHistogramError && mainPlayersApmProcessed.points.length === 0 ? (
                  <div className="chart-empty">Not enough player data to render this histogram yet.</div>
                ) : null}
                {!mainPlayersApmHistogramLoading && !mainPlayersApmHistogramError && mainPlayersApmProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min games (post-process): {Math.max(5, Number(mainPlayersApmMinGames) || 5)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="5"
                        max={String(Math.max(5, Number(mainPlayersApmProcessed.maxGames) || 5))}
                        step="1"
                        value={String(Math.max(5, Number(mainPlayersApmMinGames) || 5))}
                        onChange={(e) => setMainPlayersApmMinGames(Math.max(5, Number(e.target.value) || 5))}
                      />
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersApmProcessed.bins,
                        x_axis_label: 'Average APM',
                        y_axis_label: 'Density',
                        mean: mainPlayersApmProcessed.mean,
                        stddev: mainPlayersApmProcessed.stddev,
                        chart_height: 620,
                        overlay_points: mainPlayersApmProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                        })),
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(mainPlayersApmProcessed.playersIncluded) || 0} players (>=${Math.max(5, Number(mainPlayersApmMinGames) || 5)} games). Mean ${Number(mainPlayersApmProcessed.mean || 0).toFixed(1)} APM, stddev ${Number(mainPlayersApmProcessed.stddev || 0).toFixed(1)}.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : mainPlayersTab === 'first-unit-delay' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {!mainPlayersDelayHistogramLoading && !mainPlayersDelayHistogramError ? (
                  <>
                    <div className="workflow-card-subtitle"><span>Included building to unit cases</span></div>
                    <div className="workflow-pattern-pills workflow-games-filter-pills">
                      <button
                        type="button"
                        className={`workflow-filter-pill ${(mainPlayersDelaySelectedCases || []).includes('all') ? 'workflow-filter-pill-active' : ''}`}
                        onClick={() => toggleMainPlayersDelayCase('all')}
                      >
                        All
                      </button>
                      {mainPlayersDelayCaseOptions.map((option) => {
                        const active = (mainPlayersDelaySelectedCases || []).includes(option.case_key);
                        return (
                          <button
                            key={`wf-delay-case-${option.case_key}`}
                            type="button"
                            className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                            onClick={() => toggleMainPlayersDelayCase(option.case_key)}
                          >
                            {`${option.building_name} -> ${option.unit_name} (${Number(option.sample_count || 0)})`}
                          </button>
                        );
                      })}
                    </div>
                  </>
                ) : null}
                {mainPlayersDelayHistogramLoading ? <div className="chart-empty">Loading first-unit delay...</div> : null}
                {!mainPlayersDelayHistogramLoading && mainPlayersDelayHistogramError ? <div className="chart-empty">{mainPlayersDelayHistogramError}</div> : null}
                {!mainPlayersDelayHistogramLoading && !mainPlayersDelayHistogramError && mainPlayersDelayProcessed.points.length === 0 ? (
                  <div className="chart-empty">
                    Not enough player delay samples to render this distribution yet.
                    {!(mainPlayersDelaySelectedCases || []).includes('all') ? (
                      <>
                        {' '}
                        <button
                          type="button"
                          className="workflow-link-btn"
                          onClick={() => setMainPlayersDelaySelectedCases(['all'])}
                        >
                          Clear case filters
                        </button>
                      </>
                    ) : null}
                  </div>
                ) : null}
                {!mainPlayersDelayHistogramLoading && !mainPlayersDelayHistogramError && mainPlayersDelayProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min samples (post-process): {Math.max(5, Number(mainPlayersDelayMinSamples) || 5)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="5"
                        max={String(Math.max(5, Number(mainPlayersDelayProcessed.maxGames) || 5))}
                        step="1"
                        value={String(Math.max(5, Number(mainPlayersDelayMinSamples) || 5))}
                        onChange={(e) => setMainPlayersDelayMinSamples(Math.max(5, Number(e.target.value) || 5))}
                      />
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersDelayProcessed.bins,
                        x_axis_label: 'Average delay (seconds)',
                        y_axis_label: 'Density',
                        overlay_value_label: 's delay',
                        overlay_count_label: 'samples',
                        mean: mainPlayersDelayProcessed.mean,
                        stddev: mainPlayersDelayProcessed.stddev,
                        chart_height: 620,
                        overlay_points: mainPlayersDelayProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                        })),
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(mainPlayersDelayProcessed.playersIncluded) || 0} players (>=${Math.max(5, Number(mainPlayersDelayMinSamples) || 5)} samples). Mean ${Number(mainPlayersDelayProcessed.mean || 0).toFixed(1)}s, stddev ${Number(mainPlayersDelayProcessed.stddev || 0).toFixed(1)}s.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : mainPlayersTab === 'unit-production-cadence' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {mainPlayersCadenceHistogramLoading ? <div className="chart-empty">Loading unit production cadence...</div> : null}
                {!mainPlayersCadenceHistogramLoading && mainPlayersCadenceHistogramError ? <div className="chart-empty">{mainPlayersCadenceHistogramError}</div> : null}
                {!mainPlayersCadenceHistogramLoading && !mainPlayersCadenceHistogramError && mainPlayersCadenceProcessed.points.length === 0 ? (
                  <div className="chart-empty">Not enough cadence data to render this distribution yet.</div>
                ) : null}
                {!mainPlayersCadenceHistogramLoading && !mainPlayersCadenceHistogramError && mainPlayersCadenceProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min games (post-process): {Math.max(4, Number(mainPlayersCadenceMinGames) || 4)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="4"
                        max={String(Math.max(4, Number(mainPlayersCadenceProcessed.maxGames) || 4))}
                        step="1"
                        value={String(Math.max(4, Number(mainPlayersCadenceMinGames) || 4))}
                        onChange={(e) => setMainPlayersCadenceMinGames(Math.max(4, Number(e.target.value) || 4))}
                      />
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersCadenceProcessed.bins,
                        x_axis_label: 'Average cadence score',
                        y_axis_label: 'Density',
                        overlay_value_label: 'cadence',
                        overlay_count_label: 'games',
                        mean: mainPlayersCadenceProcessed.mean,
                        stddev: mainPlayersCadenceProcessed.stddev,
                        chart_height: 620,
                        overlay_points: mainPlayersCadenceProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                          tooltip_lines: [
                            `${String(player.player_name || '')}`,
                            `Cadence score: ${Number(player.average_apm || 0).toFixed(3)}`,
                            `Rate per minute: ${Number(player.average_rate_per_min || 0).toFixed(2)}`,
                            `Gap CV: ${Number(player.average_cv_gap || 0).toFixed(2)}`,
                            `Burstiness: ${Number(player.average_burstiness || 0).toFixed(2)}`,
                            `Idle gap ratio (>=20s): ${(Number(player.average_idle20_ratio || 0) * 100).toFixed(1)}%`,
                            `Games used: ${Number(player.games_played || 0)}`,
                          ],
                        })),
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(mainPlayersCadenceProcessed.playersIncluded) || 0} players (>=${Math.max(4, Number(mainPlayersCadenceMinGames) || 4)} games). Mean ${Number(mainPlayersCadenceProcessed.mean || 0).toFixed(3)}, stddev ${Number(mainPlayersCadenceProcessed.stddev || 0).toFixed(3)}.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : mainPlayersTab === 'viewport-multitasking' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {mainPlayersViewportHistogramLoading ? <div className="chart-empty">Loading viewport multitasking...</div> : null}
                {!mainPlayersViewportHistogramLoading && mainPlayersViewportHistogramError ? <div className="chart-empty">{mainPlayersViewportHistogramError}</div> : null}
                {!mainPlayersViewportHistogramLoading && !mainPlayersViewportHistogramError && mainPlayersViewportProcessed.points.length === 0 ? (
                  <div className="chart-empty">Not enough viewport multitasking data to render this distribution yet.</div>
                ) : null}
                {!mainPlayersViewportHistogramLoading && !mainPlayersViewportHistogramError && mainPlayersViewportProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <div className="workflow-summary-filter-row workflow-slider-row">
                      <label className="workflow-summary-filter-check">
                        <span>Min games (post-process): {Math.max(4, Number(mainPlayersViewportMinGames) || 4)}</span>
                      </label>
                      <input
                        type="range"
                        className="workflow-slider-input"
                        min="4"
                        max={String(Math.max(4, Number(mainPlayersViewportProcessed.maxGames) || 4))}
                        step="1"
                        value={String(Math.max(4, Number(mainPlayersViewportMinGames) || 4))}
                        onChange={(e) => setMainPlayersViewportMinGames(Math.max(4, Number(e.target.value) || 4))}
                      />
                    </div>
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersViewportProcessed.bins,
                        x_axis_label: VIEWPORT_SWITCH_RATE_CONFIG.axisLabel,
                        y_axis_label: 'Density',
                        overlay_value_label: VIEWPORT_SWITCH_RATE_CONFIG.overlayValueLabel,
                        overlay_count_label: 'games',
                        mean: mainPlayersViewportProcessed.mean,
                        stddev: mainPlayersViewportProcessed.stddev,
                        chart_height: 620,
                        overlay_points: mainPlayersViewportProcessed.points.map((player) => ({
                          value: Number(player.average_apm || 0),
                          label: String(player.player_name || ''),
                          player_key: String(player.player_key || ''),
                          games_played: Number(player.games_played || 0),
                          tooltip_lines: [
                            `${String(player.player_name || '')}`,
                            `${VIEWPORT_SWITCH_RATE_CONFIG.title}: ${VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(player.average_apm)}`,
                            `Games used: ${Number(player.games_played || 0)}`,
                          ],
                        })),
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`Population shown: ${Number(mainPlayersViewportProcessed.playersIncluded) || 0} players (>=${Math.max(4, Number(mainPlayersViewportMinGames) || 4)} games after post-filter). Mean ${VIEWPORT_SWITCH_RATE_CONFIG.summaryFormatter(mainPlayersViewportProcessed.mean)}, stddev ${VIEWPORT_SWITCH_RATE_CONFIG.summaryFormatter(mainPlayersViewportProcessed.stddev)}.`}
                    </div>
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        )}

        {activeView === 'game' && (
          <div className="workflow-panel">
            {mainGameDetailLoading ? (
              <div className="loading">Loading game report...</div>
            ) : mainGame ? (
              <>
                <div className="workflow-title-row workflow-title-row--solo">
                  <h2 className="workflow-game-players-heading">{renderMainGameListPlayers(mainGame)}</h2>
                </div>
                <div className="workflow-meta workflow-meta--game-header">
                  <span>{formatRelativeReplayDate(mainGame.replay_date)}</span>
                  <span>{mainGame.map_name}</span>
                  <span>{formatDuration(mainGame.duration_seconds)}</span>
                  <button
                    type="button"
                    className="btn-switch btn-switch-see-replay workflow-meta-stage-btn"
                    disabled={mainGameSeeLoading}
                    title="Clones this replay into your configured replay ingestion folder as _watch_me.rep so you can easily find it within Starcraft."
                    onClick={copyMainGameToWatchMe}
                  >
                    {mainGameSeeLoading ? 'Copying…' : 'Stage watch replay'}
                  </button>
                </div>
                <div className="workflow-game-tab-stack">
                  <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label="Game report sections">
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'summary'}
                      className={`workflow-production-tab ${mainGameTab === 'summary' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('summary')}
                    >
                      Summary
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'events'}
                      className={`workflow-production-tab ${mainGameTab === 'events' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('events')}
                    >
                      Game Events
                    </button>
                    {Array.isArray(mainGame?.build_orders) && mainGame.build_orders.length > 0 ? (
                      <button
                        type="button"
                        role="tab"
                        aria-selected={mainGameTab === 'build-orders'}
                        className={`workflow-production-tab ${mainGameTab === 'build-orders' ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => setMainGameTab('build-orders')}
                      >
                        Build Orders
                      </button>
                    ) : null}
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'units'}
                      className={`workflow-production-tab ${mainGameTab === 'units' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('units')}
                    >
                      Units
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'timings'}
                      className={`workflow-production-tab ${mainGameTab === 'timings' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('timings')}
                    >
                      Timings
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={isMainGameSkillProxyTab(mainGameTab)}
                      className={`workflow-production-tab ${isMainGameSkillProxyTab(mainGameTab) ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => {
                        if (isMainGameSkillProxyTab(mainGameTab)) return;
                        setMainGameTab('first-unit-efficiency');
                      }}
                    >
                      Skill proxies
                    </button>
                  </div>
                  {isMainGameSkillProxyTab(mainGameTab) ? (
                    <div className="workflow-skill-proxy-subnav">
                      <div className="workflow-production-tabs workflow-skill-proxy-tabs" role="tablist" aria-label="Skill proxy views">
                        <button
                          type="button"
                          role="tab"
                          aria-selected={mainGameTab === 'first-unit-efficiency'}
                          className={`workflow-production-tab ${mainGameTab === 'first-unit-efficiency' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainGameTab('first-unit-efficiency')}
                        >
                          First unit efficiency
                        </button>
                        <button
                          type="button"
                          role="tab"
                          aria-selected={mainGameTab === 'unit-production-cadence'}
                          className={`workflow-production-tab ${mainGameTab === 'unit-production-cadence' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainGameTab('unit-production-cadence')}
                        >
                          Unit production cadence
                        </button>
                        <button
                          type="button"
                          role="tab"
                          aria-selected={mainGameTab === 'viewport-multitasking'}
                          className={`workflow-production-tab ${mainGameTab === 'viewport-multitasking' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainGameTab('viewport-multitasking')}
                        >
                          Viewport multitasking
                        </button>
                      </div>
                      {mainGameTab === 'unit-production-cadence' ? (
                        <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                          {SKILL_PROXY_CADENCE_INFO_TEXT}
                        </div>
                      ) : null}
                      {mainGameTab === 'viewport-multitasking' ? (
                        <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                          {SKILL_PROXY_VIEWPORT_INFO_TEXT}
                        </div>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                {mainGameSeeNotice ? (
                  <div className={`workflow-see-notice ${mainGameSeeNoticeError ? 'workflow-see-notice-error' : ''}`}>{mainGameSeeNotice}</div>
                ) : null}

                {mainGameTab === 'summary' && (
                  <>
                    <div className="workflow-summary-map-row">
                      <div className="workflow-summary-map-col">
                        {mainMapVisualAvailable ? (
                          <button
                            type="button"
                            className="workflow-map-thumb-btn workflow-map-thumb-btn--events-link"
                            onClick={() => setMainGameTab('events')}
                            title="Open Game Events"
                          >
                            <div className="workflow-map-thumb-btn-inner">
                              {renderSummaryMapStack({
                                legendItems: selectedMainGameLegend,
                                showLegend: false,
                                imageUrl: mainMapVisualThumbURL,
                                mapAlt: `${mainGame.map_name} map`,
                                bounds: mainEventMapBounds,
                                startPolygons: summaryMapStartPolygons,
                              })}
                              <span className="workflow-map-thumb-btn-hover-label" aria-hidden="true">Game Events</span>
                            </div>
                          </button>
                        ) : (
                          <div className="workflow-map-summary-fallback">
                            Map image unavailable for this replay map.
                            {mainMapVisual?.resolution_note ? ` (${mainMapVisual.resolution_note})` : ''}
                          </div>
                        )}
                      </div>
                      <div className="workflow-summary-features-col">
                        {mainGameFeaturingPillsList.length > 0 ? (
                          <>
                            <div className="workflow-summary-features-title">This game</div>
                            <div className="workflow-pattern-pills">
                              {mainGameFeaturingPillsList.map((pill) => renderFeaturingPill(pill, 'summary-game'))}
                            </div>
                          </>
                        ) : (
                          <div className="workflow-subtle-note">No featured highlights for this replay.</div>
                        )}
                      </div>
                    </div>
                    <div className="workflow-player-rows" style={{ '--workflow-player-name-width': `${mainPlayerNameWidthCh}ch` }}>
                      {(mainGame.players || []).map((player) => {
                        const raceIcon = getRaceIcon(player.race);
                        const gameSummaryParts = playerGameSummarySignalParts(player, mainGame?.game_events);
                        const trustGameEventsForDrops = Array.isArray(mainGame?.game_events) && mainGame.game_events.length > 0;
                        return (
                          <div key={player.player_id} className="workflow-player-row" style={{ borderLeft: `3px solid ${getTeamColor(player.team)}` }}>
                            <div className="workflow-player-line">
                              <div className="workflow-player-name workflow-player-name-block">
                                {raceIcon ? <img src={raceIcon} alt={player.race || 'race'} className="unit-icon-inline workflow-summary-race-icon" /> : null}
                                {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                                <button
                                  type="button"
                                  className="workflow-player-name-link"
                                  title="Analyze player"
                                  style={gamePlayerNameStyle(player)}
                                  onClick={() => openMainPlayer(player.player_key)}
                                >
                                  {player.name}
                                </button>
                              </div>
                              <div className="workflow-player-actions">
                                <span className="workflow-player-apm"><strong>APM</strong> {player.apm}</span>
                              </div>
                              <div className="workflow-pattern-pills">
                                {gameSummaryParts.positive.map(renderGameSummarySignalPill)}
                                {filterSummaryPillPatterns(player.detected_patterns, trustGameEventsForDrops).map((pattern, idx) => renderPatternPill(pattern, `player-${player.player_id}-${idx}`))}
                              </div>
                            </div>
                          </div>
                        );
                      })}
                    </div>
                  </>
                )}

                {mainGameTab === 'events' && (
                  <div className="workflow-card workflow-card-recent-games">
                    <div className="workflow-section-top-row">
                      <div className="workflow-events-filter-cluster workflow-events-topic-filters">
                        <label className={`workflow-summary-filter-check${!gameEventTopicAvailability.nuke ? ' workflow-summary-filter-check--disabled' : ''}`}>
                          <input
                            type="checkbox"
                            disabled={!gameEventTopicAvailability.nuke}
                            checked={mainSummaryFilters.nuke}
                            onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, nuke: e.target.checked }))}
                          />
                          nuke
                        </label>
                        <label className={`workflow-summary-filter-check${!gameEventTopicAvailability.drop ? ' workflow-summary-filter-check--disabled' : ''}`}>
                          <input
                            type="checkbox"
                            disabled={!gameEventTopicAvailability.drop}
                            checked={mainSummaryFilters.drop}
                            onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, drop: e.target.checked }))}
                          />
                          drop
                        </label>
                        <label className={`workflow-summary-filter-check${!gameEventTopicAvailability.recall ? ' workflow-summary-filter-check--disabled' : ''}`}>
                          <input
                            type="checkbox"
                            disabled={!gameEventTopicAvailability.recall}
                            checked={mainSummaryFilters.recall}
                            onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, recall: e.target.checked }))}
                          />
                          recall
                        </label>
                        <label className={`workflow-summary-filter-check${!gameEventTopicAvailability.scout ? ' workflow-summary-filter-check--disabled' : ''}`}>
                          <input
                            type="checkbox"
                            disabled={!gameEventTopicAvailability.scout}
                            checked={mainSummaryFilters.scout}
                            onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, scout: e.target.checked }))}
                          />
                          scout
                        </label>
                        <label className={`workflow-summary-filter-check${!gameEventTopicAvailability.becameRace ? ' workflow-summary-filter-check--disabled' : ''}`}>
                          <input
                            type="checkbox"
                            disabled={!gameEventTopicAvailability.becameRace}
                            checked={mainSummaryFilters.becameRace}
                            onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, becameRace: e.target.checked }))}
                          />
                          became race
                        </label>
                        <label className={`workflow-summary-filter-check${!gameEventTopicAvailability.rush ? ' workflow-summary-filter-check--disabled' : ''}`}>
                          <input
                            type="checkbox"
                            disabled={!gameEventTopicAvailability.rush}
                            checked={mainSummaryFilters.rush}
                            onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, rush: e.target.checked }))}
                          />
                          rush
                        </label>
                      </div>
                      <div className="workflow-section-warning">
                        ⚠️ Event narratives are derived from imperfect replay signals: expect some errors.
                      </div>
                    </div>
                    <div className="workflow-events-layout">
                        <div className="workflow-event-map-panel">
                          {mainMapVisualAvailable ? (
                            <>
                              {mainGamePlayers.length > 0 ? (
                                <div className="workflow-event-map-legend workflow-event-map-legend--filters" role="group" aria-label="Filter events by player">
                                  {mainGamePlayers.map((player) => (
                                    <label
                                      key={`event-filter-${player.player_id}`}
                                      className="workflow-event-legend-filter-item"
                                      style={legendTextStyle(player.color, playerColorToCss(player.color))}
                                    >
                                      <input
                                        type="checkbox"
                                        checked={mainEventsPlayerEnabledById[String(player.player_id)] !== false}
                                        onChange={(e) => setMainEventsPlayerEnabledById((prev) => ({
                                          ...prev,
                                          [String(player.player_id)]: e.target.checked,
                                        }))}
                                      />
                                      <span>{player.name}</span>
                                    </label>
                                  ))}
                                  <button
                                    type="button"
                                    className="workflow-legend-bulk-btn"
                                    onClick={() => setMainEventsPlayerEnabledById(
                                      Object.fromEntries(mainGamePlayers.map((p) => [String(p.player_id), false])),
                                    )}
                                  >
                                    None
                                  </button>
                                  <button
                                    type="button"
                                    className="workflow-legend-bulk-btn"
                                    onClick={() => setMainEventsPlayerEnabledById(
                                      Object.fromEntries(mainGamePlayers.map((p) => [String(p.player_id), true])),
                                    )}
                                  >
                                    All
                                  </button>
                                </div>
                              ) : null}
                              <div className="workflow-event-map-frame">
                                <img src={mainMapVisualURL} alt={`${mainGame.map_name} event overlay`} className="workflow-event-map-image" />
                                {selectedMainGameEvent ? (
                                  <svg className="workflow-event-map-overlay" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
                                    <defs>
                                      <marker
                                        id="workflow-event-arrowhead"
                                        markerWidth="5"
                                        markerHeight="5"
                                        refX="4.5"
                                        refY="2.5"
                                        orient="auto"
                                      >
                                        <polygon points="0 0, 5 2.5, 0 5" fill={selectedMainGameArrow?.color || 'currentColor'} />
                                      </marker>
                                    </defs>
                                    {effectiveMainGameOwnershipPolygons.map((overlay) => (
                                      <polygon
                                        key={overlay.key}
                                        points={overlay.points}
                                        className="workflow-event-map-base-polygon"
                                        style={{ fill: `${overlay.ownerColor}66`, stroke: overlay.ownerColor }}
                                      />
                                    ))}
                                    {effectiveMainGameArrow ? (
                                      <>
                                        <line
                                          x1={effectiveMainGameArrow.from.x}
                                          y1={effectiveMainGameArrow.from.y}
                                          x2={effectiveMainGameArrow.to.x}
                                          y2={effectiveMainGameArrow.to.y}
                                          className="workflow-event-map-attack-line"
                                          style={{ color: effectiveMainGameArrow.color, stroke: effectiveMainGameArrow.color }}
                                          markerEnd="url(#workflow-event-arrowhead)"
                                        />
                                      </>
                                    ) : null}
                                  </svg>
                                ) : null}
                                {effectiveMainGameArrow && selectedMainGameArrowUnits.length > 0 ? (
                                  <div
                                    className={`workflow-event-map-unit-overlay ${selectedMainGameArrowUnits.length > 2 ? 'workflow-event-map-unit-overlay--grid' : ''}`}
                                    style={{
                                      left: `${(effectiveMainGameArrow.from.x + effectiveMainGameArrow.to.x) / 2}%`,
                                      top: `${(effectiveMainGameArrow.from.y + effectiveMainGameArrow.to.y) / 2}%`,
                                    }}
                                  >
                                    {selectedMainGameArrowUnits.map((unit, unitIdx) => (
                                      <img
                                        key={`${selectedMainGameEventKeyResolved}-${unit.name}-${unitIdx}`}
                                        src={unit.icon}
                                        alt={unit.name}
                                        title={unit.name}
                                        className="workflow-event-map-unit-icon"
                                      />
                                    ))}
                                  </div>
                                ) : null}
                                {effectiveMainGameLeaveFlag ? (
                                  <div
                                    className="workflow-event-map-flag-overlay"
                                    style={{
                                      left: `${effectiveMainGameLeaveFlag.x}%`,
                                      top: `${effectiveMainGameLeaveFlag.y}%`,
                                    }}
                                    title="Player left the game"
                                  >
                                    <span role="img" aria-label="Player left">
                                      🏳️
                                    </span>
                                  </div>
                                ) : null}
                                {selectedMainGameExpansionOverlay ? (
                                  <img
                                    src={selectedMainGameExpansionOverlay.icon}
                                    alt="Expansion building"
                                    className="workflow-event-map-expansion-overlay"
                                    style={{
                                      left: `${selectedMainGameExpansionOverlay.point.x}%`,
                                      top: `${selectedMainGameExpansionOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                              </div>
                            </>
                          ) : (
                            <div className="workflow-map-summary-fallback">
                              Map image unavailable for event overlays.
                              {mainMapVisual?.resolution_note ? ` (${mainMapVisual.resolution_note})` : ''}
                            </div>
                          )}
                        </div>
                        <div className="workflow-events">
                          {filteredGameEvents.length > 0 ? (
                            filteredGameEvents.map((event) => {
                              const topicIndex = topicFilteredGameEvents.indexOf(event);
                              const eventKey = gameEventTopicKey(topicIndex);
                              const selected = eventKey === selectedMainGameEventKeyResolved;
                              return (
                                <button
                                  key={eventKey}
                                  type="button"
                                  className={`workflow-event-row ${selected ? 'workflow-event-row-selected' : ''}`}
                                  onClick={() => setMainSelectedGameEventKey(eventKey)}
                                >
                                  <span>{formatDuration(event.second)}</span>
                                  <span className="workflow-event-row-body">
                                    <span>{gameEventDescription(event)}</span>
                                  </span>
                                </button>
                              );
                            })
                          ) : (
                            <div className="chart-empty">No events match the current filters. Use All to show players again.</div>
                          )}
                        </div>
                      </div>
                  </div>
                )}

                {mainGameTab === 'units' && (
                  <div className="workflow-card workflow-card-chat-summary">
                    <div className="workflow-production-top-row">
                      <div className="workflow-production-tabs" role="tablist" aria-label="Production type tabs">
                        <button
                          className={`workflow-production-tab ${mainProductionTab === 'units' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainProductionTab('units')}
                          role="tab"
                          aria-selected={mainProductionTab === 'units'}
                        >
                          Units
                        </button>
                        <button
                          className={`workflow-production-tab ${mainProductionTab === 'buildings' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainProductionTab('buildings')}
                          role="tab"
                          aria-selected={mainProductionTab === 'buildings'}
                        >
                          Buildings
                        </button>
                      </div>
                      <div className="workflow-section-warning">
                        ⚠️ Replay commands contain significant false positives. Expect inflated numbers.
                      </div>
                    </div>
                    <div className="workflow-summary-filter-row">
                      {mainProductionTab === 'units' ? (
                        <>
                          <div className="workflow-radio-group">
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="all"
                                checked={mainUnitFilterMode === 'all'}
                                onChange={(e) => setMainUnitFilterMode(e.target.value)}
                              />
                              <span>All units</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="workers"
                                checked={mainUnitFilterMode === 'workers'}
                                onChange={(e) => setMainUnitFilterMode(e.target.value)}
                              />
                              <span>Workers only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="non-workers"
                                checked={mainUnitFilterMode === 'non-workers'}
                                onChange={(e) => setMainUnitFilterMode(e.target.value)}
                              />
                              <span>Non-workers only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="spellcasters"
                                checked={mainUnitFilterMode === 'spellcasters'}
                                onChange={(e) => setMainUnitFilterMode(e.target.value)}
                              />
                              <span>Spellcasters only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="tier-2"
                                checked={mainUnitFilterMode === 'tier-2'}
                                onChange={(e) => setMainUnitFilterMode(e.target.value)}
                              />
                              <span>Tier 2 only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-units-filter"
                                value="tier-3"
                                checked={mainUnitFilterMode === 'tier-3'}
                                onChange={(e) => setMainUnitFilterMode(e.target.value)}
                              />
                              <span>Tier 3 only</span>
                            </label>
                          </div>
                          <input
                            type="text"
                            className="workflow-summary-filter-input"
                            placeholder="Filter unit name..."
                            value={mainUnitNameFilter}
                            onChange={(e) => setMainUnitNameFilter(e.target.value)}
                          />
                        </>
                      ) : (
                        <>
                          <div className="workflow-radio-group">
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="all"
                                checked={mainBuildingFilterMode === 'all'}
                                onChange={(e) => setMainBuildingFilterMode(e.target.value)}
                              />
                              <span>All buildings</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="defenses"
                                checked={mainBuildingFilterMode === 'defenses'}
                                onChange={(e) => setMainBuildingFilterMode(e.target.value)}
                              />
                              <span>Defenses only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="tier-2"
                                checked={mainBuildingFilterMode === 'tier-2'}
                                onChange={(e) => setMainBuildingFilterMode(e.target.value)}
                              />
                              <span>Tier 2 only</span>
                            </label>
                            <label className="workflow-radio-option">
                              <input
                                type="radio"
                                name="workflow-buildings-filter"
                                value="tier-3"
                                checked={mainBuildingFilterMode === 'tier-3'}
                                onChange={(e) => setMainBuildingFilterMode(e.target.value)}
                              />
                              <span>Tier 3 only</span>
                            </label>
                          </div>
                          <input
                            type="text"
                            className="workflow-summary-filter-input"
                            placeholder="Filter building name..."
                            value={mainBuildingNameFilter}
                            onChange={(e) => setMainBuildingNameFilter(e.target.value)}
                          />
                        </>
                      )}
                    </div>
                    <div className="table-container">
                      <table className="data-table workflow-table workflow-production-table">
                        <thead>
                          <tr>
                            <th>Slice</th>
                            {mainGamePlayers.map((player) => (
                              <th
                                key={player.player_id}
                                style={hasTeamInfo ? { backgroundColor: teamColorRgba(player.team, 0.2) } : undefined}
                              >
                                {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                                {player.name}
                              </th>
                            ))}
                          </tr>
                        </thead>
                        <tbody>
                          {(mainGame.units_by_slice || []).map((slice) => (
                            <tr key={slice.slice_start_second}>
                              <td>{slice.slice_label}</td>
                              {mainGamePlayers.map((player) => {
                                const playerSlice = (slice.players || []).find((item) => item.player_id === player.player_id);
                                const filtered = filterProductionEntries(playerSlice?.units || [], mainProductionTab);
                                return (
                                  <td
                                    key={`${slice.slice_start_second}-${player.player_id}`}
                                    style={hasTeamInfo ? { backgroundColor: teamColorRgba(player.team, 0.08) } : undefined}
                                  >
                                    {filtered.length === 0 ? (
                                      <span className="workflow-empty-inline">-</span>
                                    ) : (
                                      <div className="workflow-unit-chips">
                                        {filtered.map((unit) => (
                                          <span key={`${player.player_id}-${unit.unit_type}`} className="workflow-unit-chip">
                                            {getUnitIcon(unit.unit_type) ? <img src={getUnitIcon(unit.unit_type)} alt={unit.unit_type} className="workflow-unit-chip-icon" /> : null}
                                            <strong className="workflow-unit-chip-count">x{unit.count}</strong>
                                          </span>
                                        ))}
                                      </div>
                                    )}
                                  </td>
                                );
                              })}
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  </div>
                )}

                {mainGameTab === 'timings' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-section-top-row">
                      <div className="workflow-production-tabs workflow-timing-tabs" role="tablist" aria-label="Timing category tabs">
                        {TIMING_CATEGORY_CONFIG.map((cfg) => (
                          <button
                            key={cfg.id}
                            className={`workflow-production-tab ${mainTimingCategory === cfg.id ? 'workflow-production-tab-active' : ''}`}
                            onClick={() => setMainTimingCategory(cfg.id)}
                            role="tab"
                            aria-selected={mainTimingCategory === cfg.id}
                          >
                            {cfg.label}
                          </button>
                        ))}
                      </div>
                      {mainTimingNotice ? (
                        <div className="workflow-section-warning">{mainTimingNotice}</div>
                      ) : null}
                    </div>
                    {mainTimingCategory === 'hp_upgrades' ? (
                      <>
                        {mainHpTimingByRace.map((raceChart) => (
                          <div key={`hp-${raceChart.race}`} className="workflow-card">
                            <div className="workflow-card-title"><span>{`${raceChart.raceLabel} HP upgrades timings`}</span></div>
                            <div className="workflow-radio-group">
                              {raceChart.labelOptions.map((labelName) => (
                                <label key={`${raceChart.race}-${labelName}`} className="workflow-radio-option">
                                  <input
                                    type="radio"
                                    name={`workflow-hp-filter-${raceChart.race}`}
                                    value={labelName}
                                    checked={raceChart.selected === labelName}
                                    onChange={(e) => setMainHpUpgradeFilters((prev) => ({ ...prev, [raceChart.race]: e.target.value }))}
                                  />
                                  <span>{labelName}</span>
                                </label>
                              ))}
                            </div>
                            <TimingScatterRows
                              title=""
                              series={raceChart.series}
                              durationSeconds={mainGame.duration_seconds}
                              colorByLabel={mainTimingUsesLabelColors}
                              showLegend={false}
                              markerMode={mainTimingCategoryConfig.markerMode || 'dot'}
                              axisMode={mainTimingAxisMode}
                              maxSecondOverride={mainTimingAxisTrimMaxSecond}
                              inlineLegend={true}
                              rowLabelMode="worker-icon"
                              rowGroupingMode="none"
                            />
                          </div>
                        ))}
                      </>
                    ) : (
                      <TimingScatterRows
                        title=""
                        series={mainTimingSeries}
                        durationSeconds={mainGame.duration_seconds}
                        colorByLabel={mainTimingUsesLabelColors}
                        showLegend={mainTimingUsesLabelColors && !mainTimingInlineLegend}
                        markerMode={mainTimingCategoryConfig.markerMode || 'dot'}
                        axisMode={mainTimingAxisMode}
                        maxSecondOverride={mainTimingAxisTrimMaxSecond}
                        inlineLegend={mainTimingInlineLegend}
                        noticeText=""
                        rowLabelMode={mainTimingInlineLegend ? 'worker-icon' : (['gas', 'expansion'].includes(mainTimingCategory) ? 'name-only' : 'race-suffix')}
                        rowGroupingMode={mainTimingInlineLegend ? 'race' : 'none'}
                      />
                    )}
                  </div>
                )}
                {mainGameTab === 'build-orders' && (
                  <div className="workflow-timing-charts">
                    {Array.isArray(mainGame?.build_orders) && mainGame.build_orders.length > 0 ? (
                      mainGame.build_orders.map((bo) => (
                        <BuildOrderTimelineRows
                          key={`build-order-${bo.player_id}-${bo.feature_key}`}
                          group={bo}
                        />
                      ))
                    ) : (
                      <div className="workflow-card">
                        <div className="chart-empty">No recognized build orders for this game.</div>
                      </div>
                    )}
                  </div>
                )}
                {mainGameTab === 'first-unit-efficiency' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-section-top-row">
                      <span className="workflow-section-top-spacer" aria-hidden="true" />
                      <div className="workflow-section-warning">
                        ⚠️ Worker travel starting build inflates these numbers.
                      </div>
                    </div>
                    {mainFirstUnitEfficiencyGroups.length > 0 ? (
                      mainFirstUnitEfficiencyGroups.map((groupEntry) => (
                        <FirstUnitEfficiencyTimelineRows
                          key={`first-unit-eff-${groupEntry.id}`}
                          group={groupEntry}
                        />
                      ))
                    ) : (
                      <div className="workflow-card">
                        <div className="chart-empty">No first unit efficiency rows found for this game.</div>
                      </div>
                    )}
                  </div>
                )}
                {mainGameTab === 'unit-production-cadence' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-card workflow-card-fingerprints">
                      {mainGameCadenceProcessed.points.length > 0 ? (
                        <Histogram
                          data={[]}
                          config={{
                            style: 'monobell_relax',
                            precomputed_bins: mainGameCadenceProcessed.bins,
                            x_axis_label: 'Cadence score',
                            y_axis_label: 'Density',
                            overlay_value_label: 'cadence',
                            overlay_count_label: 'units',
                            mean: mainGameCadenceProcessed.mean,
                            stddev: mainGameCadenceProcessed.stddev,
                            chart_height: 560,
                            overlay_points: mainGameCadenceProcessed.points.map((player) => ({
                              value: Number(player.average_apm || 0),
                              label: String(player.player_name || ''),
                              player_key: String(player.player_key || ''),
                              games_played: Number(player.games_played || 0),
                              tooltip_lines: [
                                `${String(player.player_name || '')}`,
                                `Cadence score: ${Number(player.average_apm || 0).toFixed(3)}`,
                                `Rate per minute: ${Number(player.average_rate_per_min || 0).toFixed(2)}`,
                                `Gap CV: ${Number(player.average_cv_gap || 0).toFixed(2)}`,
                                `Burstiness: ${Number(player.average_burstiness || 0).toFixed(2)}`,
                                `Idle gap ratio (>=20s): ${(Number(player.average_idle20_ratio || 0) * 100).toFixed(1)}%`,
                                `Units counted in window: ${Number(player.games_played || 0)}`,
                                `Window length: ${formatDuration(Number(player.window_seconds || 0))}`,
                              ],
                            })),
                          }}
                        />
                      ) : (
                        <div className="chart-empty">No eligible players for this game cadence window yet.</div>
                      )}
                      <div className="workflow-card-subtitle"><span>Per-player breakdown</span></div>
                      {(mainGame?.unit_production_cadence || []).map((entry) => (
                        <div key={`game-cadence-${entry.player_id}`} className="workflow-pattern-row">
                          <span style={playerAccentColor(entry.player_key) ? { color: playerAccentColor(entry.player_key), fontWeight: 600 } : undefined}>
                            {entry.is_winner ? '👑 ' : ''}{entry.player_name}
                          </span>
                          <span title={entry.eligible ? `rate=${Number(entry.rate_per_minute || 0).toFixed(2)}, cv=${Number(entry.cv_gap || 0).toFixed(2)}, burstiness=${Number(entry.burstiness || 0).toFixed(2)}, idle20=${(Number(entry.idle20_ratio || 0) * 100).toFixed(1)}%, units=${Number(entry.units_produced || 0)}, gaps=${Number(entry.gap_count || 0)}` : String(entry.ineligible_reason || '')}>
                            {entry.eligible
                              ? `${Number(entry.cadence_score || 0).toFixed(3)} cadence (${Number(entry.units_produced || 0)} units, ${formatDuration(Number(entry.window_seconds || 0))} window)`
                              : `N/A (${entry.ineligible_reason || 'insufficient data'})`}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                {mainGameTab === 'viewport-multitasking' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-card workflow-card-fingerprints">
                      {mainGameViewportProcessed.points.length > 0 ? (
                        <Histogram
                          data={[]}
                          config={{
                            style: 'monobell_relax',
                            precomputed_bins: mainGameViewportProcessed.bins,
                            x_axis_label: VIEWPORT_SWITCH_RATE_CONFIG.axisLabel,
                            y_axis_label: 'Density',
                            overlay_value_label: VIEWPORT_SWITCH_RATE_CONFIG.overlayValueLabel,
                            overlay_count_label: 'player',
                            mean: mainGameViewportProcessed.mean,
                            stddev: mainGameViewportProcessed.stddev,
                            chart_height: 560,
                            overlay_points: mainGameViewportProcessed.points.map((player) => ({
                              value: Number(player.average_apm || 0),
                              label: String(player.player_name || ''),
                              player_key: String(player.player_key || ''),
                              games_played: Number(player.games_played || 0),
                              tooltip_lines: [
                                `${String(player.player_name || '')}`,
                                `${VIEWPORT_SWITCH_RATE_CONFIG.title}: ${VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(player.average_apm)}`,
                              ],
                            })),
                          }}
                        />
                      ) : (
                        <div className="chart-empty">No eligible players for this game viewport multitasking window yet.</div>
                      )}
                      <div className="workflow-card-subtitle"><span>Per-player breakdown</span></div>
                      {(mainGame?.viewport_multitasking || []).map((entry) => (
                        <div key={`game-viewport-${entry.player_id}`} className="workflow-pattern-row">
                          <span style={playerAccentColor(entry.player_key) ? { color: playerAccentColor(entry.player_key), fontWeight: 600 } : undefined}>
                            {entry.is_winner ? '👑 ' : ''}{entry.player_name}
                          </span>
                          <span title={entry.eligible ? VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(entry.viewport_switch_rate) : String(entry.ineligible_reason || '')}>
                            {entry.eligible
                              ? VIEWPORT_SWITCH_RATE_CONFIG.valueFormatter(entry.viewport_switch_rate)
                              : `N/A (${entry.ineligible_reason || 'insufficient data'})`}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </>
            ) : (
              <div className="chart-empty">Select a game from the Games tab.</div>
            )}

            {mainGame && mainGameTab === 'summary' && (
              <>
                <form onSubmit={handleMainAsk} className="workflow-ask-form">
                  <input
                    className="widget-creation-input"
                    value={mainQuestion}
                    onChange={(e) => setMainQuestion(e.target.value)}
                    placeholder={openaiEnabled ? 'Ask AI about this game...' : 'Enable AI to ask questions'}
                    disabled={!openaiEnabled || mainAskLoading}
                  />
                  <button className="btn-create-ai" type="submit" disabled={!openaiEnabled || mainAskLoading || !mainQuestion.trim()}>
                    {mainAskLoading ? 'Asking...' : 'Ask AI'}
                  </button>
                </form>
                {renderMainAiResult()}
              </>
            )}
          </div>
        )}

        {activeView === 'player' && (
          <div className="workflow-panel">
            {mainPlayerLoading ? (
              <div className="loading">Loading player report...</div>
            ) : mainPlayer ? (
              <>
                <div className="workflow-title-row">
                  <div className="workflow-player-title-wrap">
                    <h2 style={playerAccentColor(mainPlayer.player_key) ? { color: playerAccentColor(mainPlayer.player_key) } : undefined}>{mainPlayer.player_name}</h2>
                    {(Number(mainPlayer.games_played) || 0) < 5 ? (
                      <span className="workflow-inline-warning">⚠️ Fewer than 5 replays: we cannot provide reliable player-level insights yet.</span>
                    ) : null}
                  </div>
                  <button type="button" className="btn-switch" onClick={goBackMainView}>Back</button>
                </div>
                <div className="workflow-meta">
                  <span><strong>Games</strong> {mainPlayer.games_played}</span>
                  <span><strong>Win rate</strong> {(mainPlayer.win_rate * 100).toFixed(1)}%</span>
                  <span><strong>APM</strong> {mainPlayer.average_apm?.toFixed(1)}</span>
                  <span><strong>EAPM</strong> {mainPlayer.average_eapm?.toFixed(1)}</span>
                </div>
                <div className="workflow-cards">
                  <div className="workflow-card workflow-card-race-behaviours">
                    {mainPlayerMetricsLoading ? <div className="chart-empty">Loading metrics...</div> : null}
                    {!mainPlayerMetricsLoading && mainPlayerMetricsError ? <div className="chart-empty">{mainPlayerMetricsError}</div> : null}
                    {!mainPlayerMetricsLoading && !mainPlayerMetricsError && (mainPlayerMetrics?.race_behaviour_sections || []).length === 0 ? (
                      <div className="chart-empty">No race behaviour sections available.</div>
                    ) : null}
                    {!mainPlayerMetricsLoading && !mainPlayerMetricsError && (mainPlayerMetrics?.race_behaviour_sections || []).map((section) => (
                      <div key={section.race} className="workflow-race-behaviour-section">
                        <div className="workflow-card-subtitle">
                          {getRaceIcon(section.race) ? <img src={getRaceIcon(section.race)} alt={section.race} className="unit-icon-inline workflow-race-title-icon" /> : null}
                          <span>{section.race}</span>
                        </div>
                        <div className="workflow-subtle-note">
                          {`${section.game_count} games (${((Number(section.game_rate) || 0) * 100).toFixed(1)}%), ${section.wins} wins, ${((Number(section.win_rate) || 0) * 100).toFixed(1)}% win rate`}
                        </div>
                        {(section.common_behaviours || []).length === 0 ? <div className="chart-empty">No common behaviours at 20%+ for this race.</div> : null}
                        {(section.common_behaviours || []).map((item, idx) => (
                          <div key={`${section.race}-${item.name}`} className="workflow-pattern-row">
                            <span>{renderPatternPill({ pattern_name: item.name, value: 'true' }, `player-common-${section.race}-${idx}`)}</span>
                            <span>{`${((Number(item.game_rate) || 0) * 100).toFixed(1)}% (${item.replay_count}/${section.game_count})`}</span>
                          </div>
                        ))}
                      </div>
                    ))}
                  </div>
                  <div className="workflow-card workflow-card-fingerprints">
                    <div className="workflow-card-title"><span>Population comparison</span></div>
                    {mainPlayerInsightLoading ? <div className="chart-empty">Loading population comparisons...</div> : null}
                    {!mainPlayerInsightLoading && mainPlayerInsightErrors.length > 0 ? (
                      <div className="chart-empty">{mainPlayerInsightErrors[0]}</div>
                    ) : null}
                    {!mainPlayerInsightLoading && mainPlayerInsightErrors.length === 0 ? (
                      <div className="workflow-insight-grid">
                        {mainPlayerInsights.map((insight) => {
                          const percentile = Number(insight.performance_percentile || 0);
                          const accent = insightScoreColor(percentile);
                          return (
                            <button
                              type="button"
                              key={insight.insight_type}
                              className="workflow-insight-card workflow-insight-card-link"
                              style={insight.eligible ? { borderColor: `${accent}55`, boxShadow: `inset 0 0 0 1px ${accent}22` } : undefined}
                              onClick={() => openMainPlayersSubview(playerInsightDestinationTab(insight.insight_type))}
                            >
                              <div className="workflow-insight-card-header">
                                <span>{insight.title}</span>
                              </div>
                              {insight.eligible ? (
                                <>
                                  <div className="workflow-insight-score-row">
                                    <span className="workflow-insight-score" style={{ color: accent }}>{insightSummaryLabel(percentile)}</span>
                                    <span className="workflow-insight-grade" style={{ backgroundColor: `${accent}22`, color: accent }}>{insightScoreLabel(percentile)}</span>
                                  </div>
                                  <div className="workflow-insight-value">{insight.player_value_label}</div>
                                  <div className="workflow-subtle-note">{`${insight.population_size} eligible players in population.`}</div>
                                </>
                              ) : (
                                <>
                                  <div className="workflow-insight-unavailable">Not enough data yet</div>
                                  <div className="workflow-subtle-note">{insight.ineligible_reason || 'This comparison is not available yet.'}</div>
                                </>
                              )}
                              <div className="workflow-insight-footer">
                                <span className="workflow-insight-link-hint">Open player population view</span>
                                <span className="workflow-insight-info-icon" aria-hidden="true">ⓘ</span>
                              </div>
                              <div className="workflow-insight-details">
                                <div className="workflow-subtle-note">{insight.description}</div>
                                <div className="workflow-insight-detail-list">
                                  {(insight.details || []).map((detail) => (
                                    <div key={`${insight.insight_type}-${detail.label}`} className="workflow-insight-detail-row">
                                      <span>{detail.label}</span>
                                      <span>{detail.value}</span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            </button>
                          );
                        })}
                      </div>
                    ) : null}
                    <div className="workflow-card-subtitle"><span>Usage signals</span></div>
                    {mainPlayerUsagePills.length === 0 ? (
                      <div className="workflow-subtle-note">No low-usage flags were triggered for hotkeys or queued orders.</div>
                    ) : (
                      <div className="workflow-pattern-pills">
                        {mainPlayerUsagePills.map((pill) => (
                          <span key={pill.key} className={pill.className} title={pill.title}>{pill.label}</span>
                        ))}
                      </div>
                    )}
                    <div className="workflow-card-subtitle">
                      <span>Distinctive outliers</span>
                      <HelpTooltip text={PLAYER_OUTLIER_HELP} label="Outlier calculation explanation" />
                    </div>
                    <div className="workflow-subtle-note">Same-race, human-only baselines. Items are shown in one list and prefixed by command family.</div>
                    {mainPlayerOutliersLoading ? <div className="chart-empty">Loading outliers...</div> : null}
                    {!mainPlayerOutliersLoading && mainPlayerOutliersError ? <div className="chart-empty">{mainPlayerOutliersError}</div> : null}
                    {!mainPlayerOutliersLoading && !mainPlayerOutliersError && mainPlayerOutlierItems.length === 0 ? (
                      <div className="chart-empty">No outliers crossed current thresholds.</div>
                    ) : null}
                    {!mainPlayerOutliersLoading && !mainPlayerOutliersError && mainPlayerOutlierItems.map((item) => (
                      <div key={`${item.category}-${item.race}-${item.name}`} className="workflow-pattern-row">
                        <span>{`${item.category}: ${item.pretty_name}`}</span>
                        <span className="workflow-outlier-expl">
                          <span className="workflow-outlier-rate">{`${((Number(item.player_rate) || 0) * 100).toFixed(0)}%`}</span>
                          <span>you</span>
                          <span>vs</span>
                          <span className="workflow-outlier-rate-muted">{`${((Number(item.baseline_rate) || 0) * 100).toFixed(0)}%`}</span>
                          <span>baseline</span>
                          {(item.qualified_by || []).map((qualifier) => (
                            <span key={`${item.name}-${qualifier}`} className={outlierQualifierClassName(qualifier)}>{qualifier}</span>
                          ))}
                        </span>
                      </div>
                    ))}
                  </div>
                  <div className="workflow-card workflow-card-recent-games">
                    <div className="workflow-card-title"><span>Recent games</span></div>
                    {mainPlayerRecentGamesLoading ? <div className="chart-empty">Loading recent games...</div> : null}
                    {!mainPlayerRecentGamesLoading && mainPlayerRecentGamesError ? <div className="chart-empty">{mainPlayerRecentGamesError}</div> : null}
                    {!mainPlayerRecentGamesLoading && !mainPlayerRecentGamesError && mainPlayerRecentGames.length === 0 ? (
                      <div className="chart-empty">No recent games found for this player.</div>
                    ) : null}
                    {!mainPlayerRecentGamesLoading && !mainPlayerRecentGamesError && mainPlayerRecentGames.slice(0, 6).map((g) => (
                      <button key={g.replay_id} className="workflow-recent-game-card" onClick={() => openMainGame(g.replay_id)}>
                        <div className="workflow-recent-game-header">
                          <span>{formatRelativeReplayDate(g.replay_date)}</span>
                          <span>{g.map_name}</span>
                          {g.current_player?.race ? (
                            <span className="workflow-recent-game-race">
                              {getRaceIcon(g.current_player.race) ? (
                                <img
                                  src={getRaceIcon(g.current_player.race)}
                                  alt={g.current_player.race}
                                  className="unit-icon-inline workflow-recent-game-race-icon"
                                />
                              ) : null}
                              <span>{g.current_player.race}</span>
                            </span>
                          ) : (
                            <span className="workflow-empty-inline">-</span>
                          )}
                          <span>{formatDuration(g.duration_seconds)}</span>
                        </div>
                        <div className="workflow-subtle-note">{renderPlayersMatchup(g.players_label || '')}</div>
                        <div className="workflow-recent-game-meta">
                          {g.current_player?.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                        </div>
                        {filterSummaryPillPatterns(g.current_player?.detected_patterns).length > 0 ? (
                          <div className="workflow-pattern-pills workflow-pattern-pills-compact">
                            {filterSummaryPillPatterns(g.current_player?.detected_patterns).map((pattern, idx) => renderPatternPill(pattern, `recent-${g.replay_id}-${idx}`))}
                          </div>
                        ) : null}
                      </button>
                    ))}
                  </div>
                  <div className="workflow-card workflow-card-chat-summary">
                    <div className="workflow-card-title"><span>Chat Summary</span></div>
                    {mainPlayerChatSummaryLoading ? <div className="chart-empty">Loading chat summary...</div> : null}
                    {!mainPlayerChatSummaryLoading && mainPlayerChatSummaryError ? <div className="chart-empty">{mainPlayerChatSummaryError}</div> : null}
                    {!mainPlayerChatSummaryLoading && !mainPlayerChatSummaryError && (Number(mainPlayerChatSummary?.total_messages) || 0) === 0 ? (
                      <div className="chart-empty">No chat messages found for this player in ingested games.</div>
                    ) : (
                      !mainPlayerChatSummaryLoading && !mainPlayerChatSummaryError && mainPlayerChatSummary ? (
                        <>
                          <div className="workflow-subtle-note">
                            {`${mainPlayerChatSummary?.total_messages || 0} messages across ${mainPlayerChatSummary?.games_with_chat || 0} games, ${mainPlayerChatSummary?.distinct_terms || 0} distinct terms after cleanup.`}
                          </div>
                          <div className="workflow-card-subtitle"><span>Top terms</span></div>
                          {(mainPlayerChatSummary?.top_terms || []).length === 0 ? (
                            <div className="chart-empty">Not enough messages to infer common terms.</div>
                          ) : (
                            <div className="workflow-pattern-pills">
                              {(mainPlayerChatSummary?.top_terms || []).map((item) => (
                                <span key={`player-chat-term-${item.term}`} className="workflow-pattern-pill">
                                  <span>{item.term}</span>
                                  <span>{`x${item.count}`}</span>
                                </span>
                              ))}
                            </div>
                          )}
                          <div className="workflow-card-subtitle"><span>Last 5 messages</span></div>
                          {(mainPlayerChatSummary?.example_messages || []).map((msg, idx) => (
                            <div key={`player-chat-example-${idx}`} className="workflow-event-row">
                              <span>{msg}</span>
                            </div>
                          ))}
                        </>
                      ) : null
                    )}
                  </div>
                </div>
              </>
            ) : (
              <div className="chart-empty">Select a player from a game report.</div>
            )}
            <form onSubmit={handleMainAsk} className="workflow-ask-form">
              <input
                className="widget-creation-input"
                value={mainQuestion}
                onChange={(e) => setMainQuestion(e.target.value)}
                placeholder={openaiEnabled ? 'Ask AI about this player...' : 'Enable AI to ask questions'}
                disabled={!openaiEnabled || mainAskLoading}
              />
              <button className="btn-create-ai" type="submit" disabled={!openaiEnabled || mainAskLoading || !mainQuestion.trim()}>
                {mainAskLoading ? 'Asking...' : 'Ask AI'}
              </button>
            </form>
            {renderMainAiResult()}
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

      {showGlobalReplayFilter && (
        <GlobalReplayFilterModal
          config={globalReplayFilterConfig}
          options={globalReplayFilterOptions}
          saving={globalReplayFilterSaving}
          error={globalReplayFilterError}
          onClose={() => setShowGlobalReplayFilter(false)}
          onSave={handleSaveGlobalReplayFilter}
          aliases={aliases}
          aliasesLoading={aliasesLoading}
          aliasesMessage={aliasesMessage}
          aliasesMessageIsError={aliasesMessageIsError}
          aliasForm={aliasForm}
          aliasSaving={aliasSaving}
          aliasSources={aliasSources}
          aliasEditOriginal={aliasEditOriginal}
          onAliasFormChange={setAliasForm}
          onAliasSave={handleAliasSave}
          onAliasDelete={handleAliasDelete}
          onAliasImportFile={handleAliasImportFile}
          onAliasSourcesToggle={handleAliasSourceToggle}
          onAliasEdit={handleAliasEdit}
          onAliasCancelEdit={handleAliasCancelEdit}
          onAliasExport={handleAliasExport}
        />
      )}

      {showIngestPanel && (
        <IngestModal
          ingestForm={ingestForm}
          ingestMessage={ingestMessage}
          ingestStatus={ingestStatus}
          ingestLogs={ingestLogs}
          ingestInputDir={ingestInputDir}
          ingestInputDirDirty={String(ingestInputDir || '').trim() !== String(savedIngestInputDir || '').trim()}
          ingestSettingsLoading={ingestSettingsLoading}
          ingestSettingsSaving={ingestSettingsSaving}
          ingestSocketState={ingestSocketState}
          onClose={() => {
            setShowIngestPanel(false);
            setIngestStatus('idle');
          }}
          onSubmit={handleIngestSubmit}
          onChange={setIngestForm}
          onInputDirChange={setIngestInputDir}
          onSaveInputDir={handleSaveIngestInputDir}
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

      {autoIngestNotice ? (
        <div className="ingest-toast">{autoIngestNotice}</div>
      ) : null}

      <div className="app-footer">
        <div className="footer-left">
          {replayCount !== null
            ? `${replayCount.toLocaleString()} replays in database. You can trigger an ingestion using the button above.`
            : 'Loading replay count...'}
        </div>
      </div>
    </div>
  );
}

export default App;
