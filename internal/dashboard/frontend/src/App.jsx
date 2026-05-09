import React, { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import { api } from './api';
import GlobalReplayFilterModal from './components/GlobalReplayFilterModal';
import IngestModal from './components/IngestModal';
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
import MutaliskTimingChart from './components/charts/MutaliskTimingChart';
import UnitProductionEarlyTimeline from './components/charts/UnitProductionEarlyTimeline';
import AllianceTimeline from './components/charts/AllianceTimeline';
import { getUnitIcon, getWorkerIconForRace, normalizeUnitName } from './lib/gameAssets';
import {
  PILL_SURFACES,
  useMarkerRegistry,
  renderPillText,
  pillClassName,
  lookupDefinitionForPattern,
  renderAggregatePillText,
} from './lib/markerRegistry';
import {
  CompositionPhasesRow,
  computeReplayAggregatePhases,
} from './lib/compositionPill';
import {
  getStoredAutoIngestSettings,
  saveAutoIngestSettings,
} from './lib/dashboardStorage';
import {
  formatDuration,
  formatMapNameWithKind,
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
  MAIN_PLAYER_TABS,
  MAIN_PLAYER_SKILL_PROXY_SUBTABS,
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

const cmpSemver = (a, b) => {
  const parse = (v) => String(v || '').replace(/^v/, '').split(/[.+-]/).slice(0, 3).map((n) => parseInt(n, 10) || 0);
  const [aMaj, aMin, aPat] = parse(a);
  const [bMaj, bMin, bPat] = parse(b);
  return (aMaj - bMaj) || (aMin - bMin) || (aPat - bPat);
};

const normalizeEventType = (eventType) => String(eventType || '').trim().toLowerCase();

/** Aligns with NeverUsedHotkeysPlayerDetector (7+ minute replays). */
const GAME_SUMMARY_NEGATION_MIN_SECONDS = 7 * 60;

const MAIN_GAME_SKILL_PROXY_TABS = ['first-unit-efficiency', 'unit-production-cadence', 'viewport-multitasking'];

const isMainGameSkillProxyTab = (tab) => MAIN_GAME_SKILL_PROXY_TABS.includes(tab);

const SKILL_PROXY_CADENCE_INFO_TEXT = 'ℹ️ How smoothly you keep adding army from the mid game on—not just how much, but how evenly you queue it. Formula: units/min ÷ (1 + gap CV).';

const SKILL_PROXY_VIEWPORT_INFO_TEXT = 'ℹ️ How many times a player switches between places on average per minute.';

const SKILL_PROXY_DELAY_INFO_TEXT = 'Average seconds from a production building becoming ready until the first matching unit command. Lower is better.';

// Per-insight short descriptions for the player Skill proxies > Summary cards.
// APM omitted intentionally (number is self-explanatory in that view).
const PLAYER_INSIGHT_DESCRIPTION_OVERRIDES = {
  apm: '',
  'first-unit-delay': SKILL_PROXY_DELAY_INFO_TEXT,
  'unit-production-cadence': 'How smoothly you keep adding army from the mid game on—not just how much, but how evenly you queue it. Formula: units/min ÷ (1 + gap CV).',
  'viewport-switch-rate': 'How many times a player switches between places on average per minute.',
};

const DROP_ACTOR_EVENT_TYPES = ['drop', 'reaver_drop', 'dt_drop', 'cliff_drop'];

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
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_gate'])) {
    positive.push({
      key: `ge-proxy-gate-${pid}`,
      icon: getUnitIcon('gateway'),
      label: 'Proxy gateway',
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_rax'])) {
    positive.push({
      key: `ge-proxy-rax-${pid}`,
      icon: getUnitIcon('barracks'),
      label: 'Proxy barracks',
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_factory'])) {
    positive.push({
      key: `ge-proxy-factory-${pid}`,
      icon: getUnitIcon('factory'),
      label: 'Proxy factory',
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

const gameEventDescription = (event, registry) => {
  const eventType = normalizeEventType(event?.type);
  const actor = String(event?.actor?.name || '').trim();
  const target = String(event?.target?.name || '').trim();
  const location = gameEventLocationLabel(event);

  if (typeof eventType === 'string' && eventType.startsWith('bo_')) {
    const def = registry?.[eventType];
    const boName = def?.name || prettyPatternName(eventType.replace(/^bo_/, ''));
    return actor ? `${actor} opens with ${boName}` : `Opens with ${boName}`;
  }

  if (eventType === 'player_start') {
    if (actor && location) return `${actor} starts at ${location}`;
    if (actor) return `${actor} starts`;
    return 'Player start';
  }
  if (eventType === 'leave_game') return actor ? `${actor} leaves the game` : 'Player leaves the game';
  if (eventType === 'player_stopped_playing') return actor ? `${actor} stops playing` : 'Player stops playing';
  if (eventType === 'late_alliance') {
    if (actor && target) return `${actor} allies with ${target}`;
    return actor ? `${actor} forms an alliance` : 'Alliance';
  }
  if (eventType === 'team_stacking_detected') return 'Team stacking detected';
  if (eventType === 'location_inactive') return location ? `Location inactive: ${location}` : 'Location inactive';
  if (eventType === 'expansion') {
    if (actor && isActorAtOwnNaturalBase(event)) return `${actor} expands to their natural`;
    return actor && location ? `${actor} expands to ${location}` : 'Expansion';
  }
  if (eventType === 'attack') return actor && target && location ? `${actor} attacks ${target} at ${location}` : 'Attack';
  if (eventType === 'scout') return actor && target && location ? `${actor} scouts ${target} at ${location}` : 'Scout';
  if (eventType === 'cliff_drop') {
    return actor && target && location ? `${actor} cliff drops ${target} at ${location}` : 'Cliff drop';
  }
  if (eventType === 'drop' || eventType === 'reaver_drop' || eventType === 'dt_drop') {
    return actor && target && location ? `${actor} drops on ${target} at ${location}` : 'Drop';
  }
  if (eventType === 'recall') {
    // CastRecall's X/Y is the *source* of the teleport (units pulled from
    // there to the Arbiter's location); the Arbiter's position — the actual
    // destination — isn't in the command stream. Be explicit so the reader
    // doesn't assume "at L" is where the units arrived.
    if (actor && location) return `${actor} recalls units from ${location} (destination unknown)`;
    if (actor) return `${actor} recalls units (destination unknown)`;
    return 'Recall';
  }
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
    const proxyKind = eventType === 'proxy_gate' ? 'gateway'
      : eventType === 'proxy_rax' ? 'barracks' : 'factory';
    if (actor && location) return `${actor} proxies ${proxyKind} at ${location}`;
    if (actor)             return `${actor} proxies ${proxyKind}`;
    if (location)          return `Proxy ${proxyKind} at ${location}`;
    return `Proxy ${proxyKind}`;
  }
  if (eventType === 'became_terran') return actor ? `${actor} became Terran` : 'Became Terran';
  if (eventType === 'became_zerg') return actor ? `${actor} became Zerg` : 'Became Zerg';
  if (eventType === 'mech_transition') return actor ? `${actor} transitions to Mech` : 'Mech transition';
  return prettyPatternName(event?.type || 'event');
};

const gamePlayerNameSpan = (player, key) => {
  const name = String(player?.name || '').trim();
  if (!name) return null;
  return (
    <span
      key={key}
      className="workflow-event-row-player"
      style={legendTextStyle(String(player?.color || ''), playerColorToCss(player?.color))}
    >
      {name}
    </span>
  );
};

// renderGameEventDescription returns the same sentence as gameEventDescription
// but with the actor and target wrapped in colored <span>s. Used for rendering
// the event-row body. The string variant remains for search + dedup keys.
const renderGameEventDescription = (event, registry) => {
  const eventType = normalizeEventType(event?.type);
  const actorName = String(event?.actor?.name || '').trim();
  const targetName = String(event?.target?.name || '').trim();
  const location = gameEventLocationLabel(event);
  const actorSpan = gamePlayerNameSpan(event?.actor, 'a');
  const targetSpan = gamePlayerNameSpan(event?.target, 't');

  if (typeof eventType === 'string' && eventType.startsWith('bo_')) {
    const def = registry?.[eventType];
    const boName = def?.name || prettyPatternName(eventType.replace(/^bo_/, ''));
    if (!actorName) return `Opens with ${boName}`;
    return <>{actorSpan} opens with {boName}</>;
  }

  if (eventType === 'player_start') {
    if (actorName && location) return <>{actorSpan} starts at {location}</>;
    if (actorName) return <>{actorSpan} starts</>;
    return 'Player start';
  }
  if (eventType === 'leave_game') return actorName ? <>{actorSpan} leaves the game</> : 'Player leaves the game';
  if (eventType === 'player_stopped_playing') return actorName ? <>{actorSpan} stops playing</> : 'Player stops playing';
  if (eventType === 'late_alliance') {
    if (actorName && targetName) return <>{actorSpan} allies with {targetSpan}</>;
    return actorName ? <>{actorSpan} forms an alliance</> : 'Alliance';
  }
  if (eventType === 'team_stacking_detected') return 'Team stacking detected';
  if (eventType === 'location_inactive') return location ? `Location inactive: ${location}` : 'Location inactive';
  if (eventType === 'expansion') {
    if (actorName && isActorAtOwnNaturalBase(event)) return <>{actorSpan} expands to their natural</>;
    return actorName && location ? <>{actorSpan} expands to {location}</> : 'Expansion';
  }
  if (eventType === 'attack') {
    return actorName && targetName && location
      ? <>{actorSpan} attacks {targetSpan} at {location}</>
      : 'Attack';
  }
  if (eventType === 'scout') {
    return actorName && targetName && location
      ? <>{actorSpan} scouts {targetSpan} at {location}</>
      : 'Scout';
  }
  if (eventType === 'cliff_drop') {
    return actorName && targetName && location
      ? <>{actorSpan} cliff drops {targetSpan} at {location}</>
      : 'Cliff drop';
  }
  if (eventType === 'drop' || eventType === 'reaver_drop' || eventType === 'dt_drop') {
    return actorName && targetName && location
      ? <>{actorSpan} drops on {targetSpan} at {location}</>
      : 'Drop';
  }
  if (eventType === 'recall') {
    if (actorName && location) return <>{actorSpan} recalls units from {location} (destination unknown)</>;
    if (actorName) return <>{actorSpan} recalls units (destination unknown)</>;
    return 'Recall';
  }
  if (eventType === 'nuke') {
    return actorName && targetName && location
      ? <>{actorSpan} nukes {targetSpan} at {location}</>
      : 'Nuke';
  }
  if (eventType === 'cannon_rush' || eventType === 'bunker_rush' || eventType === 'zergling_rush') {
    const rushKind = eventType === 'cannon_rush' ? 'cannon' : eventType === 'bunker_rush' ? 'bunker' : 'zergling';
    if (actorName && targetName) return <>{actorSpan} {rushKind} rushes {targetSpan}</>;
    if (actorName && location) return <>{actorSpan} {rushKind} rushes at {location}</>;
    if (actorName) return <>{actorSpan} {rushKind} rushes</>;
    return 'Rush';
  }
  if (eventType === 'takeover') {
    if (actorName && isActorAtOwnNaturalBase(event)) return <>{actorSpan} takes over their natural</>;
    return actorName && location ? <>{actorSpan} takes over {location}</> : 'Takeover';
  }
  if (eventType === 'proxy_gate' || eventType === 'proxy_rax' || eventType === 'proxy_factory') {
    const proxyKind = eventType === 'proxy_gate' ? 'gateway'
      : eventType === 'proxy_rax' ? 'barracks' : 'factory';
    if (actorName && location) return <>{actorSpan} proxies {proxyKind} at {location}</>;
    if (actorName)              return <>{actorSpan} proxies {proxyKind}</>;
    if (location)               return `Proxy ${proxyKind} at ${location}`;
    return `Proxy ${proxyKind}`;
  }
  if (eventType === 'became_terran') return actorName ? <>{actorSpan} became Terran</> : 'Became Terran';
  if (eventType === 'became_zerg') return actorName ? <>{actorSpan} became Zerg</> : 'Became Zerg';
  if (eventType === 'mech_transition') return actorName ? <>{actorSpan} transitions to Mech</> : 'Mech transition';
  return prettyPatternName(event?.type || 'event');
};

const gameEventSearchText = (event, registry) => {
  const parts = [
    gameEventDescription(event, registry),
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

// scPlayerColorMap is loaded once at app boot from /api/screp-colors and holds
// the engine's canonical name->#rrggbb mapping (keys normalized: lowercase,
// spaces stripped). Module-level because the helpers below are called from
// both module scope and component scope; React state (in the component) is
// what triggers re-render after this is populated.
let scPlayerColorMap = {};
const setScPlayerColorMapModule = (m) => {
  scPlayerColorMap = m && typeof m === 'object' ? m : {};
};

const playerColorToCss = (colorValue) => {
  const value = String(colorValue || '').trim();
  if (!value) return '#9ca3af';
  if (value.startsWith('#')) return value;
  const key = value.toLowerCase().replace(/\s+/g, '');
  return scPlayerColorMap[key] || value.toLowerCase();
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

// collectFeaturingKeysFromMainGame gathers the featuring chip keys present in
// the replay: narrative game_events (cannon_rush / bunker_rush / zergling_rush)
// by event_type; marker detections by event_type with a couple of aliases for
// composite chips ("mind_control" from became_terran/became_zerg, and the UI's
// short "recalls"/"nukes" labels).
const collectFeaturingKeysFromMainGame = (mainGame) => {
  // Returns { keys: Set<string>, rowByKey: Record<key, pattern row> }.
  // The row carries detected_second + payload so pill labels with
  // {minute}/{timestamp}/{subject} placeholders can interpolate properly.
  const keys = new Set();
  const rowByKey = {};
  const isMoney = String(mainGame?.map_kind || '') === 'Money';

  (mainGame?.game_events || []).forEach((ev) => {
    const t = normalizeEventType(ev?.type);
    if (t === 'zergling_rush')  keys.add('zergling_rush');
    if (t === 'cannon_rush')    keys.add('cannon_rush');
    if (t === 'bunker_rush')    keys.add('bunker_rush');
    if (t === 'proxy_gate')     keys.add('proxy_gate');
    if (t === 'proxy_rax')      keys.add('proxy_rax');
    if (t === 'proxy_factory')  keys.add('proxy_factory');
  });

  (mainGame?.players || []).forEach((p) => {
    (p.detected_patterns || []).forEach((pat) => {
      const key = pat?.event_type;
      if (!key) return;
      // Money maps suppress build-order chips on the replay-summary
      // featuring strip — opener timings on Big Game Hunters / Fastest
      // are uninformative. Per-player BO summary pills + the BO tab are
      // populated separately (player.detected_patterns + build_orders),
      // so they keep showing.
      if (isMoney && typeof key === 'string' && key.startsWith('bo_')) return;
      keys.add(key);
      if (!rowByKey[key]) rowByKey[key] = pat;
      if (key === 'became_terran' || key === 'became_zerg') keys.add('mind_control');
    });
  });

  return { keys, rowByKey };
};

// buildMainGameFeaturingPills produces the ordered pill list for the featuring
// strip. Ordering + game-event-only metadata (cannon_rush, bunker_rush,
// zergling_rush, mind_control) come from the backend-provided featuring_order
// and game_event_features lists. Marker pills come from the marker registry's
// games_list field; markers without one surface via a minimal fallback.
const buildMainGameFeaturingPills = (mainGame, markerDefs) => {
  if (!mainGame) return [];
  const { keys, rowByKey } = collectFeaturingKeysFromMainGame(mainGame);
  const registry = markerDefs?.markers || {};
  const order = Array.isArray(markerDefs?.featuring_order) ? markerDefs.featuring_order : [];
  const gameEventFeaturesByKey = {};
  (markerDefs?.game_event_features || []).forEach((f) => { gameEventFeaturesByKey[f.key] = f; });

  return order
    .filter((key) => keys.has(key))
    .map((key) => {
      const def = registry[key];
      if (def?.games_list) {
        // Resolve via renderPillText so {minute}/{timestamp}/{subject}
        // tokens in the games_list label/icon_key get interpolated against
        // the matching detected-pattern row (when one exists).
        const rendered = renderPillText(def, PILL_SURFACES.gamesList, rowByKey[key]);
        if (rendered) {
          return { key, label: rendered.label || def.name, iconKey: rendered.iconKey || '' };
        }
        return { key, label: def.games_list.label || def.name, iconKey: def.games_list.icon_key || '' };
      }
      const ge = gameEventFeaturesByKey[key];
      if (ge) return { key, label: ge.label, iconKey: ge.icon_key };
      return { key, label: def?.name || key, iconKey: '' };
    });
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

// polygonCenter returns the vertex-average center of a base polygon, which
// is visually closer to "the middle of the painted area" than the
// scmapanalyzer-provided base.center (biased toward mineral mass). Used for
// positioning the townhall overlay icon on expansion events.
const polygonCenter = (polygon) => {
  if (!Array.isArray(polygon) || polygon.length < 3) return null;
  let sumX = 0;
  let sumY = 0;
  let count = 0;
  polygon.forEach((p) => {
    const x = Number(p?.x);
    const y = Number(p?.y);
    if (Number.isFinite(x) && Number.isFinite(y)) {
      sumX += x;
      sumY += y;
      count += 1;
    }
  });
  if (count === 0) return null;
  return { x: sumX / count, y: sumY / count };
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

// Recall is intentionally absent: the cast's X/Y is the source area, not the
// Arbiter (destination), so a from→to arrow would draw a misleading vector.
// The Arbiter icon is rendered as a single-point overlay instead (see
// selectedMainGameRecallOverlay).
const isArrowEventType = (eventType) => ['attack', 'scout', 'drop', 'reaver_drop', 'dt_drop', 'cliff_drop', 'nuke', 'cannon_rush', 'bunker_rush', 'zergling_rush', 'proxy_gate', 'proxy_rax', 'proxy_factory'].includes(String(eventType || '').toLowerCase());

const fallbackOverlayUnitNamesForEvent = (eventType, actorRace) => {
  const normalized = normalizeEventType(eventType);
  if (normalized === 'zergling_rush') return ['zergling'];
  if (normalized === 'cannon_rush') return ['photoncannon'];
  if (normalized === 'bunker_rush') return ['bunker'];
  if (normalized === 'proxy_gate') return ['gateway'];
  if (normalized === 'proxy_rax') return ['barracks'];
  if (normalized === 'proxy_factory') return ['factory'];
  if (normalized === 'reaver_drop') return ['reaver'];
  if (normalized === 'dt_drop') return ['darktemplar'];
  // cliff_drop is a Terran-only marker classification, dropship is always correct.
  if (normalized === 'cliff_drop') return ['dropship'];
  if (normalized === 'drop') {
    const r = String(actorRace || '').toLowerCase();
    if (r === 'protoss') return ['shuttle'];
    if (r === 'zerg') return ['overlord'];
    return ['dropship'];
  }
  if (normalized === 'nuke') return ['ghost'];
  if (normalized === 'recall') return ['arbiter'];
  if (normalized === 'became_terran' || normalized === 'became_zerg') return ['darkarchon'];
  return [];
};

// gameEventRowIconEntries returns a list of inline icons to render alongside an
// event-row description. Mirrors the units rendered on the map overlay so the
// row carries the same visual signal (bunker-on-bunker-rush, arbiter-on-recall,
// race-correct townhall on expansions, etc.). The leave-game flag is returned
// as an emoji entry; everything else is a unit/building icon URL.
const gameEventRowIconEntries = (event, playerRaceByID, registry) => {
  if (!event) return [];
  const normalized = normalizeEventType(event?.type);
  const actorPid = Number(event?.actor?.player_id || 0);
  const actorRace = playerRaceByID && actorPid > 0 ? playerRaceByID.get(actorPid) : '';

  if (normalized.startsWith('bo_')) {
    const def = registry?.[normalized];
    const iconKey = def?.events_list?.icon_key
      || def?.games_list?.icon_key
      || def?.summary_player?.icon_key
      || '';
    if (!iconKey) return [];
    const icon = getUnitIcon(iconKey);
    if (!icon) return [];
    const label = def?.name || prettyPatternName(normalized.replace(/^bo_/, ''));
    return [{ src: icon, alt: label, title: label }];
  }
  if (normalized === 'leave_game') {
    return [{ emoji: '🏳️', alt: 'left the game', title: 'Player left the game' }];
  }
  if (normalized === 'player_stopped_playing') {
    return [{ emoji: '💤', alt: 'stopped playing', title: 'Player stopped playing (no Leave Game)' }];
  }
  if (normalized === 'late_alliance') {
    return [{ emoji: '🤝', alt: 'late alliance', title: 'Alliance formed after 10:00' }];
  }
  if (normalized === 'team_stacking_detected') {
    return [{ emoji: '😈', alt: 'team stacking', title: 'Stacking topology held >5 min' }];
  }
  if (normalized === 'expansion' || normalized === 'takeover') {
    const icon = getExpansionMarkerIconForRace(actorRace);
    if (!icon) return [];
    return [{ src: icon, alt: 'townhall', title: 'Expansion' }];
  }
  if (normalized === 'drop') {
    const icon = dropTransportIconForRace(actorRace);
    return icon ? [{ src: icon, alt: 'transport', title: 'Drop' }] : [];
  }

  const unitNames = Array.isArray(event?.attack_unit_types) && event.attack_unit_types.length > 0
    ? event.attack_unit_types
    : fallbackOverlayUnitNamesForEvent(event?.type, actorRace);
  const seen = new Set();
  const entries = [];
  for (const name of unitNames) {
    const icon = getUnitIcon(name);
    if (!icon) continue;
    if (seen.has(icon)) continue;
    seen.add(icon);
    entries.push({ src: icon, alt: name, title: name });
    if (entries.length >= 4) break;
  }
  return entries;
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

// prettyPatternName formats an event-type string (e.g. "zergling_rush") as a
// human-readable title ("Zergling Rush"). Used by the Game Events timeline to
// label entries whose event_type doesn't have a dedicated phrase.
const prettyPatternName = (patternName) => {
  const trimmed = String(patternName || '').trim();
  if (!trimmed) return '';
  const splitUppercase = trimmed.replace(/([a-z0-9])([A-Z])/g, '$1 $2');
  return splitUppercase
    .replace(/_/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase());
};

// shouldHidePatternFromSummaryPills suppresses markers the Summary row shouldn't
// render as pills even though the backend stored them. viewport_multitasking
// drives its own widget elsewhere; made_drops de-dupes against the narrative
// drop/reaver_drop/dt_drop game_events when the caller sets
// trustGameEventsForDrops (those drop-family events are already rendered as
// game-event pills and re-rendering the marker would double up the strip).
const shouldHidePatternFromSummaryPills = (pattern, trustGameEventsForDrops) => {
  const featureKey = pattern?.event_type;
  if (featureKey === 'viewport_multitasking') return true;
  if (trustGameEventsForDrops && featureKey === 'made_drops') return true;
  return false;
};

const filterSummaryPillPatterns = (patterns, trustGameEventsForDrops = false) => {
  const filtered = (patterns || []).filter((pattern) => !shouldHidePatternFromSummaryPills(pattern, trustGameEventsForDrops));
  // Stable-sort by detected_second so pills read chronologically (BO first,
  // then mid-game markers like SK Terran / Mech Transition). Patterns
  // without a detected_second (e.g. unit-composition rollups) sort to the
  // end. Hotkey markers carry an end-of-replay second already, so they
  // naturally land after timed markers.
  const indexed = filtered.map((pattern, idx) => ({ pattern, idx }));
  indexed.sort((a, b) => {
    const ta = Number.isFinite(a.pattern?.detected_second) ? a.pattern.detected_second : Number.POSITIVE_INFINITY;
    const tb = Number.isFinite(b.pattern?.detected_second) ? b.pattern.detected_second : Number.POSITIVE_INFINITY;
    if (ta !== tb) return ta - tb;
    return a.idx - b.idx;
  });
  return indexed.map((entry) => entry.pattern);
};

// renderPatternPill resolves a detected_patterns[] entry through the backend
// marker registry and builds a pill from the registered SummaryPlayer metadata.
// Returns null when the registry has no match or no SummaryPlayer pill.
// renderMatchupPatternSection renders a "Top build orders" / "Top markers"
// strip on a matchup or by-format card. Each entry uses the aggregate
// pill renderer (gamesList label preferred, else summaryPlayer with
// temporal placeholders stripped) so the labels read as static prose
// ("Recalls", "Threw Nukes", "Became Zerg") instead of the per-replay
// "Recalls at min N" / "Threw Nukes at N mins" form.
const renderAggregatePatternEntry = (entry, key, registry) => {
  const pattern = { event_type: entry.pattern_name };
  const def = lookupDefinitionForPattern(registry, pattern);
  const rendered = def ? renderAggregatePillText(def) : null;
  if (!rendered) {
    return (
      <span key={`${key}-fallback`} className="workflow-pattern-pill" title={entry.pattern_name}>
        {entry.pattern_name} <span className="workflow-pattern-count">×{entry.count}</span>
      </span>
    );
  }
  return (
    <span key={`${key}-wrap`} className="workflow-pattern-with-count">
      <span className={pillClassName(rendered.style)} title={rendered.title || undefined}>
        {rendered.icon ? <img src={rendered.icon} alt="" className="workflow-pattern-icon" /> : null}
        {rendered.label ? <span>{rendered.label}</span> : null}
      </span>
      <span className="workflow-pattern-count">×{entry.count}</span>
    </span>
  );
};

const renderMatchupPatternSection = (title, entries, keyPrefix, registry) => {
  const list = Array.isArray(entries) ? entries : [];
  if (list.length === 0) return null;
  return (
    <div className="workflow-player-matchup-section">
      <div className="workflow-player-matchup-section-title">{title}</div>
      <div className="workflow-pattern-pills workflow-pattern-pills-compact">
        {list.map((entry, idx) => renderAggregatePatternEntry(entry, `${keyPrefix}-${idx}`, registry))}
      </div>
    </div>
  );
};

const renderPatternPill = (pattern, keyPrefix, team, registry) => {
  if (!registry) return null;
  const def = lookupDefinitionForPattern(registry, pattern);
  if (!def) return null;
  const rendered = renderPillText(def, PILL_SURFACES.summaryPlayer, pattern);
  if (!rendered) return null;
  const className = pillClassName(rendered.style);
  const key = `${keyPrefix}-${team ? `team-${team}-` : ''}${pattern?.event_type || ''}-${pattern?.detected_second ?? ''}`;
  return (
    <span key={key} className={className} title={rendered.title || undefined}>
      {team !== undefined ? <span className="team-dot" style={{ backgroundColor: getTeamColor(team) }}></span> : null}
      {rendered.icon ? <img src={rendered.icon} alt="" className="workflow-pattern-icon" /> : null}
      {rendered.label ? <span>{rendered.label}</span> : null}
    </span>
  );
};

const formatSigned = (value) => {
  const n = Number(value) || 0;
  if (n > 0) return `+${n.toFixed(2)}`;
  return n.toFixed(2);
};

const PLAYER_INSIGHT_TYPES = {
  apm: 'apm',
  firstUnitDelay: 'first-unit-delay',
  unitProductionCadence: 'unit-production-cadence',
  viewportSwitchRate: 'viewport-switch-rate',
};

// PLAYER_SUMMARY_OUTLIER_CATEGORIES is the canonical list the FE iterates
// to fan out one HTTP request per category to /summary/outliers. Order
// here is just the request-firing order; render-time sort is by TF-IDF
// across all categories combined.
const PLAYER_SUMMARY_OUTLIER_CATEGORIES = ['Order', 'Build', 'Train', 'Morph', 'Tech', 'Upgrade'];

// PLAYER_SUMMARY_OUTLIER_PILL_CAP is how many pills the Summary tab will
// surface across all categories combined. Mirrors the cap the old
// monolithic backend computed; we apply it FE-side now since pills
// arrive incrementally.
const PLAYER_SUMMARY_OUTLIER_PILL_CAP = 12;

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

const HelpTooltip = ({ text, label }) => (
  <span className="workflow-help-wrap" aria-label={label || 'Explanation'}>
    <span className="workflow-metric-help">ⓘ</span>
    <span className="workflow-help-bubble">{text}</span>
  </span>
);

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

const MAIN_GAMES_PAGE_SIZE = 100;
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
  const markerRegistryState = useMarkerRegistry();
  const markerRegistry = markerRegistryState.markers;
  const markerDefinitions = markerRegistryState;
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showGlobalReplayFilter, setShowGlobalReplayFilter] = useState(false);
  const [openaiEnabled, setOpenaiEnabled] = useState(false);
  const [replayCount, setReplayCount] = useState(null);
  const [currentVersion, setCurrentVersion] = useState('');
  const [latestVersion, setLatestVersion] = useState('');
  const [latestVersionUrl, setLatestVersionUrl] = useState('');
  const emptyDbAutoOpenRef = useRef(false);
  const [globalReplayFilterConfig, setGlobalReplayFilterConfig] = useState(null);
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
  const [staleReplaysCount, setStaleReplaysCount] = useState(0);
  // Session-only dismissal of the stale-replays hint icon. Stored as the
  // count at the moment the user dismissed it; the icon reappears when a
  // larger stale count is detected (e.g. after a fresh ingest left some
  // older replays behind). sessionStorage = clears with the tab.
  const [dismissedStaleCount, setDismissedStaleCount] = useState(() => {
    try {
      const v = window.sessionStorage.getItem('dismissedStaleReplaysCount');
      return v == null ? 0 : Number(v) || 0;
    } catch (_) { return 0; }
  });
  const dismissStaleHint = useCallback(() => {
    try { window.sessionStorage.setItem('dismissedStaleReplaysCount', String(staleReplaysCount)); } catch (_) {}
    setDismissedStaleCount(staleReplaysCount);
  }, [staleReplaysCount]);
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
    matchups: [],
    map_kinds: [],
  });
  const [mainGamesFilters, setMainGamesFilters] = useState({
    player: [],
    map: [],
    duration: [],
    featuring: [],
    matchup: [],
    mapKind: [],
  });
  const [mainGamesShowBOFilters, setMainGamesShowBOFilters] = useState(false);
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
  // "Latest-ref" pattern: stable effects (WebSocket handler, auto-ingest interval,
  // ingest-poll tick) need to read the *current* games-list filter/page state and
  // call the *current* refresh function. Their dependency arrays intentionally
  // exclude these to avoid effect churn, so we mirror them into refs that are
  // re-assigned on every render.
  const refreshAfterIngestRef = useRef(null);
  // Auto-ingest fires every 60s; when it actually adds replays we only
  // want the game list to refresh — never the active game/player view,
  // never the player histograms, never the player overview. Earlier
  // wiring routed auto-ingest through refreshDataAfterGlobalReplayFilterSave
  // which reloads everything; that was correct for filter-save (filter
  // scope changed everywhere) but caused a full-UI blink every minute
  // for auto-ingest. Split the two paths.
  const refreshGamesAfterAutoIngestRef = useRef(null);
  // The ingest WebSocket handler emits a 'completed' status for EVERY
  // ingest run — manual button-press AND background auto-ingest tick.
  // We want the broad refresh only on manual: the auto-ingest poller
  // already does its own scoped refresh (game list only). This flag is
  // set by handleIngestSubmit before calling api.startIngest, and the
  // WebSocket 'completed' handler reads + clears it.
  const manualIngestInFlight = useRef(false);
  const mainGamesFiltersRef = useRef(null);
  const mainGamesPageRef = useRef(null);
  const [mainPlayer, setMainPlayer] = useState(null);
  const [mainPlayerRecentGames, setMainPlayerRecentGames] = useState([]);
  const [mainPlayerRecentGamesLoading, setMainPlayerRecentGamesLoading] = useState(false);
  const [mainPlayerRecentGamesError, setMainPlayerRecentGamesError] = useState('');
  const [mainPlayerChatSummary, setMainPlayerChatSummary] = useState(null);
  const [mainPlayerChatSummaryLoading, setMainPlayerChatSummaryLoading] = useState(false);
  const [mainPlayerChatSummaryError, setMainPlayerChatSummaryError] = useState('');
  const [mainPlayerShowLowConfidence, setMainPlayerShowLowConfidence] = useState(false);
  const [mainPlayerPerMatchup, setMainPlayerPerMatchup] = useState(null);
  const [mainPlayerPerMatchupLoading, setMainPlayerPerMatchupLoading] = useState(false);
  const [mainPlayerPerMatchupError, setMainPlayerPerMatchupError] = useState('');
  const [mainPlayerSpecial, setMainPlayerSpecial] = useState(null);
  const [mainPlayerSpecialLoading, setMainPlayerSpecialLoading] = useState(false);
  const [mainPlayerSpecialError, setMainPlayerSpecialError] = useState('');
  // Per-outlier-category state. Each category fires its own request so
  // pills stream into the UI as each finishes (instead of all-or-nothing
  // on a 60-90s monolithic /summary/special). Keyed by lowercase
  // category label ("order", "build", ...).
  const [mainPlayerSpecialOutliers, setMainPlayerSpecialOutliers] = useState({});
  const [mainPlayers, setMainPlayers] = useState([]);
  const [mainPlayersLoading, setMainPlayersLoading] = useState(false);
  const [mainPlayersPage, setMainPlayersPage] = useState(1);
  const [mainPlayersTotal, setMainPlayersTotal] = useState(0);
  const [mainPlayersSortBy, setMainPlayersSortBy] = useState('games');
  const [mainPlayersSortDir, setMainPlayersSortDir] = useState('desc');
  const [mainPlayersTab, setMainPlayersTab] = useState(() => initialMainRoute.playersTab);
  const [mainPlayerTab, setMainPlayerTab] = useState(() => initialMainRoute.playerTab);
  const [mainPlayerSubtab, setMainPlayerSubtab] = useState(() => initialMainRoute.playerSubtab || '');
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
  // Used purely as a re-render trigger after the screp engine color map loads;
  // the actual map lives at module scope (see scPlayerColorMap above) so the
  // module-level helpers (playerColorToCss, legendTextStyle) can consume it.
  const [, setScColorMapLoaded] = useState(false);
  const [mainSummaryFilters, setMainSummaryFilters] = useState(DEFAULT_SUMMARY_FILTERS);
  const [productionView, setProductionView] = useState('all');
  const [productionSubFilter, setProductionSubFilter] = useState('all');
  const [productionNameFilter, setProductionNameFilter] = useState('');
  const [mainTimingCategory, setMainTimingCategory] = useState('expansion');
  const [mainHpUpgradeFilters, setMainHpUpgradeFilters] = useState({
    terran: DEFAULT_HP_UPGRADE_BY_RACE.terran,
    zerg: DEFAULT_HP_UPGRADE_BY_RACE.zerg,
    protoss: DEFAULT_HP_UPGRADE_BY_RACE.protoss,
  });

  const loadGlobalReplayFilterConfig = async () => {
    const data = await api.getGlobalReplayFilter();
    setGlobalReplayFilterConfig(data);
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

  const loadScrepColors = async () => {
    try {
      const data = await api.getScrepColors();
      setScPlayerColorMapModule(data);
      setScColorMapLoaded(true);
    } catch (err) {
      console.error('Failed to load screp colors:', err);
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
      // Build Orders / Mutalisk Timing tabs are hidden when no data was
      // detected; don't leave the user stranded on an invisible tab.
      const hasBuildOrders = Array.isArray(data?.build_orders) && data.build_orders.length > 0;
      if (nextTab === 'build-orders' && !hasBuildOrders) {
        nextTab = 'summary';
      }
      const hasMutaliskTiming = Array.isArray(data?.mutalisk_timing_chart) && data.mutalisk_timing_chart.length > 0;
      if (nextTab === 'mutalisk-timing' && !hasMutaliskTiming) {
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
      setProductionView('all');
      setProductionSubFilter('all');
      setProductionNameFilter('');
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
      setMainGameSeeNotice('Copied to 000_screpdb_watch_me/watch_me.rep in your ingest folder.');
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

  const loadMainPlayerPerMatchup = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerPerMatchupLoading(true);
      setMainPlayerPerMatchupError('');
      const data = await api.getPlayerSummaryPerMatchup(normalizedPlayerKey);
      setMainPlayerPerMatchup(data);
    } catch (err) {
      setMainPlayerPerMatchupError(err.message || 'Failed to load per-matchup summary');
      setMainPlayerPerMatchup(null);
    } finally {
      setMainPlayerPerMatchupLoading(false);
    }
  };

  const loadMainPlayerSpecial = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerSpecialLoading(true);
      setMainPlayerSpecialError('');
      const data = await api.getPlayerSummarySpecial(normalizedPlayerKey);
      setMainPlayerSpecial(data);
    } catch (err) {
      setMainPlayerSpecialError(err.message || 'Failed to load player highlights');
      setMainPlayerSpecial(null);
    } finally {
      setMainPlayerSpecialLoading(false);
    }
    // Fan out per-category outlier fetches in parallel. Each settles
    // independently so the FE can render its pills as soon as that
    // category's queries return — much better UX than waiting 60-90s
    // for the previous monolithic endpoint.
    PLAYER_SUMMARY_OUTLIER_CATEGORIES.forEach((category) => {
      const key = category.toLowerCase();
      setMainPlayerSpecialOutliers((prev) => ({
        ...prev,
        [key]: { loading: true, error: '', pills: [] },
      }));
      api.getPlayerSummaryOutliers(normalizedPlayerKey, category)
        .then((data) => {
          setMainPlayerSpecialOutliers((prev) => ({
            ...prev,
            [key]: { loading: false, error: '', pills: Array.isArray(data?.pills) ? data.pills : [] },
          }));
        })
        .catch((err) => {
          setMainPlayerSpecialOutliers((prev) => ({
            ...prev,
            [key]: { loading: false, error: err.message || 'Failed to load outliers', pills: [] },
          }));
        });
    });
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

  const openMainPlayer = async (playerKey, options = {}) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    // Navigate first, fetch second. Previously the player overview fetch
    // (~10s on large corpora) blocked navigation, so clicking a player
    // produced a long blank gap before the page rendered. Now we set
    // state and route immediately — the page renders its skeleton
    // (matchups & format card grid via /summary/per-matchup, special
    // pills via /summary/special) while the overview backfills in
    // parallel. Each section has its own loading state already.
    setError(null);
    setMainPlayer(null);
    setMainPlayerLoading(true);
    setMainPlayerRecentGames([]);
    setMainPlayerRecentGamesError('');
    setMainPlayerRecentGamesLoading(false);
    setMainPlayerChatSummary(null);
    setMainPlayerChatSummaryError('');
    setMainPlayerChatSummaryLoading(false);
    setMainPlayerPerMatchup(null);
    setMainPlayerPerMatchupError('');
    setMainPlayerPerMatchupLoading(false);
    setMainPlayerShowLowConfidence(false);
    setMainPlayerSpecial(null);
    setMainPlayerSpecialError('');
    setMainPlayerSpecialLoading(false);
    setMainPlayerSpecialOutliers({});
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
    const wantTab = options.initialPlayerTab;
    const nextTab = wantTab && MAIN_PLAYER_TABS.includes(String(wantTab).trim().toLowerCase())
      ? String(wantTab).trim().toLowerCase()
      : 'summary';
    setMainPlayerTab(nextTab);
    const wantSubtab = String(options.initialPlayerSubtab || '').trim().toLowerCase();
    if (nextTab === 'skill-proxies') {
      setMainPlayerSubtab(MAIN_PLAYER_SKILL_PROXY_SUBTABS.includes(wantSubtab) ? wantSubtab : 'summary');
    } else if (nextTab === 'summary') {
      // Race subtab is dynamic; persist if provided, else resolved at render from race_breakdown.
      setMainPlayerSubtab(wantSubtab);
    } else {
      setMainPlayerSubtab('');
    }
    navigateMainView('player');
    // Background-fetch the overview without blocking navigation.
    api.getPlayer(playerKey)
      .then((data) => setMainPlayer(data))
      .catch((err) => setError(err.message))
      .finally(() => setMainPlayerLoading(false));
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

  useEffect(() => {
    setLoading(false);
    loadGlobalReplayFilterConfig().catch((err) => {
      console.error('Failed to load global replay filter config:', err);
    });
    loadTopPlayerColors();
    loadScrepColors();
    checkOpenAIStatus();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount-only.
  }, []);

  useEffect(() => {
    if (initialMainRoute.view === 'game' && initialMainRoute.replayId != null) {
      void openMainGame(initialMainRoute.replayId, { initialGameTab: initialMainRoute.gameTab });
    } else if (initialMainRoute.view === 'player' && initialMainRoute.playerKey) {
      void openMainPlayer(initialMainRoute.playerKey, {
        initialPlayerTab: initialMainRoute.playerTab,
        initialPlayerSubtab: initialMainRoute.playerSubtab,
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one-time hydration from initial URL.
  }, []);

  useEffect(() => {
    if (!currentVersion || currentVersion === 'dev') return undefined;
    let cancelled = false;
    (async () => {
      try {
        const response = await fetch('https://api.github.com/repos/marianogappa/screpdb/releases/latest');
        if (!response.ok) return;
        const release = await response.json();
        if (cancelled) return;
        const tag = String(release?.tag_name || '');
        if (!tag) return;
        if (cmpSemver(tag, currentVersion) > 0) {
          setLatestVersion(tag);
          setLatestVersionUrl(String(release?.html_url || ''));
        }
      } catch (_err) {
        // Silently ignore — offline, rate-limited, etc. Banner just stays hidden.
      }
    })();
    return () => { cancelled = true; };
  }, [currentVersion]);

  const refreshStaleReplaysCount = useCallback(async () => {
    try {
      const data = await api.getStaleReplaysCount();
      const next = Number(data?.count || 0);
      setStaleReplaysCount(next);
    } catch (err) {
      // Surface nothing to the user — banner just stays hidden if the lookup fails.
      console.error('Failed to load stale replays count:', err);
    }
  }, []);

  useEffect(() => {
    void refreshStaleReplaysCount();
  }, [refreshStaleReplaysCount]);

  useEffect(() => {
    if (ingestStatus === 'completed' || ingestStatus === 'failed' || ingestStatus === 'idle') {
      void refreshStaleReplaysCount();
    }
  }, [ingestStatus, refreshStaleReplaysCount]);


  useEffect(() => {
    if (ingestStatus !== 'running') return undefined;
    let cancelled = false;
    let lastCount = replayCount ?? 0;
    const tick = async () => {
      if (cancelled) return;
      try {
        const data = await api.getHealth();
        const next = Number(data?.total_replays || 0);
        if (next !== lastCount) {
          lastCount = next;
          setReplayCount(next);
          if (activeView === 'games') {
            // Read current filters/page via refs — closure-captured state would
            // be stale (the effect doesn't re-run when filters change), and
            // calling loadMainGames with stale filters silently reverts the
            // visible list to "no filters" while the filter pills stay active.
            await loadMainGames({ page: mainGamesPageRef.current, filters: mainGamesFiltersRef.current });
          }
        }
      } catch (err) {
        console.error('Failed to poll during ingest:', err);
      }
    };
    const timer = window.setInterval(tick, 5000);
    return () => { cancelled = true; window.clearInterval(timer); };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentionally re-runs only on status/view change; latest filters/page are read via refs (mainGamesFiltersRef, mainGamesPageRef) inside the tick.
  }, [ingestStatus, activeView]);

  useEffect(() => {
    if (suppressUrlSyncRef.current) return;
    const next = buildMainRouteSearch({
      activeView,
      selectedReplayId,
      selectedPlayerKey,
      mainGameTab,
      mainPlayersTab,
      mainPlayerTab,
      mainPlayerSubtab,
    });
    if (typeof window !== 'undefined' && mainRouteSnapshotEqual(window.location.search, next && next.length ? `?${next}` : '')) {
      return;
    }
    if (typeof window === 'undefined') return;
    window.history.pushState({ __spa: 1 }, '', mainRouteHref(next));
  }, [activeView, selectedReplayId, selectedPlayerKey, mainGameTab, mainPlayersTab, mainPlayerTab, mainPlayerSubtab]);

  useEffect(() => {
    const onPopState = () => {
      suppressUrlSyncRef.current = true;
      const r = parseMainRouteSearch(window.location.search);
      setActiveView(r.view);
      setSelectedReplayId(r.replayId);
      setSelectedPlayerKey(r.playerKey || '');
      setMainGameTab(r.gameTab);
      setMainPlayersTab(r.playersTab);
      setMainPlayerTab(r.playerTab);
      setMainPlayerSubtab(r.playerSubtab || '');
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
        const p = openMainPlayerRef.current?.(r.playerKey, {
          initialPlayerTab: r.playerTab,
          initialPlayerSubtab: r.playerSubtab,
        });
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
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'skill-proxies' || mainPlayerSubtab !== 'summary') return;
    if (!mainPlayerApmInsight && !mainPlayerApmInsightLoading && !mainPlayerApmInsightError) {
      loadMainPlayerApmInsight(selectedPlayerKey);
    }
    if (!mainPlayerDelayInsight && !mainPlayerDelayInsightLoading && !mainPlayerDelayInsightError) {
      loadMainPlayerDelayInsight(selectedPlayerKey);
    }
    if (!mainPlayerCadenceInsight && !mainPlayerCadenceInsightLoading && !mainPlayerCadenceInsightError) {
      loadMainPlayerCadenceInsight(selectedPlayerKey);
    }
    if (!mainPlayerViewportInsight && !mainPlayerViewportInsightLoading && !mainPlayerViewportInsightError) {
      loadMainPlayerViewportInsight(selectedPlayerKey);
    }
  }, [
    activeView, selectedPlayerKey, mainPlayerTab, mainPlayerSubtab,
    mainPlayerApmInsight, mainPlayerApmInsightLoading, mainPlayerApmInsightError,
    mainPlayerDelayInsight, mainPlayerDelayInsightLoading, mainPlayerDelayInsightError,
    mainPlayerCadenceInsight, mainPlayerCadenceInsightLoading, mainPlayerCadenceInsightError,
    mainPlayerViewportInsight, mainPlayerViewportInsightLoading, mainPlayerViewportInsightError,
  ]);

  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'recent-games') return;
    if (!mainPlayerRecentGames.length && !mainPlayerRecentGamesLoading && !mainPlayerRecentGamesError) {
      loadMainPlayerRecentGames(selectedPlayerKey);
    }
  }, [activeView, selectedPlayerKey, mainPlayerTab, mainPlayerRecentGames, mainPlayerRecentGamesLoading, mainPlayerRecentGamesError]);

  // Summary tab: fire the cheap per-matchup fetch first; only fire the
  // (expensive) /special pills endpoint after per-matchup resolves so the
  // two heavy aggregate queries don't contend on the single SQLite read
  // connection. Sequential firing keeps the per-card cards visible
  // quickly while the slower outlier-pill computation finishes in the
  // background.
  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'summary') return;
    if (!mainPlayerPerMatchup && !mainPlayerPerMatchupLoading && !mainPlayerPerMatchupError) {
      loadMainPlayerPerMatchup(selectedPlayerKey);
    }
  }, [
    activeView, selectedPlayerKey, mainPlayerTab,
    mainPlayerPerMatchup, mainPlayerPerMatchupLoading, mainPlayerPerMatchupError,
  ]);

  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'summary') return;
    // Wait for per-matchup to resolve (success or error) before firing the
    // slower /special endpoint. Both surfaces use the same single-conn
    // SQLite reader; running them sequentially halves total wall time on
    // large corpora.
    if (mainPlayerPerMatchupLoading) return;
    if (!mainPlayerPerMatchup && !mainPlayerPerMatchupError) return;
    if (!mainPlayerSpecial && !mainPlayerSpecialLoading && !mainPlayerSpecialError) {
      loadMainPlayerSpecial(selectedPlayerKey);
    }
  }, [
    activeView, selectedPlayerKey, mainPlayerTab,
    mainPlayerPerMatchup, mainPlayerPerMatchupLoading, mainPlayerPerMatchupError,
    mainPlayerSpecial, mainPlayerSpecialLoading, mainPlayerSpecialError,
  ]);

  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'chat-summary') return;
    if (!mainPlayerChatSummary && !mainPlayerChatSummaryLoading && !mainPlayerChatSummaryError) {
      loadMainPlayerChatSummary(selectedPlayerKey);
    }
  }, [activeView, selectedPlayerKey, mainPlayerTab, mainPlayerChatSummary, mainPlayerChatSummaryLoading, mainPlayerChatSummaryError]);

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
    let unmounted = false;
    let reconnectTimer = null;
    let reconnectAttempt = 0;
    let socket = null;

    const connect = () => {
      if (unmounted) return;
      setIngestSocketState('connecting');
      socket = api.createIngestLogsSocket();
      ingestSocketRef.current = socket;

      socket.onopen = () => {
        reconnectAttempt = 0;
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
              manualIngestInFlight.current = false;
            } else if (message.status === 'running') {
              setIngestMessage('');
            } else if (message.status === 'completed') {
              setIngestMessage('Ingestion completed.');
              // Auto-ingest fires the same 'completed' status every 60s
              // and was the source of the whole-UI blink. Only run the
              // broad refresh when this run was user-initiated (button
              // press); auto-ingest's own poller does a game-list-only
              // refresh that's already wired separately. Call via ref
              // so we always invoke the *current* render's refresh
              // function — the WebSocket handler is mount-once (deps
              // `[]`), so a direct call would close over the initial-
              // render version and refresh with empty filters.
              if (manualIngestInFlight.current) {
                manualIngestInFlight.current = false;
                void refreshAfterIngestRef.current?.();
              }
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
        if (unmounted) return;
        // Reconnect with backoff: 2s, 5s, 10s, then 30s thereafter.
        const delays = [2000, 5000, 10000, 30000];
        const delay = delays[Math.min(reconnectAttempt, delays.length - 1)];
        reconnectAttempt += 1;
        reconnectTimer = window.setTimeout(connect, delay);
      };
    };

    connect();

    return () => {
      unmounted = true;
      if (reconnectTimer) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (ingestSocketRef.current === socket) {
        ingestSocketRef.current = null;
      }
      if (socket) {
        socket.close();
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount-once: WS lives for the whole app session, independent of modal visibility.
  }, []);

  useEffect(() => {
    if (!showIngestPanel) return undefined;
    setIngestMessage('');
    void loadIngestSettings();
    return undefined;
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only refresh settings + clear message when modal opens.
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
          // Game-list-only refresh: no other screen should ever blink in
          // response to a background ingestion. If auto-ingest was a
          // no-op (didIncrease=false) nothing reloads at all. Routing
          // via a ref so the latest filter/page state is read at fire
          // time rather than the closure-captured value from when
          // auto-ingest was first enabled.
          await refreshGamesAfterAutoIngestRef.current?.();
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
      const totalReplays = Number(data?.total_replays || 0);
      setReplayCount(totalReplays);
      if (data?.version) {
        setCurrentVersion(String(data.version));
      }
      if (totalReplays === 0 && !emptyDbAutoOpenRef.current) {
        emptyDbAutoOpenRef.current = true;
        setShowIngestPanel(true);
      }
      return data;
    } catch (err) {
      console.error('Failed to check OpenAI status:', err);
      return null;
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

      // Mark this as a user-initiated ingest: the WebSocket 'completed'
      // handler should fire the broad refresh for THIS run only. Reset
      // when the handler observes 'completed' (or 'error', via the
      // status branch that doesn't refresh).
      manualIngestInFlight.current = true;
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
      // Clear the flag — no 'completed' WebSocket message will follow
      // a failed startIngest, so without this the next ingest run
      // (manual or auto) would inherit a stale "manual" verdict.
      manualIngestInFlight.current = false;
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

  const refreshDataAfterGlobalReplayFilterSave = async () => {
    await Promise.all([
      loadMainGames({ page: mainGamesPage, filters: mainGamesFilters }),
      loadMainPlayers({
        page: mainPlayersPage,
        filters: mainPlayersFilters,
        sortBy: mainPlayersSortBy,
        sortDir: mainPlayersSortDir,
      }),
      loadTopPlayerColors(),
      checkOpenAIStatus(),
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

  // Lightweight game-list-only refresh used by the auto-ingest path.
  // No active-view re-fetch, no histogram reloads — just the game list,
  // which is the only screen that newly-ingested replays affect.
  const refreshGameListOnly = async () => {
    try {
      await loadMainGames({ page: mainGamesPage, filters: mainGamesFilters });
    } catch (err) {
      console.error('Failed to refresh game list after auto-ingest:', err);
    }
  };

  // Keep the latest-ref pattern wiring up to date. Assigning during render
  // (rather than in an effect) is the standard React pattern and is safe
  // because we only *read* these refs from event/timer callbacks, never
  // during the render itself.
  refreshAfterIngestRef.current = refreshDataAfterGlobalReplayFilterSave;
  refreshGamesAfterAutoIngestRef.current = refreshGameListOnly;
  mainGamesFiltersRef.current = mainGamesFilters;
  mainGamesPageRef.current = mainGamesPage;

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
      matchup: [],
      mapKind: [],
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

  const renderPlayerLinkLabel = (name, playerKey) => {
    const color = playerAccentColor(playerKey || name);
    const style = color ? { color, fontWeight: 600 } : undefined;
    if (!playerKey) return <span style={style}>{name}</span>;
    return (
      <button
        type="button"
        className="workflow-player-name-link"
        title="Analyze player"
        style={style}
        onClick={(e) => { e.stopPropagation(); openMainPlayer(playerKey); }}
      >
        {name}
      </button>
    );
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

  const renderWorkerIcon = (race) => {
    const url = getWorkerIconForRace(race);
    if (!url) return null;
    return <img src={url} alt={race || ''} title={race || ''} className="workflow-race-icon" />;
  };

  const renderMainGameListPlayers = (game, linkPlayerNames = true) => {
    const players = Array.isArray(game?.players) ? game.players : [];
    if (players.length === 0) {
      return renderPlayersMatchup(game?.players_label || '');
    }
    const renderName = (player) => (linkPlayerNames
      ? renderPlayerLinkLabel(player.name, player.player_key)
      : renderPlayerLabel(player.name, player.player_key));
    const stackingMarker = game?.team_stacking ? (
      <span
        className="workflow-team-stacking-marker"
        title="Team stacking — uneven non-solo team sizes for over 5 minutes"
        style={{ marginLeft: 6 }}
      >
        😈
      </span>
    ) : null;
    if (!playersHaveDistinctTeams(players)) {
      const warningText = game?.team_info_incomplete
        ? 'Team information is incomplete'
        : 'This replay has no team information';
      return (
        <span>
          {players.map((player, idx) => (
            <span key={`${player.player_id}-${idx}`}>
              {player.is_winner ? <span className="workflow-crown" title="Winner">👑</span> : null}
              {renderWorkerIcon(player.race)}
              {renderName(player)}
              {idx < players.length - 1 ? ', ' : ''}
            </span>
          ))}
          <span className="workflow-no-team-warning" title={warningText}>⚠️</span>
          {stackingMarker}
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
                  {renderWorkerIcon(player.race)}
                  {renderName(player)}
                </span>
              ))}
            </span>
          </React.Fragment>
        ))}
        {stackingMarker}
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

  const topicFilteredGameEvents = useMemo(() => {
    const allEvents = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    const visibleEvents = allEvents.filter((event) => {
      if (isStructuralGameEventType(event?.type)) {
        return false;
      }
      if (normalizeEventType(event?.type) === 'takeover') {
        return false;
      }
      return summaryTextMatches(gameEventSearchText(event, markerRegistry));
    });
    const deduped = [];
    for (let idx = 0; idx < visibleEvents.length; idx += 1) {
      const event = visibleEvents[idx];
      const prev = deduped.length > 0 ? deduped[deduped.length - 1] : null;
      // Recall events are intentionally per-cast — a recall combo (multiple
      // recalls within seconds of each other) is exactly the kind of detail
      // users want to see, so skip the description-equality collapse here.
      const isRecall = normalizeEventType(event?.type) === 'recall';
      if (!isRecall && prev && gameEventDescription(prev, markerRegistry) === gameEventDescription(event, markerRegistry)) {
        continue;
      }
      deduped.push(event);
    }
    return deduped;
  }, [mainGame?.game_events, mainSummaryFilters, markerRegistry]);

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
      const text = gameEventSearchText(event, markerRegistry);
      if (SUMMARY_TOPIC_PATTERNS.nuke.test(text)) base.nuke = true;
      if (SUMMARY_TOPIC_PATTERNS.drop.test(text)) base.drop = true;
      if (SUMMARY_TOPIC_PATTERNS.recall.test(text)) base.recall = true;
      if (nt === 'scout' || SUMMARY_TOPIC_PATTERNS.scout.test(text)) base.scout = true;
      if (SUMMARY_TOPIC_PATTERNS.becameRace.test(text) || nt === 'became_terran' || nt === 'became_zerg') base.becameRace = true;
      if (SUMMARY_TOPIC_PATTERNS.rush.test(text)) base.rush = true;
    }
    return base;
  }, [mainGame?.game_events, markerRegistry]);
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
  const mainGameFeaturingPillsList = useMemo(
    () => buildMainGameFeaturingPills(mainGame, markerDefinitions),
    [mainGame, markerDefinitions],
  );
  const selectedMainGameArrow = useMemo(() => {
    if (!selectedMainGameEvent || !isArrowEventType(selectedMainGameEvent.type)) return null;
    // actor_origin is the source player's starting location. If inactivity
    // rules have stripped ownership of that starting base, anchor the arrow
    // at any base the actor still owns at event time so the visual matches
    // the player's actual map presence.
    const actorID = Number(selectedMainGameEvent?.actor?.player_id || 0);
    const ownership = Array.isArray(selectedMainGameEvent?.ownership) ? selectedMainGameEvent.ownership : [];
    const ownedByActor = ownership.filter((entry) => Number(entry?.owner?.player_id || 0) === actorID && entry?.base?.center);
    const startingOwned = ownedByActor.some((entry) => String(entry?.base?.kind || '').toLowerCase() === 'starting');
    let originPoint = selectedMainGameEvent?.actor_origin;
    if (!startingOwned && ownedByActor.length > 0) {
      originPoint = ownedByActor[0]?.base?.center;
    }
    const from = mapPointToPercent(originPoint, mainEventMapBounds);
    const to = mapPointToPercent(selectedMainGameEvent?.base?.center, mainEventMapBounds);
    if (!from || !to) return null;
    return {
      from,
      to,
      color: playerColorToCss(selectedMainGameEvent?.actor?.color),
    };
  }, [selectedMainGameEvent, mainEventMapBounds]);
  const selectedMainGameArrowUnits = useMemo(() => {
    if (!selectedMainGameArrow || !selectedMainGameEvent) return [];
    const actorPid = Number(selectedMainGameEvent?.actor?.player_id || 0);
    const actorRow = mainGamePlayers.find((player) => Number(player?.player_id || 0) === actorPid);
    const unitNames = Array.isArray(selectedMainGameEvent.attack_unit_types) && selectedMainGameEvent.attack_unit_types.length > 0
      ? selectedMainGameEvent.attack_unit_types
      : fallbackOverlayUnitNamesForEvent(selectedMainGameEvent.type, actorRow?.race);
    return unitNames
      .map((name) => ({ name, icon: getUnitIcon(name) }))
      .filter((item) => item.icon)
      .slice(0, 4);
  }, [selectedMainGameArrow, selectedMainGameEvent, mainGamePlayers]);
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
  const selectedMainGameExpansionOverlay = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'expansion') return null;
    // Prefer the polygon's geometric center over scmapanalyzer's base.center —
    // the latter is pulled toward mineral-cluster mass and lands visibly
    // off-center on asymmetric bases. Fall back to base.center when polygon
    // data is missing.
    const anchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
      || selectedMainGameEvent?.base?.center;
    if (!anchor) return null;
    const playerID = Number(selectedMainGameEvent?.actor?.player_id || 0);
    const actorRow = mainGamePlayers.find((player) => Number(player?.player_id || 0) === playerID);
    const icon = getExpansionMarkerIconForRace(actorRow?.race);
    if (!icon) return null;
    const point = mapPointToPercent(anchor, mainEventMapBounds);
    if (!point) return null;
    return { icon, point };
  }, [selectedMainGameEvent, mainGamePlayers, mainEventMapBounds]);
  // Recall has no meaningful from→to vector — the cast point is the source
  // (where units are pulled from), but the destination is the Arbiter's
  // position which isn't in the command stream. Render a single Arbiter icon
  // at the cast point instead of an arrow.
  const selectedMainGameRecallOverlay = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'recall') return null;
    const anchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
      || selectedMainGameEvent?.base?.center;
    if (!anchor) return null;
    const icon = getUnitIcon('arbiter');
    if (!icon) return null;
    const point = mapPointToPercent(anchor, mainEventMapBounds);
    if (!point) return null;
    return { icon, point };
  }, [selectedMainGameEvent, mainEventMapBounds]);

  const mainPlayerInsights = [
    mainPlayerApmInsight,
    mainPlayerViewportInsight,
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
  const mainEventRaceByPlayerID = useMemo(
    () => new Map(mainGamePlayers.map((player) => [Number(player?.player_id || 0), String(player?.race || '').trim()])),
    [mainGamePlayers],
  );
  const hasTeamInfo = useMemo(() => {
    const uniqueTeams = new Set(mainGamePlayers.map((player) => player.team));
    return uniqueTeams.size > 1;
  }, [mainGamePlayers]);
  // Track the rendered height of the Game Events map panel so the events list
  // beside it can be sized to match.
  const [mainEventMapPanelEl, setMainEventMapPanelEl] = useState(null);
  const [mainEventMapPanelHeight, setMainEventMapPanelHeight] = useState(null);
  useEffect(() => {
    if (!mainEventMapPanelEl) {
      setMainEventMapPanelHeight(null);
      return undefined;
    }
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        setMainEventMapPanelHeight(entry.contentRect.height);
      }
    });
    ro.observe(mainEventMapPanelEl);
    return () => ro.disconnect();
  }, [mainEventMapPanelEl]);
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

      const points = mappedPoints.map((point) => {
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

  // filterProductionEntries applies the unified production-view filter to a
  // list of {unit_type, ...} entries. `view` selects whether the universe is
  // 'all' / 'units' / 'buildings'; `productionSubFilter` then narrows further.
  // Under 'all', tier filters target the union of UNIT_TIER_MAP and
  // BUILDING_TIER_MAP; 'defenses' is building-only so it filters out units.
  const filterProductionEntries = (entries, view) => {
    const mode = productionSubFilter;
    const nameNeedle = String(productionNameFilter).trim().toLowerCase();
    return (entries || []).filter((entry) => {
      const unitType = String(entry?.unit_type || '');
      const key = normalizeUnitName(unitType);
      const isBuilding = (typeof entry?.is_building === 'boolean')
        ? entry.is_building
        : BUILDING_TYPE_KEYS.has(key);
      if (view === 'units' && isBuilding) return false;
      if (view === 'buildings' && !isBuilding) return false;
      if (nameNeedle && !unitType.toLowerCase().includes(nameNeedle)) return false;
      if (mode === 'all') return true;
      if (mode === 'workers') return !isBuilding && WORKER_UNIT_KEYS.has(key);
      if (mode === 'non-workers') return !isBuilding && !WORKER_UNIT_KEYS.has(key);
      if (mode === 'spellcasters') return !isBuilding && SPELLCASTER_UNIT_KEYS.has(key);
      if (mode === 'defenses') return isBuilding && DEFENSIVE_BUILDING_KEYS.has(key);
      if (mode === 'tier-1') return isBuilding ? BUILDING_TIER_MAP[key] === 1 : UNIT_TIER_MAP[key] === 1;
      if (mode === 'tier-2') return isBuilding ? BUILDING_TIER_MAP[key] === 2 : UNIT_TIER_MAP[key] === 2;
      if (mode === 'tier-3') return isBuilding ? BUILDING_TIER_MAP[key] === 3 : UNIT_TIER_MAP[key] === 3;
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
                setShowGlobalReplayFilter(true);
              }}
              className="workflow-nav-text-action"
            >
              ⚙️ Settings
            </button>
            <button type="button" onClick={() => setShowIngestPanel(true)} className="workflow-nav-text-action">
              📥 Ingest
              {!showIngestPanel && ingestStatus === 'running' ? (
                <span className="ingest-running-badge" title="Ingestion in progress — click to view logs">Ingesting…</span>
              ) : null}
            </button>
            {staleReplaysCount > 0 && staleReplaysCount > dismissedStaleCount && ingestStatus !== 'running' ? (
              <span className="stale-replays-hint-wrap">
                <span className="stale-replays-hint-icon" aria-label="Replay analysis update available">⚠️</span>
                <span className="stale-replays-hint-tooltip" role="tooltip">
                  Replay analysis just got smarter! Please re-ingest (tick &quot;Erase data&quot;).
                  <div className="stale-replays-hint-tooltip-actions">
                    <button
                      type="button"
                      className="stale-replays-hint-dismiss"
                      onClick={(ev) => { ev.stopPropagation(); dismissStaleHint(); }}
                    >
                      Dismiss
                    </button>
                  </div>
                </span>
              </span>
            ) : null}
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
              <div className="workflow-filter-group">
                {(mainGamesFilterOptions.durations || []).map((option) => {
                  const active = (mainGamesFilters.duration || []).includes(option.key);
                  return (
                    <button
                      key={`wf-duration-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleMainGameMultiFilter('duration', option.key)}
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
              <div className="workflow-filter-group">
                {(mainGamesFilterOptions.map_kinds || []).map((option) => {
                  const active = (mainGamesFilters.mapKind || []).includes(option.key);
                  return (
                    <button
                      key={`wf-mapkind-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleMainGameMultiFilter('mapKind', option.key)}
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
              <div className="workflow-filter-group">
                {(mainGamesFilterOptions.matchups || []).map((option) => {
                  const active = (mainGamesFilters.matchup || []).includes(option.key);
                  return (
                    <button
                      key={`wf-matchup-${option.key}`}
                      type="button"
                      className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                      onClick={() => toggleMainGameMultiFilter('matchup', option.key)}
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
              <div className="workflow-filter-group">
                <button
                  type="button"
                  className={`workflow-filter-pill workflow-filter-pill-disclosure ${mainGamesShowBOFilters ? 'workflow-filter-pill-active' : ''}`}
                  onClick={() => setMainGamesShowBOFilters((prev) => !prev)}
                  aria-expanded={mainGamesShowBOFilters}
                >
                  Build orders {mainGamesShowBOFilters ? '▾' : '▸'}
                </button>
                {mainGamesShowBOFilters && (mainGamesFilterOptions.featuring || [])
                  .filter((option) => (option.group || '') === 'bo')
                  .map((option) => {
                    const active = (mainGamesFilters.featuring || []).includes(option.key);
                    return (
                      <button
                        key={`wf-feature-bo-${option.key}`}
                        type="button"
                        className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''}`}
                        onClick={() => toggleMainGameMultiFilter('featuring', option.key)}
                      >
                        {option.label}
                      </button>
                    );
                  })}
              </div>
            </div>
            <div className="workflow-summary-filter-row workflow-games-filter-row">
              <div className="workflow-filter-group">
                {(mainGamesFilterOptions.featuring || [])
                  .filter((option) => (option.group || 'marker') !== 'bo')
                  .map((option) => {
                    const active = (mainGamesFilters.featuring || []).includes(option.key);
                    const iconKeys = (Array.isArray(option.icon_keys) && option.icon_keys.length)
                      ? option.icon_keys
                      : (option.icon_key ? [option.icon_key] : []);
                    const iconUrls = iconKeys.map((k) => getUnitIcon(k)).filter(Boolean);
                    const hasIcons = iconUrls.length > 0;
                    const hasEmoji = !hasIcons && Boolean(option.emoji);
                    return (
                      <button
                        key={`wf-feature-${option.key}`}
                        type="button"
                        className={`workflow-filter-pill ${active ? 'workflow-filter-pill-active' : ''} ${hasIcons ? 'workflow-filter-pill-icon' : ''}`}
                        onClick={() => toggleMainGameMultiFilter('featuring', option.key)}
                        title={option.label}
                        aria-label={option.label}
                      >
                        {hasIcons ? (
                          <>
                            {iconUrls.map((url, i) => (
                              <img key={`${option.key}-i${i}`} src={url} alt="" className="workflow-filter-pill-icon-img" />
                            ))}
                            {option.icon_label && (
                              <span className="workflow-filter-pill-icon-label">{option.icon_label}</span>
                            )}
                          </>
                        ) : hasEmoji ? (
                          <>
                            <span className="workflow-filter-pill-emoji">{option.emoji}</span>
                            <span className="workflow-filter-pill-icon-label">{option.label}</span>
                          </>
                        ) : (
                          option.label
                        )}
                      </button>
                    );
                  })}
              </div>
              <button type="button" className="workflow-filter-pill workflow-filter-pill-clear" onClick={clearMainGamesFilters}>Clear filters</button>
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
                        <td>{renderMainGameListPlayers(game, false)}</td>
                        <td>{formatMapNameWithKind(game.map_name, game.map_kind)}</td>
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
                <div className="workflow-pagination-row workflow-pagination-row-centered">
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={mainGamesPage <= 1 || mainGamesLoading}
                    onClick={() => setMainGamesPage((prev) => Math.max(1, prev - 1))}
                    aria-label="Previous page"
                  >
                    {'<'}
                  </button>
                  <span>{mainGamesFrom}-{mainGamesTo} of {mainGamesTotal}</span>
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={mainGamesPage >= mainGamesTotalPages || mainGamesLoading}
                    onClick={() => setMainGamesPage((prev) => Math.min(mainGamesTotalPages, prev + 1))}
                    aria-label="Next page"
                  >
                    {'>'}
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
                  <span>{formatMapNameWithKind(mainGame.map_name, mainGame.map_kind)}</span>
                  <span>{formatDuration(mainGame.duration_seconds)}</span>
                  {mainGame.file_path ? (
                    <code className="workflow-meta-filepath-text" title={mainGame.file_path}>
                      {mainGame.file_path.replace(/\\/g, '/').split('/').pop()}
                    </code>
                  ) : null}
                  {mainGame.file_path ? (
                    <button
                      type="button"
                      className="btn-switch workflow-meta-filepath-copy"
                      title="Copy full replay file path to clipboard"
                      onClick={() => {
                        if (navigator.clipboard && navigator.clipboard.writeText) {
                          navigator.clipboard.writeText(mainGame.file_path);
                        }
                      }}
                    >
                      Copy full path
                    </button>
                  ) : null}
                  <button
                    type="button"
                    className="btn-switch btn-switch-see-replay workflow-meta-stage-btn"
                    disabled={mainGameSeeLoading}
                    title="Clones this replay into your configured replay ingestion folder as 000_screpdb_watch_me/watch_me.rep so you can easily find it within Starcraft."
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
                    {Array.isArray(mainGame?.mutalisk_timing_chart) && mainGame.mutalisk_timing_chart.length > 0 ? (
                      <button
                        type="button"
                        role="tab"
                        aria-selected={mainGameTab === 'mutalisk-timing'}
                        className={`workflow-production-tab ${mainGameTab === 'mutalisk-timing' ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => setMainGameTab('mutalisk-timing')}
                      >
                        Mutalisk Timing
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
                    {Array.isArray(mainGame?.alliance_timeline) && mainGame.alliance_timeline.length > 0 ? (
                      <button
                        type="button"
                        role="tab"
                        aria-selected={mainGameTab === 'alliances'}
                        className={`workflow-production-tab ${mainGameTab === 'alliances' ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => setMainGameTab('alliances')}
                      >
                        Alliances{mainGame?.team_stacking ? ' 😈' : ''}
                      </button>
                    ) : null}
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
                            <div className="workflow-summary-features-title">Featuring</div>
                            <div className="workflow-pattern-pills">
                              {mainGameFeaturingPillsList.map((pill) => renderFeaturingPill(pill, 'summary-game'))}
                            </div>
                          </>
                        ) : (
                          <div className="workflow-subtle-note">No featured highlights for this replay.</div>
                        )}
                        {/* Replay-aggregate attacker-composition pills (early/mid/late).
                            Computed at display time by summing per-player counts in
                            mainGame.unit_composition_markers (Decision 6 in plan
                            ~/.claude/plans/i-want-to-explore-snoopy-hippo.md). */}
                        {Array.isArray(mainGame?.unit_composition_markers) && mainGame.unit_composition_markers.length > 0 ? (
                          <div className="workflow-summary-composition">
                            <div className="workflow-summary-features-title workflow-summary-composition-title">Unit composition</div>
                            {/* Aggregate pills: slotCount=10 (vs 6 default for
                                per-player) since this row sits alone with room
                                to spare. maxCasters intentionally unset so the
                                full cross-player notable list is visible on the
                                summary surface. Per-player pills below keep the
                                compact 6-slot, 4-caster layout. */}
                            <CompositionPhasesRow
                              phases={computeReplayAggregatePhases(mainGame.unit_composition_markers)}
                              slotCount={10}
                            />
                          </div>
                        ) : null}
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
                                {filterSummaryPillPatterns(player.detected_patterns, trustGameEventsForDrops).map((pattern, idx) => renderPatternPill(pattern, `player-${player.player_id}-${idx}`, undefined, markerRegistry))}
                              </div>
                              {/* Per-player attacker-composition phase pills.
                                  Source: mainGame.unit_composition_markers filtered to this player. */}
                              {Array.isArray(mainGame?.unit_composition_markers) ? (() => {
                                const playerPhases = mainGame.unit_composition_markers.filter((m) => m.player_id === player.player_id);
                                if (playerPhases.length === 0) return null;
                                return (
                                  <div className="workflow-pattern-pills">
                                    <CompositionPhasesRow phases={playerPhases} maxCasters={4} />
                                  </div>
                                );
                              })() : null}
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
                      {!hasTeamInfo ? (
                        <div className="workflow-section-warning">
                          ⚠️ {mainGame?.team_info_incomplete ? 'Team information is incomplete' : 'This replay has no team information'}. Expect issues like attack events firing between teammates.
                        </div>
                      ) : null}
                      <div className="workflow-section-warning">
                        ⚠️ Event narratives are derived from imperfect replay signals: expect some errors.
                      </div>
                    </div>
                    <div className="workflow-events-layout">
                        <div className="workflow-event-map-panel" ref={setMainEventMapPanelEl}>
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
                                  <svg
                                    className="workflow-event-map-overlay"
                                    viewBox="0 0 100 100"
                                    preserveAspectRatio="none"
                                    aria-hidden="true"
                                  >
                                    <defs>
                                      <marker
                                        id="workflow-event-arrowhead"
                                        markerWidth="2.5"
                                        markerHeight="2.5"
                                        refX="2.25"
                                        refY="1.25"
                                        orient="auto"
                                      >
                                        <polygon points="0 0, 2.5 1.25, 0 2.5" fill={selectedMainGameArrow?.color || 'currentColor'} />
                                      </marker>
                                    </defs>
                                    {selectedMainGameOwnershipPolygons.map((overlay) => (
                                      <polygon
                                        key={overlay.key}
                                        points={overlay.points}
                                        className="workflow-event-map-base-polygon"
                                        style={{ fill: `${overlay.ownerColor}66`, stroke: overlay.ownerColor }}
                                      />
                                    ))}
                                    {selectedMainGameArrow ? (
                                      <line
                                        key={`arrow-${selectedMainGameEventKeyResolved}`}
                                        x1={selectedMainGameArrow.from.x}
                                        y1={selectedMainGameArrow.from.y}
                                        x2={selectedMainGameArrow.to.x}
                                        y2={selectedMainGameArrow.to.y}
                                        className="workflow-event-map-attack-line"
                                        style={{ color: selectedMainGameArrow.color, stroke: selectedMainGameArrow.color }}
                                        markerEnd="url(#workflow-event-arrowhead)"
                                      />
                                    ) : null}
                                  </svg>
                                ) : null}
                                {selectedMainGameArrow && selectedMainGameArrowUnits.length > 0 ? (
                                  <div
                                    key={`unit-overlay-${selectedMainGameEventKeyResolved}`}
                                    className={`workflow-event-map-unit-overlay ${selectedMainGameArrowUnits.length > 2 ? 'workflow-event-map-unit-overlay--grid' : ''}`}
                                    style={{
                                      left: `${(selectedMainGameArrow.from.x + selectedMainGameArrow.to.x) / 2}%`,
                                      top: `${(selectedMainGameArrow.from.y + selectedMainGameArrow.to.y) / 2}%`,
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
                                {selectedMainGameLeaveFlag ? (
                                  <div
                                    key={`leave-flag-${selectedMainGameEventKeyResolved}`}
                                    className="workflow-event-map-flag-overlay"
                                    style={{
                                      left: `${selectedMainGameLeaveFlag.x}%`,
                                      top: `${selectedMainGameLeaveFlag.y}%`,
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
                                    key={`expansion-${selectedMainGameEventKeyResolved}`}
                                    src={selectedMainGameExpansionOverlay.icon}
                                    alt="Expansion building"
                                    className="workflow-event-map-expansion-overlay"
                                    style={{
                                      left: `${selectedMainGameExpansionOverlay.point.x}%`,
                                      top: `${selectedMainGameExpansionOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                                {selectedMainGameRecallOverlay ? (
                                  <img
                                    key={`recall-${selectedMainGameEventKeyResolved}`}
                                    src={selectedMainGameRecallOverlay.icon}
                                    alt="Recall cast point"
                                    title="Recall cast point — destination is the Arbiter (not in command stream)"
                                    className="workflow-event-map-expansion-overlay"
                                    style={{
                                      left: `${selectedMainGameRecallOverlay.point.x}%`,
                                      top: `${selectedMainGameRecallOverlay.point.y}%`,
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
                        <div
                          className="workflow-events"
                          style={mainEventMapPanelHeight ? { height: `${mainEventMapPanelHeight}px`, maxHeight: `${mainEventMapPanelHeight}px` } : undefined}
                        >
                          {filteredGameEvents.length > 0 ? (
                            (() => {
                              const earlyEnd = Number(mainGame?.early_game_ends_at_second) || 0;
                              const midEnd = Number(mainGame?.mid_game_ends_at_second) || 0;
                              const phaseFor = (sec) => {
                                if (earlyEnd > 0 && sec < earlyEnd) return 'early';
                                if (midEnd > 0 && sec < midEnd) return 'mid';
                                return 'late';
                              };
                              const nodes = [];
                              let lastPhase = null;
                              filteredGameEvents.forEach((event) => {
                                const topicIndex = topicFilteredGameEvents.indexOf(event);
                                const eventKey = gameEventTopicKey(topicIndex);
                                const selected = eventKey === selectedMainGameEventKeyResolved;
                                const iconEntries = gameEventRowIconEntries(event, mainEventRaceByPlayerID, markerRegistry);
                                const castEntries = event?.attack_cast_counts && typeof event.attack_cast_counts === 'object'
                                  ? Object.entries(event.attack_cast_counts)
                                  : [];
                                const phase = phaseFor(Number(event?.second) || 0);
                                const isLeaveGame = normalizeEventType(event?.type) === 'leave_game';
                                if (!isLeaveGame && phase !== lastPhase) {
                                  // Only show "Mid game" / "Late game" when mid game actually
                                  // ended; otherwise the game never reached those phases.
                                  let label = null;
                                  if (phase === 'early' && earlyEnd > 0) label = 'Early game';
                                  else if (phase === 'mid' && midEnd > 0) label = 'Mid game';
                                  else if (phase === 'late' && midEnd > 0) label = 'Late game';
                                  if (label) {
                                    nodes.push(
                                      <div key={`hdr-${phase}`} className={`workflow-events-section-header workflow-events-section-header--${phase}`}>
                                        {label}
                                      </div>,
                                    );
                                  }
                                  lastPhase = phase;
                                }
                                nodes.push(
                                  <button
                                    key={eventKey}
                                    type="button"
                                    className={`workflow-event-row ${selected ? 'workflow-event-row-selected' : ''}`}
                                    onClick={() => setMainSelectedGameEventKey(eventKey)}
                                  >
                                    <span>{formatDuration(event.second)}</span>
                                    <span className="workflow-event-row-body">
                                      <span>{renderGameEventDescription(event, markerRegistry)}</span>
                                      {(iconEntries.length > 0 || castEntries.length > 0) ? (
                                        <span className="workflow-event-row-units">
                                          {iconEntries.map((entry, idx) => (
                                            entry.emoji ? (
                                              <span key={`emoji-${idx}`} className="workflow-event-row-emoji" role="img" aria-label={entry.alt} title={entry.title}>
                                                {entry.emoji}
                                              </span>
                                            ) : (
                                              <img
                                                key={`icon-${idx}`}
                                                src={entry.src}
                                                alt={entry.alt}
                                                title={entry.title}
                                                className="workflow-event-row-icon"
                                              />
                                            )
                                          ))}
                                          {castEntries.map(([spell, count]) => (
                                            <span key={`cast-${spell}`} className="workflow-event-row-cast-pill" title={`${spell} cast ${count}× near this attack`}>
                                              {Number(count) > 1 ? `${count}× ` : ''}{spell}
                                            </span>
                                          ))}
                                        </span>
                                      ) : null}
                                    </span>
                                  </button>,
                                );
                              });
                              return nodes;
                            })()
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
                      <div className="workflow-radio-group" role="radiogroup" aria-label="Production view">
                        {[
                          { value: 'all', label: 'All' },
                          { value: 'units', label: 'Units' },
                          { value: 'buildings', label: 'Buildings' },
                        ].map((opt) => (
                          <label key={opt.value} className="workflow-radio-option">
                            <input
                              type="radio"
                              name="workflow-production-view"
                              value={opt.value}
                              checked={productionView === opt.value}
                              onChange={(e) => {
                                setProductionView(e.target.value);
                                setProductionSubFilter('all');
                              }}
                            />
                            <span>{opt.label}</span>
                          </label>
                        ))}
                      </div>
                      <div className="workflow-section-warning">
                        ⚠️ Replay commands contain significant false positives. Expect inflated numbers.
                      </div>
                    </div>
                    <div className="workflow-summary-filter-row">
                      <div className="workflow-radio-group">
                        {(productionView === 'units'
                          ? [
                              { value: 'all', label: 'All units' },
                              { value: 'workers', label: 'Workers only' },
                              { value: 'non-workers', label: 'Non-workers only' },
                              { value: 'spellcasters', label: 'Spellcasters only' },
                              { value: 'tier-1', label: 'Tier 1 only' },
                              { value: 'tier-2', label: 'Tier 2 only' },
                              { value: 'tier-3', label: 'Tier 3 only' },
                            ]
                          : productionView === 'buildings'
                            ? [
                                { value: 'all', label: 'All buildings' },
                                { value: 'defenses', label: 'Defenses only' },
                                { value: 'tier-1', label: 'Tier 1 only' },
                                { value: 'tier-2', label: 'Tier 2 only' },
                                { value: 'tier-3', label: 'Tier 3 only' },
                              ]
                            : [
                                { value: 'all', label: 'All' },
                                { value: 'tier-2', label: 'Tier 2 only' },
                                { value: 'tier-3', label: 'Tier 3 only' },
                                { value: 'defenses', label: 'Defenses only' },
                              ]
                        ).map((opt) => (
                          <label key={opt.value} className="workflow-radio-option">
                            <input
                              type="radio"
                              name="workflow-production-subfilter"
                              value={opt.value}
                              checked={productionSubFilter === opt.value}
                              onChange={(e) => setProductionSubFilter(e.target.value)}
                            />
                            <span>{opt.label}</span>
                          </label>
                        ))}
                      </div>
                      <input
                        type="text"
                        className="workflow-summary-filter-input"
                        placeholder={productionView === 'buildings' ? 'Filter building name...' : 'Filter unit name...'}
                        value={productionNameFilter}
                        onChange={(e) => setProductionNameFilter(e.target.value)}
                      />
                    </div>
                    <UnitProductionEarlyTimeline
                      players={mainGamePlayers}
                      earlyEvents={mainGame.units_early_events || []}
                      filterEvents={(events) => filterProductionEntries(events, productionView)}
                      hasTeamInfo={hasTeamInfo}
                      teamColorRgba={teamColorRgba}
                    />
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
                                const filtered = filterProductionEntries(playerSlice?.units || [], productionView);
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
                {mainGameTab === 'mutalisk-timing' && (
                  <div className="workflow-timing-charts">
                    {Array.isArray(mainGame?.mutalisk_timing_chart) && mainGame.mutalisk_timing_chart.length > 0 ? (
                      <MutaliskTimingChart
                        zSide={mainGame.mutalisk_timing_chart.find((s) => (s.feature_key || '').includes('mutalisk'))}
                        tSide={mainGame.mutalisk_timing_chart.find((s) => (s.feature_key || '').includes('turret'))}
                        summary={mainGame.mutalisk_timing_summary}
                      />
                    ) : (
                      <div className="workflow-card">
                        <div className="chart-empty">Mutalisk-Turret timing not detected for this game.</div>
                      </div>
                    )}
                  </div>
                )}
                {mainGameTab === 'alliances' && (
                  <div className="workflow-timing-charts">
                    {mainGame?.team_stacking ? (
                      <div className="workflow-section-warning">
                        😈 Team stacking detected — uneven non-solo team sizes lasted over {Math.round((mainGame.alliance_stacking_threshold_seconds || 300) / 60)} minutes.
                      </div>
                    ) : null}
                    <AllianceTimeline
                      players={Array.isArray(mainGame?.players) ? mainGame.players : []}
                      timeline={Array.isArray(mainGame?.alliance_timeline) ? mainGame.alliance_timeline : []}
                      durationSeconds={mainGame?.duration_seconds || 0}
                      stackingThresholdSeconds={mainGame?.alliance_stacking_threshold_seconds || 300}
                      getRaceIcon={getWorkerIconForRace}
                    />
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

            {mainGame && mainGameTab === 'summary' && openaiEnabled && (
              <>
                <form onSubmit={handleMainAsk} className="workflow-ask-form">
                  <input
                    className="widget-creation-input"
                    value={mainQuestion}
                    onChange={(e) => setMainQuestion(e.target.value)}
                    placeholder="Ask AI about this game..."
                    disabled={mainAskLoading}
                  />
                  <button className="btn-create-ai" type="submit" disabled={mainAskLoading || !mainQuestion.trim()}>
                    {mainAskLoading ? 'Asking...' : 'Ask AI'}
                  </button>
                </form>
                {renderMainAiResult()}
              </>
            )}
          </div>
        )}

        {activeView === 'player' && (() => {
          const isSkillProxiesTab = mainPlayerTab === 'skill-proxies';
          return (
          <div className="workflow-panel workflow-panel--player">
            {selectedPlayerKey ? (
              <>
                <div className="workflow-title-row">
                  <div className="workflow-player-title-wrap">
                    <h2 style={playerAccentColor(mainPlayer?.player_key || selectedPlayerKey) ? { color: playerAccentColor(mainPlayer?.player_key || selectedPlayerKey) } : undefined}>
                      {mainPlayer?.player_name || selectedPlayerKey}
                    </h2>
                    {mainPlayer && (Number(mainPlayer.games_played) || 0) < 5 ? (
                      <span className="workflow-inline-warning">⚠️ Fewer than 5 replays: we cannot provide reliable player-level insights yet.</span>
                    ) : null}
                  </div>
                  <button type="button" className="btn-switch" onClick={goBackMainView}>Back</button>
                </div>
                <div className="workflow-meta">
                  <span><strong>Games</strong> {mainPlayer ? mainPlayer.games_played : '—'}</span>
                  <span><strong>Win rate</strong> {mainPlayer ? `${(mainPlayer.win_rate * 100).toFixed(1)}%` : '—'}</span>
                  <span><strong>APM</strong> {mainPlayer ? mainPlayer.average_apm?.toFixed(1) : '—'}</span>
                  <span><strong>EAPM</strong> {mainPlayer ? mainPlayer.average_eapm?.toFixed(1) : '—'}</span>
                  {mainPlayerLoading ? <span className="workflow-subtle-note">loading overview…</span> : null}
                </div>
                <div className="workflow-game-tab-stack">
                  <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label="Player report sections">
                    <button type="button" role="tab" aria-selected={mainPlayerTab === 'summary'}
                      className={`workflow-production-tab ${mainPlayerTab === 'summary' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => { setMainPlayerTab('summary'); setMainPlayerSubtab(''); }}>
                      Summary
                    </button>
                    <button type="button" role="tab" aria-selected={isSkillProxiesTab}
                      className={`workflow-production-tab ${isSkillProxiesTab ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => {
                        if (isSkillProxiesTab) return;
                        setMainPlayerTab('skill-proxies');
                        setMainPlayerSubtab('');
                      }}>
                      Skill proxies
                    </button>
                    <button type="button" role="tab" aria-selected={mainPlayerTab === 'recent-games'}
                      className={`workflow-production-tab ${mainPlayerTab === 'recent-games' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => { setMainPlayerTab('recent-games'); setMainPlayerSubtab(''); }}>
                      Recent games
                    </button>
                    <button type="button" role="tab" aria-selected={mainPlayerTab === 'chat-summary'}
                      className={`workflow-production-tab ${mainPlayerTab === 'chat-summary' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => { setMainPlayerTab('chat-summary'); setMainPlayerSubtab(''); }}>
                      Chat summary
                    </button>
                  </div>

                </div>

                <div className="workflow-cards">
                  {mainPlayerTab === 'summary' && (
                    <>
                      <div className="workflow-card workflow-card-player-special">
                        <div className="workflow-card-title"><span>What's special about this player</span></div>
                        {mainPlayerSpecialLoading ? <div className="chart-empty">Loading highlights...</div> : null}
                        {!mainPlayerSpecialLoading && mainPlayerSpecialError ? <div className="chart-empty">{mainPlayerSpecialError}</div> : null}
                        {!mainPlayerSpecialLoading && !mainPlayerSpecialError ? (() => {
                          const eligible = [];
                          if (mainPlayerSpecial?.never_allied_multi_team?.eligible) {
                            const games = Number(mainPlayerSpecial.never_allied_multi_team.games || 0);
                            eligible.push({
                              key: 'never-allied',
                              label: '🚫 alliances',
                              title: `Never issued an alliance command across ${games} multi-team melee game${games === 1 ? '' : 's'}.`,
                              className: 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey',
                            });
                          }
                          if (mainPlayerSpecial?.never_hotkeys?.eligible) {
                            const games = Number(mainPlayerSpecial.never_hotkeys.games || 0);
                            eligible.push({
                              key: 'never-hotkeys',
                              label: '🚫 hotkeys',
                              title: `No hotkey-group commands across ${games} eligible game${games === 1 ? '' : 's'} (7+ minute gate).`,
                              className: 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey',
                            });
                          }
                          // Merge per-category outlier streams into one
                          // pill list, sort by TFIDF desc, then cap. Pills
                          // accumulate as each category's request resolves.
                          const allPills = [];
                          const categoryStates = PLAYER_SUMMARY_OUTLIER_CATEGORIES.map((cat) => {
                            const state = mainPlayerSpecialOutliers[cat.toLowerCase()] || { loading: false, pills: [] };
                            allPills.push(...(state.pills || []));
                            return state;
                          });
                          allPills.sort((a, b) => {
                            const ta = Number(a.tfidf) || 0;
                            const tb = Number(b.tfidf) || 0;
                            if (ta !== tb) return tb - ta;
                            return (Number(b.ratio_to_baseline) || 0) - (Number(a.ratio_to_baseline) || 0);
                          });
                          const outliers = allPills.slice(0, PLAYER_SUMMARY_OUTLIER_PILL_CAP);
                          const stillLoading = categoryStates.some((s) => s.loading);
                          if (eligible.length === 0 && outliers.length === 0 && !stillLoading) {
                            return <div className="workflow-subtle-note">Nothing distinctive flagged yet for this player.</div>;
                          }
                          return (
                            <>
                              <div className="workflow-pattern-pills">
                                {eligible.map((p) => (
                                  <span key={p.key} className={p.className} title={p.title}>{p.label}</span>
                                ))}
                                {outliers.map((it, idx) => {
                                  const label = it.pretty_label || it.pretty_name || it.name;
                                  const playerPct = ((Number(it.player_rate) || 0) * 100).toFixed(0);
                                  const baselinePct = ((Number(it.baseline_rate) || 0) * 100).toFixed(0);
                                  const ratio = (Number(it.ratio_to_baseline) || 0).toFixed(1);
                                  const qualified = (it.qualified_by || []).join(' / ');
                                  const segmentDesc = it.map_kind === 'Money'
                                    ? ' on Money maps'
                                    : it.map_kind === 'Regular'
                                      ? ' on Regular maps'
                                      : '';
                                  const title = `${it.category}: ${playerPct}% of ${it.race} games${segmentDesc} you vs ${baselinePct}% baseline (${ratio}× peers).${qualified ? ' ' + qualified + '.' : ''}`;
                                  const icon = it.icon_key ? getUnitIcon(it.icon_key) : null;
                                  return (
                                    <span
                                      key={`outlier-${idx}-${it.category}-${it.name}-${it.map_kind || 'all'}`}
                                      className="workflow-pattern-pill workflow-pattern-pill-strong workflow-summary-outlier-pill"
                                      title={title}
                                    >
                                      {icon ? <img src={icon} alt="" className="workflow-pattern-icon" /> : null}
                                      <span className="workflow-summary-outlier-pill-stack">
                                        <span className="workflow-summary-outlier-pill-label">{label}</span>
                                        <span className="workflow-summary-outlier-pill-qualifier">more than peers</span>
                                      </span>
                                    </span>
                                  );
                                })}
                              </div>
                              {stillLoading ? (
                                <div className="workflow-subtle-note">{`Loading more pills (${categoryStates.filter((s) => s.loading).length}/${PLAYER_SUMMARY_OUTLIER_CATEGORIES.length} categories pending)…`}</div>
                              ) : null}
                            </>
                          );
                        })() : null}
                      </div>

                      {mainPlayerPerMatchupLoading ? (
                        <div className="workflow-card"><div className="chart-empty">Loading matchup summary...</div></div>
                      ) : null}
                      {!mainPlayerPerMatchupLoading && mainPlayerPerMatchupError ? (
                        <div className="workflow-card"><div className="chart-empty">{mainPlayerPerMatchupError}</div></div>
                      ) : null}
                      {!mainPlayerPerMatchupLoading && !mainPlayerPerMatchupError && mainPlayerPerMatchup && (mainPlayerPerMatchup.cards || []).length > 0 ? (() => {
                        const cards = mainPlayerPerMatchup.cards || [];
                        const hasNonLow = cards.some((c) => c.confidence !== 'low');
                        const visibleCards = (hasNonLow && !mainPlayerShowLowConfidence)
                          ? cards.filter((c) => c.confidence !== 'low')
                          : cards;
                        const hiddenCount = cards.length - visibleCards.length;
                        return (
                          <div className="workflow-card workflow-card-player-matchups">
                            <div className="workflow-card-title"><span>Matchups & team formats</span></div>
                            <div className="workflow-player-matchup-grid">
                              {visibleCards.map((card) => {
                                const winPct = (Number(card.win_rate) || 0) * 100;
                                const dimmed = card.confidence === 'low' ? 'workflow-player-matchup-card--low' : '';
                                const ownIcon = getWorkerIconForRace(card.own_race);
                                const oppIcon = card.kind === 'matchup' ? getWorkerIconForRace(card.opp_race) : null;
                                let label;
                                if (card.kind === 'matchup') {
                                  const own = String(card.own_race || '').charAt(0).toUpperCase() || '?';
                                  const opp = String(card.opp_race || '').charAt(0).toUpperCase() || '?';
                                  label = `${own}v${opp}`;
                                } else {
                                  const formatLabel = card.format_class === 'multi-team' ? 'Multi-team' : card.format_class;
                                  const moneyTag = card.map_kind === 'Money' ? ' 💰' : '';
                                  label = `${formatLabel}${moneyTag}`;
                                }
                                // For format cards, add the player's race so a Random
                                // player can tell three same-format cards apart.
                                const formatRaceIcon = card.kind === 'format' ? ownIcon : null;
                                return (
                                  <div key={card.key} className={`workflow-player-matchup-card ${dimmed}`}>
                                    <div className="workflow-player-matchup-card-header">
                                      <span className="workflow-player-matchup-card-label">
                                        {formatRaceIcon ? <img src={formatRaceIcon} alt={card.own_race} title={card.own_race} className="workflow-recent-game-worker-icon" /> : null}
                                        {card.kind === 'matchup' && ownIcon ? <img src={ownIcon} alt={card.own_race} title={card.own_race} className="workflow-recent-game-worker-icon" /> : null}
                                        {oppIcon ? <span>v</span> : null}
                                        {oppIcon ? <img src={oppIcon} alt={card.opp_race} title={card.opp_race} className="workflow-recent-game-worker-icon" /> : null}
                                        <strong>{label}</strong>
                                      </span>
                                      <span className="workflow-player-matchup-card-meta">
                                        <span><strong>{card.games}</strong> games</span>
                                        <span><strong>{winPct.toFixed(0)}%</strong> wins</span>
                                        <span><strong>{(Number(card.avg_apm) || 0).toFixed(0)}</strong> APM</span>
                                        <span><strong>{(Number(card.avg_eapm) || 0).toFixed(0)}</strong> EAPM</span>
                                      </span>
                                    </div>
                                    {renderMatchupPatternSection('Top build orders', card.top_build_orders, `bo-${card.key}`, markerRegistry)}
                                    {renderMatchupPatternSection('Top markers', card.top_markers, `mk-${card.key}`, markerRegistry)}
                                  </div>
                                );
                              })}
                            </div>
                            {hasNonLow && hiddenCount > 0 ? (
                              <label className="workflow-summary-low-confidence-toggle">
                                <input
                                  type="checkbox"
                                  checked={mainPlayerShowLowConfidence}
                                  onChange={(e) => setMainPlayerShowLowConfidence(e.target.checked)}
                                />
                                <span>Show {hiddenCount} low-confidence card{hiddenCount === 1 ? '' : 's'} (&lt; 5 games)</span>
                              </label>
                            ) : null}
                          </div>
                        );
                      })() : null}
                    </>
                  )}

                  {isSkillProxiesTab && (
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
                            const overrideDesc = PLAYER_INSIGHT_DESCRIPTION_OVERRIDES[insight.insight_type];
                            const description = overrideDesc !== undefined ? overrideDesc : insight.description;
                            const popTab = playerInsightDestinationTab(insight.insight_type);
                            return (
                              <div
                                key={insight.insight_type}
                                className="workflow-insight-card workflow-insight-card-static"
                                style={insight.eligible ? { borderColor: `${accent}55`, boxShadow: `inset 0 0 0 1px ${accent}22` } : undefined}
                              >
                                <div className="workflow-insight-card-header">
                                  <span>{insight.title}</span>
                                </div>
                                {insight.eligible ? (
                                  <>
                                    <div className="workflow-insight-score-row">
                                      <span className="workflow-insight-score" style={{ color: accent }}>{insightSummaryLabel(percentile)}</span>
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
                                {description ? (
                                  <div className="workflow-subtle-note workflow-insight-description">{description}</div>
                                ) : null}
                                {popTab ? (
                                  <div className="workflow-insight-card-footer">
                                    <button
                                      type="button"
                                      className="workflow-link-btn"
                                      onClick={() => openMainPlayersSubview(popTab)}
                                    >
                                      See all players comparison →
                                    </button>
                                  </div>
                                ) : null}
                              </div>
                            );
                          })}
                        </div>
                      ) : null}
                    </div>
                  )}

                  {mainPlayerTab === 'recent-games' && (
                    <div className="workflow-card workflow-card-recent-games">
                      <div className="workflow-card-title"><span>Recent games</span></div>
                      {mainPlayerRecentGamesLoading ? <div className="chart-empty">Loading recent games...</div> : null}
                      {!mainPlayerRecentGamesLoading && mainPlayerRecentGamesError ? <div className="chart-empty">{mainPlayerRecentGamesError}</div> : null}
                      {!mainPlayerRecentGamesLoading && !mainPlayerRecentGamesError && mainPlayerRecentGames.length === 0 ? (
                        <div className="chart-empty">No recent games found for this player.</div>
                      ) : null}
                      {!mainPlayerRecentGamesLoading && !mainPlayerRecentGamesError && mainPlayerRecentGames.slice(0, 6).map((g) => {
                        const isWinner = !!g.current_player?.is_winner;
                        const hasResult = g.current_player !== undefined && g.current_player !== null;
                        const resultClass = hasResult ? (isWinner ? 'workflow-recent-game-card--win' : 'workflow-recent-game-card--loss') : '';
                        const playersList = Array.isArray(g.players) ? g.players : [];
                        const is1v1 = playersList.length === 2;
                        let matchupNode = null;
                        if (is1v1) {
                          const myKey = String(g.current_player?.player_key || '').toLowerCase();
                          const me = playersList.find((p) => String(p.player_key || '').toLowerCase() === myKey) || playersList[0];
                          const opp = playersList.find((p) => p !== me) || playersList[1];
                          const myIcon = getWorkerIconForRace(me?.race);
                          const oppIcon = getWorkerIconForRace(opp?.race);
                          matchupNode = (
                            <span className="workflow-recent-game-matchup">
                              {myIcon ? <img src={myIcon} alt={me?.race || ''} title={me?.race || ''} className="workflow-recent-game-worker-icon" /> : <span>{me?.race || '-'}</span>}
                              <span className="workflow-recent-game-vs">vs</span>
                              {oppIcon ? <img src={oppIcon} alt={opp?.race || ''} title={opp?.race || ''} className="workflow-recent-game-worker-icon" /> : <span>{opp?.race || '-'}</span>}
                            </span>
                          );
                        } else if (g.current_player?.race) {
                          const icon = getWorkerIconForRace(g.current_player.race);
                          matchupNode = (
                            <span className="workflow-recent-game-matchup">
                              {icon ? <img src={icon} alt={g.current_player.race} title={g.current_player.race} className="workflow-recent-game-worker-icon" /> : null}
                              <span>{g.current_player.race}</span>
                            </span>
                          );
                        }
                        return (
                          <button key={g.replay_id} className={`workflow-recent-game-card ${resultClass}`} onClick={() => openMainGame(g.replay_id)}>
                            <div className="workflow-recent-game-header workflow-recent-game-header--left">
                              {isWinner ? <span className="workflow-crown" title="Winner">👑</span> : null}
                              <span>{formatRelativeReplayDate(g.replay_date)}</span>
                              <span>{formatDuration(g.duration_seconds)}</span>
                              <span>{formatMapNameWithKind(g.map_name, g.map_kind)}</span>
                              {matchupNode}
                            </div>
                            <div className="workflow-subtle-note">{renderPlayersMatchup(g.players_label || '')}</div>
                            {filterSummaryPillPatterns(g.current_player?.detected_patterns).length > 0 ? (
                              <div className="workflow-pattern-pills workflow-pattern-pills-compact">
                                {filterSummaryPillPatterns(g.current_player?.detected_patterns).map((pattern, idx) => renderPatternPill(pattern, `recent-${g.replay_id}-${idx}`, undefined, markerRegistry))}
                              </div>
                            ) : null}
                          </button>
                        );
                      })}
                    </div>
                  )}

                  {mainPlayerTab === 'chat-summary' && (
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
                            <div className="workflow-card-subtitle"><span>Last 15 messages</span></div>
                            {(mainPlayerChatSummary?.example_messages || []).map((msg, idx) => (
                              <div key={`player-chat-example-${idx}`} className="workflow-event-row">
                                <span>{msg}</span>
                              </div>
                            ))}
                          </>
                        ) : null
                      )}
                    </div>
                  )}
                </div>
              </>
            ) : (
              <div className="chart-empty">Select a player from a game report.</div>
            )}
          </div>
          );
        })()}

      </div>

      {showGlobalReplayFilter && (
        <GlobalReplayFilterModal
          config={globalReplayFilterConfig}
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
          }}
          onSubmit={handleIngestSubmit}
          onChange={setIngestForm}
          onInputDirChange={setIngestInputDir}
          onSaveInputDir={handleSaveIngestInputDir}
        />
      )}

      {autoIngestNotice ? (
        <div className="ingest-toast">{autoIngestNotice}</div>
      ) : null}

      <div className="app-footer">
        <div className="footer-left">
          {replayCount !== null ? (
            <>
              {replayCount.toLocaleString()} replays in database.{' '}
              <a href="https://github.com/marianogappa/screpdb" target="_blank" rel="noopener noreferrer">screpdb</a>
              {' by '}
              <a href="https://marianogappa.github.io" target="_blank" rel="noopener noreferrer">Mariano Gappa</a>
              {'. '}
              <a href="https://github.com/marianogappa/screpdb/issues" target="_blank" rel="noopener noreferrer">🐞 Report an issue</a>
              {latestVersion ? (
                <>
                  {'. '}
                  <a href={latestVersionUrl || 'https://github.com/marianogappa/screpdb/releases/latest'} target="_blank" rel="noopener noreferrer">
                    🆕 Update available (current version {currentVersion})
                  </a>
                </>
              ) : null}
            </>
          ) : (
            'Loading replay count...'
          )}
        </div>
      </div>
    </div>
  );
}

export default App;
