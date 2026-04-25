import { useState, useRef, useCallback, useEffect } from 'react'

interface SwitcherProps {
  pgm: number; pvw: number
  setPgm: (n: number) => void; setPvw: (n: number) => void
  compact?: boolean
}

const INPUTS = [
  { id: 1, label: 'CAM1' }, { id: 2, label: 'CAM2' },
  { id: 3, label: 'CAM3' }, { id: 4, label: 'CAM4' },
  { id: 5, label: 'HDMI' }, { id: 6, label: 'FILE' },
  { id: 7, label: 'GFX'  }, { id: 8, label: 'BLK'  },
]

// "SS:FF" → ms（FF = frames @30fps）
function parseRate(val: string): number {
  const m = val.match(/^(\d{1,2}):(\d{2})$/)
  if (!m) return 1000
  return parseInt(m[1]) * 1000 + Math.round(Math.min(29, parseInt(m[2])) * (1000 / 30))
}

/* ── Rate 輸入框 ── */
function RateInput({ id, value, onChange }: { id: string; value: string; onChange: (v: string) => void }) {
  const [raw, setRaw] = useState(value)
  const handleBlur = () => {
    const m = raw.match(/^(\d{1,2}):?(\d{0,2})$/)
    if (m) {
      const fmt = `${m[1].padStart(2, '0')}:${(m[2] || '00').padStart(2, '0')}`
      setRaw(fmt); onChange(fmt)
    } else { setRaw(value) }
  }
  return (
    <input id={id} type="text" value={raw} maxLength={5} placeholder="00:00"
      onChange={e => setRaw(e.target.value)} onBlur={handleBlur}
      className="w-16 glass-dark border border-white/10 rounded-lg px-2 py-1.5
        text-[10px\] font-bold text-vbs-text text-center bg-transparent outline-none
        focus:border-vbs-accent/50 transition-colors tabular-nums"
    />
  )
}

/* ── Bus Row (2 rows of 4) ── */
function BusRow({ label, activeId, bus, onSelect }: {
  label: string; activeId: number; bus: 'pgm' | 'pvw'; onSelect: (id: number) => void
}) {
  return (
    <div className="flex flex-col gap-2">
      <span className={`text-[11px\] font-black tracking-widest ${bus === 'pgm' ? 'text-vbs-pgm' : 'text-vbs-pvw'}`}>
        {label}
      </span>
      {/* 4×2 方形格 */}
      <div className="grid grid-cols-4 gap-2">
        {INPUTS.map(inp => {
          const isActive = activeId === inp.id
          return (
            <button key={inp.id} id={`${bus}-btn-${inp.id}`}
              onClick={() => onSelect(inp.id)}
              className={`
                w-12 h-12 sm:w-14 sm:h-14 rounded-xl font-bold text-[10px\] sm:text-[11px\] border transition-all duration-100
                active:scale-95 flex items-center justify-center shrink-0
                ${isActive
                  ? bus === 'pgm'
                    ? 'pgm-active text-vbs-pgm shadow-pgm scale-105'
                    : 'pvw-active text-vbs-pvw shadow-pvw scale-105'
                  : `glass-dark border-white/8 text-vbs-muted
                     hover:text-vbs-text ${bus === 'pgm' ? 'hover:border-vbs-pgm/30' : 'hover:border-vbs-pvw/30'}`}
              `}
            >{inp.label}</button>
          )
        })}
      </div>
    </div>
  )
}

