import React, { useEffect, useMemo, useRef } from 'react';
import { getUnitIcon } from '../../lib/gameAssets';

// Hotkey timeline: one row per hotkey group per player, both players sharing
// one horizontal scroll. Building assigns render the actual building sprite,
// unit assigns a race silhouette with the selection count, and the tick
// texture shows uses tinted by what the group held at that moment.

const PX_PER_MIN = 84;
const ROW_HEIGHT = 40;

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

// Count ramp: 1 -> 12+ units, dim to bright blue.
const RAMP_LOW = [0x2e, 0x4f, 0x6e];
const RAMP_HIGH = [0xa5, 0xd4, 0xff];
export const hotkeyCountColor = (count) => {
  const t = Math.max(0, Math.min(11, (count || 1) - 1)) / 11;
  const c = RAMP_LOW.map((lo, i) => Math.round(lo + (RAMP_HIGH[i] - lo) * t));
  return `rgb(${c[0]},${c[1]},${c[2]})`;
};

const mmss = (sec) => `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`;

// Events are [sec, type, group, count, buildingID, tileX, tileY] tuples.
const EV_SEC = 0;
const EV_TYPE = 1;
const EV_GROUP = 2;
const EV_COUNT = 3;
const EV_BUILDING = 4;
const TYPE_ASSIGN_UNITS = 1;
const TYPE_ASSIGN_BUILDING = 3;

const keyboardOrder = (g) => (g === 0 ? 10 : g);

const rowSummary = (events, buildingNames) => {
  let uses = 0;
  const counts = [];
  const bldgCounts = new Map();
  let assigns = 0;
  for (const e of events) {
    if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING) {
      assigns += 1;
      const name = buildingNames[e[EV_BUILDING]] || 'building';
      bldgCounts.set(name, (bldgCounts.get(name) || 0) + 1);
    } else if (e[EV_TYPE] === TYPE_ASSIGN_UNITS) {
      assigns += 1;
      if (e[EV_COUNT] > 0) counts.push(e[EV_COUNT]);
    } else {
      uses += 1;
    }
  }
  const bldgTotal = [...bldgCounts.values()].reduce((a, b) => a + b, 0);
  if (bldgTotal > 0 && bldgTotal >= assigns * 0.6) {
    let best = '';
    let bestN = 0;
    for (const [name, n] of bldgCounts) {
      if (n > bestN) { best = name; bestN = n; }
    }
    return `${best === 'Comsat Station' ? 'Comsat' : best} · ${uses} uses`;
  }
  if (assigns === 0) return `${uses} uses`;
  counts.sort((a, b) => a - b);
  const median = counts.length ? counts[Math.floor(counts.length / 2)] : 0;
  const kind = bldgTotal > 0 ? 'mixed' : 'units';
  return `${kind} ~${median} · ${uses} uses`;
};

function TicksCanvas({ events, buildingNames, totalW, durationSeconds }) {
  const ref = useRef(null);
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    canvas.width = totalW;
    canvas.height = ROW_HEIGHT;
    const ctx = canvas.getContext('2d');
    ctx.clearRect(0, 0, totalW, ROW_HEIGHT);
    ctx.strokeStyle = 'rgba(120, 130, 140, 0.28)';
    ctx.lineWidth = 1;
    const minutes = durationSeconds / 60;
    for (let m = 1; m < minutes; m += 1) {
      const x = Math.round(m * PX_PER_MIN) + 0.5;
      ctx.beginPath();
      ctx.moveTo(x, m % 5 ? 16 : 8);
      ctx.lineTo(x, m % 5 ? 24 : 32);
      ctx.stroke();
    }
    ctx.beginPath();
    ctx.moveTo(0, ROW_HEIGHT - 0.5);
    ctx.lineTo(totalW, ROW_HEIGHT - 0.5);
    ctx.stroke();

    let current = null; // {color}
    for (const e of events) {
      const x = (e[EV_SEC] / 60) * PX_PER_MIN;
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
        ctx.moveTo(xi, 12);
        ctx.lineTo(xi, 28);
        ctx.stroke();
        ctx.globalAlpha = 1;
      }
    }
  }, [events, buildingNames, totalW, durationSeconds]);
  return <canvas ref={ref} className="hk-ticks" aria-hidden="true" />;
}

