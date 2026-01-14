import { useState, useEffect, useRef, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { X, GripVertical, Eye, Code2 } from 'lucide-react';
import axios from 'axios';
import { ChatPanel } from '../components/ChatPanel';
import { ChatSidebar } from '../components/ChatSidebar';
import { FileTree } from '../components/FileTree';
import { CodeEditor } from '../components/CodeEditor';
import { InlineChatModelSelector } from '../components/InlineChatModelSelector';
import { type FileNode, type Message, type PermissionRequest, type ToolCall, type ToolResult, type Session } from '../types';

const API_URL = '/api';
const WS_URL = '/ws';

interface Project {
  id: string;
  name: string;
  description: string;
  external_ip: string;
  frontend_port: number;
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
  is_auto?: boolean;
}

export default function WorkspacePage() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  
  const [project, setProject] = useState<Project | null>(null);
  const [projectLoading, setProjectLoading] = useState(true);
  const [projectError, setProjectError] = useState<string | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [currentSessionId, setCurrentSessionId] = useState<string | null>(null);
  const [sessionToDelete, setSessionToDelete] = useState<string | null>(null);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);
  
  // Get username from localStorage
  const username = localStorage.getItem('username') || undefined;
  
  // Pending session state - for new sessions that haven't been created yet
  // When user clicks "New Session", we create a pending session state
  // The actual session is created when user sends the first message
  const [isPendingSession, setIsPendingSession] = useState(false);
  
  // Model selection state for new/pending sessions
  const [pendingModelConfig, setPendingModelConfig] = useState<SessionModelConfig>({
    provider: 'auto',
    model: 'auto',
    is_auto: true,
  });
  
  const [messages, setMessages] = useState<Message[]>([]);
  const [openFiles, setOpenFiles] = useState<FileNode[]>([]);
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [loadingFiles, setLoadingFiles] = useState(false);
  const [expandedFolderIds, setExpandedFolderIds] = useState<Set<string>>(new Set());
  const [expandedFoldersLoaded, setExpandedFoldersLoaded] = useState(false);
  const [openFilesLoaded, setOpenFilesLoaded] = useState(false);
  
  // Pending permissions state
  const [pendingPermissions, setPendingPermissions] = useState<Map<string, PermissionRequest>>(new Map());
  
  // Processing state - 是否正在处理请求
  const [isProcessing, setIsProcessing] = useState(false);
  
  // View mode state
  const [viewMode, setViewMode] = useState<'code' | 'preview'>('code');
  
  // Resizable panel state
  const [chatPanelWidth, setChatPanelWidth] = useState(() => {
    const saved = localStorage.getItem('chat_panel_width');
    return saved ? parseInt(saved, 10) : 400;
  });
  const [isResizing, setIsResizing] = useState(false);
  const resizeRef = useRef<HTMLDivElement>(null);
  const chatPanelDivRef = useRef<HTMLDivElement>(null);
  
  // WebSocket connection
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  
  // Track last received Redis stream message ID for reconnection
  // Persist per session in localStorage to survive page refresh
  const lastStreamIdRef = useRef<string>('');
  
  // Helper to get/set last stream ID from localStorage
  const getLastStreamId = (sessionId: string): string => {
    return localStorage.getItem(`last_stream_id_${sessionId}`) || '';
  };
  
  const setLastStreamId = (sessionId: string, streamId: string) => {
    lastStreamIdRef.current = streamId;
    if (sessionId && streamId) {
      localStorage.setItem(`last_stream_id_${sessionId}`, streamId);
    }
  };

  const activeFile = openFiles.find(f => f.id === activeFileId) || null;

  // Refs for accessing latest state in WebSocket callbacks
  const projectRef = useRef<Project | null>(null);
  const currentSessionIdRef = useRef<string | null>(null);

  useEffect(() => {
    projectRef.current = project;
  }, [project]);

  useEffect(() => {
    currentSessionIdRef.current = currentSessionId;
  }, [currentSessionId]);

  useEffect(() => {
    loadProjectInfo();
    loadSessions();
  }, [projectId]);

  useEffect(() => {
    if (project?.workspace_path && projectId) {
      loadFiles(project.workspace_path, projectId);
    }
  }, [project, projectId]);

  // Load open files from localStorage
  useEffect(() => {
    if (!project) return;
    const storageKey = `open_files_${project.id}`;
    const savedOpenFiles = localStorage.getItem(storageKey);
    if (savedOpenFiles) {
      try {
        const { files, activeId } = JSON.parse(savedOpenFiles);
        setOpenFiles(files);
        if (activeId) {
          setActiveFileId(activeId);
        }
      } catch (e) {
        console.error('Failed to parse saved open files', e);
      }
    }
    setOpenFilesLoaded(true);
  }, [project]);

  // Save open files to localStorage
  useEffect(() => {
    if (!project || !openFilesLoaded) return;
    const storageKey = `open_files_${project.id}`;
    localStorage.setItem(storageKey, JSON.stringify({
      files: openFiles,
      activeId: activeFileId
    }));
  }, [openFiles, activeFileId, project, openFilesLoaded]);

  // Load expanded folders from localStorage
  useEffect(() => {
    if (!project) return;
    const storageKey = `expanded_folders_${project.id}`;
    const saved = localStorage.getItem(storageKey);
    if (saved) {
      try {
        setExpandedFolderIds(new Set(JSON.parse(saved)));
      } catch (e) {
        console.error('Failed to parse saved expanded folders', e);
      }
    }
    setExpandedFoldersLoaded(true);
  }, [project]);

  // Save expanded folders to localStorage
  useEffect(() => {
    if (!project || !expandedFoldersLoaded) return;
    const storageKey = `expanded_folders_${project.id}`;
    localStorage.setItem(storageKey, JSON.stringify(Array.from(expandedFolderIds)));
  }, [expandedFolderIds, project, expandedFoldersLoaded]);

  // Panel resize handlers
  const chatPanelWidthRef = useRef(chatPanelWidth);
  // We don't update ref automatically here because we want to control it manually during resize
  // chatPanelWidthRef.current = chatPanelWidth; 
  
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!isResizing) return;
      
      const containerWidth = window.innerWidth;
      const newWidth = containerWidth - e.clientX;
      
      // Constrain width between 280px and 800px
      const constrainedWidth = Math.min(Math.max(newWidth, 280), 800);
      
      // Update DOM directly for performance
      if (chatPanelDivRef.current) {
        chatPanelDivRef.current.style.width = `${constrainedWidth}px`;
      }
      
      // Keep ref updated for mouseup
      chatPanelWidthRef.current = constrainedWidth;
    };
    
    const handleMouseUp = () => {
      if (isResizing) {
        setIsResizing(false);
        // Update React state only once at the end
        setChatPanelWidth(chatPanelWidthRef.current);
        localStorage.setItem('chat_panel_width', chatPanelWidthRef.current.toString());
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
      }
    };
    
    if (isResizing) {
      document.body.style.cursor = 'col-resize';
      document.body.style.userSelect = 'none';
      // Initialize ref with current state when starting
      chatPanelWidthRef.current = chatPanelWidth;
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
    }
    
    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isResizing, chatPanelWidth]);

  const handleResizeStart = (e: React.MouseEvent) => {
    e.preventDefault();
    setIsResizing(true);
  };

  const handleToggleExpand = (id: string) => {
    setExpandedFolderIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  // 当选择会话时加载消息历史
  useEffect(() => {
    if (currentSessionId) {
      loadSessionMessages(currentSessionId);
    }
  }, [currentSessionId]);

  // WebSocket 连接函数 - 需要在 useEffect 之前定义
  const connectWebSocket = useCallback((sessionId: string | null, isReconnect: boolean = false) => {
    const token = localStorage.getItem('jwt_token');
    if (!token) {
      console.error('No JWT token found');
      return;
    }

    // 关闭现有连接
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }

    // 清除重连定时器
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    // 创建 WebSocket 连接，将 token 和 session_id 作为查询参数
    const sessionParam = sessionId ? `&session_id=${encodeURIComponent(sessionId)}` : '';
    const ws = new WebSocket(`${WS_URL}?token=${encodeURIComponent(token)}${sessionParam}`);
    
    ws.onopen = () => {
      console.log('WebSocket connected', sessionId ? `to session: ${sessionId}` : '(no session)');
      wsRef.current = ws;
      
      // 连接到已有会话时，总是发送 reconnect 请求检查是否有错过的消息
      // 这对于页面刷新时 AI 还在生成的情况很重要
      if (sessionId) {
        // 从 localStorage 恢复上次的 stream ID（页面刷新后仍可用）
        const savedStreamId = getLastStreamId(sessionId);
        lastStreamIdRef.current = savedStreamId;
        
        console.log('Sending reconnect request with lastStreamId:', savedStreamId || '(from beginning)');
        ws.send(JSON.stringify({
          type: 'reconnect',
          sessionID: sessionId,
          lastMsgId: savedStreamId, // 如果为空，后端会从头读取
        }));
      }
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
      
      // 3秒后重连（设置 isReconnect=true 以触发消息恢复）
      reconnectTimeoutRef.current = setTimeout(() => {
        connectWebSocket(sessionId, true);
      }, 3000);
    };
  }, []);

  // WebSocket 连接管理 - 当 session 变化时重新连接
  useEffect(() => {
    // 只有当有 sessionId 时才连接（确保消息路由到正确的 session）
    if (currentSessionId) {
      connectWebSocket(currentSessionId);
    }
    
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [currentSessionId, connectWebSocket]);

  const handleWebSocketMessage = (data: any) => {
    console.log('=== WebSocket message received ===');
    console.log('Type:', data.Type || data.type);
    console.log('Data:', data);
    
    // 保存 Redis stream ID 用于重连时恢复消息
    if (data._streamId && currentSessionIdRef.current) {
      setLastStreamId(currentSessionIdRef.current, data._streamId);
      console.log('Updated lastStreamId:', data._streamId);
    }
    
    // 处理重放的消息（来自 Redis 缓存）
    if (data._replay) {
      console.log('=== Replaying message from Redis ===');
      console.log('Stream ID:', data._streamId);
      console.log('Type:', data._type);
      console.log('Timestamp:', data._timestamp);
      
      // 根据原始消息类型处理
      const originalPayload = data._payload;
      if (data._type === 'message') {
        // 这是一个标准消息，直接处理 payload
        handleReplayedMessage(originalPayload);
      } else if (data._type === 'permission_request') {
        // 权限请求
        handlePermissionRequestMessage(originalPayload);
      } else if (data._type === 'session_update') {
        // Session 更新
        handleSessionUpdateMessage(originalPayload);
      }
      return;
    }
    
    // 处理重连状态通知
    if (data.Type === 'reconnection_status') {
      console.log('=== Reconnection status ===');
      console.log('Messages replayed:', data.messages_replayed);
      console.log('Generation still active:', data.generation_active);
      console.log('Last stream ID:', data.last_stream_id);
      
      if (data.last_stream_id && currentSessionIdRef.current) {
        setLastStreamId(currentSessionIdRef.current, data.last_stream_id);
      }
      
      // 如果生成仍在进行，保持 isProcessing 状态
      if (data.generation_active) {
        setIsProcessing(true);
      }
      return;
    }
    
    // 处理生成完成通知
    if (data.Type === 'generation_complete') {
      console.log('=== Generation complete ===');
      console.log('Session ID:', data.session_id);
      console.log('Has error:', data.error);
      setIsProcessing(false);
      return;
    }
    
    // 处理权限请求 - 优先处理，确保在消息之前
    if (data.Type === 'permission_request' || data.type === 'permission_request') {
      // 处理权限请求
      console.log('=== Permission request received ===');
      console.log('Full data:', JSON.stringify(data, null, 2));
      const request: PermissionRequest = {
        id: data.id,
        session_id: data.session_id,
        tool_call_id: data.tool_call_id,
        tool_name: data.tool_name,
        action: data.action
      };
      
      console.log('Parsed PermissionRequest:', request);
      console.log('Tool call ID to add to map:', request.tool_call_id);
      
      setPendingPermissions(prev => {
        const next = new Map(prev);
        next.set(request.tool_call_id, request);
        console.log('=== Updated pendingPermissions ===');
        console.log('Map size:', next.size);
        console.log('Map keys:', Array.from(next.keys()));
        console.log('Map entries:', Array.from(next.entries()).map(([k, v]) => ({ key: k, value: v })));
        return next;
      });
      return; // 立即返回，不处理其他逻辑
    }
    
    // 处理权限通知
    if (data.Type === 'permission_notification' || data.type === 'permission_notification') {
      console.log('Permission notification:', data);
      setPendingPermissions(prev => {
        const next = new Map(prev);
        next.delete(data.tool_call_id);
        console.log('Removed permission, remaining:', next.size);
        return next;
      });
      return; // 立即返回
    }
    
    // 处理 Session 更新 - 实时更新上下文和费用（复用 TUI 的 PubSub 机制）
    if (data.Type === 'session_update' || data.type === 'session_update') {
      console.log('=== Session update received ===');
      console.log('Full data:', data);
      console.log('Session ID:', data.id);
      console.log('Prompt tokens:', data.prompt_tokens);
      console.log('Completion tokens:', data.completion_tokens);
      console.log('Total tokens:', data.prompt_tokens + data.completion_tokens);
      console.log('Context window:', data.context_window);
      console.log('Cost:', data.cost);
      console.log('Percentage:', ((data.prompt_tokens + data.completion_tokens) / data.context_window * 100).toFixed(2) + '%');
      
      setSessions(prev => {
        const updated = prev.map(s => {
          if (s.id === data.id) {
            // 更新session信息（像 TUI 的 header/sidebar 组件一样）
            const updatedSession = {
              ...s,
              prompt_tokens: data.prompt_tokens,
              completion_tokens: data.completion_tokens,
              cost: data.cost,
              context_window: data.context_window,
              message_count: data.message_count,
              updated_at: data.updated_at
            };
            console.log('Updated session:', updatedSession);
            return updatedSession;
          }
          return s;
        });
        return updated;
      });
      return; // 立即返回
    }
    
    // 后端直接广播 message.Message 对象
    // 支持大写和小写字段名（ID/id, Role/role, Parts/parts）
    const msgId = data.ID || data.id;
    const msgRole = data.Role || data.role;
    const msgParts = data.Parts || data.parts;
    
    if (msgId && msgRole && msgParts) {
      // 标准化数据格式
      const normalizedData = {
        ...data,
        ID: msgId,
        Role: msgRole,
        Parts: msgParts,
        CreatedAt: data.CreatedAt || data.created_at || Date.now()
      };
      
      const convertedMsg = convertBackendMessageToFrontend(normalizedData);
      console.log('=== Converted message ===');
      console.log('Message ID:', convertedMsg.id);
      console.log('Role:', convertedMsg.role);
      console.log('Tool calls:', convertedMsg.toolCalls?.map(tc => ({ id: tc.id, name: tc.name, finished: tc.finished })));
      
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
      
      // 如果消息处理完成（不再流式传输），重置处理状态
      if (!convertedMsg.isStreaming && convertedMsg.role === 'assistant') {
        setIsProcessing(false);
      }
      
      // 如果消息包含工具调用结果，刷新文件树（因为工具可能修改了文件）
      // Use refs to get latest state inside WebSocket callback closure
      const currentProject = projectRef.current;

      if (convertedMsg.toolResults && convertedMsg.toolResults.length > 0 && currentProject?.workspace_path && currentProject?.id) {
        console.log('Tool result detected, refreshing file tree...');
        loadFiles(currentProject.workspace_path, currentProject.id, true);
      }
    }
  };

  // Helper function to handle replayed messages from Redis
  const handleReplayedMessage = (payload: any) => {
    // Normalize the payload (it might be the original message format)
    const msgId = payload.ID || payload.id;
    const msgRole = payload.Role || payload.role;
    const msgParts = payload.Parts || payload.parts;
    
    if (msgId && msgRole && msgParts) {
      const normalizedData = {
        ...payload,
        ID: msgId,
        Role: msgRole,
        Parts: msgParts,
        CreatedAt: payload.CreatedAt || payload.created_at || Date.now()
      };
      
      const convertedMsg = convertBackendMessageToFrontend(normalizedData);
      console.log('Replayed message converted:', convertedMsg.id);
      
      setMessages(prev => {
        const existingIndex = prev.findIndex(m => m.id === convertedMsg.id);
        if (existingIndex !== -1) {
          const newMessages = [...prev];
          newMessages[existingIndex] = convertedMsg;
          return newMessages;
        } else {
          return [...prev, convertedMsg];
        }
      });
    }
  };

  // Helper function to handle permission request messages
  const handlePermissionRequestMessage = (payload: any) => {
    const request: PermissionRequest = {
      id: payload.id,
      session_id: payload.session_id,
      tool_call_id: payload.tool_call_id,
      tool_name: payload.tool_name,
      action: payload.action
    };
    
    setPendingPermissions(prev => {
      const next = new Map(prev);
      next.set(request.tool_call_id, request);
      return next;
    });
  };

  // Helper function to handle session update messages
  const handleSessionUpdateMessage = (payload: any) => {
    setSessions(prev => {
      return prev.map(s => {
        if (s.id === payload.id) {
          return {
            ...s,
            prompt_tokens: payload.prompt_tokens,
            completion_tokens: payload.completion_tokens,
            cost: payload.cost,
            context_window: payload.context_window,
            message_count: payload.message_count,
            updated_at: payload.updated_at
          };
        }
        return s;
      });
    });
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
    setProjectLoading(true);
    setProjectError(null);
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/projects/${projectId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setProject(response.data);
    } catch (error: any) {
      console.error('Failed to load project:', error);
      setProjectError(error.response?.data?.error || error.message || 'Failed to load project');
    } finally {
      setProjectLoading(false);
    }
  };

  const loadSessions = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/projects/${projectId}/sessions`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const sessionList = response.data || [];
      
      // 调试：打印每个session的context_window
      console.log('=== Loaded sessions ===');
      sessionList.forEach((s: Session) => {
        console.log(`Session: ${s.title}, context_window: ${s.context_window}, tokens: ${s.prompt_tokens + s.completion_tokens}`);
      });
      
      setSessions(sessionList);
      
      // 自动选择第一个会话，如果没有会话则启动一个待定会话
      if (sessionList.length > 0 && !currentSessionId && !isPendingSession) {
        setCurrentSessionId(sessionList[0].id);
      } else if (sessionList.length === 0 && !isPendingSession) {
        // 如果没有会话，启动一个待定会话
        setIsPendingSession(true);
        setCurrentSessionId(null);
        setMessages([]);
        setPendingModelConfig({
          provider: 'auto',
          model: 'auto',
          is_auto: true,
        });
      }
    } catch (error) {
      console.error('Failed to load sessions:', error);
    }
  };

  const loadFiles = async (workspacePath: string, projectIdOrSessionId?: string, isBackground: boolean = false) => {
    if (!isBackground) {
      setLoadingFiles(true);
    }
    try {
      // 使用传入的 projectId（优先）或当前项目ID
      const effectiveProjectId = projectIdOrSessionId || projectId;
      
      if (!effectiveProjectId) {
        console.warn('No project ID available, skipping file tree load');
        setFiles([]);
        if (!isBackground) {
          setLoadingFiles(false);
        }
        return;
      }
      
      const token = localStorage.getItem('jwt_token');
      // 使用 project_id 而不是 session_id 来加载文件
      const url = `/api/files?project_id=${encodeURIComponent(effectiveProjectId)}&path=${encodeURIComponent(workspacePath)}`;
      console.log('Loading file tree:', { projectId: effectiveProjectId, path: workspacePath, url, isBackground });
      
      const response = await fetch(url, {
        headers: { 'Authorization': `Bearer ${token}` }
      });
      
      if (!response.ok) {
        const errorData = await response.json().catch(() => ({ error: 'Unknown error' }));
        console.error('Failed to fetch files:', errorData);
        throw new Error(errorData.error || 'Failed to fetch files');
      }
      
      const data = await response.json();
      console.log('File tree loaded:', data);
      
      if (Array.isArray(data)) {
        setFiles(data);
      } else if (data && typeof data === 'object') {
        setFiles([data]);
      } else {
        setFiles([]);
      }
    } catch (error) {
      console.error('Error fetching files:', error);
      if (!isBackground) {
        setFiles([]);
      }
    } finally {
      if (!isBackground) {
        setLoadingFiles(false);
      }
    }
  };

  const loadSessionMessages = async (sessionId: string) => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/sessions/${sessionId}/messages`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      
      console.log('Loaded session messages:', response.data);
      
      // 转换后端消息格式为前端格式 - 复用 convertBackendMessageToFrontend 逻辑
      const backendMessages = response.data || [];
      const convertedMessages: Message[] = backendMessages.map((msg: any) => {
        let textContent = '';
        let reasoning = '';
        const toolCalls: ToolCall[] = [];
        const toolResults: ToolResult[] = [];
        
        if (msg.Parts && Array.isArray(msg.Parts)) {
          msg.Parts.forEach((part: any) => {
            // Parts 直接包含字段，没有 type/data 包装
            if (part.text) {
              textContent += part.text;
            }
            if (part.thinking) {
              reasoning = part.thinking;
            }
            // 解析 tool_call - 检查是否有 name 和 input 字段
            if (part.name && part.input !== undefined && (part.id || part.ID)) {
              const toolCall: ToolCall = {
                id: part.id || part.ID,
                name: part.name,
                input: part.input,
                finished: part.finished ?? true,
                provider_executed: part.provider_executed ?? false
              };
              toolCalls.push(toolCall);
              console.log('History: Found tool call:', toolCall.id, toolCall.name);
            }
            // 解析 tool_result - 检查是否有 tool_call_id 字段
            if (part.content !== undefined && (part.tool_call_id || part.ToolCallID)) {
              const toolResult: ToolResult = {
                tool_call_id: part.tool_call_id || part.ToolCallID,
                name: part.name || '',
                content: part.content,
                is_error: part.is_error ?? false,
                metadata: part.metadata
              };
              toolResults.push(toolResult);
              console.log('History: Found tool result for:', toolResult.tool_call_id);
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
          toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
          toolResults: toolResults.length > 0 ? toolResults : undefined,
        };
      });
      
      console.log('Converted messages:', convertedMessages);
      setMessages(convertedMessages);
    } catch (error) {
      console.error('Failed to load session messages:', error);
      setMessages([]);
    }
  };

  // Create a new session with the given config
  // This is called when user sends first message in a pending session
  const createSessionWithConfig = async (config: SessionModelConfig, firstMessageContent?: string): Promise<string | null> => {
    try {
      const token = localStorage.getItem('jwt_token');
      
      // Generate a title from the first message content or use default
      const title = firstMessageContent 
        ? (firstMessageContent.length > 50 
            ? firstMessageContent.substring(0, 50) + '...' 
            : firstMessageContent)
        : `New Session ${new Date().toLocaleString()}`;
      
      // Prepare request body
      const requestBody: {
        project_id: string | undefined;
        title: string;
        is_auto?: boolean;
        model_config?: SessionModelConfig;
      } = {
        project_id: projectId,
        title: title,
      };
      
      // If using auto model, set is_auto flag
      if (config.is_auto) {
        requestBody.is_auto = true;
      } else {
        // Otherwise pass the model config
        requestBody.model_config = {
          provider: config.provider,
          model: config.model,
          api_key: config.api_key,
          base_url: config.base_url,
        };
      }
      
      const response = await axios.post(`${API_URL}/sessions`, requestBody, {
        headers: { Authorization: `Bearer ${token}` }
      });
      
      console.log('Session created:', response.data);
      loadSessions();
      return response.data.id;
    } catch (error: any) {
      console.error('Failed to create session:', error);
      alert('Failed to create session: ' + (error.response?.data?.error || error.message));
      return null;
    }
  };
  
  // Start a new pending session (called when user clicks "New Session")
  const startNewSession = () => {
    setIsPendingSession(true);
    setCurrentSessionId(null);
    setMessages([]);
    setPendingModelConfig({
      provider: 'auto',
      model: 'auto',
      is_auto: true,
    });
  };

  const handleSendMessage = async (content: string, contextFiles: FileNode[] = [], images: { url: string; filename: string; mime_type: string }[] = []) => {
    let sessionId = currentSessionId;
    
    // If this is a pending session, create the session first
    if (isPendingSession || !sessionId) {
      console.log('Creating session for pending session...');
      
      // Validate config for non-auto models
      if (!pendingModelConfig.is_auto && (!pendingModelConfig.provider || !pendingModelConfig.model)) {
        alert('Please select a model');
        return;
      }
      
      if (!pendingModelConfig.is_auto && !pendingModelConfig.api_key) {
        alert('Please enter an API key for the selected model');
        return;
      }
      
      // Create session with config
      const newSessionId = await createSessionWithConfig(pendingModelConfig, content);
      if (!newSessionId) {
        return; // Creation failed, error already shown
      }
      
      sessionId = newSessionId;
      setCurrentSessionId(newSessionId);
      setIsPendingSession(false);
      
      // Connect WebSocket to the new session
      connectWebSocket(newSessionId);
      
      // Wait a bit for WebSocket to connect
      await new Promise(resolve => setTimeout(resolve, 500));
    }

    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not connected');
      // Try to reconnect
      if (sessionId) {
        connectWebSocket(sessionId);
        await new Promise(resolve => setTimeout(resolve, 500));
      }
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
        alert('WebSocket connection failed. Please try again.');
        return;
      }
    }

    let messageContent = content;

    // 如果有附带文件，将其内容追加到消息中
    if (contextFiles.length > 0) {
        const fileContexts = contextFiles.map(file => {
            if (file.type === 'folder') {
                return `Folder: ${file.path || file.name} (Context of this folder)`;
            }

            // 尝试从 openFiles 中查找以获取最新内容（如果是打开的）
            const openFile = openFiles.find(f => f.id === file.id);
            const fileContent = openFile?.content || file.content || '// Content not available';
            
            const lineInfo = (file.startLine !== undefined && file.endLine !== undefined) 
                ? ` (${file.startLine}-${file.endLine})` 
                : '';
                
            return `File: ${file.path || file.name}${lineInfo}\n\`\`\`${file.name.split('.').pop() || ''}\n${fileContent}\n\`\`\``;
        }).join('\n\n');

        if (messageContent.trim()) {
            messageContent += '\n\n';
        }
        messageContent += `Context Files:\n${fileContexts}`;
    }

    // 不在前端预先添加用户消息，后端会广播回来
    // 这样避免消息重复显示

    // 通过 WebSocket 发送消息，包含图片附件
    const messageData: {
      type: string;
      content: string;
      sessionID: string;
      images?: { url: string; filename: string; mime_type: string }[];
    } = {
      type: 'message',
      content: messageContent,
      sessionID: sessionId!,
    };
    
    // Add images if any
    if (images.length > 0) {
      messageData.images = images.map(img => ({
        url: img.url,
        filename: img.filename,
        mime_type: img.mime_type,
      }));
      console.log('Sending message with images:', messageData.images);
    }
    
    wsRef.current.send(JSON.stringify(messageData));
    console.log('Message sent via WebSocket:', messageData);
    
    // 设置处理中状态
    setIsProcessing(true);
  };

  // 取消当前请求
  const handleCancelRequest = () => {
    if (!currentSessionId) {
      console.error('No session to cancel');
      return;
    }

    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) {
      console.error('WebSocket not connected');
      return;
    }

    const cancelData = {
      type: 'cancel',
      sessionID: currentSessionId,
    };
    
    wsRef.current.send(JSON.stringify(cancelData));
    console.log('Cancel request sent:', cancelData);
    setIsProcessing(false);
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

  // Helper function to find a file node and collect parent folder IDs
  // Matches by relative path (node.path) or by checking if the absolute path ends with the relative path
  const findFileAndParents = (nodes: FileNode[], targetPath: string, workspacePath: string, parentIds: string[] = []): { found: boolean; parentIds: string[]; fileNode?: FileNode } => {
    // Convert absolute path to relative path for matching
    let relativePath = targetPath;
    if (workspacePath && targetPath.startsWith(workspacePath)) {
      relativePath = targetPath.slice(workspacePath.length);
      if (!relativePath.startsWith('/')) {
        relativePath = '/' + relativePath;
      }
    }
    
    for (const node of nodes) {
      // Match by relative path or check if paths match
      const nodePathNormalized = node.path?.startsWith('/') ? node.path : '/' + node.path;
      const targetNormalized = relativePath.startsWith('/') ? relativePath : '/' + relativePath;
      
      if (nodePathNormalized === targetNormalized || node.path === targetPath || targetPath.endsWith(nodePathNormalized)) {
        return { found: true, parentIds, fileNode: node };
      }
      
      if (node.type === 'folder' && node.children) {
        const result = findFileAndParents(node.children, targetPath, workspacePath, [...parentIds, node.id]);
        if (result.found) {
          return result;
        }
      }
    }
    return { found: false, parentIds: [] };
  };

  // Handle clicking a file path from tool call display - simulates clicking on file tree
  const handleFileClickFromTool = (filePath: string) => {
    console.log('File clicked from tool:', filePath);
    console.log('Project workspace:', project?.workspace_path);
    
    // Find the file in the existing file tree
    const searchResult = findFileAndParents(files, filePath, project?.workspace_path || '');
    console.log('Search result:', searchResult);
    
    if (searchResult.found && searchResult.fileNode) {
      // Expand all parent folders at once
      if (searchResult.parentIds.length > 0) {
        setExpandedFolderIds(prev => {
          const next = new Set(prev);
          searchResult.parentIds.forEach(id => next.add(id));
          return next;
        });
        console.log('Expanded folders:', searchResult.parentIds);
      }
      
      // Simulate clicking on the file - use handleFileSelect
      if (searchResult.fileNode.type === 'file') {
        handleFileSelect(searchResult.fileNode);
        console.log('Selected file:', searchResult.fileNode.name);
      }
    } else {
      console.warn('File not found in file tree:', filePath);
    }
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

  // Loading state
  if (projectLoading) {
    return (
      <div className="flex h-screen w-screen bg-black items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="w-12 h-12 border-4 border-purple-500/30 border-t-purple-500 rounded-full animate-spin" />
          <p className="text-gray-400 text-sm">Loading project...</p>
        </div>
      </div>
    );
  }

  // Error state
  if (projectError || (!projectLoading && !project)) {
    return (
      <div className="flex h-screen w-screen bg-black items-center justify-center">
        <div className="flex flex-col items-center gap-4 max-w-md text-center">
          <div className="w-16 h-16 rounded-full bg-red-500/10 flex items-center justify-center">
            <X size={32} className="text-red-500" />
          </div>
          <h2 className="text-xl font-medium text-white">Failed to Load Project</h2>
          <p className="text-gray-400 text-sm">{projectError || 'Project not found'}</p>
          <div className="flex gap-3 mt-2">
            <button
              onClick={() => navigate('/projects')}
              className="px-4 py-2 bg-[#222] text-white rounded-lg hover:bg-[#333] transition-colors"
            >
              Back to Projects
            </button>
            <button
              onClick={() => loadProjectInfo()}
              className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 transition-colors"
            >
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex h-screen w-screen bg-black overflow-hidden">
      {/* Left Side Container (Takes available space) */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* 1. Global Header for Left Section (Toggle) */}
        <div className="h-12 bg-black border-b border-[#222] flex items-center justify-center relative shrink-0">
          <div className="bg-[#111] p-1 rounded-lg border border-[#333] flex items-center gap-1">
            <button
              onClick={() => setViewMode('preview')}
              className={`flex items-center gap-2 px-4 py-1.5 rounded-md text-sm font-medium transition-all ${
                viewMode === 'preview'
                  ? 'bg-[#333] text-white shadow-sm'
                  : 'text-gray-400 hover:text-gray-200 hover:bg-[#222]'
              }`}
            >
              <Eye size={16} />
              <span>Preview</span>
            </button>
            <button
              onClick={() => setViewMode('code')}
              className={`flex items-center gap-2 px-4 py-1.5 rounded-md text-sm font-medium transition-all ${
                viewMode === 'code'
                  ? 'bg-[#333] text-white shadow-sm'
                  : 'text-gray-400 hover:text-gray-200 hover:bg-[#222]'
              }`}
            >
              <Code2 size={16} />
              <span>Code</span>
            </button>
          </div>
        </div>

        {/* 2. Workspace Content (File Tree + Editor/Preview) */}
        <div className="flex-1 flex min-h-0">
          {/* File Tree - Only in code mode */}
          <div 
            className="w-[250px] shrink-0 border-r border-[#222] bg-[#0A0A0A] flex flex-col"
            style={{ display: viewMode === 'code' ? 'flex' : 'none' }}
          >
            {loadingFiles ? (
              <div className="flex items-center justify-center h-full text-gray-500 text-sm">
                Loading files...
              </div>
            ) : (
              <FileTree 
                data={files} 
                onSelectFile={handleFileSelect}
                selectedFileId={activeFileId || undefined}
                expandedIds={expandedFolderIds}
                onToggleExpand={handleToggleExpand}
              />
            )}
          </div>

          {/* Editor/Preview Area */}
          <div className="flex-1 min-w-0 bg-black flex flex-col relative">
            {/* Code View - Always rendered but toggled via CSS */}
            <div 
              className="flex-1 flex flex-col min-w-0 absolute inset-0 z-10 bg-black"
              style={{ 
                visibility: viewMode === 'code' ? 'visible' : 'hidden',
                pointerEvents: viewMode === 'code' ? 'auto' : 'none'
              }}
            >
              {openFiles.length > 0 ? (
                <>
                  <div className="flex items-center bg-[#0A0A0A] border-b border-[#222] overflow-x-auto no-scrollbar shrink-0">
                    {openFiles.map(file => (
                      <div
                        key={file.id}
                        onClick={() => setActiveFileId(file.id)}
                        className={`flex items-center gap-2 px-3 py-2 text-sm border-r border-[#222] cursor-pointer min-w-[120px] max-w-[200px] group ${
                          activeFileId === file.id 
                            ? 'bg-black text-white border-t-2 border-t-blue-500' 
                            : 'bg-[#111] text-gray-400 hover:bg-[#111]/80'
                        }`}
                      >
                        <span className="truncate flex-1">{file.name}</span>
                        <button
                          onClick={(e) => handleCloseTab(e, file.id)}
                          className="p-0.5 rounded opacity-0 group-hover:opacity-100 hover:bg-white/20 transition-opacity"
                        >
                          <X size={12} />
                        </button>
                      </div>
                    ))}
                  </div>

                  <div className="flex-1 overflow-hidden relative">
                    {activeFile ? (
                      <CodeEditor 
                        key={activeFile.id}
                        code={activeFile.content || '// No content'} 
                        onChange={() => {}}
                        readOnly={false}
                        fileName={activeFile.name}
                        filePath={activeFile.path}
                      />
                    ) : (
                      <div className="h-full flex items-center justify-center text-gray-500">
                        Select a file to view
                      </div>
                    )}
                  </div>
                </>
              ) : (
                <div className="h-full flex flex-col items-center justify-center text-gray-500 gap-4">
                  <div className="w-16 h-16 rounded-full bg-[#2d2d2d] flex items-center justify-center">
                    <Code2 size={32} className="text-gray-600" />
                  </div>
                  <p>Select a file from the explorer to start editing</p>
                </div>
              )}
            </div>

            {/* Preview View - Always rendered but behind code view when inactive */}
            <div 
              className="flex-1 flex flex-col min-w-0 bg-black absolute inset-0"
              style={{ 
                zIndex: viewMode === 'preview' ? 20 : 0
              }}
            >
              <div className="flex-1 bg-white relative">
                <iframe 
                  src={project ? `http://${project.external_ip}:${project.frontend_port}` : 'about:blank'}
                  className="absolute inset-0 w-full h-full border-none"
                  title="Application Preview"
                  allow="accelerometer; camera; encrypted-media; geolocation; gyroscope; microphone; midi; clipboard-read; clipboard-write"
                  style={{ pointerEvents: isResizing ? 'none' : 'auto' }}
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Resize Handle */}
      <div
        ref={resizeRef}
        onMouseDown={handleResizeStart}
        className={`w-1 shrink-0 cursor-col-resize group hover:w-1.5 transition-all relative ${
          isResizing ? 'bg-emerald-500' : 'bg-[#222] hover:bg-emerald-500/70'
        }`}
      >
        {/* Grip indicator */}
        <div className={`absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 transition-opacity ${
          isResizing ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'
        }`}>
          <GripVertical size={12} className="text-emerald-300" />
        </div>
      </div>

      {/* 3. Right: Chat & Session Sidebar */}
      <div 
        ref={chatPanelDivRef}
        className="shrink-0 bg-[#0A0A0A] flex relative"
        style={{ width: chatPanelWidth }}
      >
        {/* Chat Panel */}
        <div className="flex-1 overflow-hidden flex flex-col min-w-0">
          <ChatPanel 
            messages={messages} 
            session={isPendingSession ? undefined : sessions.find(s => s.id === currentSessionId)}
            onSendMessage={handleSendMessage}
            pendingPermissions={pendingPermissions}
            onPermissionApprove={(toolCallId) => handlePermissionResponse(toolCallId, true)}
            onPermissionDeny={(toolCallId) => handlePermissionResponse(toolCallId, false)}
            sessionConfigComponent={
              isPendingSession ? (
                <InlineChatModelSelector
                  selectedConfig={pendingModelConfig}
                  onConfigChange={setPendingModelConfig}
                  disabled={isProcessing}
                />
              ) : currentSessionId ? (
                <InlineChatModelSelector
                  selectedConfig={pendingModelConfig}
                  onConfigChange={setPendingModelConfig}
                  sessionId={currentSessionId}
                  disabled={isProcessing}
                />
              ) : null
            }
            isProcessing={isProcessing}
            onCancelRequest={handleCancelRequest}
            onFileClick={handleFileClickFromTool}
          />
        </div>
        
        {/* GPT-style Sidebar (Right Side) */}
        <ChatSidebar
          sessions={sessions}
          currentSessionId={currentSessionId}
          isPendingSession={isPendingSession}
          onSelectSession={(sessionId) => {
            setCurrentSessionId(sessionId);
            setIsPendingSession(false);
          }}
          onNewSession={startNewSession}
          onDeleteSession={confirmDeleteSession}
          onNavigateToProjects={() => navigate('/projects')}
          onLogout={handleLogout}
          username={username}
          maxWidth={Math.floor(chatPanelWidth / 3)}
        />
      </div>

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