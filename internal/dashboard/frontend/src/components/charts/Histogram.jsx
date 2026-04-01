import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';

const DEFAULT_COLORS = ['#4e79a7'];

function Histogram({ data, config }) {
  const svgRef = useRef(null);
  const containerRef = useRef(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });
  const [tooltip, setTooltip] = useState({ visible: false, x: 0, y: 0, text: '' });

  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        const { width, height } = containerRef.current.getBoundingClientRect();
        const newWidth = Math.max(300, width);
        const newHeight = Math.max(300, height);
        setDimensions(prev => {
          if (Math.abs(prev.width - newWidth) > 1 || Math.abs(prev.height - newHeight) > 1) {
            return { width: newWidth, height: newHeight };
          }
          return prev;
        });
      }
    };

    updateDimensions();
    const resizeObserver = new ResizeObserver(updateDimensions);
    if (containerRef.current) {
      resizeObserver.observe(containerRef.current);
    }

    return () => {
      resizeObserver.disconnect();
    };
  }, []);

  useEffect(() => {
    if (dimensions.width === 0 || !svgRef.current) {
      return;
    }

    const margin = { top: 20, right: 30, bottom: 60, left: 60 };
    const width = dimensions.width - margin.left - margin.right;
    const height = dimensions.height - margin.top - margin.bottom;

    if (width <= 0 || height <= 0) {
      return;
    }

    // Clear existing content
    d3.select(svgRef.current).selectAll('*').remove();

    const svg = d3.select(svgRef.current)
      .attr('width', width + margin.left + margin.right)
      .attr('height', height + margin.top + margin.bottom)
      .append('g')
      .attr('transform', `translate(${margin.left}, ${margin.top})`);
    setTooltip((prev) => ({ ...prev, visible: false }));

    const hasPrecomputedBins = Array.isArray(config?.precomputed_bins) && config.precomputed_bins.length > 0;

    let binsData = [];
    if (hasPrecomputedBins) {
      binsData = config.precomputed_bins
        .map((bin) => ({
          x0: Number(bin?.x0),
          x1: Number(bin?.x1),
          count: Number(bin?.count) || 0,
        }))
        .filter((bin) => Number.isFinite(bin.x0) && Number.isFinite(bin.x1) && bin.x1 >= bin.x0);
    } else {
      if (!data || data.length === 0 || !config?.histogram_value_column) return;
      const values = data.map((d) => Number(d[config.histogram_value_column]) || 0).filter((v) => !Number.isNaN(v));
      if (values.length === 0) return;
      const bins = config.histogram_bins || Math.ceil(Math.sqrt(values.length));
      binsData = d3.bin()
        .domain(d3.extent(values))
        .thresholds(bins)(values)
        .map((bin) => ({
          x0: bin.x0,
          x1: bin.x1,
          count: bin.length,
        }));
    }

    if (binsData.length === 0) return;

    const xScale = d3.scaleLinear()
      .domain([binsData[0].x0, binsData[binsData.length - 1].x1])
      .range([0, width]);

    const mean = Number(config?.mean);
    const stddev = Number(config?.stddev);
    if (config?.style === 'monobell_relax' && Number.isFinite(mean) && Number.isFinite(stddev) && stddev > 0) {
      const normalPDF = (value) => {
        const z = (value - mean) / stddev;
        return (1 / (stddev * Math.sqrt(2 * Math.PI))) * Math.exp(-0.5 * z * z);
      };
      const samples = d3.range(0, 240).map((idx) => {
        const t = idx / 239;
        const xValue = xScale.domain()[0] + t * (xScale.domain()[1] - xScale.domain()[0]);
        return { x: xValue, p: normalPDF(xValue) };
      });
      const yScale = d3.scaleLinear()
        .domain([0, Math.max(1e-6, Number(d3.max(samples, (item) => item.p) || 0))])
        .range([height, 0]);

      svg.append('g')
        .attr('transform', `translate(0, ${height})`)
        .call(d3.axisBottom(xScale).tickSizeOuter(0))
        .selectAll('text')
        .attr('fill', '#fff');
      svg.append('g')
        .call(d3.axisLeft(yScale).ticks(6))
        .selectAll('text')
        .attr('fill', '#fff');

      const lineBuilder = d3.line()
        .x((item) => xScale(item.x))
        .y((item) => yScale(item.p))
        .curve(d3.curveMonotoneX);
      svg.append('path')
        .datum(samples)
        .attr('fill', 'none')
        .attr('stroke', '#dbeafe')
        .attr('stroke-width', 2.8)
        .attr('opacity', 0.95)
        .attr('d', lineBuilder);

      svg.append('line')
        .attr('x1', xScale(mean))
        .attr('x2', xScale(mean))
        .attr('y1', 0)
        .attr('y2', height)
        .attr('stroke', '#f59e0b')
        .attr('stroke-width', 2)
        .attr('stroke-dasharray', '6,4');

      const overlayPoints = (Array.isArray(config?.overlay_points) ? config.overlay_points : [])
        .map((point) => ({
          value: Number(point?.value),
          label: String(point?.label || '').trim(),
          key: String(point?.player_key || point?.label || '').trim(),
          gamesPlayed: Number(point?.games_played || 0),
          tooltipLines: Array.isArray(point?.tooltip_lines)
            ? point.tooltip_lines.map((line) => String(line || '').trim()).filter((line) => line)
            : [],
        }))
        .filter((point) => Number.isFinite(point.value) && point.label)
        .sort((a, b) => a.value - b.value);
      const overlayValueLabel = String(config?.overlay_value_label || '').trim() || 'APM';
      const overlayCountLabel = String(config?.overlay_count_label || '').trim() || 'games';

      const palette = ['#60a5fa', '#f472b6', '#34d399', '#f59e0b', '#a78bfa', '#22d3ee', '#f87171', '#10b981', '#fb7185', '#c4b5fd'];
      const placements = overlayPoints.map((point, idx) => {
        const fullLabel = point.label;
        const label = fullLabel.length > 22 ? `${fullLabel.slice(0, 21)}...` : fullLabel;
        return {
          ...point,
          color: palette[(idx * 7) % palette.length],
          fullLabel,
          label,
          labelW: Math.max(34, Math.min(220, label.length * 7.6)),
          labelH: 15,
          xPixel: xScale(point.value),
          yCurve: yScale(normalPDF(point.value)),
          dotY: yScale(normalPDF(point.value)),
          labelX: xScale(point.value) + 8,
          labelY: yScale(normalPDF(point.value)),
        };
      });

      const laneCount = 30;
      const laneTop = 14;
      const laneBottom = height - 14;
      const laneYs = Array.from({ length: laneCount }, (_, idx) => laneTop + (idx / Math.max(1, laneCount - 1)) * (laneBottom - laneTop));
      const laneLastRightX = new Array(laneCount).fill(-1e9);
      const minGap = 14;
      placements.forEach((point, idx) => {
        let bestLane = idx % laneCount;
        let bestScore = Infinity;
        for (let lane = 0; lane < laneCount; lane += 1) {
          const free = point.xPixel + 8 - laneLastRightX[lane];
          const gapPenalty = Math.max(0, minGap - free) * 8;
          const distPenalty = Math.abs(laneYs[lane] - point.yCurve) * 0.6;
          const score = gapPenalty + distPenalty;
          if (score < bestScore) {
            bestScore = score;
            bestLane = lane;
          }
        }
        point.dotY = laneYs[bestLane];
        point.labelY = laneYs[bestLane] - 1;
        point.labelX = Math.min(width - point.labelW - 2, point.xPixel + 8);
        laneLastRightX[bestLane] = point.labelX + point.labelW;
      });

      for (let iter = 0; iter < 260; iter += 1) {
        let moved = false;
        for (let i = 0; i < placements.length; i += 1) {
          for (let j = i + 1; j < placements.length; j += 1) {
            const a = placements[i];
            const b = placements[j];
            const ax1 = a.labelX;
            const ax2 = a.labelX + a.labelW;
            const ay1 = a.labelY - a.labelH;
            const ay2 = a.labelY + 2;
            const bx1 = b.labelX;
            const bx2 = b.labelX + b.labelW;
            const by1 = b.labelY - b.labelH;
            const by2 = b.labelY + 2;
            const overlapX = ax1 < bx2 && ax2 > bx1;
            const overlapY = ay1 < by2 && ay2 > by1;
            if (!overlapX || !overlapY) continue;
            const dir = a.labelY <= b.labelY ? -1 : 1;
            a.labelY = Math.max(12, Math.min(height - 4, a.labelY + dir * 0.95));
            b.labelY = Math.max(12, Math.min(height - 4, b.labelY - dir * 0.95));
            moved = true;
          }
        }
        if (!moved) break;
      }
      placements.forEach((point) => {
        point.dotY = Math.max(10, Math.min(height - 6, point.labelY + 1));
        point.labelY = Math.max(12, Math.min(height - 4, point.labelY));
      });

      const layer = svg.append('g');
      layer.selectAll('circle')
        .data(placements)
        .enter()
        .append('circle')
        .attr('cx', (point) => point.xPixel)
        .attr('cy', (point) => point.dotY)
        .attr('r', 3.2)
        .attr('fill', (point) => point.color)
        .attr('stroke', 'rgba(255,255,255,0.92)')
        .attr('stroke-width', 1)
        .on('mousemove', (event, point) => {
          const fallback = `${point.fullLabel} - ${point.value.toFixed(1)} ${overlayValueLabel} (${point.gamesPlayed} ${overlayCountLabel})`;
          setTooltip({
            visible: true,
            x: event.clientX + 10,
            y: event.clientY + 10,
            text: point.tooltipLines.length > 0 ? point.tooltipLines.join('\n') : fallback,
          });
        })
        .on('mouseleave', () => setTooltip((prev) => ({ ...prev, visible: false })));

      layer.selectAll('text')
        .data(placements)
        .enter()
        .append('text')
        .attr('x', (point) => point.labelX)
        .attr('y', (point) => point.labelY)
        .attr('fill', (point) => point.color)
        .attr('font-size', '14px')
        .attr('font-weight', 680)
        .attr('paint-order', 'stroke')
        .attr('stroke', 'rgba(8,11,20,0.98)')
        .attr('stroke-width', 4)
        .text((point) => point.label)
        .on('mousemove', (event, point) => {
          const fallback = `${point.fullLabel} - ${point.value.toFixed(1)} ${overlayValueLabel} (${point.gamesPlayed} ${overlayCountLabel})`;
          setTooltip({
            visible: true,
            x: event.clientX + 10,
            y: event.clientY + 10,
            text: point.tooltipLines.length > 0 ? point.tooltipLines.join('\n') : fallback,
          });
        })
        .on('mouseleave', () => setTooltip((prev) => ({ ...prev, visible: false })));
    } else {
      const yMax = Math.max(1, Number(d3.max(binsData, (d) => d.count) || 0));
      const yScale = d3.scaleLinear()
        .domain([0, yMax])
        .range([height, 0]);

      const xAxis = d3.axisBottom(xScale).tickSizeOuter(0);
      svg.append('g')
        .attr('transform', `translate(0, ${height})`)
        .call(xAxis)
        .selectAll('text')
        .attr('fill', '#fff');
      svg.append('g')
        .call(d3.axisLeft(yScale))
        .selectAll('text')
        .attr('fill', '#fff');

      if (Number.isFinite(mean) && Number.isFinite(stddev) && stddev > 0) {
        const bandStart = Math.max(xScale.domain()[0], mean - stddev);
        const bandEnd = Math.min(xScale.domain()[1], mean + stddev);
        if (bandEnd > bandStart) {
          svg.append('rect')
            .attr('x', xScale(bandStart))
            .attr('y', 0)
            .attr('width', xScale(bandEnd) - xScale(bandStart))
            .attr('height', height)
            .attr('fill', '#9ec5ff')
            .attr('opacity', 0.12);
        }
      }

      svg.selectAll('.bar')
        .data(binsData)
        .enter()
        .append('rect')
        .attr('class', 'bar')
        .attr('x', (d) => xScale(d.x0))
        .attr('y', (d) => yScale(d.count))
        .attr('width', (d) => Math.max(0, xScale(d.x1) - xScale(d.x0) - 1))
        .attr('height', (d) => height - yScale(d.count))
        .attr('fill', DEFAULT_COLORS[0])
        .attr('opacity', 0.7);

      const curvePoints = binsData.map((bin) => ({
        x: (Number(bin.x0) + Number(bin.x1)) / 2,
        y: Number(bin.count) || 0,
      }));
      const lineBuilder = d3.line()
        .x((d) => xScale(d.x))
        .y((d) => yScale(d.y))
        .curve(d3.curveCatmullRom.alpha(0.5));
      if (curvePoints.length >= 2) {
        svg.append('path')
          .datum(curvePoints)
          .attr('fill', 'none')
          .attr('stroke', '#e5ecff')
          .attr('stroke-width', 2)
          .attr('opacity', 0.85)
          .attr('d', lineBuilder);
      }

      if (Number.isFinite(mean)) {
        svg.append('line')
          .attr('x1', xScale(mean))
          .attr('x2', xScale(mean))
          .attr('y1', 0)
          .attr('y2', height)
          .attr('stroke', '#f59e0b')
          .attr('stroke-width', 2)
          .attr('stroke-dasharray', '5,4');
      }
    }

    svg.append("text")
      .attr("text-anchor", "end")
      .attr("x", width)
      .attr("y", height + 35)
      .attr('fill', '#fff')
      .attr('font-size', '12px')
      .text(config?.x_axis_label || config?.histogram_value_column || 'Value');

    svg.append("text")
      .attr("text-anchor", "middle")
      .attr("x", -height / 2)
      .attr("y", -44)
      .attr("transform", "rotate(-90)")
      .attr('fill', '#fff')
      .attr('font-size', '12px')
      .text(config?.y_axis_label || 'Count');

  }, [data, config, dimensions]);

  const hasPrecomputedBins = Array.isArray(config?.precomputed_bins) && config.precomputed_bins.length > 0;
  if ((!data || data.length === 0) && !hasPrecomputedBins) {
    return <div className="chart-empty">No data available</div>;
  }

  const chartHeight = Number(config?.chart_height);
  const resolvedHeight = Number.isFinite(chartHeight) && chartHeight > 0 ? `${Math.round(chartHeight)}px` : '100%';
  const minHeight = Number.isFinite(chartHeight) && chartHeight > 0 ? `${Math.round(chartHeight)}px` : '300px';
  return (
    <div ref={containerRef} style={{ width: '100%', height: resolvedHeight, minHeight, overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="histogram" style={{ width: '100%', height: '100%', display: 'block' }} />
      {tooltip.visible ? (
        <div
          style={{
            position: 'fixed',
            left: `${tooltip.x}px`,
            top: `${tooltip.y}px`,
            background: 'rgba(7, 11, 22, 0.96)',
            border: '1px solid rgba(170, 196, 255, 0.45)',
            color: '#e7efff',
            borderRadius: '7px',
            padding: '6px 8px',
            fontSize: '12px',
            pointerEvents: 'none',
            zIndex: 9999,
            whiteSpace: 'pre-line',
          }}
        >
          {tooltip.text}
        </div>
      ) : null}
    </div>
  );
}

export default Histogram;

