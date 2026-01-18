import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import axios from 'axios';

const API_URL = '/api';

import { type Session } from '../types';

interface Project {
  id: string;
  name: string;
  description: string;
  external_ip: string;
  frontend_port: number;
  workspace_path: string;
  subdomain?: string;
}

export default function ProjectSessionsPage() {
  const { projectId } = useParams();
  const navigate = useNavigate();
  const [project, setProject] = useState<Project | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newSessionTitle, setNewSessionTitle] = useState('');

  useEffect(() => {
    loadProjectAndSessions();
  }, [projectId]);

  const loadProjectAndSessions = async () => {
    try {
      const token = localStorage.getItem('jwt_token');
      
      // Load project info
      const projectResponse = await axios.get(`${API_URL}/projects/${projectId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setProject(projectResponse.data);

      // Load sessions
      const sessionsResponse = await axios.get(`${API_URL}/projects/${projectId}/sessions`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setSessions(sessionsResponse.data || []);
    } catch (error) {
      console.error('Failed to load data:', error);
    } finally {
      setLoading(false);
    }
  };

  const createSession = async () => {
    if (!newSessionTitle.trim()) return;
    
    try {
      const token = localStorage.getItem('jwt_token');
      const response = await axios.post(`${API_URL}/sessions`, {
        project_id: projectId,
        title: newSessionTitle
      }, {
        headers: { Authorization: `Bearer ${token}` }
      });
      setShowCreateModal(false);
      setNewSessionTitle('');
      navigate(`/sessions/${response.data.id}`);
    } catch (error) {
      console.error('Failed to create session:', error);
    }
  };

  const selectSession = (sessionId: string) => {
    navigate(`/sessions/${sessionId}`);
  };

  if (loading) {
    return <div className="flex items-center justify-center h-screen">Loading...</div>;
  }

  return (
    <div className="min-h-screen bg-gray-50 p-8">
      <div className="max-w-6xl mx-auto">
        <div className="flex items-center gap-4 mb-6">
          <button
            onClick={() => navigate('/projects')}
            className="px-4 py-2 bg-gray-200 rounded-lg hover:bg-gray-300 transition"
          >
            ← Back
          </button>
        </div>

        {project && (
          <div className="bg-white rounded-lg shadow-sm p-6 mb-6">
            <h1 className="text-3xl font-bold text-gray-900 mb-2">{project.name}</h1>
            {project.description && (
              <p className="text-gray-600 mb-3">{project.description}</p>
            )}
            <div className="flex gap-4 text-sm text-gray-500">
              <span>📍 {project.subdomain || `${project.external_ip}:${project.frontend_port}`}</span>
              <span>📁 {project.workspace_path}</span>
            </div>
          </div>
        )}

        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold text-gray-900">Sessions</h2>
          <button
            onClick={() => setShowCreateModal(true)}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition"
          >
            + New Session
          </button>
        </div>

        <div className="space-y-3">
          {sessions.map(session => (
            <div
              key={session.id}
              onClick={() => selectSession(session.id)}
              className="bg-white p-6 rounded-lg shadow-sm hover:shadow-md cursor-pointer transition border border-gray-200"
            >
              <h3 className="text-lg font-semibold mb-2">{session.title}</h3>
              <div className="flex flex-col gap-2">
                <div className="flex gap-4 text-sm text-gray-500">
                  <span>💬 {session.message_count} messages</span>
                  <span>💰 ${session.cost?.toFixed(4) || '0.0000'}</span>
                  <span>📅 {new Date(session.created_at).toLocaleDateString()}</span>
                </div>
                {session.context_window > 0 && (
                  <div className="flex items-center gap-2 text-xs text-gray-400">
                    <span className="shrink-0 w-12">Context:</span>
                    <div className="flex-1 bg-gray-200 rounded-full h-1.5 overflow-hidden">
                      <div 
                        className={`h-1.5 rounded-full transition-all duration-500 ${
                            ((session.prompt_tokens + session.completion_tokens) / session.context_window) > 0.9 ? 'bg-red-500' :
                            ((session.prompt_tokens + session.completion_tokens) / session.context_window) > 0.7 ? 'bg-yellow-500' :
                            'bg-blue-500'
                        }`}
                        style={{ width: `${Math.min(100, ((session.prompt_tokens + session.completion_tokens) / session.context_window) * 100)}%` }}
                      />
                    </div>
                    <span className="shrink-0 w-12 text-right">{Math.round(((session.prompt_tokens + session.completion_tokens) / session.context_window) * 100)}%</span>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>

        {sessions.length === 0 && (
          <div className="text-center py-16 bg-white rounded-lg border-2 border-dashed border-gray-300">
            <p className="text-gray-500 text-lg mb-2">No sessions yet</p>
            <p className="text-gray-400 text-sm">Create your first session to get started!</p>
          </div>
        )}
      </div>

      {showCreateModal && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white p-8 rounded-lg w-96 shadow-xl">
            <h2 className="text-2xl font-bold mb-4">Create New Session</h2>
            <input
              type="text"
              placeholder="Session Title (e.g., Bug Fix, New Feature)"
              value={newSessionTitle}
              onChange={e => setNewSessionTitle(e.target.value)}
              onKeyPress={e => e.key === 'Enter' && createSession()}
              className="w-full px-4 py-2 border rounded-lg mb-4 focus:outline-none focus:ring-2 focus:ring-blue-500"
              autoFocus
            />
            <div className="flex gap-2">
              <button
                onClick={createSession}
                disabled={!newSessionTitle.trim()}
                className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition"
              >
                Create
              </button>
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewSessionTitle('');
                }}
                className="flex-1 px-4 py-2 bg-gray-200 rounded-lg hover:bg-gray-300 transition"
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

