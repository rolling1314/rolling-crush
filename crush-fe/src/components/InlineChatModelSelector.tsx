import { useState, useEffect, useRef } from 'react';
import { ChevronDown, Sparkles, Key, Settings, X } from 'lucide-react';
import axios from 'axios';

const API_URL = '/api';

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
  api_key?: string;
  base_url?: string;
  is_auto?: boolean;
}

interface InlineChatModelSelectorProps {
  selectedConfig: ModelConfig;
  onConfigChange: (config: ModelConfig) => void;
  disabled?: boolean;
}

export function InlineChatModelSelector({ 
  selectedConfig, 
  onConfigChange, 
  disabled = false 
}: InlineChatModelSelectorProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [showApiKeyInput, setShowApiKeyInput] = useState(false);
  const [tempApiKey, setTempApiKey] = useState('');
  const [autoModelAvailable, setAutoModelAvailable] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadProviders();
    checkAutoModel();
  }, []);

  useEffect(() => {
    if (selectedConfig.provider && !selectedConfig.is_auto) {
      loadModels(selectedConfig.provider);
    }
  }, [selectedConfig.provider]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
        setShowApiKeyInput(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const checkAutoModel = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/auto-model`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      if (response.data.configured) {
        setAutoModelAvailable(true);
      }
    } catch {
      setAutoModelAvailable(false);
    }
  };

  const loadProviders = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/providers`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setProviders(response.data || []);
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
    } catch (error) {
      console.error('Failed to load models:', error);
    }
  };

  const handleSelectAuto = () => {
    onConfigChange({
      provider: 'auto',
      model: 'auto',
      is_auto: true
    });
    setIsOpen(false);
  };

  const handleSelectProvider = (providerId: string) => {
    const provider = providers.find(p => p.id === providerId);
    onConfigChange({
      provider: providerId,
      model: '',
      is_auto: false
    });
    loadModels(providerId);
  };

  const handleSelectModel = (modelId: string) => {
    onConfigChange({
      ...selectedConfig,
      model: modelId,
      is_auto: false
    });
    // Check if this provider needs API key
    if (!selectedConfig.api_key) {
      setShowApiKeyInput(true);
    } else {
      setIsOpen(false);
    }
  };

  const handleSaveApiKey = () => {
    if (tempApiKey.trim()) {
      onConfigChange({
        ...selectedConfig,
        api_key: tempApiKey
      });
      setTempApiKey('');
      setShowApiKeyInput(false);
      setIsOpen(false);
    }
  };

  const getDisplayText = () => {
    if (selectedConfig.is_auto) {
      return 'Auto';
    }
    if (selectedConfig.model) {
      const model = models.find(m => m.id === selectedConfig.model);
      return model?.name || selectedConfig.model;
    }
    if (selectedConfig.provider) {
      const provider = providers.find(p => p.id === selectedConfig.provider);
      return provider?.name || 'Select model...';
    }
    return 'Auto';
  };

  const needsApiKey = !selectedConfig.is_auto && selectedConfig.provider && selectedConfig.model && !selectedConfig.api_key;

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
        className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs transition-colors ${
          selectedConfig.is_auto 
            ? 'bg-gradient-to-r from-purple-500/20 to-blue-500/20 text-purple-300 border border-purple-500/30' 
            : needsApiKey
              ? 'bg-red-500/10 text-red-400 border border-red-500/30'
              : 'bg-white/5 text-gray-300 border border-gray-700'
        } ${disabled ? 'opacity-50 cursor-not-allowed' : 'hover:bg-white/10 cursor-pointer'}`}
      >
        {selectedConfig.is_auto ? (
          <Sparkles className="w-3 h-3" />
        ) : needsApiKey ? (
          <Key className="w-3 h-3" />
        ) : (
          <Settings className="w-3 h-3" />
        )}
        <span className="max-w-[120px] truncate">{getDisplayText()}</span>
        <ChevronDown className={`w-3 h-3 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute bottom-full left-0 mb-1 w-64 bg-[#1a1a1a] border border-gray-700 rounded-lg shadow-xl z-50 max-h-80 overflow-hidden">
          {showApiKeyInput ? (
            <div className="p-3">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs font-medium text-gray-300">API Key Required</span>
                <button 
                  onClick={() => setShowApiKeyInput(false)}
                  className="text-gray-500 hover:text-white"
                >
                  <X size={14} />
                </button>
              </div>
              <input
                type="password"
                value={tempApiKey}
                onChange={(e) => setTempApiKey(e.target.value)}
                placeholder="Enter API key..."
                className="w-full px-2 py-1.5 bg-black border border-gray-700 rounded text-xs text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
                autoFocus
              />
              <button
                onClick={handleSaveApiKey}
                disabled={!tempApiKey.trim()}
                className="w-full mt-2 px-2 py-1.5 bg-blue-600 text-white text-xs rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Save & Continue
              </button>
            </div>
          ) : (
            <div className="overflow-y-auto max-h-72">
              {/* Auto option */}
              {autoModelAvailable && (
                <div className="p-1 border-b border-gray-700">
                  <button
                    onClick={handleSelectAuto}
                    className={`w-full flex items-center gap-2 px-3 py-2 rounded text-left text-sm transition-colors ${
                      selectedConfig.is_auto 
                        ? 'bg-purple-500/20 text-purple-300' 
                        : 'text-gray-300 hover:bg-white/5'
                    }`}
                  >
                    <Sparkles className="w-4 h-4 text-purple-400" />
                    <div>
                      <div className="font-medium">Auto</div>
                      <div className="text-[10px] text-gray-500">智谱 GLM-4.5 (推荐)</div>
                    </div>
                  </button>
                </div>
              )}

              {/* Provider/Model selection */}
              <div className="p-1">
                <div className="px-2 py-1 text-[10px] text-gray-500 uppercase tracking-wider">
                  选择模型
                </div>
                {providers.map(provider => (
                  <div key={provider.id}>
                    <button
                      onClick={() => handleSelectProvider(provider.id)}
                      className={`w-full flex items-center gap-2 px-3 py-2 rounded text-left text-sm transition-colors ${
                        selectedConfig.provider === provider.id && !selectedConfig.is_auto
                          ? 'bg-blue-500/20 text-blue-300'
                          : 'text-gray-300 hover:bg-white/5'
                      }`}
                    >
                      <div className="w-4 h-4 rounded bg-gray-700 flex items-center justify-center text-[10px] font-bold">
                        {provider.name.charAt(0)}
                      </div>
                      <span>{provider.name}</span>
                    </button>
                    
                    {/* Show models for selected provider */}
                    {selectedConfig.provider === provider.id && !selectedConfig.is_auto && (
                      <div className="ml-6 border-l border-gray-700 pl-2">
                        {models.map(model => (
                          <button
                            key={model.id}
                            onClick={() => handleSelectModel(model.id)}
                            className={`w-full px-3 py-1.5 rounded text-left text-xs transition-colors ${
                              selectedConfig.model === model.id
                                ? 'bg-green-500/20 text-green-300'
                                : 'text-gray-400 hover:bg-white/5 hover:text-gray-200'
                            }`}
                          >
                            {model.name}
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
