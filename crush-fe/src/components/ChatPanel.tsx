import React, { useState, useRef, useEffect, useMemo, memo, useCallback } from 'react';
import { Send, X, File as FileIcon, Folder as FolderIcon, ChevronDown, ChevronRight, Sparkles, Square, Copy, Check, ImagePlus, Loader2 } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import { type Message, type PermissionRequest, type FileNode, type ImageAttachment, type ImageUploadResponse, type Session } from '../types';
import { ToolCallDisplay } from './ToolCallDisplay';
import { cn } from '../lib/utils';
import 'highlight.js/styles/github-dark.css';

const API_URL = '/api';

interface ChatPanelProps {
  messages: Message[];
  session?: Session;
  onSendMessage: (content: string, files?: FileNode[], images?: ImageAttachment[]) => void;
  pendingPermissions: Map<string, PermissionRequest>;
  onPermissionApprove: (toolCallId: string) => void;
  onPermissionDeny: (toolCallId: string) => void;
  onPermissionAllowForSession?: (toolCallId: string, toolName: string, action?: string) => void;
  sessionConfigComponent?: React.ReactNode;
  isProcessing?: boolean;
  onCancelRequest?: () => void;
  onFileClick?: (filePath: string) => void;
}

const ThinkingProcess = memo(({ reasoning, isStreaming, hasContent }: { reasoning: string, isStreaming: boolean, hasContent: boolean }) => {
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
});

// Component to display images in a message
const MessageImages = memo(({ images }: { images: ImageAttachment[] }) => {
  const [selectedImage, setSelectedImage] = useState<string | null>(null);
  
  if (!images || images.length === 0) return null;
  
  return (
    <>
      <div className="flex flex-wrap gap-2 mb-2">
        {images.map((img, idx) => (
          <div key={idx} className="relative group cursor-pointer" onClick={() => setSelectedImage(img.url)}>
            <img 
              src={img.url} 
              alt={img.filename}
              className="max-h-48 max-w-[240px] object-cover rounded-lg border border-white/10 hover:border-white/30 transition-colors shadow-sm"
            />
          </div>
        ))}
      </div>
      
      {/* Full-size image modal */}
      {selectedImage && (
        <div 
          className="fixed inset-0 bg-black/90 z-[100] flex items-center justify-center p-4 backdrop-blur-sm"
          onClick={() => setSelectedImage(null)}
        >
          <div className="relative max-w-full max-h-full">
            <img 
              src={selectedImage} 
              alt="Full size"
              className="max-w-full max-h-[90vh] object-contain rounded-lg shadow-2xl"
            />
            <button
              onClick={() => setSelectedImage(null)}
              className="absolute -top-12 right-0 p-2 text-white/70 hover:text-white transition-colors"
            >
              <X size={24} />
            </button>
          </div>
        </div>
      )}
    </>
  );
});

