import React, { useEffect } from 'react';
import * as d3 from 'd3';
import { useChartDimensions } from '../../hooks/useChartDimensions';
import { DEFAULT_COLORS } from '../../constants/chartTypes';

function LineChart({ data, config }) {
  const { containerRef, svgRef, dimensions } = useChartDimensions();

  useEffect(() => {
    if (!data || data.length === 0 || !config.line_x_column || !config.line_y_columns || config.line_y_columns.length === 0 || dimensions.width === 0) {
      return;
    }

    const margin = { top: 20, right: 80, bottom: 60, left: 60 };
    const width = dimensions.width - margin.left - margin.right;
    const height = dimensions.height - margin.top - margin.bottom;

    d3.select(svgRef.current).selectAll('*').remove();

    const svg = d3.select(svgRef.current)
      .attr('width', width + margin.left + margin.right)
      .attr('height', height + margin.top + margin.bottom)
      .append('g')
      .attr('transform', `translate(${margin.left}, ${margin.top})`);

    const colors = d3.scaleOrdinal(DEFAULT_COLORS);

    const parseX = (val) => {
      if (config.line_x_axis_type === 'timestamp') return new Date(val).getTime();
      return Number(val) || 0;
    };

    const xScale = d3.scaleLinear()
      .domain(d3.extent(data, d => parseX(d[config.line_x_column])))
      .range([0, width]);

    const allYValues = [];
    config.line_y_columns.forEach(col => {
      data.forEach(d => allYValues.push(Number(d[col]) || 0));
    });

    const yScale = d3.scaleLinear()
      .domain(config.line_y_axis_from_zero ? [0, d3.max(allYValues)] : d3.extent(allYValues))
      .range([height, 0]);

    config.line_y_columns.forEach((col, colIdx) => {
      const lineData = data.map(d => ({ ...d, yValue: Number(d[col]) || 0 }));

      const lineGen = d3.line()
        .x(d => xScale(parseX(d[config.line_x_column])))
        .y(d => yScale(d.yValue))
        .curve(d3.curveMonotoneX);

      svg.append('path').datum(lineData)
        .attr('fill', 'none').attr('stroke', colors(colIdx)).attr('stroke-width', 2).attr('d', lineGen);

      svg.selectAll(`.dot-${colIdx}`).data(lineData).enter().append('circle')
        .attr('class', `dot-${colIdx}`)
        .attr('cx', d => xScale(parseX(d[config.line_x_column])))
        .attr('cy', d => yScale(d.yValue))
        .attr('r', 3).attr('fill', colors(colIdx));
    });

    svg.append('g').attr('transform', `translate(0, ${height})`).call(d3.axisBottom(xScale)).selectAll('text').attr('fill', '#fff');
    svg.append('g').call(d3.axisLeft(yScale)).selectAll('text').attr('fill', '#fff');

    svg.append('text').attr('text-anchor', 'end').attr('x', width).attr('y', height + 35)
      .attr('fill', '#fff').attr('font-size', '12px').text(config?.line_x_column);

    const legend = svg.append('g').attr('transform', `translate(${width - 70}, 20)`);
    config.line_y_columns.forEach((col, i) => {
      const item = legend.append('g').attr('transform', `translate(0, ${i * 20})`);
      item.append('line').attr('x1', 0).attr('x2', 15).attr('stroke', colors(i)).attr('stroke-width', 2);
      item.append('text').attr('x', 20).attr('y', 4).attr('fill', '#fff').attr('font-size', '11px').text(col);
    });
  }, [data, config, dimensions]);

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="line-chart" style={{ width: '100%', height: '100%', display: 'block' }} />
    </div>
  );
}

export default LineChart;
