import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import { ChatPanel } from './components/ChatPanel';
import { FileTree } from './components/FileTree';
import { CodeEditor } from './components/CodeEditor';
import { type FileNode, type Message } from './types';
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
      // ... message handling ...
      try {
        const data = JSON.parse(event.data);
        console.log('WS Message:', data);

        // Convert backend message format to frontend Message type
        let content = "";
        if (data.Parts && Array.isArray(data.Parts)) {
            content = data.Parts.map((p: any) => {
                // Adjust parsing based on actual backend JSON structure
                if (p.type === 'text' && p.data && p.data.text) return p.data.text;
                if (p.type === 'reasoning' && p.data && p.data.text) return `[Reasoning: ${p.data.text}]`;
                return "";
            }).join("");
        }
        
        // Handle "content" string field if parts are complex or not used in simple broadcast
        if (!content && typeof data.Content === 'string') {
             // Sometimes the broadcast might use a simpler format or String() representation
             // Check if data is already the message object we expect
        }

        const newMessage: Message = {
            id: data.ID || Date.now().toString(),
            role: data.Role === 'assistant' ? 'assistant' : 'user',
            content: content || "...", // Placeholder if empty
            timestamp: Date.now()
        };

        setMessages(prev => {
            const index = prev.findIndex(m => m.id === newMessage.id);
            if (index !== -1) {
                // Update existing message (streaming update)
                const newMessages = [...prev];
                // Accumulate content if needed, but usually the backend sends the full state or delta.
                // If it sends full message state update:
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

  return (
    <div className="flex h-screen w-screen bg-[#1e1e1e] text-white overflow-hidden">
      {/* Left Sidebar: Chat */}
      <div className="w-[400px] shrink-0 h-full border-r border-gray-700">
        <ChatPanel 
          messages={messages} 
          onSendMessage={handleSendMessage} 
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
