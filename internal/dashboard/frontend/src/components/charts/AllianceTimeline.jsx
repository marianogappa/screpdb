import React, { useMemo, useRef, useState, useEffect } from 'react';

// AllianceTimeline renders the alliance-topology timeline for a multi-player
// melee game. Layout (per the picked visualization):
//   ┌─────────────────────────────────────────┐
//   │   GRAPH of current topology             │  top, ~60% height
//   │   nodes = players, colored by team      │
//   │   edges = mutual alliances              │
//   ├─────────────────────────────────────────┤
//   │   PHASE BANDS  | red where stacking     │  middle: scrub strip
//   │   ●━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ ►       │  scrubber thumb on time axis
//   │   0:00                       18:42       │
//   └─────────────────────────────────────────┘
//
// Inputs:
//   players: [{ player_id, name, race, team }] — used for node labels/icons.
//   timeline: [{ sec, teams: [[player_id, ...], ...], stacking }] — ordered.
//   durationSeconds: number
//   stackingThresholdSeconds: number — bands longer than this earn 😈.
//   getRaceIcon: race -> url (or null)

const NODE_RADIUS = 22;
const NODE_LABEL_GAP = 6;
const NODE_LABEL_HEIGHT = 16;
const TEAM_GAP = 60;
const PLAYER_GAP = 12;
const GRAPH_TOP_PADDING = 18;
const GRAPH_BOTTOM_PADDING = 10;
const STRIP_HEIGHT = 32;
const STRIP_TOP_PADDING = 28;
const TIME_AXIS_HEIGHT = 22;

const TEAM_COLORS = ['#60A5FA', '#F472B6', '#34D399', '#FBBF24', '#A78BFA', '#22D3EE', '#FB7185', '#4ADE80'];

const formatMMSS = (sec) => {
  const v = Math.max(0, Math.floor(Number(sec) || 0));
  return `${Math.floor(v / 60)}:${String(v % 60).padStart(2, '0')}`;
};

// Map a team's min-pid to a stable color. Same team-membership across
// snapshots keeps the same color; merged/split teams pick up new colors,
// which is the right behavior — those *are* different teams.
const colorForTeam = (team) => {
  if (!team || !team.length) return TEAM_COLORS[0];
  return TEAM_COLORS[Math.abs(Number(team[0]) || 0) % TEAM_COLORS.length];
};

// Format like "2v2v2 + 1 solo" — useful labels for the strip.
const teamShape = (teams) => {
  const sizes = teams.map((t) => t.length);
  const nonSolo = sizes.filter((s) => s >= 2).sort((a, b) => b - a);
  const solo = sizes.filter((s) => s === 1).length;
  const left = nonSolo.length > 0 ? nonSolo.join('v') : '';
  const right = solo > 0 ? `${solo} solo` : '';
  if (left && right) return `${left} + ${right}`;
  if (left) return left;
  if (right) return right;
  return '—';
};

