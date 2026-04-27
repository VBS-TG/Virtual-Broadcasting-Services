import { useState, useRef, useCallback, useEffect } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'
import { useRuntimeStore } from '../stores/runtimeStore'

interface SwitcherProps {
  compact?: boolean
  fullScreen?: boolean
}

const DEFAULT_INPUTS = [
  { id: 1, label: 'IN-1' }, { id: 2, label: 'IN-2' },
  { id: 3, label: 'IN-3' }, { id: 4, label: 'IN-4' },
  { id: 5, label: 'IN-5' }, { id: 6, label: 'IN-6' },
  { id: 7, label: 'IN-7' }, { id: 8, label: 'IN-8' },
]

function parseRate(val: string): number {
  const m = val.match(/^(\d{1,2}):(\d{2})$/)
  if (!m) return 1000
  return parseInt(m[1]) * 1000 + Math.round(Math.min(29, parseInt(m[2])) * (1000 / 30))
}

function RateInput({ id, value, onChange }: {
  id: string; value: string; onChange: (v: string) => void
}) {
  const handleBlur = () => {
    const m = value.match(/^(\d{1,2}):?(\d{0,2})$/)
    if (m) onChange(`${m[1].padStart(2, '0')}:${(m[2] || '00').padStart(2, '0')}`)
  }
  return (
    <input id={id} type="text" value={value} maxLength={5} placeholder="00:00"
      onChange={(e) => onChange(e.target.value)}
      onBlur={handleBlur}
      className="w-16 glass-dark border border-white/10 rounded-lg px-2 py-1.5
        text-[12px] font-bold text-vbs-text text-center bg-transparent outline-none
        focus:border-vbs-accent/50 transition-colors tabular-nums"
    />
  )
}

function BusRow({ label, activeId, bus, fullScreen, inputs, onSelect }: {
  label: string; activeId: number; bus: 'pgm' | 'pvw'; fullScreen?: boolean;
  inputs: Array<{ id: number; label: string }>;
  onSelect: (id: number) => void
}) {
  return (
    <div className="flex flex-col gap-2 w-full">
      <span className={`text-[11px] font-bold uppercase tracking-widest ${bus === 'pgm' ? 'text-vbs-pgm' : 'text-vbs-pvw'}`}>
        {label} BUS
      </span>
      <div className={`grid grid-cols-4 sm:grid-cols-8 xl:grid-cols-4 gap-2 w-full ${fullScreen ? 'flex-1' : ''}`}>
        {inputs.map((inp) => {
          const isActive = activeId === inp.id
          return (
            <button key={inp.id} id={`${bus}-btn-${inp.id}`}
              onClick={() => onSelect(inp.id)}
              className={`
                aspect-square w-full rounded-2xl font-black text-[13px] md:text-[15px] xl:text-[18px]
                transition-all duration-200 active:scale-95 flex items-center justify-center shadow-lg border
                ${isActive
                  ? bus === 'pgm'
                    ? 'pgm-active text-white shadow-[0_0_15px_rgba(255,59,59,0.4)] scale-105 border-vbs-pgm/60'
                    : 'pvw-active text-white shadow-[0_0_15px_rgba(16,185,129,0.4)] scale-105 border-vbs-pvw/60'
                  : `glass-dark border-white/5 text-vbs-muted hover:text-white hover:bg-white/5`}
              `}
            >{inp.id}</button>
          )
        })}
      </div>
    </div>
  )
}

function TBar({ position, onDrag }: { position: number; onDrag: (v: number) => void }) {
  const trackRef = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const update = useCallback((clientY: number) => {
    const rect = trackRef.current?.getBoundingClientRect()
    if (!rect) return
    onDrag(Math.max(0, Math.min(100, ((clientY - rect.top) / rect.height) * 100)))
  }, [onDrag])
  useEffect(() => {
    const mm = (e: MouseEvent) => { if (dragging.current) update(e.clientY) }
    const tm = (e: TouchEvent) => { if (dragging.current && e.touches[0]) update(e.touches[0].clientY) }
    const up = () => { dragging.current = false }
    window.addEventListener('mousemove', mm)
    window.addEventListener('mouseup', up)
    window.addEventListener('touchmove', tm, { passive: true })
    window.addEventListener('touchend', up)
    return () => {
      window.removeEventListener('mousemove', mm)
      window.removeEventListener('mouseup', up)
      window.removeEventListener('touchmove', tm)
      window.removeEventListener('touchend', up)
    }
  }, [update])
  return (
    <div className="flex flex-col items-center justify-between select-none h-full w-full py-2">
      <span className="text-[11px] font-bold text-vbs-muted tracking-widest uppercase">T-BAR</span>
      <div ref={trackRef}
        className={`relative rounded-full glass-dark border border-white/5 cursor-pointer w-10 sm:w-12 h-32 sm:h-48 xl:h-[200px] shrink-0 my-4 shadow-inner`}
        onMouseDown={(e) => { dragging.current = true; update(e.clientY) }}
        onTouchStart={(e) => { dragging.current = true; update(e.touches[0].clientY) }}>
        <div className="absolute top-0 left-0 right-0 rounded-full transition-none"
          style={{ height: `${position}%`, background: 'linear-gradient(to bottom, rgba(30,144,255,0.3), transparent)' }} />
        <div className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2 z-10" style={{ top: `${position}%` }}>
          <div className="w-16 sm:w-20 h-10 rounded-full cursor-grab active:cursor-grabbing
            flex flex-col items-center justify-center gap-[4px] btn-gradient shadow-[0_4px_15px_rgba(0,0,0,0.5)]">
            <div className="w-8 h-[2px] bg-white/40 rounded-full" />
            <div className="w-8 h-[2px] bg-white/40 rounded-full" />
            <div className="w-8 h-[2px] bg-white/40 rounded-full" />
          </div>
        </div>
      </div>
      <div className="flex flex-col items-center">
        <span className="text-[10px] text-vbs-muted uppercase font-bold mb-[-4px]">Transition</span>
        <span className="text-3xl font-black text-vbs-accent tabular-nums tracking-tighter drop-shadow-md">{Math.round(position)}<span className="text-[16px] text-vbs-muted">%</span></span>
      </div>
    </div>
  )
}

