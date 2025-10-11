import React, { useState, useEffect } from 'react';
import './App.css';
import { Config } from './types';

const App: React.FC = () => {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState<boolean>(false);
  const [saveMessage, setSaveMessage] = useState<string>('');

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
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setLoading(false);
    }
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
        body: JSON.stringify(config),
      });
      
      if (!response.ok) {
        throw new Error('Failed to save configuration');
      }
      
      setSaveMessage('Configuration saved successfully!');
      setTimeout(() => setSaveMessage(''), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error');
    } finally {
      setSaving(false);
    }
  };

  const updateConfig = (field: keyof Config, value: string): void => {
    if (!config) return;
    setConfig({ ...config, [field]: value });
  };

  const updateProxy = (field: keyof Config['Proxy'], value: string | number): void => {
    if (!config) return;
    setConfig({
      ...config,
      Proxy: { ...config.Proxy, [field]: value }
    });
  };

  const updateNoProxy = (field: keyof Config['NoProxy'], value: string[]): void => {
    if (!config) return;
    setConfig({
      ...config,
      NoProxy: { ...config.NoProxy, [field]: value }
    });
  };

  const addContext = (): void => {
    if (!config) return;
    const newContext = {
      Name: 'new-context',
      Way: []
    };
    setConfig({
      ...config,
      Contexts: [...(config.Contexts || []), newContext]
    });
  };

  const updateContext = (index: number, field: keyof Config['Contexts'][0], value: string): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    newContexts[index] = { ...newContexts[index], [field]: value };
    setConfig({ ...config, Contexts: newContexts });
  };

  const deleteContext = (index: number): void => {
    if (!config) return;
    const newContexts = config.Contexts.filter((_, i) => i !== index);
    setConfig({ ...config, Contexts: newContexts });
  };

  const addWayNode = (contextIndex: number): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    const newNode = {
      probe: '',
      lb: []
    };
    newContexts[contextIndex].Way = [...(newContexts[contextIndex].Way || []), newNode];
    setConfig({ ...config, Contexts: newContexts });
  };

  const updateWayNode = (contextIndex: number, nodeIndex: number, field: string, value: string): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    newContexts[contextIndex].Way[nodeIndex] = {
      ...newContexts[contextIndex].Way[nodeIndex],
      [field]: value
    };
    setConfig({ ...config, Contexts: newContexts });
  };

  const deleteWayNode = (contextIndex: number, nodeIndex: number): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    newContexts[contextIndex].Way = newContexts[contextIndex].Way.filter((_, i) => i !== nodeIndex);
    setConfig({ ...config, Contexts: newContexts });
  };

  const addLbEntry = (contextIndex: number, nodeIndex: number): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    newContexts[contextIndex].Way[nodeIndex].lb = [
      ...(newContexts[contextIndex].Way[nodeIndex].lb || []),
      ''
    ];
    setConfig({ ...config, Contexts: newContexts });
  };

  const updateLbEntry = (contextIndex: number, nodeIndex: number, lbIndex: number, value: string): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    newContexts[contextIndex].Way[nodeIndex].lb[lbIndex] = value;
    setConfig({ ...config, Contexts: newContexts });
  };

  const deleteLbEntry = (contextIndex: number, nodeIndex: number, lbIndex: number): void => {
    if (!config) return;
    const newContexts = [...config.Contexts];
    newContexts[contextIndex].Way[nodeIndex].lb = 
      newContexts[contextIndex].Way[nodeIndex].lb.filter((_, i) => i !== lbIndex);
    setConfig({ ...config, Contexts: newContexts });
  };

  const updateListEntry = (listName: keyof Config['NoProxy'], index: number, value: string): void => {
    if (!config) return;
    const newList = [...(config.NoProxy[listName] || [])];
    newList[index] = value;
    updateNoProxy(listName, newList);
  };

  const addListEntry = (listName: keyof Config['NoProxy']): void => {
    if (!config) return;
    const newList = [...(config.NoProxy[listName] || []), ''];
    updateNoProxy(listName, newList);
  };

  const deleteListEntry = (listName: keyof Config['NoProxy'], index: number): void => {
    if (!config) return;
    const newList = (config.NoProxy[listName] || []).filter((_, i) => i !== index);
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

  if (!config) {
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
          <input
            type="text"
            value={config.CurrentContext || ''}
            onChange={(e) => updateConfig('CurrentContext', e.target.value)}
          />
        </div>
      </div>

      <div className="section">
        <h2>Proxy Settings</h2>
        <div className="form-group">
          <label>Host:</label>
          <input
            type="text"
            value={config.Proxy?.Host || ''}
            onChange={(e) => updateProxy('Host', e.target.value)}
          />
        </div>
        <div className="form-group">
          <label>Port:</label>
          <input
            type="number"
            value={config.Proxy?.Port || 0}
            onChange={(e) => updateProxy('Port', parseInt(e.target.value) || 0)}
          />
        </div>
      </div>

      <div className="section">
        <h2>No Proxy Settings</h2>
        
        <h3>List</h3>
        {(config.NoProxy?.List || []).map((item, index) => (
          <div key={index} className="list-item">
            <input
              type="text"
              value={item}
              onChange={(e) => updateListEntry('List', index, e.target.value)}
            />
            <button onClick={() => deleteListEntry('List', index)} className="btn-delete">
              Delete
            </button>
          </div>
        ))}
        <button onClick={() => addListEntry('List')} className="btn-add">
          Add List Entry
        </button>

        <h3>From Environment</h3>
        {(config.NoProxy?.FromEnv || []).map((item, index) => (
          <div key={index} className="list-item">
            <input
              type="text"
              value={item}
              onChange={(e) => updateListEntry('FromEnv', index, e.target.value)}
            />
            <button onClick={() => deleteListEntry('FromEnv', index)} className="btn-delete">
              Delete
            </button>
          </div>
        ))}
        <button onClick={() => addListEntry('FromEnv')} className="btn-add">
          Add FromEnv Entry
        </button>

        <h3>From File</h3>
        {(config.NoProxy?.FromFile || []).map((item, index) => (
          <div key={index} className="list-item">
            <input
              type="text"
              value={item}
              onChange={(e) => updateListEntry('FromFile', index, e.target.value)}
            />
            <button onClick={() => deleteListEntry('FromFile', index)} className="btn-delete">
              Delete
            </button>
          </div>
        ))}
        <button onClick={() => addListEntry('FromFile')} className="btn-add">
          Add FromFile Entry
        </button>
      </div>

      <div className="section">
        <h2>Contexts</h2>
        {(config.Contexts || []).map((context, contextIndex) => (
          <div key={contextIndex} className="context-item">
            <h3>Context {contextIndex + 1}</h3>
            <div className="form-group">
              <label>Name:</label>
              <input
                type="text"
                value={context.Name || ''}
                onChange={(e) => updateContext(contextIndex, 'Name', e.target.value)}
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
                  />
                </div>
                
                <h5>Load Balancer Entries</h5>
                {(node.lb || []).map((lb, lbIndex) => (
                  <div key={lbIndex} className="list-item">
                    <input
                      type="text"
                      value={lb}
                      onChange={(e) => updateLbEntry(contextIndex, nodeIndex, lbIndex, e.target.value)}
                    />
                    <button 
                      onClick={() => deleteLbEntry(contextIndex, nodeIndex, lbIndex)}
                      className="btn-delete"
                    >
                      Delete
                    </button>
                  </div>
                ))}
                <button 
                  onClick={() => addLbEntry(contextIndex, nodeIndex)}
                  className="btn-add-small"
                >
                  Add LB Entry
                </button>
                
                <button 
                  onClick={() => deleteWayNode(contextIndex, nodeIndex)}
                  className="btn-delete"
                >
                  Delete Way Node
                </button>
              </div>
            ))}
            <button onClick={() => addWayNode(contextIndex)} className="btn-add">
              Add Way Node
            </button>
            
            <button onClick={() => deleteContext(contextIndex)} className="btn-delete">
              Delete Context
            </button>
          </div>
        ))}
        <button onClick={addContext} className="btn-add">
          Add Context
        </button>
      </div>

      <div className="actions">
        <button onClick={saveConfig} disabled={saving} className="btn-save">
          {saving ? 'Saving...' : 'Save Configuration'}
        </button>
        <button onClick={fetchConfig} className="btn-refresh">
          Refresh
        </button>
      </div>
    </div>
  );
};

export default App;
