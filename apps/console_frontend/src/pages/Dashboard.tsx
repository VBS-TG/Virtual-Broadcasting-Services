import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useRuntimeStore } from '../stores/runtimeStore'
import { useTelemetryStore } from '../stores/telemetryStore'
import TelemetryPanel from '../components/TelemetryPanel'
import PageShell from '../components/PageShell'
import { NAV_ITEMS } from '../config/navigation'

export default function Dashboard() {
  const navigate = useNavigate()
  const { config, fetch: fetchRuntime } = useRuntimeStore()
  const telemetry = useTelemetryStore((s) => s.data)
  const fetchTelemetry = useTelemetryStore((s) => s.fetch)

  useEffect(() => {
    fetchRuntime()
    fetchTelemetry()
  }, [fetchRuntime, fetchTelemetry])

  const nodeStatus = [
    { label: 'CONSOLE', online: Boolean(config) },
    { label: 'ROUTE', online: Boolean(telemetry?.route?.online) },
    { label: 'ENGINE', online: Boolean(telemetry?.engine?.online) },
  ]

  return (
    <PageShell 
      title="總覽" 
      description="System Overview"
      extra={<span className="text-[12px] font-bold text-vbs-accent bg-vbs-accent/10 px-3 py-1 rounded-full uppercase tracking-tighter">Live Status</span>}
    >
      <div className="grid grid-cols-1 md:grid-cols-12 gap-5 auto-rows-min">
        
        {/* ── 核心節點狀態 ── */}
        <div className="md:col-span-8 grid grid-cols-1 sm:grid-cols-3 gap-5">
          {nodeStatus.map((n) => (
            <div key={n.label} className="glass rounded-[32px] p-6 flex flex-col justify-between shadow-xl min-h-[160px]">
              <div className="flex items-center gap-2">
                <span className={`w-2 h-2 rounded-full animate-pulse-slow shrink-0 ${n.online ? 'bg-vbs-pvw' : 'bg-vbs-pgm'}`} />
                <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">{n.label}</span>
              </div>
              <div className="mt-4 flex items-baseline">
                <span className={`text-5xl font-black tracking-tighter drop-shadow-md ${n.online ? 'text-white' : 'text-vbs-pgm'}`}>
                  {n.online ? 'ON' : 'OFF'}
                </span>
                <span className="text-[12px] text-vbs-muted ml-2 font-bold uppercase tracking-widest">{n.online ? 'Online' : 'Offline'}</span>
              </div>
            </div>
          ))}
        </div>

        {/* ── 快速導覽分區 ── */}
        <div className="md:col-span-4 grid grid-cols-2 gap-4">
          {NAV_ITEMS.slice(1).map((item) => (
            <button
              key={item.path}
              onClick={() => navigate(item.path)}
              className="glass rounded-[28px] flex flex-col items-center justify-center p-5 hover:bg-white/5 active:scale-95 transition-all shadow-lg group border border-white/5"
            >
              <item.icon className="w-6 h-6 text-vbs-muted group-hover:text-vbs-accent mb-2 transition-colors" />
              <span className="text-[14px] font-black text-white tracking-widest uppercase drop-shadow-md">{item.label}</span>
              <span className="text-[10px] font-bold text-vbs-muted uppercase tracking-tight mt-1 opacity-60">{item.labelEn}</span>
            </button>
          ))}
        </div>

        {/* ── 資源統計摘要 ── */}
        <div className="md:col-span-12 mt-2">
          <div className="glass rounded-[36px] p-8 shadow-2xl overflow-hidden relative">
             <div className="flex items-center justify-between mb-8">
               <div className="flex flex-col">
                  <span className="text-[12px] font-black text-vbs-muted uppercase tracking-widest">Resource Telemetry</span>
                  <span className="text-2xl font-black text-white uppercase tracking-tighter">實時數據摘要</span>
               </div>
               <button onClick={() => navigate('/telemetry')} className="text-[11px] font-black text-vbs-accent bg-vbs-accent/10 px-4 py-2 rounded-xl hover:bg-vbs-accent/20 transition-all uppercase tracking-widest border border-vbs-accent/20">查看詳情</button>
             </div>
             <TelemetryPanel compact />
          </div>
        </div>

      </div>
    </PageShell>
  )
}