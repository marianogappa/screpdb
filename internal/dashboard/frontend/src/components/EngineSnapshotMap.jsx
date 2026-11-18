import React, { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useT } from '../lib/i18nContext';
import { api } from '../api';
import { getUnitIcon, normalizeUnitName, spriteKeyOf } from '../lib/gameAssets';
import { formatDuration } from '../lib/formatters';
import { placeLabels } from '../lib/labelPlacement';
import { baseOutline } from '../lib/baseOutline';
import { GOOD_ENOUGH, placeTitle } from '../lib/titlePlacement';
import GameMap from './GameMap';

const TILE = 32;
// scmapanalyzer's sprites are Remastered HD2 art: two sprite pixels per map pixel.
const SPRITE_SCALE = 0.5;
const LIFT_HEIGHT = 24;
const FALLBACK_METRIC = { w: 56, h: 56, ax: 28, ay: 28 };

// Flyers are drawn above everything on the ground, as the game draws air above ground.
const FLYERS = new Set([
  'overlord', 'mutalisk', 'scourge', 'guardian', 'devourer', 'queen', 'wraith', 'dropship', 'sciencevessel', 'battlecruiser',
  'valkyrie', 'scout', 'carrier', 'arbiter', 'corsair', 'shuttle', 'observer', 'interceptor',
]);

// Only the Lurker has a burrowed sprite; other burrowed units are drawn see-through.
const variantOf = (unitType, burrowed) => (burrowed && normalizeUnitName(unitType).endsWith('lurker') ? 'burrowed' : '');

const metricKey = (unitType, burrowed) => {
  const key = spriteKeyOf(unitType);
  const variant = variantOf(unitType, burrowed);
  return key && variant ? `${key}@${variant}` : key;
};

const spriteMetrics = new Map();

// useSpriteMetrics fetches each sprite's size and anchor once per page: the anchor is the pixel the
// unit's position falls on, so drawing at position minus anchor places it as the game does.
function useSpriteMetrics(keys) {
  const [, setLoaded] = useState(0);
  useEffect(() => {
    const missing = keys.filter((k) => k && !spriteMetrics.has(k));
    if (missing.length === 0) return;
    for (const k of missing) spriteMetrics.set(k, null);
    api.getSpriteMetrics(missing)
      .then((metrics) => {
        for (const [k, m] of Object.entries(metrics || {})) spriteMetrics.set(k, m);
        setLoaded((n) => n + 1);
      })
      .catch(() => { for (const k of missing) spriteMetrics.delete(k); });
  }, [keys]);
  return (key) => spriteMetrics.get(key) || null;
}

const naturalWidths = new Map();

function useNaturalWidth(href) {
  const [, setLoaded] = useState(0);
  useEffect(() => {
    if (!href || naturalWidths.has(href)) return;
    naturalWidths.set(href, 0);
    const img = new Image();
    img.onload = () => {
      naturalWidths.set(href, img.naturalWidth);
      setLoaded((n) => n + 1);
    };
    img.src = href;
  }, [href]);
  return href ? naturalWidths.get(href) || 0 : 0;
}

// The wrench of the game's "under construction" status, on a 24-unit grid.
const WRENCH_PATH = 'M22.7 19l-9.1-9.1c.9-2.3.4-5-1.5-6.9-2-2-5-2.4-7.4-1.3L9 6 6 9 1.6 4.7C.4 7.1.9 10.1 2.9 12.1c1.9 1.9 4.6 2.4 6.9 1.5l9.1 9.1c.4.4 1 .4 1.4 0l2.3-2.3c.5-.4.5-1.1.1-1.4z';

