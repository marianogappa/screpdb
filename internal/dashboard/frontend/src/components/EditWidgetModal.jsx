import React, { useState, useEffect } from 'react';

function EditWidgetModal({ widget, onClose, onSave }) {
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [query, setQuery] = useState('');
  const [content, setContent] = useState('');

  useEffect(() => {
    if (widget) {
      setName(widget.name || '');
      setDescription(widget.description?.valid ? widget.description.string || '' : '');
      setQuery(widget.query || '');
      setContent(widget.content || '');
    }
  }, [widget]);

  const handleSubmit = (e) => {
    e.preventDefault();
    onSave({
      name,
      description: description || null,
      query,
      content,
    });
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Edit Widget</h2>
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

          <div className="form-group">
            <label>SQL Query</label>
            <textarea
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              rows="5"
              className="form-textarea"
            />
          </div>

          <div className="form-group">
            <label>HTML Content</label>
            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              rows="10"
              className="form-textarea"
            />
          </div>

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

export default EditWidgetModal;

