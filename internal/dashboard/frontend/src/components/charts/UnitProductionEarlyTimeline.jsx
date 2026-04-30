import React, { useMemo, useRef, useState } from 'react';
import { getUnitIcon } from '../../lib/gameAssets';

// UnitProductionEarlyTimeline renders the first 4 minutes of a game's
// production output as a vertical time-scaled chart: time on the Y axis
// (linear, 0–240s), one column per player, every produced unit/building
// shown as an icon at its exact second. Workers receive ordinal labels
// ("5th SCV") supplied by the backend; non-workers show only the time.
//
// Input shapes (match workflowUnitEarlyEventPlayer / workflowUnitEarlyEvent):
//   players[]: { player_id, player_key, name, race, team, is_winner }
//   earlyEvents[]: {
//     player_id, player_key, name,
//     events: [{ second, unit_type, is_building, label, count }, ...]
//   }

const WINDOW_SECONDS = 240;
const COLUMN_WIDTH = 220;
const HEADER_HEIGHT = 36;
const TOP_PADDING = 16;
const BOTTOM_PADDING = 24;
const PLOT_HEIGHT = 2160;
const ICON_SIZE = 22;
const ICON_SPACING = 1;
const TEXT_GAP = 4;
// Events that fall within this many seconds of the previous one share a row
// (icons stacked horizontally) instead of getting indented vertically — the
// vertical resolution of the chart is ~9px/sec, so anything tighter than this
// would visually overlap anyway.
const CLUSTER_WINDOW_SECONDS = 2;

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

const ordinalPrefix = (label) => {
  if (!label) return '';
  const m = String(label).match(/^(\d+(?:st|nd|rd|th))\b/i);
  return m ? m[1] : '';
};

const buildClusters = (events) => {
  const sorted = [...(events || [])].sort(
    (a, b) => (Number(a?.second) || 0) - (Number(b?.second) || 0),
  );
  const clusters = [];
  let current = [];
  for (const ev of sorted) {
    const sec = Number(ev?.second) || 0;
    if (current.length === 0) {
      current = [ev];
      continue;
    }
    const lastSec = Number(current[current.length - 1]?.second) || 0;
    if (sec - lastSec <= CLUSTER_WINDOW_SECONDS) {
      current.push(ev);
    } else {
      clusters.push(current);
      current = [ev];
    }
  }
  if (current.length) clusters.push(current);
  return clusters;
};

