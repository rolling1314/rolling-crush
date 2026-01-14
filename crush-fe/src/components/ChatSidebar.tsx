import React, { useState } from 'react';
import { 
  Plus, 
  MessageSquare, 
  Search,
  LogOut,
  Trash2,
  Sparkles,
  User,
  Home,
  PanelRightClose,
  PanelRightOpen
} from 'lucide-react';
import { type Session } from '../types';
import { cn } from '../lib/utils';

interface ChatSidebarProps {
  sessions: Session[];
  currentSessionId: string | null;
  isPendingSession: boolean;
  onSelectSession: (sessionId: string) => void;
  onNewSession: () => void;
  onDeleteSession: (sessionId: string) => void;
  onNavigateToProjects: () => void;
  onLogout: () => void;
  username?: string;
  maxWidth?: number; // Maximum width when expanded (1/4 of chat panel)
}

export const ChatSidebar: React.FC<ChatSidebarProps> = ({
  sessions,
  currentSessionId,
  isPendingSession,
  onSelectSession,
  onNewSession,
  onDeleteSession,
  onNavigateToProjects,
  onLogout,
  username,
  maxWidth = 280,
}) => {
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [hoveredSessionId, setHoveredSessionId] = useState<string | null>(null);

  const collapsedWidth = 48;
  const expandedWidth = Math.min(maxWidth, 360);

  const filteredSessions = sessions.filter(session =>
    session.title.toLowerCase().includes(searchQuery.toLowerCase())
  );

  // Group sessions by date
  const groupedSessions = React.useMemo(() => {
    const now = new Date();
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
    const yesterday = today - 86400000;
    const lastWeek = today - 7 * 86400000;
    const lastMonth = today - 30 * 86400000;

    const groups: { label: string; sessions: Session[] }[] = [
      { label: 'Today', sessions: [] },
      { label: 'Yesterday', sessions: [] },
      { label: 'Last 7 days', sessions: [] },
      { label: 'Last 30 days', sessions: [] },
      { label: 'Older', sessions: [] },
    ];

    filteredSessions.forEach(session => {
      const sessionTime = session.updated_at || session.created_at;
      if (sessionTime >= today) {
        groups[0].sessions.push(session);
      } else if (sessionTime >= yesterday) {
        groups[1].sessions.push(session);
      } else if (sessionTime >= lastWeek) {
        groups[2].sessions.push(session);
      } else if (sessionTime >= lastMonth) {
        groups[3].sessions.push(session);
      } else {
        groups[4].sessions.push(session);
      }
    });

    return groups.filter(g => g.sessions.length > 0);
  }, [filteredSessions]);

  return (
    <div 
      className={cn(
        "h-full flex flex-col bg-[#0A0A0A] border-l border-[#1a1a1a] transition-all duration-300 ease-in-out relative",
        isCollapsed ? "items-center" : ""
      )}
      style={{ width: isCollapsed ? collapsedWidth : expandedWidth }}
    >
      {/* Header with Logo and Toggle */}
      <div className={cn(
        "h-12 flex items-center shrink-0 border-b border-[#1a1a1a]",
        isCollapsed ? "justify-center px-2" : "justify-between px-3"
      )}>
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-gradient-to-br from-purple-500 to-blue-500 flex items-center justify-center shrink-0">
            <Sparkles size={14} className="text-white" />
          </div>
          {!isCollapsed && (
            <span className="font-semibold text-white text-sm">Crush</span>
          )}
        </div>
        {!isCollapsed && (
          <button
            onClick={() => setIsCollapsed(true)}
            className="p-1.5 rounded-md text-gray-500 hover:text-white hover:bg-[#1a1a1a] transition-colors"
            title="Collapse"
          >
            <PanelRightClose size={18} />
          </button>
        )}
      </div>
      
      {/* Toggle button when collapsed */}
      {isCollapsed && (
        <button
          onClick={() => setIsCollapsed(false)}
          className="mt-2 p-2 rounded-lg text-gray-500 hover:text-white hover:bg-[#1a1a1a] transition-colors"
          title="Expand"
        >
          <PanelRightOpen size={18} />
        </button>
      )}

      {/* New Chat Button */}
      <div className={cn(
        "shrink-0",
        isCollapsed ? "p-2" : "p-3"
      )}>
        <button
          onClick={onNewSession}
          className={cn(
            "flex items-center gap-2 rounded-lg transition-all text-gray-300 hover:text-white",
            isCollapsed 
              ? "w-10 h-10 justify-center bg-[#1a1a1a] hover:bg-[#222] border border-[#333]" 
              : "w-full px-3 py-2.5 bg-[#1a1a1a] hover:bg-[#222] border border-[#333]"
          )}
          title="New Chat"
        >
          <Plus size={18} />
          {!isCollapsed && <span className="text-sm font-medium">New Chat</span>}
        </button>
      </div>

      {/* Search (only when expanded) */}
      {!isCollapsed && (
        <div className="px-3 pb-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
            <input
              type="text"
              placeholder="Search..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-9 pr-3 py-2 bg-[#111] border border-[#222] rounded-lg text-sm text-gray-300 placeholder-gray-600 focus:outline-none focus:border-[#333] focus:ring-1 focus:ring-purple-500/20 transition-colors"
            />
          </div>
        </div>
      )}

      {/* Session List */}
      <div className="flex-1 overflow-y-auto scrollbar-thin scrollbar-thumb-[#333] scrollbar-track-transparent">
        {isCollapsed ? (
          // Collapsed view - empty, just spacing
          <div className="flex-1" />
        ) : (
          // Expanded view - grouped sessions
          <div className="px-2 py-1">
            {/* Pending session */}
            {isPendingSession && (
              <div className="px-1 py-1 mb-2">
                <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-purple-500/10 border border-purple-500/20">
                  <MessageSquare size={14} className="text-purple-400 shrink-0" />
                  <span className="text-sm text-purple-300 truncate">New Chat</span>
                </div>
              </div>
            )}

            {groupedSessions.map((group, groupIndex) => (
              <div key={groupIndex} className="mb-3">
                <div className="px-3 py-1.5 text-xs text-gray-500 font-medium">
                  {group.label}
                </div>
                {group.sessions.map(session => (
                  <div
                    key={session.id}
                    onMouseEnter={() => setHoveredSessionId(session.id)}
                    onMouseLeave={() => setHoveredSessionId(null)}
                    onClick={() => onSelectSession(session.id)}
                    className={cn(
                      "group flex items-center gap-2 px-3 py-2.5 mx-1 rounded-lg cursor-pointer transition-colors relative",
                      currentSessionId === session.id
                        ? "bg-[#1a1a1a] text-white"
                        : "text-gray-400 hover:bg-[#111] hover:text-gray-200"
                    )}
                  >
                    <MessageSquare size={14} className="shrink-0" />
                    <span className="text-sm truncate flex-1">{session.title}</span>
                    
                    {/* Delete button on hover */}
                    {hoveredSessionId === session.id && (
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          onDeleteSession(session.id);
                        }}
                        className="p-1 rounded hover:bg-red-500/20 text-gray-500 hover:text-red-400 transition-colors"
                        title="Delete"
                      >
                        <Trash2 size={12} />
                      </button>
                    )}
                  </div>
                ))}
              </div>
            ))}

            {filteredSessions.length === 0 && !isPendingSession && (
              <div className="px-3 py-8 text-center">
                <div className="w-12 h-12 rounded-full bg-[#1a1a1a] flex items-center justify-center mx-auto mb-3">
                  <MessageSquare size={20} className="text-gray-600" />
                </div>
                <p className="text-sm text-gray-500">
                  {searchQuery ? 'No matches found' : 'No chats yet'}
                </p>
                <p className="text-xs text-gray-600 mt-1">
                  Click New Chat to start
                </p>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Bottom Section - User & Actions */}
      <div className={cn(
        "shrink-0 border-t border-[#1a1a1a]",
        isCollapsed ? "p-2" : "p-3"
      )}>
        {isCollapsed ? (
          // Collapsed bottom icons
          <div className="flex flex-col items-center gap-2">
            <button
              onClick={onNavigateToProjects}
              className="w-10 h-10 rounded-lg flex items-center justify-center text-gray-400 hover:text-white hover:bg-[#1a1a1a] transition-colors"
              title="Projects"
            >
              <Home size={18} />
            </button>
            <div 
              className="w-8 h-8 rounded-full bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center text-white text-xs font-medium cursor-pointer"
              title={username || 'User'}
            >
              {username ? username.charAt(0).toUpperCase() : <User size={14} />}
            </div>
          </div>
        ) : (
          // Expanded bottom section
          <div className="space-y-2">
            {/* Back to Projects */}
            <button
              onClick={onNavigateToProjects}
              className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-gray-400 hover:text-white hover:bg-[#111] transition-colors"
            >
              <Home size={16} />
              <span className="text-sm">Projects</span>
            </button>

            {/* User Profile */}
            <div className="flex items-center gap-3 px-3 py-2 rounded-lg bg-[#111] border border-[#1a1a1a]">
              <div className="w-8 h-8 rounded-full bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center text-white text-sm font-medium shrink-0">
                {username ? username.charAt(0).toUpperCase() : <User size={14} />}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-sm text-white truncate">{username || 'User'}</div>
                <div className="text-xs text-gray-500">Free</div>
              </div>
              <button
                onClick={onLogout}
                className="p-1.5 rounded text-gray-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
                title="Logout"
              >
                <LogOut size={14} />
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
