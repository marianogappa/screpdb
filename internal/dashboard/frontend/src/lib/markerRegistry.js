// markerRegistry loads the backend-authored Pill metadata once per session, so
// pill rendering needs none of the per-marker branches that used to live in
// App.jsx. Source of truth is internal/patterns/markers/definitions.go, served
// by /api/custom/markers/definitions.

import { useEffect, useState } from 'react';
import { api } from '../api';
import { getUnitIcon, normalizeUnitName } from './gameAssets';
import { t } from './i18nContext';

export const markerName = (definition) => {
  if (!definition) return '';
  return t.server(`server.marker.${definition.feature_key}.name`, definition.name || '');
};

const pillText = (definition, surface, field) => {
  const pill = definition?.[surface];
  if (!pill) return '';
  return t.server(`server.marker.${definition.feature_key}.${surface}.${field}`, pill[field] || '');
};

// Keys match the JSON fields the backend endpoint emits.
export const PILL_SURFACES = Object.freeze({
  summaryPlayer: 'summary_player',
  summaryReplay: 'summary_replay',
  gamesList:     'games_list',
  eventsList:    'events_list',
});

// Used by the aggregate surface, where there is no single timestamp to
// interpolate: the marker fired across many games at varying times.
const stripTemporalPlaceholders = (template) => {
  if (!template) return '';
  let out = template;
  out = out.replace(/\s+at\s+min(?:ute)?s?\s*\{(?:minute|timestamp)\}/gi, '');
  out = out.replace(/\s+at\s+\{(?:minute|timestamp)\}\s*min(?:ute)?s?/gi, '');
  out = out.replace(/\s*\{(?:minute|timestamp)\}\s*분?\s*(?:에|경|쯤)?/g, ' ');
  out = out.replace(/\s*\{(?:minute|timestamp)\}\s*/g, ' ');
  return out.trim();
};

// {subject} reads the marker's payload via the definition's Subject, {minute} is
// detected_second / 60, and {timestamp} formats detected_second as M:SS.
const interpolatePlaceholders = (template, { subject, minute, timestamp }) => {
  if (!template) return '';
  let out = template;
  if (out.includes('{subject}')) {
    out = out.split('{subject}').join(subject == null ? '' : String(subject));
  }
  if (out.includes('{minute}')) {
    out = out.split('{minute}').join(minute == null ? '' : String(minute));
  }
  if (out.includes('{timestamp}')) {
    out = out.split('{timestamp}').join(timestamp == null ? '' : String(timestamp));
  }
  return out;
};

// Static subjects return their configured Value; payload_field subjects read the
// named field and stringify it, joining arrays with ",".
const resolveSubject = (subjectDef, payload) => {
  if (!subjectDef) return '';
  if (subjectDef.kind === 'static') return subjectDef.value || '';
  if (subjectDef.kind === 'payload_field' && subjectDef.field) {
    let parsed = payload;
    if (typeof payload === 'string' && payload.length > 0) {
      try { parsed = JSON.parse(payload); } catch (err) { parsed = null; }
    }
    if (parsed && typeof parsed === 'object') {
      const raw = parsed[subjectDef.field];
      if (Array.isArray(raw)) return raw.map((item) => t.buildLabel(String(item))).join(',');
      if (raw != null) return t.buildLabel(String(raw));
    }
  }
  return '';
};

const minuteFromSecond = (second) => {
  if (!Number.isFinite(Number(second))) return null;
  return Math.floor(Number(second) / 60);
};

const timestampFromSecond = (second) => {
  if (!Number.isFinite(Number(second))) return null;
  const total = Math.max(0, Math.floor(Number(second)));
  const m = Math.floor(total / 60);
  const s = String(total % 60).padStart(2, '0');
  return `${m}:${s}`;
};

// Returns null when the surface has no pill declared.
export const renderPillText = (definition, surface, row) => {
  if (!definition) return null;
  const pill = definition[surface];
  if (!pill) return null;

  const subject   = resolveSubject(pill.subject, row?.payload);
  const minute    = minuteFromSecond(row?.detected_second);
  const timestamp = timestampFromSecond(row?.detected_second);

  const label   = interpolatePlaceholders(pillText(definition, surface, 'label'), { subject, minute, timestamp });
  const iconKey = interpolatePlaceholders(pill.icon_key, { subject, minute, timestamp });

  return {
    label,
    iconKey,
    icon: iconKey ? getUnitIcon(iconKey) : null,
    style: pill.style || '',
    title: pillText(definition, surface, 'title'),
  };
};

