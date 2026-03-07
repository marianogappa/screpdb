import React, { useEffect, useId } from 'react';
import * as d3 from 'd3';
import { useChartDimensions } from '../../hooks/useChartDimensions';

function Heatmap({ data, config }) {
  const gradientId = useId();
  const { containerRef, svgRef, dimensions } = useChartDimensions();

  useEffect(() => {
    if (!data || data.length === 0 || !config.heatmap_x_column || !config.heatmap_y_column || !config.heatmap_value_column || dimensions.width === 0) {
      return;
    }

    const margin = { top: 20, right: 80, bottom: 60, left: 100 };
    const width = dimensions.width - margin.left - margin.right;
    const height = dimensions.height - margin.top - margin.bottom;

    d3.select(svgRef.current).selectAll('*').remove();

    const svg = d3.select(svgRef.current)
      .attr('width', width + margin.left + margin.right)
      .attr('height', height + margin.top + margin.bottom)
      .append('g')
      .attr('transform', `translate(${margin.left}, ${margin.top})`);

    const xCategories = [...new Set(data.map(d => String(d[config.heatmap_x_column])))].sort();
    const yCategories = [...new Set(data.map(d => String(d[config.heatmap_y_column])))].sort();

    const xScale = d3.scaleBand().domain(xCategories).range([0, width]).padding(0.05);
    const yScale = d3.scaleBand().domain(yCategories).range([0, height]).padding(0.05);

    const values = data.map(d => Number(d[config.heatmap_value_column]) || 0);
    const colorScale = d3.scaleSequential(d3.interpolateViridis).domain(d3.extent(values));

    const valueMap = new Map();
    data.forEach(d => {
      valueMap.set(`${d[config.heatmap_x_column]}_${d[config.heatmap_y_column]}`, Number(d[config.heatmap_value_column]) || 0);
    });

    svg.selectAll('.cell')
      .data(xCategories.flatMap(x => yCategories.map(y => ({ x, y }))))
      .enter().append('rect')
      .attr('class', 'cell')
      .attr('x', d => xScale(d.x)).attr('y', d => yScale(d.y))
      .attr('width', xScale.bandwidth()).attr('height', yScale.bandwidth())
      .attr('fill', d => { const v = valueMap.get(`${d.x}_${d.y}`); return v !== undefined ? colorScale(v) : '#333'; })
      .attr('stroke', '#1a1a1a').attr('stroke-width', 1)
      .on('mouseover', function () { d3.select(this).attr('opacity', 0.8); })
      .on('mouseout', function () { d3.select(this).attr('opacity', 1); });

    svg.append('g').attr('transform', `translate(0, ${height})`).call(d3.axisBottom(xScale))
      .selectAll('text').attr('fill', '#fff').attr('transform', 'rotate(-45)').style('text-anchor', 'end');
    svg.append('g').call(d3.axisLeft(yScale)).selectAll('text').attr('fill', '#fff');

    const legendWidth = 20;
    const legendHeight = 200;
    const legend = svg.append('g').attr('transform', `translate(${width + 20}, 0)`);
    const legendScale = d3.scaleLinear().domain(d3.extent(values)).range([legendHeight, 0]);
    legend.append('g').call(d3.axisRight(legendScale).ticks(5)).selectAll('text').attr('fill', '#fff');

    const defs = svg.append('defs');
    const gradient = defs.append('linearGradient').attr('id', gradientId)
      .attr('x1', '0%').attr('x2', '0%').attr('y1', '0%').attr('y2', '100%');

    const stops = 10;
    for (let i = 0; i <= stops; i++) {
      const value = d3.extent(values)[0] + (d3.extent(values)[1] - d3.extent(values)[0]) * (i / stops);
      gradient.append('stop').attr('offset', `${(i / stops) * 100}%`).attr('stop-color', colorScale(value));
    }

    legend.append('rect').attr('width', legendWidth).attr('height', legendHeight).style('fill', `url(#${gradientId})`);
  }, [data, config, dimensions, gradientId]);

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="heatmap" style={{ width: '100%', height: '100%', display: 'block' }} />
    </div>
  );
}

export default Heatmap;
