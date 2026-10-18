import React, { useEffect, useMemo, useState } from 'react';
import { useT } from '../lib/i18nContext';
import { formatDuration } from '../lib/formatters';
import { api } from '../api';
import { getUnitIcon, normalizeUnitName } from '../lib/gameAssets';
import GameMap from './GameMap';

// Sprite boxes are in map pixels. scmapanalyzer's sprites are drawn at twice the map's scale, so a box
// of half their size puts each unit at its real size: 36 px holds a Marine or a Zergling, 72 px an
// Ultralisk or a Carrier. The whole map is shown far smaller, so there everything is drawn 2.5 times larger.
const LARGE_UNITS = new Set([
  'ultralisk', 'archon', 'darkarchon', 'carrier', 'battlecruiser', 'arbiter', 'guardian', 'devourer', 'reaver',
  'siegetanktankmode', 'siegetanksiegemode', 'dropship', 'shuttle', 'overlord', 'sciencevessel', 'scout', 'queen', 'defiler', 'lurker',
]);
const UNIT_BOX = 36;
const LARGE_UNIT_BOX = 72;
const WHOLE_MAP_SCALE = 2.5;

const unitBox = (unitType) => (LARGE_UNITS.has(normalizeUnitName(unitType).replace(/^(terran|zerg|protoss)/, '')) ? LARGE_UNIT_BOX : UNIT_BOX);

// Only the Lurker has a burrowed sprite; other burrowed units are drawn standing.
const variantOf = (unitType, burrowed) => (burrowed && normalizeUnitName(unitType).endsWith('lurker') ? 'burrowed' : '');

function Sprite({ unitType, x, y, w, h, color, faded, burrowed, title }) {
  const href = getUnitIcon(unitType, { color, variant: variantOf(unitType, burrowed) });
  return (
    <g opacity={faded ? 0.45 : 1}>
      {href ? <image href={href} x={x - w / 2} y={y - h / 2} width={w} height={h} preserveAspectRatio="xMidYMid meet" /> : null}
      <title>{title}</title>
    </g>
  );
}

const clampRegion = (region, width, height) => {
  const x = Math.max(0, Math.min(region.x, width - 1));
  const y = Math.max(0, Math.min(region.y, height - 1));
  return { x, y, w: Math.min(region.w, width - x), h: Math.min(region.h, height - y) };
};

// EngineSnapshotMap draws the game as the engine had it at one moment: every building standing (faded
// while under construction) and the army from the nearest army sample. It opens zoomed to the event.
// eventSeconds are the snapshot seconds of every event of this game: the first exact request computes
// them all in one engine pass, so moving between events afterwards is instant.
export default function EngineSnapshotMap({ replayId, event, eventSeconds, mapUrl, mapWidth, mapHeight, colorByPlayerId, mapAlt }) {
  const t = useT();
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
  if (!width || !height) return null;
  const showZoomed = zoomed && region;
  const color = (id) => colorByPlayerId?.get(id) || '#e2e8f0';

  return (
    <div className="engine-snapshot">
      <GameMap
        className="engine-snapshot-frame"
        mapUrl={mapUrl}
        mapWidth={width}
        mapHeight={height}
        view={showZoomed ? region : null}
        maxHeight="100vh - 28px"
        alt={mapAlt}
        mapLayers={(
          <>
            {!showZoomed && region ? (
              <rect x={region.x} y={region.y} width={region.w} height={region.h} className="engine-snapshot-region" strokeWidth={24} />
            ) : null}
            {(snapshot?.buildings || []).map((b, i) => (
              <Sprite
                key={`b-${i}`}
                unitType={b.unit_type}
                x={b.x}
                y={b.y}
                w={b.w * (showZoomed ? 1 : WHOLE_MAP_SCALE / 2)}
                h={b.h * (showZoomed ? 1 : WHOLE_MAP_SCALE / 2)}
                color={color(b.player_id)}
                faded={!b.complete}
                title={b.complete ? b.unit_type : t('snapshot.underConstruction', { unit: b.unit_type })}
              />
            ))}
            {(snapshot?.units || []).map((u, i) => {
              const size = unitBox(u.unit_type) * (showZoomed ? 1 : WHOLE_MAP_SCALE);
              return (
                <Sprite key={`u-${i}`} unitType={u.unit_type} x={u.x} y={u.y} w={size} h={size} color={color(u.player_id)} burrowed={u.burrowed} title={u.unit_type} />
              );
            })}
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