const AllianceTimeline = ({
  players = [],
  timeline = [],
  durationSeconds = 0,
  stackingThresholdSeconds = 300,
  getRaceIcon,
}) => {
  const playerByID = useMemo(() => {
    const m = {};
    for (const p of players) {
      if (p && p.player_id != null) m[p.player_id] = p;
    }
    return m;
  }, [players]);

  // Phases: each timeline entry plus its end second.
  const phases = useMemo(() => {
    const safe = Array.isArray(timeline) ? timeline : [];
    return safe.map((snap, idx) => {
      const start = Math.max(0, Number(snap.sec) || 0);
      const end = idx + 1 < safe.length
        ? Math.max(start, Number(safe[idx + 1].sec) || start)
        : Math.max(start, Number(durationSeconds) || start);
      return {
        sec: start,
        endSec: end,
        durationSec: end - start,
        teams: Array.isArray(snap.teams) ? snap.teams : [],
        stacking: !!snap.stacking,
      };
    });
  }, [timeline, durationSeconds]);

  // Default scrubber position: phase with longest duration. Tie-break to the
  // phase with mutual alliances (stacking or otherwise).
  const defaultIdx = useMemo(() => {
    if (phases.length === 0) return 0;
    let best = 0;
    let bestDur = -1;
    for (let i = 0; i < phases.length; i += 1) {
      const ph = phases[i];
      const hasMutual = ph.teams.some((t) => t.length >= 2);
      if (
        ph.durationSec > bestDur
        || (ph.durationSec === bestDur && hasMutual && !phases[best].teams.some((t) => t.length >= 2))
      ) {
        best = i;
        bestDur = ph.durationSec;
      }
    }
    return best;
  }, [phases]);

  const [phaseIdx, setPhaseIdx] = useState(defaultIdx);
  useEffect(() => { setPhaseIdx(defaultIdx); }, [defaultIdx]);

  const stripRef = useRef(null);
  const [stripWidth, setStripWidth] = useState(800);
  useEffect(() => {
    const el = stripRef.current;
    if (!el) return undefined;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = Math.floor(entry.contentRect.width);
        if (w > 0) setStripWidth(w);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  if (phases.length === 0) {
    return (
      <div className="workflow-card">
        <div className="chart-empty">No alliance commands recorded for this game.</div>
      </div>
    );
  }

  const currentPhase = phases[Math.min(phaseIdx, phases.length - 1)];
  const totalDur = Math.max(durationSeconds, phases[phases.length - 1].endSec) || 1;

  // ── Graph layout (current snapshot) ─────────────────────────────────────
  // Lay out teams left-to-right; players within each team are stacked
  // vertically. Solo teams collapse to a "Free agents" column on the right.
  const teamsForGraph = currentPhase.teams.slice().sort((a, b) => {
    // Non-solo teams first (largest first), then solos.
    if ((a.length >= 2) !== (b.length >= 2)) return a.length >= 2 ? -1 : 1;
    if (a.length !== b.length) return b.length - a.length;
    return Number(a[0] || 0) - Number(b[0] || 0);
  });

  const nodeBlockHeight = (NODE_RADIUS * 2) + NODE_LABEL_GAP + NODE_LABEL_HEIGHT;
  const tallestTeam = Math.max(1, ...teamsForGraph.map((t) => t.length));
  const graphHeight = GRAPH_TOP_PADDING
    + (tallestTeam * nodeBlockHeight)
    + ((tallestTeam - 1) * PLAYER_GAP)
    + GRAPH_BOTTOM_PADDING;
  const teamWidth = NODE_RADIUS * 2 + 60;
  const graphWidth = Math.max(
    240,
    teamsForGraph.length * teamWidth + (Math.max(0, teamsForGraph.length - 1) * TEAM_GAP) + 32,
  );

  const teamColumns = teamsForGraph.map((team, teamIdx) => {
    const cx = 16 + (teamIdx * (teamWidth + TEAM_GAP)) + (teamWidth / 2);
    const nodes = team.map((pid, i) => {
      const cy = GRAPH_TOP_PADDING + NODE_RADIUS + i * (nodeBlockHeight + PLAYER_GAP);
      const player = playerByID[pid] || { name: `#${pid}`, race: '' };
      return { pid, cx, cy, player };
    });
    return { team, color: colorForTeam(team), nodes };
  });

  const edges = [];
  teamColumns.forEach((col) => {
    if (col.nodes.length < 2) return;
    for (let i = 0; i < col.nodes.length - 1; i += 1) {
      const a = col.nodes[i];
      const b = col.nodes[i + 1];
      edges.push({ from: a, to: b, color: col.color });
    }
  });

  // ── Strip layout ────────────────────────────────────────────────────────
  const stripScale = (sec) => (Math.max(0, Math.min(totalDur, sec)) / totalDur) * stripWidth;

  const handleStripClick = (event) => {
    const rect = stripRef.current?.getBoundingClientRect();
    if (!rect) return;
    const x = event.clientX - rect.left;
    const sec = (x / Math.max(1, rect.width)) * totalDur;
    let nearest = 0;
    let nearestDelta = Infinity;
    for (let i = 0; i < phases.length; i += 1) {
      const ph = phases[i];
      // Pick the phase containing this second; fallback to nearest start.
      if (sec >= ph.sec && sec < ph.endSec) {
        nearest = i;
        nearestDelta = 0;
        break;
      }
      const delta = Math.abs(ph.sec - sec);
      if (delta < nearestDelta) {
        nearestDelta = delta;
        nearest = i;
      }
    }
    setPhaseIdx(nearest);
  };

  // ── Render ──────────────────────────────────────────────────────────────
  return (
    <div className="workflow-card workflow-alliance-timeline">
      <div className="workflow-alliance-graph-wrap" style={{ overflowX: 'auto' }}>
        <svg
          width={graphWidth}
          height={graphHeight}
          viewBox={`0 0 ${graphWidth} ${graphHeight}`}
          className="workflow-alliance-graph"
        >
          {edges.map((e, idx) => (
            <line
              key={`edge-${idx}`}
              x1={e.from.cx}
              y1={e.from.cy}
              x2={e.to.cx}
              y2={e.to.cy}
              stroke={e.color}
              strokeWidth={2}
              strokeOpacity={0.55}
            />
          ))}
          {teamColumns.flatMap((col) => col.nodes.map((n) => {
            const icon = getRaceIcon ? getRaceIcon(n.player.race) : null;
            const labelY = n.cy + NODE_RADIUS + NODE_LABEL_GAP + (NODE_LABEL_HEIGHT * 0.7);
            return (
              <g key={`node-${n.pid}`}>
                <circle
                  cx={n.cx}
                  cy={n.cy}
                  r={NODE_RADIUS}
                  fill={col.color}
                  fillOpacity={0.22}
                  stroke={col.color}
                  strokeWidth={2}
                />
                {icon ? (
                  <image
                    href={icon}
                    x={n.cx - 13}
                    y={n.cy - 13}
                    width={26}
                    height={26}
                  />
                ) : (
                  <text
                    x={n.cx}
                    y={n.cy + 4}
                    textAnchor="middle"
                    fontSize={12}
                    fill="#e5e7eb"
                  >
                    {String(n.player.race || '?').slice(0, 1).toUpperCase()}
                  </text>
                )}
                <text
                  x={n.cx}
                  y={labelY}
                  textAnchor="middle"
                  fontSize={11}
                  fill="#cbd5e1"
                >
                  {String(n.player.name || `#${n.pid}`).slice(0, 14)}
                </text>
              </g>
            );
          }))}
        </svg>
      </div>

      <div className="workflow-alliance-strip-meta">
        <span>
          <strong>{teamShape(currentPhase.teams)}</strong>
          {' '}from {formatMMSS(currentPhase.sec)} to {formatMMSS(currentPhase.endSec)}
          {currentPhase.stacking ? <span title="Uneven non-solo team sizes" style={{ marginLeft: 6 }}>😈</span> : null}
        </span>
      </div>

      <div
        ref={stripRef}
        className="workflow-alliance-strip"
        onClick={handleStripClick}
        role="slider"
        tabIndex={0}
        aria-valuemin={0}
        aria-valuemax={totalDur}
        aria-valuenow={currentPhase.sec}
        style={{ position: 'relative', height: STRIP_TOP_PADDING + STRIP_HEIGHT + TIME_AXIS_HEIGHT, cursor: 'pointer', userSelect: 'none' }}
      >
        <svg
          width="100%"
          height={STRIP_TOP_PADDING + STRIP_HEIGHT + TIME_AXIS_HEIGHT}
          viewBox={`0 0 ${stripWidth} ${STRIP_TOP_PADDING + STRIP_HEIGHT + TIME_AXIS_HEIGHT}`}
          preserveAspectRatio="none"
          style={{ display: 'block' }}
        >
          {phases.map((ph, i) => {
            const x1 = stripScale(ph.sec);
            const x2 = stripScale(ph.endSec);
            const w = Math.max(1, x2 - x1);
            const stacking = ph.stacking;
            const isLongStack = stacking && ph.durationSec > stackingThresholdSeconds;
            const isCurrent = i === phaseIdx;
            return (
              <g key={`phase-${i}`}>
                <rect
                  x={x1}
                  y={STRIP_TOP_PADDING}
                  width={w}
                  height={STRIP_HEIGHT}
                  fill={stacking ? 'rgba(248, 113, 113, 0.45)' : 'rgba(96, 165, 250, 0.25)'}
                  stroke={isCurrent ? '#fbbf24' : 'rgba(255,255,255,0.18)'}
                  strokeWidth={isCurrent ? 2 : 1}
                />
                {w > 50 ? (
                  <text
                    x={x1 + w / 2}
                    y={STRIP_TOP_PADDING + STRIP_HEIGHT / 2 + 4}
                    textAnchor="middle"
                    fontSize={11}
                    fill="#e5e7eb"
                    style={{ pointerEvents: 'none' }}
                  >
                    {teamShape(ph.teams)}
                  </text>
                ) : null}
                {isLongStack ? (
                  <text
                    x={x1 + 6}
                    y={STRIP_TOP_PADDING - 8}
                    fontSize={16}
                  >
                    😈
                  </text>
                ) : null}
              </g>
            );
          })}
          {/* Time axis ticks at 0%, 25%, 50%, 75%, 100%. */}
          {[0, 0.25, 0.5, 0.75, 1].map((f) => {
            const x = f * stripWidth;
            const t = f * totalDur;
            return (
              <g key={`tick-${f}`}>
                <line
                  x1={x}
                  y1={STRIP_TOP_PADDING + STRIP_HEIGHT}
                  x2={x}
                  y2={STRIP_TOP_PADDING + STRIP_HEIGHT + 5}
                  stroke="#94a3b8"
                />
                <text
                  x={Math.max(2, Math.min(stripWidth - 30, x))}
                  y={STRIP_TOP_PADDING + STRIP_HEIGHT + 18}
                  fontSize={10}
                  fill="#94a3b8"
                >
                  {formatMMSS(t)}
                </text>
              </g>
            );
          })}
          {/* Scrubber thumb at current phase mid-point. */}
          {(() => {
            const cx = stripScale((currentPhase.sec + currentPhase.endSec) / 2);
            return (
              <g>
                <line
                  x1={cx}
                  y1={STRIP_TOP_PADDING - 4}
                  x2={cx}
                  y2={STRIP_TOP_PADDING + STRIP_HEIGHT + 4}
                  stroke="#fbbf24"
                  strokeWidth={2}
                />
                <circle cx={cx} cy={STRIP_TOP_PADDING + STRIP_HEIGHT / 2} r={5} fill="#fbbf24" />
              </g>
            );
          })()}
        </svg>
      </div>

      <div className="workflow-alliance-legend">
        <span>Click any band on the strip to inspect that phase. Default is the longest-held topology.</span>
      </div>
    </div>
  );
};

export default AllianceTimeline;
