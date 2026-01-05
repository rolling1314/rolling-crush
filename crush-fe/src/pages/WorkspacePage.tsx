import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { X, Plus, MessageSquare, LogOut, ChevronDown, ChevronRight, Trash2 } from 'lucide-react';
import axios from 'axios';
import { ChatPanel } from '../components/ChatPanel';
import { FileTree } from '../components/FileTree';
import { CodeEditor } from '../components/CodeEditor';
import { ModelSelector } from '../components/ModelSelector';
import { SessionConfigPanel } from '../components/SessionConfigPanel';
import { type FileNode, type Message, type PermissionRequest, type ToolCall, type ToolResult } from '../types';

const API_URL = 'http://localhost:8081/api';
const WS_URL = 'ws://localhost:8080/ws';

interface Session {
  id: string;
  title: string;
  message_count: number;
  created_at: number;
}

interface Project {
  id: string;
  name: string;
  description: string;
  host: string;
  port: number;
  workspace_path: string;
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

export default function WorkspacePage() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  
  const [project, setProject] = useState<Project | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [showNewSessionModal, setShowNewSessionModal] = useState(false);
  const [newSessionTitle, setNewSessionTitle] = useState('');
  const [sessionsCollapsed, setSessionsCollapsed] = useState(false);
  const [sessionToDelete, setSessionToDelete] = useState<string | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  
  // Model selection state
  const [modelConfig, setModelConfig] = useState<SessionModelConfig>({
    provider: '',
    model: '',
    max_tokens: 4096,
  });
  
  const [messages, setMessages] = useState<Message[]>([]);
  const [openFiles, setOpenFiles] = useState<FileNode[]>([]);
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [loadingFiles, setLoadingFiles] = useState(false);
  
  // Pending permissions state
  const [pendingPermissions, setPendingPermissions] = useState<Map<string, PermissionRequest>>(new Map());
  
