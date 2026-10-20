import React, { useEffect, useId, useMemo, useState } from 'react';
import { useT } from '../lib/i18nContext';
import { api } from '../api';
import { getUnitIcon, normalizeUnitName, spriteKeyOf } from '../lib/gameAssets';
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
  return {
    url: `/api/custom/game-assets/map-crop?replay_id=${encodeURIComponent(replayId)}&x=${x}&y=${y}&w=${w}&h=${h}`,
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

// EngineSnapshotMap draws the game as the engine had it at one moment: creep, shadows, every building (a
// bar with a wrench while it is being built), every unit at its real size and place. It opens zoomed to the event.
// eventSeconds are the snapshot seconds of every event of this game: the first exact request computes
// them all in one engine pass, so moving between events afterwards is instant.
export default function EngineSnapshotMap({ replayId, event, eventSeconds, mapUrl, mapWidth, mapHeight, colorByPlayerId, mapAlt }) {
  const t = useT();
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
  const creep = useMemo(() => creepPath(snapshot?.creep), [snapshot?.creep]);
  const tileset = snapshot?.creep?.tileset || '';
  const edgesWidth = useNaturalWidth(tileset ? `/api/custom/game-assets/creep?tileset=${encodeURIComponent(tileset)}&kind=edges` : '');
  const metricKeys = useMemo(() => [...new Set([
    ...(snapshot?.buildings || []).map((b) => metricKey(b.unit_type, false)),
    ...(snapshot?.units || []).map((u) => metricKey(u.unit_type, u.burrowed)),
  ])].filter(Boolean).sort(), [snapshot]);
  const metricOf = useSpriteMetrics(metricKeys);
  if (!width || !height) return null;
  const showZoomed = Boolean(region);
  const color = (id) => colorByPlayerId?.get(id) || '#e2e8f0';
  // Bars and icons keep a readable size on screen whether the view is a base or the whole map.
  const uiScale = showZoomed ? Math.max(1, region.w / 1100) : Math.max(2.5, width / 1100);
  const segmentsId = `${ids}-seg`;
  const creepId = `${ids}-creep`;
  const warpId = `${ids}-warp`;

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

  return (
    <div className="engine-snapshot">
      <GameMap
        className="engine-snapshot-frame"
        mapUrl={mapUrl}
        mapWidth={width}
        mapHeight={height}
        view={showZoomed ? region : null}
        detail={showZoomed ? terrainCrop(replayId, region, width, height) : null}
        maxHeight="100vh - 28px"
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
            </defs>
            {creep && tileset ? <CreepTiles id={creepId} creep={snapshot.creep} path={creep} edgesWidth={showZoomed ? edgesWidth : 0} /> : null}
            {creep && !tileset ? <path d={creep} fill="#000" filter={`url(#${creepId})`} opacity={0.92} /> : null}
            {shadows}
            {ground.map((g) => g.node)}
            {air.map((a) => a.node)}
          </>
        )}
      />
    </div>
  );
}
