import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { getUnitIcon } from '../../lib/gameAssets';
import { useT } from '../../lib/i18nContext';
import { slugKey } from '../../lib/i18n';
import { getStoredHotkeyView, saveHotkeyView } from '../../lib/dashboardStorage';

// Hotkey timeline: one row per hotkey group per player, the whole game fitted to the width. Building assigns render the actual building sprite,
// unit assigns a race silhouette with the selection count, and the tick
// texture shows uses tinted by what the group held at that moment.

// Fit to viewport scales rows between these heights; the default is the roomiest.
const MIN_ROW_HEIGHT = 20;
const MAX_ROW_HEIGHT = 32;
const AXIS_LABEL_MIN_PX = 44;
// Assigns within this many pixels of the pointer share one hover card, so clumped marks stay readable.
const HOVER_RADIUS_PX = 10;
// Keycap column plus grid gap, subtracted from the container to size lanes.
const LANE_INSET = 40;

const CATEGORY_COLORS = {
  hall: '#3987e5',
  prod: '#d95926',
  tech: '#199e70',
  comsat: '#c98500',
  units: '#8b98a4',
};

const HALLS = new Set(['Hatchery', 'Lair', 'Hive', 'Command Center', 'Nexus']);
const PROD = new Set(['Gateway', 'Barracks', 'Factory', 'Starport', 'Stargate', 'Robotics Facility']);

export const hotkeyCategoryOf = (building) => {
  if (building === 'Comsat Station') return 'comsat';
  if (HALLS.has(building)) return 'hall';
  if (PROD.has(building)) return 'prod';
  return 'tech';
};

const UNIT_SILHOUETTE = { Zerg: 'Zergling', Terran: 'Marine', Protoss: 'Zealot' };

// Sprite lookup: gameAssets resolves most names; Comsat Station only resolves
// as "Comsat".
export const hotkeyBuildingIcon = (building) =>
  getUnitIcon(building === 'Comsat Station' ? 'Comsat' : building);

// Count ramp: 1 to 12 units, dim to bright blue.
const RAMP_LOW = [0x2e, 0x4f, 0x6e];
const RAMP_HIGH = [0xa5, 0xd4, 0xff];
export const hotkeyCountColor = (count) => {
  const t = Math.max(0, Math.min(11, (count || 1) - 1)) / 11;
  const c = RAMP_LOW.map((lo, i) => Math.round(lo + (RAMP_HIGH[i] - lo) * t));
  return `rgb(${c[0]},${c[1]},${c[2]})`;
};

const mmss = (sec) => `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`;

// Events are [sec, type, group, count, buildingID, tileX, tileY, unitType, unitCount] tuples; the last two
// come from the engine (unitCount 0 without it; unitType 0 is a Marine): the group's main unit and how many units it held.
export const EV_SEC = 0;
export const EV_TYPE = 1;
export const EV_GROUP = 2;
export const EV_COUNT = 3;
export const EV_BUILDING = 4;
export const EV_TILE_X = 5;
export const EV_TILE_Y = 6;
export const EV_UNIT_TYPE = 7;
export const EV_UNIT_COUNT = 8;
export const TYPE_ASSIGN_UNITS = 1;
export const TYPE_ASSIGN_BUILDING = 3;

const KEYBOARD_GROUPS = [1, 2, 3, 4, 5, 6, 7, 8, 9, 0];

function TicksCanvas({ events, buildingNames, totalW, pxPerMin, rowHeight }) {
  const ref = useRef(null);
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    canvas.width = totalW;
    canvas.height = rowHeight;
    const ctx = canvas.getContext('2d');
    ctx.clearRect(0, 0, totalW, rowHeight);
    ctx.strokeStyle = 'rgba(120, 130, 140, 0.28)';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(0, rowHeight - 0.5);
    ctx.lineTo(totalW, rowHeight - 0.5);
    ctx.stroke();

    // Recall ticks leave the row's top edge free for the plain assign squares.
    const inset = Math.max(7, Math.round(rowHeight * 0.3));
    let current = null;
    for (const e of events) {
      const x = (e[EV_SEC] / 60) * pxPerMin;
      if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING) {
        const cat = hotkeyCategoryOf(buildingNames[e[EV_BUILDING]] || '');
        current = { color: CATEGORY_COLORS[cat] };
      } else if (e[EV_TYPE] === TYPE_ASSIGN_UNITS) {
        current = { color: hotkeyCountColor(e[EV_COUNT]) };
      } else {
        ctx.strokeStyle = current ? current.color : 'rgba(139,152,164,0.6)';
        ctx.globalAlpha = current ? 0.75 : 0.4;
        const xi = Math.round(x) + 0.5;
        ctx.beginPath();
        ctx.moveTo(xi, inset);
        ctx.lineTo(xi, rowHeight - inset);
        ctx.stroke();
        ctx.globalAlpha = 1;
      }
    }
  }, [events, buildingNames, totalW, pxPerMin, rowHeight]);
  return <canvas ref={ref} className="hk-ticks" aria-hidden="true" />;
}

