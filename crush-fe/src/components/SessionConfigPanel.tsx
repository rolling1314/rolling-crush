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

export function SessionConfigPanel({ sessionId }: SessionConfigPanelProps) {
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
      const response = await fetch(`http://localhost:8081/api/sessions/${sessionId}/config`, {
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
    return (
      <div className="bg-gray-800 rounded-lg border border-gray-700">
        <div className="flex items-center gap-2 p-3">
          <Settings className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-medium text-white">Session Configuration</h3>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gray-800 rounded-lg border border-gray-700">
      {/* Header - 可点击折叠/展开 */}
      <div 
        className="flex items-center justify-between p-3 cursor-pointer hover:bg-gray-700/50 transition-colors"
        onClick={() => setIsExpanded(!isExpanded)}
      >
        <div className="flex items-center gap-2">
          <Settings className="w-4 h-4 text-blue-400" />
          <h3 className="text-sm font-medium text-white">Session Configuration</h3>
          {isExpanded ? (
            <ChevronDown className="w-4 h-4 text-gray-400" />
          ) : (
            <ChevronRight className="w-4 h-4 text-gray-400" />
          )}
        </div>
        <button
          onClick={(e) => {
            e.stopPropagation();
            setIsEditing(true);
          }}
          className="p-1.5 hover:bg-gray-600 rounded transition-colors"
          title="Edit configuration"
        >
          <Edit2 className="w-3.5 h-3.5 text-gray-400 hover:text-white" />
        </button>
      </div>

      {/* 展开时显示的内容 */}
      {isExpanded && (
        <div className="px-3 pb-3 space-y-2 border-t border-gray-700 pt-3">
          <div className="flex items-start gap-2">
            <div className="p-1.5 bg-gray-700 rounded">
              <Cpu className="w-3.5 h-3.5 text-blue-400" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-xs text-gray-400 mb-0.5">Provider & Model</div>
              <div className="text-sm text-white font-medium truncate">
                {config.provider} / {config.model}
              </div>
            </div>
          </div>

          <div className="flex items-start gap-2">
            <div className="p-1.5 bg-gray-700 rounded">
              <Key className="w-3.5 h-3.5 text-green-400" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-xs text-gray-400 mb-0.5">API Key</div>
              <div className="text-sm text-white font-mono truncate">
                {config.api_key || '****'}
              </div>
            </div>
          </div>
        </div>
      )}

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
      const response = await fetch(`http://localhost:8081/api/sessions/${sessionId}/config`, {
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