// ProgressBar is the game's construction bar: a dark frame, a segmented green fill, a wrench beside it.
function ProgressBar({ cx, top, width, progress, scale, segmentsId }) {
  const h = 7 * scale;
  const icon = 11 * scale;
  const gap = 2 * scale;
  const total = icon + gap + width;
  const x0 = cx - total / 2;
  const barX = x0 + icon + gap;
  const fill = Math.max(0, Math.min(1, progress)) * (width - 2 * scale);
  return (
    <g className="engine-progress" pointerEvents="none">
      <rect x={x0 - scale} y={top - (icon - h) / 2 - scale} width={icon + 2 * scale} height={icon + 2 * scale} rx={2 * scale} fill="#0b0f14" stroke="#5c6670" strokeWidth={scale} />
      <path d={WRENCH_PATH} transform={`translate(${x0 + scale} ${top - (icon - h) / 2 + scale}) scale(${(icon - 2 * scale) / 24})`} fill="#d9dee4" />
      <rect x={barX} y={top} width={width} height={h} fill="#05070a" stroke="#5c6670" strokeWidth={scale} />
      {fill > 0 ? <rect x={barX + scale} y={top + scale} width={fill} height={h - 2 * scale} fill={`url(#${segmentsId})`} /> : null}
    </g>
  );
}

function Sprite({ href, x, y, w, h, opacity = 1, filter, title }) {
  if (!href) return null;
  return (
    <g opacity={opacity}>
      <image href={href} x={x} y={y} width={w} height={h} preserveAspectRatio="none" filter={filter} />
      <title>{title}</title>
    </g>
  );
}

// creepPath turns the creep bits into one path of tile runs, row by row.
const creepPath = (creep) => {
  if (!creep?.bits || !creep.w) return '';
  let bytes;
  try {
    bytes = Uint8Array.from(atob(creep.bits), (c) => c.charCodeAt(0));
  } catch {
    return '';
  }
  const has = (i) => (bytes[i >> 3] >> (i & 7)) & 1;
  const rows = Math.floor((bytes.length * 8) / creep.w);
  const parts = [];
  for (let ty = 0; ty < rows; ty += 1) {
    let run = -1;
    for (let tx = 0; tx <= creep.w; tx += 1) {
      const on = tx < creep.w && has(ty * creep.w + tx);
      if (on && run < 0) run = tx;
      if (!on && run >= 0) {
        parts.push(`M${run * TILE} ${ty * TILE}h${(tx - run) * TILE}v${TILE}h${-(tx - run) * TILE}z`);
        run = -1;
      }
    }
  }
  return parts.join('');
};

// CreepTiles fills creep with the tileset's own creep tiles and draws the game's edge frame on every
// tile that borders it.
function CreepTiles({ id, creep, path, edgesWidth }) {
  const base = `/api/custom/game-assets/creep?tileset=${encodeURIComponent(creep.tileset)}`;
  const edges = creep.edges || [];
  const cells = [];
  if (edgesWidth > 0) {
    for (let i = 0; i + 2 < edges.length; i += 3) {
      const [tx, ty, frame] = [edges[i], edges[i + 1], edges[i + 2]];
      cells.push(
        <svg key={i} x={tx * TILE} y={ty * TILE} width={TILE} height={TILE} viewBox={`${(frame - 1) * TILE} 0 ${TILE} ${TILE}`}>
          <image href={`${base}&kind=edges`} width={edgesWidth} height={TILE} />
        </svg>,
      );
    }
  }
  return (
    <>
      <defs>
        <pattern id={id} width={128} height={128} patternUnits="userSpaceOnUse">
          <image href={`${base}&kind=texture`} width={128} height={128} />
        </pattern>
      </defs>
      <path d={path} fill={`url(#${id})`} />
      {cells}
    </>
  );
}

// CreepDefs paints creep the way the game reads: a dark violet organic mat with ragged edges. The
// ragged edge comes from displacing the tile outline with noise; the texture from a second noise.
function CreepDefs({ id, width, height }) {
  return (
    <filter id={id} filterUnits="userSpaceOnUse" x={0} y={0} width={width} height={height} colorInterpolationFilters="sRGB">
      <feTurbulence type="fractalNoise" baseFrequency="0.09" numOctaves="2" seed="7" result="edgeNoise" />
      <feDisplacementMap in="SourceGraphic" in2="edgeNoise" scale="22" xChannelSelector="R" yChannelSelector="G" result="shape" />
      <feTurbulence type="fractalNoise" baseFrequency="0.16" numOctaves="3" seed="3" result="grain" />
      <feTurbulence type="turbulence" baseFrequency="0.035" numOctaves="2" seed="11" result="veins" />
      <feBlend in="grain" in2="veins" mode="multiply" result="texture" />
      <feColorMatrix
        in="texture"
        type="matrix"
        values="0.38 0 0 0 0.13  0.22 0 0 0 0.08  0.3 0 0 0 0.11  0 0 0 0 1"
        result="tinted"
      />
      <feComposite in="tinted" in2="shape" operator="in" result="creep" />
      <feGaussianBlur in="shape" stdDeviation="3" result="soft" />
      <feComposite in="creep" in2="soft" operator="in" />
    </filter>
  );
}

