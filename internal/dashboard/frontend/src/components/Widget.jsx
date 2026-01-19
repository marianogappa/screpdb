import React, { useState } from 'react';
import EditWidgetModal from './EditWidgetModal';
import EditWidgetFullscreen from './EditWidgetFullscreen';
import Gauge from './charts/Gauge';
import Table from './charts/Table';
import PieChart from './charts/PieChart';
import BarChart from './charts/BarChart';
import LineChart from './charts/LineChart';
import ScatterPlot from './charts/ScatterPlot';
import Histogram from './charts/Histogram';
import Heatmap from './charts/Heatmap';

function Widget({ widget, onDelete, onUpdate }) {
  const [showEditModal, setShowEditModal] = useState(false);

  const handleDelete = () => {
    onDelete(widget.id);
  };

  const handleUpdate = (data) => {
    onUpdate(widget.id, data);
    setShowEditModal(false);
  };

  const renderChart = () => {
    if (!widget.config || !widget.config.type) {
      return <div className="chart-empty">Invalid widget configuration</div>;
    }

    // Ensure config is an object (handle case where it might be a string)
    let config = widget.config;
    if (typeof config === 'string') {
      try {
        config = JSON.parse(config);
      } catch (e) {
        return <div className="chart-empty">Error parsing widget configuration</div>;
      }
    }

    const chartProps = {
      data: widget.results || [],
      config: config,
    };

    switch (widget.config.type) {
      case 'gauge':
        return <Gauge {...chartProps} />;
      case 'table':
        return <Table {...chartProps} columns={widget.columns} />;
      case 'pie_chart':
        return <PieChart {...chartProps} />;
      case 'bar_chart':
        return <BarChart {...chartProps} />;
      case 'line_chart':
        return <LineChart {...chartProps} />;
      case 'scatter_plot':
        return <ScatterPlot {...chartProps} />;
      case 'histogram':
        return <Histogram {...chartProps} />;
      case 'heatmap':
        return <Heatmap {...chartProps} />;
      default:
        return <div className="chart-empty">Unknown widget type: {widget.config.type}</div>;
    }
  };

  return (
    <div className="widget">
      <div className="widget-header">
        <h3 className="widget-title">{widget.name}</h3>
        <div className="widget-actions">
          <button
            onClick={() => setShowEditModal(true)}
            className="btn-edit"
            title="Edit widget"
          >
            ✎
          </button>
          <button
            onClick={handleDelete}
            className="btn-delete"
            title="Delete widget"
          >
            ×
          </button>
        </div>
      </div>

      {widget.description?.valid && widget.description.string && (
        <div className="widget-description">{widget.description.string}</div>
      )}

      <div className="widget-content">
        {renderChart()}
      </div>

      {showEditModal && (
        <EditWidgetFullscreen
          widget={widget}
          onClose={() => setShowEditModal(false)}
          onSave={handleUpdate}
        />
      )}
    </div>
  );
}

export default Widget;

