import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';

const DEFAULT_COLORS = ['#4e79a7'];

function Histogram({ data, config }) {
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
    if (!data || data.length === 0 || !config?.histogram_value_column || dimensions.width === 0 || !svgRef.current) {
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

    const values = data.map(d => Number(d[config.histogram_value_column]) || 0).filter(v => !isNaN(v));

    if (values.length === 0) {
      return;
    }

    const bins = config.histogram_bins || Math.ceil(Math.sqrt(values.length));

    // Use d3.bin() for d3 v4+ (replaces d3.histogram())
    const binsData = d3.bin()
      .domain(d3.extent(values))
      .thresholds(bins)(values);

    if (binsData.length === 0) {
      return;
    }

    const xScale = d3.scaleLinear()
      .domain([binsData[0].x0, binsData[binsData.length - 1].x1])
      .range([0, width]);

    const yScale = d3.scaleLinear()
      .domain([0, d3.max(binsData, d => d.length)])
      .range([height, 0]);

    const xAxis = d3.axisBottom(xScale)
      .tickSizeOuter(0);

    svg.append('g')
      .attr('transform', `translate(0, ${height})`)
      .call(xAxis)
      .selectAll('text')
      .attr('fill', '#fff');

    svg.append('g')
      .call(d3.axisLeft(yScale))
      .selectAll('text')
      .attr('fill', '#fff');

    svg.selectAll('.bar')
      .data(binsData)
      .enter()
      .append('rect')
      .attr('class', 'bar')
      .attr('x', d => xScale(d.x0))
      .attr('y', d => yScale(d.length))
      .attr('width', d => Math.max(0, xScale(d.x1) - xScale(d.x0) - 1))
      .attr('height', d => height - yScale(d.length))
      .attr('fill', DEFAULT_COLORS[0])
      .attr('opacity', 0.7);

    svg.append("text")
      .attr("text-anchor", "end")
      .attr("x", width) // I have no idea how to set these!
      .attr("y", height + 35) // I have no idea how to set these!
      .attr('fill', '#fff')
      .attr('font-size', '12px')
      .text(config?.histogram_value_column);

    svg.append("text")
      .attr("text-anchor", "end")
      .attr("x", 100) // I have no idea how to set these!
      .attr("y", 50) // I have no idea how to set these!
      .attr("transform", "rotate(90)")
      .attr('fill', '#fff')
      .attr('font-size', '12px')
      .text('count');

  }, [data, config, dimensions]);

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', overflow: 'hidden', position: 'relative' }}>
      <svg ref={svgRef} className="histogram" style={{ width: '100%', height: '100%', display: 'block' }} />
    </div>
  );
}

export default Histogram;