/* ── T-Bar ── */
function TBar({ position, onDrag }: { position: number; onDrag: (v: number) => void }) {
  const trackRef = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)

  const update = useCallback((clientY: number) => {
    const rect = trackRef.current?.getBoundingClientRect()
    if (!rect) return
    onDrag(Math.max(0, Math.min(100, ((clientY - rect.top) / rect.height) * 100)))
  }, [onDrag])

  useEffect(() => {
    const mm = (e: MouseEvent)  => { if (dragging.current) update(e.clientY) }
    const tm = (e: TouchEvent)  => { if (dragging.current && e.touches[0]) update(e.touches[0].clientY) }
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
      <span className="text-[9px\] font-black text-vbs-muted tracking-widest">BAR</span>
      <div ref={trackRef}
        className="relative rounded-full bg-white/5 border border-white/10 cursor-pointer w-10 sm:w-12"
        style={{ height: '240px' }}
        onMouseDown={e => { dragging.current = true; update(e.clientY) }}
        onTouchStart={e => { dragging.current = true; update(e.touches[0].clientY) }}
      >
        {/* 進度填色 */}
        <div className="absolute top-0 left-0 right-0 rounded-full transition-none"
          style={{ height: `${position}%`, background: 'linear-gradient(to bottom, rgba(30,144,255,0.4), rgba(30,144,255,0.1))' }} />

        {/* 把手 */}
        <div className="absolute left-1/2 -translate-x-1/2 -translate-y-1/2 z-10"
          style={{ top: `${position}%` }}>
          <div className="w-16 sm:w-20 h-8 rounded-lg cursor-grab active:cursor-grabbing
            flex flex-col items-center justify-center gap-[4px]
            bg-gradient-to-b from-slate-300 to-slate-500 border border-white/30 shadow-2xl">
            <div className="w-10 h-[2px] bg-black/40 rounded-full" />
            <div className="w-10 h-[2px] bg-black/40 rounded-full" />
            <div className="w-10 h-[2px] bg-black/40 rounded-full" />
          </div>
        </div>
      </div>
      <span className="text-[9px\] font-bold text-vbs-accent tabular-nums mt-1">{Math.round(position)}%</span>
    </div>
  )
}

