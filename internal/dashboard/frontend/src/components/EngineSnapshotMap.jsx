import React, { useEffect, useMemo, useState } from 'react';
import { useT } from '../lib/i18nContext';
import { formatDuration } from '../lib/formatters';
import { api } from '../api';

// Radii are in map pixels: real size when zoomed, enlarged on the whole map so a unit stays visible.
const UNIT_RADIUS = 10;
const UNIT_RADIUS_WHOLE_MAP = 24;

const clampRegion = (region, width, height) => {
  const x = Math.max(0, Math.min(region.x, width - 1));
  const y = Math.max(0, Math.min(region.y, height - 1));
  return { x, y, w: Math.min(region.w, width - x), h: Math.min(region.h, height - y) };
};

// EngineSnapshotMap draws the game as the engine had it at one moment: every building standing (faded
// while under construction) and the army from the nearest army sample. It opens zoomed to the event.
export default function EngineSnapshotMap({ replayId, event, mapUrl, mapWidth, mapHeight, colorByPlayerId, mapAlt }) {
  const t = useT();
  const engine = event?.engine;
  const second = engine?.snapshot_second ?? event?.second ?? 0;
  const [snapshot, setSnapshot] = useState(null);
  const [failed, setFailed] = useState(false);
  const [zoomed, setZoomed] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setFailed(false);
    api.getEngineSnapshot(replayId, second)
      .then((data) => { if (!cancelled) setSnapshot(data); })
      .catch(() => { if (!cancelled) setFailed(true); });
    return () => { cancelled = true; };
  }, [replayId, second]);

  useEffect(() => { setZoomed(true); }, [event]);

  const width = Number(mapWidth) || 0;
  const height = Number(mapHeight) || 0;
  const region = useMemo(() => (engine?.region && width && height ? clampRegion(engine.region, width, height) : null), [engine, width, height]);
  if (!width || !height) return null;
  const showZoomed = zoomed && region;
  const view = showZoomed ? region : { x: 0, y: 0, w: width, h: height };
  const scale = view.w / width;
  const color = (id) => colorByPlayerId?.get(id) || '#e2e8f0';

  return (
    <div className="engine-snapshot">
      <div className="engine-snapshot-frame" style={{ aspectRatio: `${view.w} / ${view.h}` }}>
        <svg viewBox={`${view.x} ${view.y} ${view.w} ${view.h}`} preserveAspectRatio="xMidYMid meet" role="img" aria-label={mapAlt}>
          <image href={mapUrl} x="0" y="0" width={width} height={height} preserveAspectRatio="none" />
          {!showZoomed && region ? (
            <rect x={region.x} y={region.y} width={region.w} height={region.h} className="engine-snapshot-region" strokeWidth={24} />
          ) : null}
          {(snapshot?.buildings || []).map((b, i) => (
            <rect
              key={`b-${i}`}
              x={b.x - b.w / 2}
              y={b.y - b.h / 2}
              width={b.w}
              height={b.h}
              rx={4}
              fill={color(b.player_id)}
              fillOpacity={b.complete ? 0.75 : 0.3}
              stroke={color(b.player_id)}
              strokeWidth={(b.town_hall ? 10 : 4) * Math.max(scale, 0.35)}
              strokeDasharray={b.complete ? undefined : '12 8'}
            >
              <title>{b.complete ? b.unit_type : t('snapshot.underConstruction', { unit: b.unit_type })}</title>
            </rect>
          ))}
          {(snapshot?.units || []).map((u, i) => (
            <circle
              key={`u-${i}`}
              cx={u.x}
              cy={u.y}
              r={showZoomed ? UNIT_RADIUS : UNIT_RADIUS_WHOLE_MAP}
              fill={color(u.player_id)}
              stroke="rgba(0,0,0,0.7)"
              strokeWidth={2}
            >
              <title>{u.unit_type}</title>
            </circle>
          ))}
          {engine?.point ? (
            <circle cx={engine.point.x} cy={engine.point.y} r={showZoomed ? 70 : 140} className="engine-snapshot-point" strokeWidth={showZoomed ? 8 : 20} />
          ) : null}
        </svg>
      </div>
      <div className="engine-snapshot-meta">
        <span>
          {failed
            ? t('snapshot.unavailable')
            : snapshot
              ? t('snapshot.caption', { time: formatDuration(snapshot.second), armyTime: formatDuration(Math.max(0, snapshot.army_second)) })
              : t('snapshot.loading')}
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
