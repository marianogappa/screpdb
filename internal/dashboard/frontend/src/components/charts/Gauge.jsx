import React from 'react';

function Gauge({ data, config }) {
  if (!data || data.length === 0 || !config?.gauge_value_column) {
    return <div className="chart-empty">No data or missing value column</div>;
  }
  const firstRow = data[0];
  const value = firstRow[config.gauge_value_column];
  if (value === undefined || value === null) {
    return <div className="chart-empty">No value in selected column</div>;
  }
  const min = config.gauge_min ?? 0;
  const max = config.gauge_max ?? (typeof value === 'number' ? value * 1.2 : 100);
  const label = config.gauge_label || config.gauge_value_column;
  const numValue = typeof value === 'number' ? value : Number(value);
  const range = max - min;
  const percentage = range !== 0 && !isNaN(numValue) ? Math.min(100, Math.max(0, ((numValue - min) / range) * 100)) : 0;

  return (
    <div className="gauge-container">
      <div className="gauge-label">{label}</div>
      <div className="gauge-value">{typeof numValue === 'number' && !isNaN(numValue) ? numValue.toLocaleString() : String(value)}</div>
      <div className="gauge-bar">
        <div 
          className="gauge-fill" 
          style={{ width: `${percentage}%` }}
        />
      </div>
      {config.gauge_min !== undefined && config.gauge_max !== undefined && (
        <div className="gauge-range">
          <span>{min}</span>
          <span>{max}</span>
        </div>
      )}
    </div>
  );
}

export default Gauge;

