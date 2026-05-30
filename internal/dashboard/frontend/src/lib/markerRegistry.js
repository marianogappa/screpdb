// markerRegistry loads the backend-authored Pill metadata once per session and
// exposes a small surface for rendering marker pills without the per-marker
// branches that used to live in App.jsx. Backend source of truth is
// internal/patterns/markers/definitions.go; wire-shape comes from
// /api/custom/markers/definitions (handlerMarkersDefinitions).

import { useEffect, useState } from 'react';
import { api } from '../api';
import { getUnitIcon, normalizeUnitName } from './gameAssets';

// PILL_SURFACES enumerates the four render sites a marker may appear on.
// Keys match the JSON fields emitted by the backend endpoint.
export const PILL_SURFACES = Object.freeze({
  summaryPlayer: 'summary_player',
  summaryReplay: 'summary_replay',
  gamesList:     'games_list',
  eventsList:    'events_list',
});

// stripTemporalPlaceholders removes the "{minute}" / "{timestamp}" tokens
// and the surrounding " at min", " at  mins", " min" stems from a label.
// Used by the aggregate surface where there's no single timestamp to
// interpolate (the marker fired across many games at varying times).
const stripTemporalPlaceholders = (template) => {
  if (!template) return '';
  let out = template;
  // " at min {minute}" / " at min {timestamp}" / " at {minute} mins" etc.
  out = out.replace(/\s+at\s+min(?:ute)?s?\s*\{(?:minute|timestamp)\}/gi, '');
  out = out.replace(/\s+at\s+\{(?:minute|timestamp)\}\s*min(?:ute)?s?/gi, '');
  out = out.replace(/\s*\{(?:minute|timestamp)\}\s*/g, ' ');
  return out.trim();
};

// interpolatePlaceholders resolves {subject}, {minute}, and {timestamp} in a template string.
// {subject} reads the marker's payload (JSON blob) via the definition's Subject;
// {minute} comes from detected_second divided by 60 (integer);
// {timestamp} formats detected_second as M:SS.
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

// resolveSubject runs the Subject resolver declared by the marker definition.
// Static subjects return their configured Value; payload_field subjects read
// the named field and stringify it (joining arrays with ",").
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
      if (Array.isArray(raw)) return raw.join(',');
      if (raw != null) return String(raw);
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

// renderPillText computes the final displayed label + icon-key for a (marker,
// surface, row) triple. Returns null when the surface has no pill declared.
export const renderPillText = (definition, surface, row) => {
  if (!definition) return null;
  const pill = definition[surface];
  if (!pill) return null;

  const subject   = resolveSubject(pill.subject, row?.payload);
  const minute    = minuteFromSecond(row?.detected_second);
  const timestamp = timestampFromSecond(row?.detected_second);

  const label   = interpolatePlaceholders(pill.label, { subject, minute, timestamp });
  const iconKey = interpolatePlaceholders(pill.icon_key, { subject, minute, timestamp });

  return {
    label,
    iconKey,
    icon: iconKey ? getUnitIcon(iconKey) : null,
    style: pill.style || '',
    title: pill.title || '',
  };
};

