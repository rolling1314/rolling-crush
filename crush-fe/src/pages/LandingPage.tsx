import React, { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Code, CornerDownLeft, AlertCircle, X, Mail, ArrowLeft } from 'lucide-react';

type AuthMode = 'login' | 'register' | 'verify' | 'forgot' | 'reset';

const LandingPage = () => {
  const navigate = useNavigate();
  const [stars, setStars] = useState<{ top: string; left: string; size: string; opacity: number }[]>([]);
  const emailInputRef = useRef<HTMLInputElement>(null);
  
  // Auth State
  const [mode, setMode] = useState<AuthMode>('login');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [username, setUsername] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [countdown, setCountdown] = useState(0);

  // Modal state
  const [showVerifyModal, setShowVerifyModal] = useState(false);
  const [verifyType, setVerifyType] = useState<'register' | 'reset_password'>('register');

  useEffect(() => {
    // Generate random stars
    const generateStars = () => {
      const starCount = 100;
      const newStars = Array.from({ length: starCount }).map(() => ({
        top: `${Math.random() * 100}%`,
        left: `${Math.random() * 100}%`,
        size: `${Math.random() * 2 + 1}px`,
        opacity: Math.random() * 0.7 + 0.3,
      }));
      setStars(newStars);
    };

    generateStars();
  }, []);

  // Countdown timer for resend code
  useEffect(() => {
    if (countdown > 0) {
      const timer = setTimeout(() => setCountdown(countdown - 1), 1000);
      return () => clearTimeout(timer);
    }
  }, [countdown]);

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        localStorage.setItem('jwt_token', data.token);
        localStorage.setItem('username', data.user.username);
        localStorage.setItem('user_id', data.user.id);
        localStorage.setItem('email', data.user.email);
        navigate('/projects');
      } else {
        setError(data.message || '登录失败，请检查邮箱和密码');
      }
    } catch (err) {
      console.error('Login error:', err);
      setError('无法连接到服务器，请检查网络连接');
    } finally {
      setIsLoading(false);
    }
  };

  const handleSendVerificationCode = async (type: 'register' | 'reset_password') => {
    if (!email) {
      setError('请输入邮箱地址');
      return;
    }

    setError('');
    setIsLoading(true);

    try {
      const response = await fetch('/api/auth/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, type }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        setVerifyType(type);
        setShowVerifyModal(true);
        setCountdown(60);
        setSuccess('验证码已发送到您的邮箱');
      } else {
        setError(data.error || data.message || '发送验证码失败');
      }
    } catch (err) {
      console.error('Send code error:', err);
      setError('无法连接到服务器');
    } finally {
      setIsLoading(false);
    }
  };

  const handleRegisterWithCode = async () => {
    if (!verificationCode) {
      setError('请输入验证码');
      return;
    }
    if (password !== confirmPassword) {
      setError('两次输入的密码不一致');
      return;
    }

    setError('');
    setIsLoading(true);

    try {
      const response = await fetch('/api/auth/register-with-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, email, password, code: verificationCode }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        localStorage.setItem('jwt_token', data.token);
        localStorage.setItem('username', data.user.username);
        localStorage.setItem('user_id', data.user.id);
        localStorage.setItem('email', data.user.email);
        setShowVerifyModal(false);
        navigate('/projects');
      } else {
        setError(data.error || data.message || '注册失败');
      }
    } catch (err) {
      console.error('Register error:', err);
      setError('无法连接到服务器');
    } finally {
      setIsLoading(false);
    }
  };

  const handleResetPassword = async () => {
    if (!verificationCode) {
      setError('请输入验证码');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('两次输入的密码不一致');
      return;
    }
    if (newPassword.length < 6) {
      setError('密码长度至少6位');
      return;
    }

    setError('');
    setIsLoading(true);

    try {
      const response = await fetch('/api/auth/reset-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, code: verificationCode, new_password: newPassword }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        setShowVerifyModal(false);
        setSuccess('密码重置成功，请使用新密码登录');
        setMode('login');
        setPassword('');
        setNewPassword('');
        setConfirmPassword('');
        setVerificationCode('');
      } else {
        setError(data.error || data.message || '重置密码失败');
      }
    } catch (err) {
      console.error('Reset password error:', err);
      setError('无法连接到服务器');
    } finally {
      setIsLoading(false);
    }
  };

  const handleTryEnterClick = () => {
    emailInputRef.current?.focus();
  };

  const handleGitHubLogin = async () => {
    try {
      const response = await fetch('/api/auth/github');
      const data = await response.json();
      if (data.auth_url) {
        window.location.href = data.auth_url;
      } else {
        setError('Failed to initialize GitHub login');
      }
    } catch (err) {
      console.error('GitHub login error:', err);
      setError('Failed to connect to server for GitHub login');
    }
  };

  const renderLoginForm = () => (
    <form onSubmit={handleLogin} className="space-y-4">
      {error && (
        <div className="flex items-center gap-2 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300">
          <AlertCircle size={16} />
          {error}
        </div>
      )}
      {success && (
        <div className="flex items-center gap-2 p-3 bg-emerald-900/20 border border-emerald-900/50 rounded-lg text-sm text-emerald-300">
          {success}
        </div>
      )}
      
      <div className="space-y-3">
        <input
          ref={emailInputRef}
          type="email"
          placeholder="输入邮箱地址"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
          required
        />
        <input
          type="password"
          placeholder="密码"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
          required
        />
      </div>

      <div className="flex justify-end">
        <button
          type="button"
          onClick={() => { setMode('forgot'); setError(''); setSuccess(''); }}
          className="text-sm text-gray-400 hover:text-cyan-400 transition-colors"
        >
          忘记密码？
        </button>
      </div>

      <button
        type="submit"
        disabled={isLoading}
        className="w-full py-2.5 bg-[#333] hover:bg-[#444] text-white rounded-lg font-medium transition-colors disabled:opacity-50"
      >
        {isLoading ? '登录中...' : '登录'}
      </button>

      <p className="text-center text-sm text-gray-500">
        还没有账号？{' '}
        <button
          type="button"
          onClick={() => { setMode('register'); setError(''); setSuccess(''); }}
          className="text-cyan-400 hover:text-cyan-300 transition-colors"
        >
          注册
        </button>
      </p>
    </form>
  );

  const renderRegisterForm = () => (
    <div className="space-y-4">
      {error && (
        <div className="flex items-center gap-2 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300">
          <AlertCircle size={16} />
          {error}
        </div>
      )}
      
      <div className="space-y-3">
        <input
          type="text"
          placeholder="用户名"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
          required
        />
        <input
          type="email"
          placeholder="邮箱地址"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
          required
        />
        <input
          type="password"
          placeholder="密码（至少6位）"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
          required
          minLength={6}
        />
        <input
          type="password"
          placeholder="确认密码"
          value={confirmPassword}
          onChange={(e) => setConfirmPassword(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
          required
          minLength={6}
        />
      </div>

      <button
        type="button"
        onClick={() => {
          if (!username || !email || !password || !confirmPassword) {
            setError('请填写所有字段');
            return;
          }
          if (password !== confirmPassword) {
            setError('两次输入的密码不一致');
            return;
          }
          if (password.length < 6) {
            setError('密码长度至少6位');
            return;
          }
          handleSendVerificationCode('register');
        }}
        disabled={isLoading}
        className="w-full py-2.5 bg-gradient-to-r from-cyan-600 to-teal-600 hover:from-cyan-500 hover:to-teal-500 text-white rounded-lg font-medium transition-all disabled:opacity-50"
      >
        {isLoading ? '发送中...' : '获取验证码'}
      </button>

      <p className="text-center text-sm text-gray-500">
        已有账号？{' '}
        <button
          type="button"
          onClick={() => { setMode('login'); setError(''); setSuccess(''); }}
          className="text-cyan-400 hover:text-cyan-300 transition-colors"
        >
          登录
        </button>
      </p>
    </div>
  );

  const renderForgotPasswordForm = () => (
    <div className="space-y-4">
      <button
        type="button"
        onClick={() => { setMode('login'); setError(''); setSuccess(''); }}
        className="flex items-center gap-2 text-gray-400 hover:text-white transition-colors text-sm"
      >
        <ArrowLeft size={16} />
        返回登录
      </button>

      {error && (
        <div className="flex items-center gap-2 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300">
          <AlertCircle size={16} />
          {error}
        </div>
      )}
      
      <div className="space-y-3">
        <input
          type="email"
          placeholder="输入注册邮箱"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-pink-500/50 focus:ring-1 focus:ring-pink-500/50 transition-all"
          required
        />
      </div>

      <button
        type="button"
        onClick={() => handleSendVerificationCode('reset_password')}
        disabled={isLoading}
        className="w-full py-2.5 bg-gradient-to-r from-pink-600 to-rose-600 hover:from-pink-500 hover:to-rose-500 text-white rounded-lg font-medium transition-all disabled:opacity-50"
      >
        {isLoading ? '发送中...' : '发送重置验证码'}
      </button>
    </div>
  );

  const renderVerificationModal = () => (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
      <div className="w-full max-w-md mx-4 bg-gradient-to-b from-[#0f1419] to-[#0a0a0a] rounded-2xl border border-white/10 overflow-hidden animate-fade-in-up">
        {/* Header */}
        <div className={`p-6 text-center ${verifyType === 'register' ? 'bg-gradient-to-r from-cyan-900/30 to-teal-900/30' : 'bg-gradient-to-r from-pink-900/30 to-rose-900/30'}`}>
          <button
            onClick={() => { setShowVerifyModal(false); setVerificationCode(''); setError(''); }}
            className="absolute top-4 right-4 text-gray-400 hover:text-white transition-colors"
          >
            <X size={20} />
          </button>
          <h2 className={`text-xl font-semibold ${verifyType === 'register' ? 'text-cyan-400' : 'text-pink-400'}`}>
            {verifyType === 'register' ? '欢迎注册' : '密码重置'}
          </h2>
        </div>

        {/* Content */}
        <div className="p-6 space-y-4">
          <p className="text-gray-400 text-sm">
            尊敬的用户：
          </p>
          <p className="text-gray-400 text-sm">
            {verifyType === 'register' 
              ? '您正在进行注册操作，请输入以下验证码完成验证：' 
              : '您正在进行密码重置操作，请输入验证码：'}
          </p>

          {error && (
            <div className="flex items-center gap-2 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300">
              <AlertCircle size={16} />
              {error}
            </div>
          )}

          {/* Verification code input */}
          <div className={`p-4 rounded-xl border ${verifyType === 'register' ? 'bg-cyan-500/5 border-cyan-500/20' : 'bg-pink-500/5 border-pink-500/20'}`}>
            <input
              type="text"
              value={verificationCode}
              onChange={(e) => setVerificationCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              placeholder="输入6位验证码"
              className={`w-full text-center text-3xl font-mono tracking-[0.5em] bg-transparent border-none outline-none ${verifyType === 'register' ? 'text-cyan-400' : 'text-pink-400'} placeholder-gray-600`}
              maxLength={6}
            />
          </div>

          {/* Password fields for reset */}
          {verifyType === 'reset_password' && (
            <div className="space-y-3">
              <input
                type="password"
                placeholder="新密码（至少6位）"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-pink-500/50 focus:ring-1 focus:ring-pink-500/50 transition-all"
                minLength={6}
              />
              <input
                type="password"
                placeholder="确认新密码"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-pink-500/50 focus:ring-1 focus:ring-pink-500/50 transition-all"
                minLength={6}
              />
            </div>
          )}

          {/* Tips */}
          <div className="bg-white/5 rounded-lg p-4">
            <p className={`text-sm font-medium mb-2 ${verifyType === 'register' ? 'text-cyan-400' : 'text-pink-400'}`}>
              安全提示：
            </p>
            <ul className="text-xs text-gray-500 space-y-1 list-disc list-inside">
              <li>验证码有效期为5分钟</li>
              <li>请勿将验证码泄露给他人</li>
              <li>如非本人操作，请忽略此邮件</li>
            </ul>
          </div>

          {/* Actions */}
          <div className="flex gap-3">
            <button
              onClick={() => handleSendVerificationCode(verifyType)}
              disabled={countdown > 0 || isLoading}
              className="flex-1 py-2.5 bg-[#222] hover:bg-[#333] text-gray-300 rounded-lg font-medium transition-colors disabled:opacity-50"
            >
              {countdown > 0 ? `${countdown}秒后重发` : '重新发送'}
            </button>
            <button
              onClick={verifyType === 'register' ? handleRegisterWithCode : handleResetPassword}
              disabled={isLoading || verificationCode.length !== 6}
              className={`flex-1 py-2.5 text-white rounded-lg font-medium transition-all disabled:opacity-50 ${
                verifyType === 'register' 
                  ? 'bg-gradient-to-r from-cyan-600 to-teal-600 hover:from-cyan-500 hover:to-teal-500' 
                  : 'bg-gradient-to-r from-pink-600 to-rose-600 hover:from-pink-500 hover:to-rose-500'
              }`}
            >
              {isLoading ? '处理中...' : '确认'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );

  return (
    <div className="min-h-screen bg-[#050505] text-white overflow-hidden relative flex flex-col font-sans selection:bg-white/20">
      {/* Background Effects */}
      <div className="absolute inset-0 z-0 pointer-events-none">
        {/* Top center light source */}
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[1000px] h-[500px] bg-white/[0.03] blur-[100px] rounded-full" />
        
        {/* Stars */}
        {stars.map((star, i) => (
          <div
            key={i}
            className="absolute bg-white rounded-full animate-pulse"
            style={{
              top: star.top,
              left: star.left,
              width: star.size,
              height: star.size,
              opacity: star.opacity,
              animationDuration: `${Math.random() * 3 + 2}s`,
            }}
          />
        ))}
      </div>

      {/* Navigation */}
      <nav className="relative z-10 flex items-center justify-between px-6 py-4 md:px-12">
        <div className="flex items-center gap-2 cursor-pointer" onClick={() => navigate('/')}>
          <Code className="w-6 h-6 text-white" />
          <span className="font-serif font-semibold text-xl tracking-tight">Enter</span>
        </div>

        <div className="flex items-center gap-6">
          <a href="#" className="hidden md:block text-sm text-gray-400 hover:text-white transition-colors">Platform</a>
          <a href="#" className="hidden md:block text-sm text-gray-400 hover:text-white transition-colors">Solutions</a>
          <a href="#" className="hidden md:block text-sm text-gray-400 hover:text-white transition-colors">Pricing</a>
          <a
            href="https://discord.com" 
            target="_blank" 
            rel="noopener noreferrer"
            className="hidden md:flex items-center gap-2 text-sm text-gray-400 hover:text-white transition-colors"
          >
            Discord
          </a>
          <button
            onClick={handleTryEnterClick}
            className="px-4 py-2 text-sm font-medium bg-white text-black rounded-lg hover:bg-gray-200 transition-colors"
          >
            Try Enter
          </button>
        </div>
      </nav>

      {/* Main Content */}
      <main className="relative z-10 flex flex-col md:flex-row flex-1 px-6 md:px-12 pt-12 md:pt-24 gap-12 max-w-7xl mx-auto w-full">
        {/* Left Column: Text + Login */}
        <div className="flex-1 flex flex-col justify-start md:max-w-lg z-20">
          <h1 className="text-6xl md:text-7xl font-serif font-medium tracking-tight leading-[1.1] mb-6 animate-fade-in-up">
            <span className="block">Limitless?</span>
            <span className="block">Unleashed.</span>
          </h1>
          
          <p className="text-xl text-gray-400 mb-12 font-serif animate-fade-in-up delay-100">
            The engine for innovators
          </p>

          <div className="w-full max-w-sm bg-[#111111] p-6 rounded-2xl border border-white/10 shadow-2xl animate-fade-in-up delay-200">
            <button 
              onClick={handleGitHubLogin}
              className="w-full flex items-center justify-center gap-2 bg-white text-black py-2.5 rounded-lg font-medium hover:bg-gray-100 transition-colors mb-4"
            >
               <svg className="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
              </svg>
              Continue with GitHub
            </button>

            <div className="relative my-6 text-center">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-white/10"></div>
              </div>
              <span className="relative px-2 bg-[#111111] text-xs text-gray-500 uppercase">OR</span>
            </div>

            {mode === 'login' && renderLoginForm()}
            {mode === 'register' && renderRegisterForm()}
            {mode === 'forgot' && renderForgotPasswordForm()}
          </div>

          <div className="mt-6 text-xs text-gray-500 max-w-sm">
             By continuing, you acknowledge Enter's <a href="#" className="underline decoration-gray-700 hover:text-gray-400">Privacy Policy</a>.
          </div>
        </div>

        {/* Right Column: Visual */}
        <div className="flex-1 relative hidden md:block">
           <div className="absolute top-0 right-0 w-full h-[600px] bg-gradient-to-br from-purple-500/10 via-blue-500/5 to-transparent rounded-2xl border border-white/5 backdrop-blur-sm overflow-hidden animate-fade-in-up delay-300">
              {/* Abstract Code/Terminal Visual */}
              <div className="absolute inset-0 p-8 font-mono text-sm leading-relaxed text-gray-400 opacity-60 pointer-events-none select-none">
                <div className="text-purple-400 mb-4">// System Initialization</div>
                <div className="text-green-400">$ connect --secure</div>
                <div className="mb-2">Connecting to neural interface... <span className="text-blue-400">OK</span></div>
                <div className="mb-2">Loading cognitive modules... <span className="text-blue-400">OK</span></div>
                <div className="text-yellow-400 mb-4">Warning: High computational load detected</div>
                <br />
                <div className="text-purple-400 mb-2">import {`{ Universe }`} from 'reality';</div>
                <div className="mb-2">const user = await Universe.connect(credentials);</div>
                <div className="mb-2">if (user.isReady) {`{`}</div>
                <div className="pl-4 mb-2 text-blue-300">user.empower();</div>
                <div className="pl-4 mb-2 text-blue-300">return "Impossible is nothing";</div>
                <div>{`}`}</div>
                
                {/* Visual Glitch/Cursor */}
                <div className="absolute bottom-20 right-20 w-32 h-32 bg-blue-500/20 blur-3xl rounded-full animate-pulse"></div>
              </div>
           </div>
           
           {/* Floating elements */}
           <div className="absolute top-20 right-[-20px] bg-[#1a1a1a] p-4 rounded-xl border border-white/10 shadow-xl animate-float delay-100">
             <Code className="w-8 h-8 text-blue-400" />
           </div>
           <div className="absolute bottom-40 left-10 bg-[#1a1a1a] p-4 rounded-xl border border-white/10 shadow-xl animate-float delay-700">
             <CornerDownLeft className="w-6 h-6 text-purple-400" />
           </div>
        </div>
      </main>

      {/* Verification Modal */}
      {showVerifyModal && renderVerificationModal()}
    </div>
  );
};

export default LandingPage;