// terrainCrop is the region grown to whole tiles, for the sharp terrain image the whole-map one lacks.
const terrainCrop = (replayId, region, width, height) => {
  const x = Math.max(0, Math.floor(region.x / TILE));
  const y = Math.max(0, Math.floor(region.y / TILE));
  const w = Math.min(Math.ceil((region.x + region.w) / TILE), Math.floor(width / TILE)) - x;
  const h = Math.min(Math.ceil((region.y + region.h) / TILE), Math.floor(height / TILE)) - y;
  if (w <= 0 || h <= 0) return null;
  const url = `/api/custom/game-assets/map-crop?replay_id=${encodeURIComponent(replayId)}&x=${x}&y=${y}&w=${w}&h=${h}`;
  return {
    url: `${url}&layer=terrain`,
    resourcesUrl: `${url}&layer=resources`,
    x: x * TILE,
    y: y * TILE,
    w: w * TILE,
    h: h * TILE,
  };
};

const clampRegion = (region, width, height) => {
  const x = Math.max(0, Math.min(region.x, width - 1));
  const y = Math.max(0, Math.min(region.y, height - 1));
  return { x, y, w: Math.min(region.w, width - x), h: Math.min(region.h, height - y) };
};

// zoomOut grows a view about its centre, kept inside the map, so a title can find room beside the event.
const zoomOut = (view, factor, width, height) => {
  const w = Math.min(width, view.w * factor);
  const h = Math.min(height, view.h * factor);
  const x = Math.max(0, Math.min(width - w, view.x + view.w / 2 - w / 2));
  const y = Math.max(0, Math.min(height - h, view.y + view.h / 2 - h / 2));
  return { x, y, w, h };
};

const ZOOM_OUT_STEPS = [1, 1.35, 1.7];
const TITLE_FONT_PX = 28;
const TITLE_LINE_PX = 35;
const TITLE_PAD_PX = 12;

let measureCanvas = null;
const textWidth = (text) => {
  if (typeof document === 'undefined') return text.length * TITLE_FONT_PX * 0.55;
  measureCanvas = measureCanvas || document.createElement('canvas');
  const ctx = measureCanvas.getContext('2d');
  ctx.font = `650 ${TITLE_FONT_PX}px ${getComputedStyle(document.body).fontFamily}`;
  return ctx.measureText(text).width;
};

// The Ghost painting a nuke stands within its sight range of the target: 11 tiles with Ocular Implants.
const NUKE_GHOST_REACH = 14 * TILE;

const nukeGhost = (units, playerId, point) => {
  let best = null;
  let bestDist = NUKE_GHOST_REACH;
  for (const u of units || []) {
    if (u.player_id !== playerId || spriteKeyOf(u.unit_type) !== 'ghost') continue;
    const d = Math.hypot(u.x - point.x, u.y - point.y);
    if (d <= bestDist) {
      best = u;
      bestDist = d;
    }
  }
  return best;
};

const polygonCentroid = (polygon) => {
  const n = (polygon || []).length;
  if (n === 0) return null;
  return {
    x: polygon.reduce((sum, p) => sum + p.x, 0) / n,
    y: polygon.reduce((sum, p) => sum + p.y, 0) / n,
  };
};

const polygonOutline = (polygon) => baseOutline(polygon, TILE);

const polygonPoints = (polygon) => polygonOutline(polygon).map((p) => `${p.x},${p.y}`).join(' ');

// The label's size on screen, in CSS pixels: the pill's padding, dot and gap, then the name.
const labelCssSize = (name) => ({ w: 30 + 7.2 * name.length, h: 22 });

const baseKey = (b) => `${b.name}-${b.clock}-${b.polygon[0].x}`;

