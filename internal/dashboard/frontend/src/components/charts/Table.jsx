import React from 'react';

function Table({ data, config }) {
  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  const columns = config.table_columns && config.table_columns.length > 0
    ? config.table_columns
    : Object.keys(data[0]);

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            {columns.map((col) => (
              <th key={col}>{col}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, idx) => (
            <tr key={idx}>
              {columns.map((col) => (
                <td key={col}>
                  {row[col] !== null && row[col] !== undefined
                    ? typeof row[col] === 'number'
                      ? row[col].toLocaleString()
                      : String(row[col])
                    : '-'}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default Table;

