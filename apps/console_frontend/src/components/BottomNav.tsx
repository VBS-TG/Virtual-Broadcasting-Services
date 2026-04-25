import type { PageKey } from '../types'

interface BottomNavProps {
  activePage: PageKey
  setActivePage: (p: PageKey) => void
}

const NAV_ITEMS: { key: PageKey; label: string; icon: string }[] = [
  { key: 'dashboard',   label: '總覽',  icon: '⬡' },
  { key: 'multiviewer', label: '監看',  icon: '▦' },
  { key: 'switcher',    label: '導播',  icon: '◉' },
  { key: 'telemetry',   label: '遙測',  icon: '◈' },
  { key: 'control',     label: '控制',  icon: '⊕' },
]

export default function BottomNav({ activePage, setActivePage }: BottomNavProps) {
  return (
    <nav className="glass-dark border-t border-white/5 w-full flex items-center justify-around
      px-2 py-2 safe-bottom z-30 shrink-0">
      {NAV_ITEMS.map(item => {
        const isActive = activePage === item.key
        return (
          <button
            key={item.key}
            id={`bottom-nav-${item.key}`}
            onClick={() => setActivePage(item.key)}
            className={`
              flex flex-col items-center gap-0.5 px-3 py-1.5 rounded-xl
              transition-all duration-200 min-w-[52px]
              ${isActive
                ? 'text-vbs-pvw'
                : 'text-vbs-muted hover:text-vbs-text'}
            `}
          >
            <span className={`text-[11px\] leading-none transition-transform duration-200 ${isActive ? 'scale-110' : ''}`}>
              {item.icon}
            </span>
            <span className="text-xs font-semibold tracking-wide leading-none">{item.label}</span>
            {/* 底部指示線 */}
            {isActive && (
              <span className="absolute bottom-1.5 w-5 h-0.5 bg-vbs-pvw rounded-full" />
            )}
          </button>
        )
      })}
    </nav>
  )
}