const baseKindLabel = (t, base, nameOf) => {
  if (base.kind === 'main') return t('events.base.main');
  if (base.kind === 'natural') {
    return base.natural_of && base.natural_of !== base.player_id
      ? t('events.base.naturalOf', { player: nameOf(base.natural_of) })
      : t('events.base.natural');
  }
  return t('events.expansion');
};

function BaseTooltip({ hover, color, nameOf }) {
  const t = useT();
  const { base } = hover;
  const icon = getUnitIcon(base.town_hall);
  return (
    <div className="workflow-event-map-tooltip engine-base-tooltip" style={{ left: `${hover.x}px`, top: `${hover.y}px` }}>
      <span className="workflow-event-map-tooltip-dot" style={{ background: color(base.player_id) }} />
      <span className="workflow-event-map-tooltip-text">
        <strong>{nameOf(base.player_id)}</strong>
        <span className="workflow-event-map-tooltip-base engine-base-tooltip-kind">
          {icon ? <img src={icon} alt="" className="engine-base-tooltip-icon" /> : null}
          {baseKindLabel(t, base, nameOf)}
        </span>
        {base.complete ? null : (
          <span className="workflow-event-map-tooltip-base">{t('snapshot.underConstruction', { unit: base.town_hall })}</span>
        )}
        {base.since != null ? (
          <span className="workflow-event-map-tooltip-base">{t('events.base.since', { time: formatDuration(base.since) })}</span>
        ) : null}
      </span>
    </div>
  );
}

