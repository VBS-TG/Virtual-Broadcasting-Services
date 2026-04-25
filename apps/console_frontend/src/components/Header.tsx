import { useState, useEffect } from 'react'
import logoTxtImg from '../assets/images/vbslogo-txtimg.svg'

export default function Header() {
  return (
    <header className="glass-dark border-b border-white/5 h-12 md:h-14 flex items-center px-3 md:px-5 gap-3 md:gap-4 shrink-0 z-30">
      {/* Logo */}
      <img src={logoTxtImg} alt="VBS" className="h-6 md:h-7 w-auto" />

      {/* 分隔線（平板以上顯示） */}
      <div className="w-px h-5 bg-white/10 hidden md:block" />
      <div className="hidden md:flex flex-col leading-none">
        <span className="text-sm font-semibold text-vbs-muted uppercase tracking-widest">Console</span>
        <span className="text-sm text-vbs-text/70 font-medium">Virtual Broadcasting Services</span>
      </div>

      <div className="flex-1" />

      {/* 右側狀態區 */}
      <div className="flex items-center gap-2 md:gap-3">
        {/* STREAMING（手機縮為小圓點） */}
        <div className="flex items-center gap-1.5 glass rounded-lg px-2 md:px-3 py-1.5">
          <span className="w-1.5 h-1.5 rounded-full bg-vbs-pvw animate-pulse-slow shrink-0" />
          <span className="text-sm md:text-xs font-bold text-vbs-pvw hidden xs:inline">STREAMING</span>
        </div>

        {/* ON AIR */}
        <div className="flex items-center gap-1.5 glass rounded-lg px-2 md:px-3 py-1.5">
          <span className="w-1.5 h-1.5 rounded-full bg-vbs-pgm animate-pulse-pgm shrink-0" />
          <span className="text-sm md:text-xs font-black text-vbs-pgm tracking-wide">ON AIR</span>
        </div>

        {/* 計時器（平板以上） */}
        <Uptime />
      </div>
    </header>
  )
}

function Uptime() {
  const [secs, setSecs] = useState(0)
  useEffect(() => {
    const t = setInterval(() => setSecs(s => s + 1), 1000)
    return () => clearInterval(t)
  }, [])
  const h = String(Math.floor(secs / 3600)).padStart(2, '0')
  const m = String(Math.floor((secs % 3600) / 60)).padStart(2, '0')
  const s = String(secs % 60).padStart(2, '0')
  return (
    <div className="text-sm md:text-xs font-semibold text-vbs-muted tabular-nums hidden sm:block">
      {h}:{m}:{s}
    </div>
  )
}
