import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, AlertCircle } from 'lucide-react';

export default function RegisterPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [codeSent, setCodeSent] = useState(false);
  const [countdown, setCountdown] = useState(0);
  const navigate = useNavigate();

  // Countdown timer for resend code
  useEffect(() => {
    if (countdown > 0) {
      const timer = setTimeout(() => setCountdown(countdown - 1), 1000);
      return () => clearTimeout(timer);
    }
  }, [countdown]);

  const handleSendCode = async () => {
    setError('');

    if (!email) {
      setError('Please enter your email');
      return;
    }

    if (!password || password.length < 6) {
      setError('Password must be at least 6 characters');
      return;
    }

    setIsLoading(true);

    try {
      const response = await fetch('/api/auth/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, type: 'register' }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        setCodeSent(true);
        setCountdown(60);
        setSuccess('Verification code sent to your email');
      } else {
        setError(data.error || data.message || 'Failed to send verification code');
      }
    } catch (err) {
      console.error('Send code error:', err);
      setError('Unable to connect to server');
    } finally {
      setIsLoading(false);
    }
  };

  const handleRegister = async () => {
    if (!verificationCode) {
      setError('Please enter verification code');
      return;
    }

    setError('');
    setIsLoading(true);

    // Use email prefix as username
    const username = email.split('@')[0];

    try {
      const response = await fetch('/api/auth/register-with-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          username,
          email,
          password,
          code: verificationCode,
        }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        localStorage.setItem('jwt_token', data.token);
        localStorage.setItem('username', data.user.username);
        localStorage.setItem('user_id', data.user.id);
        localStorage.setItem('email', data.user.email);
        navigate('/projects');
      } else {
        setError(data.error || data.message || 'Registration failed');
      }
    } catch (err: any) {
      setError(err.response?.data?.error || 'Registration failed');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#050505] relative overflow-hidden">
      {/* Background gradient */}
      <div className="absolute inset-0 pointer-events-none">
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[400px] bg-white/[0.02] blur-[100px] rounded-full" />
      </div>

      <div className="relative bg-[#111111] p-8 rounded-2xl border border-white/10 shadow-2xl w-full max-w-md mx-4">
        <button
          onClick={() => navigate('/')}
          className="flex items-center gap-2 text-gray-400 hover:text-white mb-6 text-sm transition-colors"
        >
          <ArrowLeft size={16} />
          Back to login
        </button>

        <h1 className="text-2xl font-bold mb-6 text-center text-white">Create account</h1>

        {error && (
          <div className="mb-4 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300 flex items-center gap-2">
            <AlertCircle size={16} />
            {error}
          </div>
        )}

        {success && (
          <div className="mb-4 p-3 bg-green-900/20 border border-green-900/50 rounded-lg text-sm text-green-300">
            {success}
          </div>
        )}

        <div className="space-y-4">
          <input
            type="email"
            placeholder="Enter your email"
            value={email}
            onChange={e => setEmail(e.target.value)}
            className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/30 transition-all"
            required
            disabled={codeSent}
          />
          <input
            type="password"
            placeholder="Password (min 6 chars)"
            value={password}
            onChange={e => setPassword(e.target.value)}
            className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/30 transition-all"
            required
            minLength={6}
            disabled={codeSent}
          />

          {!codeSent ? (
            <button
              type="button"
              onClick={handleSendCode}
              disabled={isLoading}
              className="w-full py-2.5 bg-[#333] hover:bg-[#444] text-white rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {isLoading ? 'Sending...' : 'Get verification code'}
            </button>
          ) : (
            <>
              <div className="flex gap-2">
                <input
                  type="text"
                  placeholder="Enter 6-digit code"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  className="flex-1 px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/30 transition-all text-center tracking-widest"
                  maxLength={6}
                />
                <button
                  type="button"
                  onClick={handleSendCode}
                  disabled={countdown > 0 || isLoading}
                  className="px-4 py-2.5 bg-[#222] hover:bg-[#333] text-gray-400 rounded-lg text-sm transition-colors disabled:opacity-50 whitespace-nowrap"
                >
                  {countdown > 0 ? `${countdown}s` : 'Resend'}
                </button>
              </div>
              <button
                type="button"
                onClick={handleRegister}
                disabled={isLoading || verificationCode.length !== 6}
                className="w-full py-2.5 bg-[#333] hover:bg-[#444] text-white rounded-lg font-medium transition-colors disabled:opacity-50"
              >
                {isLoading ? 'Creating account...' : 'Create account'}
              </button>
            </>
          )}
        </div>

        <p className="mt-6 text-center text-sm text-gray-500">
          Already have an account?{' '}
          <button
            onClick={() => navigate('/')}
            className="text-white hover:text-gray-300 transition-colors"
          >
            Sign in
          </button>
        </p>
      </div>
    </div>
  );
}
