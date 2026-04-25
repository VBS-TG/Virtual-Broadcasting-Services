import type { PageKey } from '../types'

interface SidebarProps {
  activePage: PageKey
  setActivePage: (p: PageKey) => void
}

const NAV_ITEMS: { key: PageKey; label: string; icon: string; desc: string }[] = [
  { key: 'dashboard',   label: 'Dashboard',   icon: '⬡', desc: '總覽' },
  { key: 'telemetry',   label: 'Telemetry',   icon: '◈', desc: '遙測' },
  { key: 'switcher',    label: 'Switcher',     icon: '◉', desc: '導播' },
  { key: 'multiviewer', label: 'Multiview',   icon: '▦', desc: '監看' },
  { key: 'control',     label: 'Control',      icon: '⊕', desc: '控制' },
]

export default function Sidebar({ activePage, setActivePage }: SidebarProps) {
  return (
    <aside className="glass-dark border-r border-white/5 w-16 flex flex-col items-center py-3 gap-1 shrink-0 z-20">
      {NAV_ITEMS.map(item => {
        const isActive = activePage === item.key
        return (
          <button
            key={item.key}
            id={`nav-${item.key}`}
            onClick={() => setActivePage(item.key)}
            title={item.label}
            className={`
              group relative w-11 h-11 rounded-xl flex flex-col items-center justify-center gap-0.5
              transition-all duration-200
              ${isActive
                ? 'glass border border-vbs-cyan/40 text-vbs-pvw shadow-pvw'
                : 'text-vbs-muted hover:text-vbs-text hover:glass hover:border hover:border-white/10'}
            `}
          >
            <span className="text-[11px\] leading-none">{item.icon}</span>
            <span className="text-xs font-mono leading-none opacity-70">{item.desc}</span>

            {/* Tooltip */}
            <span className="absolute left-full ml-2 px-2 py-1 glass rounded-md text-sm font-medium text-vbs-text
              whitespace-nowrap pointer-events-none opacity-0 group-hover:opacity-100
              transition-opacity duration-150 z-50">
              {item.label}
            </span>

            {/* Active indicator */}
            {isActive && (
              <span className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-6 bg-vbs-pvw rounded-r-full" />
            )}
          </button>
        )
      })}

      <div className="flex-1" />

      {/* Settings */}
      <button
        className="w-11 h-11 rounded-xl flex items-center justify-center text-vbs-muted
          hover:text-vbs-text hover:glass hover:border hover:border-white/10 transition-all duration-200 text-[11px\]"
        title="設定"
      >
        ⚙
      </button>
    </aside>
  )
}
