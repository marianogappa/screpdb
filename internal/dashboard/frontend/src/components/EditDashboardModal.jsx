import React, { useState, useEffect } from 'react';
import ReplayFilterEditor from './ReplayFilterEditor';

function EditDashboardModal({ dashboard, onClose, onSave }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [replaysFilterSQL, setReplaysFilterSQL] = useState('');

  useEffect(() => {
    if (dashboard) {
      setName(dashboard.name || '');
      setDescription(dashboard.description?.valid ? dashboard.description.string || '' : '');
      setReplaysFilterSQL(dashboard.replays_filter_sql || '');
    }
  }, [dashboard]);

  const handleSubmit = (e) => {
    e.preventDefault();
    onSave({
      name,
      description: description || null,
      replays_filter_sql: replaysFilterSQL,
    });
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Edit Dashboard</h2>
          <button onClick={onClose} className="btn-close">×</button>
        </div>
        
        <form onSubmit={handleSubmit} className="edit-form">
          <div className="form-group">
            <label>Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              className="form-input"
            />
          </div>

          <div className="form-group">
            <label>Description</label>
            <input
              type="text"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="form-input"
            />
          </div>

          <ReplayFilterEditor value={replaysFilterSQL} onChange={setReplaysFilterSQL} />

          <div className="form-actions">
            <button type="button" onClick={onClose} className="btn-cancel">
              Cancel
            </button>
            <button type="submit" className="btn-save">
              Save
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

export default EditDashboardModal;
