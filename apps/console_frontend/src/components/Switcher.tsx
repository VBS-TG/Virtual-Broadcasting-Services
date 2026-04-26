import { useState, useRef, useCallback, useEffect } from 'react'
import { useSwitcherStore } from '../stores/switcherStore'

interface SwitcherProps {
  compact?: boolean
  fullScreen?: boolean
}

// [MOCK] 輸入源標籤目前固定
// TODO: 後端就緒後從 runtimeStore 取得動態標籤
const INPUTS = [
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

function BusRow({ label, activeId, bus, fullScreen, onSelect }: {
  label: string; activeId: number; bus: 'pgm' | 'pvw'; fullScreen?: boolean;
  onSelect: (id: number) => void
}) {
  return (
    <div className="flex flex-col gap-2">
      <span className={`text-[14px] font-black tracking-widest ${bus === 'pgm' ? 'text-vbs-pgm' : 'text-vbs-pvw'}`}>
        {label}
      </span>
      <div className={`grid grid-cols-4 gap-2 sm:gap-3 ${fullScreen ? 'flex-1' : ''}`}>
        {INPUTS.map((inp) => {
          const isActive = activeId === inp.id
          return (
            <button key={inp.id} id={`${bus}-btn-${inp.id}`}
              onClick={() => onSelect(inp.id)}
              className={`
                w-14 h-14 sm:w-16 sm:h-16 xl:w-20 xl:h-20 shrink-0 mx-auto rounded-xl font-bold text-[14px] xl:text-[18px] border
                transition-all duration-100 active:scale-95 flex items-center justify-center
                ${isActive
                  ? bus === 'pgm'
                    ? 'pgm-active text-vbs-pgm shadow-pgm scale-105'
                    : 'pvw-active text-vbs-pvw shadow-pvw scale-105'
                  : `glass-dark border-white/8 text-vbs-muted hover:text-vbs-text
                     ${bus === 'pgm' ? 'hover:border-vbs-pgm/30' : 'hover:border-vbs-pvw/30'}`}
              `}
            >{inp.label}</button>
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
    <div className="flex flex-col items-center gap-2 select-none h-full justify-center">
      <span className="text-[15px] font-black text-vbs-muted tracking-widest">BAR</span>
      <div ref={trackRef}
        className={`relative rounded-full bg-white/5 border border-white/10 cursor-pointer w-10 sm:w-12 h-[160px] sm:h-[180px] xl:h-[220px] shrink-0`}
        onMouseDown={(e) => { dragging.current = true; update(e.clientY) }}
        onTouchStart={(e) => { dragging.current = true; update(e.touches[0].clientY) }}>
        <div className="absolute top-0 left-0 right-0 rounded-full transition-none"
          style={{ height: `${position}%`, background: 'linear-gradient(to bottom, rgba(30,144,255,0.4), rgba(30,144,255,0.1))' }} />
        <div className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2 z-10" style={{ top: `${position}%` }}>
          <div className="w-16 sm:w-20 h-8 rounded-lg cursor-grab active:cursor-grabbing
            flex flex-col items-center justify-center gap-[4px]
            bg-gradient-to-b from-slate-300 to-slate-500 border border-white/30 shadow-2xl">
            <div className="w-10 h-[2px] bg-black/40 rounded-full" />
            <div className="w-10 h-[2px] bg-black/40 rounded-full" />
            <div className="w-10 h-[2px] bg-black/40 rounded-full" />
          </div>
        </div>
      </div>
      <span className="text-[15px] font-bold text-vbs-accent tabular-nums mt-1">{Math.round(position)}%</span>
    </div>
  )
}

export default function Switcher({ compact, fullScreen }: SwitcherProps) {
  const { state, setProgram, setPreview } = useSwitcherStore()
  const [tbarPos, setTbarPos] = useState(0)
  const [transitioning, setTransitioning] = useState(false)
  const [ftbOn, setFtbOn] = useState(false)
  const [autoRate, setAutoRate] = useState('01:00')
  const [ftbRate, setFtbRate] = useState('01:00')
  const rafRef = useRef<number | null>(null)
  const cooldown = useRef(false)

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
    setTimeout(() => { cooldown.current = false }, 120)
  }, [state, setProgram, setPreview])

  const doAuto = useCallback(() => {
    if (cooldown.current || transitioning) return
    cooldown.current = true
    animateTo(tbarPos, tbarPos < 50 ? 100 : 0, parseRate(autoRate), () => {
      if (tbarPos < 50) {
        setProgram(state.preview)
        setPreview(state.program)
        setTbarPos(0)
      }
    })
  }, [state, tbarPos, autoRate, transitioning, animateTo, setProgram, setPreview])

  const doFtb = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    if (!ftbOn) { setFtbOn(true); animateTo(0, 100, parseRate(ftbRate)) }
    else { setFtbOn(false); animateTo(100, 0, parseRate(ftbRate)) }
  }, [ftbOn, ftbRate, animateTo])

  const handleTbarDrag = useCallback((v: number) => {
    if (transitioning) return
    setTbarPos(v)
    if (v >= 99.5) {
      setProgram(state.preview)
      setPreview(state.program)
      setTimeout(() => setTbarPos(0), 80)
    }
  }, [state, transitioning, setProgram, setPreview])

  return (
    <div className={`${fullScreen ? 'w-full h-full flex flex-col gap-2' : `glass rounded-xl flex flex-col gap-4 ${compact ? 'p-4' : 'p-6 md:p-8'}`}`}>
      {!fullScreen && (
        <div className="flex items-center justify-between pb-2 border-b border-white/5">
          <span className="text-[14px] font-black text-vbs-muted uppercase tracking-widest">Virtual Switcher</span>
          <span className="text-[15px] font-bold text-vbs-pvw hidden sm:inline">VBS CORE</span>
        </div>
      )}

      <div className={`flex flex-col xl:flex-row flex-wrap gap-6 xl:gap-8 items-stretch justify-center ${fullScreen ? 'flex-1 h-full overflow-hidden' : ''}`}>
        {/* Bus Buttons */}
        <div className={`flex flex-col gap-4 justify-between ${fullScreen ? 'h-full' : ''}`}>
          <BusRow label="PGM" activeId={state.program} bus="pgm" fullScreen={fullScreen}
            onSelect={(id) => { if (!cooldown.current) setProgram(id) }} />
          <BusRow label="PVW" activeId={state.preview} bus="pvw" fullScreen={fullScreen}
            onSelect={(id) => { if (!cooldown.current) setPreview(id) }} />
        </div>

        {/* T-Bar desktop */}
        <div className={`hidden xl:flex flex-col items-center justify-center px-4 border-l border-r border-white/5 py-2 ${fullScreen ? 'h-full py-4' : ''}`}>
          <TBar position={tbarPos} onDrag={handleTbarDrag} />
        </div>
        {/* T-Bar mobile */}
        <div className={`xl:hidden w-full flex justify-center py-4 border-t border-b border-white/5 ${fullScreen ? 'h-[120px]' : ''}`}>
          <TBar position={tbarPos} onDrag={handleTbarDrag} />
        </div>

        {/* Controls */}
        <div className={`flex flex-col gap-4 justify-center ${fullScreen ? 'h-full' : ''}`}>
          <div className="flex justify-center w-full">
            <div className="flex flex-col items-center gap-2">
              <button id="auto-btn" onClick={doAuto} disabled={transitioning}
                className={`w-16 h-16 sm:w-20 sm:h-20 xl:w-20 xl:h-20 shrink-0 rounded-xl font-black text-[15px] xl:text-[18px] border transition-all active:scale-95 shadow-lg
                  ${transitioning
                    ? 'bg-vbs-accent/30 border-vbs-accent text-vbs-accent shadow-[0_0_20px_rgba(30,144,255,0.4)]'
                    : 'glass border-vbs-accent/50 text-vbs-accent hover:bg-vbs-accent/15 hover:border-vbs-accent'}`}>
                AUTO
              </button>
              <RateInput id="auto-rate" value={autoRate} onChange={setAutoRate} />
            </div>
          </div>

          <div className="flex gap-4">
            <div className="flex flex-col items-center gap-2">
              <button id="cut-btn" onClick={doCut}
                className={`w-16 h-16 sm:w-20 sm:h-20 xl:w-20 xl:h-20 shrink-0 rounded-xl font-black text-[15px] xl:text-[18px] border transition-all active:scale-95 shadow-lg
                  glass border-vbs-pgm/50 text-vbs-pgm hover:bg-vbs-pgm/20 hover:border-vbs-pgm hover:shadow-[0_0_20px_rgba(255,59,59,0.5)]`}>
                CUT
              </button>
              <div className="h-[28px]" />
            </div>
            <div className="flex flex-col items-center gap-2">
              <button id="ftm-btn" onClick={doFtb}
                className={`w-16 h-16 sm:w-20 sm:h-20 xl:w-20 xl:h-20 shrink-0 rounded-xl font-black text-[15px] xl:text-[18px] border transition-all active:scale-95 shadow-lg
                  ${ftbOn
                    ? 'bg-black border-white/60 text-white shadow-[0_0_20px_rgba(255,255,255,0.3)]'
                    : 'glass border-white/20 text-vbs-muted hover:text-vbs-text hover:border-white/40'}`}>
                FTM
              </button>
              <RateInput id="ftb-rate" value={ftbRate} onChange={setFtbRate} />
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