function UnitProductionEarlyTimeline({
  players,
  earlyEvents,
  filterEvents,
  hasTeamInfo,
  teamColorRgba,
}) {
  const wrapperRef = useRef(null);
  const [hover, setHover] = useState(null);

  const filteredByPlayer = useMemo(() => {
    const byPlayerID = new Map();
    (earlyEvents || []).forEach((entry) => {
      byPlayerID.set(entry.player_id, entry.events || []);
    });
    return (players || []).map((player) => {
      const events = byPlayerID.get(player.player_id) || [];
      const filtered = typeof filterEvents === 'function'
        ? filterEvents(events)
        : events;
      return { player, events: filtered };
    });
  }, [players, earlyEvents, filterEvents]);

  const totalEvents = filteredByPlayer.reduce((sum, e) => sum + e.events.length, 0);
  if (!players || players.length === 0 || totalEvents === 0) {
    return null;
  }

  const chartWidth = COLUMN_WIDTH * players.length;
  const chartHeight = HEADER_HEIGHT + TOP_PADDING + PLOT_HEIGHT + BOTTOM_PADDING;
  const yAt = (second) => {
    const clamped = Math.max(0, Math.min(WINDOW_SECONDS, Number(second) || 0));
    return HEADER_HEIGHT + TOP_PADDING + (clamped / WINDOW_SECONDS) * PLOT_HEIGHT;
  };
  const ticks = [0, 30, 60, 90, 120, 150, 180, 210, 240];

  const updateHover = (event, payload) => {
    if (!wrapperRef.current) return;
    const rect = wrapperRef.current.getBoundingClientRect();
    setHover({
      x: event.clientX - rect.left + 12,
      y: event.clientY - rect.top + 10,
      ...payload,
    });
  };
  const clearHover = () => setHover(null);

  return (
    <div
      ref={wrapperRef}
      className="workflow-early-timeline-wrap"
      onMouseLeave={clearHover}
    >
      <svg
        className="workflow-early-timeline"
        width={chartWidth}
        height={chartHeight}
        viewBox={`0 0 ${chartWidth} ${chartHeight}`}
        preserveAspectRatio="xMinYMin meet"
      >
        {/* Player column headers + tinted backgrounds */}
        {filteredByPlayer.map(({ player }, colIdx) => {
          const x0 = colIdx * COLUMN_WIDTH;
          const headerFill = hasTeamInfo ? teamColorRgba(player.team, 0.2) : 'rgba(255,255,255,0.06)';
          const bodyFill = hasTeamInfo ? teamColorRgba(player.team, 0.05) : 'rgba(255,255,255,0.02)';
          // Stronger column outline when team colours are absent — without it the
          // per-player slots blur together.
          const borderColor = hasTeamInfo ? 'rgba(255,255,255,0.06)' : 'rgba(255,255,255,0.18)';
          return (
            <g key={`hdr-${player.player_id}`}>
              <rect x={x0} y={0} width={COLUMN_WIDTH} height={HEADER_HEIGHT} fill={headerFill} />
              <rect x={x0} y={HEADER_HEIGHT} width={COLUMN_WIDTH} height={chartHeight - HEADER_HEIGHT} fill={bodyFill} />
              <rect x={x0 + 0.5} y={0.5} width={COLUMN_WIDTH - 1} height={chartHeight - 1} fill="none" stroke={borderColor} strokeWidth="1" />
              <text
                x={x0 + COLUMN_WIDTH / 2}
                y={HEADER_HEIGHT / 2 + 5}
                textAnchor="middle"
                fill="rgba(255,255,255,0.95)"
                fontSize="13"
              >
                {player.is_winner ? '👑 ' : ''}{player.name}
              </text>
            </g>
          );
        })}

        {/* Y-axis time gridlines + labels */}
        {ticks.map((t) => (
          <g key={`tick-${t}`}>
            <line
              x1={0}
              y1={yAt(t)}
              x2={chartWidth}
              y2={yAt(t)}
              stroke="rgba(255,255,255,0.10)"
              strokeWidth="1"
            />
            <text
              x={4}
              y={yAt(t) - 3}
              fill="rgba(255,255,255,0.55)"
              fontSize="10"
            >
              {formatTime(t)}
            </text>
          </g>
        ))}

        {/* Per-player events */}
        {filteredByPlayer.map(({ player, events }, colIdx) => {
          const colCenter = colIdx * COLUMN_WIDTH + COLUMN_WIDTH / 2;
          if (events.length === 0) {
            return (
              <text
                key={`empty-${player.player_id}`}
                x={colCenter}
                y={HEADER_HEIGHT + PLOT_HEIGHT / 2}
                textAnchor="middle"
                fill="rgba(255,255,255,0.35)"
                fontSize="11"
              >
                no production
              </text>
            );
          }
          const clusters = buildClusters(events);
          return clusters.flatMap((cluster, clusterIdx) => {
            const n = cluster.length;
            const firstSec = Number(cluster[0]?.second) || 0;
            const y = yAt(firstSec);
            const totalIconsWidth = n * ICON_SIZE + Math.max(0, n - 1) * ICON_SPACING;
            const startX = colCenter - totalIconsWidth / 2;
            const renderIcon = (ev, idx) => {
              const iconURL = getUnitIcon(ev.unit_type);
              const iconLeft = startX + idx * (ICON_SIZE + ICON_SPACING);
              const count = Number(ev.count) || 1;
              const onHoverEnter = (e) => updateHover(e, {
                unitType: ev.unit_type,
                second: Number(ev.second) || 0,
                ordinalLabel: ev.label || '',
                isBuilding: Boolean(ev.is_building),
                count,
              });
              return (
                <g
                  key={`ev-${player.player_id}-${clusterIdx}-${idx}`}
                  onMouseEnter={onHoverEnter}
                  onMouseMove={onHoverEnter}
                  onMouseLeave={clearHover}
                >
                  {iconURL ? (
                    <image
                      href={iconURL}
                      xlinkHref={iconURL}
                      x={iconLeft}
                      y={y - ICON_SIZE / 2}
                      width={ICON_SIZE}
                      height={ICON_SIZE}
                    />
                  ) : (
                    <circle
                      cx={iconLeft + ICON_SIZE / 2}
                      cy={y}
                      r={ICON_SIZE / 2}
                      fill="rgba(148,163,184,0.6)"
                    />
                  )}
                  {n === 1 && count > 1 ? (
                    <text
                      x={iconLeft + ICON_SIZE + 1}
                      y={y - ICON_SIZE / 2 + 4}
                      fill="rgba(251,191,36,0.95)"
                      fontSize="10"
                      fontWeight="700"
                    >
                      x{count}
                    </text>
                  ) : null}
                </g>
              );
            };
            const nodes = cluster.map(renderIcon);
            // Single text label after the icon row. For solo events keep the
            // ordinal prefix ("5th") if present; the unit name is implicit
            // in the icon, so we drop it. For clusters, just the time.
            const soloOrdinal = n === 1 ? ordinalPrefix(cluster[0]?.label) : '';
            const labelText = soloOrdinal
              ? `${soloOrdinal} @ ${formatTime(firstSec)}`
              : `@ ${formatTime(firstSec)}`;
            nodes.push(
              <text
                key={`lbl-${player.player_id}-${clusterIdx}`}
                x={startX + totalIconsWidth + TEXT_GAP}
                y={y + 4}
                fill="rgba(255,255,255,0.85)"
                fontSize="10"
              >
                {labelText}
              </text>,
            );
            return nodes;
          });
        })}
      </svg>
      {hover ? (
        <div
          className="workflow-timing-tooltip"
          style={{ left: `${hover.x}px`, top: `${hover.y}px` }}
        >
          <div><strong>{hover.unitType}</strong>{hover.count > 1 ? ` x${hover.count}` : ''}</div>
          {hover.ordinalLabel ? <div>{hover.ordinalLabel}</div> : null}
          <div><strong>Time</strong> {formatTime(hover.second)}</div>
          <div style={{ opacity: 0.7 }}>{hover.isBuilding ? 'Building' : 'Unit'}</div>
        </div>
      ) : null}
    </div>
  );
}

export default UnitProductionEarlyTimeline;
