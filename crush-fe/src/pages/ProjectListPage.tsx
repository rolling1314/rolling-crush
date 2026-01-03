import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { LogOut } from 'lucide-react';
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
          <h1 className="text-3xl font-bold text-gray-900">My Projects</h1>
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
              className="bg-white p-6 rounded-lg shadow hover:shadow-lg cursor-pointer transition"
            >
              <h3 className="text-xl font-semibold mb-2">{project.name}</h3>
              <p className="text-gray-600 text-sm mb-3">{project.description}</p>
              <div className="text-xs text-gray-500 space-y-1">
                <p><span className="font-medium">Address:</span> {project.host}:{project.port}</p>
                <p><span className="font-medium">Path:</span> {project.workspace_path}</p>
                <p className="text-gray-400 mt-2">
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
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center">
          <div className="bg-white p-8 rounded-lg w-[500px]">
            <h2 className="text-2xl font-bold mb-4">Create New Project</h2>
            <input
              type="text"
              placeholder="Project Name"
              value={newProject.name}
              onChange={e => setNewProject({ ...newProject, name: e.target.value })}
              className="w-full px-4 py-2 border rounded-lg mb-3"
            />
            <textarea
              placeholder="Description"
              value={newProject.description}
              onChange={e => setNewProject({ ...newProject, description: e.target.value })}
              className="w-full px-4 py-2 border rounded-lg mb-3"
              rows={2}
            />
            <div className="grid grid-cols-2 gap-3 mb-3">
              <input
                type="text"
                placeholder="Host (e.g., localhost)"
                value={newProject.host}
                onChange={e => setNewProject({ ...newProject, host: e.target.value })}
                className="px-4 py-2 border rounded-lg"
              />
              <input
                type="number"
                placeholder="Port"
                value={newProject.port}
                onChange={e => setNewProject({ ...newProject, port: parseInt(e.target.value) || 8080 })}
                className="px-4 py-2 border rounded-lg"
              />
            </div>
            <input
              type="text"
              placeholder="Workspace Path (e.g., /path/to/project)"
              value={newProject.workspace_path}
              onChange={e => setNewProject({ ...newProject, workspace_path: e.target.value })}
              className="w-full px-4 py-2 border rounded-lg mb-4"
            />
            <div className="flex gap-2">
              <button
                onClick={createProject}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
              >
                Create
              </button>
              <button
                onClick={() => setShowCreateModal(false)}
                className="flex-1 px-4 py-2 bg-gray-200 rounded-lg hover:bg-gray-300"
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

