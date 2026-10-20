import React, { useEffect, useId, useMemo, useState } from 'react';
import { useT } from '../lib/i18nContext';
import { formatDuration } from '../lib/formatters';
import { api } from '../api';
import { getUnitIcon, normalizeUnitName } from '../lib/gameAssets';
import GameMap from './GameMap';

const TILE = 32;
// scmapanalyzer's sprites are Remastered HD2 art: two sprite pixels per map pixel. Drawing each at half
// its own size puts every unit and building at its real size, whatever it is.
const SPRITE_SCALE = 0.5;
// These four came from other art at other scales; until scmapanalyzer redraws them, each is drawn at the
// height (map pixels) the game gives it.
const SPRITE_HEIGHT_OVERRIDE = { goliath: 46, darktemplar: 34, archon: 64, darkarchon: 60 };
// The whole map is shown far smaller, so there units are drawn 2.5 times larger to stay visible.
const WHOLE_MAP_UNIT_SCALE = 2.5;
const LIFT_HEIGHT = 24;

const spriteKey = (unitType) => normalizeUnitName(unitType).replace(/^(terran|zerg|protoss)/, '');

// Only the Lurker has a burrowed sprite; other burrowed units are drawn see-through.
const variantOf = (unitType, burrowed) => (burrowed && normalizeUnitName(unitType).endsWith('lurker') ? 'burrowed' : '');

const spriteSizes = new Map();

// useSpriteSizes learns each sprite's own size the first time it is drawn; sizes do not depend on color.
function useSpriteSizes(hrefsByKey) {
  const [, setLoaded] = useState(0);
  useEffect(() => {
    let live = true;
    for (const [key, href] of hrefsByKey) {
      if (spriteSizes.has(key)) continue;
      spriteSizes.set(key, null);
      const img = new Image();
      img.onload = () => {
        spriteSizes.set(key, { w: img.naturalWidth, h: img.naturalHeight });
        if (live) setLoaded((n) => n + 1);
      };
      img.onerror = () => spriteSizes.delete(key);
      img.src = href;
    }
    return () => { live = false; };
  }, [hrefsByKey]);
  return (key) => spriteSizes.get(key) || null;
}

const drawSize = (key, natural) => {
  if (!natural) return null;
  const override = SPRITE_HEIGHT_OVERRIDE[key];
  if (override) return { w: (natural.w * override) / natural.h, h: override };
  return { w: natural.w * SPRITE_SCALE, h: natural.h * SPRITE_SCALE };
};

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

