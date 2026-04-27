import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useRuntimeStore } from '../stores/runtimeStore'
import { useSwitcherStore } from '../stores/switcherStore'
import { useTelemetryStore } from '../stores/telemetryStore'
import Multiviewer from '../components/Multiviewer'
import Switcher from '../components/Switcher'
import TelemetryPanel from '../components/TelemetryPanel'

export default function Dashboard() {
  const navigate = useNavigate()
  const { config, fetch: fetchRuntime, lastApplyResult } = useRuntimeStore()
  const { state: switchState, fetchState } = useSwitcherStore()
  const telemetry = useTelemetryStore((s) => s.data)
  const fetchTelemetry = useTelemetryStore((s) => s.fetch)

  useEffect(() => {
    fetchRuntime()
    fetchState()
    fetchTelemetry()
  }, [fetchRuntime, fetchState, fetchTelemetry])

  const nodeCards = [
    { id: 'console', label: 'CONSOLE', online: Boolean(config) },
    { id: 'route', label: 'ROUTE', online: Boolean(telemetry?.route?.online) },
    { id: 'engine', label: 'ENGINE', online: Boolean(telemetry?.engine?.online) },
  ]

  return (
    <div className="h-full overflow-y-auto p-4 md:p-6 lg:p-8">
      {/* Bento Box Grid */}
      <div className="grid grid-cols-1 md:grid-cols-12 gap-5 auto-rows-min max-w-[1600px] mx-auto">
        
        {/* ── 狀態條 (跨全寬) ── */}
        <div className="md:col-span-12">
          <StatusStrip nodes={nodeCards} />
        </div>

        {/* ── Node Status Cards (左側 8 格，分三塊) ── */}
        <div className="md:col-span-8 grid grid-cols-3 gap-5">
          {nodeCards.map((n) => (
            <div key={n.id} className="glass rounded-3xl p-5 md:p-6 flex flex-col justify-between shadow-xl min-h-[140px]">
              <div className="flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full animate-pulse-slow shrink-0 ${n.online ? 'bg-vbs-pvw' : 'bg-vbs-pgm'}`} />
                <span className="text-[12px] font-semibold text-vbs-muted uppercase tracking-widest">{n.label}</span>
              </div>
              <div className="mt-4 flex items-baseline">
                <span className={`text-4xl md:text-5xl font-black tracking-tighter drop-shadow-md ${n.online ? 'text-white' : 'text-vbs-pgm'}`}>
                  {n.online ? 'ON' : 'OFF'}
                </span>
                <span className="text-[12px] text-vbs-muted ml-2 font-bold">{n.online ? 'LINE' : 'LINE'}</span>
              </div>
            </div>
          ))}
        </div>

        {/* ── Runtime Summary Card (右側 4 格) ── */}
        <div className="md:col-span-4 glass rounded-3xl p-5 md:p-6 flex flex-col justify-between shadow-xl min-h-[140px]">
          <div className="flex items-center justify-between">
            <span className="text-[12px] font-semibold text-vbs-muted uppercase tracking-widest">Runtime Config</span>
            <button onClick={() => navigate('/runtime')} className="text-[11px] font-bold text-vbs-accent hover:text-white transition-colors bg-white/5 px-2 py-1 rounded-md">編輯</button>
          </div>
          {config ? (
            <div className="flex justify-between items-end mt-4">
              <div className="flex flex-col">
                <span className="text-[11px] text-vbs-muted uppercase font-bold mb-1">Inputs</span>
                <span className="text-4xl md:text-5xl font-black text-white tracking-tighter drop-shadow-md">{config.inputs}</span>
              </div>
              <div className="flex flex-col items-end">
                <span className="text-[11px] text-vbs-muted uppercase font-bold mb-1">PGM / AUX</span>
                <span className="text-3xl md:text-4xl font-bold text-white tracking-tighter drop-shadow-md">{config.pgm_count}<span className="text-vbs-muted text-2xl mx-1">/</span>{config.aux_count}</span>
              </div>
            </div>
          ) : (
            <p className="text-[12px] text-vbs-muted mt-4">載入中…</p>
          )}
        </div>

        {/* ── Last Apply Summary Card (跨全寬或 8 格) ── */}
        <div className="md:col-span-8 glass rounded-3xl p-5 md:p-6 shadow-xl flex flex-col justify-between min-h-[140px]">
          <span className="text-[12px] font-semibold text-vbs-muted uppercase tracking-widest mb-4">Last Apply Result</span>
          {lastApplyResult ? (() => {
            const r = lastApplyResult
            return (
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                <div className="flex flex-col">
                  <span className="text-[11px] text-vbs-muted uppercase font-bold mb-1">Time</span>
                  <span className="text-2xl font-black text-white tracking-tight">{new Date(r.timestamp).toLocaleTimeString('zh-TW', { hour12: false, hour: '2-digit', minute:'2-digit' })}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[11px] text-vbs-muted uppercase font-bold mb-1">Route</span>
                  <span className={`text-2xl font-black tracking-tight drop-shadow-md ${r.route ? 'text-vbs-pvw' : 'text-vbs-pgm'}`}>{r.route ? 'OK' : 'FAIL'}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[11px] text-vbs-muted uppercase font-bold mb-1">Engine</span>
                  <span className={`text-2xl font-black tracking-tight drop-shadow-md ${r.engine ? 'text-vbs-pvw' : 'text-vbs-pgm'}`}>{r.engine ? 'OK' : 'FAIL'}</span>
                </div>
                <div className="flex flex-col">
                  <span className="text-[11px] text-vbs-muted uppercase font-bold mb-1">Rollback</span>
                  <span className={`text-2xl font-black tracking-tight drop-shadow-md ${r.rolled_back ? 'text-vbs-warning' : 'text-white/30'}`}>{r.rolled_back ? 'YES' : 'NO'}</span>
                </div>
              </div>
            )
          })() : (
            <p className="text-[12px] text-vbs-muted">尚未 Apply</p>
          )}
        </div>

        {/* ── Quick Actions (右側 4 格) ── */}
        <div className="md:col-span-4 grid grid-cols-2 gap-3">
          {[
            { id: 'qa-switcher',    label: 'Switcher',   path: '/switcher' },
            { id: 'qa-runtime',     label: 'Runtime',    path: '/runtime' },
            { id: 'qa-telemetry',   label: 'Telemetry',  path: '/telemetry' },
            { id: 'qa-system',      label: 'Health',     path: '/system' },
          ].map((a) => (
            <button
              key={a.id} id={a.id} onClick={() => navigate(a.path)}
              className="glass rounded-2xl flex flex-col items-center justify-center p-4 hover:bg-white/5 active:scale-95 transition-all shadow-lg"
            >
              <span className="text-[16px] font-bold text-white drop-shadow-md">{a.label}</span>
            </button>
          ))}
        </div>

        {/* ── 主內容：Multiviewer + Switcher ── */}
        <div className="md:col-span-12 grid grid-cols-1 md:grid-cols-12 gap-5 mt-2">
          <div className="md:col-span-8 glass rounded-3xl p-4 shadow-xl">
            <Multiviewer pgm={switchState.program} pvw={switchState.preview} compact />
          </div>
          <div className="md:col-span-4 glass rounded-3xl p-4 shadow-xl">
            <Switcher compact />
          </div>
        </div>

        {/* ── Telemetry 摘要 ── */}
        <div className="md:col-span-12 glass rounded-3xl shadow-xl overflow-hidden mt-2">
          <TelemetryPanel compact />
        </div>

      </div>
    </div>
  )
}

