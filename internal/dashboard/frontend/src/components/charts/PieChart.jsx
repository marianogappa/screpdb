import React, { useEffect } from 'react';
import * as d3 from 'd3';
import { useChartDimensions } from '../../hooks/useChartDimensions';
import { DEFAULT_COLORS } from '../../constants/chartTypes';

function PieChart({ data, config }) {
  const { containerRef, svgRef, dimensions } = useChartDimensions({ deps: [data, config] });

  useEffect(() => {
    if (!data || data.length === 0 || !config.pie_label_column || !config.pie_value_column) return;
    if (dimensions.width === 0 || dimensions.height === 0) return;

    const size = Math.max(Math.min(dimensions.width, dimensions.height, 400), 200);
    const legendWidth = 180;
    const availableWidth = size - legendWidth - 40;
    const availableHeight = size - 40;
    const pieSize = Math.min(availableWidth, availableHeight);
    const radius = pieSize / 2 - 10;
    const pieCenterX = (size - legendWidth) / 2;
    const pieCenterY = size / 2;

    d3.select(svgRef.current).selectAll('*').remove();

    const svg = d3.select(svgRef.current).attr('width', size).attr('height', size);
    const pieGroup = svg.append('g').attr('transform', `translate(${pieCenterX}, ${pieCenterY})`);
    const colors = d3.scaleOrdinal(DEFAULT_COLORS);

    const pie = d3.pie().value(d => Number(d[config.pie_value_column]) || 0).sort(null);
    const arc = d3.arc().innerRadius(0).outerRadius(radius);
    const arcs = pie(data);

    pieGroup.selectAll('path').data(arcs).enter().append('path')
      .attr('fill', (d, i) => colors(i)).attr('stroke', '#1a1a1a').attr('stroke-width', 2).attr('d', arc)
      .style('cursor', 'pointer')
      .on('mouseover', function () { d3.select(this).attr('opacity', 0.9).attr('stroke-width', 3).attr('stroke', '#fff'); })
      .on('mouseout', function () { d3.select(this).attr('opacity', 1).attr('stroke-width', 2).attr('stroke', '#1a1a1a'); });

    const total = d3.sum(data, x => Number(x[config.pie_value_column]) || 0);

    const centerText = pieGroup.append('g').attr('text-anchor', 'middle').attr('pointer-events', 'none');
    centerText.append('text').attr('y', -8).attr('fill', '#fff').attr('font-size', '24px').attr('font-weight', '600').text('Total');
    centerText.append('text').attr('y', 20).attr('fill', 'rgba(255, 255, 255, 0.9)').attr('font-size', '20px').attr('font-weight', '400')
      .text(total.toLocaleString('en-US', { maximumFractionDigits: 0 }));

    const legendHeight = data.length * 24 + 20;
    const legendX = size - legendWidth + 10;
    const legendY = (size - legendHeight) / 2;
    const legend = svg.append('g').attr('transform', `translate(${legendX}, ${legendY})`);

    const legendItems = legend.selectAll('.legend-item').data(arcs).enter().append('g')
      .attr('class', 'legend-item').attr('transform', (d, i) => `translate(0, ${i * 24})`);

    legendItems.append('rect').attr('width', 16).attr('height', 16).attr('fill', (d, i) => colors(i))
      .attr('rx', 3).attr('stroke', 'rgba(255, 255, 255, 0.2)').attr('stroke-width', 1);

    const legendText = legendItems.append('text').attr('x', 22).attr('y', 12).attr('fill', '#fff')
      .attr('font-size', '14px').attr('font-weight', '500').attr('dominant-baseline', 'middle');
    legendText.append('tspan').text(d => d.data[config.pie_label_column]).attr('font-weight', '600');
    legendText.append('tspan')
      .text(d => ` ${((d.data[config.pie_value_column] || 0) / total * 100).toFixed(1)}%`)
      .attr('fill', 'rgba(255, 255, 255, 0.75)').attr('font-weight', '400').attr('dx', '4');
  }, [data, config, dimensions]);

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  if (!config || !config.pie_label_column || !config.pie_value_column) {
    const missingFields = [];
    if (!config) { missingFields.push('config'); }
    else {
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

  const displaySize = Math.min(dimensions.width, dimensions.height, 400);

  return (
    <div ref={containerRef} style={{ width: '100%', height: '100%', minHeight: '300px', display: 'flex', alignItems: 'center', justifyContent: 'center', overflow: 'hidden', position: 'relative', padding: '20px' }}>
      {dimensions.width > 0 && dimensions.height > 0 ? (
        <svg ref={svgRef} className="pie-chart" style={{ width: `${displaySize}px`, height: `${displaySize}px`, maxWidth: '100%', maxHeight: '100%', display: 'block' }} />
      ) : (
        <div className="chart-empty">Calculating dimensions...</div>
      )}
    </div>
  );
}

export default PieChart;