export default function Switcher({ compact, fullScreen }: SwitcherProps) {
  const { state, setProgram, setPreview } = useSwitcherStore()
  const runtimeInputs = useRuntimeStore((s) => s.config?.inputs ?? 8)
  const [tbarPos, setTbarPos] = useState(0)
  const inputs = DEFAULT_INPUTS.slice(0, Math.max(1, Math.min(8, runtimeInputs)))

  const [transitioning, setTransitioning] = useState(false)
  const [ftbOn, setFtbOn] = useState(false)
  const [autoRate, setAutoRate] = useState('01:00')
  const [ftbRate] = useState('01:00')
  const [lastOp, setLastOp] = useState<{ time: string; payload: any; error?: string } | null>(null)
  const rafRef = useRef<number | null>(null)
  const cooldown = useRef(false)

  const logOp = useCallback((action: string, payload: any) => {
    setLastOp({
      time: new Date().toLocaleTimeString('zh-TW', { hour12: false }),
      payload: { action, ...payload }
    })
  }, [])

  const animateTo = useCallback((from: number, to: number, ms: number, done?: () => void) => {
    if (rafRef.current) cancelAnimationFrame(rafRef.current)
    setTransitioning(true)
    const start = performance.now()
    const tick = (now: number) => {
      const p = Math.min(1, (now - start) / ms)
      const ease = p < 0.5 ? 2 * p * p : 1 - Math.pow(-2 * p + 2, 2) / 2
      setTbarPos(from + (to - from) * ease)
      if (p < 1) { rafRef.current = requestAnimationFrame(tick) }
      else { setTbarPos(to); setTransitioning(false); cooldown.current = false; done?.() }
    }
    rafRef.current = requestAnimationFrame(tick)
  }, [])

  const doCut = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    // Optimistic: swap pgm/pvw 同時發送 API（store 內處理）
    const [np, nv] = [state.preview, state.program]
    setProgram(np)
    setPreview(nv)
    setTbarPos(0)
    logOp('CUT', { program: np, preview: nv })
    setTimeout(() => { cooldown.current = false }, 120)
  }, [state, setProgram, setPreview, logOp])

  const doAuto = useCallback(() => {
    if (cooldown.current || transitioning) return
    cooldown.current = true
    animateTo(tbarPos, tbarPos < 50 ? 100 : 0, parseRate(autoRate), () => {
      if (tbarPos < 50) {
        setProgram(state.preview)
        setPreview(state.program)
        setTbarPos(0)
        logOp('AUTO', { program: state.preview, preview: state.program, rate: autoRate })
      }
    })
  }, [state, tbarPos, autoRate, transitioning, animateTo, setProgram, setPreview, logOp])

  const doFtb = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    if (!ftbOn) { setFtbOn(true); animateTo(0, 100, parseRate(ftbRate)); logOp('FTB_ON', { rate: ftbRate }) }
    else { setFtbOn(false); animateTo(100, 0, parseRate(ftbRate)); logOp('FTB_OFF', { rate: ftbRate }) }
  }, [ftbOn, ftbRate, animateTo, logOp])

  const handleTbarDrag = useCallback((v: number) => {
    if (transitioning) return
    setTbarPos(v)
    if (v >= 99.5) {
      setProgram(state.preview)
      setPreview(state.program)
      logOp('TBAR_COMPLETE', { program: state.preview, preview: state.program })
      setTimeout(() => setTbarPos(0), 80)
    }
  }, [state, transitioning, setProgram, setPreview, logOp])
  return (
    <div className={`${fullScreen ? 'w-full h-full flex flex-col gap-2 p-4' : `flex flex-col gap-4 ${compact ? 'p-1' : 'p-2'}`}`}>
      {!fullScreen && !compact && (
        <div className="flex items-center justify-between pb-2 border-b border-white/5">
          <span className="text-[12px] font-bold text-vbs-muted uppercase tracking-widest">Virtual Switcher</span>
          <span className="text-[12px] font-bold text-vbs-pvw hidden sm:inline uppercase">VBS Core</span>
        </div>
      )}

      {/* Bento Grid Layout for Switcher */}
      <div className={`grid grid-cols-1 xl:grid-cols-12 gap-4 ${fullScreen ? 'flex-1 h-full' : ''}`}>
        
        {/* Left: Bus Buttons (PGM & PVW) - Span 7 */}
        <div className={`xl:col-span-7 flex flex-col gap-4 justify-center glass-dark rounded-3xl p-5 md:p-6 shadow-xl ${fullScreen ? 'h-full' : ''}`}>
          <BusRow label="Program" activeId={state.program} bus="pgm" fullScreen={fullScreen}
            inputs={inputs}
            onSelect={(id) => { if (!cooldown.current) setProgram(id) }} />
          <div className="h-px w-full bg-white/5 my-2" />
          <BusRow label="Preview" activeId={state.preview} bus="pvw" fullScreen={fullScreen}
            inputs={inputs}
            onSelect={(id) => { if (!cooldown.current) setPreview(id) }} />
        </div>

        {/* Middle: T-Bar - Span 2 */}
        <div className={`xl:col-span-2 flex flex-col items-center justify-center glass rounded-3xl shadow-xl ${fullScreen ? 'h-full' : 'py-6 xl:py-2'}`}>
          <TBar position={tbarPos} onDrag={handleTbarDrag} />
        </div>

        {/* Right: Controls - Span 3 */}
        <div className={`xl:col-span-3 flex xl:flex-col gap-4 justify-center glass-dark rounded-3xl p-5 md:p-6 shadow-xl ${fullScreen ? 'h-full' : ''}`}>
          
          <div className="flex flex-col items-center gap-2 flex-1 xl:flex-none">
            <span className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest w-full text-center mb-1">Transition</span>
            <button id="auto-btn" onClick={doAuto} disabled={transitioning}
              className={`w-full max-w-[120px] aspect-video rounded-2xl font-black text-[18px] transition-all active:scale-95 shadow-lg
                ${transitioning
                  ? 'bg-vbs-accent/50 text-white shadow-[0_0_20px_rgba(30,144,255,0.6)]'
                  : 'btn-gradient'}`}>
              AUTO
            </button>
            <RateInput id="auto-rate" value={autoRate} onChange={setAutoRate} />
          </div>

          <div className="hidden xl:block h-px w-full bg-white/5" />

          <div className="flex gap-4 flex-1 xl:flex-none justify-center">
            <div className="flex flex-col items-center gap-2">
              <span className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest mb-1 xl:hidden">Cut</span>
              <button id="cut-btn" onClick={doCut}
                className={`w-[60px] h-[60px] xl:w-full xl:max-w-[120px] xl:aspect-video rounded-2xl font-black text-[16px] xl:text-[18px] border transition-all active:scale-95 shadow-lg
                  glass border-vbs-pgm/40 text-vbs-pgm hover:bg-vbs-pgm/20 hover:border-vbs-pgm/80 hover:shadow-[0_0_20px_rgba(255,59,59,0.4)]`}>
                CUT
              </button>
            </div>
            <div className="flex flex-col items-center gap-2">
              <span className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest mb-1 xl:hidden">Fade</span>
              <button id="ftm-btn" onClick={doFtb}
                className={`w-[60px] h-[60px] xl:w-full xl:max-w-[120px] xl:aspect-video rounded-2xl font-black text-[16px] xl:text-[18px] border transition-all active:scale-95 shadow-lg
                  ${ftbOn
                    ? 'bg-white/10 border-white/40 text-white shadow-[inset_0_0_20px_rgba(255,255,255,0.2)]'
                    : 'glass border-white/10 text-vbs-muted hover:text-white hover:border-white/30 hover:bg-white/5'}`}>
                FTM
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* ── Last Operation Result ── */}
      {!fullScreen && !compact && (
        <div className="mt-2">
          <div className="glass-dark rounded-2xl p-4 shadow-xl border border-white/5 flex flex-col">
            <span className="text-[11px] font-bold text-vbs-muted uppercase tracking-widest mb-3">Last Request</span>
            {lastOp ? (
              <div className="flex items-center gap-3 font-mono">
                <span className="text-[14px] text-vbs-accent font-bold">[{lastOp.time}]</span> 
                <span className="text-[13px] text-white opacity-80 break-all">{JSON.stringify(lastOp.payload)}</span>
                {lastOp.error && <span className="text-[13px] text-vbs-pgm font-bold ml-2">Error: {lastOp.error}</span>}
              </div>
            ) : (
              <span className="text-[13px] text-vbs-muted">尚未執行切換操作</span>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
