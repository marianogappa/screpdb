import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';

const DEFAULT_COLORS = ['#4e79a7', '#f28e2c', '#e15759', '#76b7b2', '#59a14f', '#edc949', '#af7aa1', '#ff9d9a', '#9c755f', '#bab0ac'];

function ScatterPlot({ data, config }) {
  const svgRef = useRef(null);
  const containerRef = useRef(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

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
    if (!data || data.length === 0 || !config.scatter_x_column || !config.scatter_y_column || dimensions.width === 0) {
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

    const xScale = d3.scaleLinear()
      .domain(d3.extent(data, d => Number(d[config.scatter_x_column]) || 0))
      .range([0, width]);

    const yScale = d3.scaleLinear()
      .domain(d3.extent(data, d => Number(d[config.scatter_y_column]) || 0))
      .range([height, 0]);

    const sizeScale = config.scatter_size_column
      ? d3.scaleSqrt()
        .domain(d3.extent(data, d => Number(d[config.scatter_size_column]) || 1))
        .range([3, 15])
      : () => 5;

    const colorScale = config.scatter_color_column
      ? d3.scaleOrdinal(DEFAULT_COLORS)
        .domain([...new Set(data.map(d => String(d[config.scatter_color_column])))])
      : () => DEFAULT_COLORS[0];

    svg.selectAll('.dot')
      .data(data)
      .enter()
      .append('circle')
      .attr('class', 'dot')
      .attr('cx', d => xScale(Number(d[config.scatter_x_column]) || 0))
      .attr('cy', d => yScale(Number(d[config.scatter_y_column]) || 0))
      .attr('r', d => sizeScale(config.scatter_size_column ? d[config.scatter_size_column] : 1))
      .attr('fill', d => config.scatter_color_column ? colorScale(String(d[config.scatter_color_column])) : DEFAULT_COLORS[0])
      .attr('opacity', 0.6)
      .on('mouseover', function () {
        d3.select(this).attr('opacity', 1).attr('r', d => sizeScale(config.scatter_size_column ? d[config.scatter_size_column] : 1) + 2);
      })
      .on('mouseout', function () {
        d3.select(this).attr('opacity', 0.6).attr('r', d => sizeScale(config.scatter_size_column ? d[config.scatter_size_column] : 1));
      });

    svg.append('g')
      .attr('transform', `translate(0, ${height})`)
      .call(d3.axisBottom(xScale))
      .selectAll('text')
      .attr('fill', '#fff');

    svg.append('g')
      .call(d3.axisLeft(yScale))
      .selectAll('text')
      .attr('fill', '#fff');

    svg.append("text")
      .attr("text-anchor", "end")
      .attr("x", width) // I have no idea how to set these!
      .attr("y", height + 35) // I have no idea how to set these!
      .attr('fill', '#fff')
      .attr('font-size', '12px')
      .text(config?.scatter_x_column);

    svg.append("text")
      .attr("text-anchor", "end")
      .attr("x", 100) // I have no idea how to set these!
      .attr("y", 50) // I have no idea how to set these!
      .attr("transform", "rotate(90)")
      .attr('fill', '#fff')
      .attr('font-size', '12px')
      .text(config?.scatter_y_column);

  }, [data, config, dimensions]);

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="scatter-plot" style={{ width: '100%', height: '100%', display: 'block' }} />
    </div>
  );
}

export default ScatterPlot;

