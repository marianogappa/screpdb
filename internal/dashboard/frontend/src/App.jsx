import React, { useState, useEffect, useLayoutEffect, useMemo, useRef, useCallback } from 'react';
import { api } from './api';
import { useT, t } from './lib/i18nContext';
import { slugKey } from './lib/i18n';
import { countryCodeToFlag, countryCodeToName } from './lib/countries';
import GlobalReplayFilterModal from './components/GlobalReplayFilterModal';
import FilterOmnibar from './components/FilterOmnibar';
import GamingSessionPanel from './components/GamingSessionPanel';
import LanguageSwitcher from './components/LanguageSwitcher';
import Histogram from './components/charts/Histogram';
import TimingScatterRows from './components/charts/TimingScatterRows';
import FirstUnitEfficiencyTimelineRows from './components/charts/FirstUnitEfficiencyTimelineRows';
import BuildOrderTimelineRows from './components/charts/BuildOrderTimelineRows';
import MutaliskTimingChart from './components/charts/MutaliskTimingChart';
import UnitProductionEarlyTimeline from './components/charts/UnitProductionEarlyTimeline';
import SupplyTimeline from './components/charts/SupplyTimeline';
import HotkeyTimeline from './components/charts/HotkeyTimeline';
import HotkeyMaps from './components/charts/HotkeyMaps';
import HotkeySignature from './components/charts/HotkeySignature';
import AllianceTimeline from './components/charts/AllianceTimeline';
import { getUnitIcon, getWorkerIconForRace, normalizeUnitName } from './lib/gameAssets';
import {
  PILL_SURFACES,
  useMarkerRegistry,
  renderPillText,
  pillClassName,
  pillEventTypeClass,
  isBuildOrderEventType,
  isOpenerEventType,
  lookupDefinitionForPattern,
  renderAggregatePillText,
  featureIsBeta,
  betaTooltip,
  markerName,
} from './lib/markerRegistry';

// BetaTag renders a small superscript β with a plain-language tooltip, shown on
// any marker/build-order pill whose detection hasn't been human-curated yet.
const BetaTag = () => {
  const t = useT();
  return (
    <sup className="workflow-beta-tag" title={betaTooltip()} aria-label={t('marker.betaAria')}>β</sup>
  );
};

