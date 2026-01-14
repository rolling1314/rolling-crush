import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';

import { 
  LogOut, 
  Settings, 
  Users, 
  Folder, 
  Globe, 
  MoreHorizontal, 
  LayoutGrid,
  Diamond,
  ChevronDown,
  X,
  PanelLeft,
  Plus,
  Gift,
  MessageCircle,
  Terminal,
  Command
} from 'lucide-react';
import axios from 'axios';

const API_URL = '/api';

interface Project {
  id: string;
  name: string;
  description: string;
  external_ip: string;
  frontend_port: number;
  workspace_path: string;
  backend_language?: string;
  backend_port?: number;
  container_name?: string;
  need_database?: boolean;
  created_at: number;
  updated_at: number;
}

// Mock gradients for project covers
const GRADIENTS = [
  'from-blue-500 to-purple-600',
  'from-yellow-200 to-yellow-500',
  'from-purple-500 to-indigo-600',
  'from-green-400 to-emerald-600',
  'from-pink-500 to-rose-500',
  'from-orange-400 to-red-500',
];

export default function ProjectListPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newProject, setNewProject] = useState({ 
    name: '', 
    backend_language: '',  // '', 'go', 'java', 'python'
    need_database: false 
  });
  const [creating, setCreating] = useState(false);
  const navigate = useNavigate();
  const username = localStorage.getItem('username') || 'User';
  const email = localStorage.getItem('email') || 'user@example.com';
  const [showUserMenu, setShowUserMenu] = useState(false);
  const [isCollapsed, setIsCollapsed] = useState(false);

  // Helper to get avatar text (first 3 chars of email)
  const getAvatarText = (email: string) => {
    return email.substring(0, 3);
  };

  useEffect(() => {
    loadProjects();
  }, []);

  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && showCreateModal) {
        setShowCreateModal(false);
      }
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [showCreateModal]);

  const loadProjects = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.get(`${API_URL}/projects`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setProjects(response.data || []);
    } catch (error) {
      console.error('Failed to load projects:', error);
    } finally {
      setLoading(false);
    }
  };

  const createProject = async () => {
    if (!newProject.name.trim()) return;
    
    setCreating(true);
    try {
      const token = localStorage.getItem('jwt_token');
      
      // 构建请求数据
      const projectData: any = {
        name: newProject.name,
        backend_language: newProject.backend_language || null,
        need_database: newProject.need_database
      };
      
      await axios.post(`${API_URL}/projects`, projectData, {
        headers: { Authorization: `Bearer ${token}` }
      });
      
      setShowCreateModal(false);
      setNewProject({ name: '', backend_language: '', need_database: false });
      loadProjects();
    } catch (error: any) {
      console.error('Failed to create project:', error);
      alert('创建项目失败: ' + (error.response?.data?.error || error.message));
    } finally {
      setCreating(false);
    }
  };

  const selectProject = (projectId: string) => {
    navigate(`/projects/${projectId}`);
  };

  const handleLogout = () => {
    localStorage.removeItem('jwt_token');
    localStorage.removeItem('username');
    localStorage.removeItem('user_id');
    navigate('/login');
  };

  if (loading) {
    return <div className="flex items-center justify-center h-screen bg-black text-white">Loading...</div>;
  }

  return (
    <div className="flex h-screen w-full bg-black text-white overflow-hidden font-sans">
      {/* Sidebar */}
      <div 
        className={`${
          isCollapsed ? 'w-[70px]' : 'w-[280px]'
        } flex-shrink-0 flex flex-col border-r border-[#222] bg-[#0A0A0A] transition-all duration-300 ease-in-out relative`}
      >
        {/* Toggle Button - Only visible on hover or when collapsed logic needed */}
        <button 
          onClick={() => setIsCollapsed(!isCollapsed)}
          className={`absolute top-3 right-3 text-gray-500 hover:text-white transition-colors z-10 ${isCollapsed ? 'hidden' : ''}`}
        >
          <PanelLeft size={18} />
        </button>

        {/* Sidebar Header / Logo */}
        <div className={`p-4 ${isCollapsed ? 'flex justify-center items-center' : ''}`}>
           <div className="flex items-center gap-2 text-white font-bold text-lg cursor-pointer">
              {isCollapsed ? (
                <div 
                  onClick={() => setIsCollapsed(!isCollapsed)} 
                  className="cursor-pointer hover:text-gray-300"
                >
                   <Terminal size={24} />
                </div>
              ) : (
                <>
                  <Terminal size={20} />
                  <span>Enter</span>
                </>
              )}
           </div>
        </div>

        {/* Workspace & Navigation */}
        <div className={`flex-1 flex flex-col ${isCollapsed ? 'px-2 items-center' : 'p-4'} space-y-6 overflow-y-auto no-scrollbar`}>
           
           {/* Workspace Switcher */}
           {isCollapsed ? (
             <div className="w-10 h-10 rounded-lg bg-cyan-300 flex items-center justify-center text-black font-bold text-lg mb-2 cursor-pointer hover:opacity-90">
                {username.charAt(0).toUpperCase()}
             </div>
           ) : (
             <div className="mb-2">
                <button className="flex items-center gap-3 w-full p-2 hover:bg-[#1A1A1A] rounded-lg transition-colors group">
                  <div className="w-8 h-8 rounded bg-cyan-300 flex items-center justify-center text-black font-bold text-lg">
                    {username.charAt(0).toUpperCase()}
                  </div>
                  <div className="flex-1 text-left min-w-0">
                    <div className="text-sm font-medium text-gray-200 truncate">{username}'s Workspace</div>
                    <div className="text-xs text-gray-500 truncate">Owner · 1 Member</div>
                  </div>
                  <ChevronDown size={16} className="text-gray-500 group-hover:text-gray-300" />
                </button>
                
                <div className="mt-2 px-2 flex items-center gap-2 text-xs text-yellow-500 bg-yellow-500/10 py-1.5 rounded border border-yellow-500/20 w-fit">
                  <Diamond size={12} fill="currentColor" />
                  <span>1,062 Credits</span>
                </div>
             </div>
           )}

           {/* New Project Action */}
           {isCollapsed ? (
              <button
                onClick={() => setShowCreateModal(true)}
                className="w-10 h-10 rounded-lg border border-[#333] hover:bg-[#222] flex items-center justify-center text-white transition-colors"
                title="New Project"
              >
                <Plus size={20} />
              </button>
           ) : (
              <button
                onClick={() => setShowCreateModal(true)}
                className="w-full bg-[#1A1A1A] hover:bg-[#222] text-white py-2 px-4 rounded-lg border border-[#333] flex items-center justify-center gap-2 transition-all font-medium text-sm"
              >
                <span>New Project</span>
              </button>
           )}

           {/* Navigation Links */}
           <div className={`space-y-1 w-full ${isCollapsed ? 'flex flex-col items-center gap-2' : ''}`}>
              {!isCollapsed && <div className="text-xs font-semibold text-gray-500 px-2 mb-2 uppercase tracking-wider">Workspace</div>}
              
              <button className={`flex items-center gap-3 ${isCollapsed ? 'p-2 justify-center' : 'px-2 py-2 w-full'} bg-[#1A1A1A] text-white rounded-md text-sm font-medium`}>
                <Folder size={18} className={isCollapsed ? 'text-white' : 'text-gray-400'} />
                {!isCollapsed && <span>Projects</span>}
              </button>
              
              <button className={`flex items-center gap-3 ${isCollapsed ? 'p-2 justify-center' : 'px-2 py-2 w-full'} text-gray-400 hover:text-white hover:bg-[#1A1A1A] rounded-md text-sm transition-colors`}>
                <Users size={18} />
                {!isCollapsed && <span>Members</span>}
              </button>

              {!isCollapsed && <div className="text-xs font-semibold text-gray-500 px-2 mt-6 mb-2 uppercase tracking-wider">Discovery</div>}
              
              <button className={`flex items-center gap-3 ${isCollapsed ? 'p-2 justify-center mt-4' : 'px-2 py-2 w-full'} text-gray-400 hover:text-white hover:bg-[#1A1A1A] rounded-md text-sm transition-colors`}>
                <Globe size={18} />
                {!isCollapsed && <span>Community</span>}
              </button>

              {/* Gift / Credits Icon for Collapsed Mode */}
              {isCollapsed && (
                 <button className="flex items-center justify-center p-2 text-gray-400 hover:text-white hover:bg-[#1A1A1A] rounded-md transition-colors" title="Credits">
                    <Gift size={18} />
                 </button>
              )}
           </div>
        </div>

        {/* User Footer */}
        <div className={`mt-auto border-t border-[#222] ${isCollapsed ? 'p-2 pb-4 flex flex-col items-center gap-4' : 'pt-4'}`}>
           {/* Bottom Icons (Settings & Discord) */}
           {isCollapsed ? (
             <div className="flex flex-col gap-4 mb-2">
                <button className="text-gray-500 hover:text-gray-300">
                   <Settings size={20} />
                </button>
                <button className="text-gray-500 hover:text-gray-300">
                   <MessageCircle size={20} />
                </button>
             </div>
           ) : null}

          <div className={`flex items-center ${isCollapsed ? 'justify-center' : 'justify-between px-2 py-2'} text-gray-500 relative`}>
            <div className="relative">
               {/* Trigger */}
               <div 
                 onClick={() => setShowUserMenu(!showUserMenu)}
                 className={`w-8 h-8 rounded-full flex items-center justify-center text-[10px] cursor-pointer transition-colors border select-none ${showUserMenu ? 'bg-white text-black border-white' : 'bg-[#1A1A1A] text-gray-400 hover:text-white border-transparent hover:border-gray-600'}`}
               >
                 {getAvatarText(email)}
               </div>
               
               {/* Popover */}
               {showUserMenu && (
                 <>
                   <div className="fixed inset-0 z-40" onClick={() => setShowUserMenu(false)} />
                   <div 
                     className={`absolute bottom-full ${isCollapsed ? 'left-full ml-2' : 'left-0 mb-2'} w-64 bg-[#252526] border border-gray-700 rounded-xl shadow-2xl p-2 z-50 overflow-hidden`}
                   >
                       {/* User Info Header */}
                       <div className="flex flex-col items-center py-6">
                          <div className="w-16 h-16 rounded-full bg-[#333] flex items-center justify-center text-xl text-gray-200 mb-3 border border-gray-600">
                            {getAvatarText(email)}
                          </div>
                          <div className="text-white font-medium text-lg">{username}</div>
                          <div className="text-gray-500 text-sm">{email}</div>
                       </div>
                       
                       <div className="h-px bg-gray-700 my-2 mx-2" />
                       
                       <button
                           onClick={handleLogout}
                           className="w-full flex items-center gap-3 px-3 py-2 text-gray-300 hover:text-white hover:bg-[#3e3e42] rounded-lg text-sm transition-colors"
                       >
                           <LogOut size={16} />
                           <span>Log Out</span>
                       </button>
                   </div>
                 </>
               )}
            </div>

            {!isCollapsed && (
              <div className="flex gap-3 items-center">
                <Settings size={18} className="hover:text-gray-300 cursor-pointer transition-colors" />
                <div className="w-5 h-5 bg-indigo-500 rounded-full" />
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Main Content */}
      <div className="flex-1 overflow-y-auto bg-black p-8">
        <div className="max-w-[1200px] mx-auto">
          {/* Header */}
          <div className="flex justify-between items-end mb-8">
            <div>
              <h1 className="text-3xl font-medium text-white mb-2">Projects</h1>
              <p className="text-gray-500 text-sm">
                Here are the current projects of the workspace, and also you can create more.
              </p>
            </div>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-4 py-2 bg-white text-black text-sm font-medium rounded hover:bg-gray-200 transition-colors"
            >
              New Project
            </button>
          </div>

          {/* Project Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {projects.map((project, index) => {
              const gradient = GRADIENTS[index % GRADIENTS.length];
              const date = new Date(project.created_at);
              const dateString = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`;
              
              return (
                <div
                  key={project.id}
                  onClick={() => selectProject(project.id)}
                  className="group cursor-pointer"
                >
                  {/* Card Image Area */}
                  <div className={`h-48 w-full rounded-xl bg-gradient-to-br ${gradient} mb-3 relative overflow-hidden group-hover:opacity-90 transition-opacity`}>
                    <div className="absolute inset-0 flex items-center justify-center">
                      {project.description === 'Work in progress' ? (
                         <div className="text-center">
                            <div className="w-12 h-12 bg-white/20 rounded-md mx-auto mb-2 rotate-12 backdrop-blur-sm" />
                            <span className="text-white/80 font-medium text-sm">Working in progress</span>
                         </div>
                      ) : (
                        <h3 className="text-white text-2xl font-bold opacity-80 px-6 text-center">{project.name}</h3>
                      )}
                    </div>
                  </div>
                  
                  {/* Card Meta */}
                  <div className="flex items-center gap-3 px-1">
                    <div className="w-8 h-8 rounded-full bg-[#1A1A1A] border border-[#333] flex items-center justify-center text-xs text-gray-400 flex-shrink-0">
                      {username.charAt(0).toUpperCase()}
                    </div>
                    <div className="flex-1 min-w-0">
                      <h4 className="text-gray-200 font-medium text-sm truncate">{project.name}</h4>
                      <p className="text-gray-500 text-xs truncate">{dateString}</p>
                    </div>
                    <button className="text-gray-500 hover:text-gray-300 opacity-0 group-hover:opacity-100 transition-opacity p-1">
                      <MoreHorizontal size={16} />
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
          
          {projects.length === 0 && (
             <div className="flex flex-col items-center justify-center py-20 text-gray-600">
               <div className="w-16 h-16 rounded-full bg-[#111] flex items-center justify-center mb-4">
                 <LayoutGrid size={32} />
               </div>
               <p>No projects yet.</p>
               <button 
                 onClick={() => setShowCreateModal(true)}
                 className="mt-4 text-blue-400 hover:text-blue-300 text-sm"
               >
                 Create your first project
               </button>
             </div>
          )}
        </div>
      </div>

      {/* Create Project Modal (Dark Theme) */}
      {showCreateModal && (
        <div 
          className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              setShowCreateModal(false);
            }
          }}
        >
          <div className="bg-[#1A1A1A] rounded-xl border border-[#333] shadow-2xl w-full max-w-[500px] overflow-hidden">
            <div className="p-6">
              <div className="flex items-center justify-between mb-6">
                <h2 className="text-xl font-bold text-white">Create New Project</h2>
                <button
                  onClick={() => setShowCreateModal(false)}
                  className="p-1 hover:bg-[#333] rounded transition-colors text-gray-400 hover:text-white"
                >
                  <X size={20} />
                </button>
              </div>

              <div className="space-y-4">
                {/* 项目名称 */}
                <div>
                  <label className="block text-xs font-medium text-gray-400 mb-1.5 uppercase tracking-wide">
                    Project Name
                  </label>
                  <input
                    type="text"
                    placeholder="e.g. My Awesome App"
                    value={newProject.name}
                    onChange={e => setNewProject({ ...newProject, name: e.target.value })}
                    className="w-full px-4 py-2.5 bg-[#0A0A0A] border border-[#333] rounded-lg focus:border-purple-500 focus:ring-1 focus:ring-purple-500 outline-none transition-all text-white placeholder-gray-600 text-sm"
                    autoFocus
                  />
                </div>

                {/* 后端语言选择 */}
                <div>
                  <label className="block text-xs font-medium text-gray-400 mb-1.5 uppercase tracking-wide">
                    Backend Language (Optional)
                  </label>
                  <div className="grid grid-cols-4 gap-2">
                    <button
                      type="button"
                      onClick={() => setNewProject({ ...newProject, backend_language: '' })}
                      className={`px-4 py-2.5 rounded-lg border text-sm font-medium transition-all ${
                        newProject.backend_language === '' 
                          ? 'bg-gradient-to-r from-purple-600 to-blue-600 border-purple-500 text-white' 
                          : 'bg-[#0A0A0A] border-[#333] text-gray-400 hover:border-purple-500'
                      }`}
                    >
                      None
                    </button>
                    <button
                      type="button"
                      onClick={() => setNewProject({ ...newProject, backend_language: 'go' })}
                      className={`px-4 py-2.5 rounded-lg border text-sm font-medium transition-all ${
                        newProject.backend_language === 'go' 
                          ? 'bg-gradient-to-r from-purple-600 to-blue-600 border-purple-500 text-white' 
                          : 'bg-[#0A0A0A] border-[#333] text-gray-400 hover:border-purple-500'
                      }`}
                    >
                      Go
                    </button>
                    <button
                      type="button"
                      onClick={() => setNewProject({ ...newProject, backend_language: 'java' })}
                      className={`px-4 py-2.5 rounded-lg border text-sm font-medium transition-all ${
                        newProject.backend_language === 'java' 
                          ? 'bg-gradient-to-r from-purple-600 to-blue-600 border-purple-500 text-white' 
                          : 'bg-[#0A0A0A] border-[#333] text-gray-400 hover:border-purple-500'
                      }`}
                    >
                      Java
                    </button>
                    <button
                      type="button"
                      onClick={() => setNewProject({ ...newProject, backend_language: 'python' })}
                      className={`px-4 py-2.5 rounded-lg border text-sm font-medium transition-all ${
                        newProject.backend_language === 'python' 
                          ? 'bg-gradient-to-r from-purple-600 to-blue-600 border-purple-500 text-white' 
                          : 'bg-[#0A0A0A] border-[#333] text-gray-400 hover:border-purple-500'
                      }`}
                    >
                      Python
                    </button>
                  </div>
                </div>

                {/* 数据库选项 - 仅在选择了后端语言时显示 */}
                {newProject.backend_language && (
                  <div>
                    <label className="flex items-center gap-2 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={newProject.need_database}
                        onChange={e => setNewProject({ ...newProject, need_database: e.target.checked })}
                        className="w-4 h-4 rounded bg-[#0A0A0A] border-[#333] text-purple-500 focus:ring-purple-500"
                      />
                      <span className="text-sm text-gray-300">Need Database (PostgreSQL)</span>
                    </label>
                  </div>
                )}

                {/* 提示信息 */}
                <div className="p-3 bg-purple-500/10 border border-purple-500/20 rounded-lg">
                  <p className="text-xs text-purple-400">
                    ✨ A Docker container will be automatically created with frontend (Vite) 
                    {newProject.backend_language && ` and ${newProject.backend_language.toUpperCase()} backend`}
                    {newProject.need_database && ' with PostgreSQL database'}
                  </p>
                </div>
              </div>

              <div className="flex gap-3 mt-8">
                <button
                  onClick={() => setShowCreateModal(false)}
                  disabled={creating}
                  className="flex-1 px-4 py-2.5 bg-[#252525] text-white font-medium rounded-lg hover:bg-[#333] disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                >
                  Cancel
                </button>
                <button
                  onClick={createProject}
                  disabled={!newProject.name.trim() || creating}
                  className="flex-1 px-4 py-2.5 bg-gradient-to-r from-purple-600 to-blue-600 text-white font-medium rounded-lg hover:from-purple-700 hover:to-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors text-sm"
                >
                  {creating ? 'Creating...' : 'Create Project'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}