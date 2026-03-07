import React, { useEffect } from 'react';
import * as d3 from 'd3';
import { useChartDimensions } from '../../hooks/useChartDimensions';
import { DEFAULT_COLORS } from '../../constants/chartTypes';

function BarChart({ data, config }) {
  const { containerRef, svgRef, dimensions } = useChartDimensions();

  useEffect(() => {
    if (!data || data.length === 0 || !config.bar_label_column || !config.bar_value_column || dimensions.width === 0) {
      return;
    }

    const margin = { top: 20, right: 30, bottom: 60, left: 60 };
    const width = dimensions.width - margin.left - margin.right;
    const height = dimensions.height - margin.top - margin.bottom;

    d3.select(svgRef.current).selectAll('*').remove();

    const svg = d3.select(svgRef.current)
      .attr('width', width + margin.left + margin.right)
      .attr('height', height + margin.top + margin.bottom)
      .append('g')
      .attr('transform', `translate(${margin.left}, ${margin.top})`);

    const colors = d3.scaleOrdinal(DEFAULT_COLORS);
    const isHorizontal = config.bar_horizontal || false;

    if (isHorizontal) {
      const yScale = d3.scaleBand()
        .domain(data.map(d => String(d[config.bar_label_column])))
        .range([0, height])
        .padding(0.2);

      const xScale = d3.scaleLinear()
        .domain([0, d3.max(data, d => Number(d[config.bar_value_column]) || 0)])
        .range([0, width]);

      svg.append('g').call(d3.axisLeft(yScale)).selectAll('text').attr('fill', '#fff');
      svg.append('g').attr('transform', `translate(0, ${height})`).call(d3.axisBottom(xScale)).selectAll('text').attr('fill', '#fff');

      svg.selectAll('.bar').data(data).enter().append('rect')
        .attr('class', 'bar')
        .attr('y', d => yScale(String(d[config.bar_label_column])))
        .attr('x', 0)
        .attr('height', yScale.bandwidth())
        .attr('width', d => xScale(Number(d[config.bar_value_column]) || 0))
        .attr('fill', (d, i) => colors(i));

      svg.append('text').attr('text-anchor', 'end').attr('x', width).attr('y', height + 35)
        .attr('fill', '#fff').attr('font-size', '12px').text(config?.bar_value_column);
    } else {
      const xScale = d3.scaleBand()
        .domain(data.map(d => String(d[config.bar_label_column])))
        .range([0, width])
        .padding(0.2);

      const yScale = d3.scaleLinear()
        .domain([0, d3.max(data, d => Number(d[config.bar_value_column]) || 0)])
        .range([height, 0]);

      svg.append('g').attr('transform', `translate(0, ${height})`).call(d3.axisBottom(xScale))
        .selectAll('text').attr('fill', '#fff').attr('transform', 'rotate(-45)').style('text-anchor', 'end');
      svg.append('g').call(d3.axisLeft(yScale)).selectAll('text').attr('fill', '#fff');

      svg.selectAll('.bar').data(data).enter().append('rect')
        .attr('class', 'bar')
        .attr('x', d => xScale(String(d[config.bar_label_column])))
        .attr('y', d => yScale(Number(d[config.bar_value_column]) || 0))
        .attr('width', xScale.bandwidth())
        .attr('height', d => height - yScale(Number(d[config.bar_value_column]) || 0))
        .attr('fill', (d, i) => colors(i));

      svg.append('text').attr('text-anchor', 'end').attr('x', 100).attr('y', 50)
        .attr('transform', 'rotate(90)').attr('fill', '#fff').attr('font-size', '12px').text(config.bar_value_column);
    }
  }, [data, config, dimensions]);

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="bar-chart" style={{ width: '100%', height: '100%', display: 'block' }} />
    </div>
  );
}

export default BarChart;
