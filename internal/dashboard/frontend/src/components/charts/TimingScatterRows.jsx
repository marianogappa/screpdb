import React, { useMemo, useRef, useState } from 'react';

const PLAYER_COLORS = ['#7dd3fc', '#a7f3d0', '#f9a8d4', '#fcd34d', '#c4b5fd', '#fca5a5'];
const LABEL_COLORS = ['#60a5fa', '#34d399', '#f472b6', '#f59e0b', '#a78bfa', '#ef4444', '#22d3ee', '#84cc16', '#f97316', '#e879f9', '#14b8a6', '#f43f5e'];

const formatTime = (seconds) => {
  const value = Math.max(0, Math.floor(Number(seconds) || 0));
  return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
};

const buildTimeTicks = (maxSecond) => (
  Array.from({ length: 7 }).map((_, i) => Math.round((maxSecond * i) / 6))
);

const colorForLabel = (key) => {
  const text = String(key || '').trim().toLowerCase();
  if (!text) return LABEL_COLORS[0];
  let hash = 0;
  for (let i = 0; i < text.length; i += 1) {
    hash = ((hash << 5) - hash) + text.charCodeAt(i);
    hash |= 0;
  }
  return LABEL_COLORS[Math.abs(hash) % LABEL_COLORS.length];
};

// Truncate over-long player names so the row label never eats into the plot
// area. The full name stays available via the row's <title> tooltip.
const truncateName = (name, max = 16) => {
  const text = String(name || '');
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
};

