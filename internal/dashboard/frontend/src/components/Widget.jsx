import React, { useState, useEffect, useRef } from 'react';
import EditWidgetModal from './EditWidgetModal';

function Widget({ widget, onDelete, onUpdate }) {
  const [showEditModal, setShowEditModal] = useState(false);
  const [refinePrompt, setRefinePrompt] = useState('');
  const iframeRef = useRef(null);

  useEffect(() => {
    if (!widget || !iframeRef.current) return;

    const iframe = iframeRef.current;
    const dataVar = `sqlRowsForWidget${widget.id}`;
    const widgetContent = widget.content || '';

    // Check if D3.js is already imported in the content
    const hasD3Import = /d3js\.org|d3\.v\d+\.min\.js|d3\.min\.js/i.test(widgetContent);

    // Build the HTML document for the iframe
    let htmlContent = `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">`;

    // Only add D3.js if it's not already in the content
    if (!hasD3Import) {
      htmlContent += `
  <script src="https://d3js.org/d3.v7.min.js"></script>`;
    }

    htmlContent += `
  <style>
    * {
      margin: 0;
      padding: 0;
      box-sizing: border-box;
    }
    body {
      width: 100%;
      height: 100%;
      overflow: hidden;
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
    }
  </style>
</head>
<body>
  <script>
    // Make data available as a global variable
    var ${dataVar} = ${JSON.stringify(widget.results || [])};
  </script>
  ${widgetContent}
</body>
</html>`;

    // Use srcdoc instead of document.write for better reliability
    iframe.srcdoc = htmlContent;
  }, [widget]);

  const handleRefine = (e) => {
    e.preventDefault();
    // TODO: Implement refine functionality when backend is ready
    console.log('Refine prompt:', refinePrompt);
    setRefinePrompt('');
  };

  const handleDelete = () => {
    onDelete(widget.id);
  };

  const handleUpdate = (data) => {
    onUpdate(widget.id, data);
    setShowEditModal(false);
  };

  return (
    <div className="widget">
      <div className="widget-header">
        <h3 className="widget-title">{widget.name}</h3>
        <div className="widget-actions">
          <button
            onClick={() => setShowEditModal(true)}
            className="btn-edit"
            title="Edit widget"
          >
            ✎
          </button>
          <button
            onClick={handleDelete}
            className="btn-delete"
            title="Delete widget"
          >
            ×
          </button>
        </div>
      </div>
      
      {widget.description?.valid && widget.description.string && (
        <div className="widget-description">{widget.description.string}</div>
      )}
      
      <iframe
        ref={iframeRef}
        className="widget-content-iframe"
        title={`Widget ${widget.id}`}
        sandbox="allow-scripts allow-same-origin"
      />
      
      <form onSubmit={handleRefine} className="widget-refine">
        <input
          type="text"
          value={refinePrompt}
          onChange={(e) => setRefinePrompt(e.target.value)}
          placeholder="Refine: e.g., 'Show only Zerg games'"
          className="refine-input"
        />
      </form>

      {showEditModal && (
        <EditWidgetModal
          widget={widget}
          onClose={() => setShowEditModal(false)}
          onSave={handleUpdate}
        />
      )}
    </div>
  );
}

export default Widget;

