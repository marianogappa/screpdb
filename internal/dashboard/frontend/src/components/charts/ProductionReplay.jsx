import React, { useEffect, useMemo, useRef, useState } from 'react';
import { getUnitIcon, normalizeUnitName } from '../../lib/gameAssets';

// ProductionReplay renders a game's full-game production stream
// (production_timeline) as a scrubbable / playable view of army construction
// over time: a playhead advances through game seconds and each player's
// buildings and units appear as they were produced. It's the per-event
// (exact-second) counterpart to the bucketed units_by_slice table — the point
// is to *watch* composition and cadence emerge, not read a static aggregate.
//
// Input shapes (match workflowProductionTimelinePlayer / workflowProductionEvent):
//   players[]: { player_id, name, race, team, is_winner }
//   timeline[]: { player_id, events: [{ second, unit_type, is_building, count }] }

const WORKER_KEYS = new Set(['scv', 'drone', 'probe']);
// Supply providers — overlord is a unit, depot/pylon are buildings, so this is
// matched by name regardless of is_building.
const SUPPLY_KEYS = new Set(['overlord', 'supplydepot', 'pylon']);
// Buildings that produce attacking units. Hatchery/Lair/Hive make Zerg units
// via larva, so they count as producers here.
const ATTACKER_PRODUCER_KEYS = new Set([
  'barracks', 'factory', 'starport', 'gateway', 'roboticsfacility', 'stargate',
  'hatchery', 'lair', 'hive',
]);

// Game-seconds advanced per real-time second at each speed step. A full game is
// 15-30 game-minutes, so even the slowest step compresses heavily.
const SPEED_STEPS = [15, 30, 60, 120, 240];
const DEFAULT_SPEED = 60;

// Cumulative-lane chips produced within this many game-seconds behind the
// playhead get a "just produced" glow.
const RECENT_WINDOW_SECONDS = 4;
// Army units produced within this gap of each other belong to the same cluster.
const CLUSTER_GAP_SECONDS = 4;
// After a cluster's last unit, the whole cluster fades to nothing over this many
// game-seconds.
const FADE_SECONDS = 12;
const SQUARE_SIZE = 20;

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

const num = (v) => Number(v) || 0;

// accumulate folds a player's events up to `cutoff` into ordered, grouped chip
// lists for the cumulative lanes (supply + attacker-producing buildings +
// workers). The army stream is handled separately as transient clusters. Each
// group carries the latest contributing second so the UI can flag fresh entries.
const accumulate = (events, cutoff) => {
  const supply = new Map();
  const producers = new Map();
  const workers = new Map();
  let armyTotal = 0;
  const add = (map, name, count, second) => {
    const prev = map.get(name) || { unitType: name, count: 0, lastSecond: 0 };
    prev.count += count;
    prev.lastSecond = second;
    map.set(name, prev);
  };
  for (const ev of events || []) {
    const second = num(ev?.second);
    if (second > cutoff) break; // events are pre-sorted ascending by second
    const name = String(ev?.unit_type || '');
    const key = normalizeUnitName(name);
    const count = num(ev?.count) || 1;
    if (SUPPLY_KEYS.has(key)) add(supply, name, count, second);
    else if (ev?.is_building && ATTACKER_PRODUCER_KEYS.has(key)) add(producers, name, count, second);
    else if (ev?.is_building) continue; // other buildings (tech/defense/expansions) not shown
    else if (WORKER_KEYS.has(key)) add(workers, name, count, second);
    else armyTotal += count;
  }
  const toSorted = (map) => [...map.values()].sort((a, b) => (
    b.count - a.count || a.unitType.localeCompare(b.unitType)
  ));
  return {
    supply: toSorted(supply),
    producers: toSorted(producers),
    workers: toSorted(workers),
    armyTotal,
  };
};

// buildArmyClusters groups a player's attacking-unit events into time clusters.
// Each unit becomes its own square (count is expanded, no "xN" multiplier).
const buildArmyClusters = (events) => {
  const army = (events || []).filter((ev) => {
    if (ev?.is_building) return false;
    const key = normalizeUnitName(ev?.unit_type);
    return !SUPPLY_KEYS.has(key) && !WORKER_KEYS.has(key);
  });
  const clusters = [];
  let current = null;
  for (const ev of army) {
    const second = num(ev?.second);
    if (!current) {
      current = { start: second, end: second, units: [] };
    } else if (second - current.end <= CLUSTER_GAP_SECONDS) {
      current.end = second;
    } else {
      clusters.push(current);
      current = { start: second, end: second, units: [] };
    }
    const count = num(ev?.count) || 1;
    for (let i = 0; i < count; i += 1) {
      current.units.push({ unitType: String(ev?.unit_type || ''), second });
    }
  }
  if (current) clusters.push(current);
  return clusters;
};

