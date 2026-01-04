import React, { useState, useRef, useEffect } from 'react';
import { Send, User, Bot } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeHighlight from 'rehype-highlight';
import { type Message, type PermissionRequest } from '../types';
import { ToolCallDisplay } from './ToolCallDisplay';
import { cn } from '../lib/utils';
import 'highlight.js/styles/github-dark.css';

interface ChatPanelProps {
  messages: Message[];
  onSendMessage: (content: string) => void;
  pendingPermissions: Map<string, PermissionRequest>;
  onPermissionApprove: (toolCallId: string) => void;
  onPermissionDeny: (toolCallId: string) => void;
}

export const ChatPanel = ({ 
  messages, 
  onSendMessage,
  pendingPermissions,
  onPermissionApprove,
  onPermissionDeny 
}: ChatPanelProps) => {
  const [input, setInput] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;
    onSendMessage(input);
    setInput('');
  };

  return (
    <div className="flex flex-col h-full bg-[#1e1e1e]">
      <div className="p-4 border-b border-gray-700 bg-[#252526]">
        <h2 className="text-sm font-semibold text-gray-200">AI Assistant</h2>
      </div>
      
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.map((msg) => (
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
            <div className="flex flex-col gap-2 flex-1">
              {/* Reasoning content (if present) */}
              {msg.reasoning && (
                <div className="p-3 rounded-md text-xs bg-purple-900/20 text-purple-200 border border-purple-700/30 reasoning-box">
                  <div className="font-semibold mb-2 flex items-center gap-2">
                    <span className="text-purple-400">💭</span>
                    <span>Thinking Process</span>
                  </div>
                  <div className="whitespace-pre-wrap font-mono text-purple-100/80 leading-relaxed streaming-content">
                    {msg.reasoning}
                  </div>
                </div>
              )}
              
              {/* Tool Calls */}
              {msg.toolCalls && msg.toolCalls.length > 0 && (
                <div className="space-y-2">
                  {msg.toolCalls.map((toolCall) => {
                    const result = msg.toolResults?.find(r => r.tool_call_id === toolCall.id);
                    const needsPermission = pendingPermissions.has(toolCall.id);
                    console.log('Rendering ToolCall:', {
                      id: toolCall.id,
                      name: toolCall.name,
                      needsPermission,
                      pendingPermissionsSize: pendingPermissions.size,
                      pendingKeys: Array.from(pendingPermissions.keys())
                    });
                    return (
                      <ToolCallDisplay
                        key={toolCall.id}
                        toolCall={toolCall}
                        result={result}
                        needsPermission={needsPermission}
                        onApprove={onPermissionApprove}
                        onDeny={onPermissionDeny}
                      />
                    );
                  })}
                </div>
              )}
              
              {/* Main content */}
              {msg.content && (
                <div className={cn(
                  "p-3 rounded-lg text-sm leading-relaxed prose prose-invert prose-sm max-w-none streaming-content",
                  msg.role === 'user' 
                    ? "bg-blue-600/10 text-blue-100 border border-blue-600/20" 
                    : "bg-gray-700/50 text-gray-200 border border-gray-600/30",
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
                    <span className="whitespace-pre-wrap">{msg.content}</span>
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

      <div className="p-4 border-t border-gray-700 bg-[#252526]">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <input
            type="text"
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Ask something about the code..."
            className="flex-1 bg-[#3c3c3c] border border-gray-600 rounded-md px-3 py-2 text-sm text-gray-200 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 placeholder-gray-500"
          />
          <button
            type="submit"
            disabled={!input.trim()}
            className="p-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            <Send size={18} />
          </button>
        </form>
      </div>
    </div>
  );
};

