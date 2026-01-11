import React, { useEffect, useState, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import { Code, ArrowRight, CornerDownLeft, AlertCircle } from 'lucide-react';

const LandingPage = () => {
  const navigate = useNavigate();
  const [stars, setStars] = useState<{ top: string; left: string; size: string; opacity: number }[]>([]);
  const emailInputRef = useRef<HTMLInputElement>(null);
  
  // Login State
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

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

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);

    try {
      const response = await fetch('http://localhost:8081/api/auth/login', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
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
        setError(data.message || 'Login failed. Please try again.');
      }
    } catch (err) {
      console.error('Login error:', err);
      setError('Unable to connect to server. Please check if the server is running.');
    } finally {
      setIsLoading(false);
    }
  };

  const handleTryEnterClick = () => {
    emailInputRef.current?.focus();
  };

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
            <span className="block">Impossible?</span>
            <span className="block">Possible.</span>
          </h1>
          
          <p className="text-xl text-gray-400 mb-12 font-serif animate-fade-in-up delay-100">
            The AI for problem solvers
          </p>

          <div className="w-full max-w-sm bg-[#111111] p-6 rounded-2xl border border-white/10 shadow-2xl animate-fade-in-up delay-200">
            <button className="w-full flex items-center justify-center gap-2 bg-white text-black py-2.5 rounded-lg font-medium hover:bg-gray-100 transition-colors mb-4">
               <svg className="w-5 h-5" viewBox="0 0 24 24">
                <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
                <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
                <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
                <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
              </svg>
              Continue with Google
            </button>

            <div className="relative my-6 text-center">
              <div className="absolute inset-0 flex items-center">
                <div className="w-full border-t border-white/10"></div>
              </div>
              <span className="relative px-2 bg-[#111111] text-xs text-gray-500 uppercase">OR</span>
            </div>

            <form onSubmit={handleLogin} className="space-y-4">
              {error && (
                <div className="flex items-center gap-2 p-3 bg-red-900/20 border border-red-900/50 rounded-lg text-sm text-red-300">
                  <AlertCircle size={16} />
                  {error}
                </div>
              )}
              
              <div className="space-y-3">
                 <input
                  ref={emailInputRef}
                  type="email"
                  placeholder="Enter your email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/30 transition-all"
                  required
                />
                <input
                  type="password"
                  placeholder="Password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="w-full px-4 py-2.5 bg-[#1a1a1a] border border-white/10 rounded-lg text-white placeholder-gray-500 focus:outline-none focus:border-white/30 focus:ring-1 focus:ring-white/30 transition-all"
                  required
                />
              </div>

              <button
                type="submit"
                disabled={isLoading}
                className="w-full py-2.5 bg-[#333] hover:bg-[#444] text-white rounded-lg font-medium transition-colors disabled:opacity-50"
              >
                {isLoading ? 'Signing in...' : 'Continue with email'}
              </button>
            </form>
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
    </div>
  );
};

export default LandingPage;
