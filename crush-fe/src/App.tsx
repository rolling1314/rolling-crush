import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { ChatPanel } from './components/ChatPanel';
import { FileTree } from './components/FileTree';
import { CodeEditor } from './components/CodeEditor';
import { type FileNode, type Message, type ToolCall, type ToolResult, type PermissionRequest } from './types';
import { cn } from './lib/utils';

const INITIAL_MESSAGES: Message[] = [
  {
    id: '1',
    role: 'assistant',
    content: 'Hello! I can help you understand the server code. What would you like to know?',
    timestamp: Date.now()
  }
];

function App() {
  const [messages, setMessages] = useState<Message[]>(INITIAL_MESSAGES);
  const [openFiles, setOpenFiles] = useState<FileNode[]>([]);
  const [activeFileId, setActiveFileId] = useState<string | null>(null);
  const [files, setFiles] = useState<FileNode[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  const [wsConnection, setWsConnection] = useState<WebSocket | null>(null);
  const [pendingPermissions, setPendingPermissions] = useState<Map<string, PermissionRequest>>(new Map());

  // Derive the active file object from the ID and openFiles list
  const activeFile = openFiles.find(f => f.id === activeFileId) || null;

  useEffect(() => {
    const fetchFiles = async () => {
      try {
        const response = await fetch('/api/files');
        if (!response.ok) {
          throw new Error('Failed to fetch files');
        }
        const data = await response.json();
        console.log('Fetched files:', data); // Debug log
        if (Array.isArray(data)) {
          setFiles(data);
        } else if (data && typeof data === 'object') {
          // If a single root object is returned, wrap it in an array
          setFiles([data]);
        } else {
          console.error('API returned unexpected data format:', data);
          setFiles([]);
        }
      } catch (error) {
        console.error('Error fetching files:', error);
        // Fallback or empty state could be handled here
      } finally {
        setIsLoading(false);
      }
    };

    fetchFiles();

    // WebSocket Connection
    const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${wsProtocol}//${window.location.host}/ws`);

    ws.onopen = () => {
      console.log('Connected to WebSocket');
      setWsConnection(ws);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        console.log('WS Message:', data);

        // Handle permission requests
        if (data.Type === 'permission_request' || data.tool_call_id) {
          const permissionReq: PermissionRequest = {
            id: data.id || data.ID,
            session_id: data.session_id || data.SessionID,
            tool_call_id: data.tool_call_id,
            tool_name: data.tool_name,
            action: data.action
          };
          setPendingPermissions(prev => new Map(prev).set(permissionReq.tool_call_id, permissionReq));
          console.log('Permission request received:', permissionReq);
          return;
        }

        // Parse backend message format
        let textContent = "";
        let reasoningContent = "";
        const toolCalls: ToolCall[] = [];
        const toolResults: ToolResult[] = [];
        
        if (data.Parts && Array.isArray(data.Parts)) {
          data.Parts.forEach((part: any) => {
            // Handle direct text field
            if (part.text) {
              textContent += part.text;
            }
            // Handle thinking field (reasoning)
            if (part.thinking) {
              reasoningContent += part.thinking;
            }
            // Handle tool calls
            if (part.type === 'tool_call' || (part.id && part.name && part.input !== undefined)) {
              const toolCall: ToolCall = {
                id: part.id || part.data?.id,
                name: part.name || part.data?.name,
                input: part.input || part.data?.input || '',
                finished: part.finished ?? part.data?.finished ?? false,
                provider_executed: part.provider_executed ?? part.data?.provider_executed
              };
              toolCalls.push(toolCall);
            }
            // Handle tool results
            if (part.type === 'tool_result' || part.tool_call_id) {
              const toolResult: ToolResult = {
                tool_call_id: part.tool_call_id || part.data?.tool_call_id,
                name: part.name || part.data?.name,
                content: part.content || part.data?.content || '',
                data: part.data?.data,
                mime_type: part.mime_type || part.data?.mime_type,
                metadata: part.metadata || part.data?.metadata,
                is_error: part.is_error ?? part.data?.is_error ?? false
              };
              toolResults.push(toolResult);
            }
            // Handle nested data structure (fallback)
            if (part.type === 'text' && part.data?.text) {
              textContent += part.data.text;
            }
            if (part.type === 'reasoning' && part.data?.thinking) {
              reasoningContent += part.data.thinking;
            }
          });
        }

        // Skip if message has no content and no tool calls
        if (!textContent && !reasoningContent && toolCalls.length === 0 && toolResults.length === 0) {
          console.log('Skipping empty message');
          return;
        }

        const newMessage: Message = {
          id: data.ID || Date.now().toString(),
          role: data.Role === 'assistant' ? 'assistant' : (data.Role === 'tool' ? 'tool' : 'user'),
          content: textContent || reasoningContent || (toolCalls.length > 0 ? `Executing ${toolCalls.length} tool(s)...` : "..."),
          reasoning: reasoningContent || undefined,
          timestamp: data.UpdatedAt || data.CreatedAt || Date.now(),
          isStreaming: data.Role === 'assistant' && (!textContent || textContent.length < 10),
          toolCalls: toolCalls.length > 0 ? toolCalls : undefined,
          toolResults: toolResults.length > 0 ? toolResults : undefined
        };

        console.log('Parsed message:', newMessage);

        setMessages(prev => {
          const index = prev.findIndex(m => m.id === newMessage.id);
          if (index !== -1) {
            // Update existing message (streaming update)
            const newMessages = [...prev];
            newMessages[index] = newMessage;
            return newMessages;
          } else {
            // Append new message
            return [...prev, newMessage];
          }
        });

      } catch (e) {
        console.error('Error parsing WS message:', e);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => {
      ws.close();
    };
  }, []);

  const handleSendMessage = (content: string) => {
    const newMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content,
      timestamp: Date.now()
    };
    setMessages(prev => [...prev, newMessage]);

    if (wsConnection && wsConnection.readyState === WebSocket.OPEN) {
        wsConnection.send(JSON.stringify({ content }));
    } else {
        console.error("WebSocket is not connected");
    }
  };

  const handleFileSelect = (file: FileNode) => {
    // Check if file is already open
    if (!openFiles.some(f => f.id === file.id)) {
      setOpenFiles(prev => [...prev, file]);
    }
    setActiveFileId(file.id);
  };

  const handleCloseTab = (e: React.MouseEvent, fileId: string) => {
    e.stopPropagation();
    setOpenFiles(prev => {
      const newFiles = prev.filter(f => f.id !== fileId);
      // If we closed the active file, switch to the last opened file or null
      if (fileId === activeFileId) {
        const lastFile = newFiles[newFiles.length - 1];
        setActiveFileId(lastFile ? lastFile.id : null);
      }
      return newFiles;
    });
  };

  const handleCodeChange = (newContent: string) => {
    if (activeFileId) {
      setOpenFiles(prev => prev.map(f => 
        f.id === activeFileId ? { ...f, content: newContent } : f
      ));
    }
  };

  const handlePermissionApprove = (toolCallId: string) => {
    const permission = pendingPermissions.get(toolCallId);
    if (!permission) return;

    if (wsConnection && wsConnection.readyState === WebSocket.OPEN) {
      wsConnection.send(JSON.stringify({
        type: 'permission_response',
        id: permission.id,
        tool_call_id: toolCallId,
        granted: true
      }));
      setPendingPermissions(prev => {
        const newMap = new Map(prev);
        newMap.delete(toolCallId);
        return newMap;
      });
    }
  };

  const handlePermissionDeny = (toolCallId: string) => {
    const permission = pendingPermissions.get(toolCallId);
    if (!permission) return;

    if (wsConnection && wsConnection.readyState === WebSocket.OPEN) {
      wsConnection.send(JSON.stringify({
        type: 'permission_response',
        id: permission.id,
        tool_call_id: toolCallId,
        granted: false,
        denied: true
      }));
      setPendingPermissions(prev => {
        const newMap = new Map(prev);
        newMap.delete(toolCallId);
        return newMap;
      });
    }
  };

  return (
    <div className="flex h-screen w-screen bg-[#1e1e1e] text-white overflow-hidden">
      {/* Left Sidebar: Chat */}
      <div className="w-[400px] shrink-0 h-full border-r border-gray-700">
        <ChatPanel 
          messages={messages} 
          onSendMessage={handleSendMessage}
          pendingPermissions={pendingPermissions}
          onPermissionApprove={handlePermissionApprove}
          onPermissionDeny={handlePermissionDeny}
        />
      </div>

      {/* Right Area: File Tree + Code */}
      <div className="flex-1 flex h-full min-w-0">
        {/* File Explorer */}
        <div className="w-[250px] shrink-0 border-r border-gray-700 bg-[#1e1e1e]">
          {isLoading ? (
            <div className="flex items-center justify-center h-full text-gray-500 text-sm">
              Loading...
            </div>
          ) : (
            <FileTree 
              data={files} 
              onSelectFile={handleFileSelect}
              selectedFileId={activeFileId || undefined}
            />
          )}
        </div>

        {/* Code Editor Area */}
        <div className="flex-1 min-w-0 bg-[#1e1e1e] flex flex-col">
          {openFiles.length > 0 ? (
            <>
              {/* Tabs Header */}
              <div className="flex items-center bg-[#252526] border-b border-gray-700 overflow-x-auto no-scrollbar">
                {openFiles.map(file => (
                  <div
                    key={file.id}
                    onClick={() => setActiveFileId(file.id)}
                    className={cn(
                      "flex items-center gap-2 px-3 py-2 text-sm border-r border-gray-700 min-w-[120px] max-w-[200px] cursor-pointer select-none group",
                      activeFileId === file.id 
                        ? "bg-[#1e1e1e] text-white border-t-2 border-t-blue-500" 
                        : "bg-[#2d2d2d] text-gray-400 hover:bg-[#2d2d2d]/80 border-t-2 border-t-transparent"
                    )}
                  >
                    <span className="truncate flex-1">{file.name}</span>
                    <button
                      onClick={(e) => handleCloseTab(e, file.id)}
                      className={cn(
                        "p-0.5 rounded-md hover:bg-white/20 opacity-0 group-hover:opacity-100 transition-opacity",
                        activeFileId === file.id && "opacity-100"
                      )}
                    >
                      <X size={12} />
                    </button>
                  </div>
                ))}
              </div>

              {/* Editor Content */}
              <div className="flex-1 overflow-hidden relative">
                {activeFile ? (
                  <CodeEditor 
                    // Force re-mount when switching files to ensure fresh state if needed, 
                    // though CodeMirror usually handles updates well. 
                    // Using key={activeFile.id} ensures clean state switch.
                    key={activeFile.id} 
                    code={activeFile.content || '// No content available'} 
                    onChange={handleCodeChange}
                    readOnly={false}
                  />
                ) : (
                   <div className="h-full flex items-center justify-center text-gray-500">
                    Select a file to view
                  </div>
                )}
              </div>
            </>
          ) : (
            <div className="h-full flex items-center justify-center text-gray-500">
              Select a file to view content
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default App;
