import React, { useState } from 'react';
import Modal from './ui/Modal';
import FormField from './ui/FormField';
import Button from './ui/Button';
import ReplayFilterEditor from './ReplayFilterEditor';

function EditDashboardModal({ dashboard, onClose, onSave }) {
  const [name, setName] = useState(dashboard.name || '');
  const [description, setDescription] = useState(dashboard.description || '');
  const [replaysFilterSQL, setReplaysFilterSQL] = useState(dashboard.replays_filter_sql || '');

  const handleSubmit = (e) => {
    e.preventDefault();
    onSave({ name, description, replays_filter_sql: replaysFilterSQL });
  };

  return (
    <Modal title="Edit Dashboard" onClose={onClose}>
      <form className="edit-form" onSubmit={handleSubmit}>
        <FormField label="Name" required value={name} onChange={setName} placeholder="Dashboard name" />
        <FormField label="Description" value={description} onChange={setDescription} placeholder="Optional description" />
        <ReplayFilterEditor value={replaysFilterSQL} onChange={setReplaysFilterSQL} />
        <div className="form-actions">
          <Button variant="secondary" type="button" onClick={onClose}>Cancel</Button>
          <Button variant="primary" type="submit">Save</Button>
        </div>
      </form>
    </Modal>
  );
}

export default EditDashboardModal;