// EngineSnapshotMap draws the game as the engine had it at one moment: creep, shadows, every building (a
// bar with a wrench while it is being built), every unit at its real size and place. It opens zoomed to the event.
// eventSeconds are the snapshot seconds of every event of this game: the first exact request computes
// them all in one engine pass, so moving between events afterwards is instant.
export default function EngineSnapshotMap({ replayId, event, eventSeconds, mapUrl, mapWidth, mapHeight, colorByPlayerId, nameByPlayerId, teamColorByPlayerId, leaveMarker, mapAlt, title, titleText, centerTitle, progress }) {
  const t = useT();
  const [hover, setHover] = useState(null);
  const rootRef = useRef(null);
  const [frameWidth, setFrameWidth] = useState(0);
  useEffect(() => {
    const el = rootRef.current;
    if (!el) return undefined;
    const ro = new ResizeObserver(([entry]) => setFrameWidth(Math.round(entry.contentRect.width / 20) * 20));
    ro.observe(el);
    return () => ro.disconnect();
  }, [mapWidth, mapHeight]);
  const ids = useId().replace(/:/g, '');
  const engine = event?.engine;
  const second = engine?.snapshot_second ?? event?.second ?? 0;
  const [snapshot, setSnapshot] = useState(null);

  useEffect(() => {
    let cancelled = false;
    let exactArrived = false;
    api.getEngineSnapshot(replayId, second)
      .then((data) => { if (!cancelled && !exactArrived) setSnapshot(data); })
      .catch(() => {});
    api.getEngineSnapshot(replayId, second, { exact: true, seconds: eventSeconds || [] })
      .then((data) => {
        if (cancelled) return;
        exactArrived = true;
        setSnapshot(data);
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [replayId, second, eventSeconds]);

  const width = Number(mapWidth) || 0;
  const height = Number(mapHeight) || 0;
  const region = useMemo(() => (engine?.region && width && height ? clampRegion(engine.region, width, height) : null), [engine, width, height]);
  const focus = useMemo(() => (engine?.focus && width && height ? clampRegion(engine.focus, width, height) : null), [engine, width, height]);
  const creep = useMemo(() => creepPath(snapshot?.creep), [snapshot?.creep]);
  const tileset = snapshot?.creep?.tileset || '';
  const edgesWidth = useNaturalWidth(tileset ? `/api/custom/game-assets/creep?tileset=${encodeURIComponent(tileset)}&kind=edges` : '');
  const metricKeys = useMemo(() => [...new Set([
    ...(snapshot?.buildings || []).map((b) => metricKey(b.unit_type, false)),
    ...(snapshot?.units || []).map((u) => metricKey(u.unit_type, u.burrowed)),
  ])].filter(Boolean).sort(), [snapshot]);
  const metricOf = useSpriteMetrics(metricKeys);
  // Owner labels go where they cover the least of the map; placed once per snapshot and frame size, so
  // they hold still while the event is on screen. The title then takes the emptiest patch left, wrapping
  // to fit it, and the view zooms out a step at a time while no patch is empty enough.
  const layout = useMemo(() => {
    const whole = { x: 0, y: 0, w: width, h: height };
    if (!width || !height || !frameWidth) return { view: region, labelSpots: new Map(), title: null };
    const obstacles = [
      ...(snapshot?.buildings || []).map((b) => ({ x: b.x - b.w / 2, y: b.y - b.h / 2, w: b.w, h: b.h })),
      ...(snapshot?.units || []).map((u) => ({ x: u.x - 14, y: u.y - 14, w: 28, h: 28 })),
    ];
    const marks = [
      ...(Array.isArray(event?.path) ? event.path : []),
      ...(event?.type === 'nuke' && engine?.point ? [engine.point] : []),
    ].map((p) => ({ x: p.x - 3 * TILE, y: p.y - 3 * TILE, w: 6 * TILE, h: 6 * TILE }));
    const owned = (snapshot?.bases || []).filter((b) => (b.polygon || []).length >= 3 && b.player_id != null).map((b) => polygonOutline(b.polygon));
    const lineWidth = titleText ? textWidth(titleText) : 0;
    let chosen = null;
    for (const factor of region && titleText && !centerTitle ? ZOOM_OUT_STEPS : [1]) {
      const view = region ? zoomOut(region, factor, width, height) : whole;
      const mapPerCss = view.w / frameWidth;
      const labels = (snapshot?.bases || [])
        .filter((b) => (b.polygon || []).length >= 3 && nameByPlayerId?.get(b.player_id))
        .map((b) => {
          const size = labelCssSize(nameByPlayerId.get(b.player_id));
          return { key: baseKey(b), outline: polygonOutline(b.polygon), w: size.w * mapPerCss, h: size.h * mapPerCss };
        });
      const labelSpots = placeLabels({ labels, obstacles, avoid: focus ? [focus] : [], view, width, height });
      if (!titleText || centerTitle) return { view: region ? view : null, labelSpots, title: null };
      const labelBoxes = labels.filter((l) => labelSpots.has(l.key)).map((l) => {
        const c = labelSpots.get(l.key);
        return { x: c.x - l.w / 2, y: c.y - l.h / 2, w: l.w, h: l.h };
      });
      const placed = placeTitle({
        view, width, height, mapPerCss,
        textWidthCss: lineWidth, lineHeightCss: TITLE_LINE_PX, padCss: TITLE_PAD_PX,
        obstacles: [...obstacles, ...labelBoxes, ...marks], avoid: focus ? [focus] : [], polygons: owned,
      });
      const option = { view: region ? view : null, labelSpots, title: placed };
      if (!chosen || (placed && chosen.title && placed.covered < chosen.title.covered)) chosen = option;
      if (placed && placed.covered <= GOOD_ENOUGH) return option;
    }
    return chosen;
  }, [snapshot, region, focus, width, height, frameWidth, nameByPlayerId, titleText, centerTitle, event?.path, event?.type, engine?.point]);
  const view = layout.view;
  const labelSpots = layout.labelSpots;
  if (!width || !height) return null;
  // A player who left or stopped playing is marked where the event says: the snapshot of that very second
  // may no longer hold any of their bases.
  const leaveBases = leaveMarker ? (snapshot?.bases || []).filter((b) => b.player_id === leaveMarker.playerID && (b.polygon || []).length >= 3) : [];
  const leaveBase = leaveBases.find((b) => b.kind === 'main') || leaveBases[0] || null;
  const leavePoint = engine?.point || (leaveBase ? polygonCentroid(leaveBase.polygon) : event?.base?.center || null);
  const showZoomed = Boolean(view);
  // Terrain, then creep on the floor, then minerals and geysers, then everything else.
  const crop = showZoomed ? terrainCrop(replayId, view, width, height) : null;
  const color = (id) => colorByPlayerId?.get(id) || '#e2e8f0';
  const nameOf = (id) => nameByPlayerId?.get(id) || '';
  const bases = (snapshot?.bases || []).filter((b) => (b.polygon || []).length >= 3);
  // Bars and icons keep a readable size on screen whether the view is a base or the whole map.
  const uiScale = showZoomed ? Math.max(1, view.w / 1100) : Math.max(2.5, width / 1100);
  const segmentsId = `${ids}-seg`;
  const creepId = `${ids}-creep`;
  const warpId = `${ids}-warp`;
  const focusId = `${ids}-focus`;
  const hoverBase = (e, base) => {
    const box = rootRef.current?.getBoundingClientRect();
    if (box) setHover({ base, x: e.clientX - box.left + 14, y: e.clientY - box.top + 14 });
  };

  // place puts an image so its anchor lands on (x, y), at the given map-pixel scale.
  const place = (m, x, y, scale) => ({
    x: x - m.ax * SPRITE_SCALE * scale,
    y: y - m.ay * SPRITE_SCALE * scale,
    w: m.w * SPRITE_SCALE * scale,
    h: m.h * SPRITE_SCALE * scale,
  });

  // The game draws shadows first, then everything on the ground from north to south, then flyers.
  const shadows = [];
  const ground = [];
  const air = [];
  (snapshot?.buildings || []).forEach((b, i) => {
    const m = metricOf(metricKey(b.unit_type, false)) || FALLBACK_METRIC;
    const lift = b.lifted ? LIFT_HEIGHT : 0;
    const box = place(m, b.x, b.y - lift, 1);
    if (m.shadow) {
      const sb = place(m.shadow, b.x, b.y, 1);
      shadows.push(<image key={`bs-${i}`} href={getUnitIcon(b.unit_type, { shadow: true })} x={sb.x} y={sb.y} width={sb.w} height={sb.h} preserveAspectRatio="none" />);
    }
    const building = b.progress != null;
    const title = building ? t('snapshot.underConstruction', { unit: b.unit_type }) : b.unit_type;
    (b.lifted ? air : ground).push({
      y: b.y,
      node: (
        <g key={`b-${i}`}>
          <Sprite
            href={getUnitIcon(b.unit_type, { color: color(b.player_id) })}
            x={box.x}
            y={box.y}
            w={box.w}
            h={box.h}
            filter={building ? `url(#${warpId})` : undefined}
            title={title}
          />
          {building ? (
            <ProgressBar cx={b.x} top={b.y + b.h / 2 + 2 * uiScale} width={Math.min(b.w * 0.8, 110 * uiScale)} progress={b.progress} scale={uiScale} segmentsId={segmentsId} />
          ) : null}
        </g>
      ),
    });
  });

  (snapshot?.units || []).forEach((u, i) => {
    const m = metricOf(metricKey(u.unit_type, u.burrowed)) || FALLBACK_METRIC;
    const box = place(m, u.x, u.y, 1);
    const variant = variantOf(u.unit_type, u.burrowed);
    const burrowedLook = u.burrowed && !variant;
    if (m.shadow && !burrowedLook) {
      const sb = place(m.shadow, u.x, u.y, 1);
      shadows.push(<image key={`us-${i}`} href={getUnitIcon(u.unit_type, { shadow: true, variant })} x={sb.x} y={sb.y} width={sb.w} height={sb.h} preserveAspectRatio="none" />);
    }
    (FLYERS.has(spriteKeyOf(u.unit_type)) ? air : ground).push({
      y: u.y,
      node: (
        <Sprite
          key={`u-${i}`}
          href={getUnitIcon(u.unit_type, { color: color(u.player_id), variant })}
          x={box.x}
          y={box.y}
          w={box.w}
          h={box.h}
          opacity={burrowedLook ? 0.45 : 1}
          title={u.unit_type}
        />
      ),
    });
  });
  const byY = (a, b) => a.y - b.y;
  ground.sort(byY);
  air.sort(byY);
  const nukePoint = event?.type === 'nuke' && engine?.point ? engine.point : null;
  const insightPath = Array.isArray(event?.path) ? event.path : [];
  const pathNumbered = event?.type === 'elevator';
  const pathColor = color(event?.actor?.player_id);
  const ghost = nukePoint ? nukeGhost(snapshot?.units, event?.actor?.player_id, nukePoint) : null;

  return (
    <div className="engine-snapshot" ref={rootRef}>
      <GameMap
        className="engine-snapshot-frame"
        mapUrl={mapUrl}
        mapWidth={width}
        mapHeight={height}
        view={showZoomed ? view : null}
        detail={crop}
        maxHeight="100vh"
        alt={mapAlt}
        mapLayers={(
          <>
            <defs>
              <pattern id={segmentsId} width={4 * uiScale} height={8} patternUnits="userSpaceOnUse">
                <rect width={3 * uiScale} height={8} fill="#38d13c" />
                <rect width={3 * uiScale} height={2 * uiScale} fill="#9cf59a" />
              </pattern>
              {creep && !tileset ? <CreepDefs id={creepId} width={width} height={height} /> : null}
              <filter id={warpId} colorInterpolationFilters="sRGB">
                <feColorMatrix type="matrix" values="0.25 0.25 0.1 0 0.12  0.3 0.35 0.15 0 0.25  0.35 0.4 0.25 0 0.45  0 0 0 0.85 0" />
              </filter>
              {focus ? (
                <radialGradient
                  id={focusId}
                  gradientUnits="userSpaceOnUse"
                  cx={0}
                  cy={0}
                  r={1}
                  gradientTransform={`translate(${focus.x + focus.w / 2} ${focus.y + focus.h / 2}) scale(${focus.w * 0.75 + 2 * TILE} ${focus.h * 0.75 + 2 * TILE})`}
                >
                  <stop offset="0.55" stopColor="#05070c" stopOpacity="0" />
                  <stop offset="1" stopColor="#05070c" stopOpacity="0.5" />
                </radialGradient>
              ) : null}
            </defs>
            {creep && tileset ? <CreepTiles id={creepId} creep={snapshot.creep} path={creep} edgesWidth={showZoomed ? edgesWidth : 0} /> : null}
            {creep && !tileset ? <path d={creep} fill="#000" filter={`url(#${creepId})`} opacity={0.92} /> : null}
            {crop ? <image href={crop.resourcesUrl} x={crop.x} y={crop.y} width={crop.w} height={crop.h} preserveAspectRatio="none" /> : null}
            {bases.map((b) => (
              <polygon
                key={`base-${b.name}-${b.clock}-${b.polygon[0].x}`}
                className={`engine-base${b.complete ? '' : ' engine-base--building'}`}
                points={polygonPoints(b.polygon)}
                style={{ fill: color(b.player_id), stroke: teamColorByPlayerId?.get(b.player_id) || color(b.player_id), strokeWidth: (teamColorByPlayerId ? 7 : 3) * uiScale, strokeDasharray: b.complete ? undefined : `${12 * uiScale} ${8 * uiScale}` }}
              />
            ))}
            {shadows}
            {ground.map((g) => g.node)}
            {air.map((a) => a.node)}
            {focus ? <rect className="engine-focus-dim" width={width} height={height} fill={`url(#${focusId})`} /> : null}
            {ghost ? (
              <g className="engine-nuke-ghost" pointerEvents="none">
                <line x1={ghost.x} y1={ghost.y} x2={nukePoint.x} y2={nukePoint.y} stroke="#ff2b2b" strokeOpacity={0.7} strokeWidth={2 * uiScale} strokeDasharray={`${6 * uiScale} ${5 * uiScale}`} />
                <circle className="engine-nuke-ghost-ring" cx={ghost.x} cy={ghost.y} r={Math.max(24, 18 * uiScale)} strokeWidth={3 * uiScale} />
              </g>
            ) : null}
            {nukePoint ? (
              <g className={`engine-nuke-target${event?.result === 'cancelled' || event?.result === 'ghost_killed' ? ' engine-nuke-target--failed' : ''}`} pointerEvents="none">
                <circle className="engine-nuke-target-wave" cx={nukePoint.x} cy={nukePoint.y} r={24 * uiScale} strokeWidth={4 * uiScale} />
                <circle cx={nukePoint.x} cy={nukePoint.y} r={24 * uiScale} fill="#ff1a1a" fillOpacity={0.3} />
                <circle cx={nukePoint.x} cy={nukePoint.y} r={12 * uiScale} fill="#ff1a1a" stroke="#fff" strokeWidth={2.5 * uiScale} />
              </g>
            ) : null}
            {insightPath.length > 0 ? (
              <g className="engine-insight-path" pointerEvents="none">
                {pathNumbered && insightPath.length > 1 ? (
                  <polyline
                    points={insightPath.map((p) => `${p.x},${p.y}`).join(' ')}
                    fill="none"
                    stroke={pathColor}
                    strokeOpacity={0.8}
                    strokeWidth={2.5 * uiScale}
                    strokeDasharray={`${7 * uiScale} ${5 * uiScale}`}
                  />
                ) : null}
                {insightPath.map((p, i) => (pathNumbered ? (
                  <g key={`hop-${i}`}>
                    <circle cx={p.x} cy={p.y} r={11 * uiScale} fill={pathColor} stroke="#fff" strokeWidth={2 * uiScale} />
                    <text x={p.x} y={p.y} fontSize={12 * uiScale}>{i + 1}</text>
                  </g>
                ) : (
                  <path
                    key={`kill-${i}`}
                    d={`M${p.x - 7 * uiScale},${p.y - 7 * uiScale}L${p.x + 7 * uiScale},${p.y + 7 * uiScale}M${p.x + 7 * uiScale},${p.y - 7 * uiScale}L${p.x - 7 * uiScale},${p.y + 7 * uiScale}`}
                    stroke="#ff3b3b"
                    strokeWidth={3 * uiScale}
                    strokeLinecap="round"
                  />
                )))}
              </g>
            ) : null}
            {bases.map((b) => (
              <polygon
                key={`base-hit-${b.name}-${b.clock}-${b.polygon[0].x}`}
                className="engine-base-hit"
                points={polygonPoints(b.polygon)}
                onMouseEnter={(e) => hoverBase(e, b)}
                onMouseMove={(e) => hoverBase(e, b)}
                onMouseLeave={() => setHover(null)}
              />
            ))}
          </>
        )}
      >
        {leaveMarker && leavePoint ? (
          <div className="workflow-event-map-leave-marker" style={{ left: `${(leavePoint.x / width) * 100}%`, top: `${(leavePoint.y / height) * 100}%` }}>
            <div
              className="engine-leave-marker"
              style={{ transform: `translateX(${leavePoint.x < width * 0.15 ? '50%' : leavePoint.x > width * 0.85 ? '-50%' : '0'})` }}
            >
              <span className="workflow-event-map-leave-emoji" role="img" aria-label={leaveMarker.label}>{leaveMarker.emoji}</span>
              <span className="workflow-event-map-leave-name map-pill" style={{ borderColor: leaveMarker.color }}>
                <span className="map-pill-dot" style={{ background: leaveMarker.color }} />
                {leaveMarker.name}
              </span>
            </div>
          </div>
        ) : null}
        {bases.map((b) => {
          const c = labelSpots.get(baseKey(b));
          return c && nameOf(b.player_id) && b !== leaveBase ? (
            <span
              key={`base-label-${baseKey(b)}`}
              className="engine-base-label map-pill"
              style={{ left: `${(c.x / width) * 100}%`, top: `${(c.y / height) * 100}%`, borderColor: color(b.player_id) }}
            >
              <span className="map-pill-dot" style={{ background: color(b.player_id) }} />
              {nameOf(b.player_id)}
            </span>
          ) : null;
        })}
      </GameMap>
      {title && centerTitle ? <div className="engine-title engine-title--center">{title}</div> : null}
      {title && !centerTitle && layout.title ? (
        <div
          className="engine-title"
          style={{ left: `${layout.title.x}px`, top: `${layout.title.y}px`, width: `${layout.title.w}px`, padding: `${TITLE_PAD_PX}px` }}
        >
          {title}
        </div>
      ) : null}
      {progress}
      {hover ? <BaseTooltip hover={hover} color={color} nameOf={nameOf} /> : null}
    </div>
  );
}
