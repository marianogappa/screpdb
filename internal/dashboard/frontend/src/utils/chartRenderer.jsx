import React from 'react';
import Gauge from '../components/charts/Gauge';
import Table from '../components/charts/Table';
import PieChart from '../components/charts/PieChart';
import BarChart from '../components/charts/BarChart';
import LineChart from '../components/charts/LineChart';
import ScatterPlot from '../components/charts/ScatterPlot';
import Histogram from '../components/charts/Histogram';
import Heatmap from '../components/charts/Heatmap';

const CHART_COMPONENTS = {
  gauge: Gauge,
  table: Table,
  pie_chart: PieChart,
  bar_chart: BarChart,
  line_chart: LineChart,
  scatter_plot: ScatterPlot,
  histogram: Histogram,
  heatmap: Heatmap,
};

export function renderChart({ data, config, columns, emptyMessage }) {
  if (!config || !config.type) {
    return <div className="chart-empty">{emptyMessage || 'Select a widget type'}</div>;
  }

  let parsedConfig = config;
  if (typeof parsedConfig === 'string') {
    try {
      parsedConfig = JSON.parse(parsedConfig);
    } catch {
      return <div className="chart-empty">Error parsing widget configuration</div>;
    }
  }

  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  const Component = CHART_COMPONENTS[parsedConfig.type];
  if (!Component) {
    return <div className="chart-empty">Unknown widget type: {parsedConfig.type}</div>;
  }

  const props = { data, config: parsedConfig };
  if (parsedConfig.type === 'table') {
    props.columns = columns;
  }

  return <Component {...props} />;
}
