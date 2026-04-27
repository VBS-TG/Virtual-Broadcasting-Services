import { useEffect, useState, useRef, useCallback } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import Switcher from '../components/Switcher'
import Multiviewer from '../components/Multiviewer'
import { ExternalLink } from 'lucide-react'

export default function SwitcherPage() {
  const { state, fetchState, error } = useSwitcherStore()
  
  // 拖曳相關的 State 與 Ref
  const containerRef = useRef<HTMLDivElement>(null)
  const [leftWidth, setLeftWidth] = useState(60) // 預設左側佔 60%
  const [isDragging, setIsDragging] = useState(false)

  useEffect(() => { fetchState() }, [fetchState])

  const openPopout = (path: string, name: string) => {
    window.open(path, name, 'width=1000,height=600,menubar=no,toolbar=no,location=no,status=no')
  }

  // 滑鼠按下：開始拖曳
  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault() // 防止選取到文字
    setIsDragging(true)
  }

  // 滑鼠移動：計算新寬度
  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isDragging || !containerRef.current) return
    const containerRect = containerRef.current.getBoundingClientRect()
    // 計算滑鼠位置佔總寬度的百分比
    let newWidth = ((e.clientX - containerRect.left) / containerRect.width) * 100
    
    // 限制拖曳範圍（最小 30%，最大 80%），防止某邊被完全擠不見
    if (newWidth < 30) newWidth = 30
    if (newWidth > 80) newWidth = 80
    
    setLeftWidth(newWidth)
  }, [isDragging])

  // 滑鼠放開：結束拖曳
  const handleMouseUp = useCallback(() => {
    setIsDragging(false)
  }, [])

  // 註冊全域滑鼠事件
  useEffect(() => {
    if (isDragging) {
      window.addEventListener('mousemove', handleMouseMove)
      window.addEventListener('mouseup', handleMouseUp)
    } else {
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }
    return () => {
      window.removeEventListener('mousemove', handleMouseMove)
      window.removeEventListener('mouseup', handleMouseUp)
    }
  }, [isDragging, handleMouseMove, handleMouseUp])

  return (
    // 使用 CSS 變數 --left-width 將狀態傳遞給 Tailwind
    <div 
      ref={containerRef} 
      className="h-full w-full overflow-hidden p-3 md:p-4 flex flex-col xl:flex-row"
      style={{ '--left-width': `${leftWidth}%` } as React.CSSProperties}
    >
      
      {/* 拖曳時顯示的透明遮罩，防止滑鼠事件被內層元件吃掉導致中斷 */}
      {isDragging && <div className="fixed inset-0 z-50 cursor-col-resize" />}

      {/* ── MutiView Section (Left) ── */}
      {/* 在 xl 螢幕下寬度由變數控制，其他尺寸則自動填滿 (flex-1) */}
      <div className="flex-1 xl:flex-none xl:w-[var(--left-width)] flex flex-col gap-2 min-w-0 min-h-0 xl:pr-3 pb-4 xl:pb-0">
        <div className="flex items-center justify-between shrink-0">
          <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest flex items-center gap-2">
            MutiView
            {error && <span className="text-[13px] text-vbs-pgm font-semibold ml-2">{error}</span>}
          </h2>
          <button
            onClick={() => openPopout('/popout/multiviewer', 'mutiview_popout')}
            className="text-vbs-accent hover:text-vbs-accent/70 transition-colors flex items-center gap-1 text-[12px] bg-vbs-accent/10 px-2 py-1 rounded-md"
            title="獨立視窗"
          >
            <ExternalLink className="w-3 h-3" />
            <span>彈出</span>
          </button>
        </div>
        <div className="flex-1 min-h-0">
          <Multiviewer pgm={state.program} pvw={state.preview} fullScreen />
        </div>
      </div>

      {/* ── 分割拖曳桿 (Resizer) ── */}
      {/* 只在 xl 螢幕顯示，提供視覺提示與拖曳觸發 */}
      <div
        className="hidden xl:flex w-2 -mx-1 cursor-col-resize group items-center justify-center shrink-0 z-10 relative"
        onMouseDown={handleMouseDown}
      >
        {/* 擴大感應區 */}
        <div className="absolute inset-y-0 -inset-x-2 z-20" />
        {/* 視覺上的一條線 */}
        <div className={`w-1 h-16 rounded-full transition-all ${
          isDragging 
            ? 'bg-vbs-accent shadow-[0_0_10px_rgba(59,130,246,0.6)]' 
            : 'bg-white/10 group-hover:bg-white/40'
        }`} />
      </div>

      {/* ── Switcher Section (Right) ── */}
      {/* 右側自動吃掉剩下空間 (flex-1) */}
      <div className="flex-1 shrink-0 flex flex-col gap-2 overflow-y-auto max-h-full xl:pl-3">
        <div className="flex items-center justify-between shrink-0">
          <h2 className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Virtual Switcher</h2>
          <button
            onClick={() => openPopout('/popout/switcher', 'switcher_popout')}
            className="text-vbs-accent hover:text-vbs-accent/70 transition-colors flex items-center gap-1 text-[12px] bg-vbs-accent/10 px-2 py-1 rounded-md"
            title="獨立視窗"
          >
            <ExternalLink className="w-3 h-3" />
            <span>彈出</span>
          </button>
        </div>
        <div className="flex-1 min-w-[320px]">
          <Switcher compact />
        </div>
      </div>
      
    </div>
  )
}