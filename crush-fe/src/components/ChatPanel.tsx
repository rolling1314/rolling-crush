import React, { useState, useRef, useEffect, useMemo } from 'react';
import { Send, User, Bot, History, X, File as FileIcon, Folder as FolderIcon, ChevronDown, ChevronRight, Sparkles, Square } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import { type Message, type PermissionRequest, type FileNode } from '../types';
import { ToolCallDisplay } from './ToolCallDisplay';
import { cn } from '../lib/utils';
import 'highlight.js/styles/github-dark.css';

interface ChatPanelProps {
  messages: Message[];
  onSendMessage: (content: string, files?: FileNode[]) => void;
  pendingPermissions: Map<string, PermissionRequest>;
  onPermissionApprove: (toolCallId: string) => void;
  onPermissionDeny: (toolCallId: string) => void;
  onToggleHistory?: () => void;
  sessionConfigComponent?: React.ReactNode;
  isProcessing?: boolean;
  onCancelRequest?: () => void;
  onFileClick?: (filePath: string) => void;
}

const ThinkingProcess = ({ reasoning, isStreaming, hasContent }: { reasoning: string, isStreaming: boolean, hasContent: boolean }) => {
  const [isOpen, setIsOpen] = useState(false);
  
  // We consider it "actively thinking" if the message is streaming AND there is no content yet.
  // Once content starts arriving, the thinking phase is effectively over from a UX perspective.
  const isThinking = isStreaming && !hasContent;
  
  return (
    <div className="border-b border-white/10 mb-2 pb-2">
        <button 
            onClick={() => setIsOpen(!isOpen)}
            className={cn(
                "group flex items-center gap-3 px-2 py-1 rounded transition-all w-full text-left select-none",
                isThinking 
                    ? "cursor-wait" 
                    : "hover:bg-white/5"
            )}
        >
            {/* Breathing light effect / Icon */}
            <div className="relative flex items-center justify-center w-4 h-4">
                {isThinking ? (
                    <>
                        <div className="absolute inset-0 bg-purple-500 rounded-full animate-ping opacity-20 duration-1000" />
                        <div className="w-2 h-2 bg-purple-400 rounded-full shadow-[0_0_10px_rgba(168,85,247,0.8)] animate-pulse" />
                    </>
                ) : (
                    <Sparkles size={12} className="text-purple-400/70" />
                )}
            </div>
            
            <div className="flex flex-col">
                <span className={cn(
                    "text-xs font-medium transition-colors",
                    isThinking ? "text-purple-300" : "text-gray-400 group-hover:text-purple-300"
                )}>
                    {isThinking ? "Thinking..." : "Thinking Process"}
                </span>
            </div>
            
            <div className="ml-auto text-gray-500 group-hover:text-gray-300 transition-colors">
                {isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            </div>
        </button>
        
        {isOpen && (
            <div className="mt-2 ml-1 pl-3 border-l-2 border-purple-500/20 overflow-hidden animate-in fade-in slide-in-from-top-1">
               <div className="text-xs font-mono text-gray-400 whitespace-pre-wrap leading-relaxed opacity-90 py-1">
                 {reasoning}
               </div>
            </div>
        )}
    </div>
  );
};

const UserMessageRenderer = ({ content }: { content: string }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  
  const { text, files } = useMemo(() => {
    const contextMarker = '\n\nContext Files:\n';
    const splitIndex = content.lastIndexOf(contextMarker);
    
    if (splitIndex === -1) {
      return { text: content, files: [] };
    }
    
    const text = content.substring(0, splitIndex);
    const fileContext = content.substring(splitIndex + contextMarker.length);
    const files: { type: 'file' | 'folder', path: string }[] = [];
    
    const entries = fileContext.split('\n\n');
    entries.forEach(entry => {
        if (entry.startsWith('Folder: ')) {
            const firstLine = entry.split('\n')[0];
            const path = firstLine.substring(8).replace(' (Context of this folder)', '');
            files.push({ type: 'folder', path });
        } else if (entry.startsWith('File: ')) {
            const firstLine = entry.split('\n')[0];
            const path = firstLine.substring(6);
            files.push({ type: 'file', path });
        }
    });
    
    return { text, files };
  }, [content]);

  if (files.length === 0) {
    return <span className="whitespace-pre-wrap">{text}</span>;
  }

  return (
    <div className="flex flex-col gap-2">
      <span className="whitespace-pre-wrap">{text}</span>
      
      <div className="bg-blue-900/20 border border-blue-700/30 rounded-md overflow-hidden">
        <button 
          onClick={() => setIsExpanded(!isExpanded)}
          className="flex items-center gap-2 w-full px-3 py-2 text-xs text-blue-300 hover:bg-blue-800/30 transition-colors text-left"
        >
          {isExpanded ? <ChevronDown size={14} className="shrink-0" /> : <ChevronRight size={14} className="shrink-0" />}
          <span className="font-medium shrink-0">Context:</span>
          {!isExpanded ? (
             <div className="flex gap-2 overflow-hidden items-center flex-1">
                {files.slice(0, 3).map((f, i) => (
                    <span key={i} className="flex items-center gap-1 bg-blue-900/40 px-1.5 py-0.5 rounded border border-blue-700/30 truncate max-w-[150px]">
                        {f.type === 'folder' ? <FolderIcon size={10} /> : <FileIcon size={10} />}
                        <span className="truncate">{f.path.split('/').pop()}</span>
                    </span>
                ))}
                {files.length > 3 && <span className="shrink-0 opacity-70">+{files.length - 3} more</span>}
             </div>
          ) : (
             <span>{files.length} items included</span>
          )}
        </button>
        
        {isExpanded && (
          <div className="px-3 py-2 bg-black/20 text-xs text-blue-200/80 space-y-1 border-t border-blue-700/30 max-h-[200px] overflow-y-auto">
            {files.map((file, idx) => (
              <div key={idx} className="flex items-center gap-2 font-mono">
                {file.type === 'folder' ? <FolderIcon size={12} className="shrink-0" /> : <FileIcon size={12} className="shrink-0" />}
                <span className="truncate" title={file.path}>{file.path}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export const ChatPanel = ({ 
  messages, 
  onSendMessage,
  pendingPermissions,
  onPermissionApprove,
  onPermissionDeny,
  onToggleHistory,
  sessionConfigComponent,
  isProcessing = false,
  onCancelRequest,
  onFileClick
}: ChatPanelProps) => {
  const [input, setInput] = useState('');
  const [attachedFiles, setAttachedFiles] = useState<FileNode[]>([]);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputContainerRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() && attachedFiles.length === 0) return;
    onSendMessage(input, attachedFiles);
    setInput('');
    setAttachedFiles([]);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit(e as any);
    }
  };

  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (inputContainerRef.current) {
        inputContainerRef.current.style.borderColor = '#3b82f6'; // blue-500
    }
  };

  const handleDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (inputContainerRef.current) {
        inputContainerRef.current.style.borderColor = '#4b5563'; // gray-600
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    if (inputContainerRef.current) {
        inputContainerRef.current.style.borderColor = '#4b5563'; // gray-600
    }

    try {
      const data = e.dataTransfer.getData('application/json');
      if (data) {
        const fileNode = JSON.parse(data) as FileNode;
        // Allow both files and folders
        setAttachedFiles(prev => {
            if (prev.some(f => f.id === fileNode.id)) return prev;
            return [...prev, fileNode];
        });
      }
    } catch (err) {
      console.error('Failed to parse dropped data', err);
    }
  };

  const removeAttachedFile = (fileId: string) => {
    setAttachedFiles(prev => prev.filter(f => f.id !== fileId));
  };

  return (
    <div className="flex flex-col h-full bg-[#1e1e1e]">
      <div className="p-4 border-b border-gray-700 bg-[#252526] flex justify-between items-center">
        <h2 className="text-sm font-semibold text-gray-200">AI Assistant</h2>
        {onToggleHistory && (
          <button 
            onClick={onToggleHistory}
            className="p-1 hover:bg-gray-700 rounded text-gray-400 hover:text-white transition-colors"
            title="Session History"
          >
            <History size={18} />
          </button>
        )}
      </div>
      
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages
          // 过滤空消息（没有内容、没有思考过程、没有有效工具调用的消息）
          .filter((msg) => {
            const hasContent = msg.content && msg.content.trim();
            const hasReasoning = msg.reasoning && msg.reasoning.trim();
            const hasToolCalls = msg.toolCalls && msg.toolCalls.some(tc => tc && tc.id && tc.name);
            const isStreaming = msg.isStreaming;
            // 保留有任何内容的消息，或正在流式传输的消息
            return hasContent || hasReasoning || hasToolCalls || isStreaming;
          })
          .map((msg) => (
          <div
            key={msg.id}
            className={cn(
              "flex gap-3 max-w-[90%] message-container streaming-message",
              msg.role === 'user' ? "ml-auto flex-row-reverse" : "mr-auto"
            )}
          >
            <div className={cn(
              "w-8 h-8 rounded-full flex items-center justify-center shrink-0",
              msg.role === 'user' ? "bg-blue-600" : "bg-green-600"
            )}>
              {msg.role === 'user' ? <User size={16} /> : <Bot size={16} />}
            </div>
            <div className={cn(
              "flex flex-col gap-2 flex-1 min-w-0 p-3 rounded-lg text-sm leading-relaxed",
              msg.role === 'user' 
                ? "bg-blue-600/10 text-blue-100 border border-blue-600/20" 
                : "bg-gray-700/50 text-gray-200 border border-gray-600/30"
            )}>
              {/* Reasoning content (Collapsible) */}
              {msg.reasoning && (
                <ThinkingProcess 
                    reasoning={msg.reasoning}
                    isStreaming={!!msg.isStreaming}
                    hasContent={!!msg.content}
                />
              )}
              
              {/* Tool Calls - filter out invalid ones */}
              {msg.toolCalls && msg.toolCalls.length > 0 && (
                <div className="space-y-2">
                  {msg.toolCalls
                    .filter(tc => tc && tc.id && tc.name) // Only render valid tool calls
                    .map((toolCall) => {
                      const result = msg.toolResults?.find(r => r.tool_call_id === toolCall.id);
                      const needsPermission = pendingPermissions.has(toolCall.id);
                      console.log('=== ChatPanel: Rendering ToolCall ===');
                      console.log('Tool Call ID:', toolCall.id);
                      console.log('Tool Call Name:', toolCall.name);
                      console.log('Pending permissions Map keys:', Array.from(pendingPermissions.keys()));
                      console.log('needsPermission:', needsPermission);
                      console.log('Has result:', !!result);
                      return (
                        <ToolCallDisplay
                          key={toolCall.id}
                          toolCall={toolCall}
                          result={result}
                          needsPermission={needsPermission}
                          onApprove={onPermissionApprove}
                          onDeny={onPermissionDeny}
                          onFileClick={onFileClick}
                        />
                      );
                    })}
                </div>
              )}
              
              {/* Main content */}
              {msg.content && (
                <div className={cn(
                  "prose prose-invert prose-sm max-w-none streaming-content",
                  msg.isStreaming && "streaming-cursor"
                )}>
                  {msg.role === 'assistant' || msg.role === 'tool' ? (
                    <ReactMarkdown
                      remarkPlugins={[remarkGfm]}
                      rehypePlugins={[rehypeHighlight]}
                      components={{
                        code: ({node, inline, className, children, ...props}: any) => {
                          return inline ? (
                            <code className="bg-gray-800 px-1.5 py-0.5 rounded text-xs font-mono text-green-400" {...props}>
                              {children}
                            </code>
                          ) : (
                            <code className={cn("block bg-gray-900 p-3 rounded-md overflow-x-auto text-xs", className)} {...props}>
                              {children}
                            </code>
                          );
                        },
                        pre: ({children, ...props}: any) => (
                          <pre className="bg-gray-900 rounded-md overflow-x-auto my-2" {...props}>
                            {children}
                          </pre>
                        ),
                        p: ({children, ...props}: any) => (
                          <p className="mb-2 last:mb-0 leading-relaxed" {...props}>
                            {children}
                          </p>
                        ),
                        ul: ({children, ...props}: any) => (
                          <ul className="list-disc list-inside mb-2 space-y-1" {...props}>
                            {children}
                          </ul>
                        ),
                        ol: ({children, ...props}: any) => (
                          <ol className="list-decimal list-inside mb-2 space-y-1" {...props}>
                            {children}
                          </ol>
                        ),
                        li: ({children, ...props}: any) => (
                          <li className="ml-2" {...props}>
                            {children}
                          </li>
                        ),
                        h1: ({children, ...props}: any) => (
                          <h1 className="text-lg font-bold mb-2 mt-3 first:mt-0" {...props}>
                            {children}
                          </h1>
                        ),
                        h2: ({children, ...props}: any) => (
                          <h2 className="text-base font-bold mb-2 mt-3 first:mt-0" {...props}>
                            {children}
                          </h2>
                        ),
                        h3: ({children, ...props}: any) => (
                          <h3 className="text-sm font-bold mb-2 mt-2 first:mt-0" {...props}>
                            {children}
                          </h3>
                        ),
                        blockquote: ({children, ...props}: any) => (
                          <blockquote className="border-l-4 border-gray-600 pl-3 italic my-2" {...props}>
                            {children}
                          </blockquote>
                        ),
                      }}
                    >
                      {msg.content}
                    </ReactMarkdown>
                  ) : (
                    <UserMessageRenderer content={msg.content} />
                  )}
                  {msg.isStreaming && (
                    <span className="inline-block w-1.5 h-4 ml-1 bg-green-500 animate-pulse align-middle"></span>
                  )}
                </div>
              )}
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      <div className="p-4 bg-[#252526]">
        <div 
            ref={inputContainerRef}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            className="relative bg-[#3c3c3c] border border-gray-600 rounded-lg flex flex-col focus-within:ring-1 focus-within:ring-blue-500 focus-within:border-blue-500 transition-colors"
        >
          {attachedFiles.length > 0 && (
            <div className="flex flex-wrap gap-2 p-2 border-b border-gray-600/50">
                {attachedFiles.map(file => (
                    <div key={file.id} className="flex items-center gap-1.5 px-2 py-1 bg-[#1e1e1e]/50 rounded text-xs text-blue-300 border border-blue-500/30">
                        {file.type === 'folder' ? <FolderIcon size={12} /> : <FileIcon size={12} />}
                        <span className="truncate max-w-[150px]">{file.name}</span>
                        <button 
                            onClick={() => removeAttachedFile(file.id)}
                            className="hover:text-white ml-0.5"
                        >
                            <X size={12} />
                        </button>
                    </div>
                ))}
            </div>
          )}
          
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={attachedFiles.length > 0 ? "Describe what to do with these files..." : "问问关于代码的问题......"}
            className="w-full bg-transparent border-none text-sm text-gray-200 placeholder-gray-500 p-3 min-h-[60px] max-h-[200px] resize-none focus:ring-0 focus:outline-none scrollbar-thin scrollbar-thumb-gray-600 scrollbar-track-transparent"
            rows={2}
          />
          
          <div className="flex justify-between items-center px-2 pb-2">
            <div className="flex items-center">
               {sessionConfigComponent}
            </div>
            
            {isProcessing ? (
              <button
                onClick={onCancelRequest}
                className="relative p-1.5 bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors group"
                title="取消请求"
              >
                {/* 呼吸灯效果 */}
                <span className="absolute inset-0 rounded-md bg-red-500 animate-ping opacity-30" />
                <span className="absolute inset-0 rounded-md bg-red-400 animate-pulse opacity-40" />
                <Square size={16} className="relative z-10 fill-current" />
              </button>
            ) : (
              <button
                onClick={handleSubmit}
                disabled={!input.trim() && attachedFiles.length === 0}
                className="p-1.5 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Send size={16} />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