// clusterOpacity is 1 while the playhead is inside the cluster's span, then
// fades linearly to 0 over FADE_SECONDS once the cluster's last unit passes.
const clusterOpacity = (cluster, cutoff) => {
  if (cutoff < cluster.start) return 0;
  if (cutoff <= cluster.end) return 1;
  const t = (cutoff - cluster.end) / FADE_SECONDS;
  return t >= 1 ? 0 : 1 - t;
};

function UnitSquare({ unitType }) {
  const icon = getUnitIcon(unitType);
  if (icon) {
    return (
      <img
        src={icon}
        alt={unitType}
        title={unitType}
        width={SQUARE_SIZE}
        height={SQUARE_SIZE}
        style={{ display: 'block', borderRadius: 3 }}
      />
    );
  }
  return (
    <span
      title={unitType}
      style={{
        display: 'block',
        width: SQUARE_SIZE,
        height: SQUARE_SIZE,
        borderRadius: 3,
        background: 'rgba(148,163,184,0.6)',
      }}
    />
  );
}

function ChipRow({ entries, cutoff }) {
  if (!entries.length) {
    return <span className="workflow-empty-inline">-</span>;
  }
  return (
    <div className="workflow-unit-chips">
      {entries.map((entry) => {
        const fresh = cutoff - entry.lastSecond <= RECENT_WINDOW_SECONDS;
        const icon = getUnitIcon(entry.unitType);
        return (
          <span
            key={entry.unitType}
            className="workflow-unit-chip"
            title={`${entry.unitType} x${entry.count}`}
            style={fresh ? {
              boxShadow: '0 0 0 1px rgba(251,191,36,0.9)',
              background: 'rgba(251,191,36,0.14)',
            } : undefined}
          >
            {icon ? <img src={icon} alt={entry.unitType} className="workflow-unit-chip-icon" /> : null}
            <strong className="workflow-unit-chip-count">x{entry.count}</strong>
          </span>
        );
      })}
    </div>
  );
}

function ArmyClusterLane({ clusters, cutoff }) {
  const visible = clusters
    .map((cluster) => ({ cluster, opacity: clusterOpacity(cluster, cutoff) }))
    .filter(({ opacity }) => opacity > 0.01);
  if (!visible.length) {
    return <span className="workflow-empty-inline">-</span>;
  }
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 10, minHeight: SQUARE_SIZE }}>
      {visible.map(({ cluster, opacity }) => (
        <div
          key={`${cluster.start}-${cluster.end}`}
          style={{ display: 'flex', flexWrap: 'wrap', gap: 2, opacity }}
        >
          {cluster.units.map((unit, idx) => (
            <UnitSquare key={`${cluster.start}-${idx}`} unitType={unit.unitType} />
          ))}
        </div>
      ))}
    </div>
  );
}

