import { useState, useEffect } from 'react'
import Header from './components/Header'
import Sidebar from './components/Sidebar'
import BottomNav from './components/BottomNav'
import TelemetryPanel from './components/TelemetryPanel'
import Switcher from './components/Switcher'
import Multiviewer from './components/Multiviewer'
import ControlPanel from './components/ControlPanel'
import Login from './components/Login'
import type { PageKey } from './types'
import './index.css'
import './App.css'

export default function App() {
  const [isLoggedIn, setIsLoggedIn] = useState(false)
  const [activePage, setActivePage] = useState<PageKey>('dashboard')
  const [pgm, setPgm] = useState<number>(1)
  const [pvw, setPvw] = useState<number>(2)

  if (!isLoggedIn) {
    return <Login onLogin={() => setIsLoggedIn(true)} />
  }

  return (
    <div className="flex flex-col h-full w-full overflow-hidden">
      <Header />

      <div className="flex flex-1 overflow-hidden min-h-0">
        {/* Sidebar：lg 以上顯示 */}
        <div className="hidden lg:flex">
          <Sidebar activePage={activePage} setActivePage={setActivePage} />
        </div>

        {/* 主內容區 */}
        <main className="flex-1 overflow-hidden min-h-0">
          {activePage === 'dashboard' && (
            <Dashboard pgm={pgm} pvw={pvw} setPgm={setPgm} setPvw={setPvw} />
          )}
          {activePage === 'telemetry' && (
            <div className="h-full overflow-y-auto p-3 md:p-4">
              <TelemetryPanel />
            </div>
          )}
          {activePage === 'switcher' && (
            <div className="h-full overflow-y-auto p-3 md:p-4">
              <Switcher pgm={pgm} pvw={pvw} setPgm={setPgm} setPvw={setPvw} />
            </div>
          )}
          {activePage === 'multiviewer' && (
            <div className="h-full overflow-y-auto p-3 md:p-4">
              <Multiviewer pgm={pgm} />
            </div>
          )}
          {activePage === 'control' && <ControlPanel />}
        </main>
      </div>

      {/* Bottom nav：lg 以下顯示 */}
      <div className="flex lg:hidden">
        <BottomNav activePage={activePage} setActivePage={setActivePage} />
      </div>
    </div>
  )
}

/* ── Dashboard 複合版面 ── */
function Dashboard({ pgm, pvw, setPgm, setPvw }: {
  pgm: number; pvw: number
  setPgm: (n: number) => void
  setPvw: (n: number) => void
}) {
  return (
    <div className="h-full overflow-y-auto">
      <div className="p-3 md:p-4 flex flex-col gap-3">

        {/* 狀態條（所有尺寸） */}
        <StatusStrip />

        {/*
          手機 (default): 單欄堆疊
          平板 (md): Multiviewer 全寬 + Switcher 側邊 2欄
          桌面 (lg): 8+4 欄位
        */}
        <div className="flex flex-col md:grid md:grid-cols-12 gap-3">
          {/* Multiviewer */}
          <div className="md:col-span-8">
            <Multiviewer pgm={pgm} compact />
          </div>
          {/* Switcher */}
          <div className="md:col-span-4">
            <Switcher pgm={pgm} pvw={pvw} setPgm={setPgm} setPvw={setPvw} compact />
          </div>
        </div>

        {/* Telemetry：手機 / 平板縮排後顯示 */}
        <TelemetryPanel compact />

      </div>
    </div>
  )
}

/* ── 狀態條 ── */
function StatusStrip() {
  const [time, setTime] = useState(new Date())
  useEffect(() => {
    const t = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(t)
  }, [])

  const nodes = [
    { id: 'CAPTURE', status: 'ONLINE', color: 'text-vbs-pvw' },
    { id: 'ROUTE',   status: 'ONLINE', color: 'text-vbs-pvw' },
    { id: 'ENGINE',  status: 'ONLINE', color: 'text-vbs-pvw' },
  ]

  return (
    <div className="glass rounded-xl px-3 md:px-4 py-2.5 flex flex-wrap items-center justify-between gap-2">
      {/* 節點狀態（md+ 顯示完整 id，手機顯示縮寫） */}
      <div className="flex items-center gap-3 md:gap-5">
        {nodes.map(n => (
          <div key={n.id} className="flex items-center gap-1.5">
            <span className="w-1.5 h-1.5 rounded-full bg-vbs-pvw animate-pulse-slow shrink-0" />
            <span className="text-sm font-semibold text-vbs-muted hidden sm:inline">{n.id}</span>
            <span className={`text-sm font-bold ${n.color}`}>{n.status}</span>
          </div>
        ))}
      </div>

      <div className="flex items-center gap-3">
        <span className="text-sm text-vbs-muted hidden md:inline">SRT AES-256 ✓</span>
        <span className="text-sm font-semibold text-vbs-text tabular-nums">
          {time.toLocaleTimeString('zh-TW', { hour12: false })}
        </span>
      </div>
    </div>
  )
}
