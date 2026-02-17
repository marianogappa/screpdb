import React, { useState } from 'react';
import EditWidgetFullscreen from './EditWidgetFullscreen';
import { renderChart } from '../utils/chartRenderer';

function Widget({ widget, onDelete, onUpdate, showDragHandle }) {
  const [showEditModal, setShowEditModal] = useState(false);

  const handleUpdate = (data) => {
    onUpdate(widget.id, data);
    setShowEditModal(false);
  };

  return (
    <div className="widget">
      <div className="widget-header">
        {showDragHandle && (
          <div className="widget-drag-handle" title="Drag to reorder">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor">
              <circle cx="4" cy="3" r="1.5"/><circle cx="10" cy="3" r="1.5"/>
              <circle cx="4" cy="7" r="1.5"/><circle cx="10" cy="7" r="1.5"/>
              <circle cx="4" cy="11" r="1.5"/><circle cx="10" cy="11" r="1.5"/>
            </svg>
          </div>
        )}
        <h3 className="widget-title">{widget.name}</h3>
        <div className="widget-actions">
          <button onClick={() => setShowEditModal(true)} className="btn-widget-action" title="Edit widget">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
          </button>
          <button onClick={() => onDelete(widget.id)} className="btn-widget-action btn-widget-delete" title="Delete widget">
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
          </button>
        </div>
      </div>

      {widget.description?.valid && widget.description.string && (
        <div className="widget-description">{widget.description.string}</div>
      )}

      <div className="widget-content">
        {renderChart({
          data: widget.results || [],
          config: widget.config,
          columns: widget.columns,
        })}
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