// renderAggregatePillText computes the label/icon for a marker on the
// aggregate Summary-tab cards (per-matchup, per-format) where there's no
// single replay context. Priority:
//   1. games_list pill — its labels are already temporal-free and the
//      shortest user-facing form ("Recalls", "Nukes").
//   2. definition.name — backend-friendly noun phrase ("Made drops",
//      "Became Terran"). Used when there's no games_list and the
//      summary_player label only contains placeholders/race tokens that
//      strip down to something less descriptive than the Name.
//   3. summary_player with temporal placeholders stripped — final fallback
//      for markers without a games_list / Name that fits.
//
// Style/title come from the chosen surface; icon resolves via the chosen
// surface's icon_key (or stays null when none was set).
export const renderAggregatePillText = (definition) => {
  if (!definition) return null;
  const gl = definition.games_list;
  if (gl && gl.label) {
    const iconKey = stripTemporalPlaceholders(gl.icon_key || '');
    return {
      label: stripTemporalPlaceholders(gl.label),
      iconKey,
      icon: iconKey ? getUnitIcon(iconKey) : null,
      style: gl.style || '',
      title: gl.title || '',
    };
  }
  const sp = definition.summary_player;
  const spIcon = sp ? stripTemporalPlaceholders(sp.icon_key || '') : '';
  if (definition.name) {
    return {
      label: definition.name,
      iconKey: spIcon,
      icon: spIcon ? getUnitIcon(spIcon) : null,
      style: sp?.style || '',
      title: sp?.title || '',
    };
  }
  if (sp && sp.label) {
    return {
      label: stripTemporalPlaceholders(sp.label),
      iconKey: spIcon,
      icon: spIcon ? getUnitIcon(spIcon) : null,
      style: sp.style || '',
      title: sp.title || '',
    };
  }
  return null;
};

// pillClassName maps a backend PillStyle to the existing CSS classes. Keeps the
// styling table small and explicit so adding a new style requires one edit here.
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

// isBuildOrderEventType reports whether an event_type (== marker FeatureKey)
// belongs to an opening build order. All initial-BO FeatureKeys are prefixed
// "bo_" (including the residual "bo_*_other" catch-alls); the "opener_unresolved"
// N/A marker is deliberately NOT one — it keeps its plain absence styling.
export const isBuildOrderEventType = (eventType) =>
  typeof eventType === 'string' && eventType.startsWith('bo_');

// isOpenerEventType is the "fills the opener slot" predicate: a real build
// order OR the "opener unresolved" (N/A) marker. Used for ordering (the opener
// always leads the row) and for the "BUILD ORDER" legend.
export const isOpenerEventType = (eventType) =>
  isBuildOrderEventType(eventType) || eventType === 'opener_unresolved';

// pillEventTypeClass returns an extra CSS class that distinguishes a pill by
// what it represents (independent of its backend PillStyle): build orders, the
// unresolved-opener N/A pill, and the "used hotkeys" pill each get their own
// minimal treatment. Returns '' for everything else.
export const pillEventTypeClass = (eventType) => {
  if (isBuildOrderEventType(eventType)) return 'workflow-pattern-pill-bo';
  if (eventType === 'opener_unresolved') return 'workflow-pattern-pill-na';
  if (eventType === 'used_hotkey_groups') return 'workflow-pattern-pill-keys';
  return '';
};

// useMarkerRegistry fetches /api/custom/markers/definitions once on mount and
// exposes the full payload to consumers: markers keyed by FeatureKey, plus the
// ordered featuring key list and the game-event-only feature metadata used by
// the featuring-chip strip. Stable across a session (bumped only when
// AlgorithmVersion changes on the backend).
export const useMarkerRegistry = () => {
  const [state, setState] = useState({
    markers: {},
    featuring_order: [],
    game_event_features: [],
    algorithmVersion: 0,
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
          algorithmVersion: Number(resp?.algorithm_version) || 0,
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

// lookupDefinitionForPattern resolves a detected_patterns[] row to its backend
// definition. Tries the canonical event_type first, then falls back to a
// normalized pattern_name lookup for rows emitted by older codepaths.
export const lookupDefinitionForPattern = (registry, pattern) => {
  if (!registry || !pattern) return null;
  const byEventType = pattern.event_type ? registry[pattern.event_type] : null;
  if (byEventType) return byEventType;

  // Fallback: some older endpoints still pass only pattern_name. Scan the
  // registry for a case-insensitive name match.
  const normalized = normalizeUnitName(pattern.pattern_name);
  if (!normalized) return null;
  for (const key of Object.keys(registry)) {
    const def = registry[key];
    if (normalizeUnitName(def?.name) === normalized) return def;
  }
  return null;
};
