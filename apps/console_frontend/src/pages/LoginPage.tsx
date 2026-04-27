import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import { validateAccessToken } from '../lib/jwt'
import { adminEmailLogin, exchangeGuestPIN } from '../lib/apiClient'
import { Settings } from 'lucide-react'
import kvLogoImg from '../assets/images/vbs kv-logo.png'

export default function LoginPage() {
  const [teamEmail, setTeamEmail] = useState('')
  const [teamCode, setTeamCode] = useState('')
  const [adminEmail, setAdminEmail] = useState('')
  const [showAdminPanel, setShowAdminPanel] = useState(false)
  const [loading, setLoading] = useState(false)
  const [adminLoading, setAdminLoading] = useState(false)
  const [error, setError] = useState('')
  const login = useAuthStore((s) => s.login)
  const navigate = useNavigate()

  const handleAdminEnter = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    const email = adminEmail.trim().toLowerCase()
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError('請輸入有效管理員 Email')
      return
    }
    setAdminLoading(true)
    const res = await adminEmailLogin(email)
    if (res.error || !res.data?.access_token) {
      setAdminLoading(false)
      setError(res.error ?? '管理員登入失敗')
      return
    }
    const verified = validateAccessToken(res.data.access_token)
    if (!verified.ok || !verified.token || verified.role !== 'admin') {
      setAdminLoading(false)
      setError(verified.error ?? '管理員 JWT 驗證失敗')
      return
    }
    login(verified.token, 'admin', email)
    navigate('/dashboard', { replace: true })
  }

  const handleTeamEnter = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    const email = teamEmail.trim()
    const code = teamCode.trim()
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError('請輸入有效 Email（團隊成員識別）')
      return
    }
    if (!/^\d{6}$/.test(code)) {
      setError('請輸入 6 碼團隊授權碼')
      return
    }
    setLoading(true)
    const exchanged = await exchangeGuestPIN(code)
    if (exchanged.error || !exchanged.data?.access_token) {
      setLoading(false)
      setError(exchanged.error ?? '授權碼兌換失敗')
      return
    }
    const result = validateAccessToken(exchanged.data.access_token)
    if (!result.ok || !result.token || !result.role) {
      setLoading(false)
      setError(result.error ?? '兌換成功但 Token 驗證失敗')
      return
    }
    login(result.token, result.role)
    navigate('/dashboard', { replace: true })
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
            <div className="mb-8 flex items-center justify-between">
              <h1 className="text-[26px] font-black text-white tracking-widest uppercase">Login</h1>
              <span className="px-2 py-0.5 rounded text-[12px] font-bold bg-vbs-accent/20 text-vbs-accent">PROD</span>
            </div>
            
            <form onSubmit={handleTeamEnter} className="flex flex-col gap-2">
              <label className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">Team Email</label>
              <input
                id="team-email-input"
                type="email"
                placeholder="team@company.com"
                value={teamEmail}
                onChange={(e) => setTeamEmail(e.target.value)}
                className="w-full glass-dark border border-white/10 rounded-xl px-4 py-3 text-[17px] font-medium text-vbs-text bg-transparent outline-none focus:border-vbs-accent/50 transition-all"
              />
              <label className="text-[15px] font-semibold text-vbs-muted uppercase tracking-widest">Admin 提供的 6 碼 Code</label>
              <input
                id="team-code-input"
                type="text"
                maxLength={6}
                placeholder="123456"
                value={teamCode}
                onChange={(e) => setTeamCode(e.target.value.replace(/\D/g, ''))}
                className="w-full glass-dark border border-white/10 rounded-xl px-4 py-3 text-[20px] tracking-[0.2em] font-bold text-vbs-text bg-transparent outline-none focus:border-vbs-accent/50 transition-all text-center"
              />
              <button
                id="team-login-btn"
                type="submit"
                disabled={loading}
                className={`mt-1 px-6 py-3 rounded-xl font-bold text-[16px] transition-all active:scale-95 ${loading ? 'glass-dark border border-white/5 text-vbs-muted cursor-not-allowed' : 'bg-vbs-accent/20 border border-vbs-accent/50 text-vbs-accent hover:bg-vbs-accent/30 hover:border-vbs-accent shadow-[0_0_15px_rgba(30,144,255,0.2)]'}`}
              >
                {loading ? '驗證中…' : '以 Team Code 登入'}
              </button>
            </form>

            <div className="mt-2">
              {error && (
                <div className="glass border border-vbs-pgm/40 rounded-lg px-3 py-2">
                  <p className="text-[14px] text-vbs-pgm font-semibold">{error}</p>
                </div>
              )}
            </div>

            <div className="mt-8 pt-4 flex items-center justify-between opacity-70">
              <span className="text-[14px] text-vbs-muted">Developed by 雲嘉視聽</span>
              <span className="text-[14px] text-vbs-muted">v0.1.0</span>
            </div>
          </div>
        </div>

        <button
          id="admin-gear-login"
          type="button"
          onClick={() => { setShowAdminPanel((v) => !v); setError('') }}
          className="absolute bottom-4 right-4 w-10 h-10 rounded-full glass border border-white/15 text-vbs-muted hover:text-vbs-accent hover:border-vbs-accent/40 transition-all flex items-center justify-center z-20"
          title="後台管理登入"
        >
          <Settings className="w-5 h-5" />
        </button>

        {showAdminPanel && (
          <div className="absolute bottom-16 right-4 z-20 w-[320px] glass rounded-xl border border-white/10 p-3">
            <p className="text-[12px] text-vbs-muted font-semibold uppercase tracking-widest mb-2">後台管理登入</p>
            <form onSubmit={handleAdminEnter} className="flex flex-col gap-2">
              <input
                id="admin-email-input"
                type="email"
                placeholder="admin@company.com"
                value={adminEmail}
                onChange={(e) => setAdminEmail(e.target.value)}
                className="w-full glass-dark border border-white/10 rounded-lg px-3 py-2 text-[15px] text-vbs-text bg-transparent outline-none focus:border-vbs-accent/50 transition-all"
              />
              <button
                id="admin-login-btn"
                type="submit"
                disabled={adminLoading}
                className={`w-full py-2 rounded-lg text-[14px] font-bold border transition-all ${adminLoading ? 'glass-dark border-white/5 text-vbs-muted cursor-not-allowed' : 'bg-vbs-accent/20 border-vbs-accent/40 text-vbs-accent hover:bg-vbs-accent/30'}`}
              >
                {adminLoading ? '驗證中…' : '以管理員 Email 進入'}
              </button>
            </form>
          </div>
        )}
      </div>
    </>
  )
}
