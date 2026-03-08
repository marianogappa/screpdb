import React from 'react';
import Gauge from '../components/charts/Gauge';
import Table from '../components/charts/Table';
import PieChart from '../components/charts/PieChart';
import BarChart from '../components/charts/BarChart';
import LineChart from '../components/charts/LineChart';
import ScatterPlot from '../components/charts/ScatterPlot';
import Histogram from '../components/charts/Histogram';
import Heatmap from '../components/charts/Heatmap';

class ChartErrorBoundary extends React.Component {
  state = { hasError: false, error: null };

  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }

  componentDidCatch(error, errorInfo) {
    console.error('Chart render error:', error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="chart-empty chart-error">
          Chart error: {this.state.error?.message || 'Something went wrong'}
        </div>
      );
    }
    return this.props.children;
  }
}

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

const Empty = ({ children }) => <div className="chart-empty">{children}</div>;

export function renderChart({ data, config, columns, emptyMessage }) {
  if (!config || !config.type) {
    return <Empty>{emptyMessage || 'Select a widget type'}</Empty>;
  }

  let parsedConfig = config;
  if (typeof parsedConfig === 'string') {
    try {
      parsedConfig = JSON.parse(parsedConfig);
    } catch {
      return <Empty>Error parsing widget configuration</Empty>;
    }
  }

  if (!data || data.length === 0) {
    return <Empty>No data available</Empty>;
  }

  const Component = CHART_COMPONENTS[parsedConfig.type];
  if (!Component) {
    return <Empty>Unknown widget type: {parsedConfig.type}</Empty>;
  }

  const props = { data, config: parsedConfig };
  if (parsedConfig.type === 'table') {
    props.columns = columns;
  }

  return (
    <ChartErrorBoundary>
      <Component {...props} />
    </ChartErrorBoundary>
  );
}
