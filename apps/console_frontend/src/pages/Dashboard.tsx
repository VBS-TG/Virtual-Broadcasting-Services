import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useRuntimeStore } from '../stores/runtimeStore'
import { useSwitcherStore } from '../stores/switcherStore'
import Multiviewer from '../components/Multiviewer'
import Switcher from '../components/Switcher'
import TelemetryPanel from '../components/TelemetryPanel'

// [MOCK] 節點狀態目前固定為 ONLINE
// TODO: 後端就緒後從 /healthz 或 telemetry 取得真實狀態
const MOCK_NODES = [
  { id: 'console', label: 'CONSOLE', status: 'ONLINE' as const, color: 'text-vbs-pvw', dot: 'bg-vbs-pvw' },
  { id: 'route',   label: 'ROUTE',   status: 'ONLINE' as const, color: 'text-vbs-pvw', dot: 'bg-vbs-pvw' },
  { id: 'engine',  label: 'ENGINE',  status: 'ONLINE' as const, color: 'text-vbs-pvw', dot: 'bg-vbs-pvw' },
]

export default function Dashboard() {
  const navigate = useNavigate()
  const { config, fetch: fetchRuntime } = useRuntimeStore()
  const { state: switchState, fetchState } = useSwitcherStore()

  useEffect(() => {
    fetchRuntime()
    fetchState()
  }, [fetchRuntime, fetchState])

  return (
    <div className="h-full overflow-y-auto">
      <div className="p-3 md:p-4 flex flex-col gap-3">

        {/* ── 狀態條 ── */}
        <StatusStrip />

        {/* ── Node Status Cards ── */}
        <div className="grid grid-cols-3 gap-3">
          {MOCK_NODES.map((n) => (
            <div key={n.id} className="glass rounded-xl p-3 flex flex-col gap-1.5">
              <div className="flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full animate-pulse-slow shrink-0 ${n.dot}`} />
                <span className="text-[15px] font-black text-vbs-muted tracking-widest">{n.label}</span>
              </div>
              <span className={`text-[17px] font-black ${n.color}`}>{n.status}</span>
            </div>
          ))}
        </div>

        {/* ── Runtime Summary Card ── */}
        <div className="glass rounded-xl p-4 flex flex-col gap-2">
          <div className="flex items-center justify-between">
            <span className="text-[15px] font-black text-vbs-muted uppercase tracking-widest">Runtime Config</span>
            <button
              id="goto-runtime"
              onClick={() => navigate('/runtime')}
              className="text-[15px] font-bold text-vbs-accent hover:underline"
            >
              編輯 →
            </button>
          </div>
          {config ? (
            <div className="grid grid-cols-3 gap-3 text-[17px]">
              <div>
                <p className="text-vbs-muted text-[15px] mb-0.5">Inputs</p>
                <p className="font-bold text-vbs-text">{config.inputs}</p>
              </div>
              <div>
                <p className="text-vbs-muted text-[15px] mb-0.5">PGM</p>
                <p className="font-bold text-vbs-text">{config.pgm_count}</p>
              </div>
              <div>
                <p className="text-vbs-muted text-[15px] mb-0.5">AUX</p>
                <p className="font-bold text-vbs-text">{config.aux_count}</p>
              </div>
            </div>
          ) : (
            <p className="text-[15px] text-vbs-muted">載入中…</p>
          )}
        </div>

        {/* ── Quick Actions ── */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
          {[
            { id: 'qa-switcher',    label: ' 導播切換', path: '/switcher' },
            { id: 'qa-runtime',     label: ' Runtime',   path: '/runtime' },
            { id: 'qa-telemetry',   label: ' 遙測監控',  path: '/telemetry' },
            { id: 'qa-system',      label: ' 健康檢查',  path: '/system' },
          ].map((a) => (
            <button
              key={a.id}
              id={a.id}
              onClick={() => navigate(a.path)}
              className="glass border border-white/8 rounded-xl py-3 text-[15px] font-bold text-vbs-muted
                hover:text-vbs-text hover:border-vbs-accent/30 hover:glass transition-all active:scale-95"
            >
              {a.label}
            </button>
          ))}
        </div>

        {/* ── 主內容：Multiviewer + Switcher ── */}
        <div className="flex flex-col md:grid md:grid-cols-12 gap-3">
          <div className="md:col-span-8">
            <Multiviewer pgm={switchState.program} pvw={switchState.preview} compact />
          </div>
          <div className="md:col-span-4">
            <Switcher compact />
          </div>
        </div>

        {/* ── Telemetry 摘要 ── */}
        <TelemetryPanel compact />

      </div>
    </div>
  )
}

function StatusStrip() {
  const [time, setTime] = useState(new Date())
  useEffect(() => {
    const t = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(t)
  }, [])

  return (
    <div className="glass rounded-xl px-3 md:px-4 py-2.5 flex flex-wrap items-center justify-between gap-2">
      <div className="flex items-center gap-3 md:gap-5">
        {MOCK_NODES.map((n) => (
          <div key={n.id} className="flex items-center gap-1.5">
            <span className={`w-1.5 h-1.5 rounded-full animate-pulse-slow shrink-0 ${n.dot}`} />
            <span className="text-[15px] font-semibold text-vbs-muted hidden sm:inline">{n.label}</span>
            <span className={`text-[15px] font-black ${n.color}`}>{n.status}</span>
          </div>
        ))}
      </div>
      <div className="flex items-center gap-3">
        <span className="text-[15px] text-vbs-muted hidden md:inline">SRT AES-256 </span>
        <span className="text-[15px] font-semibold text-vbs-text tabular-nums">
          {time.toLocaleTimeString('zh-TW', { hour12: false })}
        </span>
      </div>
    </div>
  )
}
