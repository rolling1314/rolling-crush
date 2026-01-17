import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, AlertCircle, X } from 'lucide-react';

export default function RegisterPage() {
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [verificationCode, setVerificationCode] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [showVerifyModal, setShowVerifyModal] = useState(false);
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

    setIsLoading(true);

    try {
      const response = await fetch('/api/auth/send-code', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, type: 'register' }),
      });

      const data = await response.json();

      if (response.ok && data.success) {
        setShowVerifyModal(true);
        setCountdown(60);
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

  const handleRegister = async () => {
    if (!verificationCode) {
      setError('请输入验证码');
      return;
    }

    setError('');
    setIsLoading(true);

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
        setError(data.error || data.message || '注册失败');
      }
    } catch (err: any) {
      setError(err.response?.data?.error || '注册失败');
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#050505] relative overflow-hidden">
      {/* Background gradient */}
      <div className="absolute inset-0 pointer-events-none">
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[800px] h-[400px] bg-cyan-500/5 blur-[100px] rounded-full" />
        <div className="absolute bottom-0 right-0 w-[600px] h-[300px] bg-purple-500/5 blur-[100px] rounded-full" />
      </div>

      <div className="relative bg-[#111111] p-8 rounded-2xl border border-white/10 shadow-2xl w-full max-w-md mx-4">
        <button
          onClick={() => navigate('/')}
          className="flex items-center gap-2 text-gray-400 hover:text-white mb-6 text-sm transition-colors"
        >
          <ArrowLeft size={16} />
          返回登录
        </button>

        <h1 className="text-2xl font-bold mb-6 text-center text-white">注册账号</h1>

        {error && (
          <div className="mb-4 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300 flex items-center gap-2">
            <AlertCircle size={16} />
            {error}
          </div>
        )}

        <div className="space-y-4">
          <input
            type="text"
            placeholder="用户名"
            value={username}
            onChange={e => setUsername(e.target.value)}
            className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
            required
          />
          <input
            type="email"
            placeholder="邮箱地址"
            value={email}
            onChange={e => setEmail(e.target.value)}
            className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
            required
          />
          <input
            type="password"
            placeholder="密码（至少6位）"
            value={password}
            onChange={e => setPassword(e.target.value)}
            className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
            required
            minLength={6}
          />
          <input
            type="password"
            placeholder="确认密码"
            value={confirmPassword}
            onChange={e => setConfirmPassword(e.target.value)}
            className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-cyan-500/50 focus:ring-1 focus:ring-cyan-500/50 transition-all"
            required
            minLength={6}
          />
          <button
            type="button"
            onClick={handleSendCode}
            disabled={isLoading}
            className="w-full py-2.5 bg-gradient-to-r from-cyan-600 to-teal-600 hover:from-cyan-500 hover:to-teal-500 text-white rounded-lg font-medium transition-all disabled:opacity-50"
          >
            {isLoading ? '发送中...' : '获取验证码'}
          </button>
        </div>

        <p className="mt-6 text-center text-sm text-gray-500">
          已有账号？{' '}
          <button
            onClick={() => navigate('/')}
            className="text-cyan-400 hover:text-cyan-300 transition-colors"
          >
            立即登录
          </button>
        </p>
      </div>

      {/* Verification Modal */}
      {showVerifyModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-md mx-4 bg-gradient-to-b from-[#0f1419] to-[#0a0a0a] rounded-2xl border border-white/10 overflow-hidden animate-fade-in-up">
            {/* Header */}
            <div className="p-6 text-center bg-gradient-to-r from-cyan-900/30 to-teal-900/30 relative">
              <button
                onClick={() => { setShowVerifyModal(false); setVerificationCode(''); setError(''); }}
                className="absolute top-4 right-4 text-gray-400 hover:text-white transition-colors"
              >
                <X size={20} />
              </button>
              <h2 className="text-xl font-semibold text-cyan-400">欢迎注册</h2>
            </div>

            {/* Content */}
            <div className="p-6 space-y-4">
              <p className="text-gray-400 text-sm">尊敬的用户：</p>
              <p className="text-gray-400 text-sm">
                您正在进行注册操作，请输入以下验证码完成验证：
              </p>

              {error && (
                <div className="flex items-center gap-2 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300">
                  <AlertCircle size={16} />
                  {error}
                </div>
              )}

              {/* Verification code input */}
              <div className="p-4 rounded-xl border bg-cyan-500/5 border-cyan-500/20">
                <input
                  type="text"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  placeholder="输入6位验证码"
                  className="w-full text-center text-3xl font-mono tracking-[0.5em] bg-transparent border-none outline-none text-cyan-400 placeholder-gray-600"
                  maxLength={6}
                />
              </div>

              {/* Tips */}
              <div className="bg-white/5 rounded-lg p-4">
                <p className="text-sm font-medium mb-2 text-cyan-400">安全提示：</p>
                <ul className="text-xs text-gray-500 space-y-1 list-disc list-inside">
                  <li>验证码有效期为5分钟</li>
                  <li>请勿将验证码泄露给他人</li>
                  <li>如非本人操作，请忽略此邮件</li>
                </ul>
              </div>

              {/* Actions */}
              <div className="flex gap-3">
                <button
                  onClick={handleSendCode}
                  disabled={countdown > 0 || isLoading}
                  className="flex-1 py-2.5 bg-[#222] hover:bg-[#333] text-gray-300 rounded-lg font-medium transition-colors disabled:opacity-50"
                >
                  {countdown > 0 ? `${countdown}秒后重发` : '重新发送'}
                </button>
                <button
                  onClick={handleRegister}
                  disabled={isLoading || verificationCode.length !== 6}
                  className="flex-1 py-2.5 bg-gradient-to-r from-cyan-600 to-teal-600 hover:from-cyan-500 hover:to-teal-500 text-white rounded-lg font-medium transition-all disabled:opacity-50"
                >
                  {isLoading ? '处理中...' : '确认注册'}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
