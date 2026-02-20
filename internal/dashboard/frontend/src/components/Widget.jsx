import React from 'react';
import { renderChart } from '../utils/chartRenderer';
import Icon from './ui/Icon';

function Widget({ widget, onDelete, onEdit, showDragHandle }) {
  return (
    <div className="widget">
      <div className="widget-header">
        {showDragHandle && (
          <div className="widget-drag-handle">
            <Icon name="drag" size={12} />
          </div>
        )}
        <h3 className="widget-title">{widget.name}</h3>
        <div className="widget-actions">
          <button className="btn-widget-action" onClick={() => onEdit(widget)} title="Edit">
            <Icon name="edit" size={14} />
          </button>
          <button className="btn-widget-action btn-widget-delete" onClick={() => onDelete(widget.id)} title="Delete">
            <Icon name="trash" size={14} />
          </button>
        </div>
      </div>
      {widget.description && <div className="widget-description">{widget.description}</div>}
      <div className="widget-content">
        {renderChart({
          data: widget.data,
          config: widget.config,
          columns: widget.columns,
          emptyMessage: 'No data to display',
        })}
      </div>
    </div>
  );
}

export default Widget;
