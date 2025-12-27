import React from 'react';

function Gauge({ data, config }) {
  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  const value = data[0][config.gauge_value_column];
  const min = config.gauge_min ?? 0;
  const max = config.gauge_max ?? (value * 1.2);
  const label = config.gauge_label || config.gauge_value_column;
  const percentage = ((value - min) / (max - min)) * 100;

  return (
    <div className="gauge-container">
      <div className="gauge-label">{label}</div>
      <div className="gauge-value">{typeof value === 'number' ? value.toLocaleString() : value}</div>
      <div className="gauge-bar">
        <div 
          className="gauge-fill" 
          style={{ width: `${Math.min(100, Math.max(0, percentage))}%` }}
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

