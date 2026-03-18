import React, { useState, useEffect, useRef } from 'react';
import { api } from '../api';
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

function Widget({ widget, onDelete, onUpdate, dashboardUrl, variableValues }) {
  const [showEditModal, setShowEditModal] = useState(false);
  const [results, setResults] = useState(widget.results || null);
  const [columns, setColumns] = useState(widget.columns || null);
  const [dataLoading, setDataLoading] = useState(!!widget.query && !widget.results);
  const [dataError, setDataError] = useState(null);
  const fetchId = useRef(0);

  useEffect(() => {
    if (!widget.query) return;

    const id = ++fetchId.current;
    setDataLoading(true);
    setDataError(null);

    api.executeQuery(widget.query, variableValues || {}, dashboardUrl)
      .then(data => {
        if (id !== fetchId.current) return;
        setResults(data.results);
        setColumns(data.columns);
      })
      .catch(err => {
        if (id !== fetchId.current) return;
        setDataError(err.message);
      })
      .finally(() => {
        if (id !== fetchId.current) return;
        setDataLoading(false);
      });
  }, [widget.query, variableValues, dashboardUrl]);

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

    let config = widget.config;
    if (typeof config === 'string') {
      try {
        config = JSON.parse(config);
      } catch (e) {
        return <div className="chart-empty">Error parsing widget configuration</div>;
      }
    }

    const chartProps = {
      data: results || [],
      config: config,
    };

    switch (widget.config.type) {
      case 'gauge':
        return <Gauge {...chartProps} />;
      case 'table':
        return <Table {...chartProps} columns={columns} />;
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
        {dataLoading ? (
          <div className="widget-loading">
            <div className="widget-spinner"></div>
          </div>
        ) : dataError ? (
          <div className="chart-empty">Query error: {dataError}</div>
        ) : (
          renderChart()
        )}
      </div>

      {showEditModal && (
        <EditWidgetFullscreen
          widget={{ ...widget, results, columns }}
          onClose={() => setShowEditModal(false)}
          onSave={handleUpdate}
        />
      )}
    </div>
  );
}

export default Widget;