const isAssign = (e) => e[EV_TYPE] === TYPE_ASSIGN_UNITS || e[EV_TYPE] === TYPE_ASSIGN_BUILDING;

// assignUnits is what a unit assign held as [unitType, count] pairs: the engine's full composition when the
// digest has it, else its main unit, else nothing (the stream only knows the count).
const assignUnits = (e, composition) => {
  if (composition?.length) return composition;
  if (e[EV_UNIT_COUNT] > 0) return [[e[EV_UNIT_TYPE], e[EV_UNIT_COUNT]]];
  return null;
};

const sameAssign = (a, b) => a.e[EV_TYPE] === b.e[EV_TYPE]
  && a.e[EV_BUILDING] === b.e[EV_BUILDING]
  && JSON.stringify(a.units) === JSON.stringify(b.units)
  && (a.units || a.e[EV_TYPE] === TYPE_ASSIGN_BUILDING || Math.min(a.e[EV_COUNT], 12) === Math.min(b.e[EV_COUNT], 12));

// hoverEntries gathers the row's assigns near the pointer and keeps one entry per run of the same assign,
// since players re-press ctrl+number.
const hoverEntries = (events, compositions, indexOf, sec, pxPerMin) => {
  const radiusSec = (HOVER_RADIUS_PX / pxPerMin) * 60;
  const near = events
    .filter((e) => isAssign(e) && Math.abs(e[EV_SEC] - sec) <= radiusSec)
    .map((e) => ({ e, units: e[EV_TYPE] === TYPE_ASSIGN_UNITS ? assignUnits(e, compositions?.[indexOf.get(e)]) : null }));
  const folded = [];
  for (const entry of near) {
    const last = folded[folded.length - 1];
    if (!last || !sameAssign(last, entry)) folded.push(entry);
  }
  return folded;
};

function HoverCard({ hover, buildingNames, unitNames, race }) {
  const t = useT();
  const flipX = hover.x > window.innerWidth - 280;
  return (
    <div
      className="hk-hover"
      style={{ left: `${hover.x + (flipX ? -12 : 12)}px`, top: `${hover.y + 14}px`, transform: flipX ? 'translateX(-100%)' : undefined }}
    >
      {hover.entries.map(({ e, units }) => {
        let body;
        if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING) {
          const name = buildingNames[e[EV_BUILDING]] || '';
          const icon = hotkeyBuildingIcon(name);
          body = (
            <>
              {icon ? <img src={icon} alt="" /> : null}
              <span>{name ? t.server(`server.name.${slugKey(name)}`, name) : t('chart.hotkey.buildingFallback')}</span>
            </>
          );
        } else if (units) {
          body = units.map(([type, count]) => (
            <span key={type} className="hk-hover-unit" title={unitNames[type] ? t.server(`server.name.${slugKey(unitNames[type])}`, unitNames[type]) : undefined}>
              <img src={getUnitIcon(unitNames[type] || '')} alt={unitNames[type] || ''} />
              <b>×{count}</b>
            </span>
          ));
        } else {
          body = (
            <span className="hk-hover-unit">
              <img src={getUnitIcon(UNIT_SILHOUETTE[race] || 'Marine')} alt="" className="hk-silhouette" />
              <b>×{Math.min(e[EV_COUNT], 12)}</b>
            </span>
          );
        }
        return (
          <div key={`${e[EV_SEC]}-${e[EV_TYPE]}-${e[EV_BUILDING]}`} className="hk-hover-row">
            <span className="hk-hover-time">{mmss(e[EV_SEC])}</span>
            {body}
          </div>
        );
      })}
    </div>
  );
}

