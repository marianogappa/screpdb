import React, { useMemo, useRef, useState } from 'react';
import { getUnitIcon } from '../../lib/gameAssets';

// BuildOrderTimelineRows renders one SVG chart per player, comparing their
// actual build-order milestone timings against the expert ("progamer") template.
//
// Input `group` shape (matches workflowBuildOrderPlayer on the backend):
//   {
//     player_id, player_key, name, race, build_order, feature_key,
//     events: [{
//       key, subject,
//       target_second, tolerance_early_seconds, tolerance_late_seconds,
//       actual_second, found, delta_seconds, within_tolerance,
//     }, ...]
//   }

const LEGEND_TOOLTIP =
  'Ranges are derived from averages across tens of thousands of progamer replays. This model is an approximation — data is imperfect and degrades as the metagame evolves.';

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

function BuildOrderTimelineRows({ group }) {
  const wrapperRef = useRef(null);
  const [hover, setHover] = useState(null);

  const prepared = useMemo(() => {
    const events = Array.isArray(group?.events) ? group.events : [];
    if (events.length === 0) {
      return { events: [], minSecond: 0, maxSecond: 60 };
    }
    let minSecond = Infinity;
    let maxSecond = -Infinity;
    events.forEach((e) => {
      // Expert-backed rows contribute target ± tolerance to the bounds;
      // count-only rows (no_expert) only have an actual timing.
      if (!e.no_expert) {
        const target = Number(e.target_second) || 0;
        const early = Number(e.tolerance_early_seconds) || 0;
        const late = Number(e.tolerance_late_seconds) || 0;
        minSecond = Math.min(minSecond, target - early);
        maxSecond = Math.max(maxSecond, target + late);
      }
      if (e.found) {
        const actual = Number(e.actual_second) || 0;
        minSecond = Math.min(minSecond, actual);
        maxSecond = Math.max(maxSecond, actual);
      }
    });
    if (!Number.isFinite(minSecond)) minSecond = 0;
    if (!Number.isFinite(maxSecond)) maxSecond = Math.max(60, minSecond + 10);
    const span = Math.max(1, maxSecond - minSecond);
    const pad = Math.max(6, Math.round(span * 0.15));
    return {
      events,
      minSecond: Math.max(0, minSecond - pad),
      maxSecond: maxSecond + pad,
    };
  }, [group]);

  if (prepared.events.length === 0) return null;

  const chartWidth = 980;
  const leftPadding = 150;
  const rightPadding = 40;
  const topPadding = 34;
  const bottomPadding = 42;
  const rowHeight = 44;
  const iconSize = 22;
  const plotWidth = chartWidth - leftPadding - rightPadding;
  const chartHeight = topPadding + bottomPadding + (prepared.events.length * rowHeight);
  const xAt = (second) => {
    const span = Math.max(1, prepared.maxSecond - prepared.minSecond);
    const bounded = Math.max(prepared.minSecond, Math.min(prepared.maxSecond, Number(second) || prepared.minSecond));
    return leftPadding + (((bounded - prepared.minSecond) / span) * plotWidth);
  };
  const yAt = (idx) => topPadding + idx * rowHeight + (rowHeight / 2);

  const tickCount = 6;
  const ticks = Array.from({ length: tickCount }).map((_, idx) => {
    const span = Math.max(1, prepared.maxSecond - prepared.minSecond);
    return Math.round(prepared.minSecond + ((span * idx) / (tickCount - 1)));
  });

  const updateHover = (event, payload) => {
    if (!wrapperRef.current) return;
    const rect = wrapperRef.current.getBoundingClientRect();
    setHover({
      x: event.clientX - rect.left + 12,
      y: event.clientY - rect.top + 10,
      ...payload,
    });
  };

  return (
    <div className="workflow-card timing-chart-card">
      <div className="workflow-first-unit-title" style={{ display: 'flex', alignItems: 'center', width: '100%' }}>
        <span><strong>{group?.name}</strong></span>
        <span className="workflow-first-unit-title-slash">·</span>
        <span>{group?.build_order}</span>
        <span
          style={{ marginLeft: 'auto', color: 'rgba(251, 191, 36, 1)', cursor: 'help' }}
          title={LEGEND_TOOLTIP}
        >
          * Progamer average ranges
        </span>
      </div>
      <div ref={wrapperRef} className="workflow-timing-chart-wrap">
        <svg className="workflow-timing-scatter" viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMinYMin meet">
          {prepared.events.map((entry, idx) => {
            const noExpert = Boolean(entry.no_expert);
            const target = Number(entry.target_second) || 0;
            const early = Number(entry.tolerance_early_seconds) || 0;
            const late = Number(entry.tolerance_late_seconds) || 0;
            const actual = Number(entry.actual_second) || 0;
            const withinTolerance = Boolean(entry.within_tolerance);
            const found = Boolean(entry.found);
            const actualColor = noExpert
              ? 'rgba(148, 197, 230, 0.95)' // neutral blue — no golden range to compare against
              : (found
                ? (withinTolerance ? 'rgba(34, 197, 94, 0.95)' : 'rgba(239, 68, 68, 0.95)')
                : 'rgba(148, 163, 184, 0.6)');
            const iconURL = getUnitIcon(entry.subject || entry.key);
            const rowY = yAt(idx);
            return (
              <g key={`bo-row-${idx}-${entry.key}`}>
                {/* Row separator */}
                <line
                  x1={leftPadding}
                  y1={rowY}
                  x2={chartWidth - rightPadding}
                  y2={rowY}
                  stroke="rgba(255,255,255,0.1)"
                  strokeWidth="1"
                />
                {/* Row label: icon + key text. Both are shown so ordinal
                    drone rows ("1st Drone", "5th Drone", ...) are
                    legible alongside the unit icon. */}
                {iconURL ? (
                  <>
                    <image
                      href={iconURL}
                      xlinkHref={iconURL}
                      x={leftPadding - iconSize - 6}
                      y={rowY - iconSize / 2}
                      width={iconSize}
                      height={iconSize}
                    >
                      <title>{entry.key}</title>
                    </image>
                    <text
                      x={leftPadding - iconSize - 12}
                      y={rowY + 4}
                      textAnchor="end"
                      fill="rgba(255,255,255,0.85)"
                      fontSize="11"
                    >
                      {entry.key}
                    </text>
                  </>
                ) : (
                  <text
                    x={leftPadding - 10}
                    y={rowY + 4}
                    textAnchor="end"
                    fill="rgba(255,255,255,0.9)"
                    fontSize="12"
                  >
                    {entry.key}
                  </text>
                )}
                {/* Tolerance band + expert target marker — only for rows
                    with a golden Expert target (NoExpert rows render only
                    the actual tick). */}
                {!noExpert ? (
                  <>
                    <line
                      x1={xAt(target - early)}
                      y1={rowY}
                      x2={xAt(target + late)}
                      y2={rowY}
                      stroke="rgba(251, 191, 36, 0.35)"
                      strokeWidth="14"
                      strokeLinecap="round"
                    />
                    <g
                      onMouseEnter={(e) => updateHover(e, {
                        pointKind: 'Expert target',
                        eventKey: entry.key,
                        time: target,
                        tol: `±${early === late ? early : `${early}/${late}`}s`,
                      })}
                      onMouseMove={(e) => updateHover(e, {
                        pointKind: 'Expert target',
                        eventKey: entry.key,
                        time: target,
                        tol: `±${early === late ? early : `${early}/${late}`}s`,
                      })}
                      onMouseLeave={() => setHover(null)}
                    >
                      <line
                        x1={xAt(target)}
                        y1={rowY - 9}
                        x2={xAt(target)}
                        y2={rowY + 9}
                        stroke="rgba(251, 191, 36, 1)"
                        strokeWidth="2"
                      />
                    </g>
                  </>
                ) : null}
                {/* Actual marker: dotted vertical guide + icon + time label */}
                {found ? (
                  <>
                    <line
                      x1={xAt(actual)}
                      y1={topPadding}
                      x2={xAt(actual)}
                      y2={chartHeight - bottomPadding}
                      stroke={actualColor}
                      strokeWidth="1"
                      strokeDasharray="3,3"
                      opacity="0.55"
                    />
                    <text
                      x={xAt(actual)}
                      y={topPadding - 8}
                      textAnchor="middle"
                      fill={actualColor}
                      fontSize="10"
                    >
                      {formatTime(actual)}
                    </text>
                    <g
                      onMouseEnter={(e) => updateHover(e, {
                        pointKind: 'Player actual',
                        eventKey: entry.key,
                        time: actual,
                        withinTolerance,
                      })}
                      onMouseMove={(e) => updateHover(e, {
                        pointKind: 'Player actual',
                        eventKey: entry.key,
                        time: actual,
                        withinTolerance,
                      })}
                      onMouseLeave={() => setHover(null)}
                    >
                      {iconURL ? (
                        <>
                          <circle
                            cx={xAt(actual)}
                            cy={rowY}
                            r={iconSize / 2 + 2}
                            fill="rgba(9,10,16,0.9)"
                            stroke={actualColor}
                            strokeWidth="1.5"
                          />
                          <image
                            href={iconURL}
                            xlinkHref={iconURL}
                            x={xAt(actual) - iconSize / 2}
                            y={rowY - iconSize / 2}
                            width={iconSize}
                            height={iconSize}
                          />
                        </>
                      ) : (
                        <circle
                          cx={xAt(actual)}
                          cy={rowY}
                          r="6"
                          fill={actualColor}
                          stroke="rgba(9,10,16,0.95)"
                          strokeWidth="1.25"
                        />
                      )}
                    </g>
                  </>
                ) : (
                  <text
                    x={xAt(target)}
                    y={rowY + 4}
                    textAnchor="middle"
                    fill={actualColor}
                    fontSize="10"
                  >
                    —
                  </text>
                )}
              </g>
            );
          })}
          {/* Bottom ticks */}
          {ticks.map((second) => (
            <g key={`bo-tick-${second}`}>
              <line
                x1={xAt(second)}
                y1={topPadding - 6}
                x2={xAt(second)}
                y2={chartHeight - bottomPadding + 6}
                stroke="rgba(255,255,255,0.14)"
                strokeWidth="1"
              />
              <text
                x={xAt(second)}
                y={chartHeight - bottomPadding + 20}
                textAnchor="middle"
                fill="rgba(255,255,255,0.75)"
                fontSize="11"
              >
                {formatTime(second)}
              </text>
            </g>
          ))}
        </svg>
        {hover ? (
          <div
            className="workflow-timing-tooltip"
            style={{ left: `${hover.x}px`, top: `${hover.y}px` }}
          >
            <div><strong>{hover.eventKey}</strong></div>
            <div><strong>Point</strong> {hover.pointKind}</div>
            <div><strong>Time</strong> {formatTime(hover.time)}</div>
            {hover.tol ? <div><strong>Tolerance</strong> {hover.tol}</div> : null}
            {hover.pointKind === 'Player actual' ? (
              <div>{hover.withinTolerance ? '(within tolerance)' : '(out of tolerance)'}</div>
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export default BuildOrderTimelineRows;