// renderAggregatePillText computes the label/icon for the aggregate Summary-tab
// cards, where there is no single replay context. In priority order:
//
//   1. games_list — already temporal-free and the shortest user-facing form.
//   2. definition.name — used when there is no games_list and summary_player
//      strips down to something less descriptive than the Name.
//   3. summary_player with temporal placeholders stripped.
export const renderAggregatePillText = (definition) => {
  if (!definition) return null;
  const gl = definition.games_list;
  if (gl && gl.label) {
    const iconKey = stripTemporalPlaceholders(gl.icon_key || '');
    return {
      label: stripTemporalPlaceholders(pillText(definition, 'games_list', 'label')),
      iconKey,
      icon: iconKey ? getUnitIcon(iconKey) : null,
      style: gl.style || '',
      title: pillText(definition, 'games_list', 'title'),
    };
  }
  const sp = definition.summary_player;
  const spIcon = sp ? stripTemporalPlaceholders(sp.icon_key || '') : '';
  if (definition.name) {
    return {
      label: markerName(definition),
      iconKey: spIcon,
      icon: spIcon ? getUnitIcon(spIcon) : null,
      style: sp?.style || '',
      title: pillText(definition, 'summary_player', 'title'),
    };
  }
  if (sp && sp.label) {
    return {
      label: stripTemporalPlaceholders(pillText(definition, 'summary_player', 'label')),
      iconKey: spIcon,
      icon: spIcon ? getUnitIcon(spIcon) : null,
      style: sp.style || '',
      title: pillText(definition, 'summary_player', 'title'),
    };
  }
  return null;
};

// Plain-language on purpose: this isn't a research tool.
export const betaTooltip = () => t('marker.betaTooltip');

// Uncurated means no human has verified the detection (see GOLDEN_TIERS.md).
// The backend sends `curated: false` for those; game-event-only features carry
// no `curated` field and are never flagged.
export const featureIsBeta = (definition) =>
  !!definition && definition.curated === false;

// Keeps the styling table small and explicit, so a new style is one edit here.
export const pillClassName = (style) => {
  switch (style) {
    case 'strong':
      return 'workflow-pattern-pill workflow-pattern-pill-strong';
    case 'negative':
      return 'workflow-pattern-pill workflow-low-usage-pill workflow-low-usage-pill-hotkey';
    case 'inline':
      return 'workflow-pattern-pill workflow-pattern-pill-inline';
    default:
      return 'workflow-pattern-pill';
  }
};

// All initial-BO FeatureKeys are prefixed "bo_", including the residual
// "bo_*_other" catch-alls. "opener_unresolved" is deliberately NOT one — it
// keeps its plain absence styling.
export const isBuildOrderEventType = (eventType) =>
  typeof eventType === 'string' && eventType.startsWith('bo_');

// isOpenerEventType is the "fills the opener slot" predicate: a real build order
// OR the unresolved-opener marker. Drives ordering and the legend.
export const isOpenerEventType = (eventType) =>
  isBuildOrderEventType(eventType) || eventType === 'opener_unresolved';

// An extra class distinguishing a pill by what it REPRESENTS, independent of its
// backend PillStyle. Returns '' for everything else.
export const pillEventTypeClass = (eventType) => {
  if (isBuildOrderEventType(eventType)) return 'workflow-pattern-pill-bo';
  if (eventType === 'opener_unresolved') return 'workflow-pattern-pill-na';
  return '';
};

// Fetched once on mount and stable for the session.
export const useMarkerRegistry = () => {
  const [state, setState] = useState({
    markers: {},
    featuring_order: [],
    game_event_features: [],
    loading: true,
    error: null,
  });

  useEffect(() => {
    let cancelled = false;
    api.getMarkerDefinitions()
      .then((resp) => {
        if (cancelled) return;
        setState({
          markers: resp?.markers || {},
          featuring_order: Array.isArray(resp?.featuring_order) ? resp.featuring_order : [],
          game_event_features: Array.isArray(resp?.game_event_features) ? resp.game_event_features : [],
          loading: false,
          error: null,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setState((prev) => ({ ...prev, loading: false, error: err }));
      });
    return () => { cancelled = true; };
  }, []);

  return state;
};

// Tries the canonical event_type first, then a normalized pattern_name lookup
// for rows emitted by older codepaths.
export const lookupDefinitionForPattern = (registry, pattern) => {
  if (!registry || !pattern) return null;
  const byEventType = pattern.event_type ? registry[pattern.event_type] : null;
  if (byEventType) return byEventType;

  // Some older endpoints pass only pattern_name.
  const normalized = normalizeUnitName(pattern.pattern_name);
  if (!normalized) return null;
  for (const key of Object.keys(registry)) {
    const def = registry[key];
    if (normalizeUnitName(def?.name) === normalized) return def;
  }
  return null;
};
