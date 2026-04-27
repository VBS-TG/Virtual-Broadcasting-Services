import { useState, useRef, useCallback } from 'react'
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
      className="w-[68px] h-[36px] shrink-0 glass-dark border border-white/10 rounded-lg px-2 
        text-[13px] font-bold text-vbs-text text-center bg-transparent outline-none
        focus:border-vbs-accent/50 transition-colors tabular-nums shadow-inner"
    />
  )
}

function BusRow({ label, activeId, bus, inputs, onSelect }: {
  label: string; activeId: number; bus: 'pgm' | 'pvw';
  inputs: Array<{ id: number; label: string }>;
  onSelect: (id: number) => void
}) {
  return (
    <div className="flex flex-col gap-3 w-full">
      <span className={`text-[12px] font-black uppercase tracking-widest ${bus === 'pgm' ? 'text-vbs-pgm' : 'text-vbs-pvw'}`}>
        {label} BUS
      </span>
      <div className="flex flex-wrap gap-3 w-full justify-start">
        {inputs.map((inp) => {
          const isActive = activeId === inp.id
          return (
            <button
              key={inp.id}
              id={`${bus}-btn-${inp.id}`}
              onClick={() => onSelect(inp.id)}
              className={`
                /* 絕對固定大小：不允許縮放、延展或擠壓變形 */
                w-[68px] h-[68px] shrink-0
                rounded-2xl font-black text-[18px]
                flex items-center justify-center 
                transition-all duration-200 active:scale-95 shadow-lg border relative
                ${isActive
                  ? bus === 'pgm'
                    ? 'pgm-active text-white shadow-[0_0_20px_rgba(255,59,59,0.5)] scale-[1.05] border-vbs-pgm/60 z-10'
                    : 'pvw-active text-white shadow-[0_0_20px_rgba(16,185,129,0.5)] scale-[1.05] border-vbs-pvw/60 z-10'
                  : 'glass-dark border-white/5 text-vbs-muted hover:text-white hover:bg-white/10 z-0'}
              `}
            >
              {inp.id}
            </button>
          )
        })}
      </div>
    </div>
  )
}

