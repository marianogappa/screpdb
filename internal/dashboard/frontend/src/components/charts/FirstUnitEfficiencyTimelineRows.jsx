import React, { useMemo, useRef, useState } from 'react';

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

const buildTicks = (minSecond, maxSecond, count = 6) => {
  const span = Math.max(1, maxSecond - minSecond);
  return Array.from({ length: count }).map((_, idx) => {
    if (count <= 1) return Math.round(minSecond);
    return Math.round(minSecond + ((span * idx) / (count - 1)));
  });
};

function FirstUnitEfficiencyTimelineRows({ group }) {
  const wrapperRef = useRef(null);
  const [hover, setHover] = useState(null);
  const prepared = useMemo(() => {
    const rows = Array.isArray(group?.rows) ? group.rows : [];
    const withTiming = rows
      .map((row) => {
        const start = Number(row?.building_start_second);
        const ready = Number(row?.building_ready_second);
        const unit = Number(row?.unit_second);
        if (!Number.isFinite(start) || !Number.isFinite(ready) || !Number.isFinite(unit)) return null;
        return {
          ...row,
          building_start_second: start,
          building_ready_second: ready,
          unit_second: unit,
        };
      })
      .filter(Boolean)
      .sort((a, b) => String(a.player_name || '').localeCompare(String(b.player_name || '')));
    if (withTiming.length === 0) {
      return { rows: [], minSecond: 0, maxSecond: 60, ticks: [0, 12, 24, 36, 48, 60] };
    }
    const minStartSecond = withTiming.reduce((minValue, row) => Math.min(minValue, row.building_start_second), Number.POSITIVE_INFINITY);
    const maxUnitSecond = withTiming.reduce((maxValue, row) => Math.max(maxValue, row.unit_second), 0);
    const rawSpan = Math.max(1, maxUnitSecond - minStartSecond);
    const padding = Math.max(6, Math.min(20, Math.round(rawSpan * 0.1)));
    const minSecond = Math.max(0, minStartSecond - padding);
    const maxSecond = Math.max(minSecond + 10, maxUnitSecond + padding);
    return {
      rows: withTiming,
      minSecond,
      maxSecond,
      ticks: buildTicks(minSecond, maxSecond, 6),
    };
  }, [group]);

  if (prepared.rows.length === 0) return null;

  const chartWidth = 980;
  const leftPadding = 260;
  const rightPadding = 24;
  const topPadding = 34;
  const bottomPadding = 42;
  const rowHeight = 34;
  const plotWidth = chartWidth - leftPadding - rightPadding;
  const chartHeight = topPadding + bottomPadding + (prepared.rows.length * rowHeight);
  const xAt = (second) => {
    const bounded = Math.max(prepared.minSecond, Math.min(prepared.maxSecond, Number(second) || prepared.minSecond));
    const span = Math.max(1, prepared.maxSecond - prepared.minSecond);
    return leftPadding + (((bounded - prepared.minSecond) / span) * plotWidth);
  };
  const yAt = (idx) => topPadding + idx * rowHeight + (rowHeight / 2);
  const ticks = prepared.ticks;
  const eventSeconds = useMemo(() => {
    const unique = new Set();
    prepared.rows.forEach((row) => {
      unique.add(row.building_start_second);
      unique.add(row.building_ready_second);
      unique.add(row.unit_second);
    });
    return Array.from(unique).sort((a, b) => a - b);
  }, [prepared.rows]);
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
      <div className="workflow-first-unit-title">
        {group?.building_icon ? (
          <img src={group.building_icon} alt={group?.building_name || 'Building'} className="workflow-first-unit-title-icon workflow-first-unit-title-icon-building" />
        ) : null}
        <span className="workflow-first-unit-title-arrow">→</span>
        {(group?.unit_icons || []).map((icon, idx) => (
          <React.Fragment key={`${group?.id || 'group'}-unit-icon-${idx}`}>
            {idx > 0 ? <span className="workflow-first-unit-title-slash">/</span> : null}
            <img src={icon} alt={group?.unit_names?.[idx] || 'Unit'} className="workflow-first-unit-title-icon workflow-first-unit-title-icon-unit" />
          </React.Fragment>
        ))}
      </div>
      <div ref={wrapperRef} className="workflow-timing-chart-wrap">
        <svg className="workflow-timing-scatter" viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMinYMin meet">
          {eventSeconds.map((second, idx) => (
            <g key={`event-line-${second}`}>
              <line
                x1={xAt(second)}
                y1={topPadding - 14}
                x2={xAt(second)}
                y2={chartHeight - bottomPadding - 8}
                stroke="rgba(255,255,255,0.56)"
                strokeWidth="1"
                strokeDasharray="3 4"
              />
              <text
                x={xAt(second)}
                y={idx % 2 === 0 ? topPadding - 18 : topPadding - 8}
                textAnchor="middle"
                fill="rgba(255,255,255,0.82)"
                fontSize="9"
                className="workflow-timing-inline-label"
              >
                {formatTime(second)}
              </text>
            </g>
          ))}
          {prepared.rows.map((entry, idx) => (
            <g key={`${entry.player_id}-${entry.building_name}-${idx}`}>
              <line
                x1={leftPadding}
                y1={yAt(idx)}
                x2={chartWidth - rightPadding}
                y2={yAt(idx)}
                stroke="rgba(255,255,255,0.1)"
                strokeWidth="1"
              />
              <text
                x={leftPadding - 10}
                y={yAt(idx) + 4}
                textAnchor="end"
                fill="rgba(255,255,255,0.9)"
                fontSize="12"
              >
                {entry.player_name}
              </text>
              <line
                x1={xAt(entry.building_start_second)}
                y1={yAt(idx)}
                x2={xAt(entry.building_ready_second)}
                y2={yAt(idx)}
                stroke="rgba(251, 191, 36, 0.95)"
                strokeWidth="10"
                strokeLinecap="round"
              />
              {Math.abs(xAt(entry.building_ready_second) - xAt(entry.building_start_second)) >= 44 ? (
                <text
                  x={(xAt(entry.building_start_second) + xAt(entry.building_ready_second)) / 2}
                  y={yAt(idx) + 3}
                  textAnchor="middle"
                  fill="rgba(20,20,24,0.95)"
                  fontSize="8"
                  className="workflow-first-unit-buildtime-label"
                >
                  {`${Math.max(0, Number(entry.build_duration_seconds) || 0)}s build time`}
                </text>
              ) : null}
              <line
                x1={xAt(entry.building_ready_second)}
                y1={yAt(idx)}
                x2={xAt(entry.unit_second)}
                y2={yAt(idx)}
                stroke="rgba(239, 68, 68, 0.95)"
                strokeWidth="3.5"
                strokeLinecap="round"
                strokeDasharray="5 4"
              />
              {entry.building_icon ? (
                <g
                  onMouseEnter={(event) => updateHover(event, {
                    playerName: entry.player_name,
                    pointKind: 'Building triggered',
                    time: entry.building_start_second,
                  })}
                  onMouseMove={(event) => updateHover(event, {
                    playerName: entry.player_name,
                    pointKind: 'Building triggered',
                    time: entry.building_start_second,
                  })}
                  onMouseLeave={() => setHover(null)}
                >
                  <image
                    href={entry.building_icon}
                    x={xAt(entry.building_start_second) - 18}
                    y={yAt(idx) - 18}
                    width="36"
                    height="36"
                    preserveAspectRatio="xMidYMid meet"
                  />
                </g>
              ) : null}
              <g
                onMouseEnter={(event) => updateHover(event, {
                  playerName: entry.player_name,
                  pointKind: 'Building ready',
                  time: entry.building_ready_second,
                  duration: entry.build_duration_seconds,
                })}
                onMouseMove={(event) => updateHover(event, {
                  playerName: entry.player_name,
                  pointKind: 'Building ready',
                  time: entry.building_ready_second,
                  duration: entry.build_duration_seconds,
                })}
                onMouseLeave={() => setHover(null)}
              >
                <circle
                  cx={xAt(entry.building_ready_second)}
                  cy={yAt(idx)}
                  r="5.5"
                  fill="rgba(251, 191, 36, 1)"
                  stroke="rgba(9,10,16,0.95)"
                  strokeWidth="1"
                />
              </g>
              {entry.unit_icon ? (
                <g
                  onMouseEnter={(event) => updateHover(event, {
                    playerName: entry.player_name,
                    pointKind: 'Unit created',
                    time: entry.unit_second,
                    gap: entry.gap_after_ready_seconds,
                  })}
                  onMouseMove={(event) => updateHover(event, {
                    playerName: entry.player_name,
                    pointKind: 'Unit created',
                    time: entry.unit_second,
                    gap: entry.gap_after_ready_seconds,
                  })}
                  onMouseLeave={() => setHover(null)}
                >
                  <image
                    href={entry.unit_icon}
                    x={xAt(entry.unit_second) - 13.5}
                    y={yAt(idx) - 13.5}
                    width="27"
                    height="27"
                    preserveAspectRatio="xMidYMid meet"
                  />
                </g>
              ) : null}
              <text
                x={xAt(entry.unit_second) + 8}
                y={yAt(idx) + 4}
                fill="rgba(239, 68, 68, 0.95)"
                fontSize="9.8"
                className="workflow-timing-inline-label"
              >
                {`+${Math.max(0, Number(entry.gap_after_ready_seconds) || 0)}s`}
              </text>
            </g>
          ))}

          {ticks.map((second) => (
            <g key={`tick-${second}`}>
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
            <div><strong>{hover.playerName}</strong></div>
            <div><strong>Point</strong> {hover.pointKind}</div>
            <div><strong>Time</strong> {formatTime(hover.time)}</div>
            {Number.isFinite(Number(hover.duration)) ? <div><strong>Build time</strong> {Math.max(0, Number(hover.duration) || 0)}s</div> : null}
            {Number.isFinite(Number(hover.gap)) ? <div><strong>Delay</strong> +{Math.max(0, Number(hover.gap) || 0)}s</div> : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export default FirstUnitEfficiencyTimelineRows;
