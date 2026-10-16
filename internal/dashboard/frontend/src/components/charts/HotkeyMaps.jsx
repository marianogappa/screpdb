import React, { useEffect, useMemo, useState } from 'react';
import {
  EV_SEC, EV_TYPE, EV_GROUP, EV_TILE_X, TYPE_ASSIGN_UNITS, TYPE_ASSIGN_BUILDING,
} from './HotkeyTimeline';
import { useT } from '../../lib/i18nContext';
import { getUnitIcon } from '../../lib/gameAssets';
import GameMap from '../GameMap';

// Per-player map crops: the slice of the map holding the player's hotkeyed buildings at a chosen moment.
// The server says what to draw; GameMap draws it. The slider notches at building-assign moments (at most
// one per minute); when several assigns land in the same minute, the moment showing the most buildings wins.

const TILE_UNKNOWN = 255;
const TILE = 32;
const CHIP = 26;
const CHIP_GAP = 4;

const hotkeyMapDataURL = (replayId, playerId, second) =>
  `/api/custom/hotkeys/map-data?replay_id=${encodeURIComponent(replayId)}&player_id=${encodeURIComponent(playerId)}&second=${encodeURIComponent(second)}`;

const terrainCropURL = (replayId, crop) =>
  `/api/custom/game-assets/map-crop?replay_id=${encodeURIComponent(replayId)}&x=${crop.x / TILE}&y=${crop.y / TILE}&w=${crop.w / TILE}&h=${crop.h / TILE}`;

// locatedBuildingsAt counts the distinct groups whose latest assign at or
// before the cutoff is a located building, mirroring the backend reducer.
const locatedBuildingsAt = (events, cutoffSec) => {
  const last = new Map();
  for (const e of events) {
    if (e[EV_SEC] > cutoffSec || e[EV_GROUP] > 9) continue;
    if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING || e[EV_TYPE] === TYPE_ASSIGN_UNITS) {
      last.set(e[EV_GROUP], e);
    }
  }
  let n = 0;
  for (const e of last.values()) {
    if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING && e[EV_TILE_X] !== TILE_UNKNOWN) n += 1;
  }
  return n;
};

// snapshotNotches picks one moment per minute: each located building assign is
// a candidate, and within a minute the candidate whose snapshot shows the most
// buildings wins (later moment breaks ties), so a quickly replaced hotkey
// still surfaces at its best moment.
export const snapshotNotches = (events) => {
  const byMinute = new Map();
  for (const e of events || []) {
    if (e[EV_TYPE] !== TYPE_ASSIGN_BUILDING || e[EV_TILE_X] === TILE_UNKNOWN || e[EV_GROUP] > 9) continue;
    const sec = e[EV_SEC];
    const minute = Math.floor(sec / 60);
    const count = locatedBuildingsAt(events, sec);
    const cur = byMinute.get(minute);
    if (!cur || count > cur.count || (count === cur.count && sec > cur.sec)) {
      byMinute.set(minute, { sec, minute, count });
    }
  }
  return [...byMinute.values()].sort((a, b) => a.sec - b.sec);
};

function HotkeyedBuilding({ building }) {
  const { x, y, w, h } = building.footprint;
  const href = getUnitIcon(building.building);
  const rowW = building.groups.length * CHIP + (building.groups.length - 1) * CHIP_GAP;
  const chipX = x + (w - rowW) / 2;
  const chipY = y - CHIP / 2;
  return (
    <g className="hk-map-building">
      {href ? <image href={href} x={x} y={y + h - 2 * h} width={w} height={2 * h} preserveAspectRatio="xMidYMax meet" /> : null}
      <rect className="hk-map-footprint" x={x + 1} y={y + 1} width={w - 2} height={h - 2} rx="3" />
      {building.groups.map((group, i) => (
        <g key={group} className="hk-map-chip" transform={`translate(${chipX + i * (CHIP + CHIP_GAP)} ${chipY})`}>
          <rect width={CHIP} height={CHIP} rx="5" />
          <text x={CHIP / 2} y={CHIP / 2} dominantBaseline="central" textAnchor="middle">{group}</text>
        </g>
      ))}
    </g>
  );
}

function HotkeyMapPanel({ replayId, player, notches, mapUrl, mapWidth, mapHeight }) {
  const t = useT();
  const defaultIndex = useMemo(() => {
    let best = notches.length - 1;
    for (let i = 0; i < notches.length; i += 1) {
      if (notches[i].minute <= 8) best = i;
    }
    return best;
  }, [notches]);
  const [index, setIndex] = useState(defaultIndex);
  const [data, setData] = useState(null);
  const [failed, setFailed] = useState(false);
  const notch = notches[Math.min(index, notches.length - 1)];
  useEffect(() => {
    let cancelled = false;
    fetch(hotkeyMapDataURL(replayId, player.player_id, notch.sec))
      .then((res) => (res.ok ? res.json() : Promise.reject(new Error(String(res.status)))))
      .then((body) => { if (!cancelled) { setData(body); setFailed(false); } })
      .catch(() => { if (!cancelled) setFailed(true); });
    return () => { cancelled = true; };
  }, [replayId, player.player_id, notch.sec]);
  const crop = data?.crop;
  return (
    <div className="hk-map-panel">
      <div className="hk-map-head">
        <span className="hk-player-name">{player.name}</span>
        {notches.length > 1 ? (
          <input
            type="range"
            min="0"
            max={notches.length - 1}
            step="1"
            value={Math.min(index, notches.length - 1)}
            onChange={(e) => setIndex(Number(e.target.value))}
            aria-label={t('chart.hotkey.snapshotAria', { name: player.name })}
          />
        ) : null}
      </div>
      {failed || !mapUrl ? (
        <div className="chart-empty">{t('chart.hotkey.mapEmpty')}</div>
      ) : crop ? (
        <GameMap
          className="hk-map"
          mapUrl={mapUrl}
          mapWidth={mapWidth}
          mapHeight={mapHeight}
          view={crop}
          maxHeight="75vh"
          detail={{ url: terrainCropURL(replayId, crop), ...crop }}
          alt={t('chart.hotkey.mapAlt', { name: player.name, minute: notch.minute })}
          mapLayers={data.buildings.map((b) => <HotkeyedBuilding key={`${b.footprint.x},${b.footprint.y}`} building={b} />)}
        />
      ) : null}
    </div>
  );
}

export default function HotkeyMaps({ replayId, players, mapUrl, mapWidth, mapHeight }) {
  const panels = useMemo(() => (players || [])
    .filter((p) => (p.events || []).length > 0 && !p.legacy)
    .map((p) => ({ player: p, notches: snapshotNotches(p.events) }))
    .filter(({ notches }) => notches.length > 0), [players]);
  if (!panels.length) return null;
  return (
    <div className="hk-maps">
      <div className="hk-maps-grid">
        {panels.map(({ player, notches }) => (
          <HotkeyMapPanel
            key={player.player_id}
            replayId={replayId}
            player={player}
            notches={notches}
            mapUrl={mapUrl}
            mapWidth={mapWidth}
            mapHeight={mapHeight}
          />
        ))}
      </div>
    </div>
  );
}