export default function Switcher({ compact, fullScreen }: SwitcherProps) {
  const { state, setProgram, setPreview } = useSwitcherStore()
  const runtimeInputs = useRuntimeStore((s) => s.config?.inputs ?? 8)
  const inputs = DEFAULT_INPUTS.slice(0, Math.max(1, Math.min(8, runtimeInputs)))

  const [transitioning, setTransitioning] = useState(false)
  const [ftbOn, setFtbOn] = useState(false)
  const [autoRate, setAutoRate] = useState('01:00')
  const [ftbRate] = useState('01:00')
  const [lastOp, setLastOp] = useState<{ time: string; payload: any; error?: string } | null>(null)
  const cooldown = useRef(false)

  const logOp = useCallback((action: string, payload: any) => {
    setLastOp({
      time: new Date().toLocaleTimeString('zh-TW', { hour12: false }),
      payload: { action, ...payload }
    })
  }, [])

  const doCut = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    const [np, nv] = [state.preview, state.program]
    setProgram(np)
    setPreview(nv)
    logOp('CUT', { program: np, preview: nv })
    setTimeout(() => { cooldown.current = false }, 120)
  }, [state, setProgram, setPreview, logOp])

  const doAuto = useCallback(() => {
    if (cooldown.current || transitioning) return
    cooldown.current = true
    setTransitioning(true)
    
    setTimeout(() => {
      setProgram(state.preview)
      setPreview(state.program)
      logOp('AUTO', { program: state.preview, preview: state.program, rate: autoRate })
      setTransitioning(false)
      setTimeout(() => { cooldown.current = false }, 100)
    }, parseRate(autoRate))
  }, [state, autoRate, transitioning, setProgram, setPreview, logOp])

  const doFtb = useCallback(() => {
    if (cooldown.current) return
    cooldown.current = true
    setTransitioning(true)

    setTimeout(() => {
      setFtbOn(prev => {
        const next = !prev
        logOp(next ? 'FTB_ON' : 'FTB_OFF', { rate: ftbRate })
        return next
      })
      setTransitioning(false)
      setTimeout(() => { cooldown.current = false }, 100)
    }, parseRate(ftbRate))
  }, [ftbRate, logOp])

  return (
    <div className={`${fullScreen ? 'w-full h-full flex flex-col gap-3 p-4' : `flex flex-col gap-4 ${compact ? 'p-1' : 'p-2'}`}`}>
      {!fullScreen && !compact && (
        <div className="flex items-center justify-between pb-2 border-b border-white/5">
          <span className="text-[12px] font-bold text-vbs-muted uppercase tracking-widest">Virtual Switcher</span>
          <span className="text-[12px] font-bold text-vbs-pvw hidden sm:inline uppercase">VBS Core</span>
        </div>
      )}

      <div className={`flex flex-col gap-4 ${fullScreen ? 'flex-1 h-full' : ''}`}>

        {/* ── 上半部：Bus 按鈕區 ── */}
        <div className={`flex flex-col gap-6 justify-center glass-dark rounded-3xl p-5 md:p-8 shadow-xl ${fullScreen ? 'flex-1' : ''}`}>
          <BusRow label="Program" activeId={state.program} bus="pgm" inputs={inputs}
            onSelect={(id) => { if (!cooldown.current) setProgram(id) }} />
          
          <div className="h-px w-full bg-white/5 my-1" />
          
          <BusRow label="Preview" activeId={state.preview} bus="pvw" inputs={inputs}
            onSelect={(id) => { if (!cooldown.current) setPreview(id) }} />
        </div>

        {/* ── 下半部：Transition 控制橫幅 ── */}
        {/* 🚀 加入 flex-wrap 允許內部元素在寬度不夠時換行 */}
        <div className="glass rounded-3xl p-5 md:p-6 shadow-xl flex flex-wrap items-center justify-start gap-5 relative overflow-hidden">
          
          {/* Transition 標題 */}
          <span className="text-[12px] font-black text-vbs-muted uppercase tracking-[0.2em] w-full xl:w-auto shrink-0">
            Transition
          </span>

          {/* 控制按鈕群組 */}
          <div className="flex flex-wrap items-center gap-4 md:gap-6">
            
            {/* AUTO 與時間輸入區塊 */}
            <div className="flex flex-wrap items-center gap-3 shrink-0">
              <button id="auto-btn" onClick={doAuto} disabled={transitioning}
                className={`
                  /* 🚀 絕對固定大小：不變形 */
                  w-[110px] h-[55px] shrink-0
                  rounded-xl font-black text-[16px] transition-all active:scale-95 shadow-lg flex items-center justify-center
                  ${transitioning
                    ? 'bg-vbs-accent/50 text-white shadow-[0_0_20px_rgba(30,144,255,0.6)] animate-pulse'
                    : 'btn-gradient'}
                `}>
                AUTO
              </button>
              <RateInput id="auto-rate" value={autoRate} onChange={setAutoRate} />
            </div>

            {/* 硬切與 FTM 區塊 */}
            <div className="flex flex-wrap items-center gap-3 shrink-0">
              <button id="cut-btn" onClick={doCut}
                className="
                  /* 🚀 絕對固定大小：不變形 */
                  w-[90px] h-[55px] shrink-0
                  rounded-xl font-black text-[16px] border transition-all active:scale-95 shadow-lg glass border-vbs-pgm/40 text-vbs-pgm hover:bg-vbs-pgm/20 hover:border-vbs-pgm/80 hover:shadow-[0_0_20px_rgba(255,59,59,0.4)]
                ">
                CUT
              </button>
              
              <button id="ftm-btn" onClick={doFtb}
                className={`
                  /* 🚀 絕對固定大小：不變形 */
                  w-[90px] h-[55px] shrink-0
                  rounded-xl font-black text-[16px] border transition-all active:scale-95 shadow-lg
                  ${ftbOn
                    ? 'bg-white/10 border-white/40 text-white shadow-[inset_0_0_20px_rgba(255,255,255,0.2)] animate-pulse'
                    : 'glass border-white/10 text-vbs-muted hover:text-white hover:border-white/30 hover:bg-white/5'}
                `}>
                FTM
              </button>
            </div>

          </div>
        </div>
      </div>

      {/* ── Last Operation Result ── */}
      {!fullScreen && !compact && (
        <div className="mt-1">
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