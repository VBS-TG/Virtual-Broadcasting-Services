import { useEffect, useState, useRef, useCallback } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import { useShowConfigStore } from '../stores/showConfigStore'
import Switcher from '../components/Switcher'
import Multiviewer from '../components/Multiviewer'
import { ExternalLink, Wifi, WifiOff } from 'lucide-react'
import PageShell from '../components/PageShell'

export default function SwitcherPage() {
  const { state, fetchState, error } = useSwitcherStore()
  const { draft, fetch: fetchShowConfig, updateDraft, saveDraft, applyDraft, saving, applying } = useShowConfigStore()
  
  const containerRef = useRef<HTMLDivElement>(null)
  const [leftWidth, setLeftWidth] = useState(60)
  const [isDragging, setIsDragging] = useState(false)
  const [activeCell, setActiveCell] = useState<number | null>(0)

  useEffect(() => {
    fetchState()
    fetchShowConfig()
  }, [fetchState, fetchShowConfig])

  const openPopout = (path: string, name: string) => {
    window.open(path, name, 'width=1000,height=600,menubar=no,toolbar=no,location=no,status=no')
  }

  const handleMouseDown = (e: React.MouseEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!isDragging || !containerRef.current) return
    const containerRect = containerRef.current.getBoundingClientRect()
    let newWidth = ((e.clientX - containerRect.left) / containerRect.width) * 100
    if (newWidth < 30) newWidth = 30
    if (newWidth > 80) newWidth = 80
    setLeftWidth(newWidth)
  }, [isDragging])

  const handleMouseUp = useCallback(() => {
    setIsDragging(false)
  }, [])

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

  const isConnected = !error
  const inputLabels = Array.from(
    { length: 8 },
    (_, i) => draft?.sources?.find((s) => s.slot_id === `input${i + 1}`)?.display_name || `Source${i + 1}`
  )
  const cellSources = Array.from(
    { length: 8 },
    (_, i) => String((draft?.multiview?.cells as any[])?.[i]?.source ?? `input${i + 1}`)
  )
  const buttonMappings = Array.from(
    { length: 8 },
    (_, i) => String(((draft?.switcher?.rows as any[])?.[0]?.buttons ?? [])[i]?.source ?? `input${i + 1}`)
  )

  return (
    <PageShell 
      title="導播" 
      description="Live Switcher Control"
      extra={
        <div className={`flex items-center gap-2 px-3 py-1 rounded-lg border ${isConnected ? 'bg-vbs-pvw/10 border-vbs-pvw/30 text-vbs-pvw' : 'bg-vbs-pgm/10 border-vbs-pgm/30 text-vbs-pgm'}`}>
          {isConnected ? <Wifi className="w-3.5 h-3.5" /> : <WifiOff className="w-3.5 h-3.5" />}
          <span className="text-[11px] font-black uppercase tracking-widest">{isConnected ? '已連線' : '已斷線'}</span>
        </div>
      }
    >
      <div 
        ref={containerRef} 
        className="h-[calc(100vh-220px)] w-full flex flex-col xl:flex-row relative"
        style={{ '--left-width': `${leftWidth}%` } as React.CSSProperties}
      >
        {isDragging && <div className="fixed inset-0 z-50 cursor-col-resize" />}

        {/* ── MutiView Section (Left) ── */}
        <div className="flex-1 xl:flex-none xl:w-[var(--left-width)] flex flex-col gap-3 min-w-0 min-h-0 xl:pr-4 pb-4 xl:pb-0">
          <div className="flex items-center justify-between shrink-0">
            <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">MultiView Monitor</span>
            <div className="flex items-center gap-2">
              <button
                onClick={async () => {
                  const okSave = await saveDraft()
                  if (!okSave) return alert('Multiview Mapping 儲存失敗')
                  const okApply = await applyDraft()
                  if (!okApply) return alert('Multiview Mapping 套用失敗')
                  alert('Multiview Mapping 已套用')
                }}
                disabled={saving || applying}
                className="text-white border border-white/10 bg-white/5 px-2 py-1 rounded text-[11px] font-black uppercase tracking-widest disabled:opacity-50"
              >
                套用 Mapping
              </button>
              <button
                onClick={() => openPopout('/popout/multiviewer', 'mutiview_popout')}
                className="text-vbs-accent hover:text-vbs-accent/70 transition-colors flex items-center gap-1 text-[11px] font-black uppercase tracking-widest"
              >
                <ExternalLink className="w-3 h-3" />
                <span>彈出視窗</span>
              </button>
            </div>
          </div>
          <div className="flex-1 min-h-0 glass rounded-[24px] p-2 overflow-hidden shadow-inner">
            <Multiviewer
              pgm={state.program}
              pvw={state.preview}
              fullScreen
              editable
              inputLabels={inputLabels}
              cellSources={cellSources}
              buttonMappings={buttonMappings}
              activeCellIndex={activeCell}
              onActiveCellChange={setActiveCell}
              onCellSourceChange={(cellIndex, source) => {
                updateDraft((old) => {
                  const cells = [...(((old.multiview?.cells as any[]) ?? []))]
                  while (cells.length < 8) cells.push({ source: `input${cells.length + 1}` })
                  cells[cellIndex] = { ...cells[cellIndex], source }
                  return { ...old, multiview: { ...old.multiview, cells } }
                })
              }}
            />
          </div>
        </div>

        {/* ── 分割拖曳桿 (Resizer) ── */}
        <div
          className="hidden xl:flex w-2 -mx-1 cursor-col-resize group items-center justify-center shrink-0 z-10 relative"
          onMouseDown={handleMouseDown}
        >
          <div className="absolute inset-y-0 -inset-x-2 z-20" />
          <div className={`w-1 h-12 rounded-full transition-all ${
            isDragging 
              ? 'bg-vbs-accent shadow-[0_0_10px_rgba(59,130,246,0.6)]' 
              : 'bg-white/5 group-hover:bg-white/20'
          }`} />
        </div>

        {/* ── Switcher Section (Right) ── */}
        <div className="flex-1 shrink-0 flex flex-col gap-3 overflow-y-auto max-h-full xl:pl-4">
          <div className="flex items-center justify-between shrink-0">
            <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Production Switcher</span>
            <button
              onClick={() => openPopout('/popout/switcher', 'switcher_popout')}
              className="text-vbs-accent hover:text-vbs-accent/70 transition-colors flex items-center gap-1 text-[11px] font-black uppercase tracking-widest"
            >
              <ExternalLink className="w-3 h-3" />
              <span>彈出視窗</span>
            </button>
          </div>
          <div className="flex-1 min-w-[320px]">
            <Switcher compact />
          </div>
        </div>
      </div>
    </PageShell>
  )
}