const UserMessageRenderer = memo(({ content, images }: { content: string; images?: ImageAttachment[] }) => {
  const [isExpanded, setIsExpanded] = useState(false);
  
  const { text, files } = useMemo(() => {
    // Determine the start of the file context section
    const fullMarker = '\n\nContext Files:\n';
    const startMarker = 'Context Files:\n';
    
    let splitIndex = -1;
    let fileSectionStart = -1;

    // First try finding the marker preceded by newlines (standard case)
    const idxFull = content.lastIndexOf(fullMarker);
    if (idxFull !== -1) {
        splitIndex = idxFull;
        fileSectionStart = idxFull + fullMarker.length;
    } else if (content.startsWith(startMarker)) {
        // Fallback: starts directly with marker (empty user text)
        splitIndex = 0;
        fileSectionStart = startMarker.length;
    }
    
    if (splitIndex === -1) {
      return { text: content, files: [] };
    }
    
    const text = content.substring(0, splitIndex);
    const fileContext = content.substring(fileSectionStart);
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

  return (
    <div className="flex flex-col gap-1">
      {/* Display attached images at the top */}
      {images && images.length > 0 && (
        <MessageImages images={images} />
      )}

      {files.length === 0 && <span className="whitespace-pre-wrap">{text}</span>}

      {files.length > 0 && (
        <>
            <span className="whitespace-pre-wrap">{text}</span>
            <div className="bg-blue-900/20 border border-blue-700/30 rounded-md overflow-hidden mt-2">

        <button 
          onClick={() => setIsExpanded(!isExpanded)}
          className="flex items-center gap-2 w-full px-3 py-2 text-xs text-blue-300 hover:bg-blue-800/30 transition-colors text-left"
        >
          {isExpanded ? <ChevronDown size={14} className="shrink-0" /> : <ChevronRight size={14} className="shrink-0" />}
          <span className="font-medium shrink-0">Context:</span>
          {!isExpanded ? (
             <div className="flex gap-2 overflow-hidden items-center flex-1">
                {files.slice(0, 3).map((f, i) => {
                    const fullPath = f.path;
                    // Extract filename and potential line info "(12-34)"
                    // path format from workspace: "/path/to/file.go (10-20)"
                    // or just "/path/to/file.go"
                    const fileNameWithLines = fullPath.split('/').pop() || '';
                    
                    // Simple regex to split filename and line info
                    // Matches "filename.ext" or "filename.ext (10-20)"
                    const match = fileNameWithLines.match(/^(.*?)(\s\(\d+-\d+\))?$/);
                    const fileName = match ? match[1] : fileNameWithLines;
                    const lineInfo = match ? match[2] : '';

                    return (
                        <span key={i} className="flex items-center gap-1 bg-blue-900/40 px-1.5 py-0.5 rounded border border-blue-700/30 truncate max-w-[200px]">
                            {f.type === 'folder' ? <FolderIcon size={10} /> : <FileIcon size={10} />}
                            <span className="truncate">
                                {fileName}
                                {lineInfo && <span className="text-gray-400 ml-0.5 text-[10px]">{lineInfo}</span>}
                            </span>
                        </span>
                    );
                })}
                {files.length > 3 && <span className="shrink-0 opacity-70">+{files.length - 3} more</span>}
             </div>
          ) : (
             <span>{files.length} items included</span>
          )}
        </button>
        
        {isExpanded && (
          <div className="px-3 py-2 bg-black/20 text-xs text-blue-200/80 space-y-1 border-t border-blue-700/30 max-h-[200px] overflow-y-auto">
            {files.map((file, idx) => {
                const fullPath = file.path;
                const fileNameWithLines = fullPath.split('/').pop() || '';
                const match = fileNameWithLines.match(/^(.*?)(\s\(\d+-\d+\))?$/);
                const fileName = match ? match[1] : fileNameWithLines;
                const lineInfo = match ? match[2] : '';

                return (
                  <div key={idx} className="flex items-center gap-2 font-mono">
                    {file.type === 'folder' ? <FolderIcon size={12} className="shrink-0" /> : <FileIcon size={12} className="shrink-0" />}
                    <span className="truncate" title={file.path}>
                        {file.path.replace(fileNameWithLines, '')}{fileName}
                        {lineInfo && <span className="text-gray-500">{lineInfo}</span>}
                    </span>
                  </div>
                );
            })}
          </div>
        )}
      </div>
        </>
      )}
    </div>
  );
});

// Memoized message group component to avoid re-rendering unchanged messages
const MessageGroup = memo(({ 
  group, 
  allToolResults, 
  pendingPermissions,
  onPermissionApprove,
  onPermissionDeny,
  onPermissionAllowForSession,
  onFileClick
}: {
  group: Message[];
  allToolResults: Map<string, any>;
  pendingPermissions: Map<string, PermissionRequest>;
  onPermissionApprove: (toolCallId: string) => void;
  onPermissionDeny: (toolCallId: string) => void;
  onPermissionAllowForSession?: (toolCallId: string, toolName: string, action?: string) => void;
  onFileClick?: (filePath: string) => void;
}) => {
  const firstMsg = group[0];
  const isUser = firstMsg.role === 'user';
  
  return (
    <div
      className={cn(
        "flex gap-3 message-container streaming-message",
        isUser ? "ml-auto w-fit max-w-[80%] justify-end" : "w-full"
      )}
    >
      <div className={cn(
        "flex flex-col gap-2 flex-1 min-w-0 p-3 rounded-lg text-sm leading-relaxed",
        isUser 
          ? "bg-blue-600/10 text-blue-100 border border-blue-600/20" 
          : "text-gray-200 px-0"
      )}>
        {group.map((msg, index) => (
          <React.Fragment key={msg.id}>
            {index > 0 && <div className="w-full h-px bg-[#333] my-2" />}
            
            <div className="flex flex-col gap-2">
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
                      // Look up result from all messages, not just current message
                      const result = allToolResults.get(toolCall.id) || msg.toolResults?.find(r => r.tool_call_id === toolCall.id);
                      const permRequest = pendingPermissions.get(toolCall.id);
                      const needsPermission = !!permRequest;
                      return (
                        <ToolCallDisplay
                          key={toolCall.id}
                          toolCall={toolCall}
                          result={result}
                          needsPermission={needsPermission}
                          permissionRequest={permRequest ? {
                            tool_name: permRequest.tool_name,
                            action: permRequest.action,
                          } : undefined}
                          onApprove={onPermissionApprove}
                          onDeny={onPermissionDeny}
                          onAllowForSession={onPermissionAllowForSession}
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
                          if (inline) {
                            return (
                              <code className="bg-gray-800 px-1.5 py-0.5 rounded text-xs font-mono text-green-400" {...props}>
                                {children}
                              </code>
                            );
                          }
                          
                          const [copied, setCopied] = useState(false);
                          const handleCopy = () => {
                            const text = String(children).replace(/\n$/, '');
                            navigator.clipboard.writeText(text);
                            setCopied(true);
                            setTimeout(() => setCopied(false), 2000);
                          };

                          return (
                            <div className="relative group my-2">
                              <div className="absolute right-2 top-2 opacity-0 group-hover:opacity-100 transition-opacity z-10">
                                <button
                                  onClick={handleCopy}
                                  className="p-1.5 bg-gray-700/80 hover:bg-gray-600 rounded text-gray-300 hover:text-white transition-colors backdrop-blur-sm"
                                  title="Copy code"
                                >
                                  {copied ? <Check size={14} className="text-green-400" /> : <Copy size={14} />}
                                </button>
                              </div>
                              <code className={cn("block bg-gray-900 p-3 rounded-md overflow-x-auto text-xs", className)} {...props}>
                                {children}
                              </code>
                            </div>
                          );
                        },
                        pre: ({children, ...props}: any) => (
                          <pre className="bg-transparent p-0 m-0" {...props}>
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
                    <UserMessageRenderer content={msg.content} images={msg.images} />
                  )}
                  {msg.isStreaming && (
                    <span className="inline-block w-1.5 h-4 ml-1 bg-green-500 animate-pulse align-middle"></span>
                  )}
                </div>
              )}
            </div>
          </React.Fragment>
        ))}
      </div>
    </div>
  );
}, (prevProps, nextProps) => {
  // Only re-render if the group's messages actually changed
  const prevGroup = prevProps.group;
  const nextGroup = nextProps.group;
  
  // Check if group length or message IDs changed
  if (prevGroup.length !== nextGroup.length) return false;
  
  // Check if any message in the group changed
  for (let i = 0; i < prevGroup.length; i++) {
    const prevMsg = prevGroup[i];
    const nextMsg = nextGroup[i];
    
    if (
      prevMsg.id !== nextMsg.id ||
      prevMsg.content !== nextMsg.content ||
      prevMsg.isStreaming !== nextMsg.isStreaming ||
      prevMsg.toolCalls?.length !== nextMsg.toolCalls?.length
    ) {
      return false;
    }
  }
  
  // Check if pending permissions changed for this group
  return prevProps.pendingPermissions === nextProps.pendingPermissions;
});

