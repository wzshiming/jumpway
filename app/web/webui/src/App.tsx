import React, { useState, useEffect } from 'react';
import './App.css';
import { Config } from './types';

const App: React.FC = () => {
  const [config, setConfig] = useState<Config | null>(null);
  const [editedConfig, setEditedConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState<boolean>(false);
  const [saveMessage, setSaveMessage] = useState<string>('');
  const [isEditing, setIsEditing] = useState<boolean>(false);

  useEffect(() => {
    fetchConfig();
  }, []);

  const fetchConfig = async (): Promise<void> => {
    try {
      setLoading(true);
      const response = await fetch('/apis/configs');
      if (!response.ok) {
        throw new Error('Failed to fetch configuration');
      }
      const data: Config = await response.json();
      setConfig(data);
      setEditedConfig(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
  };

  const handleEdit = (): void => {
    setIsEditing(true);
    setEditedConfig(config ? { ...config } : null);
  };

  const handleCancel = (): void => {
    setIsEditing(false);
    setEditedConfig(config);
    setSaveMessage('');
    setError(null);
  };

  const saveConfig = async (): Promise<void> => {
    try {
      setSaving(true);
      setSaveMessage('');
      const response = await fetch('/apis/configs', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(editedConfig),
      });
      
      if (!response.ok) {
        throw new Error('Failed to save configuration');
      }
      
      setConfig(editedConfig);
      setIsEditing(false);
      setSaveMessage('Configuration saved successfully!');
      setTimeout(() => setSaveMessage(''), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setSaving(false);
    }
  };

  const updateConfig = (field: keyof Config, value: string): void => {
    if (!editedConfig) return;
    setEditedConfig({ ...editedConfig, [field]: value });
  };

  const updateProxy = (field: keyof Config['Proxy'], value: string | number): void => {
    if (!editedConfig) return;
    setEditedConfig({
      ...editedConfig,
      Proxy: { ...editedConfig.Proxy, [field]: value }
    });
  };

  const updateNoProxy = (field: keyof Config['NoProxy'], value: string[]): void => {
    if (!editedConfig) return;
    setEditedConfig({
      ...editedConfig,
      NoProxy: { ...editedConfig.NoProxy, [field]: value }
    });
  };

  const addContext = (): void => {
    if (!editedConfig) return;
    const newContext = {
      Name: 'new-context',
      Way: []
    };
    setEditedConfig({
      ...editedConfig,
      Contexts: [...(editedConfig.Contexts || []), newContext]
    });
  };

  const updateContext = (index: number, field: keyof Config['Contexts'][0], value: string): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    newContexts[index] = { ...newContexts[index], [field]: value };
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const deleteContext = (index: number): void => {
    if (!editedConfig) return;
    const newContexts = editedConfig.Contexts.filter((_, i) => i !== index);
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const addWayNode = (contextIndex: number): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    const newNode = {
      probe: '',
      lb: []
    };
    newContexts[contextIndex].Way = [...(newContexts[contextIndex].Way || []), newNode];
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const updateWayNode = (contextIndex: number, nodeIndex: number, field: string, value: string): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    newContexts[contextIndex].Way[nodeIndex] = {
      ...newContexts[contextIndex].Way[nodeIndex],
      [field]: value
    };
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const deleteWayNode = (contextIndex: number, nodeIndex: number): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    newContexts[contextIndex].Way = newContexts[contextIndex].Way.filter((_, i) => i !== nodeIndex);
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const addLbEntry = (contextIndex: number, nodeIndex: number): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    newContexts[contextIndex].Way[nodeIndex].lb = [
      ...(newContexts[contextIndex].Way[nodeIndex].lb || []),
      ''
    ];
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const updateLbEntry = (contextIndex: number, nodeIndex: number, lbIndex: number, value: string): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    newContexts[contextIndex].Way[nodeIndex].lb[lbIndex] = value;
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const deleteLbEntry = (contextIndex: number, nodeIndex: number, lbIndex: number): void => {
    if (!editedConfig) return;
    const newContexts = [...editedConfig.Contexts];
    newContexts[contextIndex].Way[nodeIndex].lb = 
      newContexts[contextIndex].Way[nodeIndex].lb.filter((_, i) => i !== lbIndex);
    setEditedConfig({ ...editedConfig, Contexts: newContexts });
  };

  const updateListEntry = (listName: keyof Config['NoProxy'], index: number, value: string): void => {
    if (!editedConfig) return;
    const newList = [...(editedConfig.NoProxy[listName] || [])];
    newList[index] = value;
    updateNoProxy(listName, newList);
  };

  const addListEntry = (listName: keyof Config['NoProxy']): void => {
    if (!editedConfig) return;
    const newList = [...(editedConfig.NoProxy[listName] || []), ''];
    updateNoProxy(listName, newList);
  };

  const deleteListEntry = (listName: keyof Config['NoProxy'], index: number): void => {
    if (!editedConfig) return;
    const newList = (editedConfig.NoProxy[listName] || []).filter((_, i) => i !== index);
    updateNoProxy(listName, newList);
  };

  if (loading) {
    return (
      <div className="container">
        <div className="loading">Loading configuration...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container">
        <div className="error">Error: {error}</div>
        <button onClick={fetchConfig}>Retry</button>
      </div>
    );
  }

  if (!config || !editedConfig) {
    return (
      <div className="container">
        <div className="error">No configuration available</div>
      </div>
    );
  }

  return (
    <div className="container">
      <h1>Jump Way Configuration</h1>
      
      {saveMessage && <div className="success-message">{saveMessage}</div>}
      
      <div className="section">
        <h2>Current Context</h2>
        <div className="form-group">
          <label>Current Context Name:</label>
          <select
            value={editedConfig.CurrentContext || ''}
            onChange={(e) => updateConfig('CurrentContext', e.target.value)}
            disabled={!isEditing}
          >
            <option value="">-- Select a context --</option>
            {(editedConfig.Contexts || []).map((context, index) => (
              <option key={index} value={context.Name}>
                {context.Name}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="section">
        <h2>Proxy Settings</h2>
        <div className="form-group">
          <label>Host:</label>
          <input
            type="text"
            value={editedConfig.Proxy?.Host || ''}
            onChange={(e) => updateProxy('Host', e.target.value)}
            disabled={!isEditing}
          />
        </div>
        <div className="form-group">
          <label>Port:</label>
          <input
            type="number"
            value={editedConfig.Proxy?.Port || 0}
            onChange={(e) => updateProxy('Port', parseInt(e.target.value) || 0)}
            disabled={!isEditing}
          />
        </div>
      </div>

      <div className="section">
        <h2>No Proxy Settings</h2>
        
        <h3>List</h3>
        {(editedConfig.NoProxy?.List || []).map((item, index) => (
          <div key={index} className="list-item">
            <input
              type="text"
              value={item}
              onChange={(e) => updateListEntry('List', index, e.target.value)}
              disabled={!isEditing}
            />
            {isEditing && (
              <button onClick={() => deleteListEntry('List', index)} className="btn-delete">
                Delete
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={() => addListEntry('List')} className="btn-add">
          Add List Entry
        </button>
        )}

        <h3>From Environment</h3>
        {(editedConfig.NoProxy?.FromEnv || []).map((item, index) => (
          <div key={index} className="list-item">
            <input
              type="text"
              value={item}
              onChange={(e) => updateListEntry('FromEnv', index, e.target.value)}
              disabled={!isEditing}
            />
            {isEditing && (
              <button onClick={() => deleteListEntry('FromEnv', index)} className="btn-delete">
                Delete
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={() => addListEntry('FromEnv')} className="btn-add">
            Add FromEnv Entry
          </button>
        )}

        <h3>From File</h3>
        {(editedConfig.NoProxy?.FromFile || []).map((item, index) => (
          <div key={index} className="list-item">
            <input
              type="text"
              value={item}
              onChange={(e) => updateListEntry('FromFile', index, e.target.value)}
              disabled={!isEditing}
            />
            {isEditing && (
              <button onClick={() => deleteListEntry('FromFile', index)} className="btn-delete">
                Delete
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={() => addListEntry('FromFile')} className="btn-add">
            Add FromFile Entry
          </button>
        )}
      </div>

      <div className="section">
        <h2>Contexts</h2>
        {(editedConfig.Contexts || []).map((context, contextIndex) => (
          <div key={contextIndex} className="context-item">
            <h3>Context {contextIndex + 1}</h3>
            <div className="form-group">
              <label>Name:</label>
              <input
                type="text"
                value={context.Name || ''}
                onChange={(e) => updateContext(contextIndex, 'Name', e.target.value)}
                disabled={!isEditing}
              />
            </div>
            
            <h4>Way Nodes</h4>
            {(context.Way || []).map((node, nodeIndex) => (
              <div key={nodeIndex} className="way-node">
                <div className="form-group">
                  <label>Probe:</label>
                  <input
                    type="text"
                    value={node.probe || ''}
                    onChange={(e) => updateWayNode(contextIndex, nodeIndex, 'probe', e.target.value)}
                    disabled={!isEditing}
                  />
                </div>
                
                <h5>Load Balancer Entries</h5>
                {(node.lb || []).map((lb, lbIndex) => (
                  <div key={lbIndex} className="list-item">
                    <input
                      type="text"
                      value={lb}
                      onChange={(e) => updateLbEntry(contextIndex, nodeIndex, lbIndex, e.target.value)}
                      disabled={!isEditing}
                    />
                    {isEditing && (
                      <button 
                        onClick={() => deleteLbEntry(contextIndex, nodeIndex, lbIndex)}
                        className="btn-delete"
                      >
                        Delete
                      </button>
                    )}
                  </div>
                ))}
                {isEditing && (
                  <button 
                    onClick={() => addLbEntry(contextIndex, nodeIndex)}
                    className="btn-add-small"
                  >
                    Add LB Entry
                  </button>
                )}
                
                {isEditing && (
                  <button 
                    onClick={() => deleteWayNode(contextIndex, nodeIndex)}
                    className="btn-delete"
                  >
                    Delete Way Node
                  </button>
                )}
              </div>
            ))}
            {isEditing && (
              <button onClick={() => addWayNode(contextIndex)} className="btn-add">
                Add Way Node
              </button>
            )}
            
            {isEditing && (
              <button onClick={() => deleteContext(contextIndex)} className="btn-delete">
                Delete Context
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={addContext} className="btn-add">
            Add Context
          </button>
        )}
      </div>

      <div className="actions">
        {!isEditing ? (
          <button onClick={handleEdit} className="btn-edit">
            Edit Configuration
          </button>
        ) : (
          <>
            <button onClick={saveConfig} disabled={saving} className="btn-save">
              {saving ? 'Saving...' : 'Save'}
            </button>
            <button onClick={handleCancel} disabled={saving} className="btn-cancel">
              Cancel
            </button>
          </>
        )}
      </div>
    </div>
  );
};

export default App;
