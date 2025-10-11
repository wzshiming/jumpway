import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import './App.css';
import { Config } from './types';

const App: React.FC = () => {
  const { t } = useTranslation();
  const [config, setConfig] = useState<Config | null>(null);
  const [editedConfig, setEditedConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState<boolean>(false);
  const [saveMessage, setSaveMessage] = useState<string>('');
  const [isEditing, setIsEditing] = useState<boolean>(false);

  const fetchConfig = useCallback(async (): Promise<void> => {
    try {
      setLoading(true);
      const response = await fetch('/apis/configs');
      if (!response.ok) {
        throw new Error(t('failedToFetch'));
      }
      const data: Config = await response.json();
      setConfig(data);
      setEditedConfig(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('unknownError'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);

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
        throw new Error(t('failedToSave'));
      }
      
      setConfig(editedConfig);
      setIsEditing(false);
      setSaveMessage(t('savedSuccessfully'));
      setTimeout(() => setSaveMessage(''), 3000);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('unknownError'));
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
        <div className="loading">{t('loading')}</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="container">
        <div className="error">{t('error', { message: error })}</div>
        <button onClick={fetchConfig}>{t('retry')}</button>
      </div>
    );
  }

  if (!config || !editedConfig) {
    return (
      <div className="container">
        <div className="error">{t('noConfiguration')}</div>
      </div>
    );
  }

  return (
    <div className="container">
      <h1>{t('pageTitle')}</h1>
      
      {saveMessage && <div className="success-message">{saveMessage}</div>}
      
      <div className="section">
        <h2>{t('currentContext')}</h2>
        <div className="form-group">
          <label>{t('currentContextName')}:</label>
          <select
            value={editedConfig.CurrentContext || ''}
            onChange={(e) => updateConfig('CurrentContext', e.target.value)}
            disabled={!isEditing}
          >
            <option value="">{t('selectContext')}</option>
            {(editedConfig.Contexts || []).map((context, index) => (
              <option key={index} value={context.Name}>
                {context.Name}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="section">
        <h2>{t('proxySettings')}</h2>
        <div className="form-group">
          <label>{t('host')}:</label>
          <input
            type="text"
            value={editedConfig.Proxy?.Host || ''}
            onChange={(e) => updateProxy('Host', e.target.value)}
            disabled={!isEditing}
          />
        </div>
        <div className="form-group">
          <label>{t('port')}:</label>
          <input
            type="number"
            value={editedConfig.Proxy?.Port || 0}
            onChange={(e) => updateProxy('Port', parseInt(e.target.value) || 0)}
            disabled={!isEditing}
          />
        </div>
      </div>

      <div className="section">
        <h2>{t('noProxySettings')}</h2>
        
        <h3>{t('list')}</h3>
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
                {t('delete')}
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={() => addListEntry('List')} className="btn-add">
            {t('addListEntry')}
          </button>
        )}

        <h3>{t('fromEnvironment')}</h3>
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
                {t('delete')}
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={() => addListEntry('FromEnv')} className="btn-add">
            {t('addFromEnvEntry')}
          </button>
        )}

        <h3>{t('fromFile')}</h3>
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
                {t('delete')}
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={() => addListEntry('FromFile')} className="btn-add">
            {t('addFromFileEntry')}
          </button>
        )}
      </div>

      <div className="section">
        <h2>{t('contexts')}</h2>
        {(editedConfig.Contexts || []).map((context, contextIndex) => (
          <div key={contextIndex} className="context-item">
            <h3>{t('context', { number: contextIndex + 1 })}</h3>
            <div className="form-group">
              <label>{t('name')}:</label>
              <input
                type="text"
                value={context.Name || ''}
                onChange={(e) => updateContext(contextIndex, 'Name', e.target.value)}
                disabled={!isEditing}
              />
            </div>
            
            <h4>{t('wayNodes')}</h4>
            {(context.Way || []).map((node, nodeIndex) => (
              <div key={nodeIndex} className="way-node">
                <div className="form-group">
                  <label>{t('probe')}:</label>
                  <input
                    type="text"
                    value={node.probe || ''}
                    onChange={(e) => updateWayNode(contextIndex, nodeIndex, 'probe', e.target.value)}
                    disabled={!isEditing}
                  />
                </div>
                
                <h5>{t('loadBalancerEntries')}</h5>
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
                        {t('delete')}
                      </button>
                    )}
                  </div>
                ))}
                {isEditing && (
                  <button 
                    onClick={() => addLbEntry(contextIndex, nodeIndex)}
                    className="btn-add-small"
                  >
                    {t('addLbEntry')}
                  </button>
                )}
                
                {isEditing && (
                  <button 
                    onClick={() => deleteWayNode(contextIndex, nodeIndex)}
                    className="btn-delete"
                  >
                    {t('deleteWayNode')}
                  </button>
                )}
              </div>
            ))}
            {isEditing && (
              <button onClick={() => addWayNode(contextIndex)} className="btn-add">
                {t('addWayNode')}
              </button>
            )}
            
            {isEditing && (
              <button onClick={() => deleteContext(contextIndex)} className="btn-delete">
                {t('deleteContext')}
              </button>
            )}
          </div>
        ))}
        {isEditing && (
          <button onClick={addContext} className="btn-add">
            {t('addContext')}
          </button>
        )}
      </div>

      <div className="actions">
        {!isEditing ? (
          <button onClick={handleEdit} className="btn-edit">
            {t('edit')}
          </button>
        ) : (
          <>
            <button onClick={saveConfig} disabled={saving} className="btn-save">
              {saving ? t('saving') : t('save')}
            </button>
            <button onClick={handleCancel} disabled={saving} className="btn-cancel">
              {t('cancel')}
            </button>
          </>
        )}
      </div>
    </div>
  );
};

export default App;