export const ChatPanel = memo(({ 
  messages, 
  session,
  onSendMessage,
  pendingPermissions,
  onPermissionApprove,
  onPermissionDeny,
  onPermissionAllowForSession,
  sessionConfigComponent,
  isProcessing = false,
  onCancelRequest,
  onFileClick
}: ChatPanelProps) => {
  const [input, setInput] = useState('');
  const [attachedFiles, setAttachedFiles] = useState<FileNode[]>([]);
  const [attachedImages, setAttachedImages] = useState<ImageAttachment[]>([]);
  const [isUploadingImage, setIsUploadingImage] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputContainerRef = useRef<HTMLDivElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);

  // Use a ref to track if we should auto-scroll
  const shouldAutoScrollRef = useRef(true);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = useCallback(() => {
    if (shouldAutoScrollRef.current) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, []);

  // Check if user has scrolled up
  const handleScroll = useCallback(() => {
    if (!scrollContainerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollContainerRef.current;
    const isNearBottom = scrollHeight - scrollTop - clientHeight < 100;
    shouldAutoScrollRef.current = isNearBottom;
  }, []);

  // Scroll when messages change (new messages, content updates, or streaming)
  const messageCount = messages.length;
  const lastMessageId = messages[messages.length - 1]?.id;
  const lastMessageContent = messages[messages.length - 1]?.content;
  const lastMessageIsStreaming = messages[messages.length - 1]?.isStreaming;
  useEffect(() => {
    scrollToBottom();
  }, [messageCount, lastMessageId, lastMessageContent, lastMessageIsStreaming, scrollToBottom]);

  // Collect all tool results from all messages into a Map for lookup
  const allToolResults = useMemo(() => {
    const resultsMap = new Map<string, typeof messages[0]['toolResults'][0]>();
    messages.forEach(msg => {
      if (msg.toolResults) {
        msg.toolResults.forEach(result => {
          if (result.tool_call_id) {
            resultsMap.set(result.tool_call_id, result);
          }
        });
      }
    });
    console.log('All tool results collected:', resultsMap.size, 'results');
    return resultsMap;
  }, [messages]);

  const groupedMessages = useMemo(() => {
    const filtered = messages.filter((msg) => {
      const hasContent = msg.content && msg.content.trim();
      const hasReasoning = msg.reasoning && msg.reasoning.trim();
      const hasToolCalls = msg.toolCalls && msg.toolCalls.some(tc => tc && tc.id && tc.name);
      const isStreaming = msg.isStreaming;
      return hasContent || hasReasoning || hasToolCalls || isStreaming;
    });

    const groups: Message[][] = [];
    let currentGroup: Message[] = [];

    filtered.forEach((msg) => {
      if (currentGroup.length === 0) {
        currentGroup.push(msg);
      } else {
        const lastMsg = currentGroup[currentGroup.length - 1];
        if (msg.role === 'user' || lastMsg.role === 'user') {
          groups.push(currentGroup);
          currentGroup = [msg];
        } else {
          currentGroup.push(msg);
        }
      }
    });
    
    if (currentGroup.length > 0) {
      groups.push(currentGroup);
    }
    
    return groups;
  }, [messages]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() && attachedFiles.length === 0 && attachedImages.length === 0) return;
    
    // Force scroll to bottom when user sends a message
    shouldAutoScrollRef.current = true;
    
    onSendMessage(input, attachedFiles, attachedImages);
    setInput('');
    setAttachedFiles([]);
    setAttachedImages([]);
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
        inputContainerRef.current.style.borderColor = '#a855f7'; // purple-500
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

    // Check for image files first
    if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
      const hasImages = Array.from(e.dataTransfer.files).some(f => f.type.startsWith('image/'));
      if (hasImages) {
        handleImageDrop(e.dataTransfer.files);
        return;
      }
    }

    // Handle JSON file nodes
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

  const handlePaste = (e: React.ClipboardEvent) => {
    // Check for image paste first
    const items = e.clipboardData.items;
    const hasImage = Array.from(items).some(item => item.type.startsWith('image/'));
    if (hasImage) {
      e.preventDefault();
      handleImagePaste(items);
      return;
    }

    // Handle JSON file nodes
    const data = e.clipboardData.getData('application/json');
    if (data) {
      try {
        const fileNode = JSON.parse(data) as FileNode;
        if (fileNode.name && (fileNode.path || fileNode.content)) {
          e.preventDefault();
          setAttachedFiles(prev => {
             // Generate unique ID if conflict for paste
             const id = fileNode.id || `pasted-${Date.now()}`;
             const newNode = { ...fileNode, id };
             
             if (prev.some(f => f.id === newNode.id)) return prev;
             return [...prev, newNode];
          });
        }
      } catch (err) {
        // Fallback to default paste
      }
    }
  };

  const removeAttachedFile = (fileId: string) => {
    setAttachedFiles(prev => prev.filter(f => f.id !== fileId));
  };

  const removeAttachedImage = (url: string) => {
    setAttachedImages(prev => prev.filter(img => img.url !== url));
  };

  // Upload image to server
  const uploadImage = async (file: File): Promise<ImageAttachment | null> => {
    const token = localStorage.getItem('jwt_token');
    if (!token) {
      console.error('No JWT token for image upload');
      return null;
    }

    const formData = new FormData();
    formData.append('image', file);

    try {
      const response = await fetch(`${API_URL}/upload`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${token}`,
        },
        body: formData,
      });

      if (!response.ok) {
        const error = await response.json();
        console.error('Image upload failed:', error);
        return null;
      }

      const result: ImageUploadResponse = await response.json();
      return {
        url: result.url,
        filename: result.filename,
        mime_type: result.mime_type,
        size: result.size,
      };
    } catch (error) {
      console.error('Image upload error:', error);
      return null;
    }
  };

  // Handle file input change for images
  const handleImageSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    setIsUploadingImage(true);
    
    for (const file of Array.from(files)) {
      // Validate file type
      if (!file.type.startsWith('image/')) {
        console.warn('Skipping non-image file:', file.name);
        continue;
      }

      // Create local preview
      const localPreview = URL.createObjectURL(file);
      
      // Upload to server
      const uploaded = await uploadImage(file);
      if (uploaded) {
        uploaded.localPreview = localPreview;
        setAttachedImages(prev => [...prev, uploaded]);
      } else {
        URL.revokeObjectURL(localPreview);
      }
    }

    setIsUploadingImage(false);
    // Reset input
    if (imageInputRef.current) {
      imageInputRef.current.value = '';
    }
  };

  // Handle image paste from clipboard
  const handleImagePaste = async (items: DataTransferItemList) => {
    for (const item of Array.from(items)) {
      if (item.type.startsWith('image/')) {
        const file = item.getAsFile();
        if (file) {
          setIsUploadingImage(true);
          const localPreview = URL.createObjectURL(file);
          const uploaded = await uploadImage(file);
          if (uploaded) {
            uploaded.localPreview = localPreview;
            setAttachedImages(prev => [...prev, uploaded]);
          } else {
            URL.revokeObjectURL(localPreview);
          }
          setIsUploadingImage(false);
        }
      }
    }
  };

  // Handle image drop
  const handleImageDrop = async (files: FileList) => {
    setIsUploadingImage(true);
    
    for (const file of Array.from(files)) {
      if (file.type.startsWith('image/')) {
        const localPreview = URL.createObjectURL(file);
        const uploaded = await uploadImage(file);
        if (uploaded) {
          uploaded.localPreview = localPreview;
          setAttachedImages(prev => [...prev, uploaded]);
        } else {
          URL.revokeObjectURL(localPreview);
        }
      }
    }
    
    setIsUploadingImage(false);
  };

  return (
    <div className="flex flex-col h-full bg-black">
      <div className="h-12 px-4 border-b border-[#1a1a1a] bg-[#0A0A0A] flex items-center gap-4">
        <h2 className="text-sm font-medium text-gray-200 truncate">
          {session?.title || (messages.length === 0 ? 'New Chat' : 'Chat')}
        </h2>
        {session && (
          <div className="flex items-center gap-3 ml-auto">
            {session.context_window > 0 ? (
              <div className="flex items-center gap-2 text-xs text-gray-500" title={`Used: ${session.prompt_tokens + session.completion_tokens} / ${session.context_window} tokens`}>
                <div className="w-16 bg-[#1a1a1a] rounded-full h-1 overflow-hidden">
                    <div 
                      className={`h-full rounded-full transition-all duration-500 ${
                        ((session.prompt_tokens + session.completion_tokens) / session.context_window) > 0.9 ? 'bg-red-500' :
                        ((session.prompt_tokens + session.completion_tokens) / session.context_window) > 0.7 ? 'bg-yellow-500' :
                        'bg-emerald-500'
                      }`}
                      style={{ width: `${Math.min(100, ((session.prompt_tokens + session.completion_tokens) / session.context_window) * 100)}%` }}
                    />
                </div>
                <span className="text-gray-500">{Math.round(((session.prompt_tokens + session.completion_tokens) / session.context_window) * 100)}%</span>
              </div>
            ) : (session.prompt_tokens + session.completion_tokens) > 0 ? (
              <div className="flex items-center gap-1 text-xs text-gray-500" title="Total tokens used (context window unknown)">
                <span>{((session.prompt_tokens + session.completion_tokens) / 1000).toFixed(1)}k tokens</span>
              </div>
            ) : null}
            <div className="flex items-center gap-1 text-xs">
                <span className="text-gray-500">${session.cost?.toFixed(4) || '0.0000'}</span>
            </div>
          </div>
        )}
      </div>
      
      <div 
        ref={scrollContainerRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto p-4 space-y-4"
      >
        {/* Welcome message for new chat */}
        {groupedMessages.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="w-16 h-16 rounded-full bg-gradient-to-br from-purple-500/20 to-blue-500/20 flex items-center justify-center mb-4 border border-purple-500/30">
              <Sparkles className="w-8 h-8 text-purple-400" />
            </div>
            <h3 className="text-lg font-medium text-gray-200 mb-2">Start a New Chat</h3>
            <p className="text-sm text-gray-500 max-w-sm">
              Select a model and send a message to begin. Auto mode uses Z.AI GLM-4.5.
            </p>
          </div>
        )}
        {groupedMessages.map((group) => (
          <MessageGroup
            key={group[0].id}
            group={group}
            allToolResults={allToolResults}
            pendingPermissions={pendingPermissions}
            onPermissionApprove={onPermissionApprove}
            onPermissionDeny={onPermissionDeny}
            onPermissionAllowForSession={onPermissionAllowForSession}
            onFileClick={onFileClick}
          />
        ))}
        <div ref={messagesEndRef} />
      </div>

      <div className="p-4 bg-[#0A0A0A]">
        <div 
            ref={inputContainerRef}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            className="relative bg-[#111] border border-[#333] rounded-lg flex flex-col focus-within:ring-1 focus-within:ring-purple-500 focus-within:border-purple-500 transition-colors"
        >
          {/* Attached files and images */}
          {(attachedFiles.length > 0 || attachedImages.length > 0) && (
            <div className="flex flex-wrap gap-2 p-2 border-b border-[#333]/50">
                {/* Attached files */}
                {attachedFiles.map(file => (
                    <div key={file.id} className="flex items-center gap-1.5 px-2 py-1 bg-black/50 rounded text-xs text-blue-300 border border-blue-500/30">
                        {file.type === 'folder' ? <FolderIcon size={12} /> : <FileIcon size={12} />}
                        <span className="truncate max-w-[150px]">
                            {file.name}
                            {file.startLine !== undefined && file.endLine !== undefined && (
                                <span className="text-gray-400 ml-1">
                                    ({file.startLine}-{file.endLine})
                                </span>
                            )}
                        </span>
                        <button 
                            onClick={() => removeAttachedFile(file.id)}
                            className="hover:text-white ml-0.5"
                        >
                            <X size={12} />
                        </button>
                    </div>
                ))}
                
                {/* Attached images */}
                {attachedImages.map(img => (
                    <div key={img.url} className="relative group">
                        <img 
                            src={img.localPreview || img.url} 
                            alt={img.filename}
                            className="h-16 w-16 object-cover rounded border border-green-500/30"
                        />
                        <button 
                            onClick={() => removeAttachedImage(img.url)}
                            className="absolute -top-1 -right-1 p-0.5 bg-red-600 rounded-full text-white opacity-0 group-hover:opacity-100 transition-opacity"
                        >
                            <X size={10} />
                        </button>
                        <span className="absolute bottom-0 left-0 right-0 bg-black/70 text-[10px] text-green-300 truncate px-1">
                            {img.filename}
                        </span>
                    </div>
                ))}
            </div>
          )}
          
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onPaste={handlePaste}
            onKeyDown={handleKeyDown}
            placeholder={attachedFiles.length > 0 ? "Describe what to do with these files..." : "Ask anything about your code..."}
            className="w-full bg-transparent border-none text-sm text-gray-200 placeholder-gray-500 p-3 min-h-[60px] max-h-[200px] resize-none focus:ring-0 focus:outline-none scrollbar-thin scrollbar-thumb-gray-600 scrollbar-track-transparent"
            rows={2}
          />
          
          <div className="flex justify-between items-center px-2 pb-2">
            <div className="flex items-center gap-2">
               {sessionConfigComponent}
               
               {/* Image upload button */}
               <input
                 ref={imageInputRef}
                 type="file"
                 accept="image/jpeg,image/png,image/gif,image/webp"
                 multiple
                 onChange={handleImageSelect}
                 className="hidden"
               />
               <button
                 onClick={() => imageInputRef.current?.click()}
                 disabled={isUploadingImage}
                 className="p-1.5 text-gray-400 hover:text-green-400 hover:bg-green-500/10 rounded-md transition-colors disabled:opacity-50"
                 title="Upload image"
               >
                 {isUploadingImage ? (
                   <Loader2 size={16} className="animate-spin" />
                 ) : (
                   <ImagePlus size={16} />
                 )}
               </button>
            </div>
            
            {isProcessing ? (
              <button
                onClick={onCancelRequest}
                className="relative p-1.5 bg-red-600 text-white rounded-md hover:bg-red-700 transition-colors group"
                title="Cancel"
              >
                {/* Breathing light effect */}
                <span className="absolute inset-0 rounded-md bg-red-500 animate-ping opacity-30" />
                <span className="absolute inset-0 rounded-md bg-red-400 animate-pulse opacity-40" />
                <Square size={16} className="relative z-10 fill-current" />
              </button>
            ) : (
              <button
                onClick={handleSubmit}
                disabled={!input.trim() && attachedFiles.length === 0 && attachedImages.length === 0}
                className="p-1.5 bg-gradient-to-r from-purple-600 to-blue-600 text-white rounded-md hover:from-purple-700 hover:to-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <Send size={16} />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}, (prevProps, nextProps) => {
  // Custom comparison function for memo
  // Only re-render if these props actually change
  return (
    prevProps.messages === nextProps.messages &&
    prevProps.session?.id === nextProps.session?.id &&
    prevProps.session?.cost === nextProps.session?.cost &&
    prevProps.session?.prompt_tokens === nextProps.session?.prompt_tokens &&
    prevProps.session?.completion_tokens === nextProps.session?.completion_tokens &&
    prevProps.pendingPermissions === nextProps.pendingPermissions &&
    prevProps.isProcessing === nextProps.isProcessing &&
    prevProps.onSendMessage === nextProps.onSendMessage &&
    prevProps.sessionConfigComponent === nextProps.sessionConfigComponent
  );
});