export default function HotkeyTimeline({ payload }) {
  const t = useT();
  const players = payload?.players || [];
  const buildingNames = payload?.buildings || {};
  const unitNames = payload?.units || {};
  const durationSeconds = Math.max(60, payload?.duration_seconds || 60);
  const minutesTotal = durationSeconds / 60;

  const containerRef = useRef(null);
  const [view, setView] = useState(getStoredHotkeyView);
  const setOption = (key) => (ev) => {
    const next = { ...view, [key]: ev.target.checked };
    setView(next);
    saveHotkeyView(next);
  };
  const [rowHeight, setRowHeight] = useState(MAX_ROW_HEIGHT);
  const [laneWidth, setLaneWidth] = useState(0);
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return undefined;
    const measure = () => setLaneWidth(Math.max(0, el.clientWidth - LANE_INSET));
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  const pxPerMin = laneWidth > 0 ? laneWidth / minutesTotal : 1;
  const totalW = Math.ceil(minutesTotal * pxPerMin);

  const [hover, setHover] = useState(null);
  const onRowHover = (ev, row, events) => {
    const rect = ev.currentTarget.getBoundingClientRect();
    const sec = ((ev.clientX - rect.left) / pxPerMin) * 60;
    const entries = hoverEntries(events, row.player.compositions, row.indexOf, sec, pxPerMin);
    setHover(entries.length ? { x: ev.clientX, y: ev.clientY, entries, row } : null);
  };

  const rowsByPlayer = useMemo(() => players
    .map((p) => {
      const byGroup = new Map(KEYBOARD_GROUPS.map((g) => [g, []]));
      const indexOf = new Map();
      let used = false;
      (p.events || []).forEach((e, i) => {
        if (e[EV_GROUP] > 9) return;
        byGroup.get(e[EV_GROUP]).push(e);
        indexOf.set(e, i);
        used = true;
      });
      return { player: p, groups: KEYBOARD_GROUPS, byGroup, indexOf, used };
    })
    .filter(({ used }) => used), [players]);

  const rowCount = rowsByPlayer.length * KEYBOARD_GROUPS.length;
  const canFit = rowsByPlayer.length === 2;
  const fit = canFit && view.fit;
  // Everything but the rows (page header, player names, axes, options) stays the same height, so the
  // rows get whatever the viewport has left after it.
  useLayoutEffect(() => {
    if (!fit) {
      setRowHeight(MAX_ROW_HEIGHT);
      return undefined;
    }
    const fitRows = () => {
      const el = containerRef.current;
      if (!el || !rowCount) return;
      const rect = el.getBoundingClientRect();
      const rest = rect.height - rowCount * rowHeight;
      const available = window.innerHeight - (rect.top + window.scrollY) - rest - 8;
      const next = Math.min(MAX_ROW_HEIGHT, Math.max(MIN_ROW_HEIGHT, Math.floor(available / rowCount)));
      if (next !== rowHeight) setRowHeight(next);
    };
    fitRows();
    window.addEventListener('resize', fitRows);
    return () => window.removeEventListener('resize', fitRows);
  }, [fit, rowCount, rowHeight]);
  const scale = (rowHeight - MIN_ROW_HEIGHT) / (MAX_ROW_HEIGHT - MIN_ROW_HEIGHT);
  const sizeVars = {
    '--hk-row': `${rowHeight}px`,
    '--hk-building': `${Math.round(18 + 6 * scale)}px`,
    '--hk-unit': `${Math.round(15 + 3 * scale)}px`,
    '--hk-key-font': `${(10.5 + 1.5 * scale).toFixed(1)}px`,
  };

  const axisLabels = [];
  const minutes = Math.floor(durationSeconds / 60);
  const step = [1, 2, 4, 5, 10, 15, 20, 30].find((m) => m * pxPerMin >= AXIS_LABEL_MIN_PX) || 60;
  for (let m = 0; m <= minutes; m += step) axisLabels.push(m);

  if (!rowsByPlayer.length) {
    return <div className="chart-empty">{t('chart.hotkey.empty')}</div>;
  }

  const axisRow = (key) => (
    <div key={key} className="hk-row hk-axis-row">
      <div />
      <div className="hk-viewport hk-axis">
        <div className="hk-track" style={{ width: `${totalW}px` }}>
          {axisLabels.map((m) => (
            <span key={m} className="hk-axis-label" style={{ left: `${m * pxPerMin}px` }}>{t('chart.hotkey.minuteTick', { minute: m })}</span>
          ))}
        </div>
      </div>
    </div>
  );

  return (
    <div className="hk-timeline" ref={containerRef} style={sizeVars}>
      {payload?.unit_types_exact ? null : (
        <div className="workflow-section-warning hk-warning">
          {t('chart.hotkey.selectionWarning')}
        </div>
      )}
      {hover ? <HoverCard hover={hover} buildingNames={buildingNames} unitNames={unitNames} race={hover.row.player.race} /> : null}
      {rowsByPlayer.map((row) => {
        const { player, groups, byGroup } = row;
        return (
        <div key={player.player_id} className="hk-player">
          <div className="hk-player-head">
            <span className="hk-player-name">{player.name}</span>
            {player.legacy ? <span className="hk-player-stat">{t('chart.hotkey.legacyStream')}</span> : null}
          </div>
          {groups.map((g) => {
            const events = byGroup.get(g);
            return (
              <div key={g} className="hk-row">
                <div className="hk-keycap">{g}</div>
                <div className="hk-viewport" onPointerMove={(ev) => onRowHover(ev, row, events)} onPointerLeave={() => setHover(null)}>
                  <div className="hk-track" style={{ width: `${totalW}px` }}>
                    <TicksCanvas
                      events={events}
                      buildingNames={buildingNames}
                      totalW={totalW}
                      pxPerMin={pxPerMin}
                      rowHeight={rowHeight}
                    />
                    {events.map((e, i) => {
                      const x = (e[EV_SEC] / 60) * pxPerMin;
                      if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING) {
                        const name = buildingNames[e[EV_BUILDING]] || '';
                        const icon = hotkeyBuildingIcon(name);
                        const displayName = name
                          ? t.server(`server.name.${slugKey(name)}`, name)
                          : t('chart.hotkey.buildingFallback');
                        const label = t('chart.hotkey.assignBuildingTitle', { time: mmss(e[EV_SEC]), group: g, name: displayName });
                        if (view.noImages || !icon) {
                          return (
                            <span
                              key={i}
                              aria-label={label}
                              className="hk-mark-square"
                              style={{ left: `${x}px`, background: CATEGORY_COLORS[hotkeyCategoryOf(name)] }}
                            />
                          );
                        }
                        return (
                          <img
                            key={i}
                            src={icon}
                            alt={label}
                            className="hk-mark-building"
                            style={{ left: `${x}px` }}
                          />
                        );
                      }
                      if (e[EV_TYPE] === TYPE_ASSIGN_UNITS) {
                        const unitName = e[EV_UNIT_COUNT] > 0 ? unitNames[e[EV_UNIT_TYPE]] : null;
                        // Streams encoded before the 12-unit cap can carry
                        // stale-tag inflated counts; clamp at render too.
                        const n = unitName ? e[EV_UNIT_COUNT] : Math.min(e[EV_COUNT], 12);
                        const unitsLabel = t.plural('chart.hotkey.assignUnitsTitle', n, { time: mmss(e[EV_SEC]), group: g });
                        if (view.noImages) {
                          return (
                            <span
                              key={i}
                              aria-label={unitsLabel}
                              className="hk-mark-square"
                              style={{ left: `${x}px`, background: hotkeyCountColor(n) }}
                            />
                          );
                        }
                        return (
                          <span
                            key={i}
                            className="hk-mark-units"
                            style={{ left: `${x}px` }}
                            aria-label={unitsLabel}
                          >
                            {unitName
                              ? <img src={getUnitIcon(unitName)} alt="" />
                              : <img src={getUnitIcon(UNIT_SILHOUETTE[player.race] || 'Marine')} alt="" className="hk-silhouette" />}
                            <i className="hk-count" style={{ background: hotkeyCountColor(n) }}>{n}</i>
                          </span>
                        );
                      }
                      return null;
                    })}
                  </div>
                </div>
              </div>
            );
          })}
          {axisRow(`axis-${player.player_id}`)}
        </div>
        );
      })}
      <div className="hk-options">
        {canFit ? (
          <label>
            <input type="checkbox" checked={!!view.fit} onChange={setOption('fit')} />
            {t('chart.hotkey.fitToViewport')}
          </label>
        ) : null}
        <label>
          <input type="checkbox" checked={!!view.noImages} onChange={setOption('noImages')} />
          {t('chart.hotkey.removeImages')}
        </label>
      </div>
    </div>
  );
}
