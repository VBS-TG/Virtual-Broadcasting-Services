import { useNavigate, useLocation } from 'react-router-dom'
import { useAuthStore } from '../stores/authStore'
import logoTxtImg from '../assets/images/vbslogo-img.svg'
import { LayoutDashboard, Settings2, MonitorPlay, Activity, ShieldAlert, FileText, Settings, Lock, KeyRound } from 'lucide-react'

const NAV_ITEMS = [
  { path: '/dashboard',   label: 'Dashboard',  icon: LayoutDashboard, desc: '總覽'   },
  { path: '/switcher',    label: 'Switcher',   icon: MonitorPlay,     desc: '導播'   },
  { path: '/runtime',     label: 'Runtime',    icon: Settings2,       desc: '配置',  adminOnly: true },
  { path: '/rental-sessions', label: 'Rentals', icon: KeyRound,       desc: '租賃',  adminOnly: true },
  { path: '/telemetry',   label: 'Telemetry',  icon: Activity,        desc: '遙測'   },
  { path: '/system',      label: 'System',     icon: ShieldAlert,     desc: '健康'   },
  { path: '/logs',        label: 'Logs',       icon: FileText,        desc: '日誌'   },
  { path: '/settings',    label: 'Settings',   icon: Settings,        desc: '設定',  adminOnly: true },
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
    <aside 
      className={`
        glass-dark flex flex-col items-center py-6 gap-2 shrink-0 z-20 
        my-4 ml-4 h-[calc(100vh-32px)] w-[92px] rounded-[36px] /* 寬度從 72px 改為 92px */
        
        /* --- iOS 26 核心立體效果 --- */
        border border-white/10 
        shadow-[
          0_20px_50px_rgba(0,0,0,0.5),
          inset_0_1px_1px_rgba(255,255,255,0.15),
          inset_0_0_0_1px_rgba(255,255,255,0.05)
        ]
        relative overflow-hidden
      `}
    >
      {/* 1. 頂部：Logo */}
      <div className="shrink-0 mb-3 px-3 flex justify-center w-full">
        {/* 如果包含文字的 logo 太寬，可以把 w-[64px] 改小，或換成純圖示的 logo */}
        <img src={logoTxtImg} alt="VBS" className="w-[64px] h-auto object-contain drop-shadow-md" />
      </div>

      {/* 2. 狀態區：ON AIR (從 Header 移植) */}
      <div className="shrink-0 mb-4 flex flex-col items-center gap-2">
        <div className="flex items-center gap-1.5 glass bg-black/20 border border-white/5 rounded-lg px-2 py-1.5 shadow-inner">
          <span className="w-1.5 h-1.5 rounded-full bg-vbs-pgm animate-pulse shrink-0 shadow-[0_0_8px_currentColor]" />
          <span className="text-[10px] font-black text-vbs-pgm tracking-wide leading-none">ON AIR</span>
        </div>
      </div>

      {/* 3. 導覽列選單 (保持滾動功能) */}
      <div className="flex flex-col gap-3 w-full items-center overflow-y-auto pb-4 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] [scrollbar-width:none]">
        {NAV_ITEMS.map((item) => {
          const isActive = location.pathname === item.path
          const isReadOnly = !isAdmin && item.adminOnly
          return (
            <button
              key={item.path}
              onClick={() => navigate(item.path)}
              className={`
                group relative w-[68px] h-[56px] shrink-0 rounded-[22px] flex flex-col items-center justify-center gap-1
                transition-all duration-300
                ${isActive
                  ? 'bg-white/15 text-white shadow-[inset_0_1px_1px_rgba(255,255,255,0.2),0_4px_12px_rgba(0,0,0,0.3)]'
                  : 'text-vbs-muted hover:text-white hover:bg-white/5'}
              `}
            >
              {/* Active 指示條優化：加上外發光 */}
              {isActive && (
                <span className="absolute left-1 top-1/2 -translate-y-1/2 w-[3px] h-4 bg-white rounded-full shadow-[0_0_10px_rgba(255,255,255,0.8)]" />
              )}

              <item.icon className={`w-5 h-5 ${isActive ? 'drop-shadow-[0_0_5px_rgba(255,255,255,0.5)]' : ''}`} />
              <span className={`text-[10px] font-medium leading-none ${isActive ? 'opacity-100' : 'opacity-60'}`}>
                {item.desc}
              </span>

              {isReadOnly && <Lock className="w-3 h-3 absolute top-1.5 right-1.5 opacity-40" />}
            </button>
          )
        })}
      </div>

      <div className="flex-1" />

      {/* 4. 底部：角色標籤 + 登出按鈕 */}
      <div className="flex flex-col gap-3 w-full items-center mb-2 shrink-0">
        
        {/* 角色標籤 (從 Header 移植) */}
        {user && (
          <span className={`text-[10px] font-black px-2.5 py-1 rounded-lg tracking-widest leading-none shadow-sm
            ${isAdmin ? 'bg-vbs-accent/20 text-vbs-accent border border-vbs-accent/30' : 'bg-vbs-pvw/20 text-vbs-pvw border border-vbs-pvw/30'}`}>
            {user.role?.toUpperCase()}
          </span>
        )}

        {/* 登出按鈕 */}
        <button
          onClick={handleLogout}
          className="w-[44px] h-[44px] shrink-0 rounded-full flex items-center justify-center text-vbs-muted 
            bg-white/5 border border-white/5
            hover:text-white hover:bg-red-500/20 hover:border-red-500/30
            shadow-[inset_0_1px_1px_rgba(255,255,255,0.1)]
            transition-all duration-300"
        >
          <span className="text-[11px] font-bold">登出</span>
        </button>
      </div>
    </aside>
  )
}