function Sprite({ href, x, y, w, h, opacity = 1, title }) {
  if (!href) return null;
  return (
    <g opacity={opacity}>
      <image href={href} x={x} y={y} width={w} height={h} preserveAspectRatio="none" />
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

// EngineSnapshotMap draws the game as the engine had it at one moment: creep, every building (a bar with a
// wrench while it is being built), every unit at its real size. It opens zoomed to the event.
// eventSeconds are the snapshot seconds of every event of this game: the first exact request computes
// them all in one engine pass, so moving between events afterwards is instant.
export default function EngineSnapshotMap({ replayId, event, eventSeconds, mapUrl, mapWidth, mapHeight, colorByPlayerId, mapAlt }) {
  const t = useT();
  const ids = useId().replace(/:/g, '');
  const engine = event?.engine;
  const second = engine?.snapshot_second ?? event?.second ?? 0;
  const [snapshot, setSnapshot] = useState(null);
  const [failed, setFailed] = useState(false);
  const [zoomed, setZoomed] = useState(true);

  useEffect(() => {
    let cancelled = false;
    let exactArrived = false;
    setFailed(false);
    api.getEngineSnapshot(replayId, second)
      .then((data) => { if (!cancelled && !exactArrived) setSnapshot(data); })
      .catch(() => { if (!cancelled && !exactArrived) setFailed(true); });
    api.getEngineSnapshot(replayId, second, { exact: true, seconds: eventSeconds || [] })
      .then((data) => {
        if (cancelled) return;
        exactArrived = true;
        setSnapshot(data);
        setFailed(false);
      })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [replayId, second, eventSeconds]);

  useEffect(() => { setZoomed(true); }, [event]);

  const width = Number(mapWidth) || 0;
  const height = Number(mapHeight) || 0;
  const region = useMemo(() => (engine?.region && width && height ? clampRegion(engine.region, width, height) : null), [engine, width, height]);
  const creep = useMemo(() => creepPath(snapshot?.creep), [snapshot?.creep]);
  const hrefsByKey = useMemo(() => {
    const out = new Map();
    for (const item of [...(snapshot?.buildings || []), ...(snapshot?.units || [])]) {
      const key = spriteKey(item.unit_type);
      if (!out.has(key)) {
        const href = getUnitIcon(item.unit_type);
        if (href) out.set(key, href);
      }
    }
    return out;
  }, [snapshot]);
  const naturalSize = useSpriteSizes(hrefsByKey);
  if (!width || !height) return null;
  const showZoomed = zoomed && region;
  const color = (id) => colorByPlayerId?.get(id) || '#e2e8f0';
  // Bars and icons keep a readable size on screen whether the view is a base or the whole map.
  const uiScale = showZoomed ? Math.max(1, region.w / 1100) : Math.max(2.5, width / 1100);
  const segmentsId = `${ids}-seg`;
  const creepId = `${ids}-creep`;

  const buildings = (snapshot?.buildings || []).map((b, i) => {
    const key = spriteKey(b.unit_type);
    const size = drawSize(key, naturalSize(key)) || { w: b.w, h: b.h };
    const scale = showZoomed ? 1 : 1.25;
    const w = size.w * scale;
    const h = size.h * scale;
    const footBottom = b.y + (b.h * scale) / 2;
    const lift = b.lifted ? LIFT_HEIGHT : 0;
    const building = b.progress != null;
    const title = building ? t('snapshot.underConstruction', { unit: b.unit_type }) : b.unit_type;
    return (
      <g key={`b-${i}`}>
        {b.lifted ? <ellipse cx={b.x} cy={footBottom - (b.h * scale) / 4} rx={w / 3} ry={h / 8} fill="#000" opacity={0.35} /> : null}
        <Sprite href={getUnitIcon(b.unit_type, { color: color(b.player_id) })} x={b.x - w / 2} y={footBottom - h - lift} w={w} h={h} title={title} />
        {building ? (
          <ProgressBar cx={b.x} top={footBottom + 2 * uiScale} width={Math.min(b.w * scale * 0.8, 110 * uiScale)} progress={b.progress} scale={uiScale} segmentsId={segmentsId} />
        ) : null}
      </g>
    );
  });

  const units = (snapshot?.units || []).map((u, i) => {
    const key = spriteKey(u.unit_type);
    const natural = drawSize(key, naturalSize(key)) || { w: 28, h: 28 };
    const scale = showZoomed ? 1 : WHOLE_MAP_UNIT_SCALE;
    const w = natural.w * scale;
    const h = natural.h * scale;
    const burrowedLook = u.burrowed && !variantOf(u.unit_type, true);
    return (
      <Sprite
        key={`u-${i}`}
        href={getUnitIcon(u.unit_type, { color: color(u.player_id), variant: variantOf(u.unit_type, u.burrowed) })}
        x={u.x - w / 2}
        y={u.y - h / 2}
        w={w}
        h={h}
        opacity={burrowedLook ? 0.45 : 1}
        title={u.unit_type}
      />
    );
  });

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
              {creep ? <CreepDefs id={creepId} width={width} height={height} /> : null}
            </defs>
            {creep ? <path d={creep} fill="#000" filter={`url(#${creepId})`} opacity={0.92} /> : null}
            {!showZoomed && region ? (
              <rect x={region.x} y={region.y} width={region.w} height={region.h} className="engine-snapshot-region" strokeWidth={24} />
            ) : null}
            {buildings}
            {units}
          </>
        )}
      />
      <div className="engine-snapshot-meta">
        <span>
          {failed
            ? t('snapshot.unavailable')
            : !snapshot
              ? t('snapshot.loading')
              : snapshot.exact
                ? t('snapshot.exactCaption', { time: formatDuration(snapshot.second) })
                : t('snapshot.caption', { time: formatDuration(snapshot.second), armyTime: formatDuration(Math.max(0, snapshot.army_second)) })}
        </span>
        {region ? (
          <button type="button" className="btn-switch engine-snapshot-zoom" onClick={() => setZoomed((z) => !z)}>
            {zoomed ? t('snapshot.wholeMap') : t('snapshot.zoomIn')}
          </button>
        ) : null}
      </div>
    </div>
  );
}
