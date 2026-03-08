import React, { useState } from 'react';
import { api } from '../api';
import ReplayFilterEditor, { BUILTIN_FILTERS } from './ReplayFilterEditor';
import Modal from './ui/Modal';
import FormField from './ui/FormField';
import Button from './ui/Button';
import { useToast } from './Toast';

function DashboardManager({ dashboards, currentUrl, onClose, onRefresh, onSwitch }) {
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [editingUrl, setEditingUrl] = useState(null);
  const [formData, setFormData] = useState({ name: '', url: '', description: '', replaysFilterSQL: '' });
  const [error, setError] = useState('');
  const { addToast } = useToast();

  const handleCreate = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.createDashboard({
        name: formData.name,
        url: formData.url || formData.name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
        description: formData.description,
        replays_filter_sql: formData.replaysFilterSQL,
      });
      setFormData({ name: '', url: '', description: '', replaysFilterSQL: '' });
      setShowCreateForm(false);
      addToast('Dashboard created', 'success');
      onRefresh();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUpdate = async (dashboard) => {
    setError('');
    try {
      await api.updateDashboard(dashboard.url, {
        name: formData.name,
        description: formData.description,
        replays_filter_sql: formData.replaysFilterSQL,
      });
      setEditingUrl(null);
      addToast('Dashboard updated', 'success');
      onRefresh();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleDelete = async (url) => {
    if (!confirm(`Delete dashboard "${url}"? All widgets will be lost.`)) return;
    try {
      await api.deleteDashboard(url);
      addToast('Dashboard deleted', 'success');
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
      description: dashboard.description || '',
      replaysFilterSQL: dashboard.replays_filter_sql || '',
    });
  };

  return (
    <Modal title="Manage Dashboards" onClose={onClose} className="dashboard-manager">
      {error && <div className="error-message" style={{ margin: '0 24px' }}>{error}</div>}

      <div className="dashboard-list">
        {dashboards.map((d) => (
          <div key={d.url} className="dashboard-item">
            {editingUrl === d.url ? (
              <div className="edit-form-inline">
                <input className="form-input" value={formData.name} onChange={(e) => setFormData(prev => ({ ...prev, name: e.target.value }))} placeholder="Name" />
                <input className="form-input" value={formData.description} onChange={(e) => setFormData(prev => ({ ...prev, description: e.target.value }))} placeholder="Description" />
                <Button variant="primary" onClick={() => handleUpdate(d)}>Save</Button>
                <Button variant="secondary" onClick={() => setEditingUrl(null)}>Cancel</Button>
              </div>
            ) : (
              <>
                <div className="dashboard-item-info">
                  <div className="dashboard-item-name">{d.name}</div>
                  <div className="dashboard-item-url">/{d.url}</div>
                  {d.description && <div className="dashboard-item-desc">{d.description}</div>}
                </div>
                <div className="dashboard-item-actions">
                  {d.url === currentUrl && <span className="current-badge">Current</span>}
                  <button className="btn-edit" onClick={() => startEdit(d)}>Edit</button>
                  {d.url !== currentUrl && (
                    <button className="btn-switch" onClick={() => { onSwitch(d.url); onClose(); }}>Switch</button>
                  )}
                  <button className="btn-delete" onClick={() => handleDelete(d.url)}>Delete</button>
                </div>
              </>
            )}
          </div>
        ))}
      </div>

      {showCreateForm ? (
        <form className="create-form" onSubmit={handleCreate}>
          <h3>New Dashboard</h3>
          <FormField label="Name" required value={formData.name} onChange={(v) => setFormData(prev => ({ ...prev, name: v }))} placeholder="My Dashboard" />
          <FormField label="URL slug" value={formData.url} onChange={(v) => setFormData(prev => ({ ...prev, url: v }))} placeholder="auto-generated from name" />
          <FormField label="Description" value={formData.description} onChange={(v) => setFormData(prev => ({ ...prev, description: v }))} placeholder="Optional description" />
          <ReplayFilterEditor value={formData.replaysFilterSQL} onChange={(v) => setFormData(prev => ({ ...prev, replaysFilterSQL: v }))} />
          <div className="form-actions">
            <Button variant="secondary" type="button" onClick={() => setShowCreateForm(false)}>Cancel</Button>
            <Button variant="primary" type="submit">Create</Button>
          </div>
        </form>
      ) : (
        <button className="btn-create-dashboard" onClick={() => setShowCreateForm(true)}>
          + New Dashboard
        </button>
      )}
    </Modal>
  );
}

export default DashboardManager;
