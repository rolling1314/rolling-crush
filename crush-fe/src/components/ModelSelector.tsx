import { useState, useEffect } from 'react';
import axios from 'axios';

const API_URL = 'http://localhost:8081/api';

interface Provider {
  id: string;
  name: string;
  base_url: string;
  type: string;
}

interface Model {
  id: string;
  name: string;
  default_max_tokens: number;
}

interface ModelConfig {
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

interface ModelSelectorProps {
  onConfigChange: (config: ModelConfig) => void;
  initialConfig?: ModelConfig;
  showAdvanced?: boolean; // 是否显示高级设置（base_url等）
}

export const ModelSelector = ({ onConfigChange, initialConfig, showAdvanced = false }: ModelSelectorProps) => {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [selectedProvider, setSelectedProvider] = useState(initialConfig?.provider || '');
  const [selectedModel, setSelectedModel] = useState(initialConfig?.model || '');
  const [config, setConfig] = useState<ModelConfig>(initialConfig || {
    provider: '',
    model: '',
    max_tokens: 4096,
  });
  const [isValidating, setIsValidating] = useState(false);
  const [validationStatus, setValidationStatus] = useState<'idle' | 'validating' | 'success' | 'error'>('idle');
  const [validationMessage, setValidationMessage] = useState('');

  useEffect(() => {
    loadProviders();
  }, []);

  useEffect(() => {
    if (selectedProvider) {
      loadModels(selectedProvider);
    }
  }, [selectedProvider]);

  useEffect(() => {
    if (selectedProvider && selectedModel) {
      const newConfig = {
        ...config,
        provider: selectedProvider,
        model: selectedModel,
      };
      setConfig(newConfig);
      onConfigChange(newConfig);
    }
  }, [selectedProvider, selectedModel]);

  const loadProviders = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/providers`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setProviders(response.data || []);
      
      if (response.data && response.data.length > 0 && !selectedProvider) {
        setSelectedProvider(response.data[0].id);
      }
    } catch (error) {
      console.error('Failed to load providers:', error);
    }
  };

  const loadModels = async (providerId: string) => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/providers/${providerId}/models`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setModels(response.data || []);
      
      if (response.data && response.data.length > 0 && !selectedModel) {
        setSelectedModel(response.data[0].id);
      }
    } catch (error) {
      console.error('Failed to load models:', error);
    }
  };

  const handleConfigUpdate = (key: keyof ModelConfig, value: any) => {
    const newConfig = { ...config, [key]: value };
    setConfig(newConfig);
    onConfigChange(newConfig);
    
    // 如果修改了API Key，重置验证状态
    if (key === 'api_key') {
      setValidationStatus('idle');
      setValidationMessage('');
    }
  };

  const validateApiKey = async () => {
    if (!config.api_key || !config.api_key.trim()) {
      setValidationMessage('Please enter an API key');
      return;
    }

    if (!selectedProvider || !selectedModel) {
      setValidationMessage('Please select a provider and model');
      return;
    }

    setIsValidating(true);
    setValidationStatus('validating');
    setValidationMessage('Validating API key...');

    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.post(
        `${API_URL}/providers/test-connection`,
        {
          provider: selectedProvider,
          model: selectedModel,
          api_key: config.api_key,
          base_url: config.base_url || undefined,
        },
        {
          headers: { Authorization: `Bearer ${token}` }
        }
      );

      if (response.data.success) {
        setValidationStatus('success');
        setValidationMessage('✓ API key is valid');
      } else {
        setValidationStatus('error');
        setValidationMessage(response.data.error || 'Validation failed');
      }
    } catch (error: any) {
      setValidationStatus('error');
      const errorMessage = error.response?.data?.error || 'Failed to validate API key';
      setValidationMessage('✗ ' + errorMessage);
    } finally {
      setIsValidating(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Provider Selection */}
      <div>
        <label className="block text-sm font-medium text-gray-300 mb-2">
          Provider <span className="text-red-500">*</span>
        </label>
        <select
          value={selectedProvider}
          onChange={(e) => {
            setSelectedProvider(e.target.value);
            setValidationStatus('idle');
            setValidationMessage('');
          }}
          className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
        >
          <option value="">Select a provider</option>
          {providers.map(p => (
            <option key={p.id} value={p.id}>{p.name}</option>
          ))}
        </select>
      </div>

      {/* Model Selection */}
      {selectedProvider && (
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-2">
            Model <span className="text-red-500">*</span>
          </label>
          <select
            value={selectedModel}
            onChange={(e) => {
              setSelectedModel(e.target.value);
              setValidationStatus('idle');
              setValidationMessage('');
            }}
            className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
          >
            <option value="">Select a model</option>
            {models.map(m => (
              <option key={m.id} value={m.id}>{m.name}</option>
            ))}
          </select>
        </div>
      )}

      {/* API Key */}
      {selectedProvider && selectedModel && (
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-2">
            API Key <span className="text-red-500">*</span>
          </label>
          <div className="space-y-2">
            <input
              type="password"
              value={config.api_key || ''}
              onChange={(e) => handleConfigUpdate('api_key', e.target.value)}
              placeholder="Enter your API key..."
              className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
            />
            <div className="flex items-center gap-2">
              <button
                onClick={validateApiKey}
                disabled={isValidating || !config.api_key}
                className="px-3 py-1.5 bg-blue-600 text-white text-sm rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {isValidating ? 'Validating...' : 'Test Connection'}
              </button>
              {validationMessage && (
                <span className={`text-sm ${
                  validationStatus === 'success' ? 'text-green-400' : 
                  validationStatus === 'error' ? 'text-red-400' : 
                  'text-gray-400'
                }`}>
                  {validationMessage}
                </span>
              )}
            </div>
            <p className="text-xs text-gray-500">
              Required for most providers. Keep it secure.
            </p>
          </div>
        </div>
      )}

      {/* Base URL (仅在showAdvanced为true时显示) */}
      {showAdvanced && selectedProvider && selectedModel && (
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-2">
            Base URL (Optional)
          </label>
          <input
            type="text"
            value={config.base_url || ''}
            onChange={(e) => handleConfigUpdate('base_url', e.target.value)}
            placeholder="https://api.example.com/v1"
            className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
          />
          <p className="text-xs text-gray-500 mt-1">
            Override the default API endpoint if needed
          </p>
        </div>
      )}

      {/* Advanced Settings (仅在showAdvanced为true时显示) */}
      {showAdvanced && selectedProvider && selectedModel && (
        <details className="mt-4">
          <summary className="cursor-pointer text-sm text-gray-400 hover:text-white">
            Advanced Settings
          </summary>
          <div className="mt-3 space-y-3">
            <div>
              <label className="block text-sm text-gray-400 mb-1">Max Tokens</label>
              <input
                type="number"
                value={config.max_tokens || ''}
                onChange={(e) => handleConfigUpdate('max_tokens', parseInt(e.target.value))}
                className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500"
                placeholder="4096"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Temperature (0-2)</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="2"
                value={config.temperature || ''}
                onChange={(e) => handleConfigUpdate('temperature', parseFloat(e.target.value))}
                className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500"
                placeholder="0.7"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Top P (0-1)</label>
              <input
                type="number"
                step="0.1"
                min="0"
                max="1"
                value={config.top_p || ''}
                onChange={(e) => handleConfigUpdate('top_p', parseFloat(e.target.value))}
                className="w-full px-3 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white text-sm focus:outline-none focus:border-blue-500"
                placeholder="0.9"
              />
            </div>
          </div>
        </details>
      )}
    </div>
  );
};