function TimingScatterRows({
  title,
  series,
  durationSeconds,
  colorByLabel = false,
  colorBy,
  showLegend = false,
  markerMode = 'dot',
  inlineLegend = false,
  noticeText = '',
  rowLabelMode = 'race-suffix',
  rowGroupingMode = 'none',
}) {
  const wrapperRef = useRef(null);
  const [hover, setHover] = useState(null);
  // colorBy supersedes the legacy colorByLabel boolean: 'player' tints by
  // player, 'label' hashes the item label (legacy), 'category' uses each
  // point's precomputed overlay colour so overlaid timing categories read as
  // distinct colour families.
  const colorMode = colorBy || (colorByLabel ? 'label' : 'player');
  const prepared = useMemo(() => {
    const inputSeries = Array.isArray(series) ? series : [];
    const players = [];
    const points = [];
    const legendEntries = new Map();
    let maxSecond = Number(durationSeconds) || 0;

    inputSeries.forEach((playerSeries, playerIndex) => {
      const playerName = String(playerSeries?.name || '').trim() || `Player ${playerIndex + 1}`;
      const playerRace = String(playerSeries?.race || '').trim();
      const playerRaceIcon = String(playerSeries?.race_icon || '').trim();
      const playerColor = PLAYER_COLORS[playerIndex % PLAYER_COLORS.length];
      players.push({ name: playerName, race: playerRace, raceIcon: playerRaceIcon });
      (playerSeries?.points || []).forEach((point) => {
        const second = Number(point?.second);
        if (!Number.isFinite(second)) return;
        maxSecond = Math.max(maxSecond, second);
        const label = String(point?.label || '').trim();
        const displayLabel = String(point?.display_label || '').trim() || label;
        const labelKey = displayLabel || label || `Timing #${Number(point?.order) || 1}`;
        const categoryLabel = String(point?.category_label || '').trim();
        const overlayColor = String(point?.overlay_color || '').trim();
        const pointColor = colorMode === 'label'
          ? colorForLabel(labelKey)
          : colorMode === 'category'
            ? (overlayColor || playerColor)
            : playerColor;
        const legendKey = colorMode === 'category' ? categoryLabel : labelKey;
        if (showLegend && colorMode !== 'player' && legendKey) {
          legendEntries.set(legendKey, pointColor);
        }
        points.push({
          ...point,
          playerName,
          playerRace,
          playerIndex,
          color: pointColor,
          second,
          label,
          displayLabel,
          order: Number(point?.order) || 0,
        });
      });
    });

    const legendItems = [...legendEntries.entries()]
      .map(([label, color]) => ({ label, color }))
      .sort((a, b) => a.label.localeCompare(b.label));
    return { players, points, legendItems, maxSecond: Math.max(60, maxSecond) };
  }, [series, durationSeconds, colorMode, showLegend]);

  const players = prepared.players;
  if (players.length === 0) {
    return (
      <div className="workflow-card timing-chart-card">
        {title ? <div className="workflow-card-title"><span>{title}</span></div> : null}
        <div className="chart-empty">No timing data found.</div>
      </div>
    );
  }

  const chartWidth = 980;
  const rowHeight = 36;
  const rowGroupGap = rowGroupingMode === 'race' ? 12 : 0;
  const raceIconSize = rowLabelMode === 'worker-icon' ? 30 : 0;
  const raceIconGap = rowLabelMode === 'worker-icon' ? 10 : 0;
  const topPadding = 20;
  const bottomPadding = 42;
  const leftPadding = rowLabelMode === 'worker-icon' ? 168 : 150;
  const rightPadding = 24;
  const rowOffsets = [];
  let accumulatedGroupGap = 0;
  players.forEach((player, idx) => {
    if (idx > 0 && rowGroupingMode === 'race' && player.race !== players[idx - 1].race) {
      accumulatedGroupGap += rowGroupGap;
    }
    rowOffsets.push(accumulatedGroupGap);
  });
  const chartHeight = topPadding + bottomPadding + (players.length * rowHeight) + accumulatedGroupGap;
  const plotWidth = chartWidth - leftPadding - rightPadding;
  const xAt = (second) => {
    const bounded = Math.max(0, Number(second) || 0);
    return leftPadding + (bounded / prepared.maxSecond) * plotWidth;
  };
  const yAt = (playerIndex) => topPadding + playerIndex * rowHeight + (rowOffsets[playerIndex] || 0) + rowHeight / 2;
  const xTicks = buildTimeTicks(prepared.maxSecond);
  const updateHover = (event, point) => {
    if (!wrapperRef.current) return;
    const rect = wrapperRef.current.getBoundingClientRect();
    setHover({
      x: event.clientX - rect.left + 12,
      y: event.clientY - rect.top + 10,
      point,
    });
  };

  return (
    <div className="workflow-card timing-chart-card">
      {title ? <div className="workflow-card-title"><span>{title}</span></div> : null}
      {noticeText ? (
        <div className="workflow-timing-notice">{noticeText}</div>
      ) : null}
      {showLegend && prepared.legendItems.length > 0 ? (
        <div className="workflow-timing-legend">
          {prepared.legendItems.map((item) => (
            <div key={item.label} className="workflow-timing-legend-item">
              <span className="workflow-timing-legend-swatch" style={{ backgroundColor: item.color }} />
              <span>{item.label}</span>
            </div>
          ))}
        </div>
      ) : null}
      <div ref={wrapperRef} className="workflow-timing-chart-wrap">
        <svg className="workflow-timing-scatter" viewBox={`0 0 ${chartWidth} ${chartHeight}`} preserveAspectRatio="xMinYMin meet">
          {players.map((player, idx) => (
            <g key={`row-${player.name}-${idx}`}>
              <line
                x1={leftPadding}
                y1={yAt(idx)}
                x2={chartWidth - rightPadding}
                y2={yAt(idx)}
                stroke="rgba(255,255,255,0.1)"
                strokeWidth="1"
              />
              <text
                x={rowLabelMode === 'worker-icon' ? leftPadding - raceIconSize - raceIconGap : leftPadding - 10}
                y={yAt(idx) + 4}
                textAnchor="end"
                fill="rgba(255,255,255,0.9)"
                fontSize="12"
              >
                <title>{player.race ? `${player.name} (${player.race})` : player.name}</title>
                {rowLabelMode === 'worker-icon' || rowLabelMode === 'name-only'
                  ? truncateName(player.name)
                  : (player.race ? `${truncateName(player.name, 13)} (${player.race})` : truncateName(player.name))}
              </text>
              {rowLabelMode === 'worker-icon' && player.raceIcon ? (
                <image
                  href={player.raceIcon}
                  x={leftPadding - raceIconSize}
                  y={yAt(idx) - raceIconSize / 2}
                  width={String(raceIconSize)}
                  height={String(raceIconSize)}
                  preserveAspectRatio="xMidYMid meet"
                />
              ) : null}
            </g>
          ))}

          {xTicks.map((second) => {
            const x = xAt(second);
            return (
              <g key={`tick-${second}`}>
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
            <g
              key={`point-${idx}`}
              onMouseEnter={(event) => updateHover(event, point)}
              onMouseMove={(event) => updateHover(event, point)}
              onMouseLeave={() => setHover(null)}
            >
              {markerMode === 'image' && point.marker_image ? (
                <image
                  href={point.marker_image}
                  x={xAt(point.second) - 9}
                  y={yAt(point.playerIndex) - 9}
                  width="18"
                  height="18"
                  preserveAspectRatio="xMidYMid meet"
                />
              ) : (
                <circle
                  cx={xAt(point.second)}
                  cy={yAt(point.playerIndex)}
                  r="5"
                  fill={point.color}
                  stroke="rgba(10,10,15,0.95)"
                  strokeWidth="1.5"
                />
              )}
              {inlineLegend && point.displayLabel ? (
                <text
                  x={xAt(point.second) + 8}
                  y={yAt(point.playerIndex) + (idx % 2 === 0 ? -8 : 14)}
                  fill={point.color}
                  fontSize="9.5"
                  className="workflow-timing-inline-label"
                >
                  {point.displayLabel}
                </text>
              ) : null}
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
        {hover ? (
          <div
            className="workflow-timing-tooltip"
            style={{ left: `${hover.x}px`, top: `${hover.y}px` }}
          >
            <div><strong>{hover.point.playerName}</strong>{hover.point.playerRace ? ` (${hover.point.playerRace})` : ''}</div>
            <div><strong>Time</strong> {formatTime(hover.point.second)}</div>
            {hover.point.displayLabel ? <div><strong>Item</strong> {hover.point.displayLabel}</div> : null}
            {hover.point.category_label ? <div><strong>Category</strong> {hover.point.category_label}</div> : null}
            {hover.point.order > 0 ? <div><strong>Occurrence</strong> #{hover.point.order}</div> : null}
            {hover.point.is_repeatable ? <div><strong>Level</strong> {`L${hover.point.order}/${hover.point.max_level || 3}`}</div> : null}
          </div>
        ) : null}
      </div>
    </div>
  );
}

export default TimingScatterRows;
