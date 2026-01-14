import { useState, useEffect, useRef } from 'react';
import { ChevronDown, Sparkles, Key, Settings, X, Loader2, Check, AlertCircle } from 'lucide-react';
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

interface SessionConfig {
  provider: string;
  model: string;
  api_key?: string;
  max_tokens?: number;
  reasoning_effort?: string;
}

interface InlineChatModelSelectorProps {
  selectedConfig: ModelConfig;
  onConfigChange: (config: ModelConfig) => void;
  disabled?: boolean;
  sessionId?: string; // 如果传入则为老会话模式
  onConfigSaved?: () => void; // 配置保存成功回调
}

export function InlineChatModelSelector({ 
  selectedConfig, 
  onConfigChange, 
  disabled = false,
  sessionId,
  onConfigSaved
}: InlineChatModelSelectorProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [models, setModels] = useState<Model[]>([]);
  const [showApiKeyInput, setShowApiKeyInput] = useState(false);
  const [tempApiKey, setTempApiKey] = useState('');
  const [autoModelAvailable, setAutoModelAvailable] = useState(false);
  const [sessionConfig, setSessionConfig] = useState<SessionConfig | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<'success' | 'error' | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // 是否为老会话模式
  const isExistingSession = !!sessionId;

  useEffect(() => {
    loadProviders();
    checkAutoModel();
  }, []);

  // 加载老会话配置
  useEffect(() => {
    if (sessionId) {
      loadSessionConfig();
    }
  }, [sessionId]);

  useEffect(() => {
    const provider = sessionConfig?.provider || selectedConfig.provider;
    if (provider && provider !== 'auto' && !selectedConfig.is_auto) {
      loadModels(provider);
    }
  }, [selectedConfig.provider, sessionConfig?.provider]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
        setShowApiKeyInput(false);
        setTestResult(null);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const loadSessionConfig = async () => {
    if (!sessionId) return;
    try {
      setLoading(true);
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/sessions/${sessionId}/config`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setSessionConfig(response.data);
      // 同步到 selectedConfig
      if (response.data.provider && response.data.model) {
        onConfigChange({
          provider: response.data.provider,
          model: response.data.model,
          api_key: response.data.api_key,
          is_auto: false
        });
      }
    } catch (error) {
      console.error('Failed to load session config:', error);
    } finally {
      setLoading(false);
    }
  };

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
    // 对于老会话，选完模型显示 API key 输入
    // 对于新会话，检查是否需要 API key
    const hasApiKey = isExistingSession 
      ? sessionConfig?.api_key 
      : selectedConfig.api_key;
    
    if (!hasApiKey) {
      setShowApiKeyInput(true);
    } else {
      setIsOpen(false);
    }
  };

  const testApiKey = async () => {
    const provider = selectedConfig.provider;
    const apiKey = tempApiKey || sessionConfig?.api_key;
    
    if (!provider || !apiKey) return;
    
    try {
      setTesting(true);
      setTestResult(null);
      const token = localStorage.getItem('jwt_token');
      await axios.post(`${API_URL}/providers/${provider}/test`, 
        { api_key: apiKey },
        { headers: { Authorization: `Bearer ${token}` } }
      );
      setTestResult('success');
    } catch {
      setTestResult('error');
    } finally {
      setTesting(false);
    }
  };

  const handleSaveApiKey = async () => {
    if (!tempApiKey.trim()) return;

    // 对于老会话，保存到后端
    if (isExistingSession && sessionId) {
      try {
        setSaving(true);
        const token = localStorage.getItem('jwt_token');
        await axios.put(`${API_URL}/sessions/${sessionId}/config`, {
          provider: selectedConfig.provider,
          model: selectedConfig.model,
          api_key: tempApiKey
        }, {
          headers: { Authorization: `Bearer ${token}` }
        });
        
        // 更新本地状态
        setSessionConfig(prev => prev ? { ...prev, api_key: '****' + tempApiKey.slice(-4) } : null);
        onConfigChange({
          ...selectedConfig,
          api_key: tempApiKey
        });
        onConfigSaved?.();
        setTempApiKey('');
        setShowApiKeyInput(false);
        setIsOpen(false);
      } catch (error) {
        console.error('Failed to save API key:', error);
      } finally {
        setSaving(false);
      }
    } else {
      // 新会话，只更新本地状态
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
    
    // 优先使用 sessionConfig（老会话）
    const modelId = selectedConfig.model || sessionConfig?.model;
    const providerId = selectedConfig.provider || sessionConfig?.provider;
    
    if (modelId) {
      const model = models.find(m => m.id === modelId);
      return model?.name || modelId;
    }
    if (providerId) {
      const provider = providers.find(p => p.id === providerId);
      return provider?.name || 'Select model...';
    }
    return 'Auto';
  };

  const hasApiKey = isExistingSession 
    ? !!sessionConfig?.api_key 
    : !!selectedConfig.api_key;

  const needsApiKey = !selectedConfig.is_auto && selectedConfig.provider && selectedConfig.model && !hasApiKey;

  // 获取当前配置的显示状态
  const currentProvider = selectedConfig.provider || sessionConfig?.provider;
  const currentModel = selectedConfig.model || sessionConfig?.model;

  return (
    <div className="relative" ref={dropdownRef}>
      <button
        onClick={() => !disabled && !loading && setIsOpen(!isOpen)}
        disabled={disabled || loading}
        className={`flex items-center gap-1.5 px-2 py-1 rounded-md text-xs transition-colors ${
          selectedConfig.is_auto 
            ? 'bg-gradient-to-r from-purple-500/20 to-blue-500/20 text-purple-300 border border-purple-500/30' 
            : needsApiKey
              ? 'bg-red-500/10 text-red-400 border border-red-500/30'
              : 'bg-gradient-to-r from-purple-500/20 to-blue-500/20 text-purple-300 border border-purple-500/30'
        } ${disabled || loading ? 'opacity-50 cursor-not-allowed' : 'hover:from-purple-500/30 hover:to-blue-500/30 cursor-pointer'}`}
      >
        {loading ? (
          <Loader2 className="w-3 h-3 animate-spin" />
        ) : selectedConfig.is_auto ? (
          <Sparkles className="w-3 h-3" />
        ) : needsApiKey ? (
          <Key className="w-3 h-3" />
        ) : (
          <Sparkles className="w-3 h-3" />
        )}
        <span className="max-w-[120px] truncate">{getDisplayText()}</span>
        <ChevronDown className={`w-3 h-3 transition-transform ${isOpen ? 'rotate-180' : ''}`} />
      </button>

      {isOpen && (
        <div className="absolute bottom-full left-0 mb-1 w-72 bg-[#1a1a1a] border border-purple-500/30 rounded-lg shadow-xl z-50 max-h-96 overflow-hidden">
          {showApiKeyInput ? (
            <div className="p-3">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs font-medium text-purple-300">API Key Required</span>
                <button 
                  onClick={() => {
                    setShowApiKeyInput(false);
                    setTestResult(null);
                  }}
                  className="text-gray-500 hover:text-white"
                >
                  <X size={14} />
                </button>
              </div>
              
              {/* 显示当前 API Key（如果有） */}
              {isExistingSession && sessionConfig?.api_key && (
                <div className="mb-2 p-2 bg-purple-500/10 border border-purple-500/20 rounded text-[10px] text-purple-300">
                  Current: {sessionConfig.api_key}
                </div>
              )}
              
              <div className="relative">
                <input
                  type="password"
                  value={tempApiKey}
                  onChange={(e) => setTempApiKey(e.target.value)}
                  placeholder="Enter API key..."
                  className="w-full px-2 py-1.5 bg-black border border-purple-500/30 rounded text-xs text-white placeholder-gray-500 focus:outline-none focus:border-purple-500"
                  autoFocus
                />
                {testResult && (
                  <div className="absolute right-2 top-1/2 -translate-y-1/2">
                    {testResult === 'success' ? (
                      <Check className="w-4 h-4 text-green-400" />
                    ) : (
                      <AlertCircle className="w-4 h-4 text-red-400" />
                    )}
                  </div>
                )}
              </div>
              
              {/* Test Connection 按钮 */}
              <button
                onClick={testApiKey}
                disabled={testing || !tempApiKey.trim()}
                className="w-full mt-2 px-2 py-1.5 border border-purple-500/50 text-purple-300 text-xs rounded hover:bg-purple-500/10 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-1"
              >
                {testing ? (
                  <>
                    <Loader2 className="w-3 h-3 animate-spin" />
                    Testing...
                  </>
                ) : (
                  'Test Connection'
                )}
              </button>
              
              {/* Save 按钮 */}
              <button
                onClick={handleSaveApiKey}
                disabled={saving || !tempApiKey.trim()}
                className="w-full mt-2 px-2 py-1.5 bg-gradient-to-r from-purple-600 to-blue-600 text-white text-xs rounded hover:from-purple-700 hover:to-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-1"
              >
                {saving ? (
                  <>
                    <Loader2 className="w-3 h-3 animate-spin" />
                    Saving...
                  </>
                ) : (
                  'Save & Continue'
                )}
              </button>
            </div>
          ) : (
            <div className="overflow-y-auto max-h-80">
              {/* 老会话显示当前配置信息 */}
              {isExistingSession && sessionConfig && (
                <div className="p-2 border-b border-purple-500/20 bg-purple-500/5">
                  <div className="text-[10px] text-purple-300 mb-1">当前配置</div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-gray-300">{sessionConfig.provider} / {sessionConfig.model}</span>
                    <button
                      onClick={() => setShowApiKeyInput(true)}
                      className="text-[10px] text-purple-400 hover:text-purple-300 flex items-center gap-1"
                    >
                      <Key className="w-3 h-3" />
                      修改 Key
                    </button>
                  </div>
                </div>
              )}
              
              {/* Auto option */}
              {autoModelAvailable && (
                <div className="p-1 border-b border-purple-500/20">
                  <button
                    onClick={handleSelectAuto}
                    className={`w-full flex items-center gap-2 px-3 py-2 rounded text-left text-sm transition-colors ${
                      selectedConfig.is_auto 
                        ? 'bg-gradient-to-r from-purple-500/20 to-blue-500/20 text-purple-300' 
                        : 'text-gray-300 hover:bg-purple-500/10'
                    }`}
                  >
                    <Sparkles className="w-4 h-4 text-purple-400" />
                    <div>
                      <div className="font-medium">Auto</div>
                      <div className="text-[10px] text-gray-500">Z.AI GLM-4.5 (推荐)</div>
                    </div>
                  </button>
                </div>
              )}

              {/* Provider/Model selection */}
              <div className="p-1">
                <div className="px-2 py-1 text-[10px] text-purple-400 uppercase tracking-wider">
                  选择模型
                </div>
                {providers.map(provider => (
                  <div key={provider.id}>
                    <button
                      onClick={() => handleSelectProvider(provider.id)}
                      className={`w-full flex items-center gap-2 px-3 py-2 rounded text-left text-sm transition-colors ${
                        currentProvider === provider.id && !selectedConfig.is_auto
                          ? 'bg-purple-500/20 text-purple-300'
                          : 'text-gray-300 hover:bg-purple-500/10'
                      }`}
                    >
                      <div className="w-4 h-4 rounded bg-purple-500/20 flex items-center justify-center text-[10px] font-bold text-purple-300">
                        {provider.name.charAt(0)}
                      </div>
                      <span>{provider.name}</span>
                    </button>
                    
                    {/* Show models for selected provider */}
                    {currentProvider === provider.id && !selectedConfig.is_auto && (
                      <div className="ml-6 border-l border-purple-500/20 pl-2">
                        {models.map(model => (
                          <button
                            key={model.id}
                            onClick={() => handleSelectModel(model.id)}
                            className={`w-full px-3 py-1.5 rounded text-left text-xs transition-colors ${
                              currentModel === model.id
                                ? 'bg-purple-500/20 text-purple-300'
                                : 'text-gray-400 hover:bg-purple-500/10 hover:text-purple-200'
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
