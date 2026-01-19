import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';

const DEFAULT_COLORS = ['#4e79a7', '#f28e2c', '#e15759', '#76b7b2', '#59a14f', '#edc949', '#af7aa1', '#ff9d9a', '#9c755f', '#bab0ac'];

function BarChart({ data, config }) {
  const svgRef = useRef(null);
  const containerRef = useRef(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        const { width, height } = containerRef.current.getBoundingClientRect();
        // Use actual container size, but ensure minimums for chart rendering
        const newWidth = Math.max(300, width);
        const newHeight = Math.max(300, height);
        // Only update if dimensions changed significantly (avoid feedback loops)
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

      svg.append('g')
        .call(d3.axisLeft(yScale))
        .selectAll('text')
        .attr('fill', '#fff');

      svg.append('g')
        .attr('transform', `translate(0, ${height})`)
        .call(d3.axisBottom(xScale))
        .selectAll('text')
        .attr('fill', '#fff');

      svg.selectAll('.bar')
        .data(data)
        .enter()
        .append('rect')
        .attr('class', 'bar')
        .attr('y', d => yScale(String(d[config.bar_label_column])))
        .attr('x', 0)
        .attr('height', yScale.bandwidth())
        .attr('width', d => xScale(Number(d[config.bar_value_column]) || 0))
        .attr('fill', (d, i) => colors(i));
    } else {
      const xScale = d3.scaleBand()
        .domain(data.map(d => String(d[config.bar_label_column])))
        .range([0, width])
        .padding(0.2);

      const yScale = d3.scaleLinear()
        .domain([0, d3.max(data, d => Number(d[config.bar_value_column]) || 0)])
        .range([height, 0]);

      svg.append('g')
        .attr('transform', `translate(0, ${height})`)
        .call(d3.axisBottom(xScale))
        .selectAll('text')
        .attr('fill', '#fff')
        .attr('transform', 'rotate(-45)')
        .style('text-anchor', 'end');

      svg.append('g')
        .call(d3.axisLeft(yScale))
        .selectAll('text')
        .attr('fill', '#fff');

      svg.selectAll('.bar')
        .data(data)
        .enter()
        .append('rect')
        .attr('class', 'bar')
        .attr('x', d => xScale(String(d[config.bar_label_column])))
        .attr('y', d => yScale(Number(d[config.bar_value_column]) || 0))
        .attr('width', xScale.bandwidth())
        .attr('height', d => height - yScale(Number(d[config.bar_value_column]) || 0))
        .attr('fill', (d, i) => colors(i));
    }

  }, [data, config, dimensions]);

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="bar-chart" style={{ width: '100%', height: '100%', display: 'block' }} />
    </div>
  );
}

export default BarChart;