function StatusStrip({ nodes }: { nodes: Array<{ id: string; label: string; online: boolean }> }) {
  const [time, setTime] = useState(new Date())
  useEffect(() => {
    const t = setInterval(() => setTime(new Date()), 1000)
    return () => clearInterval(t)
  }, [])

  return (
    <div className="glass rounded-2xl px-4 py-3 flex items-center justify-between shadow-lg">
      <div className="flex items-center gap-6">
        {nodes.map((n) => (
          <div key={n.id} className="flex items-center gap-2">
            <span className={`w-2 h-2 rounded-full ${n.online ? 'bg-vbs-pvw shadow-[0_0_8px_rgba(16,185,129,0.8)]' : 'bg-vbs-pgm shadow-[0_0_8px_rgba(255,59,59,0.8)]'}`} />
            <span className="text-[11px] font-bold text-vbs-muted hidden sm:inline">{n.label}</span>
          </div>
        ))}
      </div>
      <div className="flex items-baseline gap-2">
        <span className="text-[11px] font-bold text-vbs-muted uppercase tracking-wider hidden md:inline">SRT AES-256</span>
        <span className="text-xl font-black text-white tracking-tight drop-shadow-md">
          {time.toLocaleTimeString('zh-TW', { hour12: false, hour: '2-digit', minute:'2-digit', second:'2-digit' })}
        </span>
      </div>
    </div>
  )
}