function ProductionReplay({ players, timeline, durationSeconds, hasTeamInfo, teamColorRgba }) {
  const duration = Math.max(1, Math.floor(Number(durationSeconds) || 0));
  const [currentSecond, setCurrentSecond] = useState(0);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(DEFAULT_SPEED);
  const [showWorkers, setShowWorkers] = useState(false);
  const rafRef = useRef(null);
  const lastTsRef = useRef(null);

  const eventsByPlayer = useMemo(() => {
    const map = new Map();
    (timeline || []).forEach((entry) => {
      map.set(entry.player_id, entry.events || []);
    });
    return map;
  }, [timeline]);

  // Army clusters depend only on the events, so build them once per player.
  const clustersByPlayer = useMemo(() => {
    const map = new Map();
    (timeline || []).forEach((entry) => {
      map.set(entry.player_id, buildArmyClusters(entry.events || []));
    });
    return map;
  }, [timeline]);

  // rAF playback loop: advance the playhead by speed × elapsed real time, and
  // stop cleanly at the end of the game.
  useEffect(() => {
    if (!playing) {
      lastTsRef.current = null;
      return undefined;
    }
    const tick = (ts) => {
      if (lastTsRef.current == null) lastTsRef.current = ts;
      const dtSeconds = (ts - lastTsRef.current) / 1000;
      lastTsRef.current = ts;
      setCurrentSecond((prev) => {
        const next = prev + dtSeconds * speed;
        if (next >= duration) return duration;
        return next;
      });
      rafRef.current = requestAnimationFrame(tick);
    };
    rafRef.current = requestAnimationFrame(tick);
    return () => {
      if (rafRef.current) cancelAnimationFrame(rafRef.current);
    };
  }, [playing, speed, duration]);

  // Auto-pause once the playhead reaches the end.
  useEffect(() => {
    if (playing && currentSecond >= duration) setPlaying(false);
  }, [playing, currentSecond, duration]);

  const cutoff = Math.floor(currentSecond);

  const perPlayer = useMemo(() => (
    (players || []).map((player) => ({
      player,
      clusters: clustersByPlayer.get(player.player_id) || [],
      ...accumulate(eventsByPlayer.get(player.player_id), cutoff),
    }))
  ), [players, eventsByPlayer, clustersByPlayer, cutoff]);

  if (!players || players.length === 0) return null;

  const atEnd = currentSecond >= duration;
  const togglePlay = () => {
    if (atEnd) setCurrentSecond(0);
    setPlaying((p) => !p);
  };

  return (
    <div className="workflow-card workflow-card-chat-summary">
      <div className="workflow-section-info" role="note">
        🧪 Experimental. Watch army construction unfold: press play (or drag the
        slider) to see each player's buildings appear and units stream out at the
        exact second they were produced. Army units cluster by production burst
        and fade out together once the burst ends. Built from per-command timing
        for the whole game — same data the cadence proxy reads, but un-bucketed.
      </div>

      <div className="workflow-production-top-row" style={{ alignItems: 'center', gap: 12, flexWrap: 'wrap' }}>
        <button
          type="button"
          className="workflow-production-tab"
          onClick={togglePlay}
          style={{ minWidth: 88 }}
        >
          {playing ? '⏸ Pause' : atEnd ? '↻ Replay' : '▶ Play'}
        </button>
        <div className="workflow-radio-group" role="radiogroup" aria-label="Playback speed">
          {SPEED_STEPS.map((step) => (
            <label key={step} className="workflow-radio-option">
              <input
                type="radio"
                name="production-replay-speed"
                value={step}
                checked={speed === step}
                onChange={() => setSpeed(step)}
              />
              <span>{step}s/s</span>
            </label>
          ))}
        </div>
        <label className="workflow-radio-option" style={{ marginLeft: 'auto' }}>
          <input
            type="checkbox"
            checked={showWorkers}
            onChange={(e) => setShowWorkers(e.target.checked)}
          />
          <span>Show workers</span>
        </label>
      </div>

      <div style={{ display: 'flex', alignItems: 'center', gap: 12, margin: '8px 0 16px' }}>
        <input
          type="range"
          min={0}
          max={duration}
          step={1}
          value={Math.min(cutoff, duration)}
          onChange={(e) => {
            setPlaying(false);
            setCurrentSecond(Number(e.target.value));
          }}
          style={{ flex: 1 }}
          aria-label="Game time"
        />
        <span style={{ fontVariantNumeric: 'tabular-nums', minWidth: 96, textAlign: 'right' }}>
          <strong>{formatTime(cutoff)}</strong> / {formatTime(duration)}
        </span>
      </div>

      <div
        className="workflow-production-replay-grid"
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${players.length}, minmax(220px, 1fr))`,
          gap: 12,
        }}
      >
        {perPlayer.map(({ player, clusters, supply, producers, workers, armyTotal }) => {
          const headerFill = hasTeamInfo ? teamColorRgba(player.team, 0.2) : 'rgba(255,255,255,0.06)';
          const bodyFill = hasTeamInfo ? teamColorRgba(player.team, 0.05) : 'rgba(255,255,255,0.02)';
          return (
            <div
              key={player.player_id}
              style={{
                border: '1px solid rgba(255,255,255,0.1)',
                borderRadius: 8,
                background: bodyFill,
                overflow: 'hidden',
              }}
            >
              <div style={{ padding: '8px 12px', background: headerFill, fontWeight: 600 }}>
                {player.is_winner ? '👑 ' : ''}{player.name}
              </div>
              <div style={{ padding: 12, display: 'flex', flexDirection: 'column', gap: 12 }}>
                <div>
                  <div className="workflow-replay-lane-label" style={{ opacity: 0.7, fontSize: 12, marginBottom: 4 }}>
                    🏭 Production buildings
                  </div>
                  <ChipRow entries={producers} cutoff={cutoff} />
                </div>
                <div>
                  <div className="workflow-replay-lane-label" style={{ opacity: 0.7, fontSize: 12, marginBottom: 4 }}>
                    🏠 Supply
                  </div>
                  <ChipRow entries={supply} cutoff={cutoff} />
                </div>
                <div>
                  <div className="workflow-replay-lane-label" style={{ opacity: 0.7, fontSize: 12, marginBottom: 4 }}>
                    ⚔ Army — {armyTotal} unit{armyTotal === 1 ? '' : 's'}
                  </div>
                  <ArmyClusterLane clusters={clusters} cutoff={cutoff} />
                </div>
                {showWorkers ? (
                  <div>
                    <div className="workflow-replay-lane-label" style={{ opacity: 0.7, fontSize: 12, marginBottom: 4 }}>
                      ⛏ Workers
                    </div>
                    <ChipRow entries={workers} cutoff={cutoff} />
                  </div>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export default ProductionReplay;
