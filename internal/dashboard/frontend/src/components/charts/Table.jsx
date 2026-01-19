import React from 'react';

function Table({ data, config, columns }) {
  if (!data || data.length === 0) {
    return <div className="chart-empty">No data available</div>;
  }

  // Use provided columns to preserve SELECT query order
  const tableColumns = columns && columns.length > 0 ? columns : Object.keys(data[0]);

  return (
    <div className="table-container">
      <table className="data-table">
        <thead>
          <tr>
            {tableColumns.map((col) => (
              <th key={col}>{col}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row, idx) => (
            <tr key={idx}>
              {tableColumns.map((col) => (
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

