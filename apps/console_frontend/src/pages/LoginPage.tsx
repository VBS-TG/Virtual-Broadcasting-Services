import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import kvLogoImg from '../assets/images/vbs kv-logo.png'

export default function LoginPage() {
  const [token, setToken] = useState('')
  const [showToken, setShowToken] = useState(false)
  const [loading, setLoading] = useState(false)
  const login = useAuthStore((s) => s.login)
  const navigate = useNavigate()

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    // [MOCK] 跳過 token 驗證，直接登入
    // TODO: 後端就緒後加入 Bearer Token 格式驗證與 JWT 到期時間檢查
    await new Promise((r) => setTimeout(r, 500))
    login(token || 'mock-dev-token')
    navigate('/dashboard')
  }

  return (
    <>
      <style>{`
        @keyframes flowMesh {
          0% { background-position: 0% 50%; }
          33% { background-position: 100% 100%; }
          66% { background-position: 50% 0%; }
          100% { background-position: 0% 50%; }
        }
        .bg-mesh-flow {
          background: linear-gradient(135deg, #010205, #020a1f, #07353f, #010205, #020a1f);
          background-size: 300% 300%;
          animation: flowMesh 20s ease-in-out infinite;
        }
      `}</style>
      <div className="flex flex-col md:flex-row h-full w-full items-center justify-center bg-mesh-flow relative overflow-hidden">

        {/* 左側：滿版圖片 */}
        <div className="flex-1 flex justify-start items-center h-full w-full md:w-[55%] z-10">
          <img
            src={kvLogoImg}
            alt="VBS"
            className="w-full h-full object-cover object-left animate-slide-in"
          />
        </div>

        {/* 右側：登入框 */}
        <div className="flex-1 flex justify-center md:justify-start md:pl-16 lg:pl-24 w-full z-10 p-6 md:p-0">
          <div
            className="w-full max-w-sm animate-slide-in flex flex-col justify-center"
            style={{ animationDelay: '100ms', animationFillMode: 'both' }}
          >
            <div className="mb-8">
              <h1 className="text-[26px] font-black text-white tracking-widest uppercase">Login</h1>
            </div>
            <form onSubmit={handleSubmit} className="flex flex-col gap-2">
              <div className="flex items-stretch gap-3">
                <div className="relative flex-1">
                  <input
                    id="token-input"
                    type={showToken ? 'text' : 'password'}
                    placeholder="eyJhbGciOi..."
                    value={token}
                    onChange={(e) => setToken(e.target.value)}
                    className="w-full glass-dark border border-white/10 rounded-xl px-4 py-3 pr-12
                    text-[17px] font-medium text-vbs-text bg-transparent outline-none
                    focus:border-vbs-accent/50 focus:shadow-[0_0_15px_rgba(30,144,255,0.2)] transition-all"
                    autoFocus
                  />
                  {/* 顯示/隱藏 Toggle */}
                  <button
                    type="button"
                    id="toggle-show-token"
                    onClick={() => setShowToken((v) => !v)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-vbs-muted hover:text-vbs-text transition-colors text-[15px]"
                  >
                    {showToken ? '隱藏' : '顯示'}
                  </button>
                </div>

                <button
                  id="login-btn"
                  type="submit"
                  disabled={loading}
                  className={`
                  px-6 rounded-xl font-bold text-[16px] transition-all active:scale-95 whitespace-nowrap flex items-center justify-center
                  ${loading
                      ? 'glass-dark border border-white/5 text-vbs-muted cursor-not-allowed'
                      : 'bg-vbs-accent/20 border border-vbs-accent/50 text-vbs-accent hover:bg-vbs-accent/30 hover:border-vbs-accent shadow-[0_0_15px_rgba(30,144,255,0.2)]'}
                `}
                >
                  {loading ? '驗證中…' : '進入'}
                </button>
              </div>
            </form>

            <div className="mt-8 pt-4 flex items-center justify-between opacity-70">
              <span className="text-[14px] text-vbs-muted">Developed by 雲嘉視聽</span>
              <span className="text-[14px] text-vbs-muted">v0.1.0</span>
            </div>
          </div>
        </div>
      </div>
    </>
  )
}