/* ── Main Switcher ── */
export default function Switcher({ pgm, pvw, setPgm, setPvw, compact }: SwitcherProps) {
  const [tbar, setTbar]           = useState(0)
  const [transitioning, setTrans] = useState(false)
  const [autoRate, setAutoRate]   = useState('01:00')
  const [ftbRate,  setFtbRate]    = useState('01:00')
  const [ftbOn,    setFtbOn]      = useState(false)
  const rafRef   = useRef<number>()
  const cooldown = useRef(false)

  /* 動畫 T-bar */
  const animateTo = useCallback((from: number, to: number, ms: number, done?: () => void) => {
    if (rafRef.current) cancelAnimationFrame(rafRef.current)
    setTrans(true)
    const start = performance.now()
    const tick = (now: number) => {
      const p = Math.min(1, (now - start) / ms)
      const e = p < 0.5 ? 2 * p * p : 1 - Math.pow(-2 * p + 2, 2) / 2
      setTbar(from + (to - from) * e)
      if (p < 1) { rafRef.current = requestAnimationFrame(tick) }
      else { setTbar(to); setTrans(false); cooldown.current = false; done?.() }
    }
    rafRef.current = requestAnimationFrame(tick)
  }, [])

  /* CUT */
  const doCut = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    const [np, nv] = [pvw, pgm]
    setPgm(np); setPvw(nv); setTbar(0)
    setTimeout(() => { cooldown.current = false }, 120)
  }, [pgm, pvw, setPgm, setPvw])

  /* AUTO */
  const doAuto = useCallback(() => {
    if (cooldown.current || transitioning) return
    cooldown.current = true
    animateTo(tbar, tbar < 50 ? 100 : 0, parseRate(autoRate), () => {
      if (tbar < 50) { const [np, nv] = [pvw, pgm]; setPgm(np); setPvw(nv); setTbar(0) }
    })
  }, [pgm, pvw, setPgm, setPvw, tbar, autoRate, transitioning, animateTo])

  /* FTB */
  const doFtb = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    if (!ftbOn) { setFtbOn(true);  animateTo(0, 100, parseRate(ftbRate)) }
    else        { setFtbOn(false); animateTo(100, 0, parseRate(ftbRate)) }
  }, [ftbOn, ftbRate, animateTo])

  /* 手動拖 T-bar */
  const handleTbarDrag = useCallback((v: number) => {
    if (transitioning) return
    setTbar(v)
    if (v >= 99.5) {
      const [np, nv] = [pvw, pgm]; setPgm(np); setPvw(nv)
      setTimeout(() => setTbar(0), 80)
    }
  }, [pgm, pvw, setPgm, setPvw, transitioning])

  return (
    <div className={`glass rounded-xl flex flex-col gap-4 ${compact ? 'p-4' : 'p-6 md:p-8'}`}>
      {/* Header */}
      <div className="flex items-center justify-between pb-2 border-b border-white/5">
        <span className="text-[11px\] font-black text-vbs-muted uppercase tracking-widest">Virtual Switcher</span>
        <span className="text-[9px\] font-bold text-vbs-pvw hidden sm:inline">EYEVINN CORE</span>
      </div>

      {/* Main Switcher Layout: 3 Columns */}
      <div className="flex flex-col md:flex-row gap-6 md:gap-10 items-center md:items-stretch justify-center">
        
        {/* Column 1: Bus Buttons */}
        <div className="flex flex-col gap-6">
          {/* Program Bus */}
          <BusRow label="PGM" activeId={pgm} bus="pgm"
            onSelect={id => { if (!cooldown.current) setPgm(id) }} />

          {/* Preview Bus */}
          <BusRow label="PVM" activeId={pvw} bus="pvw"
            onSelect={id => { if (!cooldown.current) setPvw(id) }} />
        </div>

        {/* Column 2: T-Bar */}
        <div className="hidden md:flex flex-col items-center justify-center px-4 border-l border-r border-white/5">
          <TBar position={tbar} onDrag={handleTbarDrag} />
        </div>
        {/* Mobile T-Bar */}
        <div className="md:hidden w-full flex justify-center py-4 border-t border-b border-white/5">
           <TBar position={tbar} onDrag={handleTbarDrag} />
        </div>

        {/* Column 3: Controls */}
        <div className="flex flex-col gap-6 justify-center">
          
          {/* AUTO Block */}
          <div className="flex flex-col items-center gap-2">
            <button id="auto-btn" onClick={doAuto} disabled={transitioning}
              className={`
                w-16 h-16 sm:w-20 sm:h-20 rounded-xl font-black text-sm sm:text-[9px\] border transition-all active:scale-95 shadow-lg
                ${transitioning
                  ? 'bg-vbs-accent/30 border-vbs-accent text-vbs-accent shadow-[0_0_20px_rgba(30,144,255,0.4)]'
                  : 'glass border-vbs-accent/50 text-vbs-accent hover:bg-vbs-accent/15 hover:border-vbs-accent'}
              `}>AUTO</button>
             <RateInput id="auto-rate" value={autoRate} onChange={setAutoRate} />
          </div>

          <div className="flex gap-4">
            {/* CUT Block */}
            <div className="flex flex-col items-center gap-2">
              <button id="cut-btn" onClick={doCut}
                className="w-16 h-16 sm:w-20 sm:h-20 rounded-xl font-black text-sm sm:text-[9px\] border transition-all active:scale-95 shadow-lg
                  glass border-vbs-pgm/50 text-vbs-pgm hover:bg-vbs-pgm/20 hover:border-vbs-pgm hover:shadow-[0_0_20px_rgba(255,59,59,0.5)]">
                CUT
              </button>
               {/* Invisible spacer to match RATE input height below FTM */}
               <div className="h-9"></div>
            </div>

            {/* FTM Block */}
            <div className="flex flex-col items-center gap-2">
              <button id="ftm-btn" onClick={doFtb}
                className={`
                  w-16 h-16 sm:w-20 sm:h-20 rounded-xl font-black text-sm sm:text-[9px\] border transition-all active:scale-95 shadow-lg
                  ${ftbOn
                    ? 'bg-black border-white/60 text-white shadow-[0_0_20px_rgba(255,255,255,0.3)]'
                    : 'glass border-white/20 text-vbs-muted hover:text-vbs-text hover:border-white/40'}
                `}>FTM</button>
               <RateInput id="ftb-rate" value={ftbRate} onChange={setFtbRate} />
            </div>
          </div>

        </div>

      </div>
    </div>
  )
}