export default function HotkeyTimeline({ payload }) {
  const players = payload?.players || [];
  const buildingNames = payload?.buildings || {};
  const durationSeconds = Math.max(60, payload?.duration_seconds || 60);
  const totalW = Math.ceil((durationSeconds / 60) * PX_PER_MIN);

  const scrollRef = useRef(null);
  const tracksRef = useRef([]);
  tracksRef.current = [];
  const registerTrack = (node) => {
    if (node) tracksRef.current.push(node);
  };
  const applyScroll = () => {
    const x = scrollRef.current ? scrollRef.current.scrollLeft : 0;
    for (const t of tracksRef.current) t.style.transform = `translateX(${-x}px)`;
  };
  const dragState = useRef(null);
  const onPointerDown = (e) => {
    if (!e.target.closest('.hk-viewport')) return;
    dragState.current = e.clientX;
  };
  useEffect(() => {
    const move = (e) => {
      if (dragState.current === null || dragState.current === undefined) return;
      if (scrollRef.current) scrollRef.current.scrollLeft += dragState.current - e.clientX;
      dragState.current = e.clientX;
    };
    const up = () => { dragState.current = null; };
    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', up);
    return () => {
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', up);
    };
  }, []);
  const onWheel = (e) => {
    if (!e.target.closest('.hk-viewport')) return;
    const d = Math.abs(e.deltaX) > Math.abs(e.deltaY) ? e.deltaX : (e.shiftKey ? e.deltaY : 0);
    if (d && scrollRef.current) {
      scrollRef.current.scrollLeft += d;
      e.preventDefault();
    }
  };

  const rowsByPlayer = useMemo(() => players.map((p) => {
    const byGroup = new Map();
    for (const e of p.events || []) {
      if (e[EV_GROUP] > 9) continue;
      if (!byGroup.has(e[EV_GROUP])) byGroup.set(e[EV_GROUP], []);
      byGroup.get(e[EV_GROUP]).push(e);
    }
    const groups = [...byGroup.keys()]
      .filter((g) => byGroup.get(g).length >= 3)
      .sort((a, b) => keyboardOrder(a) - keyboardOrder(b));
    return { player: p, groups, byGroup };
  }), [players]);

  const axisLabels = [];
  const minutes = Math.floor(durationSeconds / 60);
  const step = minutes > 16 ? 4 : 2;
  for (let m = 0; m <= minutes; m += step) axisLabels.push(m);

  if (!players.length) {
    return <div className="chart-empty">No hotkey streams stored for this game.</div>;
  }

  return (
    // Wheel/drag panning needs non-passive handlers on a plain container.
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions
    <div className="hk-timeline" onPointerDown={onPointerDown} onWheelCapture={onWheel}>
      <div className="hk-legend">
        <span className="hk-legend-item">
          <img src={hotkeyBuildingIcon('Hatchery')} alt="" className="hk-legend-icon" />
          Building assign (sprite = actual building)
        </span>
        <span className="hk-legend-item">
          <img src={getUnitIcon('Marine')} alt="" className="hk-legend-icon hk-silhouette" />
          Unit assign - identity unknown, badge = count
        </span>
        <span className="hk-legend-item"><span className="hk-legend-ramp" />count 1→12+</span>
        <span className="hk-legend-item"><span className="hk-legend-ticks" />uses, tinted by group content</span>
      </div>
      {rowsByPlayer.map(({ player, groups, byGroup }) => (
        <div key={player.player_id} className="hk-player">
          <div className="hk-player-head">
            <span className="hk-player-name">{player.name}</span>
            <span className={`hk-race hk-race-${(player.race || '')[0] || 'u'}`}>{player.race}</span>
            <span className="hk-player-stat">
              {(player.events || []).length.toLocaleString()} hotkey events
              {player.legacy ? ' · legacy stream - re-analyze for building labels' : ''}
            </span>
          </div>
          {groups.length === 0 ? (
            <div className="chart-empty">No hotkey groups used.</div>
          ) : groups.map((g) => {
            const events = byGroup.get(g);
            return (
              <div key={g} className="hk-row">
                <div className="hk-keycap">{g}</div>
                <div className="hk-viewport">
                  <div className="hk-track" ref={registerTrack} style={{ width: `${totalW}px` }}>
                    <TicksCanvas
                      events={events}
                      buildingNames={buildingNames}
                      totalW={totalW}
                      durationSeconds={durationSeconds}
                    />
                    {events.map((e, i) => {
                      const x = (e[EV_SEC] / 60) * PX_PER_MIN;
                      if (e[EV_TYPE] === TYPE_ASSIGN_BUILDING) {
                        const name = buildingNames[e[EV_BUILDING]] || 'building';
                        const icon = hotkeyBuildingIcon(name);
                        const title = `${mmss(e[EV_SEC])} · assign ${g} = ${name}`;
                        return icon ? (
                          <img
                            key={i}
                            src={icon}
                            alt={name}
                            title={title}
                            className="hk-mark-building"
                            style={{ left: `${x}px` }}
                          />
                        ) : (
                          <span
                            key={i}
                            title={title}
                            className="hk-mark-dot"
                            style={{ left: `${x}px`, background: CATEGORY_COLORS[hotkeyCategoryOf(name)] }}
                          />
                        );
                      }
                      if (e[EV_TYPE] === TYPE_ASSIGN_UNITS) {
                        const n = e[EV_COUNT];
                        return (
                          <span
                            key={i}
                            className="hk-mark-units"
                            style={{ left: `${x}px` }}
                            title={`${mmss(e[EV_SEC])} · assign ${g} = ${n} unit${n === 1 ? '' : 's'} (identity not stored)`}
                          >
                            <img src={getUnitIcon(UNIT_SILHOUETTE[player.race] || 'Marine')} alt="" className="hk-silhouette" />
                            <i className="hk-count" style={{ background: hotkeyCountColor(n) }}>{n}</i>
                          </span>
                        );
                      }
                      return null;
                    })}
                  </div>
                </div>
                <div className="hk-row-summary">{rowSummary(events, buildingNames)}</div>
              </div>
            );
          })}
        </div>
      ))}
      <div className="hk-row hk-axis-row">
        <div />
        <div className="hk-viewport hk-axis">
          <div className="hk-track" ref={registerTrack} style={{ width: `${totalW}px` }}>
            {axisLabels.map((m) => (
              <span key={m} className="hk-axis-label" style={{ left: `${m * PX_PER_MIN}px` }}>{m}m</span>
            ))}
          </div>
        </div>
        <div />
      </div>
      <div className="hk-scrollbar" ref={scrollRef} onScroll={applyScroll}>
        <div style={{ width: `${totalW}px`, height: '1px' }} />
      </div>
    </div>
  );
}