const fillTemplate = (template, parts) => {
  const out = [];
  const re = /\{(\w+)\}/g;
  let last = 0;
  let match = re.exec(template);
  while (match) {
    if (match.index > last) out.push(template.slice(last, match.index));
    const value = parts[match[1]];
    if (value !== undefined && value !== null && value !== '') {
      out.push(<React.Fragment key={`${match[1]}-${match.index}`}>{value}</React.Fragment>);
    }
    last = match.index + match[0].length;
    match = re.exec(template);
  }
  if (last < template.length) out.push(template.slice(last));
  return <>{out}</>;
};
import {
  CompositionZones,
  CompositionZonesHeader,
  SpellcastsPill,
  SpellcastsChips,
  computeReplayAggregatePhases,
  collectPlayerSpells,
} from './lib/compositionPill';
import {
  getStoredShowFeaturedPros,
  saveShowFeaturedPros,
} from './lib/dashboardStorage';
import { formatLoadingShortWith, isLibraryLoading, stillLoadingCopyWith } from './lib/libraryProgress';
import {
  formatDuration,
  formatMapNameWithKind,
  mapKindEmoji,
  mapKindTooltip,
  formatRelativeReplayDate,
  formatDaysAgoCompact,
  formatPercent,
} from './lib/formatters';
import {
  parseMainRouteSearch,
  buildMainRouteSearch,
  mainRouteHref,
  mainRouteSnapshotEqual,
  shouldLoadPlayerSkillProxyInsights,
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

const normalizeEventType = (eventType) => String(eventType || '').trim().toLowerCase();

/** Aligns with NeverUsedHotkeysPlayerDetector (7+ minute replays). */
const GAME_SUMMARY_NEGATION_MIN_SECONDS = 7 * 60;

const MAIN_GAME_SKILL_PROXY_TABS = ['first-unit-efficiency', 'unit-production-cadence', 'viewport-multitasking'];

// The players-list FilterOmnibar axes. `min_games` is synthesized client-side
// (the API models it as the boolean onlyFivePlus, not an options list).
const buildPlayersOmnibarAxes = (t) => [
  { id: 'lastPlayed', label: t('players.axis.lastPlayed'), state: 'lastPlayed', source: 'last_played' },
  { id: 'minGames', label: t('players.axis.games'), state: 'onlyFivePlus', source: 'min_games' },
];
const buildPlayersOmnibarStateLabels = (t) => ({ lastPlayed: t('players.axis.lastPlayed'), onlyFivePlus: t('players.axis.games') });
const PLAYERS_OMNIBAR_STATE_ORDER = ['lastPlayed', 'onlyFivePlus'];

const isMainGameSkillProxyTab = (tab) => MAIN_GAME_SKILL_PROXY_TABS.includes(tab);

// Per-insight short descriptions for the player Skill proxies > Summary cards.
// APM omitted intentionally (number is self-explanatory in that view).
const playerInsightDescriptionOverride = (insightType) => {
  if (insightType === 'apm') return '';
  if (insightType === 'unit-production-cadence') return t('skillProxies.cadence.description');
  if (insightType === 'viewport-switch-rate') return t('skillProxies.viewport.description');
  return undefined;
};

const DROP_ACTOR_EVENT_TYPES = ['drop', 'cliff_drop'];

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

  // Drop pills — one per variant the player was the actor for. Filter UI
  // dictates the icon layout: generic Drop and Cliff drop keep the
  // single transport icon. The post-process elideGenericDropPill drops the
  // generic "drop" entry when any specific variant fired.
  const transportIcon = dropTransportIconForRace(player?.race);
  if (playerIsActorForGameEventTypes(events, pid, ['drop'])) {
    positive.push({
      key: 'drop',
      domKey: `ge-drop-${pid}`,
      icons: [transportIcon].filter(Boolean),
      label: t('events.drop'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['cliff_drop'])) {
    positive.push({
      key: 'cliff_drop',
      domKey: `ge-cliff-drop-${pid}`,
      icons: [getUnitIcon('dropship')].filter(Boolean),
      label: t('events.cliffDrop'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['cannon_rush'])) {
    positive.push({
      key: `ge-cannon-${pid}`,
      icon: getUnitIcon('photoncannon'),
      label: t('events.cannonRush'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['bunker_rush'])) {
    positive.push({
      key: `ge-bunker-${pid}`,
      icon: getUnitIcon('bunker'),
      label: t('events.bunkerRush'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_gate'])) {
    positive.push({
      key: `ge-proxy-gate-${pid}`,
      icon: getUnitIcon('gateway'),
      label: t('events.proxyGateway'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_rax'])) {
    positive.push({
      key: `ge-proxy-rax-${pid}`,
      icon: getUnitIcon('barracks'),
      label: t('events.proxyBarracks'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_factory'])) {
    positive.push({
      key: `ge-proxy-factory-${pid}`,
      icon: getUnitIcon('factory'),
      label: t('events.proxyFactory'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }
  if (playerIsActorForGameEventTypes(events, pid, ['proxy_starport'])) {
    positive.push({
      key: `ge-proxy-starport-${pid}`,
      icon: getUnitIcon('starport'),
      label: t('events.proxyStarport'),
      className: 'workflow-pattern-pill workflow-pattern-pill-strong',
    });
  }

  return { positive: elideGenericDropPill(positive), noScout: null };
};

const renderGameSummarySignalPill = (pill) => {
  // Backwards-compat: older entries use { icon }, drop pills use { icons: [] }.
  const icons = Array.isArray(pill.icons) ? pill.icons : (pill.icon ? [pill.icon] : []);
  return (
    <span key={pill.domKey || pill.key} className={pill.className} title={pill.title}>
      {icons.map((url, i) => (
        <img key={`${pill.key || pill.domKey}-i${i}`} src={url} alt="" className="workflow-pattern-icon" />
      ))}
      <span>{pill.label}</span>
    </span>
  );
};

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

// joinWithAnd renders a name list as "A and B" / "A, B, and C" — used by the
// alliance event description for 2+ player teams.
const joinWithAnd = (items) => {
  if (!Array.isArray(items) || items.length === 0) return '';
  if (items.length === 1) return items[0];
  if (items.length === 2) return `${items[0]}${t('list.twoSeparator')}${items[1]}`;
  return `${items.slice(0, -1).join(t('list.separator'))}${t('list.lastSeparator')}${items[items.length - 1]}`;
};

// isOpenFieldLocation is true for 1v1 open-field attacks (issue #186) whose
// location is a self-contained relational phrase ("in the middle", "near X's
// base") rather than a base — so it must NOT take an "at" preposition.
const isOpenFieldLocation = (event) => String(event?.base?.kind || '').toLowerCase() === 'open_field';

// isTargetAtOwnNaturalBase is true when the attacked location is the target's
// own natural — so "X attacks Y at Y's natural" renders as the non-stuttering
// "X attacks Y at their natural" (issue #186).
const isTargetAtOwnNaturalBase = (event) => {
  if (String(event?.base?.kind || '').toLowerCase() !== 'natural') return false;
  const targetStart = Number(event?.target_start_clock);
  const naturalOf = event?.base?.natural_of_clock;
  if (naturalOf == null || !Number.isFinite(targetStart)) return false;
  return Number(naturalOf) === targetStart;
};

// attackLocationClause picks the right preposition/phrasing for an attack's
// location: the target's own natural → "at their natural"; open field → the
// bare relational phrase; otherwise "at <base>".
const attackLocationClause = (event, location) => {
  if (isTargetAtOwnNaturalBase(event)) return t('events.location.atTheirNatural');
  if (isOpenFieldLocation(event)) return location;
  return t('events.location.at', { location });
};

const gameEventLocationLabel = (event) => {
  const baseName = String(event?.base?.name || '').trim();
  if (baseName) {
    const isMineralOnly = event?.base?.mineral_only === true;
    if (isMineralOnly && !baseName.toLowerCase().includes('mineral only')) {
      return t('events.location.mineralOnly', { name: baseName });
    }
    return baseName;
  }
  return '';
};

// gameEventTargetLocationLabel renders a recall event's target_base name with
// the same mineral-only suffix convention as gameEventLocationLabel. Returns
// empty string when the event has no target_base (i.e. destination unknown).
const gameEventTargetLocationLabel = (event) => {
  const baseName = String(event?.target_base?.name || '').trim();
  if (!baseName) return '';
  const isMineralOnly = event?.target_base?.mineral_only === true;
  if (isMineralOnly && !baseName.toLowerCase().includes('mineral only')) {
    return t('events.location.mineralOnly', { name: baseName });
  }
  return baseName;
};

// boOpenerLines groups the consolidated "bo_openers" event's per-(player × BO)
// entries into one line per player, preserving the backend's registry ordering
// for each player's BO names. Each line carries the player identity (name,
// color, race, winner, team) needed to render both the events-list row and the
// per-start-location map labels.
const boOpenerLines = (event) => {
  const entries = Array.isArray(event?.build_orders) ? event.build_orders : [];
  const byPlayer = new Map();
  for (const entry of entries) {
    const pid = entry?.player_id;
    if (pid == null) continue;
    const key = String(pid);
    if (!byPlayer.has(key)) {
      byPlayer.set(key, {
        playerID: pid,
        name: String(entry?.name || '').trim() || t('events.playerFallback'),
        color: entry?.color || '',
        race: entry?.race || '',
        isWinner: Boolean(entry?.is_winner),
        team: entry?.team,
        startLocation: String(entry?.start_location || '').trim(),
        boNames: [],
      });
    }
    const rawBoName = String(entry?.build_order || '').trim();
    const boName = t.buildLabel(entry?.feature_key ? t.serverExact(`server.marker.${entry.feature_key}.name`, rawBoName) : rawBoName);
    const mods = (Array.isArray(entry?.modifiers) ? entry.modifiers.filter(Boolean) : [])
      .map((mod) => t.server(`server.name.${slugKey(mod)}`, mod));
    // Fold modifier tags into the opener name inline: "2-Rax Bio (all-in, proxy)".
    const display = boName && mods.length ? `${boName} (${mods.join(', ')})` : boName;
    const line = byPlayer.get(key);
    if (display && !line.boNames.includes(display)) line.boNames.push(display);
  }
  return Array.from(byPlayer.values());
};

// boOpenerLineText renders one opener line as a sentence: "X starts at L and
// opens with BO", dropping whichever clause is missing (no start location, no
// resolved BO, or both — leaving just the name).
const boOpenerLineText = (line) => {
  const bo = line.boNames.join(' / ');
  const params = { name: line.name, location: line.startLocation, bo };
  if (line.startLocation && bo) return t('events.opener.startsAndOpens', params);
  if (line.startLocation) return t('events.opener.starts', params);
  if (bo) return t('events.opener.opens', params);
  return line.name;
};

const RUSH_SENTENCE_KEYS = {
  cannon_rush: 'cannonRush',
  bunker_rush: 'bunkerRush',
  zergling_rush: 'zerglingRush',
};

const PROXY_SENTENCE_KEYS = {
  proxy_gate: 'proxyGateway',
  proxy_rax: 'proxyBarracks',
  proxy_factory: 'proxyFactory',
  proxy_starport: 'proxyStarport',
};

const boEventName = (eventType, registry) => {
  const def = registry?.[eventType];
  return markerName(def) || t.server(`server.marker.${eventType}.name`, prettyPatternName(eventType.replace(/^bo_/, '')));
};

const fallbackEventName = (event) => {
  const rawType = event?.type || 'event';
  return t.server(`server.marker.${normalizeEventType(rawType)}.name`, prettyPatternName(rawType));
};

const gameEventDescription = (event, registry) => {
  const eventType = normalizeEventType(event?.type);
  const actor = String(event?.actor?.name || '').trim();
  const target = String(event?.target?.name || '').trim();
  const location = gameEventLocationLabel(event);

  if (eventType === 'bo_openers') {
    const lines = boOpenerLines(event);
    if (lines.length === 0) return t('events.buildOrders');
    return lines.map((line) => boOpenerLineText(line)).join(t('list.clauseSeparator'));
  }

  if (typeof eventType === 'string' && eventType.startsWith('bo_')) {
    const bo = boEventName(eventType, registry);
    return actor ? t('events.sentence.opensWith', { actor, bo }) : t('events.sentence.opensWithNoActor', { bo });
  }

  if (eventType === 'player_start') {
    if (actor && location) return t('events.sentence.startsAt', { actor, location });
    if (actor) return t('events.sentence.starts', { actor });
    return t('events.playerStart');
  }
  if (eventType === 'leave_game') return actor ? t('events.sentence.leavesGame', { actor }) : t('events.playerLeavesGame');
  if (eventType === 'player_dropped') return actor ? t('events.sentence.droppedFromGame', { actor }) : t('events.playerDroppedFromGame');
  if (eventType === 'mass_disconnect') return actor ? t('events.sentence.massDisconnect', { actor }) : t('events.massDisconnect');
  if (eventType === 'player_stopped_playing') return actor ? t('events.sentence.stopsPlaying', { actor }) : t('events.playerStopsPlaying');
  if (eventType === 'late_alliance') {
    const teams = Array.isArray(event?.alliance_teams) ? event.alliance_teams : [];
    const teamPhrases = teams
      .map((team) => Array.isArray(team) ? team.map((p) => String(p?.name || '').trim()).filter(Boolean) : [])
      .filter((names) => names.length >= 2)
      .map((names) => t('events.sentence.formAlliance', { players: joinWithAnd(names) }));
    if (teamPhrases.length > 0) return teamPhrases.join(t('list.clauseSeparator'));
    if (actor && target) return t('events.sentence.alliesWith', { actor, target });
    return actor ? t('events.sentence.formsAlliance', { actor }) : t('events.alliance');
  }
  if (eventType === 'team_stacking_detected') return t('events.teamStackingDetected');
  if (eventType === 'manner_pylon') {
    if (actor && target) return t('events.sentence.mannerPylonAt', { actor, target });
    return actor ? t('events.sentence.mannerPylon', { actor }) : t('events.mannerPylon');
  }
  if (eventType === 'first_reaver') return actor ? t('events.sentence.firstReaver', { actor }) : t('events.firstReaver');
  if (eventType === 'first_corsair') return actor ? t('events.sentence.firstCorsair', { actor }) : t('events.firstCorsair');
  if (eventType === 'speedlot') return actor ? t('events.sentence.speedlot', { actor }) : t('events.zealotSpeed');
  if (eventType === 'location_inactive') return location ? t('events.locationInactive', { location }) : t('events.locationInactiveNoLocation');
  if (eventType === 'expansion') {
    if (actor && isActorAtOwnNaturalBase(event)) return t('events.sentence.expandsToNatural', { actor });
    return actor && location ? t('events.sentence.expandsTo', { actor, location }) : t('events.expansion');
  }
  if (eventType === 'attack') {
    return actor && target && location
      ? t('events.sentence.attacks', { actor, target, locationClause: attackLocationClause(event, location) })
      : t('events.attack');
  }
  if (eventType === 'scout') return actor && target && location ? t('events.sentence.scouts', { actor, target, location }) : t('events.scout');
  if (eventType === 'cliff_drop' || eventType === 'drop') {
    const fallback = eventType === 'cliff_drop' ? t('events.cliffDrop') : t('events.drop');
    if (!actor || !target || !location) return fallback;
    // event.base is the destination polygon for drops (toReplayEvent stamps
    // DstPolyID there). Source, when worldstate could resolve it, lives on
    // payload.sb → event.source_base.
    const sourceLabel = String(event?.source_base?.name || '').trim();
    const count = Number(event?.drop_count || 0) > 1 ? t('events.fragment.count', { count: event.drop_count }) : '';
    const from = sourceLabel ? t('events.fragment.from', { location: sourceLabel }) : '';
    const params = { actor, target, location, icon: '', count, from };
    return eventType === 'cliff_drop' ? t('events.sentence.cliffDrops', params) : t('events.sentence.drops', params);
  }
  if (eventType === 'recall') {
    // CastRecall's X/Y is the *source* of the teleport; the destination is
    // the Arbiter's location, which the command stream doesn't carry. The
    // backend infers the destination via attack-coincidence + post-recall
    // activity heuristics (see worldstate.emitRecallEvents); when neither
    // proxy fires we surface "(destination unknown)".
    const targetLocation = gameEventTargetLocationLabel(event);
    const targetOwnerName = String(event?.target_owner?.name || '').trim();
    const count = Number(event?.recall_count || 0) > 1 ? t('events.fragment.count', { count: event.recall_count }) : '';
    if (!actor) return t('events.recall');
    const from = location ? t('events.fragment.from', { location }) : '';
    const params = { actor, icon: '', count, from, owner: targetOwnerName, location: targetLocation };
    if (targetLocation) {
      return targetOwnerName ? t('events.sentence.recallsToOwner', params) : t('events.sentence.recallsTo', params);
    }
    return t('events.sentence.recallsUnknown', params);
  }
  if (eventType === 'nydus_attack') {
    if (!actor || !target || !location) return t('events.offensiveNydus');
    return t('events.sentence.nydus', { actor, target, location, icon: '' });
  }
  if (eventType === 'nuke') return actor && target && location ? t('events.sentence.nukes', { actor, target, location }) : t('events.nuke');
  if (RUSH_SENTENCE_KEYS[eventType]) {
    const kind = RUSH_SENTENCE_KEYS[eventType];
    if (actor && target) return t(`events.sentence.${kind}`, { actor, target });
    if (actor && location) return t(`events.sentence.${kind}At`, { actor, location });
    if (actor) return t(`events.sentence.${kind}NoTarget`, { actor });
    return t('events.rush');
  }
  if (eventType === 'takeover') {
    if (actor && isActorAtOwnNaturalBase(event)) return t('events.sentence.takesOverNatural', { actor });
    return actor && location ? t('events.sentence.takesOver', { actor, location }) : t('events.takeover');
  }
  if (PROXY_SENTENCE_KEYS[eventType]) {
    const kind = PROXY_SENTENCE_KEYS[eventType];
    if (actor && location) return t(`events.sentence.${kind}`, { actor, location });
    if (actor) return t(`events.sentence.${kind}NoLocation`, { actor });
    if (location) return t(`events.${kind}At`, { location });
    return t(`events.${kind}`);
  }
  if (eventType === 'became_terran') return actor ? t('events.sentence.becameTerran', { actor }) : t('events.becameTerran');
  if (eventType === 'became_zerg') return actor ? t('events.sentence.becameZerg', { actor }) : t('events.becameZerg');
  if (eventType === 'mech_transition') return actor ? t('events.sentence.mechTransition', { actor }) : t('events.mechTransition');
  return fallbackEventName(event);
};

// The backend marks the user's own players by appending this to their display
// name, so every surface gets it without threading a flag through each payload.
// Splitting it back out here is what lets the marker carry its own tooltip
// rather than sitting inert inside a string.
const YOU_MARKER = '🫵';

// What the Battle.net profile tells us about a player, rendered as a compact
// strip under their name. Every field is optional: the bridge may be off, the
// profile may not be cached yet, and plenty of accounts never ladder.

const PlayerDisplayName = ({ name }) => {
  const t = useT();
  const text = String(name ?? '');
  if (!text.endsWith(YOU_MARKER)) return <>{text}</>;
  return (
    <>
      {text.slice(0, -YOU_MARKER.length).trimEnd()}
      <span className="you-marker" data-tip={t('player.youTip')}>{YOU_MARKER}</span>
    </>
  );
};

// Built-in progamer profiles come from the embedded pro pack, not from the
// user's database. Their player keys carry the "pro:" prefix.
const FEATURED_KEY_PREFIX = 'pro:';
const isFeaturedPlayerKey = (key) => String(key || '').trim().toLowerCase().startsWith(FEATURED_KEY_PREFIX);

const ProPhoto = ({ url, name, large, credit }) => {
  const t = useT();
  const cls = large ? 'pro-photo pro-photo-large' : 'pro-photo';
  if (!url) {
    return <span className={`pro-photo-placeholder ${large ? 'pro-photo-large' : ''}`} aria-hidden="true">★</span>;
  }
  const img = <img className={cls} src={url} alt={name ? t('player.portraitAlt', { name }) : t('player.portraitAltGeneric')} title={credit ? t('player.photoCredit') : undefined} loading="lazy" />;
  if (!credit) return img;
  return <a href={credit} target="_blank" rel="noopener noreferrer" className="pro-photo-link" title={t('player.photoCredit')}>{img}</a>;
};

const FeaturedBadge = ({ compact }) => {
  const t = useT();
  return (
    <span className="featured-badge" title={t('player.featured.badgeTitle')}>
      ★{compact ? '' : t('player.featured.badgeLabel')}
    </span>
  );
};

const featuredOverlayPoints = (points, valueLabel) => (Array.isArray(points) ? points : [])
  .map((point) => ({
    value: Number(point?.value),
    label: String(point?.player_name || '').trim(),
    player_key: String(point?.player_key || '').trim(),
    games_played: Number(point?.games || 0),
    featured: true,
    tooltip_lines: [
      t('player.featured.overlayName', { name: String(point?.player_name || '') }),
      t('appChart.tooltip.labelValue', { label: valueLabel, value: Number(point?.value || 0).toFixed(2) }),
      t('player.featured.sampledGames', { count: Number(point?.games || 0) }),
      t('player.featured.referenceOnly'),
    ],
  }))
  .filter((point) => Number.isFinite(point.value) && point.label);

// Country codes arrive from the SC:R bridge after the page has already
// rendered, because fetching them is rate-limited and deliberately slow. Rather
// than block the render or make the user reload, the page keeps polling a
// cache-only endpoint and publishes what lands here; every flag reads through
// it, so flags appear in place as they resolve.
const CountryFlagOverrideContext = React.createContext(null);

const CountryFlag = ({ code, playerKey }) => {
  const overrides = React.useContext(CountryFlagOverrideContext);
  const resolved = code || (playerKey ? overrides?.[String(playerKey).trim().toLowerCase()] : null);
  if (!resolved) return null;
  const flag = countryCodeToFlag(resolved);
  if (!flag) return null;
  return <span className="country-flag" title={countryCodeToName(resolved)}>{flag}</span>;
};

// COUNTRY_FLAG_POLL_MS paces the cache-only poll. It costs one local DB read and
// no bridge budget, so the cadence is chosen for how fast flags feel like they
// arrive, not for cost. COUNTRY_FLAG_POLL_MAX_TICKS stops a page that will never
// resolve (players with no Battle.net profile) from polling for ever.
// The games list refreshes on library progress at most this often: progress
// events can arrive several times a second during the initial load.

const COUNTRY_FLAG_POLL_MS = 2000;
const COUNTRY_FLAG_POLL_MAX_TICKS = 45;

// useCountryFlagBackfill polls for the country codes of players currently
// rendered without a flag, and returns the codes that have resolved so far.
// Polling stops as soon as nothing is outstanding and the server reports no
// backfill in flight.
function useCountryFlagBackfill(missingKeys, enabled) {
  const [overrides, setOverrides] = useState({});
  const missingSignature = missingKeys.join(',');

  useEffect(() => {
    if (!enabled || missingKeys.length === 0) return undefined;
    let cancelled = false;
    let ticks = 0;
    let timer = null;

    const poll = async () => {
      if (cancelled) return;
      ticks += 1;
      try {
        const res = await api.getBnetCountryCodes(missingKeys);
        if (cancelled) return;
        const codes = res?.country_codes || {};
        if (Object.keys(codes).length > 0) {
          setOverrides((prev) => ({ ...prev, ...codes }));
        }
        const stillMissing = missingKeys.some((k) => !codes[k]);
        if (!stillMissing || (!res?.pending && ticks >= 2) || ticks >= COUNTRY_FLAG_POLL_MAX_TICKS) {
          return;
        }
      } catch {
        // A failed poll is not worth surfacing: the flag simply stays absent.
        if (ticks >= COUNTRY_FLAG_POLL_MAX_TICKS) return;
      }
      timer = setTimeout(poll, COUNTRY_FLAG_POLL_MS);
    };

    timer = setTimeout(poll, COUNTRY_FLAG_POLL_MS);
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
    // missingSignature stands in for missingKeys so a re-render with an
    // equivalent array does not restart the poll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [missingSignature, enabled]);

  return overrides;
}

const FingerprintBadge = ({ match, compact }) => {
  const t = useT();
  if (!match) return null;
  const label = match.confidence === 'high' ? t('fingerprint.likely') : t('fingerprint.possibly');
  const tilde = match.confidence === 'moderate' ? '~' : '';
  const nameEl = match.liquipedia
    ? <a href={match.liquipedia} target="_blank" rel="noopener noreferrer">{match.label}</a>
    : match.label;
  if (compact) {
    return (
      <span className="workflow-fingerprint-icon-wrap">
        <a href="https://github.com/marianogappa/scfingerprint" target="_blank" rel="noopener noreferrer" className="workflow-fingerprint-icon-link">
          {tilde}<span className="workflow-fingerprint-emoji">🔎</span>
        </a>
        <span className="workflow-fingerprint-tooltip">{t('fingerprint.compactTooltip', { confidence: label, name: match.label })}</span>
      </span>
    );
  }
  return (
    <span className="workflow-fingerprint-match-row">
      <span className="workflow-fingerprint-icon-wrap">
        <a href="https://github.com/marianogappa/scfingerprint" target="_blank" rel="noopener noreferrer" className="workflow-fingerprint-icon-link">
          {tilde}<span className="workflow-fingerprint-emoji">🔎</span>
        </a>
        <span className="workflow-fingerprint-tooltip">{t('fingerprint.tooltip')}</span>
      </span>
      <span className="workflow-fingerprint-label">{fillTemplate(t('fingerprint.label'), { confidence: label, name: nameEl })}</span>
    </span>
  );
};

const gamePlayerNameSpan = (player, key) => {
  const name = String(player?.name || '').trim();
  if (!name) return null;
  return (
    <span key={key} className="workflow-event-row-player">
      <PlayerSwatch color={player?.color} title={name} />
      {name}
    </span>
  );
};

// renderGameEventDescription returns the same sentence as gameEventDescription
// but with the actor and target wrapped in colored <span>s. Used for rendering
// the event-row body. The string variant remains for search + dedup keys.
// playerRaceByID lets us inline race-correct icons (vessel for drops) without
// requiring the backend to embed race on every event row.
const renderGameEventDescription = (event, registry, playerRaceByID) => {
  const eventType = normalizeEventType(event?.type);
  const actorName = String(event?.actor?.name || '').trim();
  const targetName = String(event?.target?.name || '').trim();
  const location = gameEventLocationLabel(event);
  const actorSpan = gamePlayerNameSpan(event?.actor, 'a');
  const targetSpan = gamePlayerNameSpan(event?.target, 't');
  const fill = (key, parts) => fillTemplate(t(key), parts);

  if (eventType === 'bo_openers') {
    const lines = boOpenerLines(event);
    if (lines.length === 0) return t('events.buildOrders');
    return (
      <span className="workflow-bo-openers">
        {lines.map((line) => {
          const raceIcon = getRaceIcon(line.race);
          const nameSpan = (
            <span className="workflow-event-row-player">
              <PlayerSwatch color={line.color} title={line.name} />
              {line.name}
            </span>
          );
          const bo = line.boNames.join(' / ');
          const parts = { name: nameSpan, location: line.startLocation, bo };
          return (
            <span key={`bo-line-${line.playerID}`} className="workflow-bo-openers-line">
              {raceIcon ? <img src={raceIcon} alt={line.race || t('race.alt')} className="unit-icon-inline workflow-bo-openers-race" /> : null}
              {line.isWinner ? <span className="workflow-crown" title={t('common.winner')}>👑</span> : null}
              {line.startLocation && bo ? fill('events.opener.startsAndOpens', parts)
                : line.startLocation ? fill('events.opener.starts', parts)
                  : bo ? fill('events.opener.opens', parts)
                    : nameSpan}
            </span>
          );
        })}
      </span>
    );
  }

  if (typeof eventType === 'string' && eventType.startsWith('bo_')) {
    const bo = boEventName(eventType, registry);
    if (!actorName) return t('events.sentence.opensWithNoActor', { bo });
    return fill('events.sentence.opensWith', { actor: actorSpan, bo });
  }

  if (eventType === 'player_start') {
    if (actorName && location) return fill('events.sentence.startsAt', { actor: actorSpan, location });
    if (actorName) return fill('events.sentence.starts', { actor: actorSpan });
    return t('events.playerStart');
  }
  if (eventType === 'leave_game') return actorName ? fill('events.sentence.leavesGame', { actor: actorSpan }) : t('events.playerLeavesGame');
  if (eventType === 'player_dropped') return actorName ? fill('events.sentence.droppedFromGame', { actor: actorSpan }) : t('events.playerDroppedFromGame');
  if (eventType === 'mass_disconnect') return actorName ? fill('events.sentence.massDisconnect', { actor: actorSpan }) : t('events.massDisconnect');
  if (eventType === 'player_stopped_playing') return actorName ? fill('events.sentence.stopsPlaying', { actor: actorSpan }) : t('events.playerStopsPlaying');
  if (eventType === 'late_alliance') {
    const teams = Array.isArray(event?.alliance_teams) ? event.alliance_teams : [];
    const teamPhrases = teams
      .map((team) => Array.isArray(team) ? team.filter((p) => String(p?.name || '').trim()) : [])
      .filter((row) => row.length >= 2);
    if (teamPhrases.length > 0) {
      return (
        <>
          {teamPhrases.map((row, ti) => {
            const spans = row.map((p, pi) => (
              <React.Fragment key={`a-${ti}-${pi}`}>
                {gamePlayerNameSpan(p, `a-${ti}-p-${pi}`)}
                {pi < row.length - 2 ? t('list.separator') : null}
                {pi === row.length - 2 ? (row.length === 2 ? t('list.twoSeparator') : t('list.lastSeparator')) : null}
              </React.Fragment>
            ));
            return (
              <React.Fragment key={`team-${ti}`}>
                {ti > 0 ? t('list.clauseSeparator') : null}
                {fill('events.sentence.formAlliance', { players: spans })}
              </React.Fragment>
            );
          })}
        </>
      );
    }
    if (actorName && targetName) return fill('events.sentence.alliesWith', { actor: actorSpan, target: targetSpan });
    return actorName ? fill('events.sentence.formsAlliance', { actor: actorSpan }) : t('events.alliance');
  }
  if (eventType === 'team_stacking_detected') return t('events.teamStackingDetected');
  if (eventType === 'manner_pylon') {
    if (actorName && targetName) return fill('events.sentence.mannerPylonAt', { actor: actorSpan, target: targetSpan });
    return actorName ? fill('events.sentence.mannerPylon', { actor: actorSpan }) : t('events.mannerPylon');
  }
  if (eventType === 'first_reaver') return actorName ? fill('events.sentence.firstReaver', { actor: actorSpan }) : t('events.firstReaver');
  if (eventType === 'first_corsair') return actorName ? fill('events.sentence.firstCorsair', { actor: actorSpan }) : t('events.firstCorsair');
  if (eventType === 'speedlot') return actorName ? fill('events.sentence.speedlot', { actor: actorSpan }) : t('events.zealotSpeed');
  if (eventType === 'location_inactive') return location ? t('events.locationInactive', { location }) : t('events.locationInactiveNoLocation');
  if (eventType === 'expansion') {
    if (actorName && isActorAtOwnNaturalBase(event)) return fill('events.sentence.expandsToNatural', { actor: actorSpan });
    return actorName && location ? fill('events.sentence.expandsTo', { actor: actorSpan, location }) : t('events.expansion');
  }
  if (eventType === 'attack') {
    return actorName && targetName && location
      ? fill('events.sentence.attacks', { actor: actorSpan, target: targetSpan, locationClause: attackLocationClause(event, location) })
      : t('events.attack');
  }
  if (eventType === 'scout') {
    return actorName && targetName && location
      ? fill('events.sentence.scouts', { actor: actorSpan, target: targetSpan, location })
      : t('events.scout');
  }
  if (eventType === 'nydus_attack') {
    if (!actorName || !targetName || !location) return t('events.offensiveNydus');
    const nydusURL = getUnitIcon('nyduscanal');
    const nydusName = t.server('server.name.nydus_canal', 'Nydus Canal');
    const nydusIcon = nydusURL ? (
      <img src={nydusURL} alt={nydusName} title={nydusName} className="workflow-event-row-recall-arbiter" />
    ) : null;
    return fill('events.sentence.nydus', { actor: actorSpan, target: targetSpan, location, icon: nydusIcon });
  }
  if (eventType === 'cliff_drop' || eventType === 'drop') {
    const fallback = eventType === 'cliff_drop' ? t('events.cliffDrop') : t('events.drop');
    if (!actorName || !targetName || !location) return fallback;
    const sourceLabel = String(event?.source_base?.name || '').trim();
    const count = Number(event?.drop_count || 0) > 1 ? t('events.fragment.count', { count: event.drop_count }) : '';
    const from = sourceLabel ? t('events.fragment.from', { location: sourceLabel }) : '';
    // Inline vessel icon right after the verb — race-correct transport. Mirrors
    // the Arbiter-icon-after-"recalls" pattern. The trailing row-icon strip
    // strips the vessel to avoid duplication (see gameEventRowIconEntries).
    const actorPidForRace = Number(event?.actor?.player_id || 0);
    const actorRace = playerRaceByID && actorPidForRace > 0 ? (playerRaceByID.get(actorPidForRace) || '') : '';
    const vesselURL = dropTransportIconForRace(actorRace);
    const vesselName = (() => {
      const r = actorRace.toLowerCase();
      if (r === 'terran') return t.server('server.name.dropship', 'Dropship');
      if (r === 'protoss') return t.server('server.name.shuttle', 'Shuttle');
      if (r === 'zerg') return t.server('server.name.overlord', 'Overlord');
      return t('events.transportAlt');
    })();
    const vesselIcon = vesselURL ? (
      <img src={vesselURL} alt={vesselName} title={vesselName} className="workflow-event-row-recall-arbiter" />
    ) : null;
    const parts = { actor: actorSpan, target: targetSpan, location, icon: vesselIcon, count, from };
    return eventType === 'cliff_drop' ? fill('events.sentence.cliffDrops', parts) : fill('events.sentence.drops', parts);
  }
  if (eventType === 'recall') {
    if (!actorName) return t('events.recall');
    const targetLocation = gameEventTargetLocationLabel(event);
    const targetOwner = event?.target_owner;
    const targetOwnerSpan = gamePlayerNameSpan(targetOwner, 'to');
    const targetOwnerName = String(targetOwner?.name || '').trim();
    const count = Number(event?.recall_count || 0) > 1 ? t('events.fragment.count', { count: event.recall_count }) : '';
    const arbiterIconURL = getUnitIcon('arbiter');
    const arbiterName = t.server('server.name.arbiter', 'Arbiter');
    const arbiterIcon = arbiterIconURL ? (
      <img src={arbiterIconURL} alt={arbiterName} title={arbiterName} className="workflow-event-row-recall-arbiter" />
    ) : null;
    const from = location ? t('events.fragment.from', { location }) : '';
    const parts = { actor: actorSpan, icon: arbiterIcon, count, from, owner: targetOwnerSpan, location: targetLocation };
    if (targetLocation) {
      return targetOwnerName ? fill('events.sentence.recallsToOwner', parts) : fill('events.sentence.recallsTo', parts);
    }
    return fill('events.sentence.recallsUnknown', parts);
  }
  if (eventType === 'nuke') {
    return actorName && targetName && location
      ? fill('events.sentence.nukes', { actor: actorSpan, target: targetSpan, location })
      : t('events.nuke');
  }
  if (RUSH_SENTENCE_KEYS[eventType]) {
    const kind = RUSH_SENTENCE_KEYS[eventType];
    if (actorName && targetName) return fill(`events.sentence.${kind}`, { actor: actorSpan, target: targetSpan });
    if (actorName && location) return fill(`events.sentence.${kind}At`, { actor: actorSpan, location });
    if (actorName) return fill(`events.sentence.${kind}NoTarget`, { actor: actorSpan });
    return t('events.rush');
  }
  if (eventType === 'takeover') {
    if (actorName && isActorAtOwnNaturalBase(event)) return fill('events.sentence.takesOverNatural', { actor: actorSpan });
    return actorName && location ? fill('events.sentence.takesOver', { actor: actorSpan, location }) : t('events.takeover');
  }
  if (PROXY_SENTENCE_KEYS[eventType]) {
    const kind = PROXY_SENTENCE_KEYS[eventType];
    if (actorName && location) return fill(`events.sentence.${kind}`, { actor: actorSpan, location });
    if (actorName) return fill(`events.sentence.${kind}NoLocation`, { actor: actorSpan });
    if (location) return t(`events.${kind}At`, { location });
    return t(`events.${kind}`);
  }
  if (eventType === 'became_terran') return actorName ? fill('events.sentence.becameTerran', { actor: actorSpan }) : t('events.becameTerran');
  if (eventType === 'became_zerg') return actorName ? fill('events.sentence.becameZerg', { actor: actorSpan }) : t('events.becameZerg');
  if (eventType === 'mech_transition') return actorName ? fill('events.sentence.mechTransition', { actor: actorSpan }) : t('events.mechTransition');
  return fallbackEventName(event);
};

const gameEventSearchText = (event, registry) => {
  const parts = [
    gameEventDescription(event, registry),
    event?.type,
    String(event?.type || '').replace(/_/g, ' '),
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

/** The replay's player colour, as a swatch welded to the name.
 *
 *  It used to be the name's `color`. The SC:R palette was designed as unit
 *  fills over bright terrain, not as 12px text on #0f1016: Brown came out at
 *  1.92:1 and Navy at 1.94:1 against the app background, where WCAG AA wants
 *  4.5:1. The old workaround bolted a white text-shadow onto a hardcoded
 *  `black | navy | darkblue` allowlist — which missed Brown, the worst case.
 *
 *  A swatch fixes both halves: the hue keeps full chroma (so the colour you
 *  remember from the game still finds the name instantly, and still keys it to
 *  the base on the map) while the name itself renders in legible ink. */
/** Labels drawn ON the map keep the player colour: they sit over terrain, where
 *  the hue is exactly the signal, and a dark outline makes any hue legible over
 *  any tileset — unlike the old white halo, which was applied to a hardcoded
 *  three-colour allowlist and missed the worst offender. */
const mapLabelStyle = (color) => ({
  color: playerColorToCss(color),
  textShadow: '0 1px 3px rgba(0, 0, 0, 0.95), 0 0 2px rgba(0, 0, 0, 0.9)',
});

const PlayerSwatch = ({ color, title }) => {
  const css = playerColorToCss(color);
  if (!css) return null;
  return (
    <span
      className="rd-swatch"
      style={{ background: css }}
      title={title || String(color || '')}
      aria-hidden="true"
    />
  );
};

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
          >
            <PlayerSwatch color={item.color} title={item.name} />
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
              style={{ fill: `${overlay.ownerColor}66`, stroke: overlay.teamColor || overlay.ownerColor, strokeWidth: overlay.strokeWidth || 0.4 }}
            >
              <title>{overlay.ownerName}</title>
            </polygon>
          ))}
        </svg>
      ) : null}
    </div>
  </>
);

// renderMapNameWithKind renders the map name with its kind emoji as a hover
// target (instant "Money map" tooltip) instead of the plain prefixed string.
const renderMapNameWithKind = (mapName, mapKind) => {
  const emoji = mapKindEmoji(mapKind);
  const tip = mapKindTooltip(mapKind);
  return (
    <>
      {emoji ? <span className="workflow-map-kind-emoji" data-tip={tip}>{emoji}</span> : null}
      {emoji ? ' ' : ''}
      {String(mapName || '')}
    </>
  );
};

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
    if (t === 'proxy_starport') keys.add('proxy_starport');
    // Drop variants: every variant lights the generic 'drop' key; specific
    // subtypes (cliff_drop) also light their own
    // key. The post-process elision below drops the generic chip when a
    // specific variant is present (avoids redundant "Drop + Cliff drop").
    if (t === 'drop' || t === 'cliff_drop') {
      keys.add('drop');
      keys.add(t);
    }
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

// buildMainGameFeaturingPills produces the ordered pill list for the replay
// summary "Featuring" strip. Ordering + game-event-only metadata (cannon_rush,
// bunker_rush, zergling_rush, mind_control) come from the backend-provided
// featuring_order and game_event_features lists. Marker pills come from the
// marker registry's games_list field; markers without one surface via a minimal
// fallback.
//
// Build-order openers are deliberately EXCLUDED here: the summary strip is for
// game-characterising signatures/events (Carriers, drops, nukes, Double
// Stargate, …), not the opener — which has its own per-player pill and Build
// Orders tab. (The games-list rows keep their opener chip; that's a separate,
// server-built `game.featuring` path.)
//
// Post-process: when a more-specific drop variant pill is present
// (cliff_drop), the generic "drop" pill is elided
// so the strip doesn't carry both "Drop" + "Cliff drop".
const buildMainGameFeaturingPills = (mainGame, markerDefs) => {
  if (!mainGame) return [];
  const { keys, rowByKey } = collectFeaturingKeysFromMainGame(mainGame);
  const registry = markerDefs?.markers || {};
  const order = Array.isArray(markerDefs?.featuring_order) ? markerDefs.featuring_order : [];
  const gameEventFeaturesByKey = {};
  (markerDefs?.game_event_features || []).forEach((f) => { gameEventFeaturesByKey[f.key] = f; });

  const pills = order
    .filter((key) => keys.has(key))
    // Never show build-order openers on the summary featuring strip.
    .filter((key) => registry[key]?.kind !== 'initial_build_order')
    .map((key) => {
      const def = registry[key];
      if (def?.games_list) {
        // Resolve via renderPillText so {minute}/{timestamp}/{subject}
        // tokens in the games_list label/icon_key get interpolated against
        // the matching detected-pattern row (when one exists).
        const rendered = renderPillText(def, PILL_SURFACES.gamesList, rowByKey[key]);
        if (rendered) {
          return { key, label: rendered.label || markerName(def), iconKey: rendered.iconKey || '', beta: featureIsBeta(def) };
        }
        return { key, label: t.server(`server.marker.${key}.games_list.label`, def.games_list.label) || markerName(def), iconKey: def.games_list.icon_key || '', beta: featureIsBeta(def) };
      }
      const ge = gameEventFeaturesByKey[key];
      if (ge) return { key, label: t.server(`server.game_event.${key}.label`, ge.label), iconKey: ge.icon_key || '', iconKeys: ge.icon_keys || [] };
      return { key, label: markerName(def) || key, iconKey: '', beta: featureIsBeta(def) };
    });

  return elideGenericDropPill(pills);
};

// elideGenericDropPill removes the generic "drop" pill from a pill list when
// any more-specific drop variant (cliff_drop) is
// present in the same list. Operates on entries shaped like { key, ... } so
// the same helper can be reused across the main featuring strip, per-player
// signal pills, and the games-list table column.
const SPECIFIC_DROP_KEYS = new Set(['cliff_drop']);
const elideGenericDropPill = (pills) => {
  if (!Array.isArray(pills) || pills.length === 0) return pills;
  const hasSpecific = pills.some((p) => SPECIFIC_DROP_KEYS.has(String(p?.key || '')));
  if (!hasSpecific) return pills;
  return pills.filter((p) => String(p?.key || '') !== 'drop');
};

// elideGenericDropLabels mirrors elideGenericDropPill for the games-list
// table, whose Featuring column carries plain strings ("Drop", "Cliff drop",
// "Cliff drop 7:59") rather than {key, ...} objects. We match on the
// pre-timestamp prefix so suffixes like " 7:59" don't break detection.
const SPECIFIC_DROP_LABEL_PREFIXES = ['Cliff drop'];
const elideGenericDropLabels = (labels) => {
  if (!Array.isArray(labels) || labels.length === 0) return labels;
  const startsWith = (s, prefix) => typeof s === 'string' && (s === prefix || s.startsWith(`${prefix} `));
  const hasSpecific = labels.some((l) => SPECIFIC_DROP_LABEL_PREFIXES.some((p) => startsWith(l, p)));
  if (!hasSpecific) return labels;
  return labels.filter((l) => !startsWith(l, 'Drop'));
};

// FeaturingCell keeps the games-list Featuring column on a single row, cropping
// overflow. When the pills overflow, a trailing "…" toggle expands the cell to
// wrap and show them all; the toggle stops click propagation so it doesn't open
// the game the row links to.
/** Featuring is the derived insight — the reason this table exists — so every
 *  marker renders. It used to crop to one row behind a "…" toggle, which hid
 *  content in 83 of 100 rows while the player names beside it rendered in full. */
function FeaturingCell({ featuring, featuringKeys }) {
  const t = useT();
  const keyByLabel = new Map((featuring || []).map((label, idx) => [label, featuringKeys?.[idx] || '']));
  const labels = elideGenericDropLabels(featuring || []);

  if (labels.length === 0) {
    return <span className="rd-no-feature">{t('games.noMarkers')}</span>;
  }

  return (
    <div className="workflow-featuring-wrap">
      <div className="workflow-pattern-pills">
        {labels.map((pill, pillIdx) => {
          const key = keyByLabel.get(pill);
          return (
            <span key={`${pillIdx}-${pill}`} className="workflow-pattern-pill workflow-feature-pill">
              <span>{key ? t.server(`server.chip.featuring.${key}.label`, pill) : t.buildLabel(pill)}</span>
            </span>
          );
        })}
      </div>
    </div>
  );
}

const renderFeaturingPill = (pill, keyPrefix) => {
  const iconKeys = (Array.isArray(pill.iconKeys) && pill.iconKeys.length)
    ? pill.iconKeys
    : (pill.iconKey ? [pill.iconKey] : []);
  const iconUrls = iconKeys.map((k) => getUnitIcon(k)).filter(Boolean);
  // Build-order pills get the teal opener accent + a "BUILD ORDER" legend on
  // the top border, so they read as the opening across every surface.
  const isBO = isBuildOrderEventType(pill.key);
  const variantClass = isBO ? 'workflow-pattern-pill-bo workflow-pill-legended' : 'workflow-pattern-pill-strong';
  return (
    <span key={`${keyPrefix}-${pill.key}`} className={`workflow-pattern-pill ${variantClass} workflow-summary-feature-pill`}>
      {isBO ? <span className="workflow-pill-legend">{t('pill.buildOrderLegend')}</span> : null}
      {iconUrls.map((url, i) => (
        <img key={`${pill.key}-i${i}`} src={url} alt="" className="workflow-pattern-icon" />
      ))}
      <span>{pill.label}</span>
      {pill.beta ? <BetaTag /> : null}
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

// polygonBoundingBox returns axis-aligned min/max in the same coordinate
// space as the polygon vertices (pixel-space when called on raw event
// polygons; percent-space when called on already-converted overlay polys).
// Returns null for malformed polygons (<3 vertices).
const polygonBoundingBox = (polygon) => {
  if (!Array.isArray(polygon) || polygon.length < 3) return null;
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  let count = 0;
  polygon.forEach((p) => {
    const x = Number(p?.x);
    const y = Number(p?.y);
    if (!Number.isFinite(x) || !Number.isFinite(y)) return;
    if (x < minX) minX = x;
    if (y < minY) minY = y;
    if (x > maxX) maxX = x;
    if (y > maxY) maxY = y;
    count += 1;
  });
  if (count === 0) return null;
  return { minX, minY, maxX, maxY };
};

// polygonBoundingBoxArea is the area of the axis-aligned bounding box of
// the polygon (not the polygon's true area). Used to compare relative
// "size" of a player's owned bases when picking an anchor polygon for the
// trained-units overlay.
const polygonBoundingBoxArea = (polygon) => {
  const bb = polygonBoundingBox(polygon);
  if (!bb) return 0;
  return Math.max(0, bb.maxX - bb.minX) * Math.max(0, bb.maxY - bb.minY);
};

// distanceBetween returns the Euclidean distance between two {x, y} points.
// Returns Infinity if either argument is missing.
const distanceBetween = (a, b) => {
  if (!a || !b) return Infinity;
  const dx = Number(a.x) - Number(b.x);
  const dy = Number(a.y) - Number(b.y);
  if (!Number.isFinite(dx) || !Number.isFinite(dy)) return Infinity;
  return Math.sqrt(dx * dx + dy * dy);
};

// distanceToSegment returns the orthogonal distance from point p to the
// line segment from→to, clamping at the segment endpoints (so points "past"
// either end measure to that endpoint, not the infinite line). All
// arguments are {x, y}.
const distanceToSegment = (p, from, to) => {
  if (!p || !from || !to) return Infinity;
  const px = Number(p.x);
  const py = Number(p.y);
  const ax = Number(from.x);
  const ay = Number(from.y);
  const bx = Number(to.x);
  const by = Number(to.y);
  if (![px, py, ax, ay, bx, by].every(Number.isFinite)) return Infinity;
  const dx = bx - ax;
  const dy = by - ay;
  const lenSq = dx * dx + dy * dy;
  if (lenSq <= 1e-9) return Math.sqrt((px - ax) * (px - ax) + (py - ay) * (py - ay));
  let t = ((px - ax) * dx + (py - ay) * dy) / lenSq;
  t = Math.max(0, Math.min(1, t));
  const cx = ax + t * dx;
  const cy = ay + t * dy;
  return Math.sqrt((px - cx) * (px - cx) + (py - cy) * (py - cy));
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

// Recall is in this list now that the backend infers the destination — the
// arrow draws from the cast point (source) to the inferred Arbiter location.
// The recall arrow is suppressed at render time when no destination was
// inferred (target_base missing) so we don't draw a misleading vector.
const isArrowEventType = (eventType) => ['attack', 'scout', 'drop', 'cliff_drop', 'nydus_attack', 'nuke', 'cannon_rush', 'bunker_rush', 'zergling_rush', 'proxy_gate', 'proxy_rax', 'proxy_factory', 'proxy_starport', 'recall'].includes(String(eventType || '').toLowerCase());

const fallbackOverlayUnitNamesForEvent = (eventType, actorRace) => {
  const normalized = normalizeEventType(eventType);
  if (normalized === 'zergling_rush') return ['zergling'];
  if (normalized === 'cannon_rush') return ['photoncannon'];
  if (normalized === 'bunker_rush') return ['bunker'];
  if (normalized === 'proxy_gate') return ['gateway'];
  if (normalized === 'proxy_rax') return ['barracks'];
  if (normalized === 'proxy_factory') return ['factory'];
  if (normalized === 'proxy_starport') return ['starport'];
  // cliff_drop is a Terran-only marker classification, dropship is always correct.
  if (normalized === 'cliff_drop') return ['dropship'];
  if (normalized === 'drop') {
    const r = String(actorRace || '').toLowerCase();
    if (r === 'protoss') return ['shuttle'];
    if (r === 'zerg') return ['overlord'];
    return ['dropship'];
  }
  if (normalized === 'nuke') return ['ghost'];
  if (normalized === 'nydus_attack') return ['nyduscanal'];
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
    const label = boEventName(normalized, registry);
    return [{ src: icon, alt: label, title: label }];
  }
  if (normalized === 'leave_game') {
    return [{ emoji: '🏳️', alt: t('events.icon.leftGame.alt'), title: t('events.icon.leftGame.title') }];
  }
  if (normalized === 'player_dropped') {
    return [{ emoji: '🔌', alt: t('events.icon.dropped.alt'), title: t('events.icon.dropped.title') }];
  }
  if (normalized === 'mass_disconnect') {
    return [{ emoji: '🔌', alt: t('events.icon.massDisconnect.alt'), title: t('events.icon.massDisconnect.title') }];
  }
  if (normalized === 'player_stopped_playing') {
    return [{ emoji: '💤', alt: t('events.icon.stoppedPlaying.alt'), title: t('events.icon.stoppedPlaying.title') }];
  }
  if (normalized === 'late_alliance') {
    return [{ emoji: '🤝', alt: t('events.icon.lateAlliance.alt'), title: t('events.icon.lateAlliance.title') }];
  }
  if (normalized === 'team_stacking_detected') {
    return [{ emoji: '😈', alt: t('events.icon.teamStacking.alt'), title: t('events.icon.teamStacking.title') }];
  }
  if (normalized === 'expansion' || normalized === 'takeover') {
    const icon = getExpansionMarkerIconForRace(actorRace);
    if (!icon) return [];
    return [{ src: icon, alt: t('events.icon.townhall.alt'), title: t('events.expansion') }];
  }
  let unitNames = Array.isArray(event?.attack_unit_types) && event.attack_unit_types.length > 0
    ? event.attack_unit_types
    : fallbackOverlayUnitNamesForEvent(event?.type, actorRace);
  // Recalls render the Arbiter inline next to the verb in the row body
  // (see renderGameEventDescription), so drop it from the trailing icon
  // strip to avoid duplication. The remaining units are what got recalled.
  if (normalized === 'recall') {
    unitNames = unitNames.filter((name) => String(name || '').toLowerCase() !== 'arbiter');
  }
  // Drops render the vessel (Dropship/Shuttle/Overlord) inline next to the
  // verb in the row body. Strip it from the trailing icon strip so the
  // trailing icons are just the dropped units.
  if (['drop', 'cliff_drop'].includes(normalized)) {
    const transports = new Set(['dropship', 'shuttle', 'overlord']);
    unitNames = unitNames.filter((name) => !transports.has(String(name || '').toLowerCase()));
  }
  const seen = new Set();
  const entries = [];
  for (const name of unitNames) {
    const icon = getUnitIcon(name);
    if (!icon) continue;
    if (seen.has(icon)) continue;
    seen.add(icon);
    const unitLabel = t.server(`server.name.${slugKey(name)}`, name);
    entries.push({ src: icon, alt: unitLabel, title: unitLabel });
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

// Top-level timing tabs:
//   * Expansion & Gas — both economic timings overlaid (image markers).
//   * Upgrades & Tech — non-HP research families overlaid via checkboxes,
//     colour-coded, with the item name on each dot.
//   * HP Upgrades — weapon/armor/shield tiers, with a per-race filter (these
//     repeat per level and differ per race, so they need their own view).
const TIMING_CATEGORY_CONFIG = [
  { id: 'expansion_gas', labelKey: 'timings.category.expansionGas', source: 'expansion_gas', markerMode: 'image' },
  { id: 'research', labelKey: 'timings.category.research', source: 'research' },
  { id: 'hp_upgrades', labelKey: 'timings.category.hpUpgrades', source: 'upgrades' },
];

// Sub-categories overlaid within the "Upgrades & Tech" tab. Each carries a
// distinct colour so overlaid families stay visually separable. HP upgrades
// are intentionally excluded — they have their own tab.
const RESEARCH_SUBCATEGORIES = [
  { id: 'unit_range', labelKey: 'timings.sub.unitRange', source: 'upgrades', color: '#60a5fa' },
  { id: 'unit_speed', labelKey: 'timings.sub.unitSpeed', source: 'upgrades', color: '#a78bfa' },
  { id: 'energy', labelKey: 'timings.sub.energy', source: 'upgrades', color: '#22d3ee' },
  { id: 'capacity_cooldown_damage', labelKey: 'timings.sub.capacity', source: 'upgrades', color: '#f472b6' },
  { id: 'tech', labelKey: 'timings.sub.tech', source: 'tech', color: '#84cc16' },
];

const RESEARCH_SUBCATEGORY_BY_ID = Object.fromEntries(RESEARCH_SUBCATEGORIES.map((s) => [s.id, s]));

// Sensible default overlay: the timings most players actually compare —
// movement/range upgrades and tech — without the rarer energy/capacity noise.
const DEFAULT_RESEARCH_SUBCATEGORIES = ['unit_range', 'unit_speed', 'tech'];

const TIMING_RACE_ORDER = ['terran', 'zerg', 'protoss'];

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

const prettyRaceName = (race) => {
  const value = String(race || '').trim().toLowerCase();
  if (value === 'terran') return t('race.terran');
  if (value === 'zerg') return t('race.zerg');
  if (value === 'protoss') return t('race.protoss');
  return race || t('race.unknown');
};

const raceLabel = (race) => (race ? prettyRaceName(race) : '');

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
  attack: false,
  expansion: false,
  leaves: false,
  nuke: false,
  drop: false,
  recall: false,
  becameRace: false,
  rush: false,
  alliance: false,
  scout: false,
};

const SUMMARY_TOPIC_PATTERNS = {
  attack: /\battacks?\b/i,
  expansion: /\bexpands?\b|\bexpansion\b/i,
  leaves: /\bleaves the game\b|\bstops playing\b|\bleave game\b|\bplayer dropped\b|\bmass disconnect\b|\bstopped playing\b/i,
  nuke: /\bnuke|nuclear\b/i,
  drop: /\bdrop|dropship|shuttle\b/i,
  recall: /\brecall\b/i,
  becameRace: /\b(became|becomes)\s+(terran|zerg)\b|\bbecame_(terran|zerg)\b/i,
  rush: /\brush|all[\s-]?in|cheese\b/i,
  alliance: /\bform(s)? an alliance\b|\ballies with\b|\balliance\b/i,
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
// drop game_events when the caller sets
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
  // The opening build order always leads the row (it's the headline signal and
  // now has a guaranteed pill). After that, pills read chronologically by
  // detected_second — mid-game markers like SK Terran / Mech Transition, then
  // hotkey/absence markers which carry an end-of-replay second. Patterns
  // without a detected_second sort to the end. Stable tiebreak via index.
  const indexed = filtered.map((pattern, idx) => ({ pattern, idx }));
  indexed.sort((a, b) => {
    const aBO = isBuildOrderEventType(a.pattern?.event_type) ? 0 : 1;
    const bBO = isBuildOrderEventType(b.pattern?.event_type) ? 0 : 1;
    if (aBO !== bBO) return aBO - bBO;
    const ta = Number.isFinite(a.pattern?.detected_second) ? a.pattern.detected_second : Number.POSITIVE_INFINITY;
    const tb = Number.isFinite(b.pattern?.detected_second) ? b.pattern.detected_second : Number.POSITIVE_INFINITY;
    if (ta !== tb) return ta - tb;
    return a.idx - b.idx;
  });
  return indexed.map((entry) => entry.pattern);
};

const renderPatternPill = (pattern, keyPrefix, team, registry) => {
  if (!registry) return null;
  const def = lookupDefinitionForPattern(registry, pattern);
  if (!def) return null;
  const rendered = renderPillText(def, PILL_SURFACES.summaryPlayer, pattern);
  if (!rendered) return null;
  // Opener pills (a build order, or the "opener unresolved" N/A) carry a small
  // "BUILD ORDER" legend on their top border instead of an inline prefix — the
  // legend + accent colour identify the pill type.
  const isOpener = isOpenerEventType(pattern?.event_type);
  // A top-border legend names the pill type (opener); the Spellcasts pill
  // carries its own legend (see SpellcastsPill).
  const legendText = isOpener ? 'pill.buildOrderLegend' : null;
  const className = `${pillClassName(rendered.style)} ${pillEventTypeClass(pattern?.event_type)} ${legendText ? 'workflow-pill-legended' : ''}`.trim();
  const key = `${keyPrefix}-${team ? `team-${team}-` : ''}${pattern?.event_type || ''}-${pattern?.detected_second ?? ''}`;
  return (
    <span key={key} className={className} title={rendered.title || undefined}>
      {legendText ? <span className="workflow-pill-legend">{t(legendText)}</span> : null}
      {team !== undefined ? <span className="team-dot" style={{ backgroundColor: getTeamColor(team) }}></span> : null}
      {rendered.icon ? <img src={rendered.icon} alt="" className="workflow-pattern-icon" /> : null}
      {rendered.label ? <span>{rendered.label}</span> : null}
      {featureIsBeta(def) ? <BetaTag /> : null}
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
  unitProductionCadence: 'unit-production-cadence',
  viewportSwitchRate: 'viewport-switch-rate',
};

const VIEWPORT_SWITCH_RATE_FIELDS = {
  playerField: 'average_viewport_switch_rate',
  gameField: 'viewport_switch_rate',
};

const viewportSwitchRateText = (t) => ({
  title: t('skillProxies.viewport.title'),
  axisLabel: t('skillProxies.viewport.axisLabel'),
  overlayValueLabel: t('skillProxies.viewport.overlayValueLabel'),
  valueFormatter: (value) => t('skillProxies.viewport.value', { value: Number(value || 0).toFixed(2) }),
  summaryFormatter: (value) => `${Number(value || 0).toFixed(2)}`,
});

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

const insightValueLabel = (t, label) => {
  const match = String(label || '').match(/^(\S+)\s+(.+)$/);
  if (!match) return label;
  return t.server(`server.insight.valueLabel.${slugKey(match[2])}`, label, { value: match[1] });
};

const insightSummaryLabel = (percentile) => {
  const score = Math.max(0, Math.min(100, Number(percentile) || 0));
  if (score >= 99) return t('skillProxies.score.best');
  if (score >= 80) return t('skillProxies.score.top', { value: Math.max(1, Math.round(100 - score)) });
  return t('skillProxies.score.betterThan', { value: Math.round(score) });
};

const playerInsightDestinationTab = (insightType) => {
  switch (String(insightType || '').trim()) {
    case PLAYER_INSIGHT_TYPES.apm:
      return 'apm-histogram';
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

const MAIN_GAMES_PAGE_SIZE = 50;
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

// Target players per visual row in the games-list Players cell. Teams are
// balanced across ceil(players / this) rows and never split, so the row break
// always falls between teams.
const PLAYERS_PER_LIST_ROW = 4;

// Package-manager installs can't self-update, so we surface the exact upgrade
// command with a one-click copy button plus a link to the release notes.
const ManagedUpdateHint = ({ latestVersion, command, releaseUrl, className }) => {
  const t = useT();
  const [copied, setCopied] = useState(false);
  const copyTimerRef = useRef(null);
  useEffect(() => () => {
    if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
  }, []);
  const handleCopy = () => {
    if (!navigator.clipboard || !navigator.clipboard.writeText) return;
    navigator.clipboard.writeText(command).then(() => {
      setCopied(true);
      if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
      copyTimerRef.current = window.setTimeout(() => setCopied(false), 2000);
    }).catch(() => {});
  };
  return (
    <span className={`managed-update-hint${className ? ` ${className}` : ''}`}>
      <span className="managed-update-hint-label">🆕 {latestVersion}</span>
      <code className="managed-update-hint-cmd">{command}</code>
      <button
        type="button"
        className="managed-update-hint-copy"
        data-tip={copied ? t('update.copied') : t('update.copyCommand')}
        onClick={handleCopy}
      >
        {copied ? t('update.copiedButton') : t('update.copyButton')}
      </button>
      <a
        href={releaseUrl}
        target="_blank"
        rel="noopener noreferrer"
        className="managed-update-hint-changelog"
        data-tip={t('update.changelogTip')}
      >
        {t('update.changelog')}
      </a>
    </span>
  );
};

function App() {
  const t = useT();
  const initialMainRoute = useMemo(
    () => parseMainRouteSearch(typeof window !== 'undefined' ? window.location.search : ''),
    [],
  );
  const markerRegistryState = useMarkerRegistry();
  const markerRegistry = markerRegistryState.markers;
  const markerDefinitions = markerRegistryState;
  // Game-event features (drop subtypes, mind_control, rushes, proxies) keyed
  // by their event_type — used by aggregate pill renderers to surface
  // multi-icon pills for subtypes that aren't in the marker registry.
  const mainPlayerGameEventFeaturesByKey = useMemo(() => {
    const out = {};
    (markerRegistryState?.game_event_features || []).forEach((f) => { if (f?.key) out[f.key] = f; });
    return out;
  }, [markerRegistryState?.game_event_features]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);
  const [showGlobalReplayFilter, setShowGlobalReplayFilter] = useState(false);
  const [replayCount, setReplayCount] = useState(null);
  const [currentVersion, setCurrentVersion] = useState('');
  const [currentCommit, setCurrentCommit] = useState('');
  const [latestVersion, setLatestVersion] = useState('');
  const [latestVersionUrl, setLatestVersionUrl] = useState('');
  const [updateStatus, setUpdateStatus] = useState(null);
  const [updateApplying, setUpdateApplying] = useState(false);
  const [updateApplied, setUpdateApplied] = useState(false);
  const [updateError, setUpdateError] = useState('');
  const [quietUpdateDismissed, setQuietUpdateDismissed] = useState(false);
  const [loudUpdateDismissed, setLoudUpdateDismissed] = useState(false);
  const [stopped, setStopped] = useState(false);
  const [bnetStatus, setBnetStatus] = useState(null);
  const emptyDbAutoOpenRef = useRef(false);
  const [globalReplayFilterConfig, setGlobalReplayFilterConfig] = useState(null);
  const [globalReplayFilterSaving, setGlobalReplayFilterSaving] = useState(false);
  const [globalReplayFilterError, setGlobalReplayFilterError] = useState('');
  const [libraryMessage, setLibraryMessage] = useState('');
  const [libraryStatus, setLibraryStatus] = useState('idle');
  const [libraryProgress, setLibraryProgress] = useState(null);
  const [replayDirInput, setReplayDirInput] = useState('');
  const [savedReplayDir, setSavedReplayDir] = useState('');
  const [librarySettingsLoading, setLibrarySettingsLoading] = useState(false);
  const [librarySettingsSaving, setLibrarySettingsSaving] = useState(false);
  const [isSampleSet, setIsSampleSet] = useState(false);
  const [detectedReplayDir, setDetectedReplayDir] = useState('');
  const [sampleSetLoading, setSampleSetLoading] = useState(false);
  const [sampleNotice, setSampleNotice] = useState('');
  // The most recent `corpus` block seen on a list/insight response. Preferred
  // over the websocket status for "is the corpus complete?" because it is the
  // truth about the data actually on screen; the websocket is the fallback
  // until a response has arrived.
  const [responseCorpus, setResponseCorpus] = useState(null);
  const librarySocketRef = useRef(null);
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
  const mainGamesTableRef = useRef(null);
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
  const [mainEventsLayoutEl, setMainEventsLayoutEl] = useState(null);
  const [mainEventsMapColPx, setMainEventsMapColPx] = useState(0);
  const openMainPlayerRef = useRef(null);
  // "Latest-ref" pattern: the mount-once library websocket handler needs to
  // read the *current* games-list filter/page state and call the *current*
  // refresh functions. Its dependency array intentionally excludes these to
  // avoid reconnect churn, so we mirror them into refs re-assigned on every
  // render.
  const refreshAfterLibraryLoadRef = useRef(null);
  // Progress and corpus events only touch the lists that new replays affect
  // (games, players, session); never the active game/player view or the
  // histograms, which would blink on every event during the initial load.
  const refreshGamesListRef = useRef(null);
  const refreshPlayersListRef = useRef(null);
  const loadGamingSessionRef = useRef(null);
  const activeViewRef = useRef(null);
  const mainGamesFiltersRef = useRef(null);
  const mainGamesPageRef = useRef(null);
  const [mainPlayer, setMainPlayer] = useState(null);
  const [mainGameHotkeys, setMainGameHotkeys] = useState(null);
  const [mainGameHotkeysLoading, setMainGameHotkeysLoading] = useState(false);
  const [mainGameHotkeysError, setMainGameHotkeysError] = useState('');
  const [mainPlayerHotkeySig, setMainPlayerHotkeySig] = useState(null);
  const [mainPlayerHotkeySigLoading, setMainPlayerHotkeySigLoading] = useState(false);
  const [mainPlayerHotkeySigError, setMainPlayerHotkeySigError] = useState('');
  const [mainPlayerLastGames, setMainPlayerLastGames] = useState(null);
  const [mainPlayerLastGamesLoading, setMainPlayerLastGamesLoading] = useState(false);
  const [mainPlayerLastGamesError, setMainPlayerLastGamesError] = useState('');
  const [mainPlayerKnownAliases, setMainPlayerKnownAliases] = useState({});
  const [mainPlayerChatSummary, setMainPlayerChatSummary] = useState(null);
  const [mainPlayerChatSummaryLoading, setMainPlayerChatSummaryLoading] = useState(false);
  const [mainPlayerChatSummaryError, setMainPlayerChatSummaryError] = useState('');
  // Per-outlier-category state. Each category fires its own request so
  // pills stream into the UI as each finishes (instead of all-or-nothing
  // on a 60-90s monolithic /summary/special). Keyed by lowercase
  // category label ("order", "build", ...).
  const [mainPlayers, setMainPlayers] = useState([]);
  const [mainPlayersFeatured, setMainPlayersFeatured] = useState([]);
  const [mainPlayersFeaturedExpanded, setMainPlayersFeaturedExpanded] = useState(false);
  const [showFeaturedPros, setShowFeaturedPros] = useState(() => getStoredShowFeaturedPros());
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
  const mainPlayersApmMinGames = 5;
  const [mainPlayersCadenceHistogram, setMainPlayersCadenceHistogram] = useState(null);
  const [mainPlayersCadenceHistogramLoading, setMainPlayersCadenceHistogramLoading] = useState(false);
  const [mainPlayersCadenceHistogramError, setMainPlayersCadenceHistogramError] = useState('');
  const mainPlayersCadenceMinGames = 4;
  const [mainPlayersViewportHistogram, setMainPlayersViewportHistogram] = useState(null);
  const [mainPlayersViewportHistogramLoading, setMainPlayersViewportHistogramLoading] = useState(false);
  const [mainPlayersViewportHistogramError, setMainPlayersViewportHistogramError] = useState('');
  const mainPlayersViewportMinGames = 4;
  const [mainPlayerApmInsight, setMainPlayerApmInsight] = useState(null);
  const [mainPlayerApmInsightLoading, setMainPlayerApmInsightLoading] = useState(false);
  const [mainPlayerApmInsightError, setMainPlayerApmInsightError] = useState('');
  const [mainPlayerCadenceInsight, setMainPlayerCadenceInsight] = useState(null);
  const [mainPlayerCadenceInsightLoading, setMainPlayerCadenceInsightLoading] = useState(false);
  const [mainPlayerCadenceInsightError, setMainPlayerCadenceInsightError] = useState('');
  const [mainPlayerViewportInsight, setMainPlayerViewportInsight] = useState(null);
  const [mainPlayerViewportInsightLoading, setMainPlayerViewportInsightLoading] = useState(false);
  const [mainPlayerViewportInsightError, setMainPlayerViewportInsightError] = useState('');
  // Used purely as a re-render trigger after the screp engine color map loads;
  // the actual map lives at module scope (see scPlayerColorMap above) so the
  // module-level helpers (playerColorToCss, mapLabelStyle) can consume it.
  const [, setScColorMapLoaded] = useState(false);
  const [mainSummaryFilters, setMainSummaryFilters] = useState(DEFAULT_SUMMARY_FILTERS);
  const [productionView, setProductionView] = useState('all');
  const [productionSubFilter, setProductionSubFilter] = useState('all');
  const [productionNameFilter, setProductionNameFilter] = useState('');
  const [mainTimingCategory, setMainTimingCategory] = useState('expansion_gas');
  const [mainResearchSubcategories, setMainResearchSubcategories] = useState(DEFAULT_RESEARCH_SUBCATEGORIES);
  const toggleResearchSubcategory = (id) => {
    setMainResearchSubcategories((prev) => (
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id]
    ));
  };
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
      if (data?.corpus) setResponseCorpus(data.corpus);
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
      setMainPlayersFeatured(Array.isArray(data?.featured_players) ? data.featured_players : []);
      setMainPlayersTotal(Number(data?.total) || 0);
      if (data?.filter_options) {
        setMainPlayersFilterOptions(data.filter_options);
      }
      if (data?.corpus) setResponseCorpus(data.corpus);
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
      if (data?.corpus) setResponseCorpus(data.corpus);
    } catch (err) {
      setMainPlayersApmHistogramError(err.message || t('errors.loadPlayersHistogram'));
      setMainPlayersApmHistogram(null);
    } finally {
      setMainPlayersApmHistogramLoading(false);
    }
  };

  const loadMainPlayersCadenceHistogram = async () => {
    try {
      setMainPlayersCadenceHistogramLoading(true);
      setMainPlayersCadenceHistogramError('');
      const data = await api.getPlayersUnitProductionCadence({ filter: 'strict', minGames: 4, limit: 0 });
      setMainPlayersCadenceHistogram(data);
      if (data?.corpus) setResponseCorpus(data.corpus);
    } catch (err) {
      setMainPlayersCadenceHistogramError(err.message || t('errors.loadPlayersCadence'));
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
      if (data?.corpus) setResponseCorpus(data.corpus);
    } catch (err) {
      setMainPlayersViewportHistogramError(err.message || t('errors.loadPlayersViewport'));
      setMainPlayersViewportHistogram(null);
    } finally {
      setMainPlayersViewportHistogramLoading(false);
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
      setMainGameHotkeys(null);
      setMainGameHotkeysError('');
      setMainGameHotkeysLoading(false);
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
      setMainSummaryFilters(DEFAULT_SUMMARY_FILTERS);
      setProductionView('all');
      setProductionSubFilter('all');
      setProductionNameFilter('');
      setMainTimingCategory('expansion_gas');
      setMainResearchSubcategories(DEFAULT_RESEARCH_SUBCATEGORIES);
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
      setMainGameSeeNotice(t('game.stage.copied'));
      mainGameSeeNoticeTimerRef.current = window.setTimeout(() => {
        setMainGameSeeNotice('');
        mainGameSeeNoticeTimerRef.current = null;
      }, 5000);
    } catch (err) {
      setMainGameSeeNotice(err.message || t('game.stage.failed'));
      setMainGameSeeNoticeError(true);
    } finally {
      setMainGameSeeLoading(false);
    }
  };

  const loadMainPlayerLastGames = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerLastGamesLoading(true);
      setMainPlayerLastGamesError('');
      const data = await api.getPlayerLastGames(normalizedPlayerKey);
      setMainPlayerLastGames(data?.last_games || []);
    } catch (err) {
      setMainPlayerLastGamesError(err.message || t('errors.loadLastGames'));
      setMainPlayerLastGames([]);
    } finally {
      setMainPlayerLastGamesLoading(false);
    }
  };

  const loadMainGameHotkeys = async (replayId) => {
    if (!replayId) return;
    try {
      setMainGameHotkeysLoading(true);
      setMainGameHotkeysError('');
      const data = await api.getGameHotkeys(replayId);
      setMainGameHotkeys(data || null);
    } catch (err) {
      setMainGameHotkeysError(err.message || t('errors.loadHotkeyStreams'));
      setMainGameHotkeys(null);
    } finally {
      setMainGameHotkeysLoading(false);
    }
  };

  const loadMainPlayerHotkeySignature = async (playerKey) => {
    const normalizedPlayerKey = String(playerKey || '').trim().toLowerCase();
    if (!normalizedPlayerKey) return;
    try {
      setMainPlayerHotkeySigLoading(true);
      setMainPlayerHotkeySigError('');
      const data = await api.getPlayerHotkeySignature(normalizedPlayerKey);
      setMainPlayerHotkeySig(data || null);
    } catch (err) {
      setMainPlayerHotkeySigError(err.message || t('errors.loadHotkeySignature'));
      setMainPlayerHotkeySig(null);
    } finally {
      setMainPlayerHotkeySigLoading(false);
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
      setMainPlayerChatSummaryError(err.message || t('errors.loadChatSummary'));
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
      setMainPlayerApmInsightError(err.message || t('errors.loadApmInsight'));
      setMainPlayerApmInsight(null);
    } finally {
      setMainPlayerApmInsightLoading(false);
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
      setMainPlayerCadenceInsightError(err.message || t('errors.loadCadenceInsight'));
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
      setMainPlayerViewportInsightError(err.message || t('errors.loadViewportInsight'));
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
    setMainPlayerLastGames(null);
    setMainPlayerLastGamesError('');
    setMainPlayerLastGamesLoading(false);
    setMainPlayerChatSummary(null);
    setMainPlayerChatSummaryError('');
    setMainPlayerChatSummaryLoading(false);
    setMainPlayerHotkeySig(null);
    setMainPlayerHotkeySigError('');
    setMainPlayerHotkeySigLoading(false);
    setMainPlayerApmInsight(null);
    setMainPlayerApmInsightError('');
    setMainPlayerApmInsightLoading(false);
    setMainPlayerCadenceInsight(null);
    setMainPlayerCadenceInsightError('');
    setMainPlayerCadenceInsightLoading(false);
    setMainPlayerViewportInsight(null);
    setMainPlayerViewportInsightError('');
    setMainPlayerViewportInsightLoading(false);
    setSelectedPlayerKey(normalizedPlayerKey);
    const wantTab = options.initialPlayerTab;
    const nextTab = wantTab && MAIN_PLAYER_TABS.includes(String(wantTab).trim().toLowerCase())
      ? String(wantTab).trim().toLowerCase()
      : 'summary';
    setMainPlayerTab(nextTab);
    const wantSubtab = String(options.initialPlayerSubtab || '').trim().toLowerCase();
    if (nextTab === 'skill-proxies') {
      setMainPlayerSubtab(MAIN_PLAYER_SKILL_PROXY_SUBTABS.includes(wantSubtab) ? wantSubtab : 'summary');
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

  const loadLibrarySettings = async () => {
    try {
      setLibrarySettingsLoading(true);
      const data = await api.getLibrarySettings();
      const nextReplayDir = String(data?.replay_dir || '');
      setReplayDirInput(nextReplayDir);
      setSavedReplayDir(nextReplayDir);
      setIsSampleSet(Boolean(data?.is_sample_set));
      setDetectedReplayDir(String(data?.detected_replay_dir || ''));
      if (data?.sample_auto_loaded) {
        // The backend fell back to the example replays because it couldn't find
        // the user's replay folder. Suppress the empty-library auto-open of the
        // modal and show a dismissable notice instead.
        emptyDbAutoOpenRef.current = true;
        setSampleNotice(t('library.message.sampleNotice'));
      }
      return nextReplayDir;
    } catch (err) {
      setLibraryMessage(err.message || t('library.message.loadSettingsFailed'));
      return '';
    } finally {
      setLibrarySettingsLoading(false);
    }
  };

  const persistReplayDir = async (replayDir = replayDirInput) => {
    const trimmed = String(replayDir || '').trim();
    if (!trimmed) {
      throw new Error(t('library.message.folderRequired'));
    }

    setLibrarySettingsSaving(true);
    try {
      const data = await api.updateLibrarySettings({ replay_dir: trimmed });
      const nextReplayDir = String(data?.replay_dir || trimmed);
      setReplayDirInput(nextReplayDir);
      setSavedReplayDir(nextReplayDir);
      setIsSampleSet(Boolean(data?.is_sample_set));
      setDetectedReplayDir(String(data?.detected_replay_dir || ''));
      return nextReplayDir;
    } finally {
      setLibrarySettingsSaving(false);
    }
  };

  const handleLoadSampleSet = async () => {
    const confirmed = window.confirm(t('library.message.confirmLoadSample'));
    if (!confirmed) {
      return;
    }
    setLibraryMessage('');
    setSampleSetLoading(true);
    try {
      await api.loadSampleSet();
      await loadLibrarySettings();
      setLibraryMessage(t('library.switchedToExamples'));
    } catch (err) {
      setLibraryMessage(err.message || t('library.message.loadSampleFailed'));
    } finally {
      setSampleSetLoading(false);
    }
  };

  const handleUseDetectedFolder = async () => {
    if (!detectedReplayDir) {
      return;
    }
    setLibraryMessage('');
    try {
      setReplayDirInput(detectedReplayDir);
      await persistReplayDir(detectedReplayDir);
      await loadLibrarySettings();
      setLibraryMessage(t('library.folderSaved'));
    } catch (err) {
      setLibraryMessage(err.message || t('library.message.switchFolderFailed'));
    }
  };

  openMainGameRef.current = openMainGame;
  openMainPlayerRef.current = openMainPlayer;

  useEffect(() => {
    setLoading(false);
    loadGlobalReplayFilterConfig().catch((err) => {
      console.error('Failed to load global replay filter config:', err);
    });
    loadScrepColors();
    // Resolve library settings (incl. the one-shot sample-auto-loaded signal)
    // before the health check so it can decide whether to auto-open the modal.
    loadLibrarySettings().finally(() => checkHealthStatus());
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
        // The binary performs the GitHub check and verification; the UI only
        // renders the result and offers a one-click, verified self-update.
        const status = await api.getUpdateStatus();
        if (cancelled || !status) return;
        setUpdateStatus(status);
        if (status.latest_version) setLatestVersion(String(status.latest_version));
        if (status.latest_release_url) setLatestVersionUrl(String(status.latest_release_url));
      } catch (_err) {
        // Silently ignore — offline, rate-limited, etc. Notice just stays hidden.
      }
    })();
    return () => { cancelled = true; };
  }, [currentVersion]);

  // Poll Battle.net bridge connection status. The backend caches the result
  // from its own 30-second probe loop, so this read is cheap.
  const bnetReconnectingRef = useRef(false);
  useEffect(() => {
    if (stopped) return undefined;
    let cancelled = false;
    const poll = async () => {
      try {
        const status = await api.getBnetStatus();
        if (cancelled) return;
        if (bnetReconnectingRef.current) {
          if (status.disabled || status.state === 'not_running') return;
          bnetReconnectingRef.current = false;
        }
        setBnetStatus(status);
      } catch (_err) {
        // Silently ignore — server may be shutting down.
      }
    };
    poll();
    const id = setInterval(poll, 10000);
    return () => { cancelled = true; clearInterval(id); };
  }, [stopped]);

  const bnetState = bnetStatus?.state || 'not_running';
  const bnetDisabled = Boolean(bnetStatus?.disabled);
  const bnetRequestsToday = bnetStatus?.requests_today ?? 0;
  const bnetDailyCap = bnetStatus?.daily_cap ?? 0;
  const bnetCooldownUntil = bnetStatus?.cooldown_until ? new Date(bnetStatus.cooldown_until) : null;
  const bnetCoolingDown = Boolean(bnetCooldownUntil) && bnetCooldownUntil > new Date();

  const [featureFlags, setFeatureFlags] = useState({});
  const [featureFlagsSaving, setFeatureFlagsSaving] = useState(false);
  const [featureFlagsMessage, setFeatureFlagsMessage] = useState('');
  const [featureFlagsMessageIsError, setFeatureFlagsMessageIsError] = useState(false);
  const [gamingSession, setGamingSession] = useState(null);
  const [gamingSessionLoading, setGamingSessionLoading] = useState(false);
  const [gamingSessionError, setGamingSessionError] = useState('');

  const gamingSessionEnabled = Boolean(featureFlags.gaming_session);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await api.getFeatureFlags();
        if (!cancelled) setFeatureFlags(res?.feature_flags || {});
      } catch {
        // Flags default off; a failed load simply leaves the previews hidden.
      }
    })();
    return () => { cancelled = true; };
  }, []);

  const handleFeatureFlagToggle = async (key, enabled) => {
    try {
      setFeatureFlagsSaving(true);
      setFeatureFlagsMessage('');
      setFeatureFlagsMessageIsError(false);
      const res = await api.setFeatureFlag(key, enabled);
      setFeatureFlags(res?.feature_flags || {});
    } catch (err) {
      setFeatureFlagsMessage(err.message || t('settings.errors.saveFlag'));
      setFeatureFlagsMessageIsError(true);
    } finally {
      setFeatureFlagsSaving(false);
    }
  };

  // The session is refetched whenever the flag turns on and whenever the user
  // opens the view, so a game finishing mid-visit shows up on the next look.
  const loadGamingSession = useCallback(async () => {
    if (!gamingSessionEnabled) return;
    try {
      setGamingSessionLoading(true);
      setGamingSessionError('');
      const res = await api.getGamingSession();
      setGamingSession(res);
    } catch (err) {
      setGamingSessionError(err.message || t('session.message.loadFailed'));
    } finally {
      setGamingSessionLoading(false);
    }
  }, [gamingSessionEnabled]);

  useEffect(() => {
    if (!gamingSessionEnabled) {
      setGamingSession(null);
      return;
    }
    void loadGamingSession();
  }, [gamingSessionEnabled, loadGamingSession]);

  // Every player currently on screen that has no flag yet. The poll below asks
  // only about these, and only while the bridge could still produce an answer.
  const missingCountryCodeKeys = useMemo(() => {
    const keys = new Set();
    const consider = (player) => {
      const key = String(player?.player_key || '').trim().toLowerCase();
      if (!key || player?.country_code) return;
      keys.add(key);
    };
    (mainPlayers || []).forEach(consider);
    (mainGames || []).forEach((game) => (game?.players || []).forEach(consider));
    (mainGame?.players || []).forEach(consider);
    if (mainPlayer) consider(mainPlayer);
    return [...keys].sort();
  }, [mainPlayers, mainGames, mainGame, mainPlayer]);

  const countryCodeOverrides = useCountryFlagBackfill(
    missingCountryCodeKeys,
    !bnetDisabled && bnetState === 'connected',
  );
  const bnetTipSuffix = bnetDisabled || bnetDailyCap <= 0 ? ''
    : bnetCoolingDown
      ? t('bnet.tip.requestsPaused', { used: bnetRequestsToday, cap: bnetDailyCap, time: bnetCooldownUntil.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) })
      : t('bnet.tip.requests', { used: bnetRequestsToday, cap: bnetDailyCap });

  const handleBnetToggle = useCallback(async () => {
    const newDisabled = !bnetDisabled;
    if (newDisabled) {
      setBnetStatus({ state: 'not_running', addr: '', disabled: true });
    } else {
      bnetReconnectingRef.current = true;
      setBnetStatus({ state: 'reconnecting', addr: '', disabled: false });
    }
    try {
      await api.setBnetDisabled(newDisabled);
    } catch (err) {
      console.error('Failed to toggle Battle.net bridge:', err);
      bnetReconnectingRef.current = false;
    }
  }, [bnetDisabled]);

  const updateTier = updateStatus?.tier || 'none';
  const updateAvailable = Boolean(updateStatus?.update_available);
  const selfUpdateSupported = Boolean(updateStatus?.self_update_supported);
  const updateLatest = String(updateStatus?.latest_version || latestVersion || '');
  const updateReleaseUrl = String(updateStatus?.latest_release_url || latestVersionUrl || 'https://github.com/marianogappa/screpdb/releases/latest');
  // When the app can't swap its own binary, surface the exact upgrade command to
  // run: the package manager for scoop/brew installs, or a re-run of the install
  // script when the install dir is read-only on macOS/Linux (empty otherwise).
  const updateManagerCommand = (() => {
    const reason = updateStatus?.reason || '';
    const manager = updateStatus?.package_manager || '';
    const os = updateStatus?.os || '';
    if (reason === 'managed') {
      if (manager === 'scoop') return 'scoop update screpdb';
      if (manager === 'homebrew') return 'brew upgrade screpdb';
      return '';
    }
    if (reason === 'not-writable' && os !== 'windows') {
      return 'curl -fsSL https://raw.githubusercontent.com/marianogappa/screpdb/main/install.sh | sh';
    }
    return '';
  })();
  const updateUnsupportedTip = (() => {
    const reason = updateStatus?.reason || '';
    const manager = updateStatus?.package_manager || '';
    if (updateManagerCommand) return t('update.runInTerminal', { command: updateManagerCommand });
    if (reason === 'managed') return t('update.viaPackageManager', { manager });
    if (reason === 'not-writable') return t('update.notWritable');
    if (reason === 'unsupported-platform') return t('update.unsupportedPlatform');
    return t('update.download');
  })();

  const handleApplyUpdate = async () => {
    if (updateApplying || updateApplied) return;
    setUpdateError('');
    setUpdateApplying(true);
    try {
      const res = await api.applyUpdate();
      if (res?.success) {
        setUpdateApplied(true);
      } else {
        throw new Error(res?.error || t('update.failed'));
      }
    } catch (err) {
      setUpdateError(String(err?.message || err));
    } finally {
      setUpdateApplying(false);
    }
  };

  const handleQuit = () => {
    if (stopped) return;
    if (!window.confirm(t('quit.confirm'))) return;
    // The server dies ~500ms after acknowledging, so the request may resolve or
    // error as the socket drops — either way it's on its way down. Show the
    // stopped screen regardless.
    api.quit().catch(() => {}).finally(() => setStopped(true));
  };

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
    if (!shouldLoadPlayerSkillProxyInsights({ activeView, selectedPlayerKey, mainPlayerTab })) return;
    if (isFeaturedPlayerKey(selectedPlayerKey)) return;
    if (!mainPlayerApmInsight && !mainPlayerApmInsightLoading && !mainPlayerApmInsightError) {
      loadMainPlayerApmInsight(selectedPlayerKey);
    }
    if (!mainPlayerCadenceInsight && !mainPlayerCadenceInsightLoading && !mainPlayerCadenceInsightError) {
      loadMainPlayerCadenceInsight(selectedPlayerKey);
    }
    if (!mainPlayerViewportInsight && !mainPlayerViewportInsightLoading && !mainPlayerViewportInsightError) {
      loadMainPlayerViewportInsight(selectedPlayerKey);
    }
  }, [
    activeView, selectedPlayerKey, mainPlayerTab,
    mainPlayerApmInsight, mainPlayerApmInsightLoading, mainPlayerApmInsightError,
    mainPlayerCadenceInsight, mainPlayerCadenceInsightLoading, mainPlayerCadenceInsightError,
    mainPlayerViewportInsight, mainPlayerViewportInsightLoading, mainPlayerViewportInsightError,
  ]);

  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'summary') return;
    if (isFeaturedPlayerKey(selectedPlayerKey)) return;
    if (!mainPlayerLastGames && !mainPlayerLastGamesLoading && !mainPlayerLastGamesError) {
      loadMainPlayerLastGames(selectedPlayerKey);
    }
  }, [activeView, selectedPlayerKey, mainPlayerTab, mainPlayerLastGames, mainPlayerLastGamesLoading, mainPlayerLastGamesError]);

  useEffect(() => {
    if (activeView !== 'game' || !selectedReplayId) return;
    if (mainGameTab !== 'hotkeys') return;
    if (!mainGameHotkeys && !mainGameHotkeysLoading && !mainGameHotkeysError) {
      loadMainGameHotkeys(selectedReplayId);
    }
  }, [activeView, selectedReplayId, mainGameTab, mainGameHotkeys, mainGameHotkeysLoading, mainGameHotkeysError]);

  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'hotkeys') return;
    if (!mainPlayerHotkeySig && !mainPlayerHotkeySigLoading && !mainPlayerHotkeySigError) {
      loadMainPlayerHotkeySignature(selectedPlayerKey);
    }
  }, [activeView, selectedPlayerKey, mainPlayerTab, mainPlayerHotkeySig, mainPlayerHotkeySigLoading, mainPlayerHotkeySigError]);

  useEffect(() => {
    if (activeView !== 'player' || !selectedPlayerKey) return;
    if (mainPlayerTab !== 'chat-summary') return;
    if (isFeaturedPlayerKey(selectedPlayerKey)) return;
    if (!mainPlayerChatSummary && !mainPlayerChatSummaryLoading && !mainPlayerChatSummaryError) {
      loadMainPlayerChatSummary(selectedPlayerKey);
    }
  }, [activeView, selectedPlayerKey, mainPlayerTab, mainPlayerChatSummary, mainPlayerChatSummaryLoading, mainPlayerChatSummaryError]);

  // Resolve which Battle.net alter-ego toons have their own local player
  // entry, so "Also plays as" can link to them.
  useEffect(() => {
    const toons = mainPlayer?.bnet_profile?.toons || [];
    setMainPlayerKnownAliases({});
    if (toons.length === 0) return undefined;
    let cancelled = false;
    (async () => {
      const found = {};
      await Promise.all(toons.map(async (t) => {
        const key = String(t?.toon || '').trim().toLowerCase();
        if (!key) return;
        try {
          await api.getPlayer(key);
          found[key] = true;
        } catch {
          // Not a local player; render the alias as plain text.
        }
      }));
      if (!cancelled) setMainPlayerKnownAliases(found);
    })();
    return () => { cancelled = true; };
  }, [mainPlayer?.bnet_profile]);

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
    let unmounted = false;
    let reconnectTimer = null;
    let reconnectAttempt = 0;
    let socket = null;
    let lastStatus = '';
    let connectedBefore = false;

    // A tab left in the background gets its timers throttled and its socket
    // closed, so it can miss every change while it sleeps. Whatever arrives
    // after a gap is not enough: the page reloads what it is showing.
    const refreshAfterGap = () => {
      void refreshGamesListRef.current?.();
      if (activeViewRef.current === 'players') void refreshPlayersListRef.current?.();
      void loadGamingSessionRef.current?.();
      void checkHealthStatus();
    };

    const applyStatus = (status) => {
      const next = String(status || 'idle');
      setLibraryStatus(next);
      // A read (initial or after a folder change) just finished: everything on
      // screen was computed from a partial corpus, so refresh broadly once. A
      // first snapshot that is already `watching` is skipped because the mount
      // effects are loading those views right now anyway.
      if (next === 'watching' && lastStatus && lastStatus !== 'watching') {
        void refreshAfterLibraryLoadRef.current?.();
        void loadGamingSessionRef.current?.();
      }
      lastStatus = next;
    };

    const connect = () => {
      if (unmounted) return;
      if (reconnectTimer) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      socket = api.createLibraryEventsSocket();
      librarySocketRef.current = socket;

      socket.onopen = () => {
        reconnectAttempt = 0;
        if (connectedBefore) refreshAfterGap();
        connectedBefore = true;
      };

      socket.onmessage = (event) => {
        try {
          const message = JSON.parse(event.data);
          if (message.type === 'snapshot') {
            if (message.progress) setLibraryProgress(message.progress);
            if (message.error) setLibraryMessage(message.error);
            applyStatus(message.status);
            return;
          }

          if (message.type === 'progress') {
            if (message.progress) setLibraryProgress(message.progress);
            if (message.status) setLibraryStatus(String(message.status));
            return;
          }

          if (message.type === 'status') {
            if (message.error) setLibraryMessage(message.error);
            applyStatus(message.status);
            return;
          }

          if (message.type === 'corpus') {
            void refreshGamesListRef.current?.();
            if (activeViewRef.current === 'players') void refreshPlayersListRef.current?.();
            void loadGamingSessionRef.current?.();
          }
        } catch (err) {
          console.error('Failed to parse library events message:', err);
        }
      };

      socket.onerror = () => {
      };

      socket.onclose = () => {
        if (librarySocketRef.current === socket) {
          librarySocketRef.current = null;
        }
        if (unmounted) return;
        // Reconnect with backoff: 2s, 5s, 10s, then 30s thereafter.
        const delays = [2000, 5000, 10000, 30000];
        const delay = delays[Math.min(reconnectAttempt, delays.length - 1)];
        reconnectAttempt += 1;
        reconnectTimer = window.setTimeout(connect, delay);
      };
    };

    connect();

    // Waking up must not wait out a backoff that grew while the tab slept.
    const onWake = () => {
      if (unmounted || document.visibilityState !== 'visible') return;
      const state = socket?.readyState;
      if (state === WebSocket.OPEN) {
        refreshAfterGap();
        return;
      }
      if (state !== WebSocket.CONNECTING) {
        reconnectAttempt = 0;
        connect();
      }
    };
    document.addEventListener('visibilitychange', onWake);
    window.addEventListener('online', onWake);

    return () => {
      unmounted = true;
      document.removeEventListener('visibilitychange', onWake);
      window.removeEventListener('online', onWake);
      if (reconnectTimer) {
        window.clearTimeout(reconnectTimer);
        reconnectTimer = null;
      }
      if (librarySocketRef.current === socket) {
        librarySocketRef.current = null;
      }
      if (socket) {
        socket.close();
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount-once: WS lives for the whole app session, independent of modal visibility.
  }, []);

  useEffect(() => {
    if (!showGlobalReplayFilter) return undefined;
    setLibraryMessage('');
    void loadLibrarySettings();
    return undefined;
    // eslint-disable-next-line react-hooks/exhaustive-deps -- only refresh settings + clear message when the modal opens.
  }, [showGlobalReplayFilter]);

  useEffect(() => () => {
    if (mainGameSeeNoticeTimerRef.current) {
      window.clearTimeout(mainGameSeeNoticeTimerRef.current);
    }
  }, []);

  const checkHealthStatus = async () => {
    try {
      const data = await api.getHealth();
      const totalReplays = Number(data?.total_replays || 0);
      setReplayCount(totalReplays);
      if (data?.version) {
        setCurrentVersion(String(data.version));
      }
      if (data?.commit) {
        setCurrentCommit(String(data.commit));
      }
      const library = data?.library || null;
      if (library?.status) {
        setLibraryStatus(String(library.status));
        setLibraryProgress((current) => current || {
          generation: library.generation,
          version: library.version,
          phase: library.phase,
          total: library.total,
          loaded: library.loaded,
          failed: library.failed,
          skipped: library.skipped,
          replay_dir: library.replay_dir,
        });
      }
      if (totalReplays === 0 && !isLibraryLoading(library?.status) && !emptyDbAutoOpenRef.current) {
        emptyDbAutoOpenRef.current = true;
        setShowGlobalReplayFilter(true);
      }
      return data;
    } catch (err) {
      console.error('Failed to check health status:', err);
      return null;
    }
  };

  const handleSaveReplayDir = async () => {
    setLibraryMessage('');
    try {
      await persistReplayDir(replayDirInput);
      await loadLibrarySettings();
      setLibraryMessage(t('library.folderSaved'));
    } catch (err) {
      setLibraryMessage(err.message || t('library.message.saveFolderFailed'));
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
      checkHealthStatus(),
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
    if (mainPlayersCadenceHistogram) {
      loadMainPlayersCadenceHistogram();
    }
  };

  // Lightweight list-only refreshes used by the library events path. No
  // active-view re-fetch, no histogram reloads: just the lists that newly
  // loaded replays affect.
  const refreshGameListOnly = async () => {
    try {
      await loadMainGames({ page: mainGamesPage, filters: mainGamesFilters });
    } catch (err) {
      console.error('Failed to refresh game list after library update:', err);
    }
  };
  const refreshPlayersListOnly = async () => {
    try {
      await loadMainPlayers({
        page: mainPlayersPage,
        filters: mainPlayersFilters,
        sortBy: mainPlayersSortBy,
        sortDir: mainPlayersSortDir,
      });
    } catch (err) {
      console.error('Failed to refresh players list after library update:', err);
    }
  };

  // Keep the latest-ref pattern wiring up to date. Assigning during render
  // (rather than in an effect) is the standard React pattern and is safe
  // because we only *read* these refs from event/timer callbacks, never
  // during the render itself.
  refreshAfterLibraryLoadRef.current = refreshDataAfterGlobalReplayFilterSave;
  refreshGamesListRef.current = refreshGameListOnly;
  refreshPlayersListRef.current = refreshPlayersListOnly;
  loadGamingSessionRef.current = loadGamingSession;
  activeViewRef.current = activeView;
  mainGamesFiltersRef.current = mainGamesFilters;
  mainGamesPageRef.current = mainGamesPage;

  // Corpus completeness for partial-load states. The `corpus` block on the
  // most recent response wins when it describes the same generation the
  // websocket is reporting; otherwise (a folder change bumped the generation,
  // or no response has arrived yet) the websocket status decides.
  const libraryLoading = (() => {
    const wsGeneration = Number(libraryProgress?.generation);
    const responseGeneration = Number(responseCorpus?.generation);
    const sameGeneration = !Number.isFinite(wsGeneration) || !Number.isFinite(responseGeneration) || responseGeneration >= wsGeneration;
    if (responseCorpus && sameGeneration && typeof responseCorpus.complete === 'boolean') {
      return !responseCorpus.complete;
    }
    return isLibraryLoading(libraryStatus);
  })();
  const libraryLoadingCopy = libraryLoading ? stillLoadingCopyWith(t) : '';

  const handleSaveGlobalReplayFilter = async (nextConfig) => {
    try {
      setGlobalReplayFilterSaving(true);
      setGlobalReplayFilterError('');
      const saved = await api.updateGlobalReplayFilter(nextConfig);
      setGlobalReplayFilterConfig(saved);
      await refreshDataAfterGlobalReplayFilterSave();
      setShowGlobalReplayFilter(false);
    } catch (err) {
      setGlobalReplayFilterError(err.message || t('settings.errors.save'));
    } finally {
      setGlobalReplayFilterSaving(false);
    }
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

  const navigateMainView = (nextView) => {
    setActiveView((currentView) => {
      if (currentView === nextView) return currentView;
      return nextView;
    });
  };

  const openMainPlayersSubview = (tab) => {
    const nextTab = String(tab || 'summary');
    setMainPlayersTab(nextTab);
    navigateMainView('players');
  };

  // Player names render in the standard foreground. The rank palette that used
  // to tint them (15 categorical hues assigned to the most-played players)
  // encoded games-played — which the sorted Games column already shows — at the
  // cost of 16 competing hues per screen, two of them below WCAG AA. The
  // replay's own player colour is a different thing entirely and is kept: it is
  // shown as a swatch on the game-detail screens, where it keys a name to its
  // base on the map.
  const renderPlayerLabel = (name) => (
    <span><PlayerDisplayName name={name} /></span>
  );

  const renderPlayerLinkLabel = (name, playerKey) => {
    if (!playerKey) return <span><PlayerDisplayName name={name} /></span>;
    return (
      <button
        type="button"
        className="workflow-player-name-link"
        title={t('player.analyze')}
        onClick={(e) => { e.stopPropagation(); openMainPlayer(playerKey); }}
      >
        <PlayerDisplayName name={name} />
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
    return <img src={url} alt={raceLabel(race)} title={raceLabel(race)} className="workflow-race-icon" />;
  };

  /** Report-page title. Deliberately NOT the games-list cell: a heading wants
   *  the match, not the metadata. Race icon and name only, teams joined by
   *  "vs", winner in weight. Colour, country, fingerprint and crown all live
   *  in the roster below, where there is room to align them. */
  const renderGameTitlePlayers = (game) => {
    const players = Array.isArray(game?.players) ? game.players : [];
    if (players.length === 0) {
      return renderPlayersMatchup(game?.players_label || '');
    }
    const winnerKnown = players.some((player) => player.is_winner);
    const teams = playersHaveDistinctTeams(players)
      ? teamGroupsFromPlayers(players)
      : [players];

    return (
      <span className="workflow-game-title-players">
        {teams.map((team, teamIdx) => (
          <React.Fragment key={`title-team-${teamIdx}`}>
            {teamIdx > 0 ? <span className="workflow-game-title-vs">vs</span> : null}
            <span className="workflow-game-title-side">
              {team.map((player) => (
                <span
                  key={player.player_id}
                  className={`workflow-game-title-player${winnerKnown && player.is_winner ? ' rd-won' : ''}`}
                >
                  {renderWorkerIcon(player.race)}
                  <PlayerDisplayName name={player.name} />
                </span>
              ))}
            </span>
          </React.Fragment>
        ))}
      </span>
    );
  };

  const renderMainGameListPlayers = (game, linkPlayerNames = true) => {
    const players = Array.isArray(game?.players) ? game.players : [];
    if (players.length === 0) {
      return renderPlayersMatchup(game?.players_label || '');
    }
    const renderName = (player) => (linkPlayerNames
      ? renderPlayerLinkLabel(player.name, player.player_key)
      : renderPlayerLabel(player.name, player.player_key));
    const showFlags = linkPlayerNames || players.length < 8;
    const stackingMarker = game?.team_stacking ? (
      <span
        className="workflow-team-stacking-marker"
        data-tip={t('games.teamStackingTip')}
      >
        😈
      </span>
    ) : null;

    // The winning side reads as weight rather than a crown per name. Only mark
    // sides when the replay actually recorded a winner, otherwise every name
    // would be dimmed as a loser.
    const winnerKnown = players.some((player) => player.is_winner);
    const outcomeClass = (player) => (winnerKnown
      ? (player.is_winner ? ' rd-won' : ' rd-lost')
      : '');

    // One layout for every player count. A 1v1 is two teams of one, so it goes
    // through exactly the same grid as a 4x2; nothing branches on size.
    const hasTeams = playersHaveDistinctTeams(players);
    const teams = hasTeams ? teamGroupsFromPlayers(players) : [players];
    const noTeamInfo = !hasTeams && players.length > 1;
    const warningText = game?.team_info_incomplete
      ? t('games.teamInfoIncomplete')
      : t('games.noTeamInfo');

    // Teams are distributed evenly over as many rows as the players need, and a
    // team is never split across a break: 4 teams of 2 give 2 teams per row,
    // 2 teams of 4 give one team per row.
    const rowCount = Math.max(1, Math.ceil(players.length / PLAYERS_PER_LIST_ROW));
    const teamsPerRow = Math.max(1, Math.ceil(teams.length / rowCount));

    // Chunk the teams into explicit rows rather than letting them wrap: a
    // wrap point depends on rendered width, so it lands wherever it fits
    // (3 teams then 1) instead of where the team shape says it should.
    const teamRows = [];
    for (let i = 0; i < teams.length; i += teamsPerRow) {
      teamRows.push(teams.slice(i, i + teamsPerRow));
    }

    return (
      <span
        className="workflow-team-matchup workflow-team-matchup--rows"
        style={{ gridTemplateColumns: `repeat(${teamsPerRow}, max-content max-content)` }}
      >
        {teamRows.map((row, rowIdx) => {
          const teamsBefore = rowIdx * teamsPerRow;
          return (
            <span className="workflow-team-row" key={`team-row-${rowIdx}`}>
              {row.map((team, idxInRow) => {
                const isLastTeamOverall = teamsBefore + idxInRow === teams.length - 1;
                return (
                  <React.Fragment key={`team-${teamsBefore + idxInRow}`}>
                    <span className="workflow-team-side">
                      {team.map((player) => (
                        <span
                          key={player.player_id}
                          className={`workflow-team-player-pill${outcomeClass(player)}`}
                        >
                          {renderWorkerIcon(player.race)}
                          {showFlags ? <CountryFlag code={player.country_code} playerKey={player.player_key} /> : null}
                          {renderName(player)}
                          <FingerprintBadge match={player.fingerprint_match} compact />
                        </span>
                      ))}
                    </span>
                    <span className="workflow-team-vs">
                      {isLastTeamOverall || idxInRow === row.length - 1 ? '' : 'vs'}
                    </span>
                  </React.Fragment>
                );
              })}
              {rowIdx === teamRows.length - 1 && noTeamInfo ? (
                <span className="workflow-no-team-warning" data-tip={warningText}>⚠️</span>
              ) : null}
              {rowIdx === teamRows.length - 1 ? stackingMarker : null}
            </span>
          );
        })}
      </span>
    );
  };

  // Every games list renders through here, so a second list cannot drift from
  // the main one's columns, widths and row behaviour the way the session view
  // had (it was missing the data-table and --main classes, so it lost the
  // column widths, and it linked player names, which swallowed the row click).
  const renderGamesListTable = ({ games, tableRef = null, selectedId = null }) => {
    // When every game on screen is 2-player (1v1) the Players column is narrow
    // and the table leaves horizontal slack; bump type/icons up from the
    // compact 8-player defaults to use it.
    const allTwoPlayer = games.length > 0
      && games.every((game) => (Array.isArray(game?.players) ? game.players.length : 0) <= 2);
    return (
      <table
        ref={tableRef}
        className={`data-table workflow-table workflow-games-list-table workflow-games-list-table--main${allTwoPlayer ? ' workflow-games-list-table--roomy' : ''}`}
      >
        <thead>
          <tr>
            <th>{t('games.table.played')}</th>
            <th>{t('games.table.players')}</th>
            <th>{t('common.map')}</th>
            <th>{t('common.time')}</th>
            <th>{t('common.featuring')}</th>
          </tr>
        </thead>
        <tbody>
          {games.map((game) => (
            <tr
              key={game.replay_id}
              className={selectedId === game.replay_id ? 'workflow-selected-row' : ''}
              onClick={() => openMainGame(game.replay_id)}
            >
              <td className="workflow-games-list-played">{formatRelativeReplayDate(game.replay_date)}</td>
              <td className="workflow-games-list-players">{renderMainGameListPlayers(game, false)}</td>
              <td className="workflow-games-list-map">{renderMapNameWithKind(game.map_name, game.map_kind)}</td>
              <td className="workflow-games-list-duration">{formatDuration(game.duration_seconds)}</td>
              <td className="workflow-games-list-featuring">
                <FeaturingCell featuring={game.featuring} featuringKeys={game.featuring_keys} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
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
      // Scouts are intentionally not surfaced: a scout's timing is misleading
      // (instantaneous for Zerg overlords, late for P/T) and we can't tell if
      // it actually arrived. Dropped per issue #159 — the BO openers carry the
      // early-game signal instead.
      if (normalizeEventType(event?.type) === 'scout') {
        return false;
      }
      return summaryTextMatches(gameEventSearchText(event, markerRegistry));
    });
    const deduped = [];
    for (let idx = 0; idx < visibleEvents.length; idx += 1) {
      const event = visibleEvents[idx];
      const prev = deduped.length > 0 ? deduped[deduped.length - 1] : null;
      // Recall combos (multiple recalls within seconds of each other) are
      // pre-collapsed on the backend (worldstate clusters by source base with
      // a 20s sliding gap). At this layer recalls dedup like every other
      // event — identical adjacent descriptions are redundant.
      if (prev && gameEventDescription(prev, markerRegistry) === gameEventDescription(event, markerRegistry)) {
        continue;
      }
      deduped.push(event);
    }
    return deduped;
  }, [mainGame?.game_events, mainSummaryFilters, markerRegistry, t]);

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
      attack: false,
      expansion: false,
      leaves: false,
      nuke: false,
      drop: false,
      recall: false,
      scout: false,
      becameRace: false,
      rush: false,
      alliance: false,
    };
    const allEvents = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    for (const event of allEvents) {
      if (isStructuralGameEventType(event?.type)) continue;
      const nt = normalizeEventType(event?.type);
      if (nt === 'takeover') continue;
      const text = gameEventSearchText(event, markerRegistry);
      if (nt === 'attack') base.attack = true;
      if (nt === 'expansion') base.expansion = true;
      if (nt === 'leave_game' || nt === 'player_dropped' || nt === 'mass_disconnect' || nt === 'player_stopped_playing') base.leaves = true;
      if (SUMMARY_TOPIC_PATTERNS.nuke.test(text)) base.nuke = true;
      if (SUMMARY_TOPIC_PATTERNS.drop.test(text)) base.drop = true;
      if (SUMMARY_TOPIC_PATTERNS.recall.test(text)) base.recall = true;
      if (nt === 'scout' || SUMMARY_TOPIC_PATTERNS.scout.test(text)) base.scout = true;
      if (SUMMARY_TOPIC_PATTERNS.becameRace.test(text) || nt === 'became_terran' || nt === 'became_zerg') base.becameRace = true;
      if (SUMMARY_TOPIC_PATTERNS.rush.test(text)) base.rush = true;
      if (nt === 'late_alliance') base.alliance = true;
    }
    return base;
  }, [mainGame?.game_events, markerRegistry, t]);
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

  // Up/Down arrows step through the visible event rows so the map animation can
  // be followed without clicking. Ignored while typing in a text field.
  useEffect(() => {
    if (mainGameTab !== 'events') return undefined;
    const handler = (e) => {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
      const target = e.target;
      const tag = String(target?.tagName || '').toLowerCase();
      const isCheckable = tag === 'input' && (target.type === 'checkbox' || target.type === 'radio');
      if (target?.isContentEditable || ((tag === 'input' && !isCheckable) || tag === 'textarea' || tag === 'select')) {
        return;
      }
      const visible = filteredGameEvents;
      if (visible.length === 0) return;
      e.preventDefault();
      const curIdx = selectedMainGameEvent ? visible.indexOf(selectedMainGameEvent) : -1;
      let nextIdx;
      if (curIdx < 0) {
        nextIdx = e.key === 'ArrowDown' ? 0 : visible.length - 1;
      } else {
        nextIdx = e.key === 'ArrowDown'
          ? Math.min(visible.length - 1, curIdx + 1)
          : Math.max(0, curIdx - 1);
      }
      const topicIdx = topicFilteredGameEvents.indexOf(visible[nextIdx]);
      if (topicIdx >= 0) setMainSelectedGameEventKey(gameEventTopicKey(topicIdx));
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [mainGameTab, filteredGameEvents, topicFilteredGameEvents, selectedMainGameEvent]);

  // Keep the selected row visible in the scrolling events list as arrows move it.
  useEffect(() => {
    if (mainGameTab !== 'events') return;
    const selected = document.querySelector('.workflow-events .workflow-event-row-selected');
    if (selected && typeof selected.scrollIntoView === 'function') {
      selected.scrollIntoView({ block: 'nearest' });
    }
  }, [selectedMainGameEventKeyResolved, mainGameTab]);
  const mainGameTeamByPlayerID = useMemo(() => {
    const m = new Map();
    (Array.isArray(mainGamePlayers) ? mainGamePlayers : []).forEach((p) => {
      if (p?.player_id != null) m.set(String(p.player_id), p.team);
    });
    return m;
  }, [mainGamePlayers]);
  // Team-colour polygon borders apply only when there's a real team (≥2 players
  // sharing one). 1v1 / 1v1v1 / FFA — every player on their own team — keep the
  // plain player-colour border. Guard on ≥2 distinct teams too so a replay with
  // no team info (everyone on team 0) isn't mistaken for one big team.
  const isTeamGame = useMemo(() => {
    const counts = new Map();
    (Array.isArray(mainGamePlayers) ? mainGamePlayers : []).forEach((p) => {
      counts.set(p?.team, (counts.get(p?.team) || 0) + 1);
    });
    if (counts.size < 2) return false;
    return [...counts.values()].some((n) => n >= 2);
  }, [mainGamePlayers]);
  const polygonStrokeFor = (ownerColor, team) => (
    isTeamGame && team != null
      ? { teamColor: getTeamColor(team), strokeWidth: 0.9 }
      : { teamColor: ownerColor, strokeWidth: 0.4 }
  );
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
        const team = mainGameTeamByPlayerID.get(String(entry.owner.player_id));
        return {
          key: `ownership-${idx}-${entry.base?.name || 'base'}`,
          points,
          ownerName: entry.owner.name,
          ownerPlayerID: Number(entry.owner.player_id || 0),
          baseName: String(entry.base?.name || '').trim(),
          ownerColor,
          ...polygonStrokeFor(ownerColor, team),
        };
      })
      .filter(Boolean);
  }, [selectedMainGameEvent, mainEventMapBounds, mainGameTeamByPlayerID, isTeamGame]);
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
      const percentPoints = polygon
        .map((point) => mapPointToPercent(point, bounds))
        .filter(Boolean);
      const points = percentPoints.map((point) => `${point.x},${point.y}`).join(' ');
      if (!points) return;
      const pid = eventActorID(ev);
      const centroid = percentPoints.reduce(
        (acc2, p) => ({ x: acc2.x + p.x / percentPoints.length, y: acc2.y + p.y / percentPoints.length }),
        { x: 0, y: 0 },
      );
      const team = pid != null ? mainGameTeamByPlayerID.get(String(pid)) : undefined;
      const ownerColor = playerColorToCss(ev.actor.color);
      acc.push({
        key: `sum-start-poly-${pid != null ? pid : idx}`,
        points,
        centroid,
        playerID: pid,
        ownerName: String(ev.actor.name || '').trim() || t('events.playerFallback'),
        baseName: String(ev?.base?.name || '').trim(),
        ownerColor,
        ...polygonStrokeFor(ownerColor, team),
      });
    });
    return acc;
  }, [mainGame?.game_events, mainEventMapBounds, mainGameTeamByPlayerID, isTeamGame, t]);
  // The BO openers event sits at 0:00 with no persisted ownership snapshot, so
  // draw the starting-location polygons directly from the player_start events.
  // Every other event keeps using its ownership snapshot.
  const selectedMainGameMapPolygons = useMemo(() => {
    const nt = normalizeEventType(selectedMainGameEvent?.type);
    if (nt === 'bo_openers') return summaryMapStartPolygons;
    // When a player leaves or stops playing, their territory vanishes from the
    // map (the marker takes its place at the last-known location).
    if (nt === 'leave_game' || nt === 'player_dropped' || nt === 'mass_disconnect' || nt === 'player_stopped_playing') {
      const goneID = Number(selectedMainGameEvent?.actor?.player_id || 0);
      if (goneID > 0) {
        return selectedMainGameOwnershipPolygons.filter((p) => p.ownerPlayerID !== goneID);
      }
    }
    return selectedMainGameOwnershipPolygons;
  }, [selectedMainGameEvent, summaryMapStartPolygons, selectedMainGameOwnershipPolygons]);
  // Per-start-location labels for the consolidated BO openers event: race icon
  // + crown + name on line 1, BO name(s) on line 2, painted on each player's
  // starting polygon. Only computed when that event is selected.
  const selectedMainGameBOLabels = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'bo_openers') return [];
    const lines = boOpenerLines(selectedMainGameEvent);
    if (lines.length === 0) return [];
    const lineByPlayer = new Map(lines.map((line) => [String(line.playerID), line]));
    return summaryMapStartPolygons
      .map((poly) => {
        const line = poly.playerID != null ? lineByPlayer.get(String(poly.playerID)) : null;
        if (!line || !poly.centroid) return null;
        return {
          key: `bo-label-${poly.key}`,
          x: poly.centroid.x,
          y: poly.centroid.y,
          name: line.name,
          color: playerColorToCss(line.color),
          rawColor: line.color,
          race: line.race,
          isWinner: line.isWinner,
          boNames: line.boNames,
        };
      })
      .filter(Boolean);
  }, [selectedMainGameEvent, summaryMapStartPolygons]);
  const mainGameFeaturingPillsList = useMemo(
    () => buildMainGameFeaturingPills(mainGame, markerDefinitions),
    [mainGame, markerDefinitions, t],
  );
  const selectedMainGameArrow = useMemo(() => {
    if (!selectedMainGameEvent || !isArrowEventType(selectedMainGameEvent.type)) return null;
    // Recall arrow: source (cast click) → target (inferred Arbiter location).
    // Suppress when no destination was inferred — drawing a vector to the
    // source would be a misleading "from→to itself".
    if (normalizeEventType(selectedMainGameEvent.type) === 'recall') {
      const targetAnchor = polygonCenter(selectedMainGameEvent?.target_base?.polygon)
        || selectedMainGameEvent?.target_base?.center
        || selectedMainGameEvent?.target_point;
      if (!targetAnchor) return null;
      const sourceAnchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
        || selectedMainGameEvent?.base?.center
        || selectedMainGameEvent?.source_point;
      const fromRaw = mapPointToPercent(sourceAnchor, mainEventMapBounds);
      const toRaw = mapPointToPercent(targetAnchor, mainEventMapBounds);
      if (!fromRaw || !toRaw) return null;
      // Pull both endpoints inward so the arrow doesn't crash into either
      // overlay icon: head stays clear of the Arbiter at the destination, and
      // the tail starts a few % past the recalled-units cluster at the source.
      // The pullback is clamped to a fraction of the arrow length so very
      // short arrows don't invert.
      const dx = toRaw.x - fromRaw.x;
      const dy = toRaw.y - fromRaw.y;
      const len = Math.sqrt(dx * dx + dy * dy);
      const headPullback = Math.min(6, len * 0.4);
      const tailPullback = Math.min(6, len * 0.4);
      const headFactor = len > 0 ? Math.max(0, len - headPullback) / len : 1;
      const tailFactor = len > 0 ? Math.min(1, tailPullback / len) : 0;
      const adjustedTo = {
        x: fromRaw.x + dx * headFactor,
        y: fromRaw.y + dy * headFactor,
      };
      const adjustedFrom = {
        x: fromRaw.x + dx * tailFactor,
        y: fromRaw.y + dy * tailFactor,
      };
      return {
        from: adjustedFrom,
        to: adjustedTo,
        sourceAnchor: fromRaw,
        color: playerColorToCss(selectedMainGameEvent?.actor?.color),
      };
    }
    // Drop arrow: source_base (where the player loaded — falls back to start
    // base on the backend when no Load was paired) → event.base (destination,
    // where the unload happened). The dropped-unit overlay is anchored at the
    // source so the user sees "here's what got loaded" at the tail.
    const eventType = normalizeEventType(selectedMainGameEvent.type);
    if (['drop', 'cliff_drop', 'nydus_attack'].includes(eventType)) {
      const sourceAnchor = polygonCenter(selectedMainGameEvent?.source_base?.polygon)
        || selectedMainGameEvent?.source_base?.center
        || selectedMainGameEvent?.source_point;
      const targetAnchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
        || selectedMainGameEvent?.base?.center
        || selectedMainGameEvent?.target_point;
      const fromRaw = mapPointToPercent(sourceAnchor, mainEventMapBounds);
      const toRaw = mapPointToPercent(targetAnchor, mainEventMapBounds);
      if (!fromRaw || !toRaw) return null;
      const dx = toRaw.x - fromRaw.x;
      const dy = toRaw.y - fromRaw.y;
      const len = Math.sqrt(dx * dx + dy * dy);
      const headPullback = Math.min(6, len * 0.4);
      const tailPullback = Math.min(6, len * 0.4);
      const headFactor = len > 0 ? Math.max(0, len - headPullback) / len : 1;
      const tailFactor = len > 0 ? Math.min(1, tailPullback / len) : 0;
      return {
        from: { x: fromRaw.x + dx * tailFactor, y: fromRaw.y + dy * tailFactor },
        to: { x: fromRaw.x + dx * headFactor, y: fromRaw.y + dy * headFactor },
        sourceAnchor: fromRaw,
        color: playerColorToCss(selectedMainGameEvent?.actor?.color),
      };
    }
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
    let unitNames = Array.isArray(selectedMainGameEvent.attack_unit_types) && selectedMainGameEvent.attack_unit_types.length > 0
      ? selectedMainGameEvent.attack_unit_types
      : fallbackOverlayUnitNamesForEvent(selectedMainGameEvent.type, actorRow?.race);
    // For recalls: show recalled units (Zealot/Dragoon/etc.) at the source
    // for any inference path (attack-coincidence or post-recall activity)
    // — the backend harvests army composition either way. Always strip
    // "Arbiter" — the recall overlay paints it separately at the
    // destination; painting it at source too would duplicate the icon.
    const eventTypeForOverlay = normalizeEventType(selectedMainGameEvent.type);
    if (eventTypeForOverlay === 'recall') {
      // No destination → no source→target arrow → no source units.
      if (!selectedMainGameEvent?.target_base) return [];
      unitNames = unitNames.filter((name) => String(name || '').toLowerCase() !== 'arbiter');
    }
    // For drops: strip the transport itself (Dropship/Shuttle/Overlord) from
    // the source-side unit overlay — the transport icon is painted separately
    // at the destination. Workers (SCV/Probe/Drone) and combat units stay.
    if (['drop', 'cliff_drop'].includes(eventTypeForOverlay)) {
      const transports = new Set(['dropship', 'shuttle', 'overlord']);
      unitNames = unitNames.filter((name) => !transports.has(String(name || '').toLowerCase()));
    }
    return unitNames
      .map((name) => ({ name, icon: getUnitIcon(name) }))
      .filter((item) => item.icon)
      .slice(0, 4);
  }, [selectedMainGameArrow, selectedMainGameEvent, mainGamePlayers]);
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
  // Drop overlay: anchor the race-correct transport vessel (Dropship/Shuttle/
  // Overlord) at the destination, mirroring the Arbiter overlay used for
  // recalls. Always render when the event is a drop variant — drops are
  // defined by the vessel and the icon is the most recognizable signal.
  const selectedMainGameDropOverlay = useMemo(() => {
    const evType = normalizeEventType(selectedMainGameEvent?.type);
    if (!['drop', 'cliff_drop'].includes(evType)) return null;
    const anchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
      || selectedMainGameEvent?.base?.center
      || selectedMainGameEvent?.target_point;
    if (!anchor) return null;
    const actorPid = Number(selectedMainGameEvent?.actor?.player_id || 0);
    const actorRow = mainGamePlayers.find((player) => Number(player?.player_id || 0) === actorPid);
    const icon = dropTransportIconForRace(actorRow?.race);
    if (!icon) return null;
    const point = mapPointToPercent(anchor, mainEventMapBounds);
    if (!point) return null;
    return { icon, point };
  }, [selectedMainGameEvent, mainGamePlayers, mainEventMapBounds]);
  // Recall overlay: anchor the Arbiter at the inferred destination when known
  // (target_base populated by the backend's attack-coincidence / post-recall
  // activity proxies); otherwise fall back to the cast point. The arrow
  // useMemo above already draws source→target when both endpoints exist.
  const selectedMainGameRecallOverlay = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'recall') return null;
    const targetAnchor = polygonCenter(selectedMainGameEvent?.target_base?.polygon)
      || selectedMainGameEvent?.target_base?.center
      || selectedMainGameEvent?.target_point;
    const sourceAnchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
      || selectedMainGameEvent?.base?.center
      || selectedMainGameEvent?.source_point;
    const anchor = targetAnchor || sourceAnchor;
    if (!anchor) return null;
    const icon = getUnitIcon('arbiter');
    if (!icon) return null;
    const point = mapPointToPercent(anchor, mainEventMapBounds);
    if (!point) return null;
    return { icon, point };
  }, [selectedMainGameEvent, mainEventMapBounds]);
  // Proxy building overlay: paint the proxied building (Gateway / Barracks /
  // Factory / Starport) at its placement coordinates for any proxy_* event —
  // the backend already populates base coordinates for these.
  const selectedMainGameProxyBuildingOverlay = useMemo(() => {
    const nt = normalizeEventType(selectedMainGameEvent?.type);
    const iconKey = {
      proxy_gate: 'gateway',
      proxy_rax: 'barracks',
      proxy_factory: 'factory',
      proxy_starport: 'starport',
    }[nt];
    if (!iconKey) return null;
    const anchor = polygonCenter(selectedMainGameEvent?.base?.polygon)
      || selectedMainGameEvent?.base?.center;
    if (!anchor) return null;
    const icon = getUnitIcon(iconKey);
    if (!icon) return null;
    const point = mapPointToPercent(anchor, mainEventMapBounds);
    if (!point) return null;
    return { icon, point };
  }, [selectedMainGameEvent, mainEventMapBounds]);
  // Alliance overlay: for late_alliance events, render one 🤝 emoji per pair
  // of allied players at the midpoint between their bases, plus a pair of
  // bidirectional arrows running base↔🤝 in each player's color. The pair is
  // skipped entirely when either player has no owned base at event time (per
  // the spec — alliance is between active footprints, not absent ones).
  const selectedMainGameAllianceOverlay = useMemo(() => {
    if (normalizeEventType(selectedMainGameEvent?.type) !== 'late_alliance') return null;
    if (!mainEventMapBounds) return null;
    const teams = Array.isArray(selectedMainGameEvent?.alliance_teams) ? selectedMainGameEvent.alliance_teams : [];
    if (teams.length === 0) return null;
    const ownership = Array.isArray(selectedMainGameEvent?.ownership) ? selectedMainGameEvent.ownership : [];
    const anchorForPlayer = (playerID) => {
      const owned = ownership.filter((entry) => Number(entry?.owner?.player_id || 0) === playerID && entry?.base?.center);
      if (owned.length === 0) return null;
      const preferred = owned.find((entry) => String(entry?.base?.kind || '').toLowerCase() === 'starting') || owned[0];
      const point = mapPointToPercent(preferred?.base?.center, mainEventMapBounds);
      if (!point) return null;
      return { point, base: preferred.base };
    };
    const pairs = [];
    const consumedBases = [];
    for (const team of teams) {
      if (!Array.isArray(team) || team.length < 2) continue;
      for (let i = 0; i < team.length; i += 1) {
        for (let j = i + 1; j < team.length; j += 1) {
          const a = team[i];
          const b = team[j];
          const aAnchor = anchorForPlayer(Number(a?.player_id || 0));
          const bAnchor = anchorForPlayer(Number(b?.player_id || 0));
          if (!aAnchor || !bAnchor) continue;
          pairs.push({
            key: `${Number(a?.player_id || 0)}-${Number(b?.player_id || 0)}`,
            a: { from: aAnchor.point, color: playerColorToCss(a?.color) },
            b: { from: bAnchor.point, color: playerColorToCss(b?.color) },
            mid: { x: (aAnchor.point.x + bAnchor.point.x) / 2, y: (aAnchor.point.y + bAnchor.point.y) / 2 },
          });
          consumedBases.push(aAnchor.base, bAnchor.base);
        }
      }
    }
    if (pairs.length === 0) return null;
    return { pairs, consumedBases };
  }, [selectedMainGameEvent, mainEventMapBounds]);
  // Animation category for the selected event — drives the per-type map
  // animation (scoped via a class on the map frame) so each event type unfolds
  // with its own choreography rather than a single generic fade.
  const selectedEventAnimCategory = useMemo(() => {
    const nt = normalizeEventType(selectedMainGameEvent?.type);
    if (!nt) return '';
    if (['drop', 'cliff_drop'].includes(nt)) return 'drop';
    if (nt === 'recall') return 'recall';
    if (nt === 'nuke') return 'nuke';
    if (nt === 'leave_game' || nt === 'player_dropped' || nt === 'mass_disconnect' || nt === 'player_stopped_playing') return 'leaves';
    if (nt === 'late_alliance') return 'alliance';
    if (nt === 'became_terran' || nt === 'became_zerg') return 'became';
    if (['cannon_rush', 'bunker_rush', 'zergling_rush', 'proxy_gate', 'proxy_rax', 'proxy_factory', 'proxy_starport'].includes(nt)) return 'rush';
    if (nt === 'attack') return 'attack';
    if (selectedMainGameExpansionOverlay) return 'expansion';
    if (selectedMainGameArrow) return 'attack';
    return '';
  }, [selectedMainGameEvent, selectedMainGameExpansionOverlay, selectedMainGameArrow]);
  // Last location the selected event's actor owned a base — found by scanning the
  // ownership snapshots up to this event. Used to anchor events whose own snapshot
  // doesn't carry a base (a player leaving, stopping, or being mind-controlled).
  const selectedActorLastBasePoint = useMemo(() => {
    if (!mainEventMapBounds) return null;
    const actorID = Number(selectedMainGameEvent?.actor?.player_id || 0);
    if (!actorID) return null;
    const events = Array.isArray(mainGame?.game_events) ? mainGame.game_events : [];
    const cutoff = Number(selectedMainGameEvent?.second);
    let bestCenter = null;
    let bestSecond = -1;
    for (const ev of events) {
      const sec = Number(ev?.second) || 0;
      if (Number.isFinite(cutoff) && sec > cutoff) continue;
      const own = Array.isArray(ev?.ownership) ? ev.ownership : [];
      const mine = own.filter((o) => Number(o?.owner?.player_id || 0) === actorID && (o?.base?.center || o?.base?.polygon));
      if (mine.length === 0 || sec < bestSecond) continue;
      const preferred = mine.find((o) => String(o?.base?.kind || '').toLowerCase() === 'starting') || mine[0];
      const c = polygonCenter(preferred?.base?.polygon) || preferred?.base?.center;
      if (c) { bestCenter = c; bestSecond = sec; }
    }
    return bestCenter ? mapPointToPercent(bestCenter, mainEventMapBounds) : null;
  }, [selectedMainGameEvent, mainGame?.game_events, mainEventMapBounds]);
  // Leave / stop-playing: the player's territory vanishes and a prominent flag
  // (left) or zzz (stopped) marker takes its place at their last-known base.
  const selectedLeaveInfo = useMemo(() => {
    const nt = normalizeEventType(selectedMainGameEvent?.type);
    const leaveTypes = ['leave_game', 'player_dropped', 'mass_disconnect', 'player_stopped_playing'];
    if (!leaveTypes.includes(nt) || !selectedActorLastBasePoint) return null;
    const actorID = Number(selectedMainGameEvent?.actor?.player_id || 0);
    const actorRow = mainGamePlayers.find((p) => Number(p?.player_id || 0) === actorID);
    const name = actorRow?.name || selectedMainGameEvent?.actor?.name || t('events.playerFallback');
    const emoji = nt === 'player_stopped_playing' ? '💤' : (nt === 'player_dropped' || nt === 'mass_disconnect') ? '🔌' : '🏳️';
    return { name, emoji, point: selectedActorLastBasePoint, color: playerColorToCss(actorRow?.color) };
  }, [selectedMainGameEvent, selectedActorLastBasePoint, mainGamePlayers, t]);
  // Became Terran/Zerg (mind control): a Dark Archon icon planted prominently at
  // the mind-controlled player's last-known base.
  const selectedBecameOverlay = useMemo(() => {
    if (selectedEventAnimCategory !== 'became' || !selectedActorLastBasePoint) return null;
    const icon = getUnitIcon('darkarchon');
    if (!icon) return null;
    return { icon, point: selectedActorLastBasePoint };
  }, [selectedEventAnimCategory, selectedActorLastBasePoint]);
  // Focal point of the selected event: where the action happens on the map, so
  // the narration caption can sit right below it (rather than pinned to the map
  // bottom). `travel` events move from source→target — the caption (and the
  // moving icons) travel together along that path; static events anchor in place.
  // Events with no map anchor (e.g. the opener summary) return null → the caption
  // falls back to the bottom of the map.
  const selectedEventFocus = useMemo(() => {
    const cat = selectedEventAnimCategory;
    const arrow = selectedMainGameArrow;
    if ((cat === 'attack' || cat === 'rush' || cat === 'drop' || cat === 'nuke') && arrow) {
      return { travel: true, from: arrow.from, to: arrow.to };
    }
    // Recall: units start at the source, the Arbiter crosses to the destination,
    // and the units reappear there. Travel when both endpoints are known.
    if (cat === 'recall' && selectedMainGameRecallOverlay) {
      if (arrow) return { travel: true, from: arrow.from, to: selectedMainGameRecallOverlay.point };
      return { travel: false, from: selectedMainGameRecallOverlay.point, to: selectedMainGameRecallOverlay.point };
    }
    if (cat === 'expansion' && selectedMainGameExpansionOverlay) {
      return { travel: false, from: selectedMainGameExpansionOverlay.point, to: selectedMainGameExpansionOverlay.point };
    }
    if (cat === 'leaves' && selectedLeaveInfo) {
      return { travel: false, from: selectedLeaveInfo.point, to: selectedLeaveInfo.point };
    }
    if (cat === 'became' && selectedBecameOverlay) {
      return { travel: false, from: selectedBecameOverlay.point, to: selectedBecameOverlay.point };
    }
    if (cat === 'alliance' && selectedMainGameAllianceOverlay?.pairs?.length) {
      const mid = selectedMainGameAllianceOverlay.pairs[0].mid;
      return { travel: false, from: mid, to: mid };
    }
    if (selectedMainGameDropOverlay) {
      return { travel: false, from: selectedMainGameDropOverlay.point, to: selectedMainGameDropOverlay.point };
    }
    if (selectedMainGameProxyBuildingOverlay) {
      return { travel: false, from: selectedMainGameProxyBuildingOverlay.point, to: selectedMainGameProxyBuildingOverlay.point };
    }
    return null;
  }, [selectedEventAnimCategory, selectedMainGameArrow, selectedMainGameExpansionOverlay, selectedMainGameRecallOverlay, selectedLeaveInfo, selectedBecameOverlay, selectedMainGameAllianceOverlay, selectedMainGameDropOverlay, selectedMainGameProxyBuildingOverlay]);
  // Map aspect ratio (w/h) so the frame can be capped by viewport height while
  // preserving shape. BW maps are typically square (→ 1).
  const mainEventMapAspect = useMemo(() => {
    const w = Number(mainGame?.map_width_pixels) || 0;
    const h = Number(mainGame?.map_height_pixels) || 0;
    return (w > 0 && h > 0) ? w / h : 1;
  }, [mainGame?.map_width_pixels, mainGame?.map_height_pixels]);
  // Size the map column to the smaller of 70% of the section or the width at
  // which the map's height fits the viewport — and hand any leftover width to
  // the events list (rather than leaving slack in a fixed 70% column).
  useLayoutEffect(() => {
    if (mainGameTab !== 'events') return undefined;
    const el = mainEventsLayoutEl;
    if (!el) return undefined;
    const compute = () => {
      if (window.innerWidth <= 1100) { setMainEventsMapColPx(0); return; }
      const layoutW = el.clientWidth;
      const widthBound = 0.7 * layoutW;
      // The map panel is sticky to the top of the viewport, so it may use almost
      // the full viewport height (small margin to avoid a scrollbar when stuck).
      const heightBound = mainEventMapAspect * (window.innerHeight - 28);
      setMainEventsMapColPx(Math.max(0, Math.round(Math.min(widthBound, heightBound))));
    };
    compute();
    const ro = new ResizeObserver(compute);
    ro.observe(el);
    window.addEventListener('resize', compute);
    return () => { ro.disconnect(); window.removeEventListener('resize', compute); };
  }, [mainGameTab, mainEventsLayoutEl, mainEventMapAspect]);
  // Slide the moving overlays (army icons, drop vessel, narration caption) from
  // the event's source point to its target point. Done via the Web Animations
  // API rather than CSS keyframes because @keyframes can't read the per-event
  // endpoint values; left/top are % of the frame, so this stays correct at any
  // map size. Re-runs on each selection (the elements remount with a fresh key).
  useLayoutEffect(() => {
    if (mainGameTab !== 'events') return;
    const focus = selectedEventFocus;
    if (!focus || !focus.travel) return;
    if (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const frame = document.querySelector('.workflow-event-map-panel .workflow-event-map-frame');
    if (!frame) return;
    const cat = selectedEventAnimCategory;
    const fromXY = { left: `${focus.from.x}%`, top: `${focus.from.y}%` };
    const toXY = { left: `${focus.to.x}%`, top: `${focus.to.y}%` };
    const slide = { duration: 2300, easing: 'cubic-bezier(0.4, 0.1, 0.5, 1)', fill: 'both' };
    const run = (sel, keyframes, opts) => {
      const el = frame.querySelector(sel);
      if (el && typeof el.animate === 'function') el.animate(keyframes, opts);
    };
    // The caption always slides with the action.
    run('.workflow-event-map-narration--travel', [fromXY, toXY], slide);
    if (cat === 'drop') {
      // Vessel crosses to the target, then vanishes (the cargo is unloaded).
      run('.workflow-event-map-vessel--travel', [
        { ...fromXY, opacity: 1, offset: 0 },
        { ...toXY, opacity: 1, offset: 0.72 },
        { ...toXY, opacity: 0, offset: 1 },
      ], { duration: 2500, easing: 'ease-in', fill: 'both' });
    } else if (cat === 'recall') {
      // Arbiter crosses to the destination; the units blink out at the source
      // and reappear at the destination as it arrives. The Arbiter rides an
      // inset stretch of the path (like the old arrow) so it doesn't sit on top
      // of the units waiting at either end.
      const lerp = (a, b, t) => ({ left: `${a.x + (b.x - a.x) * t}%`, top: `${a.y + (b.y - a.y) * t}%` });
      run('.workflow-event-map-arbiter--travel', [
        lerp(focus.from, focus.to, 0.25),
        lerp(focus.from, focus.to, 0.78),
      ], { duration: 2600, easing: 'ease-in-out', fill: 'both' });
      run('.workflow-event-map-unit-overlay', [
        { ...fromXY, opacity: 1, offset: 0 },
        { ...fromXY, opacity: 1, offset: 0.32 },
        { ...fromXY, opacity: 0, offset: 0.46 },
        { ...toXY, opacity: 0, offset: 0.6 },
        { ...toXY, opacity: 1, offset: 1 },
      ], { duration: 3200, easing: 'ease', fill: 'both' });
    } else {
      // Attack / rush / nuke: the army marches to the target.
      run('.workflow-event-map-unit-overlay--travel', [fromXY, toXY], slide);
    }
  }, [mainGameTab, selectedEventFocus, selectedEventAnimCategory, selectedMainGameEventKeyResolved]);
  // The located caption is centred on its focal point; near a map edge a wide
  // line would spill past the frame (which clips). Cap its width to what fits on
  // the tighter side of the point so it wraps to more lines instead of clipping.
  useLayoutEffect(() => {
    if (mainGameTab !== 'events') return;
    const focus = selectedEventFocus;
    const frame = document.querySelector('.workflow-event-map-panel .workflow-event-map-frame');
    const cap = frame && frame.querySelector('.workflow-event-map-narration--located');
    if (!focus || !cap) return;
    const frameW = frame.clientWidth;
    if (!frameW) return;
    const centerX = (focus.to.x / 100) * frameW;
    const avail = 2 * Math.min(centerX, frameW - centerX) - 16;
    cap.style.maxWidth = `${Math.round(Math.max(96, Math.min(240, avail)))}px`;
  }, [mainGameTab, selectedEventFocus, selectedMainGameEventKeyResolved, mainEventsMapColPx]);
  // Trained-units overlay (issue #122 BONUS): paint a small chip with each
  // player's army composition (top 4 unit types + "+N" pill) on top of the
  // player's largest owned polygon at the moment of the selected event.
  // Workers (Drone/Probe/SCV) and Overlord are filtered out by the backend
  // when building trained_units_timeline.
  //
  // mainGameTrainedUnitsByPlayer: pre-indexed per-player sample arrays,
  // already sorted by second (the backend emits them in command order +
  // shifts by build time, so insertion order is monotonic enough for our
  // binary-search lookup). Memoized once per replay load.
  const mainGameTrainedUnitsByPlayer = useMemo(() => {
    const timeline = Array.isArray(mainGame?.trained_units_timeline) ? mainGame.trained_units_timeline : [];
    const byPlayer = new Map();
    for (const s of timeline) {
      const pid = Number(s?.player_id);
      const sec = Number(s?.second);
      const unit = String(s?.unit_type || '');
      if (!Number.isFinite(pid) || !Number.isFinite(sec) || !unit) continue;
      if (!byPlayer.has(pid)) byPlayer.set(pid, []);
      byPlayer.get(pid).push({ sec, unit });
    }
    // Sort each player's samples by sec — backend emits in command order
    // (monotonic seconds_from_game_start) but add-build-time can shuffle
    // adjacent entries when fast units precede slow ones in the same
    // morph row. Cheap to sort defensively.
    for (const arr of byPlayer.values()) arr.sort((a, b) => a.sec - b.sec);
    return byPlayer;
  }, [mainGame?.trained_units_timeline]);

  // selectedMainGameTrainedUnitsByPlayer: per-player {items: [{name, count}],
  // more: int} for the event's second. Top 4 unit types by count; everything
  // else collapses into a "+N" pill.
  const selectedMainGameTrainedUnitsByPlayer = useMemo(() => {
    const eventSec = Number(selectedMainGameEvent?.second);
    if (!Number.isFinite(eventSec)) return new Map();
    const out = new Map();
    for (const [pid, samples] of mainGameTrainedUnitsByPlayer.entries()) {
      // Binary search for the right boundary: count samples with sec ≤ eventSec.
      let lo = 0;
      let hi = samples.length;
      while (lo < hi) {
        const mid = (lo + hi) >>> 1;
        if (samples[mid].sec <= eventSec) lo = mid + 1;
        else hi = mid;
      }
      if (lo === 0) continue;
      const counts = new Map();
      for (let i = 0; i < lo; i += 1) {
        const u = samples[i].unit;
        counts.set(u, (counts.get(u) || 0) + 1);
      }
      const ranked = Array.from(counts.entries())
        .map(([name, count]) => ({ name, count }))
        .sort((a, b) => (b.count - a.count) || a.name.localeCompare(b.name));
      const items = ranked.slice(0, 4);
      const more = ranked.slice(4).reduce((acc, x) => acc + x.count, 0);
      out.set(pid, { items, more });
    }
    return out;
  }, [mainGameTrainedUnitsByPlayer, selectedMainGameEvent?.second]);

  // selectedMainGameTrainedUnitsAnchors: per-player render data anchored
  // at the centroid of the player's chosen base, in percent space.
  //
  // Placement strategy (pessimistic / per-base):
  //   1. Identify the set of bases that are "off-limits" for this event —
  //      bases where another overlay will already paint something
  //      (arrow endpoints for attack/scout/drop/recall/rush/nuke, the
  //      townhall icon for expansion/takeover, the leave-flag base, etc).
  //      For arrow events that means TWO bases off-limits (source + target).
  //   2. For each player, pick the first owned base that's NOT off-limits,
  //      in priority: starting → natural → other expansions.
  //   3. Anchor at that base's polygon centroid.
  //   4. If every owned base is off-limits → don't render this player's
  //      chip. (Last-resort fallback to any non-off-limits expansion is
  //      already covered by step 2.)
  const selectedMainGameTrainedUnitsAnchors = useMemo(() => {
    if (!selectedMainGameEvent || !mainEventMapBounds) return [];
    if (selectedMainGameTrainedUnitsByPlayer.size === 0) return [];
    const ownership = Array.isArray(selectedMainGameEvent?.ownership) ? selectedMainGameEvent.ownership : [];
    if (ownership.length === 0) return [];

    // basePolygonKey hashes a polygon by its first three vertex coordinates.
    // Used to test "is this ownership entry the same base as the event's
    // base / target_base / source_base?".
    const basePolygonKey = (polygon) => {
      if (!Array.isArray(polygon) || polygon.length === 0) return '';
      return polygon.slice(0, 3).map((p) => `${Math.round(Number(p?.x))}.${Math.round(Number(p?.y))}`).join('|');
    };
    const baseCenterKey = (center) => {
      if (!center) return '';
      return `c${Math.round(Number(center.x))}.${Math.round(Number(center.y))}`;
    };
    const offLimitsKeys = new Set();
    const claimBase = (base) => {
      if (!base) return;
      const pk = basePolygonKey(base.polygon);
      if (pk) offLimitsKeys.add(pk);
      const ck = baseCenterKey(base.center);
      if (ck) offLimitsKeys.add(ck);
    };
    // event.base is always off-limits — it's where the action's primary
    // overlay (arrow target, expansion icon, leave flag, recall source,
    // drop destination) is painted.
    claimBase(selectedMainGameEvent.base);
    // For arrow events, the source side is also off-limits.
    // - Recall: event.target_base is the inferred Arbiter destination
    //   (gets the Arbiter overlay).
    // - Drop: event.source_base is the loading base (gets the units
    //   overlay); event.target_base is the destination (gets the vessel
    //   overlay, plus duplicates event.base).
    // - Attack/scout/rush/nuke: the arrow originates from the actor's
    //   start base, which sits in event.ownership too — we approximate
    //   by claiming the polygon underneath selectedMainGameArrow.from.
    claimBase(selectedMainGameEvent.target_base);
    claimBase(selectedMainGameEvent.source_base);
    // For attack/scout/rush/nuke events the source isn't a discrete base
    // field — find the ownership entry whose center is closest to the
    // arrow's from-anchor in percent space, and mark it off-limits.
    if (selectedMainGameArrow?.from && !selectedMainGameEvent.source_base) {
      let bestEntry = null;
      let bestDist = Infinity;
      for (const entry of ownership) {
        const c = polygonCenter(entry?.base?.polygon) || entry?.base?.center;
        const cp = c ? mapPointToPercent(c, mainEventMapBounds) : null;
        if (!cp) continue;
        const d = distanceBetween(cp, selectedMainGameArrow.from);
        if (d < bestDist) {
          bestDist = d;
          bestEntry = entry;
        }
      }
      if (bestEntry && bestDist < 8) claimBase(bestEntry.base);
    }
    // Alliance overlay: every base anchoring a 🤝 arrow is off-limits so
    // the player's trained-units chip moves to a different owned base
    // (preserving production info when one exists, hiding when it doesn't).
    if (selectedMainGameAllianceOverlay?.consumedBases) {
      for (const b of selectedMainGameAllianceOverlay.consumedBases) claimBase(b);
    }

    const isOffLimits = (base) => {
      if (!base) return true;
      const pk = basePolygonKey(base.polygon);
      if (pk && offLimitsKeys.has(pk)) return true;
      const ck = baseCenterKey(base.center);
      if (ck && offLimitsKeys.has(ck)) return true;
      return false;
    };

    // Group ownership by player.
    const ownedByPlayer = new Map();
    for (const entry of ownership) {
      const pid = Number(entry?.owner?.player_id || 0);
      if (!pid) continue;
      if (!entry?.base?.polygon) continue;
      if (!ownedByPlayer.has(pid)) ownedByPlayer.set(pid, []);
      ownedByPlayer.get(pid).push(entry);
    }

    const playerIDs = Array.from(ownedByPlayer.keys()).sort((a, b) => a - b);
    const out = [];

    for (const pid of playerIDs) {
      const composition = selectedMainGameTrainedUnitsByPlayer.get(pid);
      if (!composition || composition.items.length === 0) continue;

      const bases = ownedByPlayer.get(pid) || [];
      if (bases.length === 0) continue;

      // Priority: starting → natural → other expansions (by polygon
      // bounding-box area as a stable tiebreak). Skip off-limits bases
      // AND any base whose centroid would land within 9% of the arrow
      // segment (the arrow is the event's primary visual — adjacent
      // bases sometimes put a chip uncomfortably close to the line).
      const ranked = [...bases].sort((a, b) => {
        const ak = String(a.base?.kind || '').toLowerCase();
        const bk = String(b.base?.kind || '').toLowerCase();
        const rank = (k) => (k === 'starting' ? 0 : k === 'natural' ? 1 : 2);
        const r = rank(ak) - rank(bk);
        if (r !== 0) return r;
        return polygonBoundingBoxArea(b.base.polygon) - polygonBoundingBoxArea(a.base.polygon);
      });
      const ARROW_AVOID_PCT = 9;
      const arrowFrom = selectedMainGameArrow?.from || null;
      const arrowTo = selectedMainGameArrow?.to || null;
      // Alliance overlay segments base→midpoint per pair — chips should also
      // stay clear of these so 🤝 + arrows aren't covered by production icons.
      const allianceSegments = (selectedMainGameAllianceOverlay?.pairs || []).flatMap((pair) => ([
        [pair.a.from, pair.mid],
        [pair.b.from, pair.mid],
      ]));
      let anchorBase = null;
      let point = null;
      for (const b of ranked) {
        if (isOffLimits(b.base)) continue;
        const cRaw = polygonCenter(b.base.polygon) || b.base.center;
        if (!cRaw) continue;
        const pct = mapPointToPercent(cRaw, mainEventMapBounds);
        if (!pct) continue;
        if (arrowFrom && arrowTo) {
          const d = distanceToSegment(pct, arrowFrom, arrowTo);
          if (d < ARROW_AVOID_PCT) continue;
        }
        let tooClose = false;
        for (const [from, to] of allianceSegments) {
          if (distanceToSegment(pct, from, to) < ARROW_AVOID_PCT) { tooClose = true; break; }
        }
        if (tooClose) continue;
        anchorBase = b;
        point = pct;
        break;
      }
      if (!anchorBase || !point) continue;

      out.push({
        playerID: pid,
        color: playerColorToCss(anchorBase.owner?.color),
        point,
        items: composition.items,
        more: composition.more,
      });
    }

    return out;
  }, [
    selectedMainGameEvent,
    mainEventMapBounds,
    selectedMainGameTrainedUnitsByPlayer,
    selectedMainGameArrow,
    selectedMainGameAllianceOverlay,
  ]);

  const mainPlayerInsights = [
    mainPlayerApmInsight,
    mainPlayerViewportInsight,
    mainPlayerCadenceInsight,
  ].filter(Boolean);
  const mainPlayerInsightLoading = mainPlayerApmInsightLoading || mainPlayerCadenceInsightLoading || mainPlayerViewportInsightLoading;
  const mainPlayerInsightErrors = [
    mainPlayerApmInsightError,
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
  // Hover tooltip for the Game Events map: surfaces the owner (and base) of the
  // polygon under the cursor without crowding the map with always-on labels.
  const [mainEventMapHover, setMainEventMapHover] = useState(null);
  const updateMainEventMapHover = (event, payload) => {
    if (!mainEventMapPanelEl) return;
    const rect = mainEventMapPanelEl.getBoundingClientRect();
    setMainEventMapHover({
      x: event.clientX - rect.left + 12,
      y: event.clientY - rect.top + 12,
      ...payload,
    });
  };
  const clearMainEventMapHover = () => setMainEventMapHover(null);
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
  const isResearchTiming = mainTimingCategory === 'research';
  const isHpTiming = mainTimingCategory === 'hp_upgrades';
  const isExpansionGasTiming = mainTimingCategory === 'expansion_gas';
  const mainTimingSeries = useMemo(() => {
    const timings = mainGame?.timings || {};
    const sortRows = (rows) => rows.sort((a, b) => {
      const raceDiff = raceRank(mainPlayersById.get(a?.player_id)?.race) - raceRank(mainPlayersById.get(b?.player_id)?.race);
      if (raceDiff !== 0) return raceDiff;
      const nameA = String(a?.name || '').toLowerCase();
      const nameB = String(b?.name || '').toLowerCase();
      if (nameA !== nameB) return nameA.localeCompare(nameB);
      return Number(a?.player_id || 0) - Number(b?.player_id || 0);
    });
    const withRace = (playerSeries, points) => {
      const playerRace = String(mainPlayersById.get(playerSeries?.player_id)?.race || '').trim();
      return {
        ...playerSeries,
        race: playerRace,
        race_icon: getRaceIcon(playerRace),
        points: points.map((p) => ({ ...p, race: playerRace })).sort((a, b) => a.second - b.second),
      };
    };
    // Merge several per-player timing sources into one row per player, keyed by
    // player_id (preserving the first series' metadata).
    const mergeByPlayer = (collect) => {
      const byPlayer = new Map();
      const ensure = (ps) => {
        const pid = ps?.player_id;
        if (!byPlayer.has(pid)) byPlayer.set(pid, { ...ps, points: [] });
        return byPlayer.get(pid);
      };
      collect(ensure);
      return sortRows([...byPlayer.values()].map((row) => withRace(row, row.points)));
    };

    // "Expansion & Gas": both economic timings overlaid on one row per player,
    // each shown with its own building/gas icon.
    if (mainTimingCategoryConfig.source === 'expansion_gas') {
      return mergeByPlayer((ensure) => {
        const add = (series, isGas) => (Array.isArray(series) ? series : []).forEach((ps) => {
          const row = ensure(ps);
          const playerRace = String(mainPlayersById.get(ps?.player_id)?.race || '').trim();
          (ps?.points || []).forEach((point) => {
            const second = Number(point?.second);
            if (!Number.isFinite(second)) return;
            const order = Number(point?.order) || 0;
            row.points.push({
              ...point,
              second,
              order,
              label: String(point?.label || '').trim(),
              display_label: isGas ? t('timings.gasNumber', { n: order || 1 }) : t('timings.expansionNumber', { n: order || 1 }),
              category: isGas ? 'gas' : 'expansion',
              category_label: isGas ? t('timings.gas') : t('timings.expansion'),
              marker_image: isGas ? getGasMarkerIconForRace(playerRace) : getExpansionMarkerIconForRace(playerRace),
              marker_label: isGas ? t('timings.gasStructure') : t('timings.expansion'),
              is_repeatable: false,
              max_level: 1,
            });
          });
        });
        add(timings.expansion, false);
        add(timings.gas, true);
      });
    }

    // "Upgrades & Tech": merge the selected non-HP sub-categories per player so
    // they overlay on one chart, each tinted by its sub-category colour and
    // labelled with the upgrade/tech name.
    if (mainTimingCategoryConfig.source === 'research') {
      const selected = new Set(mainResearchSubcategories);
      return mergeByPlayer((ensure) => {
        if ([...selected].some((id) => id !== 'tech')) {
          (Array.isArray(timings.upgrades) ? timings.upgrades : []).forEach((ps) => {
            const row = ensure(ps);
            (ps?.points || []).forEach((point) => {
              const second = Number(point?.second);
              if (!Number.isFinite(second)) return;
              const rawLabel = String(point?.label || '').trim();
              const cat = upgradeCategoryForName(rawLabel);
              if (!selected.has(cat)) return;
              const subcfg = RESEARCH_SUBCATEGORY_BY_ID[cat];
              row.points.push({
                ...point,
                second,
                order: Number(point?.order) || 0,
                label: rawLabel,
                display_label: normalizeTimingDisplayLabel(rawLabel),
                category: cat,
                category_label: subcfg ? t(subcfg.labelKey) : t('timings.category.upgrade'),
                overlay_color: subcfg?.color || '',
                is_repeatable: false,
                max_level: 1,
              });
            });
          });
        }
        if (selected.has('tech')) {
          const techCfg = RESEARCH_SUBCATEGORY_BY_ID.tech;
          (Array.isArray(timings.tech) ? timings.tech : []).forEach((ps) => {
            const row = ensure(ps);
            (ps?.points || []).forEach((point) => {
              const second = Number(point?.second);
              if (!Number.isFinite(second)) return;
              const rawLabel = String(point?.label || '').trim();
              row.points.push({
                ...point,
                second,
                order: Number(point?.order) || 0,
                label: rawLabel,
                display_label: normalizeTimingDisplayLabel(rawLabel),
                category: 'tech',
                category_label: t(techCfg.labelKey),
                overlay_color: techCfg.color,
                is_repeatable: false,
                max_level: 1,
              });
            });
          });
        }
      });
    }

    // "HP Upgrades": every HP-tier point per player (filtered/labelled per race
    // downstream by mainHpTimingByRace).
    const rows = (Array.isArray(timings.upgrades) ? timings.upgrades : []).map((playerSeries) => {
      const points = (Array.isArray(playerSeries?.points) ? playerSeries.points : [])
        .map((point) => {
          const second = Number(point?.second);
          if (!Number.isFinite(second)) return null;
          const rawLabel = String(point?.label || '').trim();
          if (upgradeCategoryForName(rawLabel) !== 'hp_upgrades') return null;
          const order = Number(point?.order) || 0;
          return {
            ...point,
            second,
            order,
            label: rawLabel,
            display_label: rawLabel,
            category: 'hp_upgrades',
            category_label: t('timings.category.hpUpgrades'),
            is_repeatable: true,
            max_level: 3,
          };
        })
        .filter(Boolean);
      return withRace(playerSeries, points);
    });
    return sortRows(rows);
  }, [mainGame?.timings, mainTimingCategoryConfig, mainResearchSubcategories, mainPlayersById, t]);
  const mainTimingColorBy = isResearchTiming ? 'category' : 'player';
  const mainTimingNotice = isExpansionGasTiming
    ? t('timings.expansionNotice')
    : '';
  const mainHpTimingByRace = useMemo(() => {
    if (!isHpTiming) return [];
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
  }, [isHpTiming, mainTimingSeries, mainHpUpgradeFilters, t]);
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
  // Scale the Games bar to the largest value on THIS page, so the column reads
  // as a quantity. That is what the rank palette was gesturing at and could not
  // deliver: 15 categorical hues cannot be ordered by eye.
  const mainPlayersMaxGames = mainPlayers.reduce(
    (max, player) => Math.max(max, Number(player.games_played || 0)),
    0,
  );
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
        average_apm: Number(player?.[VIEWPORT_SWITCH_RATE_FIELDS.playerField] || 0),
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
        average_apm: Number(player?.[VIEWPORT_SWITCH_RATE_FIELDS.gameField] || 0),
        games_played: 1,
        viewport_switch_rate: Number(player?.viewport_switch_rate || 0),
      }))
      .filter((player) => player.player_name && Number.isFinite(player.average_apm) && player.average_apm >= 0);
    return buildHistogramSummaryFromPlayers(rows);
  }, [mainGame]);
  const toggleShowFeaturedPros = () => {
    setShowFeaturedPros((prev) => {
      saveShowFeaturedPros(!prev);
      return !prev;
    });
  };
  const viewportText = useMemo(() => viewportSwitchRateText(t), [t]);
  const playersOmnibarAxes = useMemo(() => buildPlayersOmnibarAxes(t), [t]);
  const playersOmnibarStateLabels = useMemo(() => buildPlayersOmnibarStateLabels(t), [t]);
  const featuredApmOverlayPoints = useMemo(
    () => featuredOverlayPoints(mainPlayersApmHistogram?.featured_players, t('appChart.averageApm')),
    [mainPlayersApmHistogram, t],
  );
  const featuredCadenceOverlayPoints = useMemo(
    () => featuredOverlayPoints(mainPlayersCadenceHistogram?.featured_players, t('appChart.cadenceScore')),
    [mainPlayersCadenceHistogram, t],
  );
  const featuredViewportOverlayPoints = useMemo(
    () => featuredOverlayPoints(mainPlayersViewportHistogram?.featured_players, viewportText.title),
    [mainPlayersViewportHistogram, viewportText],
  );
  const renderFeaturedToggle = (count) => (
    <label className="workflow-featured-toggle" title={t('players.featuredToggleTip')}>
      <input type="checkbox" checked={showFeaturedPros} onChange={toggleShowFeaturedPros} />
      <span>{count ? t('players.showFeaturedCount', { count }) : t('players.showFeatured')}</span>
    </label>
  );
  const mainPlayersSortIndicator = (sortBy) => {
    if (mainPlayersSortBy !== sortBy) return '';
    return mainPlayersSortDir === 'asc' ? '↑' : '↓';
  };

  if (stopped) {
    return (
      <div className="app app-stopped">
        <div className="app-stopped-card">
          <div className="app-stopped-icon">⏻</div>
          <h1>{t('stopped.title')}</h1>
          <p>{t('stopped.body')}</p>
          <p className="app-stopped-hint">{t('stopped.hint')}</p>
        </div>
      </div>
    );
  }

  return (
    <CountryFlagOverrideContext.Provider value={countryCodeOverrides}>
    <div className="app">
      <div className={`dashboard-container${activeView === 'games' ? ' dashboard-container--full' : ''}`}>
        <div className="workflow-nav workflow-nav-app">
          <div className="workflow-nav-group">
            <button type="button" className={`btn-manage ${activeView === 'games' ? 'workflow-nav-active' : ''}`} onClick={() => navigateMainView('games')}>{t('nav.games')}</button>
            <button type="button" className={`btn-manage ${activeView === 'players' ? 'workflow-nav-active' : ''}`} onClick={() => navigateMainView('players')}>{t('nav.players')}</button>
            {gamingSessionEnabled && gamingSession?.active ? (
              <button
                type="button"
                className={`btn-manage ${activeView === 'session' ? 'workflow-nav-active' : ''}`}
                onClick={() => navigateMainView('session')}
              >
                {t('nav.session')}
              </button>
            ) : null}
          </div>
          <div className="workflow-nav-group">
            {updateAvailable && updateTier === 'loud' && !loudUpdateDismissed ? (
              <span className="workflow-nav-update-nudge">
                {updateApplied ? (
                  <button
                    type="button"
                    className="workflow-nav-update-available tip-below"
                    data-tip={t('update.installedTip')}
                    onClick={() => window.location.reload()}
                  >
                    {t('update.updatedRefresh', { version: updateLatest })}
                  </button>
                ) : selfUpdateSupported ? (
                  <button
                    type="button"
                    className="workflow-nav-update-available tip-below"
                    data-tip={updateError || t('update.fromTo', { from: currentVersion, to: updateLatest })}
                    disabled={updateApplying}
                    onClick={handleApplyUpdate}
                  >
                    {updateApplying ? t('update.updating') : t('update.updateTo', { version: updateLatest })}
                  </button>
                ) : updateManagerCommand ? (
                  <ManagedUpdateHint
                    className="workflow-nav-update-available managed-update-hint--nav"
                    latestVersion={updateLatest}
                    command={updateManagerCommand}
                    releaseUrl={updateReleaseUrl}
                  />
                ) : (
                  <a
                    href={updateReleaseUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="workflow-nav-update-available tip-below"
                    data-tip={updateUnsupportedTip}
                  >
                    {t('update.available')}
                  </a>
                )}
                {!updateApplied ? (
                  <button
                    type="button"
                    className="footer-update-dismiss workflow-nav-update-dismiss"
                    aria-label={t('update.dismiss')}
                    onClick={() => setLoudUpdateDismissed(true)}
                  >
                    ×
                  </button>
                ) : null}
              </span>
            ) : null}
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
              {t('nav.settings')}
              {!showGlobalReplayFilter && isLibraryLoading(libraryStatus) ? (
                <span className="ingest-running-badge tip-below" data-tip={t('library.readingFolderTip')}>
                  {formatLoadingShortWith(t)}
                </span>
              ) : null}
            </button>
          </div>
          <div className="workflow-nav-group workflow-nav-group-right">
            <button
              type="button"
              className={`bnet-pill bnet-pill--${bnetDisabled ? 'disabled' : bnetState} tip-below`}
              data-tip={
                (bnetDisabled ? t('bnet.tip.disabled')
                : bnetState === 'reconnecting' ? t('bnet.tip.scanning')
                : bnetState === 'connected' ? t('bnet.tip.connected')
                : bnetState === 'offline' ? t('bnet.tip.offline')
                : t('bnet.tip.notDetected')) + bnetTipSuffix
              }
              onClick={handleBnetToggle}
              disabled={bnetState === 'reconnecting'}
            >
              <span className="bnet-pill-dot" />
              {bnetDisabled ? t('bnet.pill.off')
                : bnetState === 'reconnecting' ? t('bnet.pill.scanning')
                : bnetState === 'connected' ? 'SC:R'
                : bnetState === 'offline' ? t('bnet.pill.offline')
                : 'SC:R'}
            </button>
            <button
              type="button"
              onClick={handleQuit}
              className="workflow-nav-text-action workflow-nav-quit tip-below"
              data-tip={t('quit.tip')}
            >
              {t('quit.button')}
            </button>
          </div>
        </div>

        {error && (
          <div className="error-message" role="alert">
            <span className="error-message-text">{error}</span>
            <button
              type="button"
              className="error-message-dismiss"
              aria-label={t('errors.dismiss')}
              title={t('common.dismiss')}
              onClick={() => setError(null)}
            >
              ×
            </button>
          </div>
        )}

        {activeView === 'games' && (
          <div className="workflow-panel">
            <FilterOmnibar
              filterOptions={mainGamesFilterOptions}
              selected={mainGamesFilters}
              total={mainGamesTotal}
              onToggle={toggleMainGameMultiFilter}
              onClear={clearMainGamesFilters}
              loading={libraryLoading}
            />
            {mainGamesLoading ? (
              <div className="loading">{t('games.loading')}</div>
            ) : (
              <>
                {renderGamesListTable({
                  games: mainGames,
                  tableRef: mainGamesTableRef,
                  selectedId: selectedReplayId,
                })}
                <div className="workflow-pagination-row workflow-pagination-row-centered">
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={mainGamesPage <= 1 || mainGamesLoading}
                    onClick={() => setMainGamesPage((prev) => Math.max(1, prev - 1))}
                    aria-label={t('pagination.previous')}
                  >
                    {'<'}
                  </button>
                  <span>{t('pagination.range', { from: mainGamesFrom, to: mainGamesTo, total: mainGamesTotal })}</span>
                  <button
                    type="button"
                    className="btn-switch"
                    disabled={mainGamesPage >= mainGamesTotalPages || mainGamesLoading}
                    onClick={() => setMainGamesPage((prev) => Math.min(mainGamesTotalPages, prev + 1))}
                    aria-label={t('pagination.next')}
                  >
                    {'>'}
                  </button>
                </div>
              </>
            )}
          </div>
        )}

        {activeView === 'session' && (
          <GamingSessionPanel
            session={gamingSession}
            loading={gamingSessionLoading}
            error={gamingSessionError}
            renderName={(opponent) => (
              <button
                type="button"
                className="workflow-player-name-link workflow-name-with-flag"
                onClick={() => openMainPlayer(opponent.player_key)}
                title={t('player.analyze')}
              >
                <CountryFlag code={opponent.country_code} playerKey={opponent.player_key} />
                {opponent.player_name}
              </button>
            )}
          >
            {renderGamesListTable({ games: gamingSession?.games || [] })}
          </GamingSessionPanel>
        )}

        {activeView === 'players' && (
          <div className="workflow-panel">
            <div className="workflow-players-tab-stack">
              <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label={t('players.sectionsAria')}>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'summary'}
                  className={`workflow-production-tab ${mainPlayersTab === 'summary' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('summary')}
                >
                  {t('tabs.summary')}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'apm-histogram'}
                  className={`workflow-production-tab ${mainPlayersTab === 'apm-histogram' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('apm-histogram')}
                >
                  {t('players.tabs.apmHistogram')}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'unit-production-cadence'}
                  className={`workflow-production-tab ${mainPlayersTab === 'unit-production-cadence' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('unit-production-cadence')}
                >
                  {t('players.tabs.cadence')}
                </button>
                <button
                  type="button"
                  role="tab"
                  aria-selected={mainPlayersTab === 'viewport-multitasking'}
                  className={`workflow-production-tab ${mainPlayersTab === 'viewport-multitasking' ? 'workflow-production-tab-active' : ''}`}
                  onClick={() => setMainPlayersTab('viewport-multitasking')}
                >
                  {t('players.tabs.viewport')}
                </button>
              </div>
              {mainPlayersTab === 'unit-production-cadence' ? (
                <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                  {t('skillProxies.cadence.info')}
                </div>
              ) : null}
              {mainPlayersTab === 'viewport-multitasking' ? (
                <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                  {t('skillProxies.viewport.info')}
                </div>
              ) : null}
            </div>

            {mainPlayersTab === 'summary' ? (
              <>
                <FilterOmnibar
                  filterOptions={{ ...mainPlayersFilterOptions, min_games: [{ key: '5plus', label: t('players.filter.fivePlusGames') }] }}
                  axes={playersOmnibarAxes}
                  stateLabels={playersOmnibarStateLabels}
                  stateOrder={PLAYERS_OMNIBAR_STATE_ORDER}
                  noun="players"
                  loading={libraryLoading}
                  selected={{
                    lastPlayed: mainPlayersFilters.lastPlayed || [],
                    onlyFivePlus: mainPlayersFilters.onlyFivePlus ? ['5plus'] : [],
                  }}
                  total={mainPlayersTotal}
                  textFilter={{
                    value: mainPlayersFilters.name,
                    onChange: (value) => setMainPlayersSingleFilter('name', value),
                    placeholder: t('players.filterPlaceholder'),
                  }}
                  onToggle={(state, key) => {
                    if (state === 'onlyFivePlus') {
                      setMainPlayersSingleFilter('onlyFivePlus', !mainPlayersFilters.onlyFivePlus);
                    } else {
                      toggleMainPlayersMultiFilter(state, key);
                    }
                  }}
                  onClear={clearMainPlayersFilters}
                />
                {mainPlayersLoading ? (
                  <div className="loading">{t('players.loading')}</div>
                ) : (
                  <>
                    {mainPlayersFeatured.length > 0 ? (() => {
                      // Seventy built-in rows would bury the user's own players, so the
                      // group opens with one row and expands on demand. A name filter
                      // already narrows it, so filtered results always show in full.
                      const filtering = String(mainPlayersFilters.name || '').trim() !== '';
                      const collapsedCount = 8;
                      const showAll = filtering || mainPlayersFeaturedExpanded || mainPlayersFeatured.length <= collapsedCount;
                      const visible = showAll ? mainPlayersFeatured : mainPlayersFeatured.slice(0, collapsedCount);
                      return (
                      <div className="workflow-featured-players">
                        <div className="workflow-featured-grid">
                          {visible.map((pro) => (
                            <div
                              key={pro.player_key}
                              className={`workflow-featured-row ${selectedPlayerKey === pro.player_key ? 'workflow-selected-row' : ''}`}
                              role="button"
                              tabIndex={0}
                              onClick={() => openMainPlayer(pro.player_key)}
                              onKeyDown={(e) => { if (e.key === 'Enter') openMainPlayer(pro.player_key); }}
                            >
                              <ProPhoto url={pro.photo_url} name={pro.player_name} />
                              <span className="workflow-featured-name">
                                <span><CountryFlag code={pro.country_code} />{pro.player_name}</span>
                              </span>
                            </div>
                          ))}
                          {!filtering && mainPlayersFeatured.length > collapsedCount ? (
                            <button type="button" className="workflow-link-btn workflow-featured-expand" onClick={() => setMainPlayersFeaturedExpanded((prev) => !prev)}>
                              {mainPlayersFeaturedExpanded ? t('players.showFewer') : t('players.showAll', { count: mainPlayersFeatured.length })}
                            </button>
                          ) : null}
                        </div>
                      </div>
                      );
                    })() : null}
                    <table className="data-table workflow-table workflow-players-list-table">
                      <thead>
                        <tr>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('name')}>{t('common.name')} {mainPlayersSortIndicator('name')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('race')}>{t('players.table.race')} {mainPlayersSortIndicator('race')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('games')}>{t('players.table.games')} {mainPlayersSortIndicator('games')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('apm')}>{t('players.table.avgApm')} {mainPlayersSortIndicator('apm')}</th>
                          <th className="workflow-sortable" onClick={() => setMainPlayersSort('last_played')}>{t('common.lastPlayed')} {mainPlayersSortIndicator('last_played')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {mainPlayers.map((player) => (
                          <tr key={player.player_key} className={selectedPlayerKey === player.player_key ? 'workflow-selected-row' : ''} onClick={() => openMainPlayer(player.player_key)}>
                            <td><span className="workflow-name-with-flag"><CountryFlag code={player.country_code} playerKey={player.player_key} /><PlayerDisplayName name={player.player_name} /></span></td>
                            <td>{raceLabel(player.race)}</td>
                            <td>
                              <span className="rd-qty">
                                <span className="rd-qty-num">{player.games_played}</span>
                                <span
                                  className="rd-qty-bar"
                                  style={{ width: `${mainPlayersMaxGames ? Math.max(2, (Number(player.games_played || 0) / mainPlayersMaxGames) * 100) : 0}%` }}
                                />
                              </span>
                            </td>
                            <td>{Number(player.average_apm || 0).toFixed(1)}</td>
                            <td>{formatDaysAgoCompact(player.last_played_days_ago)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    <div className="workflow-pagination-row workflow-pagination-row-centered">
                      <button
                        type="button"
                        className="btn-switch"
                        disabled={mainPlayersPage <= 1 || mainPlayersLoading}
                        onClick={() => setMainPlayersPage((prev) => Math.max(1, prev - 1))}
                        aria-label={t('pagination.previous')}
                  >
                        {'<'}
                      </button>
                      <span>{t('pagination.range', { from: mainPlayersFrom, to: mainPlayersTo, total: mainPlayersTotal })}</span>
                      <button
                        type="button"
                        className="btn-switch"
                        disabled={mainPlayersPage >= mainPlayersTotalPages || mainPlayersLoading}
                        onClick={() => setMainPlayersPage((prev) => Math.min(mainPlayersTotalPages, prev + 1))}
                        aria-label={t('pagination.next')}
                  >
                        {'>'}
                      </button>
                    </div>
                  </>
                )}
              </>
            ) : mainPlayersTab === 'apm-histogram' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {mainPlayersApmHistogramLoading ? <div className="chart-empty">{t('players.apm.loading')}</div> : null}
                {!mainPlayersApmHistogramLoading && mainPlayersApmHistogramError ? <div className="chart-empty">{mainPlayersApmHistogramError}</div> : null}
                {!mainPlayersApmHistogramLoading && !mainPlayersApmHistogramError && mainPlayersApmProcessed.points.length === 0 ? (
                  <div className="chart-empty">{libraryLoadingCopy || t('players.apm.notEnough')}</div>
                ) : null}
                {!mainPlayersApmHistogramLoading && !mainPlayersApmHistogramError && mainPlayersApmProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersApmProcessed.bins,
                        x_axis_label: t('appChart.averageApm'),
                        y_axis_label: t('appChart.density'),
                        mean: mainPlayersApmProcessed.mean,
                        stddev: mainPlayersApmProcessed.stddev,
                        chart_height: 620,
                        overlay_points: [
                          ...mainPlayersApmProcessed.points.map((player) => ({
                            value: Number(player.average_apm || 0),
                            label: String(player.player_name || ''),
                            player_key: String(player.player_key || ''),
                            games_played: Number(player.games_played || 0),
                          })),
                          ...(showFeaturedPros ? featuredApmOverlayPoints : []),
                        ],
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`${t('players.apm.population', { players: Number(mainPlayersApmProcessed.playersIncluded) || 0, minGames: Math.max(5, Number(mainPlayersApmMinGames) || 5), mean: Number(mainPlayersApmProcessed.mean || 0).toFixed(1), stddev: Number(mainPlayersApmProcessed.stddev || 0).toFixed(1) })}${showFeaturedPros && featuredApmOverlayPoints.length ? t('players.featuredNote') : ''}`}
                    </div>
                    {featuredApmOverlayPoints.length > 0 ? <div className="workflow-featured-toggle-row">{renderFeaturedToggle(featuredApmOverlayPoints.length)}</div> : null}
                  </div>
                ) : null}
              </div>
            ) : mainPlayersTab === 'unit-production-cadence' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {mainPlayersCadenceHistogramLoading ? <div className="chart-empty">{t('players.cadence.loading')}</div> : null}
                {!mainPlayersCadenceHistogramLoading && mainPlayersCadenceHistogramError ? <div className="chart-empty">{mainPlayersCadenceHistogramError}</div> : null}
                {!mainPlayersCadenceHistogramLoading && !mainPlayersCadenceHistogramError && mainPlayersCadenceProcessed.points.length === 0 ? (
                  <div className="chart-empty">{libraryLoadingCopy || t('players.cadence.notEnough')}</div>
                ) : null}
                {!mainPlayersCadenceHistogramLoading && !mainPlayersCadenceHistogramError && mainPlayersCadenceProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersCadenceProcessed.bins,
                        x_axis_label: t('appChart.averageCadence'),
                        y_axis_label: t('appChart.density'),
                        overlay_value_label: t('appChart.overlay.cadence'),
                        overlay_count_label: t('appChart.overlay.games'),
                        mean: mainPlayersCadenceProcessed.mean,
                        stddev: mainPlayersCadenceProcessed.stddev,
                        chart_height: 620,
                        overlay_points: [
                          ...mainPlayersCadenceProcessed.points.map((player) => ({
                            value: Number(player.average_apm || 0),
                            label: String(player.player_name || ''),
                            player_key: String(player.player_key || ''),
                            games_played: Number(player.games_played || 0),
                            tooltip_lines: [
                              `${String(player.player_name || '')}`,
                              t('appChart.tooltip.cadenceScore', { value: Number(player.average_apm || 0).toFixed(3) }),
                              t('appChart.tooltip.ratePerMinute', { value: Number(player.average_rate_per_min || 0).toFixed(2) }),
                              t('appChart.tooltip.gapCv', { value: Number(player.average_cv_gap || 0).toFixed(2) }),
                              t('appChart.tooltip.burstiness', { value: Number(player.average_burstiness || 0).toFixed(2) }),
                              t('appChart.tooltip.idleGapRatio', { value: (Number(player.average_idle20_ratio || 0) * 100).toFixed(1) }),
                              t('appChart.tooltip.gamesUsed', { count: Number(player.games_played || 0) }),
                            ],
                          })),
                          ...(showFeaturedPros ? featuredCadenceOverlayPoints : []),
                        ],
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`${t('players.cadence.population', { players: Number(mainPlayersCadenceProcessed.playersIncluded) || 0, minGames: Math.max(4, Number(mainPlayersCadenceMinGames) || 4), mean: Number(mainPlayersCadenceProcessed.mean || 0).toFixed(3), stddev: Number(mainPlayersCadenceProcessed.stddev || 0).toFixed(3) })}${showFeaturedPros && featuredCadenceOverlayPoints.length ? t('players.featuredNote') : ''}`}
                    </div>
                    {featuredCadenceOverlayPoints.length > 0 ? <div className="workflow-featured-toggle-row">{renderFeaturedToggle(featuredCadenceOverlayPoints.length)}</div> : null}
                  </div>
                ) : null}
              </div>
            ) : mainPlayersTab === 'viewport-multitasking' ? (
              <div className="workflow-card workflow-card-fingerprints">
                {mainPlayersViewportHistogramLoading ? <div className="chart-empty">{t('players.viewport.loading')}</div> : null}
                {!mainPlayersViewportHistogramLoading && mainPlayersViewportHistogramError ? <div className="chart-empty">{mainPlayersViewportHistogramError}</div> : null}
                {!mainPlayersViewportHistogramLoading && !mainPlayersViewportHistogramError && mainPlayersViewportProcessed.points.length === 0 ? (
                  <div className="chart-empty">{libraryLoadingCopy || t('players.viewport.notEnough')}</div>
                ) : null}
                {!mainPlayersViewportHistogramLoading && !mainPlayersViewportHistogramError && mainPlayersViewportProcessed.points.length > 0 ? (
                  <div className="workflow-insight-chart workflow-insight-chart-tall">
                    <Histogram
                      data={[]}
                      config={{
                        style: 'monobell_relax',
                        precomputed_bins: mainPlayersViewportProcessed.bins,
                        x_axis_label: viewportText.axisLabel,
                        y_axis_label: t('appChart.density'),
                        overlay_value_label: viewportText.overlayValueLabel,
                        overlay_count_label: t('appChart.overlay.games'),
                        mean: mainPlayersViewportProcessed.mean,
                        stddev: mainPlayersViewportProcessed.stddev,
                        chart_height: 620,
                        overlay_points: [
                          ...mainPlayersViewportProcessed.points.map((player) => ({
                            value: Number(player.average_apm || 0),
                            label: String(player.player_name || ''),
                            player_key: String(player.player_key || ''),
                            games_played: Number(player.games_played || 0),
                            tooltip_lines: [
                              `${String(player.player_name || '')}`,
                              t('appChart.tooltip.labelValue', { label: viewportText.title, value: viewportText.valueFormatter(player.average_apm) }),
                              t('appChart.tooltip.gamesUsed', { count: Number(player.games_played || 0) }),
                            ],
                          })),
                          ...(showFeaturedPros ? featuredViewportOverlayPoints : []),
                        ],
                        on_overlay_point_click: openMainPlayer,
                      }}
                    />
                    <div className="workflow-subtle-note">
                      {`${t('players.viewport.population', { players: Number(mainPlayersViewportProcessed.playersIncluded) || 0, minGames: Math.max(4, Number(mainPlayersViewportMinGames) || 4), mean: viewportText.summaryFormatter(mainPlayersViewportProcessed.mean), stddev: viewportText.summaryFormatter(mainPlayersViewportProcessed.stddev) })}${showFeaturedPros && featuredViewportOverlayPoints.length ? t('players.featuredNote') : ''}`}
                    </div>
                    {featuredViewportOverlayPoints.length > 0 ? <div className="workflow-featured-toggle-row">{renderFeaturedToggle(featuredViewportOverlayPoints.length)}</div> : null}
                  </div>
                ) : null}
              </div>
            ) : null}
          </div>
        )}

        {activeView === 'game' && (
          <div className="workflow-panel">
            {mainGameDetailLoading ? (
              <div className="loading">{t('game.loading')}</div>
            ) : mainGame ? (
              <>
                <div className="workflow-title-row workflow-title-row--solo">
                  <h2 className="workflow-game-players-heading">{renderGameTitlePlayers(mainGame)}</h2>
                </div>
                <div className="workflow-meta workflow-meta--game-header">
                  <span>{formatRelativeReplayDate(mainGame.replay_date)}</span>
                  <span className="workflow-meta-sep" aria-hidden="true">·</span>
                  <span>{formatMapNameWithKind(mainGame.map_name, mainGame.map_kind)}</span>
                  <span className="workflow-meta-sep" aria-hidden="true">·</span>
                  <span>{formatDuration(mainGame.duration_seconds)}</span>
                  {mainGame.file_path ? (
                    <>
                      <span className="workflow-meta-sep" aria-hidden="true">·</span>
                      <code className="workflow-meta-filepath-text" title={mainGame.file_path}>
                        {mainGame.file_path.replace(/\\/g, '/').split('/').pop()}
                      </code>
                    </>
                  ) : null}
                  {mainGame.file_path ? (
                    <button
                      type="button"
                      className="btn-switch workflow-meta-filepath-copy"
                      data-tip={t('game.copyPathTip')}
                      onClick={() => {
                        if (navigator.clipboard && navigator.clipboard.writeText) {
                          navigator.clipboard.writeText(mainGame.file_path);
                        }
                      }}
                    >
                      {t('game.copyPath')}
                    </button>
                  ) : null}
                  <button
                    type="button"
                    className="btn-switch btn-switch-see-replay workflow-meta-stage-btn"
                    disabled={mainGameSeeLoading}
                    data-tip={t('game.stage.tip')}
                    onClick={copyMainGameToWatchMe}
                  >
                    {mainGameSeeLoading ? t('game.stage.copying') : t('game.stage.button')}
                  </button>
                </div>
                <div className="workflow-game-tab-stack">
                  <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label={t('game.sectionsAria')}>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'summary'}
                      className={`workflow-production-tab ${mainGameTab === 'summary' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('summary')}
                    >
                      {t('tabs.summary')}
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'events'}
                      className={`workflow-production-tab ${mainGameTab === 'events' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('events')}
                    >
                      {t('game.tabs.events')}
                    </button>
                    {Array.isArray(mainGame?.build_orders) && mainGame.build_orders.length > 0 ? (
                      <button
                        type="button"
                        role="tab"
                        aria-selected={mainGameTab === 'build-orders'}
                        className={`workflow-production-tab ${mainGameTab === 'build-orders' ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => setMainGameTab('build-orders')}
                      >
                        {t('game.tabs.buildOrders')}
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
                        {t('game.tabs.mutaTiming')}
                      </button>
                    ) : null}
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'units'}
                      className={`workflow-production-tab ${mainGameTab === 'units' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('units')}
                    >
                      {t('game.tabs.units')}
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'timings'}
                      className={`workflow-production-tab ${mainGameTab === 'timings' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('timings')}
                    >
                      {t('game.tabs.timings')}
                    </button>
                    {Array.isArray(mainGame?.alliance_timeline) && mainGame.alliance_timeline.length > 0 ? (
                      <button
                        type="button"
                        role="tab"
                        aria-selected={mainGameTab === 'alliances'}
                        className={`workflow-production-tab ${mainGameTab === 'alliances' ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => setMainGameTab('alliances')}
                      >
                        {t('game.tabs.alliances')}{mainGame?.team_stacking ? ' 😈' : ''}
                      </button>
                    ) : null}
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'hotkeys'}
                      className={`workflow-production-tab ${mainGameTab === 'hotkeys' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('hotkeys')}
                    >
                      {t('tabs.hotkeys')}
                    </button>
                    <button
                      type="button"
                      role="tab"
                      aria-selected={mainGameTab === 'supply-timeline'}
                      className={`workflow-production-tab ${mainGameTab === 'supply-timeline' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => setMainGameTab('supply-timeline')}
                    >
                      {t('game.tabs.supply')}
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
                      {t('tabs.skillProxies')}
                    </button>
                  </div>
                  {isMainGameSkillProxyTab(mainGameTab) ? (
                    <div className="workflow-skill-proxy-subnav">
                      <div className="workflow-production-tabs workflow-skill-proxy-tabs" role="tablist" aria-label={t('skillProxies.viewsAria')}>
                        <button
                          type="button"
                          role="tab"
                          aria-selected={mainGameTab === 'first-unit-efficiency'}
                          className={`workflow-production-tab ${mainGameTab === 'first-unit-efficiency' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainGameTab('first-unit-efficiency')}
                        >
                          {t('skillProxies.tabs.firstUnit')}
                        </button>
                        <button
                          type="button"
                          role="tab"
                          aria-selected={mainGameTab === 'unit-production-cadence'}
                          className={`workflow-production-tab ${mainGameTab === 'unit-production-cadence' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainGameTab('unit-production-cadence')}
                        >
                          {t('skillProxies.tabs.cadence')}
                        </button>
                        <button
                          type="button"
                          role="tab"
                          aria-selected={mainGameTab === 'viewport-multitasking'}
                          className={`workflow-production-tab ${mainGameTab === 'viewport-multitasking' ? 'workflow-production-tab-active' : ''}`}
                          onClick={() => setMainGameTab('viewport-multitasking')}
                        >
                          {t('skillProxies.tabs.viewport')}
                        </button>
                      </div>
                      {mainGameTab === 'unit-production-cadence' ? (
                        <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                          {t('skillProxies.cadence.info')}
                        </div>
                      ) : null}
                      {mainGameTab === 'viewport-multitasking' ? (
                        <div className="workflow-section-info workflow-skill-proxy-tab-info" role="note">
                          {t('skillProxies.viewport.info')}
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
                            title={t('game.summary.openEvents')}
                          >
                            <div className="workflow-map-thumb-btn-inner">
                              {renderSummaryMapStack({
                                legendItems: selectedMainGameLegend,
                                showLegend: false,
                                imageUrl: mainMapVisualThumbURL,
                                mapAlt: t('game.summary.mapAlt', { map: mainGame.map_name }),
                                bounds: mainEventMapBounds,
                                startPolygons: summaryMapStartPolygons,
                              })}
                              <span className="workflow-map-thumb-btn-hover-label" aria-hidden="true">{t('game.tabs.events')}</span>
                            </div>
                          </button>
                        ) : (
                          <div className="workflow-map-summary-fallback">
                            {t('game.summary.mapUnavailable')}
                            {mainMapVisual?.resolution_note ? ` (${mainMapVisual.resolution_note})` : ''}
                          </div>
                        )}
                      </div>
                      <div className="workflow-summary-features-col">
                        {mainGameFeaturingPillsList.length > 0 ? (
                          <>
                            <div className="workflow-summary-features-title">{t('common.featuring')}</div>
                            <div className="workflow-pattern-pills">
                              {mainGameFeaturingPillsList.map((pill) => renderFeaturingPill(pill, 'summary-game'))}
                            </div>
                          </>
                        ) : (
                          <div className="workflow-subtle-note">{t('game.summary.noHighlights')}</div>
                        )}
                        {/* Replay-aggregate attacker-composition bars (early/mid/late),
                            computed at display time by summing per-player counts in
                            mainGame.unit_composition_markers. Spellcasts (distinct
                            casts, not headcount) sit in their own block below. */}
                        {Array.isArray(mainGame?.unit_composition_markers) && mainGame.unit_composition_markers.length > 0 ? (
                          (() => {
                            const aggregatePhases = computeReplayAggregatePhases(mainGame.unit_composition_markers);
                            const aggregateSpells = collectPlayerSpells(aggregatePhases);
                            return (
                              <div className="workflow-summary-composition">
                                <div className="workflow-summary-features-title workflow-summary-composition-title">{t('game.summary.unitComposition')}</div>
                                <CompositionZonesHeader />
                                <CompositionZones phases={aggregatePhases} />
                                {aggregateSpells.length > 0 ? (
                                  <div className="workflow-summary-spellcasts">
                                    <div className="workflow-summary-features-title workflow-summary-spellcasts-title">{t('game.summary.spellcasts')}</div>
                                    <div className="workflow-pattern-pills">
                                      <SpellcastsChips spells={aggregateSpells} />
                                    </div>
                                  </div>
                                ) : null}
                              </div>
                            );
                          })()
                        ) : null}
                      </div>
                    </div>
                    <div className="workflow-player-table" style={{ '--workflow-player-name-width': `${mainPlayerNameWidthCh}ch` }}>
                      <div className="wpt-head">{t('common.name')}</div>
                      <div className="wpt-head">APM</div>
                      <div className="wpt-head">{t('common.featuring')}</div>
                      <div className="wpt-head wpt-head-comp">
                        <span>{t('game.summary.unitComposition')}</span>
                        <CompositionZonesHeader slim />
                      </div>
                      {(mainGame.players || []).map((player) => {
                        const raceIcon = getRaceIcon(player.race);
                        const gameSummaryParts = playerGameSummarySignalParts(player, mainGame?.game_events);
                        const trustGameEventsForDrops = Array.isArray(mainGame?.game_events) && mainGame.game_events.length > 0;
                        const patterns = filterSummaryPillPatterns(player.detected_patterns, trustGameEventsForDrops);
                        // BO/opener pill always leads, then game-event signal pills
                        // (Drop / Bunker rush / Proxy …), then the remaining markers.
                        const boPatterns = patterns.filter((p) => isOpenerEventType(p?.event_type));
                        const restPatterns = patterns.filter((p) => !isOpenerEventType(p?.event_type));
                        const playerPhases = Array.isArray(mainGame?.unit_composition_markers)
                          ? mainGame.unit_composition_markers.filter((m) => m.player_id === player.player_id)
                          : [];
                        return (
                          <React.Fragment key={player.player_id}>
                            <div className="wpt-cell wpt-name" style={{ borderLeftColor: getTeamColor(player.team) }}>
                              <span className="wpt-glyphs">
                                <span className="wpt-glyph wpt-glyph-race">
                                  {raceIcon ? <img src={raceIcon} alt={player.race || t('race.alt')} className="unit-icon-inline workflow-summary-race-icon" /> : null}
                                </span>
                                <span className="wpt-glyph wpt-glyph-flag">
                                  <CountryFlag code={player.country_code} playerKey={player.player_key} />
                                </span>
                                <span className="wpt-glyph wpt-glyph-colour">
                                  <PlayerSwatch color={player.color} title={player.name} />
                                </span>
                                <span className="wpt-glyph wpt-glyph-crown">
                                  {player.is_winner ? <span className="workflow-crown" title={t('common.winner')}>👑</span> : null}
                                </span>
                              </span>
                              <span className="wpt-name-col">
                                <button
                                  type="button"
                                  className="workflow-player-name-link workflow-player-name-link--strong"
                                  title={t('player.analyze')}
                                  onClick={() => openMainPlayer(player.player_key)}
                                >
                                  <PlayerDisplayName name={player.name} />
                                </button>
                                {player.fingerprint_match ? (
                                  <FingerprintBadge match={player.fingerprint_match} />
                                ) : null}
                              </span>
                            </div>
                            <div className="wpt-cell wpt-apm">{player.apm}</div>
                            <div className="wpt-cell wpt-featuring">
                              <div className="workflow-pattern-pills">
                                {boPatterns.map((pattern, idx) => renderPatternPill(pattern, `player-${player.player_id}-bo-${idx}`, undefined, markerRegistry))}
                                {gameSummaryParts.positive.map(renderGameSummarySignalPill)}
                                {restPatterns.map((pattern, idx) => renderPatternPill(pattern, `player-${player.player_id}-${idx}`, undefined, markerRegistry))}
                                <SpellcastsPill spells={collectPlayerSpells(playerPhases)} />
                              </div>
                            </div>
                            <div className="wpt-cell wpt-comp">
                              {playerPhases.length > 0 ? (
                                <CompositionZones phases={playerPhases} slim />
                              ) : null}
                            </div>
                          </React.Fragment>
                        );
                      })}
                    </div>
                  </>
                )}

                {mainGameTab === 'events' && (
                  <div className="workflow-card workflow-card-recent-games">
                    <div className="workflow-events-controls">
                      <div className="workflow-events-filters">
                      <div className="workflow-events-filter-row" role="group" aria-label={t('events.filterByTypeAria')}>
                        {[
                          { key: 'attack', label: t('events.attack') },
                          { key: 'expansion', label: t('events.expansion') },
                          { key: 'drop', label: t('events.drop') },
                          { key: 'rush', label: t('events.rush') },
                          { key: 'leaves', label: t('events.filter.leaves') },
                          { key: 'recall', label: t('events.recall') },
                          { key: 'nuke', label: t('events.nuke') },
                          { key: 'becameRace', label: t('events.filter.becameRace') },
                          { key: 'alliance', label: t('events.alliance') },
                        ].map(({ key, label }) => {
                          const available = gameEventTopicAvailability[key];
                          const active = mainSummaryFilters[key];
                          return (
                            <label
                              key={key}
                              className={`workflow-events-filter-chip${active ? ' workflow-events-filter-chip--active' : ''}${!available ? ' workflow-events-filter-chip--disabled' : ''}`}
                            >
                              <input
                                type="checkbox"
                                disabled={!available}
                                checked={active}
                                onChange={(e) => setMainSummaryFilters((prev) => ({ ...prev, [key]: e.target.checked }))}
                              />
                              {label}
                            </label>
                          );
                        })}
                      </div>
                      {mainGamePlayers.length > 0 ? (
                        <div className="workflow-events-filter-row workflow-events-player-filters" role="group" aria-label={t('events.filterByPlayerAria')}>
                          {mainGamePlayers.map((player) => {
                            const enabled = mainEventsPlayerEnabledById[String(player.player_id)] !== false;
                            return (
                              <button
                                type="button"
                                key={`event-filter-${player.player_id}`}
                                className={`workflow-events-player-chip${enabled ? '' : ' workflow-events-player-chip--off'}`}
                                aria-pressed={enabled}
                                title={enabled ? t('events.hidePlayerEvents', { name: player.name }) : t('events.showPlayerEvents', { name: player.name })}
                                onClick={() => setMainEventsPlayerEnabledById((prev) => ({
                                  ...prev,
                                  [String(player.player_id)]: !enabled,
                                }))}
                              >
                                <PlayerSwatch color={player.color} title={player.name} />
                                <span>{player.name}</span>
                              </button>
                            );
                          })}
                          <button
                            type="button"
                            className="workflow-legend-bulk-btn"
                            onClick={() => setMainEventsPlayerEnabledById(
                              Object.fromEntries(mainGamePlayers.map((p) => [String(p.player_id), false])),
                            )}
                          >
                            {t('common.none')}
                          </button>
                          <button
                            type="button"
                            className="workflow-legend-bulk-btn"
                            onClick={() => setMainEventsPlayerEnabledById(
                              Object.fromEntries(mainGamePlayers.map((p) => [String(p.player_id), true])),
                            )}
                          >
                            {t('common.all')}
                          </button>
                        </div>
                      ) : null}
                      </div>
                      <div className="workflow-events-warnings">
                        {!hasTeamInfo ? (
                          <div className="workflow-section-warning workflow-events-warning">
                            {t('events.teamWarning', { reason: mainGame?.team_info_incomplete ? t('games.teamInfoIncomplete') : t('games.noTeamInfo') })}
                          </div>
                        ) : null}
                        <div className="workflow-section-warning workflow-events-warning">
                          {t('events.narrativeWarning')}
                        </div>
                      </div>
                    </div>
                    <div
                      className="workflow-events-layout"
                      ref={setMainEventsLayoutEl}
                      style={mainEventsMapColPx ? { gridTemplateColumns: `${mainEventsMapColPx}px minmax(0, 1fr)` } : undefined}
                    >
                        <div className="workflow-event-map-panel" ref={setMainEventMapPanelEl}>
                          {mainMapVisualAvailable ? (
                            <>
                              <div
                                className={`workflow-event-map-frame${selectedEventAnimCategory ? ` workflow-anim-${selectedEventAnimCategory}` : ''}`}
                                style={{ '--map-aspect': mainEventMapAspect }}
                              >
                                <img src={mainMapVisualURL} alt={t('events.mapAlt', { map: mainGame.map_name })} className="workflow-event-map-image" />
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
                                        markerWidth="5"
                                        markerHeight="5"
                                        refX="4.5"
                                        refY="2.5"
                                        orient="auto"
                                      >
                                        <polygon points="0 0, 5 2.5, 0 5" fill={selectedMainGameArrow?.color || 'currentColor'} />
                                      </marker>
                                      {/* Marker that inherits the line's stroke color via SVG2 context-stroke,
                                          and auto-flips at markerStart via auto-start-reverse. Used by alliance
                                          overlay arrows so each pair side is colorized to its player. */}
                                      <marker
                                        id="workflow-event-arrowhead-context"
                                        markerWidth="5"
                                        markerHeight="5"
                                        refX="4.5"
                                        refY="2.5"
                                        orient="auto-start-reverse"
                                      >
                                        <polygon points="0 0, 5 2.5, 0 5" fill="context-stroke" />
                                      </marker>
                                    </defs>
                                    {selectedMainGameMapPolygons.map((overlay) => {
                                      const onEnter = (e) => updateMainEventMapHover(e, { ownerName: overlay.ownerName, baseName: overlay.baseName, ownerColor: overlay.ownerColor });
                                      return (
                                        <polygon
                                          key={overlay.key}
                                          points={overlay.points}
                                          className="workflow-event-map-base-polygon"
                                          style={{ fill: `${overlay.ownerColor}66`, stroke: overlay.teamColor || overlay.ownerColor, strokeWidth: overlay.strokeWidth || 0.4 }}
                                          onMouseEnter={onEnter}
                                          onMouseMove={onEnter}
                                          onMouseLeave={clearMainEventMapHover}
                                        />
                                      );
                                    })}
                                    {selectedMainGameArrow && !selectedEventFocus?.travel ? (
                                      <line
                                        key={`arrow-${selectedMainGameEventKeyResolved}`}
                                        x1={selectedMainGameArrow.from.x}
                                        y1={selectedMainGameArrow.from.y}
                                        x2={selectedMainGameArrow.to.x}
                                        y2={selectedMainGameArrow.to.y}
                                        className="workflow-event-map-attack-line"
                                        style={{ color: selectedMainGameArrow.color, stroke: selectedMainGameArrow.color }}
                                        markerEnd="url(#workflow-event-arrowhead)"
                                        pathLength="1"
                                      />
                                    ) : null}
                                  </svg>
                                ) : null}
                                {selectedMainGameAllianceOverlay ? (
                                  <svg
                                    className="workflow-event-map-overlay workflow-event-map-overlay-alliance"
                                    viewBox="0 0 100 100"
                                    preserveAspectRatio="none"
                                    aria-hidden="true"
                                  >
                                    <defs>
                                      <marker
                                        id="workflow-event-arrowhead-alliance"
                                        markerWidth="5"
                                        markerHeight="5"
                                        refX="4.5"
                                        refY="2.5"
                                        orient="auto-start-reverse"
                                      >
                                        <polygon points="0 0, 5 2.5, 0 5" fill="context-stroke" />
                                      </marker>
                                    </defs>
                                    {selectedMainGameAllianceOverlay.pairs.flatMap((pair) => ([
                                      <line
                                        key={`alliance-${selectedMainGameEventKeyResolved}-${pair.key}-a`}
                                        x1={pair.a.from.x}
                                        y1={pair.a.from.y}
                                        x2={pair.mid.x}
                                        y2={pair.mid.y}
                                        className="workflow-event-map-alliance-line"
                                        style={{ stroke: pair.a.color }}
                                        markerStart="url(#workflow-event-arrowhead-alliance)"
                                        markerEnd="url(#workflow-event-arrowhead-alliance)"
                                        pathLength="1"
                                      />,
                                      <line
                                        key={`alliance-${selectedMainGameEventKeyResolved}-${pair.key}-b`}
                                        x1={pair.b.from.x}
                                        y1={pair.b.from.y}
                                        x2={pair.mid.x}
                                        y2={pair.mid.y}
                                        className="workflow-event-map-alliance-line"
                                        style={{ stroke: pair.b.color }}
                                        markerStart="url(#workflow-event-arrowhead-alliance)"
                                        markerEnd="url(#workflow-event-arrowhead-alliance)"
                                        pathLength="1"
                                      />,
                                    ]))}
                                  </svg>
                                ) : null}
                                {selectedMainGameTrainedUnitsAnchors.map((entry) => {
                                  // Edge-aware anchoring: the chip extends inward
                                  // toward the map center based on which map
                                  // quadrant the anchor sits in. Prevents the
                                  // chip from clipping the map frame when the
                                  // player's polygon hugs an edge.
                                  const x = entry.point.x;
                                  const y = entry.point.y;
                                  const tx = x < 20 ? '0%' : x > 80 ? '-100%' : '-50%';
                                  const ty = y < 20 ? '0%' : y > 80 ? '-100%' : '-50%';
                                  return (
                                  <div
                                    key={`trained-units-${selectedMainGameEventKeyResolved}-${entry.playerID}`}
                                    className="workflow-event-map-trained-units"
                                    style={{
                                      left: `${x}%`,
                                      top: `${y}%`,
                                      transform: `translate(${tx}, ${ty})`,
                                    }}
                                  >
                                    {entry.items.map((u) => {
                                      const icon = getUnitIcon(u.name);
                                      if (!icon) return null;
                                      return (
                                        <span key={`tu-${entry.playerID}-${u.name}`} className="workflow-event-map-trained-unit">
                                          <img src={icon} alt={t.server(`server.name.${slugKey(u.name)}`, u.name)} title={t('events.unitCount', { unit: t.server(`server.name.${slugKey(u.name)}`, u.name), count: u.count })} />
                                          <span className="workflow-event-map-trained-unit-count">×{u.count}</span>
                                        </span>
                                      );
                                    })}
                                    {entry.more > 0 ? (
                                      <span className="workflow-event-map-trained-unit-more" title={t('events.moreUnits', { count: entry.more })}>+{entry.more}</span>
                                    ) : null}
                                  </div>
                                  );
                                })}
                                {selectedMainGameArrow && selectedMainGameArrowUnits.length > 0 ? (() => {
                                  const cat = selectedEventAnimCategory;
                                  const isRecall = cat === 'recall';
                                  const from = selectedMainGameArrow.from;
                                  const to = selectedMainGameArrow.to;
                                  const travel = cat === 'attack' || cat === 'rush';
                                  // Where the army comes to rest. Attacks/rushes march to the
                                  // target; drops unload at the target; nukes fire from the
                                  // source; recalls pull from the source; otherwise midpoint.
                                  let anchor;
                                  if (travel || cat === 'drop' || isRecall) anchor = to;
                                  else if (cat === 'nuke') anchor = from;
                                  else anchor = { x: (from.x + to.x) / 2, y: (from.y + to.y) / 2 };
                                  return (
                                    <div
                                      key={`unit-overlay-${selectedMainGameEventKeyResolved}`}
                                      className={[
                                        'workflow-event-map-unit-overlay',
                                        selectedMainGameArrowUnits.length > 2 ? 'workflow-event-map-unit-overlay--grid' : '',
                                        isRecall ? 'workflow-event-map-unit-overlay--recall' : '',
                                        travel ? 'workflow-event-map-unit-overlay--travel' : '',
                                      ].filter(Boolean).join(' ')}
                                      style={{
                                        left: `${anchor.x}%`,
                                        top: `${anchor.y}%`,
                                      }}
                                    >
                                      {selectedMainGameArrowUnits.map((unit, unitIdx) => (
                                        <img
                                          key={`${selectedMainGameEventKeyResolved}-${unit.name}-${unitIdx}`}
                                          src={unit.icon}
                                          alt={t.server(`server.name.${slugKey(unit.name)}`, unit.name)}
                                          title={t.server(`server.name.${slugKey(unit.name)}`, unit.name)}
                                          className="workflow-event-map-unit-icon"
                                        />
                                      ))}
                                    </div>
                                  );
                                })() : null}
                                {selectedLeaveInfo ? (
                                  <div
                                    key={`leave-marker-${selectedMainGameEventKeyResolved}`}
                                    className="workflow-event-map-leave-marker"
                                    style={{ left: `${selectedLeaveInfo.point.x}%`, top: `${selectedLeaveInfo.point.y}%` }}
                                  >
                                    <span className="workflow-event-map-leave-emoji" role="img" aria-label={selectedLeaveInfo.emoji === '💤' ? t('events.stoppedPlaying') : t('events.leftGame')}>
                                      {selectedLeaveInfo.emoji}
                                    </span>
                                    <span className="workflow-event-map-leave-name" style={mapLabelStyle(selectedLeaveInfo.color)}>
                                      {selectedLeaveInfo.name}
                                    </span>
                                  </div>
                                ) : null}
                                {selectedMainGameAllianceOverlay
                                  ? selectedMainGameAllianceOverlay.pairs.map((pair) => (
                                    <div
                                      key={`alliance-emoji-${selectedMainGameEventKeyResolved}-${pair.key}`}
                                      className="workflow-event-map-flag-overlay workflow-event-map-alliance-handshake"
                                      style={{ left: `${pair.mid.x}%`, top: `${pair.mid.y}%` }}
                                      title={t('events.allianceFormed')}
                                    >
                                      <span role="img" aria-label={t('events.alliance')}>🤝</span>
                                    </div>
                                  ))
                                  : null}
                                {selectedMainGameExpansionOverlay ? (
                                  <img
                                    key={`expansion-${selectedMainGameEventKeyResolved}`}
                                    src={selectedMainGameExpansionOverlay.icon}
                                    alt={t('events.expansionBuildingAlt')}
                                    className="workflow-event-map-expansion-overlay"
                                    style={{
                                      left: `${selectedMainGameExpansionOverlay.point.x}%`,
                                      top: `${selectedMainGameExpansionOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                                {selectedBecameOverlay ? (
                                  <img
                                    key={`became-${selectedMainGameEventKeyResolved}`}
                                    src={selectedBecameOverlay.icon}
                                    alt={t('events.mindControlAlt')}
                                    title={t('events.mindControlled')}
                                    className="workflow-event-map-expansion-overlay workflow-event-map-became-overlay"
                                    style={{
                                      left: `${selectedBecameOverlay.point.x}%`,
                                      top: `${selectedBecameOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                                {selectedMainGameRecallOverlay ? (
                                  <img
                                    key={`recall-${selectedMainGameEventKeyResolved}`}
                                    src={selectedMainGameRecallOverlay.icon}
                                    alt={t('events.recallDestinationAlt')}
                                    title={selectedMainGameEvent?.target_base ? t('events.recallDestinationInferred') : t('events.recallCastPoint')}
                                    className={`workflow-event-map-expansion-overlay workflow-event-map-expansion-overlay--recall-arbiter${selectedMainGameArrow ? ' workflow-event-map-arbiter--travel' : ''}`}
                                    style={{
                                      left: `${selectedMainGameRecallOverlay.point.x}%`,
                                      top: `${selectedMainGameRecallOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                                {selectedMainGameDropOverlay ? (
                                  <img
                                    key={`drop-${selectedMainGameEventKeyResolved}`}
                                    src={selectedMainGameDropOverlay.icon}
                                    alt={t('events.dropTransportAlt')}
                                    title={t('events.dropLandingPoint')}
                                    className={`workflow-event-map-expansion-overlay workflow-event-map-expansion-overlay--recall-arbiter${selectedMainGameArrow ? ' workflow-event-map-vessel--travel' : ''}`}
                                    style={{
                                      left: `${selectedMainGameDropOverlay.point.x}%`,
                                      top: `${selectedMainGameDropOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                                {selectedMainGameProxyBuildingOverlay ? (
                                  <img
                                    key={`proxy-building-${selectedMainGameEventKeyResolved}`}
                                    src={selectedMainGameProxyBuildingOverlay.icon}
                                    alt={t('events.proxyBuildingAlt')}
                                    title={t('events.proxyBuildingPlacement')}
                                    className="workflow-event-map-expansion-overlay"
                                    style={{
                                      left: `${selectedMainGameProxyBuildingOverlay.point.x}%`,
                                      top: `${selectedMainGameProxyBuildingOverlay.point.y}%`,
                                    }}
                                  />
                                ) : null}
                                {selectedMainGameBOLabels.map((label) => {
                                  const raceIcon = getRaceIcon(label.race);
                                  return (
                                    <div
                                      key={label.key}
                                      className="workflow-event-map-bo-label"
                                      style={{ left: `${label.x}%`, top: `${label.y}%` }}
                                    >
                                      <div className="workflow-event-map-bo-label-name" style={mapLabelStyle(label.color)}>
                                        {raceIcon ? <img src={raceIcon} alt={label.race || t('race.alt')} className="unit-icon-inline workflow-event-map-bo-label-race" /> : null}
                                        {label.isWinner ? <span className="workflow-crown" title={t('common.winner')}>👑</span> : null}
                                        {label.name}
                                      </div>
                                      {label.boNames.length > 0 ? (
                                        <div className="workflow-event-map-bo-label-bo">{label.boNames.join(' / ')}</div>
                                      ) : null}
                                    </div>
                                  );
                                })}
                                {selectedEventAnimCategory === 'nuke' && selectedMainGameArrow ? (
                                  <span
                                    key={`nuke-ring-${selectedMainGameEventKeyResolved}`}
                                    className="workflow-event-map-shockwave workflow-event-map-shockwave--nuke"
                                    style={{ left: `${selectedMainGameArrow.to.x}%`, top: `${selectedMainGameArrow.to.y}%` }}
                                    aria-hidden="true"
                                  />
                                ) : null}
                                {selectedEventAnimCategory === 'expansion' && selectedMainGameExpansionOverlay ? (
                                  <span
                                    key={`expand-ring-${selectedMainGameEventKeyResolved}`}
                                    className="workflow-event-map-shockwave workflow-event-map-shockwave--expansion"
                                    style={{ left: `${selectedMainGameExpansionOverlay.point.x}%`, top: `${selectedMainGameExpansionOverlay.point.y}%` }}
                                    aria-hidden="true"
                                  />
                                ) : null}
                                {selectedMainGameEvent && !selectedLeaveInfo && normalizeEventType(selectedMainGameEvent?.type) !== 'bo_openers' ? (
                                  <div
                                    key={`event-narration-${selectedMainGameEventKeyResolved}`}
                                    className={[
                                      'workflow-event-map-narration',
                                      selectedEventFocus ? 'workflow-event-map-narration--located' : 'workflow-event-map-narration--bottom',
                                      selectedEventFocus?.travel ? 'workflow-event-map-narration--travel' : '',
                                      selectedEventFocus && selectedEventFocus.to.y > 78 ? 'workflow-event-map-narration--above' : '',
                                    ].filter(Boolean).join(' ')}
                                    style={selectedEventFocus ? {
                                      left: `${selectedEventFocus.to.x}%`,
                                      top: `${selectedEventFocus.to.y}%`,
                                    } : undefined}
                                  >
                                    <span className="workflow-event-map-narration-text">
                                      {renderGameEventDescription(selectedMainGameEvent, markerRegistry, mainEventRaceByPlayerID)}
                                    </span>
                                  </div>
                                ) : null}
                              </div>
                            </>
                          ) : (
                            <div className="workflow-map-summary-fallback">
                              {t('events.mapUnavailable')}
                              {mainMapVisual?.resolution_note ? ` (${mainMapVisual.resolution_note})` : ''}
                            </div>
                          )}
                          {mainEventMapHover ? (
                            <div
                              className="workflow-event-map-tooltip"
                              style={{ left: `${mainEventMapHover.x}px`, top: `${mainEventMapHover.y}px` }}
                            >
                              <span
                                className="workflow-event-map-tooltip-dot"
                                style={{ background: mainEventMapHover.ownerColor }}
                              />
                              <span className="workflow-event-map-tooltip-text">
                                <strong>{mainEventMapHover.ownerName}</strong>
                                {mainEventMapHover.baseName ? (
                                  <span className="workflow-event-map-tooltip-base">{mainEventMapHover.baseName}</span>
                                ) : null}
                              </span>
                            </div>
                          ) : null}
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
                                const isLeaveGame = ['leave_game', 'player_dropped', 'mass_disconnect'].includes(normalizeEventType(event?.type));
                                if (!isLeaveGame && phase !== lastPhase) {
                                  // Only show "Mid game" / "Late game" when mid game actually
                                  // ended; otherwise the game never reached those phases.
                                  let label = null;
                                  if (phase === 'early' && earlyEnd > 0) label = t('events.phase.early');
                                  else if (phase === 'mid' && midEnd > 0) label = t('events.phase.mid');
                                  else if (phase === 'late' && midEnd > 0) label = t('events.phase.late');
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
                                    <span className="workflow-event-row-time">{formatDuration(event.second)}</span>
                                    <span className="workflow-event-row-body">
                                      <span>{renderGameEventDescription(event, markerRegistry, mainEventRaceByPlayerID)}</span>
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
                                          {castEntries.map(([spell, count]) => {
                                            const spellName = t.server(`server.name.${slugKey(spell)}`, spell);
                                            return (
                                              <span key={`cast-${spell}`} className="workflow-event-row-cast-pill" title={t('events.castNearAttack', { spell: spellName, count })}>
                                                {Number(count) > 1 ? `${count}× ` : ''}{spellName}
                                              </span>
                                            );
                                          })}
                                        </span>
                                      ) : null}
                                    </span>
                                  </button>,
                                );
                              });
                              return nodes;
                            })()
                          ) : (
                            <div className="chart-empty">{t('events.noMatch')}</div>
                          )}
                        </div>
                      </div>
                  </div>
                )}

                {mainGameTab === 'units' && (
                  <div className="workflow-card workflow-card-chat-summary">
                    <div className="workflow-production-top-row">
                      <div className="workflow-radio-group" role="radiogroup" aria-label={t('units.viewAria')}>
                        {[
                          { value: 'all', label: t('common.all') },
                          { value: 'units', label: t('units.units') },
                          { value: 'buildings', label: t('units.buildings') },
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
                        {t('units.warning')}
                      </div>
                    </div>
                    <div className="workflow-summary-filter-row">
                      <div className="workflow-radio-group">
                        {(productionView === 'units'
                          ? [
                              { value: 'all', label: t('units.filter.allUnits') },
                              { value: 'workers', label: t('units.filter.workers') },
                              { value: 'non-workers', label: t('units.filter.nonWorkers') },
                              { value: 'spellcasters', label: t('units.filter.spellcasters') },
                              { value: 'tier-1', label: t('units.filter.tier1') },
                              { value: 'tier-2', label: t('units.filter.tier2') },
                              { value: 'tier-3', label: t('units.filter.tier3') },
                            ]
                          : productionView === 'buildings'
                            ? [
                                { value: 'all', label: t('units.filter.allBuildings') },
                                { value: 'defenses', label: t('units.filter.defenses') },
                                { value: 'tier-1', label: t('units.filter.tier1') },
                                { value: 'tier-2', label: t('units.filter.tier2') },
                                { value: 'tier-3', label: t('units.filter.tier3') },
                              ]
                            : [
                                { value: 'all', label: t('common.all') },
                                { value: 'tier-2', label: t('units.filter.tier2') },
                                { value: 'tier-3', label: t('units.filter.tier3') },
                                { value: 'defenses', label: t('units.filter.defenses') },
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
                        placeholder={productionView === 'buildings' ? t('units.filterBuildingPlaceholder') : t('units.filterUnitPlaceholder')}
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
                            <th>{t('units.table.slice')}</th>
                            {mainGamePlayers.map((player) => (
                              <th
                                key={player.player_id}
                                style={hasTeamInfo ? { backgroundColor: teamColorRgba(player.team, 0.2) } : undefined}
                              >
                                {player.is_winner ? <span className="workflow-crown" title={t('common.winner')}>👑</span> : null}
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
                                            {getUnitIcon(unit.unit_type) ? <img src={getUnitIcon(unit.unit_type)} alt={t.server(`server.name.${slugKey(unit.unit_type)}`, unit.unit_type)} className="workflow-unit-chip-icon" /> : null}
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

                {mainGameTab === 'hotkeys' && (
                  <div className="workflow-card">
                    {mainGameHotkeysLoading ? <div className="chart-empty">{t('hotkeys.loading')}</div> : null}
                    {!mainGameHotkeysLoading && mainGameHotkeysError ? <div className="chart-empty">{mainGameHotkeysError}</div> : null}
                    {!mainGameHotkeysLoading && !mainGameHotkeysError && mainGameHotkeys ? (
                      <>
                        <HotkeyTimeline payload={mainGameHotkeys} />
                        <HotkeyMaps
                          replayId={selectedReplayId}
                          players={mainGameHotkeys.players || []}
                        />
                      </>
                    ) : null}
                  </div>
                )}

                {mainGameTab === 'supply-timeline' && (
                  <SupplyTimeline
                    players={mainGamePlayers}
                    timeline={mainGame.production_timeline || []}
                    durationSeconds={mainGame.duration_seconds || 0}
                    playerColor={playerColorToCss}
                  />
                )}

                {mainGameTab === 'timings' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-section-top-row">
                      <div className="workflow-production-tabs workflow-timing-tabs" role="tablist" aria-label={t('timings.tabsAria')}>
                        {TIMING_CATEGORY_CONFIG.map((cfg) => (
                          <button
                            key={cfg.id}
                            className={`workflow-production-tab ${mainTimingCategory === cfg.id ? 'workflow-production-tab-active' : ''}`}
                            onClick={() => setMainTimingCategory(cfg.id)}
                            role="tab"
                            aria-selected={mainTimingCategory === cfg.id}
                          >
                            {t(cfg.labelKey)}
                          </button>
                        ))}
                      </div>
                      {mainTimingNotice ? (
                        <div className="workflow-section-warning">{mainTimingNotice}</div>
                      ) : null}
                    </div>
                    {isResearchTiming ? (
                      <div className="workflow-timing-overlay-toggles" role="group" aria-label={t('timings.overlayAria')}>
                        {RESEARCH_SUBCATEGORIES.map((sub) => {
                          const active = mainResearchSubcategories.includes(sub.id);
                          return (
                            <label
                              key={sub.id}
                              className={`workflow-timing-overlay-toggle ${active ? 'is-active' : ''}`}
                              style={active ? { borderColor: sub.color, color: sub.color } : undefined}
                            >
                              <input
                                type="checkbox"
                                checked={active}
                                onChange={() => toggleResearchSubcategory(sub.id)}
                              />
                              <span className="workflow-timing-overlay-swatch" style={{ background: sub.color }} />
                              <span>{t(sub.labelKey)}</span>
                            </label>
                          );
                        })}
                      </div>
                    ) : null}
                    {isHpTiming ? (
                      mainHpTimingByRace.length === 0 ? (
                        <div className="workflow-card"><div className="chart-empty">{t('timings.noHp')}</div></div>
                      ) : (
                        mainHpTimingByRace.map((raceChart) => (
                          <div key={`hp-${raceChart.race}`} className="workflow-card">
                            <div className="workflow-card-title"><span>{t('timings.hpTitle', { race: raceChart.raceLabel })}</span></div>
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
                                  <span>{t.server(`server.name.${slugKey(labelName)}`, labelName)}</span>
                                </label>
                              ))}
                            </div>
                            <TimingScatterRows
                              title=""
                              series={raceChart.series}
                              durationSeconds={mainGame.duration_seconds}
                              colorBy="player"
                              showLegend={false}
                              markerMode="dot"
                              inlineLegend={true}
                              rowLabelMode="worker-icon"
                              rowGroupingMode="none"
                            />
                          </div>
                        ))
                      )
                    ) : isResearchTiming && mainResearchSubcategories.length === 0 ? (
                      <div className="workflow-card"><div className="chart-empty">{t('timings.selectCategory')}</div></div>
                    ) : (
                      <TimingScatterRows
                        title=""
                        series={mainTimingSeries}
                        durationSeconds={mainGame.duration_seconds}
                        colorBy={mainTimingColorBy}
                        showLegend={isResearchTiming}
                        markerMode={mainTimingCategoryConfig.markerMode || 'dot'}
                        inlineLegend={isResearchTiming}
                        noticeText=""
                        rowLabelMode={isResearchTiming ? 'worker-icon' : 'name-only'}
                        rowGroupingMode={isResearchTiming ? 'race' : 'none'}
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
                          beta={featureIsBeta(markerRegistry?.[bo.feature_key])}
                        />
                      ))
                    ) : (
                      <div className="workflow-card">
                        <div className="chart-empty">{t('buildOrders.none')}</div>
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
                        <div className="chart-empty">{t('mutaTiming.none')}</div>
                      </div>
                    )}
                  </div>
                )}
                {mainGameTab === 'alliances' && (
                  <div className="workflow-timing-charts">
                    {mainGame?.team_stacking ? (
                      <div className="workflow-section-warning">
                        {t('alliances.stackingWarning', { minutes: Math.round((mainGame.alliance_stacking_threshold_seconds || 300) / 60) })}
                      </div>
                    ) : null}
                    <AllianceTimeline
                      players={Array.isArray(mainGame?.players) ? mainGame.players : []}
                      timeline={Array.isArray(mainGame?.alliance_timeline) ? mainGame.alliance_timeline : []}
                      chat={Array.isArray(mainGame?.alliance_tab_chat) ? mainGame.alliance_tab_chat : []}
                      gameEvents={Array.isArray(mainGame?.game_events) ? mainGame.game_events : []}
                      durationSeconds={mainGame?.duration_seconds || 0}
                      earlyEndsAt={mainGame?.early_game_ends_at_second || 0}
                      midEndsAt={mainGame?.mid_game_ends_at_second || 0}
                      stackingThresholdSeconds={mainGame?.alliance_stacking_threshold_seconds || 300}
                      getRaceIcon={getWorkerIconForRace}
                      getPlayerColor={(p) => playerColorToCss(p?.color)}
                    />
                  </div>
                )}
                {mainGameTab === 'first-unit-efficiency' && (
                  <div className="workflow-timing-charts">
                    <div className="workflow-section-top-row">
                      <span className="workflow-section-top-spacer" aria-hidden="true" />
                      <div className="workflow-section-warning">
                        {t('firstUnit.warning')}
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
                        <div className="chart-empty">{t('firstUnit.none')}</div>
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
                            x_axis_label: t('appChart.cadenceScore'),
                            y_axis_label: t('appChart.density'),
                            overlay_value_label: t('appChart.overlay.cadence'),
                            overlay_count_label: t('appChart.overlay.units'),
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
                                t('appChart.tooltip.cadenceScore', { value: Number(player.average_apm || 0).toFixed(3) }),
                                t('appChart.tooltip.ratePerMinute', { value: Number(player.average_rate_per_min || 0).toFixed(2) }),
                                t('appChart.tooltip.gapCv', { value: Number(player.average_cv_gap || 0).toFixed(2) }),
                                t('appChart.tooltip.burstiness', { value: Number(player.average_burstiness || 0).toFixed(2) }),
                                t('appChart.tooltip.idleGapRatio', { value: (Number(player.average_idle20_ratio || 0) * 100).toFixed(1) }),
                                t('appChart.tooltip.unitsInWindow', { count: Number(player.games_played || 0) }),
                                t('appChart.tooltip.windowLength', { value: formatDuration(Number(player.window_seconds || 0)) }),
                              ],
                            })),
                          }}
                        />
                      ) : (
                        <div className="chart-empty">{t('game.cadence.noEligible')}</div>
                      )}
                      <div className="workflow-card-subtitle"><span>{t('game.perPlayerBreakdown')}</span></div>
                      {(mainGame?.unit_production_cadence || []).map((entry) => (
                        <div key={`game-cadence-${entry.player_id}`} className="workflow-pattern-row">
                          <span>
                            {entry.is_winner ? '👑 ' : ''}{entry.player_name}
                          </span>
                          <span title={entry.eligible ? `rate=${Number(entry.rate_per_minute || 0).toFixed(2)}, cv=${Number(entry.cv_gap || 0).toFixed(2)}, burstiness=${Number(entry.burstiness || 0).toFixed(2)}, idle20=${(Number(entry.idle20_ratio || 0) * 100).toFixed(1)}%, units=${Number(entry.units_produced || 0)}, gaps=${Number(entry.gap_count || 0)}` : String(entry.ineligible_reason || '')}>
                            {entry.eligible
                              ? t('game.cadence.value', { score: Number(entry.cadence_score || 0).toFixed(3), units: Number(entry.units_produced || 0), window: formatDuration(Number(entry.window_seconds || 0)) })
                              : t('game.notAvailable', { reason: entry.ineligible_reason || t('game.insufficientData') })}
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
                            x_axis_label: viewportText.axisLabel,
                            y_axis_label: t('appChart.density'),
                            overlay_value_label: viewportText.overlayValueLabel,
                            overlay_count_label: t('appChart.overlay.player'),
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
                                t('appChart.tooltip.labelValue', { label: viewportText.title, value: viewportText.valueFormatter(player.average_apm) }),
                              ],
                            })),
                          }}
                        />
                      ) : (
                        <div className="chart-empty">{t('game.viewport.noEligible')}</div>
                      )}
                      <div className="workflow-card-subtitle"><span>{t('game.perPlayerBreakdown')}</span></div>
                      {(mainGame?.viewport_multitasking || []).map((entry) => (
                        <div key={`game-viewport-${entry.player_id}`} className="workflow-pattern-row">
                          <span>
                            {entry.is_winner ? '👑 ' : ''}{entry.player_name}
                          </span>
                          <span title={entry.eligible ? viewportText.valueFormatter(entry.viewport_switch_rate) : String(entry.ineligible_reason || '')}>
                            {entry.eligible
                              ? viewportText.valueFormatter(entry.viewport_switch_rate)
                              : t('game.notAvailable', { reason: entry.ineligible_reason || t('game.insufficientData') })}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </>
            ) : (
              <div className="chart-empty">{t('game.selectPrompt')}</div>
            )}
          </div>
        )}

        {activeView === 'player' && (() => {
          const isFeaturedPlayer = isFeaturedPlayerKey(selectedPlayerKey);
          const isSkillProxiesTab = mainPlayerTab === 'skill-proxies' && !isFeaturedPlayer;
          const featured = mainPlayer?.featured || null;
          return (
          <div className="workflow-panel workflow-panel--player">
            {selectedPlayerKey ? (
              <>
                <div className={`workflow-title-row ${isFeaturedPlayer ? 'workflow-player-title-featured' : ''}`}>
                  {isFeaturedPlayer ? <ProPhoto url={featured?.photo_url} name={featured?.label} credit={featured?.photo_credit} large /> : null}
                  <div className="workflow-player-title-wrap">
                    <h2>
                      <span className="workflow-name-with-flag">
                        <CountryFlag code={mainPlayer?.country_code} playerKey={isFeaturedPlayer ? '' : (mainPlayer?.player_key || selectedPlayerKey)} />
                        <PlayerDisplayName name={mainPlayer?.player_name || (isFeaturedPlayer ? '' : selectedPlayerKey)} />
                      </span>
                      {isFeaturedPlayer ? <FeaturedBadge /> : null}
                      {!isFeaturedPlayer && mainPlayer?.fingerprint_match ? (
                        <span className="workflow-fingerprint-match"><FingerprintBadge match={mainPlayer.fingerprint_match} /></span>
                      ) : null}
                    </h2>
                    {isFeaturedPlayer && featured ? (
                      <div className="workflow-featured-links">
                        {featured.main_race ? <span>{raceLabel(featured.main_race)}</span> : null}
                        <span>{t('player.featured.gamesSampled', { count: featured.games_sampled })}</span>
                        {featured.liquipedia ? <a href={featured.liquipedia} target="_blank" rel="noopener noreferrer">Liquipedia</a> : null}
                      </div>
                    ) : null}
                    {!isFeaturedPlayer && mainPlayer && (Number(mainPlayer.games_played) || 0) < 5 ? (
                      <span className="workflow-inline-warning">{t('player.fewReplaysWarning')}</span>
                    ) : null}
                  </div>
                </div>
                {mainPlayerLoading ? (
                  <div className="workflow-meta">
                    <span className="workflow-subtle-note">{t('player.loadingOverview')}</span>
                  </div>
                ) : null}
                <div className="workflow-game-tab-stack">
                  <div className="workflow-production-tabs workflow-game-main-tabs" role="tablist" aria-label={t('player.sectionsAria')}>
                    <button type="button" role="tab" aria-selected={mainPlayerTab === 'summary'}
                      className={`workflow-production-tab ${mainPlayerTab === 'summary' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => { setMainPlayerTab('summary'); setMainPlayerSubtab(''); }}>
                      {t('tabs.summary')}
                    </button>
                    <button type="button" role="tab" aria-selected={mainPlayerTab === 'hotkeys'}
                      className={`workflow-production-tab ${mainPlayerTab === 'hotkeys' ? 'workflow-production-tab-active' : ''}`}
                      onClick={() => { setMainPlayerTab('hotkeys'); setMainPlayerSubtab(''); }}>
                      {t('tabs.hotkeys')}
                    </button>
                    {!isFeaturedPlayer ? (
                      <button type="button" role="tab" aria-selected={isSkillProxiesTab}
                        className={`workflow-production-tab ${isSkillProxiesTab ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => {
                          if (isSkillProxiesTab) return;
                          setMainPlayerTab('skill-proxies');
                          setMainPlayerSubtab('summary');
                        }}>
                        {t('tabs.skillProxies')}
                      </button>
                    ) : null}
                    {!isFeaturedPlayer ? (
                      <button type="button" role="tab" aria-selected={mainPlayerTab === 'chat-summary'}
                        className={`workflow-production-tab ${mainPlayerTab === 'chat-summary' ? 'workflow-production-tab-active' : ''}`}
                        onClick={() => { setMainPlayerTab('chat-summary'); setMainPlayerSubtab(''); }}>
                        {t('player.tabs.chatSummary')}
                      </button>
                    ) : null}
                  </div>

                </div>

                <div className="workflow-cards">
                  {mainPlayerTab === 'summary' && (() => {
                    const bnet = mainPlayer?.bnet_profile;
                    const showBnet = (isFeaturedPlayer || Number(mainPlayer?.bnet_games || 0) > 0) && !!bnet;
                    const bnetRecent = Array.isArray(bnet?.recent_games) ? bnet.recent_games : [];
                    const bnetLastPlayed = bnet?.last_played_at ? formatRelativeReplayDate(bnet.last_played_at) : '';
                    const bnetHours = Number(bnet?.play_time_seconds || 0) / 3600;
                    const keyOf = (name) => String(name || '').trim().toLowerCase();
                    const aliasSeen = new Set([keyOf(mainPlayer?.player_name || selectedPlayerKey)]);
                    const aliases = [];
                    (bnet?.toons || []).forEach((t) => {
                      const key = keyOf(t.toon);
                      if (!key || aliasSeen.has(key)) return;
                      aliasSeen.add(key);
                      aliases.push(t.toon);
                    });
                    const games = Array.isArray(mainPlayerLastGames) ? mainPlayerLastGames : [];
                    return (
                      <div className="workflow-card">
                        {showBnet ? (
                          <div className="wps-section">
                            <div className="workflow-card-title wps-section-title"><span>{t('player.bnet.title')}</span></div>
                            <div className="wps-stats">
                              <div className="wps-stat">
                                <span className="wps-stat-label">{t('player.bnet.ladder')}</span>
                                <span className="wps-stat-value">
                                  {(() => {
                                    if (!bnet.plays_ladder) return t('player.bnet.unranked');
                                    const mmr = Number(bnet.mmr || bnet.highest_mmr) || 0;
                                    return mmr ? t('player.bnet.mmr', { mmr }) : t('player.bnet.playsLadder');
                                  })()}
                                </span>
                                {(() => {
                                  if (!bnet.plays_ladder) return null;
                                  const wins = Number(bnet.ladder_wins) || 0;
                                  const losses = Number(bnet.ladder_losses) || 0;
                                  const parts = [];
                                  if (wins + losses > 0) parts.push(`${wins}-${losses}`);
                                  const peak = Number(bnet.highest_mmr) || 0;
                                  if (peak > (Number(bnet.mmr) || 0)) parts.push(t('player.bnet.peak', { mmr: peak }));
                                  return parts.length ? <span className="wps-stat-sub">{parts.join(', ')}</span> : null;
                                })()}
                              </div>
                              {bnet.lifetime_games ? (
                                <div className="wps-stat">
                                  <span className="wps-stat-label">{t('player.stats.games')}</span>
                                  <span className="wps-stat-value">{bnet.lifetime_games}</span>
                                </div>
                              ) : null}
                              {bnet.lifetime_games ? (
                                <div className="wps-stat">
                                  <span className="wps-stat-label">{t('player.stats.winRate')}</span>
                                  <span className="wps-stat-value">{((100 * bnet.lifetime_wins) / bnet.lifetime_games).toFixed(1)}%</span>
                                </div>
                              ) : null}
                              {bnet.average_apm ? (
                                <div className="wps-stat">
                                  <span className="wps-stat-label">APM</span>
                                  <span className="wps-stat-value">{bnet.average_apm.toFixed(1)}</span>
                                </div>
                              ) : null}
                              {bnetHours >= 1 ? (
                                <div className="wps-stat">
                                  <span className="wps-stat-label">{t('player.bnet.timePlayed')}</span>
                                  <span className="wps-stat-value">{t('player.bnet.hours', { hours: Math.round(bnetHours).toLocaleString() })}</span>
                                </div>
                              ) : null}
                              {bnetLastPlayed ? (
                                <div className="wps-stat">
                                  <span className="wps-stat-label">{t('common.lastPlayed')}</span>
                                  <span className="wps-stat-value">{bnetLastPlayed}</span>
                                  <span className="wps-stat-sub">{t('player.bnet.gamesLastWeek', { count: Number(bnet.games_last_week || 0) })}</span>
                                </div>
                              ) : null}
                            </div>
                            {bnet.habits?.summary ? (
                              <div className="wps-about" title={t('player.bnet.habitsTip')}>
                                🕒 {bnet.habits.summary}
                              </div>
                            ) : null}
                            {aliases.length ? (
                              <div className="wps-aliases">
                                <span className="wps-stat-label">{t('player.bnet.alsoPlaysAs')}</span>
                                {aliases.map((alias) => {
                                  const aliasKey = keyOf(alias);
                                  return mainPlayerKnownAliases[aliasKey] ? (
                                    <button
                                      key={aliasKey}
                                      type="button"
                                      className="wps-alias wps-alias-known"
                                      title={t('player.viewThisPlayer')}
                                      onClick={() => openMainPlayer(aliasKey)}
                                    >
                                      {alias}
                                    </button>
                                  ) : (
                                    <span key={aliasKey} className="wps-alias">{alias}</span>
                                  );
                                })}
                              </div>
                            ) : null}
                          </div>
                        ) : null}
                        {isFeaturedPlayer ? (
                          <div className="wps-section">
                            <div className="workflow-card-title wps-section-title"><span>{t('player.bnet.recentGames')}</span></div>
                            {!bnet ? (
                              <div className="chart-empty">
                                {(!bnetDisabled && bnetState === 'connected')
                                  ? t('player.bnet.fetchingProfile')
                                  : t('player.bnet.connectPrompt')}
                              </div>
                            ) : bnetRecent.length === 0 ? (
                              <div className="chart-empty">{t('player.bnet.noRecentGames')}</div>
                            ) : (
                              <div className="workflow-bnet-games">
                                <div className="workflow-bnet-game-row wbg-head">
                                  <span className="wbg-race" />
                                  <span className="wbg-when">{t('player.bnet.table.when')}</span>
                                  <span className="wbg-map">{t('common.map')}</span>
                                  <span className="wbg-result">{t('common.winLoss')}</span>
                                  <span className="wbg-apm">APM</span>
                                  <span className="wbg-opp">{t('player.bnet.table.opponent')}</span>
                                </div>
                                {bnetRecent.map((g) => {
                                  const raceIcon = getWorkerIconForRace(g.race);
                                  const result = String(g.result || '');
                                  const resultEmoji = result === 'win' ? '✅' : result === 'loss' ? '❌' : '·';
                                  const opponents = Array.isArray(g.opponents) ? g.opponents : [];
                                  return (
                                    <div key={`${g.match_guid || g.played_at}`} className="workflow-bnet-game-row">
                                      <span className="wbg-race">{raceIcon ? <img src={raceIcon} alt={raceLabel(g.race)} title={raceLabel(g.race)} /> : null}</span>
                                      <span className="wbg-when" title={g.played_at}>{formatRelativeReplayDate(g.played_at)}</span>
                                      <span className="wbg-map" title={g.map_name}>{g.map_name || '-'}</span>
                                      <span className="wbg-result" title={result}>{resultEmoji}</span>
                                      <span className="wbg-apm">{g.apm || ''}</span>
                                      <span className="wbg-opp">{opponents.map((o) => `${o.toon}${o.race ? ` (${o.race.slice(0, 1)})` : ''}`).join(', ')}</span>
                                    </div>
                                  );
                                })}
                              </div>
                            )}
                          </div>
                        ) : null}
                        {!isFeaturedPlayer ? (
                        <div className="wps-section">
                          {showBnet ? <div className="workflow-card-title wps-section-title"><span>{t('player.localGames')}</span></div> : null}
                          <div className="wps-stats">
                            <div className="wps-stat">
                              <span className="wps-stat-label">{t('player.stats.games')}</span>
                              <span className="wps-stat-value">{mainPlayer ? mainPlayer.games_played : '-'}</span>
                            </div>
                            <div className="wps-stat">
                              <span className="wps-stat-label">{t('player.stats.winRate')}</span>
                              <span className="wps-stat-value">{mainPlayer ? `${(mainPlayer.win_rate * 100).toFixed(1)}%` : '-'}</span>
                            </div>
                            <div className="wps-stat">
                              <span className="wps-stat-label">APM</span>
                              <span className="wps-stat-value">{mainPlayer ? mainPlayer.average_apm?.toFixed(1) : '-'}</span>
                              <span className="wps-stat-sub">{mainPlayer ? t('player.stats.eapm', { value: mainPlayer.average_eapm?.toFixed(1) }) : ''}</span>
                            </div>
                          </div>
                        {mainPlayerLastGamesLoading ? <div className="chart-empty">{t('player.lastGames.loading')}</div> : null}
                        {!mainPlayerLastGamesLoading && mainPlayerLastGamesError ? <div className="chart-empty">{mainPlayerLastGamesError}</div> : null}
                        {!mainPlayerLastGamesLoading && !mainPlayerLastGamesError && games.length > 0 ? (
                          <div className="workflow-last-games">
                            <div className="workflow-last-game-row wlg-head">
                              <span className="wlg-race" />
                              <span className="wlg-format">{t('player.lastGames.type')}</span>
                              <span className="wlg-len">{t('common.time')}</span>
                              <span className="wlg-map">{t('common.map')}</span>
                              <span className="wlg-result">{t('common.winLoss')}</span>
                              <span className="wlg-apm">APM</span>
                              <span className="wlg-featuring">{t('common.featuring')}</span>
                              <span className="wlg-comp"><CompositionZonesHeader /></span>
                            </div>
                            {games.map((g) => {
                              const cp = g.current_player;
                              const patterns = filterSummaryPillPatterns(cp?.detected_patterns || [], false);
                              const boPatterns = patterns.filter((pt) => isOpenerEventType(pt?.event_type));
                              const restPatterns = patterns.filter((pt) => !isOpenerEventType(pt?.event_type));
                              const phases = Array.isArray(cp?.composition) ? cp.composition : [];
                              const resultEmoji = cp?.disconnected ? '🔌' : (cp?.is_winner ? '✅' : '❌');
                              const resultTitle = cp?.disconnected ? t('player.result.disconnected') : (cp?.is_winner ? t('player.result.win') : t('player.result.loss'));
                              const raceIcon = getWorkerIconForRace(cp?.race);
                              return (
                                <div
                                  key={g.replay_id}
                                  className="workflow-last-game-row"
                                  role="button"
                                  tabIndex={0}
                                  onClick={() => openMainGame(g.replay_id)}
                                  onKeyDown={(e) => { if (e.key === 'Enter') openMainGame(g.replay_id); }}
                                >
                                  <span className="wlg-race">
                                    {raceIcon ? <img src={raceIcon} alt={raceLabel(cp?.race)} title={raceLabel(cp?.race)} /> : null}
                                  </span>
                                  <span className="wlg-format">
                                    {(() => {
                                      const format = g.team_format || '';
                                      const matchup = String(g.matchup || '');
                                      if (format && format !== '1v1') return format;
                                      if (!matchup) return format;
                                      // Orient the matchup from this player's side: kospetrov
                                      // as Zerg in a PvZ game reads ZvP.
                                      const mine = String(cp?.race || '').slice(0, 1).toUpperCase();
                                      const sides = matchup.split('v');
                                      if (mine && sides.length === 2 && sides[1] === mine && sides[0] !== mine) {
                                        return `${sides[1]}v${sides[0]}`;
                                      }
                                      return matchup;
                                    })()}
                                  </span>
                                  <span className="wlg-len">{formatDuration(g.duration_seconds)}</span>
                                  <span className="wlg-map">{renderMapNameWithKind(g.map_name, g.map_kind)}</span>
                                  <span className="wlg-result" title={resultTitle}>{resultEmoji}</span>
                                  <span className="wlg-apm" title="APM">{cp?.apm ?? ''}</span>
                                  <span className="wlg-featuring">
                                    <div className="workflow-pattern-pills">
                                      {boPatterns.map((pattern, idx) => renderPatternPill(pattern, `lg-${g.replay_id}-bo-${idx}`, undefined, markerRegistry))}
                                      {restPatterns.map((pattern, idx) => renderPatternPill(pattern, `lg-${g.replay_id}-${idx}`, undefined, markerRegistry))}
                                      <SpellcastsPill spells={collectPlayerSpells(phases)} />
                                    </div>
                                  </span>
                                  <span className="wlg-comp">
                                    {phases.length > 0 ? <CompositionZones phases={phases} /> : null}
                                  </span>
                                </div>
                              );
                            })}
                          </div>
                        ) : null}
                        {!mainPlayerLastGamesLoading && !mainPlayerLastGamesError && mainPlayerLastGames && games.length === 0 ? (
                          <div className="chart-empty">{t('player.lastGames.none')}</div>
                        ) : null}
                        </div>
                        ) : null}
                      </div>
                    );
                  })()}

                  {isSkillProxiesTab && (
                    <div className="workflow-card workflow-card-fingerprints">
                      <div className="workflow-card-title"><span>{t('skillProxies.populationComparison')}</span></div>
                      {isFeaturedPlayer ? (
                        <div className="workflow-subtle-note">
                          {t('skillProxies.featuredNote', { name: featured?.label || t('skillProxies.thisProgamer') })}
                        </div>
                      ) : null}
                      {mainPlayerInsightLoading ? <div className="chart-empty">{t('skillProxies.loading')}</div> : null}
                      {!mainPlayerInsightLoading && mainPlayerInsightErrors.length > 0 ? (
                        <div className="chart-empty">{mainPlayerInsightErrors[0]}</div>
                      ) : null}
                      {!mainPlayerInsightLoading && mainPlayerInsightErrors.length === 0 ? (
                        <div className="workflow-insight-grid">
                          {mainPlayerInsights.map((insight) => {
                            const percentile = Number(insight.performance_percentile || 0);
                            const accent = insightScoreColor(percentile);
                            const overrideDesc = isFeaturedPlayer ? undefined : playerInsightDescriptionOverride(insight.insight_type);
                            const description = overrideDesc !== undefined
                              ? overrideDesc
                              : (isFeaturedPlayer ? insight.description : t.server(`server.insight.${insight.insight_type}.description`, insight.description));
                            const reasonArgs = Array.isArray(insight.ineligible_reason_args) ? insight.ineligible_reason_args : [];
                            const ineligibleReason = insight.ineligible_reason_key
                              ? t.server(`server.insight.reason.${insight.ineligible_reason_key}`, insight.ineligible_reason, Object.fromEntries(reasonArgs.map((arg, idx) => [idx, arg])))
                              : insight.ineligible_reason;
                            const popTab = playerInsightDestinationTab(insight.insight_type);
                            return (
                              <div
                                key={insight.insight_type}
                                className="workflow-insight-card workflow-insight-card-static"
                                style={insight.eligible ? { borderColor: `${accent}55`, boxShadow: `inset 0 0 0 1px ${accent}22` } : undefined}
                              >
                                <div className="workflow-insight-card-header">
                                  <span>{t.server(`server.insight.${insight.insight_type}.title`, insight.title)}</span>
                                </div>
                                {insight.eligible ? (
                                  <>
                                    <div className="workflow-insight-score-row">
                                      <span className="workflow-insight-score" style={{ color: accent }}>{insightSummaryLabel(percentile)}</span>
                                    </div>
                                    <div className="workflow-insight-value">{insightValueLabel(t, insight.player_value_label)}</div>
                                    <div className="workflow-subtle-note">{t('skillProxies.populationSize', { count: insight.population_size })}</div>
                                  </>
                                ) : (
                                  <>
                                    <div className="workflow-insight-unavailable">{t('skillProxies.notEnoughData')}</div>
                                    <div className="workflow-subtle-note">{libraryLoadingCopy || ineligibleReason || t('skillProxies.notAvailable')}</div>
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
                                      {t('skillProxies.seeAllPlayers')}
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

                  {mainPlayerTab === 'hotkeys' && (
                    <div className="workflow-card">
                      {mainPlayerHotkeySigLoading ? <div className="chart-empty">{t('hotkeys.loadingSignature')}</div> : null}
                      {!mainPlayerHotkeySigLoading && mainPlayerHotkeySigError ? <div className="chart-empty">{mainPlayerHotkeySigError}</div> : null}
                      {!mainPlayerHotkeySigLoading && !mainPlayerHotkeySigError && mainPlayerHotkeySig ? (
                        <HotkeySignature payload={mainPlayerHotkeySig} loadingNotice={libraryLoadingCopy} />
                      ) : null}
                    </div>
                  )}

                  {mainPlayerTab === 'chat-summary' && (
                    <div className="workflow-card workflow-card-chat-summary">
                      <div className="workflow-card-title"><span>{t('chat.title')}</span></div>
                      {mainPlayerChatSummaryLoading ? <div className="chart-empty">{t('chat.loading')}</div> : null}
                      {!mainPlayerChatSummaryLoading && mainPlayerChatSummaryError ? <div className="chart-empty">{mainPlayerChatSummaryError}</div> : null}
                      {!mainPlayerChatSummaryLoading && !mainPlayerChatSummaryError && (Number(mainPlayerChatSummary?.total_messages) || 0) === 0 ? (
                        <div className="chart-empty">{libraryLoadingCopy || t('chat.none')}</div>
                      ) : (
                        !mainPlayerChatSummaryLoading && !mainPlayerChatSummaryError && mainPlayerChatSummary ? (
                          <>
                            <div className="workflow-subtle-note">
                              {t('chat.stats', { messages: mainPlayerChatSummary?.total_messages || 0, games: mainPlayerChatSummary?.games_with_chat || 0, terms: mainPlayerChatSummary?.distinct_terms || 0 })}
                            </div>
                            <div className="workflow-card-subtitle"><span>{t('chat.topTerms')}</span></div>
                            {(mainPlayerChatSummary?.top_terms || []).length === 0 ? (
                              <div className="chart-empty">{t('chat.notEnough')}</div>
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
                            <div className="workflow-card-subtitle"><span>{t('chat.lastMessages')}</span></div>
                            {(mainPlayerChatSummary?.example_messages || []).map((msg, idx) => (
                              <div key={`player-chat-example-${idx}`} className="workflow-chat-line">
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
              <div className="chart-empty">{t('player.selectPrompt')}</div>
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
          featureFlags={featureFlags}
          featureFlagsSaving={featureFlagsSaving}
          featureFlagsMessage={featureFlagsMessage}
          featureFlagsMessageIsError={featureFlagsMessageIsError}
          onFeatureFlagToggle={handleFeatureFlagToggle}
          libraryMessage={libraryMessage}
          replayDirInput={replayDirInput}
          savedReplayDir={savedReplayDir}
          librarySettingsLoading={librarySettingsLoading}
          librarySettingsSaving={librarySettingsSaving}
          isSampleSet={isSampleSet}
          detectedReplayDir={detectedReplayDir}
          sampleSetLoading={sampleSetLoading}
          onReplayDirChange={setReplayDirInput}
          onSaveReplayDir={handleSaveReplayDir}
          onLoadSampleSet={handleLoadSampleSet}
          onUseDetectedFolder={handleUseDetectedFolder}
          onDismissMessage={() => setLibraryMessage('')}
        />
      )}

      {sampleNotice ? (
        <div className="ingest-toast ingest-toast-dismissable" role="status">
          <span>{sampleNotice}</span>
          <button
            type="button"
            className="ingest-toast-dismiss"
            aria-label={t('common.dismiss')}
            onClick={() => setSampleNotice('')}
          >
            ×
          </button>
        </div>
      ) : null}

      <div className="app-footer">
        <div className="footer-left">
          {replayCount !== null ? (
            <>
              {t('footer.replaysInDatabase', { count: replayCount.toLocaleString() })}
              <span className="workflow-meta-sep" aria-hidden="true"> · </span>
              <a href="https://github.com/marianogappa/screpdb" target="_blank" rel="noopener noreferrer">screpdb</a>
              {t('footer.by')}
              <a href="https://marianogappa.github.io" target="_blank" rel="noopener noreferrer">Mariano Gappa</a>
              {currentVersion ? (
                <>
                  <span className="workflow-meta-sep" aria-hidden="true"> · </span>
                  <span
                    className="footer-version"
                    title={currentCommit ? t('footer.commit', { commit: currentCommit }) : undefined}
                  >
                    {currentVersion}
                    {currentCommit && currentCommit !== 'unknown' ? ` (${currentCommit})` : ''}
                  </span>
                </>
              ) : null}
              <span className="workflow-meta-sep" aria-hidden="true"> · </span>
              <a href="https://github.com/marianogappa/screpdb/issues/new/choose" target="_blank" rel="noopener noreferrer">{t('footer.reportIssue')}</a>
              {updateAvailable && updateTier === 'quiet' && !quietUpdateDismissed ? (
                <>
                  <span className="workflow-meta-sep" aria-hidden="true"> · </span>
                  <span className="footer-update-nudge">
                    {updateApplied ? (
                      <button type="button" className="footer-update-link" onClick={() => window.location.reload()}>
                        {t('update.updatedRefreshFooter', { version: updateLatest })}
                      </button>
                    ) : selfUpdateSupported ? (
                      <button
                        type="button"
                        className="footer-update-link"
                        disabled={updateApplying}
                        onClick={handleApplyUpdate}
                        data-tip={updateError || t('update.fromTo', { from: currentVersion, to: updateLatest })}
                      >
                        {updateApplying ? t('update.updatingPlain') : t('update.updateTo', { version: updateLatest })}
                      </button>
                    ) : updateManagerCommand ? (
                      <ManagedUpdateHint
                        className="managed-update-hint--footer"
                        latestVersion={updateLatest}
                        command={updateManagerCommand}
                        releaseUrl={updateReleaseUrl}
                      />
                    ) : (
                      <a href={updateReleaseUrl} target="_blank" rel="noopener noreferrer" className="footer-update-link" data-tip={updateUnsupportedTip}>
                        {t('update.availableVersion', { version: updateLatest })}
                      </a>
                    )}
                    {!updateApplied ? (
                      <button
                        type="button"
                        className="footer-update-dismiss"
                        aria-label={t('update.dismiss')}
                        onClick={() => setQuietUpdateDismissed(true)}
                      >
                        ×
                      </button>
                    ) : null}
                  </span>
                  {updateError ? <span className="footer-update-error">{updateError}</span> : null}
                </>
              ) : null}
            </>
          ) : (
            t('footer.loadingReplayCount')
          )}
        </div>
        <div className="footer-right">
          <LanguageSwitcher />
        </div>
      </div>
    </div>
    </CountryFlagOverrideContext.Provider>
  );
}

export default App;
