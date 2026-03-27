import React, { useMemo } from 'react';

const PLOT_COLORS = ['#7dd3fc', '#a7f3d0', '#f9a8d4', '#fcd34d', '#c4b5fd', '#fca5a5'];

function TimingScatterRows({ title, series, durationSeconds }) {
  const prepared = useMemo(() => {
    const inputSeries = Array.isArray(series) ? series : [];
    const players = inputSeries.map((s) => String(s?.name || '').trim()).filter(Boolean);
    const points = [];
    let maxSecond = Number(durationSeconds) || 0;

    inputSeries.forEach((playerSeries, playerIndex) => {
      const playerName = String(playerSeries?.name || '').trim() || `Player ${playerIndex + 1}`;
      const playerColor = PLOT_COLORS[playerIndex % PLOT_COLORS.length];
      (playerSeries?.points || []).forEach((point) => {
        const second = Number(point?.second);
        if (!Number.isFinite(second)) return;
        maxSecond = Math.max(maxSecond, second);
        points.push({
          playerName,
          playerIndex,
          color: playerColor,
          second,
          label: String(point?.label || '').trim(),
          order: Number(point?.order) || 0,
        });
      });
    });

    return { players, points, maxSecond: Math.max(60, maxSecond) };
  }, [series, durationSeconds]);

  const players = prepared.players;
  if (players.length === 0) {
    return (
      <div className="workflow-card timing-chart-card">
        <div className="workflow-card-title"><span>{title}</span></div>
        <div className="chart-empty">No timing data found.</div>
      </div>
    );
  }

  const chartWidth = 980;
  const rowHeight = 36;
  const topPadding = 20;
  const bottomPadding = 40;
  const leftPadding = 180;
  const rightPadding = 24;
  const chartHeight = topPadding + bottomPadding + (players.length * rowHeight);
  const plotWidth = chartWidth - leftPadding - rightPadding;
  const xTicks = 6;

  const xAt = (second) => leftPadding + (Math.max(0, second) / prepared.maxSecond) * plotWidth;
  const yAt = (playerIndex) => topPadding + playerIndex * rowHeight + rowHeight / 2;

  return (
    <div className="workflow-card timing-chart-card">
      <div className="workflow-card-title"><span>{title}</span></div>
      <svg className="workflow-timing-scatter" viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMinYMin meet">
        {players.map((name, idx) => (
          <g key={`row-${name}-${idx}`}>
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
              {name}
            </text>
          </g>
        ))}

        {Array.from({ length: xTicks + 1 }).map((_, i) => {
          const second = Math.round((prepared.maxSecond * i) / xTicks);
          const x = xAt(second);
          return (
            <g key={`tick-${i}`}>
              <line
                x1={x}
                y1={topPadding - 6}
                x2={x}
                y2={chartHeight - bottomPadding + 6}
                stroke="rgba(255,255,255,0.14)"
                strokeWidth="1"
              />
              <text
                x={x}
                y={chartHeight - bottomPadding + 20}
                textAnchor="middle"
                fill="rgba(255,255,255,0.75)"
                fontSize="11"
              >
                {Math.floor(second / 60)}m
              </text>
            </g>
          );
        })}

        {prepared.points.map((point, idx) => (
          <g key={`point-${idx}`}>
            <circle
              cx={xAt(point.second)}
              cy={yAt(point.playerIndex)}
              r="5"
              fill={point.color}
              stroke="rgba(10,10,15,0.95)"
              strokeWidth="1.5"
            />
            <title>
              {`${point.playerName}: ${point.label ? `${point.label} ` : ''}#${point.order} at ${Math.floor(point.second / 60)}:${String(point.second % 60).padStart(2, '0')}`}
            </title>
          </g>
        ))}

        <text
          x={leftPadding + plotWidth / 2}
          y={chartHeight - 8}
          textAnchor="middle"
          fill="rgba(255,255,255,0.8)"
          fontSize="12"
        >
          Game time
        </text>
      </svg>
    </div>
  );
}

export default TimingScatterRows;
