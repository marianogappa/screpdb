import React, { useEffect, useRef, useState } from 'react';
import * as d3 from 'd3';

const DEFAULT_COLORS = ['#4e79a7', '#f28e2c', '#e15759', '#76b7b2', '#59a14f', '#edc949', '#af7aa1', '#ff9d9a', '#9c755f', '#bab0ac'];

function PieChart({ data, config }) {
  const svgRef = useRef(null);
  const containerRef = useRef(null);
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const updateDimensions = () => {
      if (containerRef.current) {
        const { width, height } = containerRef.current.getBoundingClientRect();
        // Use minimum to keep it circular, but ensure we have some size
        const size = Math.max(Math.min(width, height, 400), 200);
        setDimensions(prev => {
          if (Math.abs(prev.width - size) > 1 || Math.abs(prev.height - size) > 1) {
            return { width: size, height: size };
          }
          return prev;
        });
      }
    };

    // Initial update
    updateDimensions();
    
    // Also try after a short delay in case container isn't ready yet
    const timeoutId = setTimeout(updateDimensions, 100);
    
    const resizeObserver = new ResizeObserver(updateDimensions);
    if (containerRef.current) {
      resizeObserver.observe(containerRef.current);
    }

    return () => {
      clearTimeout(timeoutId);
      resizeObserver.disconnect();
    };
  }, [data, config]); // Re-run when data or config changes

  useEffect(() => {
    if (!data || data.length === 0 || !config.pie_label_column || !config.pie_value_column) {
      return;
    }
    
    // Wait for dimensions to be set
    if (dimensions.width === 0 || dimensions.height === 0) {
      return;
    }

    const width = dimensions.width;
    const height = dimensions.height;

    // Calculate space needed for legend (estimate: 25px per item + padding)
    const legendWidth = 180;
    const legendHeight = data.length * 24 + 20;

    // Make pie chart smaller to accommodate legend
    const availableWidth = width - legendWidth - 40;
    const availableHeight = height - 40;
    const pieSize = Math.min(availableWidth, availableHeight);
    const radius = pieSize / 2 - 10;

    // Center pie chart accounting for legend space
    const pieCenterX = (width - legendWidth) / 2;
    const pieCenterY = height / 2;

    // Clear previous content
    d3.select(svgRef.current).selectAll('*').remove();

    const svg = d3.select(svgRef.current)
      .attr('width', width)
      .attr('height', height);

    // Pie chart group
    const pieGroup = svg.append('g')
      .attr('transform', `translate(${pieCenterX}, ${pieCenterY})`);

    const colors = config.colors && config.colors.length > 0
      ? d3.scaleOrdinal(config.colors)
      : d3.scaleOrdinal(DEFAULT_COLORS);

    const pie = d3.pie()
      .value(d => Number(d[config.pie_value_column]) || 0)
      .sort(null);

    const arc = d3.arc()
      .innerRadius(0)
      .outerRadius(radius);

    const arcs = pie(data);

    // Draw arcs with hover effects
    pieGroup.selectAll('path')
      .data(arcs)
      .enter()
      .append('path')
      .attr('fill', (d, i) => colors(i))
      .attr('stroke', '#1a1a1a')
      .attr('stroke-width', 2)
      .attr('d', arc)
      .style('cursor', 'pointer')
      .on('mouseover', function (event, d) {
        d3.select(this)
          .attr('opacity', 0.9)
          .attr('stroke-width', 3)
          .attr('stroke', '#fff');
      })
      .on('mouseout', function () {
        d3.select(this)
          .attr('opacity', 1)
          .attr('stroke-width', 2)
          .attr('stroke', '#1a1a1a');
      });

    // Calculate total for percentages
    const total = d3.sum(data, x => Number(x[config.pie_value_column]) || 0);

    // Add legend on the right side
    const legendX = width - legendWidth + 10;
    const legendY = (height - legendHeight) / 2;

    const legend = svg.append('g')
      .attr('transform', `translate(${legendX}, ${legendY})`);

    const legendItems = legend.selectAll('.legend-item')
      .data(arcs)
      .enter()
      .append('g')
      .attr('class', 'legend-item')
      .attr('transform', (d, i) => `translate(0, ${i * 24})`);

    legendItems.append('rect')
      .attr('width', 16)
      .attr('height', 16)
      .attr('fill', (d, i) => colors(i))
      .attr('rx', 3)
      .attr('stroke', 'rgba(255, 255, 255, 0.2)')
      .attr('stroke-width', 1);

    const legendText = legendItems.append('text')
      .attr('x', 22)
      .attr('y', 12)
      .attr('fill', '#fff')
      .attr('font-size', '14px')
      .attr('font-weight', '500')
      .attr('dominant-baseline', 'middle');

    legendText.append('tspan')
      .text(d => d.data[config.pie_label_column])
      .attr('font-weight', '600');

    legendText.append('tspan')
      .text(d => {
        const percent = ((d.data[config.pie_value_column] || 0) / total * 100).toFixed(1);
        return ` ${percent}%`;
      })
      .attr('fill', 'rgba(255, 255, 255, 0.75)')
      .attr('font-weight', '400')
      .attr('dx', '4');

  }, [data, config, dimensions]);

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  if (!config || !config.pie_label_column || !config.pie_value_column) {
    const missingFields = [];
    if (!config) {
      missingFields.push('config');
    } else {
      if (!config.pie_label_column) missingFields.push('pie_label_column');
      if (!config.pie_value_column) missingFields.push('pie_value_column');
    }
    return (
      <div className="chart-empty">
        Missing configuration: {missingFields.join(', ')} are required.
        {config && (
          <div style={{ fontSize: '0.8em', marginTop: '0.5em', opacity: 0.7 }}>
            Config keys: {Object.keys(config).join(', ')}
          </div>
        )}
      </div>
    );
  }

  return (
    <div 
      ref={containerRef} 
      style={{ 
        width: '100%', 
        height: '100%', 
        minHeight: '300px', 
        display: 'flex', 
        alignItems: 'center', 
        justifyContent: 'center', 
        overflow: 'hidden', 
        position: 'relative',
        padding: '20px'
      }}
    >
      {dimensions.width > 0 && dimensions.height > 0 ? (
        <svg 
          ref={svgRef} 
          className="pie-chart" 
          style={{ 
            width: `${dimensions.width}px`, 
            height: `${dimensions.height}px`, 
            maxWidth: '100%', 
            maxHeight: '100%', 
            display: 'block' 
          }} 
        />
      ) : (
        <div className="chart-empty">Calculating dimensions...</div>
      )}
    </div>
  );
}

export default PieChart;

