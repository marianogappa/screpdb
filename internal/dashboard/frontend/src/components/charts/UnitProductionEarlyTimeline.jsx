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
const LANE_OFFSETS = [-46, 0, 46]; // x offsets within a player column

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
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
        viewBox={`0 0 ${chartWidth} ${chartHeight}`}
        preserveAspectRatio="xMinYMin meet"
      >
        {/* Player column headers + tinted backgrounds */}
        {filteredByPlayer.map(({ player }, colIdx) => {
          const x0 = colIdx * COLUMN_WIDTH;
          const headerFill = hasTeamInfo ? teamColorRgba(player.team, 0.2) : 'rgba(255,255,255,0.06)';
          const bodyFill = hasTeamInfo ? teamColorRgba(player.team, 0.05) : 'rgba(255,255,255,0.02)';
          return (
            <g key={`hdr-${player.player_id}`}>
              <rect x={x0} y={0} width={COLUMN_WIDTH} height={HEADER_HEIGHT} fill={headerFill} />
              <rect x={x0} y={HEADER_HEIGHT} width={COLUMN_WIDTH} height={chartHeight - HEADER_HEIGHT} fill={bodyFill} />
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
          return events.map((ev, evIdx) => {
            const second = Number(ev.second) || 0;
            const y = yAt(second);
            const laneX = colCenter + LANE_OFFSETS[evIdx % LANE_OFFSETS.length];
            const iconURL = getUnitIcon(ev.unit_type);
            const count = Number(ev.count) || 1;
            const labelText = ev.label
              ? `${ev.label} @ ${formatTime(second)}`
              : formatTime(second);
            const onHoverEnter = (e) => updateHover(e, {
              unitType: ev.unit_type,
              second,
              ordinalLabel: ev.label || '',
              isBuilding: Boolean(ev.is_building),
              count,
            });
            return (
              <g
                key={`ev-${player.player_id}-${evIdx}`}
                onMouseEnter={onHoverEnter}
                onMouseMove={onHoverEnter}
                onMouseLeave={clearHover}
              >
                {iconURL ? (
                  <image
                    href={iconURL}
                    xlinkHref={iconURL}
                    x={laneX - ICON_SIZE / 2}
                    y={y - ICON_SIZE / 2}
                    width={ICON_SIZE}
                    height={ICON_SIZE}
                  />
                ) : (
                  <circle
                    cx={laneX}
                    cy={y}
                    r={ICON_SIZE / 2}
                    fill="rgba(148,163,184,0.6)"
                  />
                )}
                {count > 1 ? (
                  <text
                    x={laneX + ICON_SIZE / 2 + 1}
                    y={y - ICON_SIZE / 2 + 4}
                    fill="rgba(251,191,36,0.95)"
                    fontSize="10"
                    fontWeight="700"
                  >
                    x{count}
                  </text>
                ) : null}
                <text
                  x={laneX + ICON_SIZE / 2 + 4}
                  y={y + 4}
                  fill="rgba(255,255,255,0.85)"
                  fontSize="10"
                >
                  {labelText}
                </text>
              </g>
            );
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
