import React, { useState } from 'react';
import { api } from '../api';

function DashboardManager({ dashboards, currentUrl, onClose, onRefresh, onSwitch }) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingUrl, setEditingUrl] = useState(null);
  const [formData, setFormData] = useState({ name: '', url: '', description: '' });
  const [error, setError] = useState(null);

  const handleCreate = async (e) => {
    e.preventDefault();
    setError(null);
    try {
      await api.createDashboard({
        name: formData.name,
        url: formData.url,
        description: formData.description || null,
      });
      setShowCreateForm(false);
      setFormData({ name: '', url: '', description: '' });
      onRefresh();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUpdate = async (url, data) => {
    setError(null);
    try {
      await api.updateDashboard(url, data);
      setEditingUrl(null);
      onRefresh();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDelete = async (url) => {
    if (url === 'default') {
      alert('Cannot delete the default dashboard');
      return;
    }
    if (!confirm(`Are you sure you want to delete "${url}"?`)) return;

    setError(null);
    try {
      await api.deleteDashboard(url);
      if (currentUrl === url) {
        onSwitch('default');
      }
      onRefresh();
    } catch (err) {
      setError(err.message);
    }
  };

  const startEdit = (dashboard) => {
    setEditingUrl(dashboard.url);
    setFormData({
      name: dashboard.name,
      url: dashboard.url,
      description: dashboard.description?.valid ? dashboard.description.string : '',
    });
  };

  const cancelEdit = () => {
    setEditingUrl(null);
    setFormData({ name: '', url: '', description: '' });
  };

  const saveEdit = async (e) => {
    e.preventDefault();
    await handleUpdate(editingUrl, {
      name: formData.name,
      description: formData.description || null,
    });
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content dashboard-manager" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Manage Dashboards</h2>
          <button onClick={onClose} className="btn-close">×</button>
        </div>

        {error && <div className="error-message">{error}</div>}

        <div className="dashboard-list">
          {dashboards.map((dashboard) => (
            <div key={dashboard.url} className="dashboard-item">
              {editingUrl === dashboard.url ? (
                <form onSubmit={saveEdit} className="edit-form-inline">
                  <input
                    type="text"
                    value={formData.name}
                    onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                    required
                    className="form-input"
                  />
                  <input
                    type="text"
                    value={formData.description}
                    onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                    placeholder="Description (optional)"
                    className="form-input"
                  />
                  <button type="submit" className="btn-save">Save</button>
                  <button type="button" onClick={cancelEdit} className="btn-cancel">Cancel</button>
                </form>
              ) : (
                <>
                  <div className="dashboard-item-info">
                    <div className="dashboard-item-name">{dashboard.name}</div>
                    <div className="dashboard-item-url">/{dashboard.url}</div>
                    {dashboard.description?.valid && dashboard.description.string && (
                      <div className="dashboard-item-desc">{dashboard.description.string}</div>
                    )}
                  </div>
                  <div className="dashboard-item-actions">
                    {dashboard.url === currentUrl && (
                      <span className="current-badge">Current</span>
                    )}
                    <button onClick={() => onSwitch(dashboard.url)} className="btn-switch">
                      Switch
                    </button>
                    <button onClick={() => startEdit(dashboard)} className="btn-edit">
                      Edit
                    </button>
                    {dashboard.url !== 'default' && (
                      <button
                        onClick={() => handleDelete(dashboard.url)}
                        className="btn-delete"
                      >
                        Delete
                      </button>
                    )}
                  </div>
                </>
              )}
            </div>
          ))}
        </div>

        {showCreateForm ? (
          <form onSubmit={handleCreate} className="create-form">
            <h3>Create New Dashboard</h3>
            <div className="form-group">
              <label>Name</label>
              <input
                type="text"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                required
                className="form-input"
              />
            </div>
            <div className="form-group">
              <label>URL</label>
              <input
                type="text"
                value={formData.url}
                onChange={(e) => setFormData({ ...formData, url: e.target.value })}
                required
                className="form-input"
                placeholder="e.g., my-dashboard"
              />
            </div>
            <div className="form-group">
              <label>Description</label>
              <input
                type="text"
                value={formData.description}
                onChange={(e) => setFormData({ ...formData, description: e.target.value })}
                className="form-input"
              />
            </div>
            <div className="form-actions">
              <button type="submit" className="btn-save">Create</button>
              <button
                type="button"
                onClick={() => {
                  setShowCreateForm(false);
                  setFormData({ name: '', url: '', description: '' });
                }}
                className="btn-cancel"
              >
                Cancel
              </button>
            </div>
          </form>
        ) : (
          <button
            onClick={() => setShowCreateForm(true)}
            className="btn-create-dashboard"
          >
            + Create New Dashboard
          </button>
        )}
      </div>
    </div>
  );
}

export default DashboardManager;

