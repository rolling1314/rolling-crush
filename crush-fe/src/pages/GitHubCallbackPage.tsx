import React, { useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Code, Loader2, AlertCircle } from 'lucide-react';

const GitHubCallbackPage = () => {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    const handleCallback = async () => {
      // Check for error from backend redirect
      const errorParam = searchParams.get('error');
      if (errorParam) {
        const errorMessages: Record<string, string> = {
          'missing_code': 'No authorization code received from GitHub',
          'invalid_state': 'Invalid state parameter - possible CSRF attack',
          'exchange_failed': 'Failed to exchange authorization code',
          'user_info_failed': 'Failed to get user info from GitHub',
          'create_user_failed': 'Failed to create user account',
          'token_failed': 'Failed to generate authentication token',
        };
        setError(errorMessages[errorParam] || `GitHub authorization failed: ${errorParam}`);
        setIsLoading(false);
        return;
      }

      // Check for token from backend redirect (successful auth)
      const token = searchParams.get('token');
      const userId = searchParams.get('user_id');
      const username = searchParams.get('username');
      const email = searchParams.get('email');
      const avatarUrl = searchParams.get('avatar_url');

      if (token && userId && username && email) {
        // Save auth data to localStorage
        localStorage.setItem('jwt_token', token);
        localStorage.setItem('username', username);
        localStorage.setItem('user_id', userId);
        localStorage.setItem('email', email);
        if (avatarUrl) {
          localStorage.setItem('avatar_url', avatarUrl);
        }
        
        // Redirect to projects page
        navigate('/projects');
        return;
      }

      // If no token and no error, something went wrong
      setError('Invalid callback - no authentication data received');
      setIsLoading(false);
    };

    handleCallback();
  }, [searchParams, navigate]);

  return (
    <div className="min-h-screen bg-[#050505] text-white flex items-center justify-center">
      <div className="text-center">
        <div className="flex items-center justify-center gap-2 mb-8">
          <Code className="w-8 h-8 text-white" />
          <span className="font-serif font-semibold text-2xl tracking-tight">Enter</span>
        </div>

        {isLoading ? (
          <div className="flex flex-col items-center gap-4">
            <Loader2 className="w-8 h-8 animate-spin text-white" />
            <p className="text-gray-400">Completing GitHub authentication...</p>
          </div>
        ) : error ? (
          <div className="max-w-md">
            <div className="flex items-center gap-2 p-4 bg-red-900/20 border border-red-900/50 rounded-lg text-red-300 mb-6">
              <AlertCircle size={20} />
              <span>{error}</span>
            </div>
            <button
              onClick={() => navigate('/')}
              className="px-6 py-2.5 bg-white text-black rounded-lg font-medium hover:bg-gray-100 transition-colors"
            >
              Back to Login
            </button>
          </div>
        ) : null}
      </div>
    </div>
  );
};

export default GitHubCallbackPage;