  // WebSocket connection
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);

  const activeFile = openFiles.find(f => f.id === activeFileId) || null;

  useEffect(() => {
    loadProjectInfo();
    loadSessions();
  }, [projectId]);

  useEffect(() => {
    if (project?.workspace_path) {
      loadFiles(project.workspace_path);
    }
  }, [project]);

  // 当选择会话时加载消息历史
  useEffect(() => {
    if (currentSessionId) {
      loadSessionMessages(currentSessionId);
    }
  }, [currentSessionId]);

  // WebSocket 连接管理
  useEffect(() => {
    connectWebSocket();
    
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, []);

  const connectWebSocket = useCallback(() => {
    const token = localStorage.getItem('jwt_token');
    if (!token) {
      console.error('No JWT token found');
      return;
    }

    // 创建 WebSocket 连接，将 token 作为查询参数
    const ws = new WebSocket(`${WS_URL}?token=${encodeURIComponent(token)}`);
    
    ws.onopen = () => {
      console.log('WebSocket connected');
      wsRef.current = ws;
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleWebSocketMessage(data);
      } catch (error) {
        console.error('Failed to parse WebSocket message:', error);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    ws.onclose = () => {
      console.log('WebSocket disconnected, attempting to reconnect...');
      wsRef.current = null;
      
      // 5秒后重连
      reconnectTimeoutRef.current = setTimeout(() => {
        connectWebSocket();
      }, 5000);
    };
  }, []);

  const handleWebSocketMessage = (data: any) => {
    console.log('WebSocket message received:', data);
    
    // 后端直接广播 message.Message 对象
    // 检查是否是消息对象（有 ID, Role, Parts 等字段）
    if (data.ID && data.Role && data.Parts) {
      const convertedMsg = convertBackendMessageToFrontend(data);
      
      setMessages(prev => {
        // 检查消息是否已存在
        const existingIndex = prev.findIndex(m => m.id === convertedMsg.id);
        
        if (existingIndex !== -1) {
          // 更新现有消息（流式更新）
          const newMessages = [...prev];
          newMessages[existingIndex] = convertedMsg;
          return newMessages;
        } else {
          // 添加新消息
          return [...prev, convertedMsg];
        }
      });
    } else if (data.Type === 'permission_request' || data.type === 'permission_request') {
      // 处理权限请求
      console.log('=== Permission request received ===', data);
      const request: PermissionRequest = {
        id: data.id,
        session_id: data.session_id,
        tool_call_id: data.tool_call_id,
        tool_name: data.tool_name,
        action: data.action
      };
      
      console.log('Adding permission to map, tool_call_id:', request.tool_call_id);
      setPendingPermissions(prev => {
        const next = new Map(prev);
        next.set(request.tool_call_id, request);
        console.log('Pending permissions now has', next.size, 'items:', Array.from(next.keys()));
        return next;
      });
    } else if (data.Type === 'permission_notification' || data.type === 'permission_notification') {
      // 处理权限结果通知
      console.log('Permission notification:', data);
      setPendingPermissions(prev => {
        const next = new Map(prev);
        next.delete(data.tool_call_id);
        console.log('Removed permission, remaining:', next.size);
        return next;
      });
    }
  };

  const convertBackendMessageToFrontend = (backendMsg: any): Message => {
    let textContent = '';
    let reasoning = '';
    const toolCalls: ToolCall[] = [];
    const toolResults: ToolResult[] = [];
    
    if (backendMsg.Parts && Array.isArray(backendMsg.Parts)) {
      backendMsg.Parts.forEach((part: any) => {
        // Parts 直接包含字段
        if (part.text) {
          textContent += part.text;
        }
        if (part.thinking) {
          reasoning = part.thinking;
        }
        // 解析 tool_call
        if (part.name && part.input !== undefined && (part.id || part.ID)) {
          const toolCall: ToolCall = {
            id: part.id || part.ID,
            name: part.name,
            input: part.input,
            finished: part.finished ?? true,
            provider_executed: part.provider_executed ?? false
          };
          toolCalls.push(toolCall);
          console.log('Found tool call:', toolCall.id, toolCall.name);
        }
        // 解析 tool_result
        if (part.content !== undefined && (part.tool_call_id || part.ToolCallID)) {
          const toolResult: ToolResult = {
            tool_call_id: part.tool_call_id || part.ToolCallID,
            name: part.name || '',
            content: part.content,
            is_error: part.is_error ?? false
          };
          toolResults.push(toolResult);
          console.log('Found tool result for:', toolResult.tool_call_id);
        }
      });
    }
    
    // 检查消息是否完成（有 finish part）
    const isFinished = backendMsg.Parts?.some((part: any) => part.reason);
    
    return {
      id: backendMsg.ID || backendMsg.id,
      role: backendMsg.Role || backendMsg.role,
      content: textContent,
      reasoning: reasoning || undefined,
      timestamp: backendMsg.CreatedAt || backendMsg.created_at || Date.now(),
      isStreaming: !isFinished, // 如果没有 finish reason，说明还在流式传输
      toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
      toolResults: toolResults.length > 0 ? toolResults : undefined,
    };
  };

  const loadProjectInfo = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/projects/${projectId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setProject(response.data);
    } catch (error) {
      console.error('Failed to load project:', error);
    }
  };

  const loadSessions = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/projects/${projectId}/sessions`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const sessionList = response.data || [];
      setSessions(sessionList);
      
      // 自动选择第一个会话
      if (sessionList.length > 0 && !currentSessionId) {
        setCurrentSessionId(sessionList[0].id);
      }
    } catch (error) {
      console.error('Failed to load sessions:', error);
    }
  };

  const loadFiles = async (workspacePath: string) => {
    setLoadingFiles(true);
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await fetch(`http://localhost:8081/api/files?path=${encodeURIComponent(workspacePath)}`, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      
      if (!response.ok) {
        throw new Error('Failed to fetch files');
      }
      
      const data = await response.json();
      if (Array.isArray(data)) {
        setFiles(data);
      } else if (data && typeof data === 'object') {
        setFiles([data]);
      } else {
        setFiles([]);
      }
    } catch (error) {
      console.error('Error fetching files:', error);
      setFiles([]);
    } finally {
      setLoadingFiles(false);
    }
  };

  const loadSessionMessages = async (sessionId: string) => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/sessions/${sessionId}/messages`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      
      console.log('Loaded session messages:', response.data);
      
      // 转换后端消息格式为前端格式
      const backendMessages = response.data || [];
      const convertedMessages: Message[] = backendMessages.map((msg: any) => {
        let textContent = '';
        let reasoning = '';
        
        if (msg.Parts && Array.isArray(msg.Parts)) {
          msg.Parts.forEach((part: any) => {
            // Parts 直接包含字段，没有 type/data 包装
            if (part.text) {
              textContent += part.text;
            }
            if (part.thinking) {
              reasoning = part.thinking;
            }
          });
        }
        
        return {
          id: msg.ID || msg.id,
          role: msg.Role || msg.role,
          content: textContent,
          reasoning: reasoning || undefined,
          timestamp: msg.CreatedAt || msg.created_at || Date.now(),
          isStreaming: false,
        };
      });
      
      console.log('Converted messages:', convertedMessages);
      setMessages(convertedMessages);
    } catch (error) {
      console.error('Failed to load session messages:', error);
      setMessages([]);
    }
  };

  const createSession = async () => {
    if (!newSessionTitle.trim()) {
      alert('Please enter a session title');
      return;
    }
    
    if (!modelConfig.provider || !modelConfig.model) {
      alert('Please select a provider and model');
      return;
    }

    if (!modelConfig.api_key || !modelConfig.api_key.trim()) {
      alert('Please enter an API key');
      return;
    }
    
    try {
      const token = localStorage.getItem('jwt_token');
      
      // 直接创建session，配置会保存到session_model_configs表
      const response = await axios.post(`${API_URL}/sessions`, {
        project_id: projectId,
        title: newSessionTitle,
        model_config: modelConfig
      }, {
        headers: { Authorization: `Bearer ${token}` }
      });
      
      setShowNewSessionModal(false);
      setNewSessionTitle('');
      setModelConfig({
        provider: '',
        model: '',
        max_tokens: 4096,
      });
      loadSessions();
      setCurrentSessionId(response.data.id);
    } catch (error: any) {
      console.error('Failed to create session:', error);
      alert('Failed to create session: ' + (error.response?.data?.error || error.message));
    }
  };

  const handleSendMessage = (content: string) => {
    if (!currentSessionId) {
      console.error('No session selected');
      return;
    }

    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not connected');
      return;
    }

    // 添加用户消息到UI
    const userMessage: Message = {
      id: `temp-${Date.now()}`,
      role: 'user',
      content,
      timestamp: Date.now(),
      isStreaming: false,
    };
    setMessages(prev => [...prev, userMessage]);

    // 通过 WebSocket 发送消息
    const messageData = {
      type: 'message',
      content,
      sessionID: currentSessionId,
    };
    
    wsRef.current.send(JSON.stringify(messageData));
    console.log('Message sent via WebSocket:', messageData);
  };

  const handlePermissionResponse = (toolCallId: string, granted: boolean) => {
    const request = pendingPermissions.get(toolCallId);
    if (!request) {
      console.error('Permission request not found for tool_call_id:', toolCallId);
      return;
    }

    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not connected');
      return;
    }

    const response = {
      type: 'permission_response',
      id: request.id,
      tool_call_id: toolCallId,
      granted,
      denied: !granted
    };

    wsRef.current.send(JSON.stringify(response));
    console.log('Permission response sent:', response);

    // Optimistically remove from pending
    setPendingPermissions(prev => {
      const next = new Map(prev);
      next.delete(toolCallId);
      return next;
    });
  };

  const handleFileSelect = (file: FileNode) => {
    if (!openFiles.some(f => f.id === file.id)) {
      setOpenFiles(prev => [...prev, file]);
    }
    setActiveFileId(file.id);
  };

  const handleCloseTab = (e: React.MouseEvent, fileId: string) => {
    e.stopPropagation();
    setOpenFiles(prev => {
      const newFiles = prev.filter(f => f.id !== fileId);
      if (fileId === activeFileId) {
        const lastFile = newFiles[newFiles.length - 1];
        setActiveFileId(lastFile ? lastFile.id : null);
      }
      return newFiles;
    });
  };

  const handleDeleteSession = async (sessionId: string) => {
    try {
      const token = localStorage.getItem('jwt_token');
      await axios.delete(`${API_URL}/sessions/${sessionId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      
      // Remove session from list
      setSessions(prev => prev.filter(s => s.id !== sessionId));
      
      // If deleted session was current, clear current session
      if (currentSessionId === sessionId) {
        setCurrentSessionId(null);
        setMessages([]);
      }
      
      console.log('Session deleted successfully');
    } catch (error) {
      console.error('Failed to delete session:', error);
      alert('Failed to delete session');
    }
  };

  const confirmDeleteSession = (sessionId: string) => {
    setSessionToDelete(sessionId);
    setShowDeleteConfirm(true);
  };

  const handleDeleteConfirmed = async () => {
    if (sessionToDelete) {
      await handleDeleteSession(sessionToDelete);
      setSessionToDelete(null);
      setShowDeleteConfirm(false);
    }
  };

  const handleLogout = () => {
    localStorage.removeItem('jwt_token');
    localStorage.removeItem('username');
    localStorage.removeItem('user_id');
    navigate('/login');
  };

  return (
    <div className="flex h-screen w-screen bg-[#1e1e1e] overflow-hidden">
      {/* 最左侧：会话列表（可折叠） */}
      <div className={`${sessionsCollapsed ? 'w-12' : 'w-64'} bg-[#252526] border-r border-gray-700 flex flex-col transition-all duration-300`}>
        {sessionsCollapsed ? (
          // 折叠状态
          <div className="flex flex-col h-full">
            <button
              onClick={() => setSessionsCollapsed(false)}
              className="p-3 hover:bg-gray-700 border-b border-gray-700"
              title="Expand Sessions"
            >
              <ChevronRight size={18} className="text-gray-400" />
            </button>
            <div className="flex-1"></div>
            <button
              onClick={handleLogout}
              className="p-3 hover:bg-gray-700 border-t border-gray-700"
              title="Logout"
            >
              <LogOut size={18} className="text-gray-400" />
            </button>
          </div>
        ) : (
          // 展开状态
          <>
            <div className="p-4 border-b border-gray-700">
              <button
                onClick={() => navigate('/projects')}
                className="text-gray-400 hover:text-white text-sm mb-3"
              >
                ← Back to Projects
              </button>
              <div className="flex items-center justify-between">
                <h2 className="text-white font-semibold">Sessions</h2>
                <div className="flex gap-1">
                  <button
                    onClick={() => setShowNewSessionModal(true)}
                    className="p-1 hover:bg-gray-700 rounded"
                    title="New Session"
                  >
                    <Plus size={18} className="text-gray-400" />
                  </button>
                  <button
                    onClick={() => setSessionsCollapsed(true)}
                    className="p-1 hover:bg-gray-700 rounded"
                    title="Collapse"
                  >
                    <ChevronDown size={18} className="text-gray-400" />
                  </button>
                </div>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto">
              {sessions.map(session => (
                <div
                  key={session.id}
                  className={`group p-3 border-b border-gray-700 ${
                    currentSessionId === session.id ? 'bg-gray-700' : ''
                  }`}
                >
                  <div className="flex items-start gap-2">
                    <div 
                      className="flex items-start gap-2 flex-1 min-w-0 cursor-pointer"
                      onClick={() => setCurrentSessionId(session.id)}
                    >
                      <MessageSquare size={16} className="text-gray-400 mt-1 flex-shrink-0" />
                      <div className="flex-1 min-w-0">
                        <div className="text-white text-sm truncate">{session.title}</div>
                        <div className="text-gray-500 text-xs">{session.message_count} messages</div>
                      </div>
                    </div>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        confirmDeleteSession(session.id);
                      }}
                      className="opacity-0 group-hover:opacity-100 p-1 hover:bg-red-600/20 rounded transition-all"
                      title="Delete session"
                    >
                      <Trash2 size={14} className="text-red-400 hover:text-red-300" />
                    </button>
                  </div>
                </div>
              ))}
              
              {sessions.length === 0 && (
                <div className="p-4 text-center text-gray-500 text-sm">
                  No sessions yet.<br/>Click + to create one.
                </div>
              )}
            </div>

            {/* 退出登录按钮 */}
            <div className="p-3 border-t border-gray-700">
              <button
                onClick={handleLogout}
                className="w-full flex items-center gap-2 px-3 py-2 text-gray-400 hover:text-white hover:bg-gray-700 rounded transition-colors"
              >
                <LogOut size={16} />
                <span className="text-sm">Logout</span>
              </button>
            </div>
          </>
        )}
      </div>

      {/* 左侧：AI 助手和会话配置 */}
      <div className="w-[350px] shrink-0 border-r border-gray-700 flex flex-col">
        <div className="flex-1 overflow-hidden">
          <ChatPanel 
            messages={messages} 
            onSendMessage={handleSendMessage}
            pendingPermissions={pendingPermissions}
            onPermissionApprove={(toolCallId) => handlePermissionResponse(toolCallId, true)}
            onPermissionDeny={(toolCallId) => handlePermissionResponse(toolCallId, false)}
          />
        </div>
        {currentSessionId && (
          <div className="border-t border-gray-700 p-4 bg-[#1e1e1e] max-h-[400px] overflow-y-auto">
            <SessionConfigPanel sessionId={currentSessionId} />
          </div>
        )}
      </div>

      {/* 右侧：文件树和编辑器 */}
      <div className="flex-1 flex min-w-0">
        <div className="w-[250px] shrink-0 border-r border-gray-700 bg-[#1e1e1e]">
          {loadingFiles ? (
            <div className="flex items-center justify-center h-full text-gray-500 text-sm">
              Loading files...
            </div>
          ) : (
            <FileTree 
              data={files} 
              onSelectFile={handleFileSelect}
              selectedFileId={activeFileId || undefined}
            />
          )}
        </div>

        <div className="flex-1 min-w-0 bg-[#1e1e1e] flex flex-col">
          {openFiles.length > 0 ? (
            <>
              <div className="flex items-center bg-[#252526] border-b border-gray-700 overflow-x-auto no-scrollbar">
                {openFiles.map(file => (
                  <div
                    key={file.id}
                    onClick={() => setActiveFileId(file.id)}
                    className={`flex items-center gap-2 px-3 py-2 text-sm border-r border-gray-700 cursor-pointer ${
                      activeFileId === file.id 
                        ? 'bg-[#1e1e1e] text-white' 
                        : 'bg-[#2d2d2d] text-gray-400 hover:bg-[#2d2d2d]/80'
                    }`}
                  >
                    <span className="truncate">{file.name}</span>
                    <button
                      onClick={(e) => handleCloseTab(e, file.id)}
                      className="p-0.5 rounded hover:bg-white/20"
                    >
                      <X size={12} />
                    </button>
                  </div>
                ))}
              </div>

              <div className="flex-1 overflow-hidden">
                {activeFile && (
                  <CodeEditor 
                    key={activeFile.id}
                    code={activeFile.content || '// No content'} 
                    onChange={() => {}}
                    readOnly={false}
                  />
                )}
              </div>
            </>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-500">
              Select a file to view
            </div>
          )}
        </div>
      </div>

      {/* 新建会话模态框 */}
      {showNewSessionModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-[#252526] p-6 rounded-lg w-[500px] max-h-[80vh] overflow-y-auto border border-gray-700">
            <h2 className="text-xl font-bold text-white mb-4">New Session</h2>
            
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-300 mb-2">
                  Session Title
                </label>
                <input
                  type="text"
                  placeholder="Enter session title..."
                  value={newSessionTitle}
                  onChange={e => setNewSessionTitle(e.target.value)}
                  onKeyPress={e => e.key === 'Enter' && createSession()}
                  className="w-full px-4 py-2 bg-[#3c3c3c] border border-gray-600 rounded text-white focus:outline-none focus:border-blue-500"
                  autoFocus
                />
              </div>

              <ModelSelector 
                onConfigChange={(config) => setModelConfig(config)}
                initialConfig={modelConfig}
                showAdvanced={false}
              />
            </div>

            <div className="flex gap-2 mt-6">
              <button
                onClick={createSession}
                disabled={!newSessionTitle.trim() || !modelConfig.provider || !modelConfig.model || !modelConfig.api_key}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                Create
              </button>
              <button
                onClick={() => {
                  setShowNewSessionModal(false);
                  setNewSessionTitle('');
                }}
                className="flex-1 px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 删除确认对话框 */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-[#252526] p-6 rounded-lg w-[400px] border border-gray-700">
            <h2 className="text-xl font-bold text-white mb-4">Delete Session</h2>
            
            <p className="text-gray-300 mb-6">
              Are you sure you want to delete this session? This action cannot be undone.
            </p>

            <div className="flex gap-2">
              <button
                onClick={() => {
                  setShowDeleteConfirm(false);
                  setSessionToDelete(null);
                }}
                className="flex-1 px-4 py-2 bg-gray-600 text-white rounded hover:bg-gray-700"
              >
                Cancel
              </button>
              <button
                onClick={handleDeleteConfirmed}
                className="flex-1 px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700"
              >
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

