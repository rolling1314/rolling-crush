import React, { useState, useRef, useEffect } from 'react';
import { Send, User, Bot } from 'lucide-react';
import { type Message } from '../types';
import { cn } from '../lib/utils';

interface ChatPanelProps {
  messages: Message[];
  onSendMessage: (content: string) => void;
}

export const ChatPanel = ({ messages, onSendMessage }: ChatPanelProps) => {
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
    <div className="flex flex-col h-full bg-[#1e1e1e] border-r border-gray-700">
      <div className="p-4 border-b border-gray-700 bg-[#252526]">
        <h2 className="text-sm font-semibold text-gray-200">AI Assistant</h2>
      </div>
      
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={cn(
              "flex gap-3 max-w-[90%]",
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
              "p-3 rounded-lg text-sm leading-relaxed",
              msg.role === 'user' 
                ? "bg-blue-600/10 text-blue-100 border border-blue-600/20" 
                : "bg-gray-700/50 text-gray-200 border border-gray-600/30"
            )}>
              {msg.content}
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

