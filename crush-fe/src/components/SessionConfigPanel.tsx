import { useState, useEffect } from 'react';
import { Settings, Edit2, Key, Cpu, ChevronDown, ChevronRight, X } from 'lucide-react';
import { ModelSelector } from './ModelSelector';

interface SessionConfig {
  provider: string;
  model: string;
  api_key: string;
  max_tokens?: number;
  reasoning_effort?: string;
}

interface SessionConfigPanelProps {
  sessionId: string;
  compact?: boolean;
}

interface SessionModelConfig {
  provider: string;
  model: string;
  base_url?: string;
  api_key?: string;
  max_tokens?: number;
  temperature?: number;
  top_p?: number;
  reasoning_effort?: string;
  think?: boolean;
}

export function SessionConfigPanel({ sessionId, compact = false }: SessionConfigPanelProps) {
  const [config, setConfig] = useState<SessionConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadConfig();
  }, [sessionId]);

  const loadConfig = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`/api/sessions/${sessionId}/config`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to load session config');
      }

      const data = await response.json();
      setConfig(data);
    } catch (err) {
      console.error('Error loading session config:', err);
      setError('Failed to load configuration');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    if (compact) {
      return <div className="text-xs text-gray-500">Loading config...</div>;
    }
    return (
      <div className="bg-gray-800 rounded-lg border border-gray-700">
        <div className="flex items-center gap-2 p-3">
          <Settings className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-medium text-white">Session Configuration</h3>
        </div>
      </div>
    );
  }

  if (error) {
    if (compact) {
        return <div className="text-xs text-red-500">Config Error</div>;
    }
    return (
      <div className="bg-gray-800 rounded-lg border border-gray-700">
        <div className="flex items-center gap-2 p-3">
          <Settings className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-medium text-white">Session Configuration</h3>
        </div>
      </div>
    );
  }

  if (!config || !config.provider) {
    if (compact) {
        return (
            <div className="flex items-center gap-2">
                 <button 
                    onClick={() => setIsEditing(true)}
                    className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-red-500/10 hover:bg-red-500/20 text-red-400 text-xs transition-colors"
                 >
                    <Settings className="w-3.5 h-3.5" />
                    <span>Configure</span>
                 </button>
                 {isEditing && (
                    <EditConfigModal
                      sessionId={sessionId}
                      currentConfig={{ provider: '', model: '', api_key: '' }} 
                      onClose={() => setIsEditing(false)}
                      onSave={() => {
                        setIsEditing(false);
                        loadConfig();
                      }}
                    />
                 )}
            </div>
        );
    }
    return (
      <div className="bg-gray-800 rounded-lg border border-gray-700">
        <div className="flex items-center gap-2 p-3">
          <Settings className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-medium text-white">Session Configuration</h3>
        </div>
      </div>
    );
  }

  if (compact) {
    return (
      <div className="flex items-center gap-2">
        <div className="flex items-center gap-1.5 px-2 py-1 rounded-md bg-white/5 text-xs text-gray-300 select-none">
           <Cpu className="w-3.5 h-3.5 text-blue-400" />
           <span>{config.model}</span>
        </div>
        <button
          onClick={() => setIsEditing(true)}
          className="p-1 text-gray-500 hover:text-white transition-colors rounded-md hover:bg-white/10"
          title="Session Settings"
        >
          <Settings className="w-4 h-4" />
        </button>

        {isEditing && (
          <EditConfigModal
            sessionId={sessionId}
            currentConfig={config}
            onClose={() => setIsEditing(false)}
            onSave={() => {
              setIsEditing(false);
              loadConfig();
            }}
          />
        )}
      </div>
    );
  }

  return (
    <div className="mt-2">
      {/* Configuration Bar */}
      <div 
        className="flex items-center justify-between px-4 py-3 bg-[#2d2d2d] rounded-lg cursor-pointer hover:bg-[#3d3d3d] transition-colors border border-gray-700"
        onClick={() => setIsEditing(true)}
      >
        <div className="flex items-center gap-3">
          <div className="p-1.5 bg-blue-500/10 rounded-md">
            <Settings className="w-4 h-4 text-blue-400" />
          </div>
          <div className="flex flex-col">
            <span className="text-sm font-medium text-gray-200">会话配置</span>
            <span className="text-xs text-gray-500">
              {config?.provider && config?.model ? `${config.provider} / ${config.model}` : 'Configure model'}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <ChevronRight className="w-4 h-4 text-gray-500" />
        </div>
      </div>

      {isEditing && (
        <EditConfigModal
          sessionId={sessionId}
          currentConfig={config!}
          onClose={() => setIsEditing(false)}
          onSave={() => {
            setIsEditing(false);
            loadConfig();
          }}
        />
      )}
    </div>
  );
}

interface EditConfigModalProps {
  sessionId: string;
  currentConfig: SessionConfig;
  onClose: () => void;
  onSave: () => void;
}

function EditConfigModal({ sessionId, currentConfig, onClose, onSave }: EditConfigModalProps) {
  const [modelConfig, setModelConfig] = useState<SessionModelConfig>({
    provider: currentConfig.provider,
    model: currentConfig.model,
    api_key: '',
    max_tokens: currentConfig.max_tokens || 4096,
    reasoning_effort: currentConfig.reasoning_effort || '',
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSave = async () => {
    try {
      setSaving(true);
      setError(null);

      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`/api/sessions/${sessionId}/config`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify(modelConfig),
      });

      if (!response.ok) {
        throw new Error('Failed to update configuration');
      }

      onSave();
    } catch (err) {
      console.error('Error updating config:', err);
      setError('Failed to update configuration');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-[#252526] p-6 rounded-lg w-[500px] max-h-[80vh] overflow-y-auto border border-gray-700">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-xl font-bold text-white">Edit Session Configuration</h3>
          <button
            onClick={onClose}
            className="p-1 hover:bg-gray-700 rounded transition-colors"
          >
            <X className="w-5 h-5 text-gray-400" />
          </button>
        </div>

        {error && (
          <div className="mb-4 p-3 bg-red-500 bg-opacity-10 border border-red-500 rounded-lg text-red-400 text-sm">
            {error}
          </div>
        )}

        <div className="mb-4 p-3 bg-blue-500 bg-opacity-10 border border-blue-500 rounded-lg text-blue-400 text-sm">
          <div className="flex items-center gap-2 mb-1">
            <Key className="w-4 h-4" />
            <span className="font-medium">Current API Key: {currentConfig.api_key}</span>
          </div>
          <div className="text-xs text-gray-400">Leave API key empty to keep current key, or enter a new one to update</div>
        </div>

        <ModelSelector 
          onConfigChange={(config) => setModelConfig(config)}
          initialConfig={modelConfig}
          showAdvanced={false}
        />

        <div className="flex gap-3 mt-6">
          <button
            onClick={onClose}
            disabled={saving}
            className="flex-1 px-4 py-2 bg-gray-600 hover:bg-gray-700 text-white rounded transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !modelConfig.provider || !modelConfig.model}
            className="flex-1 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {saving ? 'Saving...' : 'Save Configuration'}
          </button>
        </div>
      </div>
    </div>
  );
}

