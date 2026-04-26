import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import logoTxtImg from '../assets/images/vbslogo-txtimg.svg'

export default function Header() {
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <header className="glass-dark border-b border-white/5 h-12 md:h-14 flex items-center px-3 md:px-5 gap-3 md:gap-4 shrink-0 z-30">
      {/* Logo */}
      <img src={logoTxtImg} alt="VBS" className="h-6 md:h-7 w-auto" />

      <div className="w-px h-5 bg-white/10 hidden md:block" />
      <div className="hidden md:flex flex-col leading-none">
        <span className="text-[12px] font-semibold text-vbs-muted uppercase tracking-widest">Console</span>
        <span className="text-[15px] text-vbs-text/70 font-medium">Virtual Broadcasting Services</span>
      </div>

      <div className="flex-1" />

      {/* 右側狀態區 */}
      <div className="flex items-center gap-2 md:gap-3">
        {/* STREAMING 狀態 */}
        <div className="flex items-center gap-1.5 glass rounded-lg px-2 md:px-3 py-1.5">
          <span className="w-1.5 h-1.5 rounded-full bg-vbs-pvw animate-pulse-slow shrink-0" />
          <span className="text-[12px] font-bold text-vbs-pvw hidden xs:inline">STREAMING</span>
        </div>

        {/* ON AIR */}
        <div className="flex items-center gap-1.5 glass rounded-lg px-2 md:px-3 py-1.5">
          <span className="w-1.5 h-1.5 rounded-full bg-vbs-pgm animate-pulse-pgm shrink-0" />
          <span className="text-[12px] font-black text-vbs-pgm tracking-wide">ON AIR</span>
        </div>

        {/* Uptime */}
        <Uptime />

        {/* 角色 + 登出（桌面） */}
        {user && (
          <>
            <span className={`hidden sm:inline text-[15px] font-black px-2 py-1 rounded-lg tracking-widest
              ${user.role === 'admin' ? 'bg-vbs-accent/20 text-vbs-accent border border-vbs-accent/30' : 'bg-vbs-pvw/20 text-vbs-pvw border border-vbs-pvw/30'}`}>
              {user.role.toUpperCase()}
            </span>
            <button
              id="header-logout"
              onClick={handleLogout}
              title="登出"
              className="hidden sm:flex items-center gap-1 text-vbs-muted hover:text-vbs-pgm transition-colors text-[15px] font-semibold"
            >
              
            </button>
          </>
        )}
      </div>
    </header>
  )
}

function Uptime() {
  const [secs, setSecs] = useState(0)
  useEffect(() => {
    const t = setInterval(() => setSecs((s) => s + 1), 1000)
    return () => clearInterval(t)
  }, [])
  const h = String(Math.floor(secs / 3600)).padStart(2, '0')
  const m = String(Math.floor((secs % 3600) / 60)).padStart(2, '0')
  const s = String(secs % 60).padStart(2, '0')
  return (
    <div className="text-[15px] font-semibold text-vbs-muted tabular-nums hidden sm:block">
      {h}:{m}:{s}
    </div>
  )
}
