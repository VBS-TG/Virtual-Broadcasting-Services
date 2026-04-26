import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import logoTxtImg from '../assets/images/vbslogo-txtimg.svg'

import { LayoutDashboard, Settings2, MonitorPlay, Activity, ShieldAlert, FileText, Settings, Lock } from 'lucide-react'

const NAV_ITEMS = [
  { path: '/dashboard',   label: 'Dashboard',  icon: LayoutDashboard, desc: '總覽'   },
  { path: '/switcher',    label: 'Switcher',   icon: MonitorPlay,     desc: '導播'   },
  { path: '/runtime',     label: 'Runtime',    icon: Settings2,       desc: '配置'   },
  { path: '/telemetry',   label: 'Telemetry',  icon: Activity,        desc: '遙測'   },
  { path: '/system',      label: 'System',     icon: ShieldAlert,     desc: '健康'   },
  { path: '/logs',        label: 'Logs',       icon: FileText,        desc: '日誌'   },
  { path: '/settings',    label: 'Settings',   icon: Settings,        desc: '設定'   },
]

export default function Sidebar() {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuthStore()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const isAdmin = user?.role === 'admin'

  return (
    <aside className="glass-dark border-r border-white/5 w-16 flex flex-col items-center py-3 gap-1 shrink-0 z-20">
      {/* Logo 小圖示 */}
      <div className="mb-2 px-1">
        <img src={logoTxtImg} alt="VBS" className="w-9 opacity-60" />
      </div>

      {NAV_ITEMS.map((item) => {
        const isActive = location.pathname === item.path
        const isReadOnly = !isAdmin && (item.path === '/runtime' || item.path === '/settings')
        return (
          <button
            key={item.path}
            id={`nav-${item.path.replace('/', '')}`}
            onClick={() => navigate(item.path)}
            title={item.label}
            className={`
              group relative w-11 h-11 rounded-xl flex flex-col items-center justify-center gap-0.5
              transition-all duration-200
              ${isActive
                ? 'glass border border-vbs-cyan/40 text-vbs-cyan shadow-[0_0_12px_rgba(34,211,238,0.3)]'
                : 'text-vbs-muted hover:text-vbs-text hover:glass hover:border hover:border-white/10'}
            `}
          >
            <item.icon className="w-5 h-5 mb-0.5" />
            <span className="text-[12px] font-mono leading-none opacity-70">{item.desc}</span>

            {isReadOnly && <Lock className="w-3 h-3 absolute top-1 right-1 opacity-50" />}

            {/* Tooltip */}
            <span className="absolute left-full ml-2 px-2 py-1 glass rounded-md text-[15px] font-medium text-vbs-text
              whitespace-nowrap pointer-events-none opacity-0 group-hover:opacity-100
              transition-opacity duration-150 z-50 flex items-center gap-1">
              {item.label}
              {isReadOnly && <span className="text-vbs-muted text-[12px] ml-1">(唯讀)</span>}
            </span>

            {/* Active bar */}
            {isActive && (
              <span className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-vbs-cyan rounded-r-full" />
            )}
          </button>
        )
      })}

      <div className="flex-1" />

      {/* 角色徽章 */}
      {user && (
        <div className="mb-1 flex flex-col items-center">
          <span className={`text-[10px] font-black px-1.5 py-0.5 rounded-md tracking-widest
            ${user.role === 'admin' ? 'bg-vbs-accent/20 text-vbs-accent' : 'bg-vbs-pvw/20 text-vbs-pvw'}`}>
            {user.role.toUpperCase()}
          </span>
        </div>
      )}

      {/* 登出 */}
      <button
        id="nav-logout"
        onClick={handleLogout}
        title="登出"
        className="w-11 h-11 rounded-xl flex items-center justify-center text-vbs-muted
          hover:text-vbs-pgm hover:glass hover:border hover:border-vbs-pgm/30 transition-all duration-200"
      >
        <span className="text-[12px] font-bold">登出</span>
      </button>
    </aside>
  )
}
