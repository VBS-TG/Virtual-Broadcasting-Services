import { useNavigate, useLocation } from 'react-router-dom'

import { LayoutDashboard, MonitorPlay, Activity, FileText, Network, KeyRound } from 'lucide-react'

const NAV_ITEMS = [
  { path: '/dashboard',   label: '總覽', icon: LayoutDashboard },
  { path: '/switcher',    label: '導播', icon: MonitorPlay },
  { path: '/pipeline',    label: '鏈路', icon: Network },
  { path: '/rental-sessions', label: '租賃', icon: KeyRound },
  { path: '/telemetry',   label: '遙測', icon: Activity },
  { path: '/logs',        label: '日誌', icon: FileText },
]

export default function BottomNav() {
  const navigate = useNavigate()
  const location = useLocation()

  return (
    <nav className="glass-dark border-t border-white/5 w-full flex items-center justify-around
      px-2 py-2 safe-bottom z-30 shrink-0">
      {NAV_ITEMS.map((item) => {
        const isActive = location.pathname === item.path
        return (
          <button
            key={item.path}
            id={`bottom-nav-${item.path.replace('/', '')}`}
            onClick={() => navigate(item.path)}
            className={`
              flex flex-col items-center gap-0.5 px-3 py-1.5 rounded-xl
              transition-all duration-200 min-w-[52px]
              ${isActive ? 'text-vbs-cyan' : 'text-vbs-muted hover:text-vbs-text'}
            `}
          >
            <div className={`transition-transform duration-200 mb-0.5 ${isActive ? 'scale-110' : ''}`}>
              <item.icon className="w-5 h-5" />
            </div>
            <span className="text-[10px] font-semibold tracking-wide leading-none">{item.label}</span>
          </button>
        )
      })}
    </nav>
  )
}
