import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { LogOut, X } from 'lucide-react';
import axios from 'axios';

const API_URL = 'http://localhost:8081/api';

interface Project {
  id: string;
  name: string;
  description: string;
  host: string;
  port: number;
  workspace_path: string;
  created_at: number;
  updated_at: number;
}

export default function ProjectListPage() {
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newProject, setNewProject] = useState({ 
    name: '', 
    description: '', 
    host: 'localhost', 
    port: 8080, 
    workspace_path: '.' 
  });
  const navigate = useNavigate();

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
    
    try {
      const token = localStorage.getItem('jwt_token');
      await axios.post(`${API_URL}/projects`, newProject, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setShowCreateModal(false);
      setNewProject({ name: '', description: '', host: 'localhost', port: 8080, workspace_path: '.' });
      loadProjects();
    } catch (error) {
      console.error('Failed to create project:', error);
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
    return <div className="flex items-center justify-center h-screen">Loading...</div>;
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex justify-between items-center mb-8">
          <h1 className="text-4xl font-bold text-gray-900">My Projects</h1>
          <div className="flex gap-3">
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
            >
              New Project
            </button>
            <button
              onClick={handleLogout}
              className="px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 flex items-center gap-2"
              title="Logout"
            >
              <LogOut size={18} />
              <span>Logout</span>
            </button>
          </div>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {projects.map(project => (
            <div
              key={project.id}
              onClick={() => selectProject(project.id)}
              className="bg-white p-6 rounded-lg shadow-md hover:shadow-xl cursor-pointer transition-all duration-200 border border-gray-200 hover:border-blue-300"
            >
              <h3 className="text-2xl font-bold text-gray-900 mb-3 leading-tight">{project.name}</h3>
              {project.description && (
                <p className="text-gray-700 text-base mb-4 font-medium">{project.description}</p>
              )}
              <div className="text-sm text-gray-600 space-y-2 border-t border-gray-100 pt-3">
                <p><span className="font-semibold text-gray-800">Address:</span> <span className="text-gray-700">{project.host}:{project.port}</span></p>
                <p><span className="font-semibold text-gray-800">Path:</span> <span className="text-gray-700 font-mono text-xs">{project.workspace_path}</span></p>
                <p className="text-gray-500 text-xs mt-3">
                  Created {new Date(project.created_at).toLocaleDateString()}
                </p>
              </div>
            </div>
          ))}
        </div>

        {projects.length === 0 && (
          <div className="text-center py-12">
            <p className="text-gray-500">No projects yet. Create your first project!</p>
          </div>
        )}
      </div>

      {showCreateModal && (
        <div 
          className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
          onClick={(e) => {
            if (e.target === e.currentTarget) {
              setShowCreateModal(false);
            }
          }}
        >
          <div className="bg-white rounded-xl shadow-2xl w-full max-w-[600px] max-h-[90vh] overflow-y-auto">
            {/* Header */}
            <div className="flex items-center justify-between p-6 border-b border-gray-200">
              <h2 className="text-2xl font-bold text-gray-900">Create New Project</h2>
              <button
                onClick={() => setShowCreateModal(false)}
                className="p-2 hover:bg-gray-100 rounded-lg transition-colors"
                aria-label="Close"
              >
                <X size={20} className="text-gray-500" />
              </button>
            </div>

            {/* Form Content */}
            <div className="p-6 space-y-5">
              {/* Project Name */}
              <div>
                <label htmlFor="project-name" className="block text-sm font-semibold text-gray-700 mb-2">
                  Project Name <span className="text-red-500">*</span>
                </label>
                <input
                  id="project-name"
                  type="text"
                  placeholder="Enter project name"
                  value={newProject.name}
                  onChange={e => setNewProject({ ...newProject, name: e.target.value })}
                  className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-blue-500 focus:ring-2 focus:ring-blue-200 outline-none transition-all text-gray-900 placeholder-gray-400"
                  autoFocus
                />
              </div>

              {/* Description */}
              <div>
                <label htmlFor="project-description" className="block text-sm font-semibold text-gray-700 mb-2">
                  Description
                </label>
                <textarea
                  id="project-description"
                  placeholder="Enter project description (optional)"
                  value={newProject.description}
                  onChange={e => setNewProject({ ...newProject, description: e.target.value })}
                  className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-blue-500 focus:ring-2 focus:ring-blue-200 outline-none transition-all resize-none text-gray-900 placeholder-gray-400"
                  rows={3}
                />
              </div>

              {/* Host and Port */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label htmlFor="project-host" className="block text-sm font-semibold text-gray-700 mb-2">
                    Host
                  </label>
                  <input
                    id="project-host"
                    type="text"
                    placeholder="localhost"
                    value={newProject.host}
                    onChange={e => setNewProject({ ...newProject, host: e.target.value })}
                    className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-blue-500 focus:ring-2 focus:ring-blue-200 outline-none transition-all text-gray-900 placeholder-gray-400"
                  />
                </div>
                <div>
                  <label htmlFor="project-port" className="block text-sm font-semibold text-gray-700 mb-2">
                    Port
                  </label>
                  <input
                    id="project-port"
                    type="number"
                    placeholder="8080"
                    value={newProject.port}
                    onChange={e => setNewProject({ ...newProject, port: parseInt(e.target.value) || 8080 })}
                    className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-blue-500 focus:ring-2 focus:ring-blue-200 outline-none transition-all text-gray-900 placeholder-gray-400"
                  />
                </div>
              </div>

              {/* Workspace Path */}
              <div>
                <label htmlFor="project-path" className="block text-sm font-semibold text-gray-700 mb-2">
                  Workspace Path
                </label>
                <input
                  id="project-path"
                  type="text"
                  placeholder="/path/to/project or . for current directory"
                  value={newProject.workspace_path}
                  onChange={e => setNewProject({ ...newProject, workspace_path: e.target.value })}
                  className="w-full px-4 py-3 border-2 border-gray-300 rounded-lg focus:border-blue-500 focus:ring-2 focus:ring-blue-200 outline-none transition-all font-mono text-sm text-gray-900 placeholder-gray-400"
                />
              </div>
            </div>

            {/* Footer */}
            <div className="flex gap-3 p-6 border-t border-gray-200 bg-gray-50 rounded-b-xl">
              <button
                onClick={createProject}
                disabled={!newProject.name.trim()}
                className="flex-1 px-6 py-3 bg-blue-600 text-white font-semibold rounded-lg hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors shadow-sm"
              >
                Create Project
              </button>
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-6 py-3 bg-white text-gray-700 font-semibold rounded-lg hover:bg-gray-100 border-2 border-gray-300 transition-colors"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

